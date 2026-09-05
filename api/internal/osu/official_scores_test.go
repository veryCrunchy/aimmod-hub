package osu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

func scoreTestServer(t *testing.T, handler http.HandlerFunc) *Server {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			_, _ = w.Write([]byte(`{"access_token":"token","expires_in":3600}`))
			return
		}
		if r.Header.Get("x-api-version") != "20220705" || r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("incorrect score headers: %v", r.Header)
		}
		handler(w, r)
	}))
	t.Cleanup(upstream.Close)
	s, err := NewServer(Config{OfficialBaseURL: upstream.URL, OfficialClientID: "1", OfficialClientSecret: "secret", ProviderRequestsPerSecond: 10000})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPublicScoresPagesCachedAndDeduplicated(t *testing.T) {
	calls := 0
	s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if q.Get("mode") != "osu" || q.Get("legacy_only") != "0" || q.Get("limit") != "100" {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		if strings.HasSuffix(r.URL.Path, "recent") && q.Get("include_fails") != "1" {
			t.Error("failed plays excluded")
		}
		offset, _ := strconv.Atoi(q.Get("offset"))
		scores := []map[string]any{}
		count := 100
		if offset == 100 {
			count = 1
		}
		for i := 0; i < count; i++ {
			scores = append(scores, map[string]any{"id": i + offset + 1, "user_id": 7, "ruleset_id": 0, "beatmap_id": 9, "ended_at": "2026-09-05T10:00:00Z", "statistics": map[string]int{"great": 100}})
		}
		_ = json.NewEncoder(w).Encode(scores)
	})
	for i := 0; i < 2; i++ {
		result, err := s.GetPublicUserScores(context.Background(), 7, "osu")
		if err != nil || len(result.Scores) != 101 || result.Coverage.Best.Pages != 2 || result.Coverage.Recent.Pages != 2 || result.Coverage.CompleteHistory || result.Coverage.Best.HasMore {
			t.Fatalf("result %+v error %v", result.Coverage, err)
		}
	}
	if calls != 4 {
		t.Fatalf("calls=%d want4 cached category/offset pages", calls)
	}
}

func TestPublicScoresRateLimitPreservesPartialAndStops(t *testing.T) {
	calls := 0
	s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("offset") == "100" {
			w.WriteHeader(429)
			return
		}
		scores := make([]map[string]any, 100)
		for i := range scores {
			scores[i] = map[string]any{"id": i + 1, "user_id": 7, "ruleset_id": 0, "beatmap_id": 9}
		}
		_ = json.NewEncoder(w).Encode(scores)
	})
	result, err := s.GetPublicUserScores(context.Background(), 7, "osu")
	if err != nil || len(result.Scores) != 100 || calls != 2 || result.Coverage.Best.Status != "rate_limited" || result.Coverage.Recent.Status != "rate_limited" {
		t.Fatalf("calls=%d result=%+v err=%v", calls, result.Coverage, err)
	}
}

