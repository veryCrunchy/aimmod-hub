package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

type replayEnrichmentStore struct {
	fakeOsuSyncStore
	replay store.OsuPublicReplay
}

func (s *replayEnrichmentStore) GetOsuPublicReplay(context.Context, string) (store.OsuPublicReplay, error) {
	return s.replay, nil
}

func (s *replayEnrichmentStore) ListOsuCommunity(context.Context, int) ([]store.OsuPublicReplay, error) {
	return []store.OsuPublicReplay{s.replay}, nil
}

func (s *replayEnrichmentStore) GetOsuPublicProfile(context.Context, string, int) (store.OsuPublicProfile, error) {
	return store.OsuPublicProfile{RecentReplays: []store.OsuPublicReplay{s.replay}}, nil
}

type replayEnrichmentProvider struct {
	mu sync.Mutex
	scoreProviderStub
	detail   osuservice.OfficialScoreDetail
	calls    int
	download osuservice.OfficialReplayDownload
}

func (s *replayEnrichmentProvider) GetPublicScore(context.Context, int64) (osuservice.OfficialScoreDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.detail, nil
}
func (s *replayEnrichmentProvider) DownloadPublicReplay(context.Context, int64) (osuservice.OfficialReplayDownload, error) {
	return s.download, nil
}

func TestUploadedReplayEnrichesOnlyExactMissingPP(t *testing.T) {
	for _, scenario := range []string{"exact", "zero", "known", "owner", "map", "mode", "score", "null", "failure"} {
		t.Run(scenario, func(t *testing.T) {
			local := store.OsuPublicReplay{ShareID: "osu_" + strings.Repeat("a", 32), Visibility: "unlisted", OnlineScoreID: 42, OsuUserID: 7, BeatmapID: 9, Ruleset: "osu", Analysis: json.RawMessage(`{"retained":true}`), HasReplayFile: true}
			official := local
			pp := 123.
			official.PerformancePoints = &pp
			provider := &replayEnrichmentProvider{detail: osuservice.OfficialScoreDetail{Status: "available"}}
			switch scenario {
			case "zero":
				pp = 0
			case "known":
				known := 99.
				local.PerformancePoints = &known
			case "owner":
				official.OsuUserID++
			case "map":
				official.BeatmapID++
			case "mode":
				official.Ruleset = "mania"
			case "score":
				official.OnlineScoreID++
			case "null":
				official.PerformancePoints = nil
			case "failure":
				provider.detail.Status = "rate_limited"
			}
			provider.detail.Item = &osuservice.PublicScoreItem{OsuPublicReplay: official}
			h := newOsuSyncHandler(&replayEnrichmentStore{replay: local}, nil, provider)
			w := httptest.NewRecorder()
			h.handleReplay(w, httptest.NewRequest("GET", "/api/osu/v1/replays/"+local.ShareID, nil))
			var result store.OsuPublicReplay
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if w.Code != 200 || w.Header().Get("Cache-Control") != "private, no-store" || string(result.Analysis) != string(local.Analysis) || !result.HasReplayFile {
				t.Fatalf("lost local data: %+v", result)
			}
			switch scenario {
			case "exact", "zero":
				if result.PerformancePoints == nil || *result.PerformancePoints != pp {
					t.Fatal("missing official PP")
				}
			case "known":
				if result.PerformancePoints == nil || *result.PerformancePoints != 99 || provider.calls != 0 {
					t.Fatal("known PP changed/queried")
				}
			default:
				if result.PerformancePoints != nil {
					t.Fatal("false enrichment")
				}
			}
		})
	}
}

func TestOfficialReplayRouteStreamsAttachmentWithoutPublicCaching(t *testing.T) {
	provider := &replayEnrichmentProvider{download: osuservice.OfficialReplayDownload{Status: "available", Body: io.NopCloser(strings.NewReader("binary-test")), Size: 11}}
	h := newOsuProfileScoresHandler(profileScoreStoreStub{}, provider)
	mux := http.NewServeMux()
	h.register(mux, "")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/osu/v1/official-scores/42/replay", nil))
	if w.Code != 200 || w.Body.String() != "binary-test" || w.Header().Get("Content-Type") != "application/octet-stream" || w.Header().Get("Cache-Control") != "private, no-store" || !strings.Contains(w.Header().Get("Content-Disposition"), "osu-42.osr") {
		t.Fatalf("%d %v %s", w.Code, w.Header(), w.Body.String())
	}
}

func TestOfficialReplayRouteReportsActualPermissionAndSizeFailures(t *testing.T) {
	for _, tc := range []struct {
		status string
		code   int
	}{{"permission_denied", 403}, {"authentication_failed", 401}, {"not_found", 404}, {"rate_limited", 429}, {"too_large", 413}, {"redirect_rejected", 502}} {
		provider := &replayEnrichmentProvider{download: osuservice.OfficialReplayDownload{Status: tc.status}}
		h := newOsuProfileScoresHandler(profileScoreStoreStub{}, provider)
		w := httptest.NewRecorder()
		h.scoreDetail(w, httptest.NewRequest("GET", "/api/osu/v1/official-scores/42/replay", nil))
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.status) {
			t.Errorf("%s: %d %s", tc.status, w.Code, w.Body.String())
		}
	}
}
