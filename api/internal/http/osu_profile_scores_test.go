package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

type profileScoreStoreStub struct {
	profile store.OsuPublicProfile
	err     error
}

func (s profileScoreStoreStub) GetOsuPublicProfile(context.Context, string, int) (store.OsuPublicProfile, error) {
	return s.profile, s.err
}

func (s profileScoreStoreStub) GetOsuPublicProfileByOsuUserID(context.Context, int64, int) (store.OsuPublicProfile, error) {
	return s.profile, s.err
}

type numericProfileStoreStub struct{ profileScoreStoreStub }

func (s numericProfileStoreStub) GetOsuPublicProfile(context.Context, string, int) (store.OsuPublicProfile, error) {
	return store.OsuPublicProfile{OsuUserID: 999, HubHandle: "77"}, nil
}

func TestNumericProfileScoresKeepsLinkedUploadsWithoutNumericHandleConflation(t *testing.T) {
	local := store.OsuPublicReplay{ShareID: "local", OsuUserID: 77, Ruleset: "osu", Visibility: "public"}
	st := numericProfileStoreStub{profileScoreStoreStub{profile: store.OsuPublicProfile{OsuUserID: 77, HubHandle: "linked-player", RecentReplays: []store.OsuPublicReplay{local}, SharedReplayCount: 1}}}
	provider := &scoreProviderStub{}
	h := newOsuProfileScoresHandler(st, provider)
	w := httptest.NewRecorder()
	h.profileScores(w, httptest.NewRequest("GET", "/api/osu/v1/profile-scores/77", nil))
	var result profileScoresResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || result.Profile.HubHandle != "linked-player" || provider.userID != 77 || len(result.Items) != 1 || result.Items[0].ShareID != "local" {
		t.Fatalf("%d %+v", w.Code, result)
	}
}

type scoreProviderStub struct {
	result  osuservice.OfficialScoresResult
	profile store.OsuPublicProfile
	userID  int64
}

func (s *scoreProviderStub) GetPublicUserScores(_ context.Context, id int64, _ string) (osuservice.OfficialScoresResult, error) {
	s.userID = id
	return s.result, nil
}
func (s *scoreProviderStub) GetPublicScoreProfile(_ context.Context, id int64, _ string) (store.OsuPublicProfile, error) {
	s.userID = id
	return s.profile, nil
}
func (s *scoreProviderStub) GetPublicScore(context.Context, int64) (osuservice.OfficialScoreDetail, error) {
	return osuservice.OfficialScoreDetail{Status: "not_found"}, nil
}

func TestProfileScoresRegisteredRouteMergesAndRetainsCoverage(t *testing.T) {
	pp := 123.
	local := store.OsuPublicReplay{ShareID: "osu_local", Visibility: "public", OsuUserID: 7, OnlineScoreID: 42, BeatmapID: 9, Ruleset: "osu", HasReplayFile: true}
	official := local
	official.ShareID = ""
	official.PerformancePoints = &pp
	provider := &scoreProviderStub{result: osuservice.OfficialScoresResult{Scores: []osuservice.OfficialPublicScore{{Replay: official}}, Coverage: osuservice.OfficialScoreCoverage{Best: osuservice.ScoreCoverage{Status: "rate_limited"}}}}
	h := newOsuProfileScoresHandler(profileScoreStoreStub{profile: store.OsuPublicProfile{OsuUserID: 7, RecentReplays: []store.OsuPublicReplay{local}, SharedReplayCount: 2}}, provider)
	mux := http.NewServeMux()
	h.register(mux, "")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/osu/v1/profile-scores/player", nil))
	var result profileScoresResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(result.Items) != 1 || result.Items[0].Source != "merged" || *result.Items[0].PerformancePoints != pp || !result.Local.HasMore || result.Coverage.CompleteHistory || result.Coverage.Best.Status != "rate_limited" {
		t.Fatalf("status%d result%+v", w.Code, result)
	}
}

func TestProfileScoresNumericOfficialUserWithoutHub(t *testing.T) {
	provider := &scoreProviderStub{profile: store.OsuPublicProfile{OsuUserID: 77, OsuUsername: "Official player"}}
	h := newOsuProfileScoresHandler(profileScoreStoreStub{err: errors.New("no Hub profile")}, provider)
	w := httptest.NewRecorder()
	h.profileScores(w, httptest.NewRequest("GET", "/api/osu/v1/profile-scores/77?mode=osu", nil))
	var result profileScoresResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || provider.userID != 77 || result.Profile.OsuUsername != "Official player" {
		t.Fatalf("%d %+v", w.Code, result)
	}
}

func TestProfileScoresValidationAndLocalOnlyAvailability(t *testing.T) {
	h := newOsuProfileScoresHandler(profileScoreStoreStub{profile: store.OsuPublicProfile{OsuUserID: 7}}, nil)
	for _, suffix := range []string{"player?mode=unknown", "player?limit=0", "player?limit=101", "player/extra", ""} {
		w := httptest.NewRecorder()
		h.profileScores(w, httptest.NewRequest("GET", "/api/osu/v1/profile-scores/"+suffix, nil))
		if w.Code != 400 {
			t.Errorf("%s returned%d", suffix, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.profileScores(w, httptest.NewRequest("GET", "/api/osu/v1/profile-scores/player", nil))
	var result profileScoresResponse
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 200 || result.Coverage.Best.Status != "not_configured" {
		t.Fatalf("%d %+v", w.Code, result)
	}
}

func TestOfficialScoreDetailValidationAndNotFound(t *testing.T) {
	h := newOsuProfileScoresHandler(profileScoreStoreStub{}, &scoreProviderStub{})
	for _, tc := range []struct {
		id   string
		want int
	}{{"0", 400}, {"abc", 400}, {"42", 404}, {"42/file", 400}} {
		w := httptest.NewRecorder()
		h.scoreDetail(w, httptest.NewRequest("GET", "/api/osu/v1/official-scores/"+tc.id, nil))
		if w.Code != tc.want {
			t.Errorf("%s got%d want%d", tc.id, w.Code, tc.want)
		}
	}
}
