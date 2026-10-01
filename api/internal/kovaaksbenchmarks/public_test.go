package kovaaksbenchmarks

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const syntheticDetail = `{
  "benchmark_progress": 640,
  "overall_rank": 1,
  "categories": {
    "Zeta Clicking": {"benchmark_progress": 300, "category_rank": 1, "scenarios": {
      "Synthetic Zulu": {"score": 25000, "leaderboard_rank": 12, "scenario_rank": 2, "rank_maxes": [100, 200, 300], "leaderboard_id": 11},
      "Synthetic Alpha": {"score": 15000, "leaderboard_rank": null, "scenario_rank": 1, "rank_maxes": [100, 200, 300], "leaderboard_id": 12}
    }},
    "Alpha Tracking": {"benchmark_progress": 340, "category_rank": 0, "scenarios": {
      "Synthetic Track": {"score": 0, "leaderboard_rank": null, "scenario_rank": 0, "rank_maxes": [1000, 2000, 3000], "leaderboard_id": 13}
    }}
  },
  "ranks": [
    {"name": "No Rank", "icon": "", "color": "", "frame": ""},
    {"name": "Bronze", "icon": "b.svg", "color": "#a0522d", "frame": ""},
    {"name": "Silver", "icon": "s.svg", "color": "#c0c0c0", "frame": ""},
    {"name": "Gold", "icon": "g.svg", "color": "#ffd700", "frame": ""}
  ]
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewClient()
	client.apiBaseURL = server.URL
	client.baseURL = server.URL + "/benchmarks"
	client.steamBaseURL = server.URL + "/steam"
	return client, server
}

func TestBenchmarkDetailKeepsAuthorOrderAndProgress(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(syntheticDetail))
	})
	detail, err := client.GetBenchmarkDetail(context.Background(), 7, "76561190000000001")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(detail.CategoryOrder, ",") != "Zeta Clicking,Alpha Tracking" {
		t.Fatalf("category order: %v", detail.CategoryOrder)
	}
	if got := strings.Join(detail.Categories["Zeta Clicking"].ScenarioOrder, ","); got != "Synthetic Zulu,Synthetic Alpha" {
		t.Fatalf("scenario order: %s", got)
	}
	if detail.BenchmarkProgress != 640 || detail.Categories["Alpha Tracking"].BenchmarkProgress != 340 {
		t.Fatal("benchmark progress not read")
	}
	pages := BuildCategoryPages(detail, PageOptions{LocalScores: map[string]float64{"Synthetic Track": 2100}, ComputeRanks: true})
	if len(pages) != 2 || pages[0].CategoryName != "Zeta Clicking" {
		t.Fatalf("pages: %+v", pages)
	}
	track := pages[1].Scenarios[0]
	if track.ScoreSource != "aimmod" || track.ScenarioRank.RankName != "Silver" || track.Score != 2100 {
		t.Fatalf("local score should win and compute rank: %+v", track)
	}
	if zulu := pages[0].Scenarios[0]; zulu.ScoreSource != "kovaaks" || zulu.Score != 250 || len(zulu.Thresholds) != 3 {
		t.Fatalf("kovaaks score scaled: %+v", zulu)
	}
	ranked := BuildCategoryPages(detail, PageOptions{RankedOnly: true})
	if len(ranked) != 1 {
		t.Fatal("ranked-only drops unranked categories")
	}
	overall := OverallRankFromCategories(pages, detail.Ranks)
	if overall.RankName != "Bronze" {
		t.Fatalf("weakest link: %+v", overall)
	}
}

func TestScenarioLeaderboardKeepsPublicFieldsOnly(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/leaderboard/scores/global" || r.URL.Query().Get("leaderboardId") != "11" || r.URL.Query().Get("max") != "100" {
			t.Errorf("unexpected request %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"total": 3, "page": 0, "max": 100, "data": [
		  {"steamId": "76561190000000001", "score": 812.5, "rank": 1, "steamAccountName": "Synthetic One", "webappUsername": "synthetic-one",
		   "country": "NL", "attributes": {"cm360": 34.5, "epoch": 1767225600000, "resolution": "1920x1080", "hash": "x"}}]}`))
	})
	board, err := client.GetScenarioLeaderboard(context.Background(), 11, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	if board.Total != 3 || board.PageSize != 100 || len(board.Entries) != 1 {
		t.Fatalf("board: %+v", board)
	}
	entry := board.Entries[0]
	if entry.Country != "nl" || entry.CM360 != 34.5 || entry.KovaaksUsername != "synthetic-one" || entry.PlayedAt.Year() != 2026 {
		t.Fatalf("entry: %+v", entry)
	}
}

