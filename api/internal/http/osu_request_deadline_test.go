package httpserver

import (
	"context"
	"encoding/json"
	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	"net/http/httptest"
	"testing"
	"time"
)

type slowScoreProvider struct{ scoreProviderStub }

func (s *slowScoreProvider) GetPublicUserScores(ctx context.Context, _ int64, _ string) (osuservice.OfficialScoresResult, error) {
	<-ctx.Done()
	return osuservice.OfficialScoresResult{Scores: []osuservice.OfficialPublicScore{{Replay: store.OsuPublicReplay{OnlineScoreID: 42, OsuUserID: 7, BeatmapID: 9, Ruleset: "osu", Visibility: "public"}}}, Coverage: osuservice.OfficialScoreCoverage{Best: osuservice.ScoreCoverage{Status: "available", Fetched: 1}}}, ctx.Err()
}
func (s *slowScoreProvider) GetPublicScore(ctx context.Context, _ int64) (osuservice.OfficialScoreDetail, error) {
	<-ctx.Done()
	return osuservice.OfficialScoreDetail{}, ctx.Err()
}

func TestProfileDeadlinePreservesKnownUploadsAndPartialOfficialScores(t *testing.T) {
	local := store.OsuPublicReplay{ShareID: "synthetic-share", OsuUserID: 7, Ruleset: "osu", Visibility: "public"}
	h := newOsuProfileScoresHandler(profileScoreStoreStub{profile: store.OsuPublicProfile{OsuUserID: 7, RecentReplays: []store.OsuPublicReplay{local}}}, &slowScoreProvider{})
	h.timeout = 20 * time.Millisecond
	w := httptest.NewRecorder()
	h.profileScores(w, httptest.NewRequest("GET", "/api/osu/v1/profile-scores/synthetic-player", nil))
	var result profileScoresResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(result.Items) != 2 || result.Coverage.Best.Fetched != 1 || result.Coverage.Recent.Status != "unavailable" {
		t.Fatalf("partial response lost: status=%d items=%d coverage=%+v", w.Code, len(result.Items), result.Coverage)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("timeout response cached")
	}
}

func TestScoreDeadlineReturnsExplicitUncachedUnavailable(t *testing.T) {
	h := newOsuProfileScoresHandler(profileScoreStoreStub{}, &slowScoreProvider{})
	h.timeout = 20 * time.Millisecond
	w := httptest.NewRecorder()
	h.scoreDetail(w, httptest.NewRequest("GET", "/api/osu/v1/official-scores/42", nil))
	if w.Code != 503 || w.Body.Len() == 0 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("timeout looked successful: %d %q", w.Code, w.Body.String())
	}
}
