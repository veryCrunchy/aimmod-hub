package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type trainingStub struct {
	saved      int
	owner      int64
	handle     string
	visibility string
	err        error
}

func (s *trainingStub) GetUserByUploadToken(_ context.Context, token string) (store.AuthUser, error) {
	if token != "Bearer synthetic" {
		return store.AuthUser{}, errors.New("unauthenticated")
	}
	return store.AuthUser{UserID: 42}, nil
}
func (s *trainingStub) GetUserBySession(_ context.Context, token string) (store.AuthUser, error) {
	if token != "synthetic" {
		return store.AuthUser{}, errors.New("unauthenticated")
	}
	return store.AuthUser{UserID: 42}, nil
}
func (s *trainingStub) SaveTraining(_ context.Context, id int64, items []store.TrainingSession) error {
	s.owner = id
	s.saved = len(items)
	return s.err
}
func (s *trainingStub) ListTraining(_ context.Context, handle string, id int64, _ int, _ string) ([]store.TrainingSession, bool, error) {
	s.owner = id
	s.handle = handle
	return []store.TrainingSession{}, false, s.err
}
func (s *trainingStub) SetTrainingVisibility(_ context.Context, id int64, _ string, v string) (bool, error) {
	s.owner = id
	s.visibility = v
	return true, s.err
}
func trainingBody() []byte {
	s := store.TrainingSession{ID: "00000000-0000-4000-8000-000000000001", CompletedAt: time.Now(), Visibility: "private", Notes: 100, Hits: 98, Within25: 80, PlayedSeconds: 30, Setup: store.TrainingSetup{Mode: "steady", Engine: "osu", Bpm: 120, Seconds: 30, AimSpacing: 100, CircleSize: 4, ApproachRate: 7, SliderBeats: 1, MusicSource: "cues", ConfigurationHash: strings.Repeat("a", 64)}}
	body, _ := json.Marshal(map[string]any{"schemaVersion": 1, "sessions": []store.TrainingSession{s}})
	return body
}
func TestTrainingUploadAuthenticationAndStrictContract(t *testing.T) {
	for _, test := range []struct {
		name, token string
		body        []byte
		status      int
	}{
		{"valid", "Bearer synthetic", trainingBody(), 200}, {"unauthenticated", "", trainingBody(), 401},
		{"trailing JSON", "Bearer synthetic", append(trainingBody(), []byte(` {}`)...), 400},
		{"spoofed owner", "Bearer synthetic", []byte(`{"schemaVersion":1,"userId":99,"sessions":[]}`), 400},
		{"oversized", "Bearer synthetic", []byte(strings.Repeat("x", 129<<10)), 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &trainingStub{}
			r := httptest.NewRequest("POST", "/", bytes.NewReader(test.body))
			r.Header.Set("Authorization", test.token)
			w := httptest.NewRecorder()
			(trainingHandler{store: stub}).upload(w, r)
			if w.Code != test.status {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
			if test.status == 200 && stub.owner != 42 {
				t.Fatal("wrong owner")
			}
			if test.status != 200 && stub.saved > 0 {
				t.Fatal("saved rejected body")
			}
		})
	}
}
func TestTrainingDuplicateConflict(t *testing.T) {
	s := &trainingStub{err: store.ErrTrainingConflict}
	r := httptest.NewRequest("POST", "/", bytes.NewReader(trainingBody()))
	r.Header.Set("Authorization", "Bearer synthetic")
	w := httptest.NewRecorder()
	(trainingHandler{store: s}).upload(w, r)
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
}
func TestTrainingPublicLookupNeverUsesViewerCredentials(t *testing.T) {
	s := &trainingStub{}
	r := httptest.NewRequest("GET", "/api/osu/v1/training/profiles/practice-player?days=90", nil)
	r.Header.Set("Authorization", "Bearer synthetic")
	w := httptest.NewRecorder()
	(trainingHandler{store: s}).profile(w, r)
	if w.Code != 200 || s.owner != 0 || s.handle != "practice-player" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("public boundary failed")
	}
}
func TestTrainingOwnerAndVisibilityBoundaries(t *testing.T) {
	s := &trainingStub{}
	h := trainingHandler{store: s}
	r := httptest.NewRequest("GET", "/api/osu/v1/training/me", nil)
	w := httptest.NewRecorder()
	h.mine(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "synthetic"})
	w = httptest.NewRecorder()
	h.mine(w, r)
	if w.Code != 200 || s.owner != 42 {
		t.Fatal("owner access failed")
	}
	for _, site := range []string{"cross-site", "same-origin"} {
		r = httptest.NewRequest("POST", "/api/osu/v1/training/visibility", strings.NewReader(`{"id":"00000000-0000-4000-8000-000000000001","visibility":"private"}`))
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "synthetic"})
		r.Header.Set("Sec-Fetch-Site", site)
		w = httptest.NewRecorder()
		h.visibility(w, r)
		if site == "cross-site" && w.Code != 403 || site == "same-origin" && w.Code != 200 {
			t.Fatal("CSRF boundary failed", w.Code)
		}
	}
}