func TestPublicScoresBoundsAndCancellation(t *testing.T) {
	calls := 0
	s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		scores := make([]map[string]any, 100)
		for i := range scores {
			scores[i] = map[string]any{"id": offset + i + 1, "user_id": 7, "ruleset_id": 0, "beatmap_id": 9}
		}
		_ = json.NewEncoder(w).Encode(scores)
	})
	result, err := s.GetPublicUserScores(context.Background(), 7, "osu")
	if err != nil || calls != 4 || result.Coverage.Best.Status != "page_limit" || !result.Coverage.Best.HasMore {
		t.Fatalf("%+v %v calls%d", result.Coverage, err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.GetPublicUserScores(ctx, 7, "osu"); err != context.Canceled {
		t.Fatalf("cancellation %v", err)
	}
	if calls != 4 {
		t.Error("cancelled request reached transport")
	}
}

func TestOfficialDetailKeepsPPAndDoesNotInventReplayPermission(t *testing.T) {
	s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/scores/42" {
			t.Errorf("path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":42,"user_id":7,"ruleset_id":0,"beatmap_id":9,"pp":321.5,"has_replay":true}`))
	})
	result, err := s.GetPublicScore(context.Background(), 42)
	if err != nil || result.Status != "available" || result.Item == nil || *result.Item.PerformancePoints != 321.5 || !result.Replay.Exists || result.Replay.DownloadAvailable || result.Replay.Status != "permission_unchecked" || result.Replay.DownloadURL != "/api/osu/v1/official-scores/42/replay" {
		t.Fatalf("%+v %v", result, err)
	}
}

func mergeFixture() store.OsuPublicReplay {
	return store.OsuPublicReplay{OsuUserID: 7, BeatmapID: 9, Ruleset: "osu", Visibility: "public", OnlineScoreID: 42,
		BeatmapChecksum: strings.Repeat("a", 32), PlayedAt: time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC), TotalScore: 1000000, MaxCombo: 100, Count300: 100, Accuracy: 1, Passed: true}
}

func TestMergeExactIDRepairsNullPPAndRetainsUploads(t *testing.T) {
	local := mergeFixture()
	local.ShareID = "osu_local"
	local.HasReplayFile = true
	local.Analysis = json.RawMessage(`{"retained":true}`)
	official := mergeFixture()
	pp := 312.5
	official.PerformancePoints = &pp
	items := MergePublicScores([]store.OsuPublicReplay{local}, []OfficialPublicScore{{Replay: official}})
	if len(items) != 1 || items[0].Source != "merged" || items[0].PPSource != "official" || *items[0].PerformancePoints != pp || !items[0].HasReplayFile || string(items[0].Analysis) != string(local.Analysis) || items[0].ShareID != local.ShareID || len(items[0].Uploads) != 1 {
		t.Fatalf("%+v", items)
	}
	if local.PerformancePoints != nil {
		t.Error("mutated original")
	}
}

func TestMergeContentRequiresStrictUnambiguousIdentity(t *testing.T) {
	for _, scenario := range []string{"same", "different_id", "time", "score", "combo", "judgements", "mods", "checksum", "owner", "mode", "ambiguous"} {
		t.Run(scenario, func(t *testing.T) {
			local := mergeFixture()
			local.OnlineScoreID = 0
			official := mergeFixture()
			switch scenario {
			case "different_id":
				local.OnlineScoreID = 43
			case "time":
				local.PlayedAt = local.PlayedAt.Add(time.Second)
			case "score":
				local.TotalScore--
			case "combo":
				local.MaxCombo--
			case "judgements":
				local.CountMiss++
			case "mods":
				local.Mods = []string{"DT"}
			case "checksum":
				local.BeatmapChecksum = ""
			case "owner":
				local.OsuUserID++
			case "mode":
				local.Ruleset = "taiko"
			}
			officials := []OfficialPublicScore{{Replay: official, FallbackEligible: true}}
			if scenario == "ambiguous" {
				other := official
				other.OnlineScoreID++
				officials = append(officials, OfficialPublicScore{Replay: other, FallbackEligible: true})
			}
			items := MergePublicScores([]store.OsuPublicReplay{local}, officials)
			want := len(officials) + 1
			if scenario == "same" {
				want = 1
			}
			if len(items) != want {
				t.Fatalf("got%d want%d", len(items), want)
			}
		})
	}
}

func TestMergeMissingOfficialPPDoesNotClearLocalAndPrivateNeverLeaks(t *testing.T) {
	local := mergeFixture()
	pp := 100.
	local.PerformancePoints = &pp
	official := mergeFixture()
	items := MergePublicScores([]store.OsuPublicReplay{local}, []OfficialPublicScore{{Replay: official}})
	if len(items) != 1 || items[0].PPSource != "local" || *items[0].PerformancePoints != pp {
		t.Fatalf("%+v", items)
	}
	local.Visibility = "private"
	if items = MergePublicScores([]store.OsuPublicReplay{local}, nil); len(items) != 0 {
		t.Fatal("private upload leaked")
	}
}
