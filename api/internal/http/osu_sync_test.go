package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

type fakeOsuSyncStore struct {
	saved       store.OsuSyncInput
	target      store.OsuReplayUploadTarget
	completedID int64
	storageKey  string
}

func (f *fakeOsuSyncStore) GetUserByUploadToken(_ context.Context, token string) (store.AuthUser, error) {
	if token != "Bearer valid" {
		return store.AuthUser{}, io.EOF
	}
	return store.AuthUser{UserID: 42}, nil
}

func (f *fakeOsuSyncStore) SaveOsuSync(_ context.Context, _ int64, input store.OsuSyncInput) (store.OsuSyncResult, error) {
	f.saved = input
	return store.OsuSyncResult{ShareID: strings.Repeat("osu_a", 1) + strings.Repeat("a", 27), Visibility: input.Visibility, Created: true, ReplayUploadRequired: input.Replay != nil && input.Replay.UploadFile}, nil
}

func (f *fakeOsuSyncStore) GetOsuReplayUploadTarget(_ context.Context, _ int64, _ string) (store.OsuReplayUploadTarget, error) {
	return f.target, nil
}

func (f *fakeOsuSyncStore) CompleteOsuReplayUpload(_ context.Context, scoreID int64, storageKey string, _ int64) error {
	f.completedID = scoreID
	f.storageKey = storageKey
	return nil
}

func (f *fakeOsuSyncStore) GetOsuPublicReplay(context.Context, string) (store.OsuPublicReplay, error) {
	return store.OsuPublicReplay{}, nil
}

func (f *fakeOsuSyncStore) ListOsuCommunity(context.Context, int) ([]store.OsuPublicReplay, error) {
	return []store.OsuPublicReplay{}, nil
}

func (f *fakeOsuSyncStore) GetOsuPublicProfile(context.Context, string, int) (store.OsuPublicProfile, error) {
	return store.OsuPublicProfile{}, nil
}

func (f *fakeOsuSyncStore) GetOsuReplayFile(context.Context, string) (string, string, int64, error) {
	return "", "", 0, io.EOF
}

type memoryMedia struct {
	objects map[string][]byte
}

func (m *memoryMedia) Put(_ context.Context, key string, _ string, body io.Reader, _ int64) error {
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	m.objects[key], _ = io.ReadAll(body)
	return nil
}

func (m *memoryMedia) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	payload, ok := m.objects[key]
	if !ok {
		return nil, "", io.EOF
	}
	return io.NopCloser(bytes.NewReader(payload)), "application/x-osu-replay", nil
}

func (m *memoryMedia) Delete(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

func validSyncPayload() store.OsuSyncInput {
	return store.OsuSyncInput{
		SchemaVersion:  1,
		ClientUploadID: "score-1",
		ContentHash:    strings.Repeat("a", 64),
		Profile:        store.OsuProfileInput{OsuUserID: 123, Username: "player"},
		BeatmapSet:     store.OsuBeatmapSetInput{SetKey: "online:10", OnlineID: 10, Title: "Map"},
		Difficulty: store.OsuBeatmapDifficultyInput{
			DifficultyKey: "online:20", SetKey: "online:10", OnlineID: 20,
			Version: "Insane", Ruleset: "osu", StarRating: 5.2, BPM: 180, LengthMS: 90_000,
		},
		Score: store.OsuScoreInput{
			ClientScoreID: "score-1", PlayedAt: time.Now().UTC().Add(-time.Minute),
			TotalScore: 1_000_000, Accuracy: .98, MaxCombo: 500, Count300: 400, Count100: 10,
			Mods: []string{"HD"}, Passed: true,
		},
	}
}

func TestOsuSyncDefaultsToPrivateAndRequiresMatchingIdempotencyKey(t *testing.T) {
	t.Parallel()
	fakeStore := &fakeOsuSyncStore{}
	handler := newOsuSyncHandler(fakeStore, &memoryMedia{})
	payload, _ := json.Marshal(validSyncPayload())

	req := httptest.NewRequest(http.MethodPost, "/api/osu/v1/sync", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("Idempotency-Key", "score-1")
	response := httptest.NewRecorder()
	handler.handleSync(response, req)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if fakeStore.saved.Visibility != store.OsuVisibilityPrivate {
		t.Fatalf("visibility = %q, want private", fakeStore.saved.Visibility)
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/osu/v1/sync", bytes.NewReader(payload))
	badReq.Header.Set("Authorization", "Bearer valid")
	badReq.Header.Set("Idempotency-Key", "different")
	badResponse := httptest.NewRecorder()
	handler.handleSync(badResponse, badReq)
	if badResponse.Code != http.StatusBadRequest {
		t.Fatalf("mismatched key status = %d", badResponse.Code)
	}
}

func TestOsuReplayUploadVerifiesHashAndOwnership(t *testing.T) {
	t.Parallel()
	payload := []byte("osu replay bytes")
	digest := sha256.Sum256(payload)
	hash := hex.EncodeToString(digest[:])
	shareID := "osu_" + strings.Repeat("b", 32)
	fakeStore := &fakeOsuSyncStore{target: store.OsuReplayUploadTarget{
		ScoreID: 9, ShareID: shareID, Visibility: store.OsuVisibilityUnlisted, ReplaySHA256: hash,
	}}
	mediaStore := &memoryMedia{}
	handler := newOsuSyncHandler(fakeStore, mediaStore)

	req := httptest.NewRequest(http.MethodPost, "/api/osu/v1/replays/"+shareID+"/file", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("Content-Type", "application/x-osu-replay")
	req.Header.Set("X-Content-SHA256", hash)
	response := httptest.NewRecorder()
	handler.handleReplay(response, req)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if fakeStore.completedID != 9 || !strings.HasSuffix(fakeStore.storageKey, hash+".osr") {
		t.Fatalf("upload completion = (%d, %q)", fakeStore.completedID, fakeStore.storageKey)
	}
	if !bytes.Equal(mediaStore.objects[fakeStore.storageKey], payload) {
		t.Fatal("stored replay payload differs")
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/osu/v1/replays/"+shareID+"/file", bytes.NewReader([]byte("tampered")))
	badReq.Header.Set("Authorization", "Bearer valid")
	badReq.Header.Set("Content-Type", "application/x-osu-replay")
	badReq.Header.Set("X-Content-SHA256", hash)
	badResponse := httptest.NewRecorder()
	handler.handleReplay(badResponse, badReq)
	if badResponse.Code != http.StatusBadRequest {
		t.Fatalf("tampered status = %d", badResponse.Code)
	}
}

func TestPrivateReplayIsNotServedByPublicHandler(t *testing.T) {
	t.Parallel()
	fakeStore := &fakeOsuSyncStore{}
	handler := newOsuSyncHandler(fakeStore, &memoryMedia{})
	request := httptest.NewRequest(http.MethodGet, "/media/osu-replays/osu_"+strings.Repeat("c", 32)+".osr", nil)
	response := httptest.NewRecorder()
	handler.handleReplayFile(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}