func TestScenarioSearchAndUserEndpoints(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/scenario/popular":
			_, _ = w.Write([]byte(`{"total": 2, "data": [
			  {"leaderboardId": 41, "scenarioName": "Synthetic Grid Reload", "scenario": {"aimType": "Clicking", "authors": ["demo"]}, "counts": {"plays": 900, "entries": 120}, "topScore": {"score": 99}},
			  {"leaderboardId": 42, "scenarioName": "Synthetic Grid", "scenario": {"aimType": null, "authors": []}, "counts": {"plays": 5000, "entries": 800}, "topScore": {"score": 77}}]}`))
		case "/user/scenario/total-play":
			_, _ = w.Write([]byte(`{"total": 1, "data": [{"leaderboardId": "42", "scenarioName": "Synthetic Grid", "counts": {"plays": 31}, "rank": 7, "score": 70, "attributes": {"cm360": 40, "epoch": 0}}]}`))
		case "/user/profile/by-username":
			_, _ = w.Write([]byte(`{"steamId": "76561190000000002", "steamAccountName": "Synthetic Two", "steamAccountAvatar": "a.png", "country": "DE",
			  "scenariosPlayed": "321", "created": "2024-01-02T03:04:05.000Z", "webapp": {"username": "synthetic-two", "gamingPeripherals": {"mouse": "x"}}}`))
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	found, err := client.FindScenario(ctx, "synthetic grid")
	if err != nil || found == nil || found.LeaderboardID != 42 || found.Entries != 800 {
		t.Fatalf("find: %+v %v", found, err)
	}
	scenarios, err := client.ListUserScenarios(ctx, "synthetic-two", 0, 20)
	if err != nil || len(scenarios.Items) != 1 || scenarios.Items[0].LeaderboardID != 42 || !scenarios.Items[0].PlayedAt.IsZero() {
		t.Fatalf("user scenarios: %+v %v", scenarios, err)
	}
	profile, err := client.GetUserProfile(ctx, "synthetic-two")
	if err != nil || profile.SteamID != "76561190000000002" || profile.ScenariosPlayed != 321 || profile.Country != "de" || profile.CreatedAt.Year() != 2024 {
		t.Fatalf("profile: %+v %v", profile, err)
	}
	resolved, _ := client.ResolveSteamInput(ctx, "synthetic-two")
	_ = resolved // the profile lookup seeds the resolve cache by Steam id
	if cached, ok := client.resolveCache["76561190000000002"]; !ok || cached.identity.KovaaksUsername != "synthetic-two" {
		t.Fatal("profile lookup should seed the identity cache")
	}
}

func TestFetchServesFreshCacheAndStaleOnError(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"total": 1, "data": []}`))
	})
	ctx := context.Background()
	if _, err := client.SearchScenarios(ctx, "x", 0, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SearchScenarios(ctx, "x", 0, 5); err != nil || calls.Load() != 1 {
		t.Fatalf("fresh cache should skip the network: calls=%d err=%v", calls.Load(), err)
	}
	// Leaderboards use a TTL; with no TTL (benchmark detail) we refetch and,
	// on failure, fall back to the stale copy.
	if _, err := client.GetBenchmarkDetail(ctx, 1, "76561190000000003"); err != nil {
		t.Fatal(err)
	}
	client.detailCache = map[string]cachedBenchmarkDetail{}
	fail.Store(true)
	if _, err := client.GetBenchmarkDetail(ctx, 1, "76561190000000003"); err != nil {
		t.Fatalf("stale copy should be served on provider errors: %v", err)
	}
}

func TestRateLimitBackoffFailsFast(t *testing.T) {
	var calls atomic.Int32
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	ctx := context.Background()
	if _, err := client.SearchScenarios(ctx, "a", 0, 5); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("want rate limited, got %v", err)
	}
	if _, err := client.SearchScenarios(ctx, "b", 0, 5); !errors.Is(err, ErrRateLimited) || calls.Load() != 1 {
		t.Fatalf("backoff should stop further requests: calls=%d err=%v", calls.Load(), err)
	}
}

func TestTokenBucketWaitsForTokens(t *testing.T) {
	bucket := newTokenBucket(2, 1)
	now := time.Unix(0, 0)
	bucket.now = func() time.Time { return now }
	var slept time.Duration
	bucket.sleepFor = func(_ context.Context, d time.Duration) error {
		slept += d
		now = now.Add(d)
		return nil
	}
	for i := 0; i < 3; i++ {
		if err := bucket.wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if slept < 900*time.Millisecond || slept > 1100*time.Millisecond {
		t.Fatalf("two extra tokens at 2/s should wait ~1s, waited %v", slept)
	}
}

func TestGetBenchmarkDefinitionUsesPlaceholderAccount(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("steamId") != PlaceholderSteamID {
			t.Errorf("definition reads must not use a player's id: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(syntheticDetail))
	})
	detail, err := client.GetBenchmarkDefinition(context.Background(), 9)
	if err != nil || len(detail.CategoryOrder) != 2 {
		t.Fatalf("definition: %+v %v", detail, err)
	}
}
