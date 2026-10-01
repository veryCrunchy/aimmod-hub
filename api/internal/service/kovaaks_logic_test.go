package service

import (
	"testing"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/kovaaksbenchmarks"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

func int64p(v int64) *int64 { return &v }

func TestBenchmarkQualityUsesKovaaksSignals(t *testing.T) {
	cases := []struct {
		name     string
		bench    string
		count    int
		synced   bool
		players  *int64
		hidden   bool
		reasonOK bool
	}{
		{"popular benchmark", "Example Season 2", 18, true, int64p(40000), false, true},
		{"not synced yet keeps visible", "Example Season 2", 0, false, nil, false, true},
		{"empty scenario list", "Example Season 2", 0, true, int64p(500), true, true},
		{"long digit run", "Bench 1212311231", 6, true, int64p(500), true, true},
		{"test name", "test", 4, true, nil, true, true},
		{"test prefix", "Test bench please ignore", 4, true, nil, true, true},
		{"repeated characters", "aaaaaaa", 4, true, nil, true, true},
		{"barely played", "Example Season 2", 12, true, int64p(3), true, true},
		{"unknown player count stays visible", "Example Season 2", 12, true, nil, false, true},
		{"digits only", "123", 3, true, nil, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hidden, reason := benchmarkQuality(tc.bench, tc.count, tc.synced, tc.players)
			if hidden != tc.hidden {
				t.Fatalf("hidden = %v (%q), want %v", hidden, reason, tc.hidden)
			}
			if hidden && reason == "" {
				t.Fatal("hidden benchmarks need a reason")
			}
		})
	}
}

func TestMatchScoreOrdersExactPrefixWordSubstring(t *testing.T) {
	exact := matchScore("pasu", "Pasu")
	prefix := matchScore("pasu", "Pasu Voltaic Easy")
	word := matchScore("pasu", "1wall Pasu Reload")
	sub := matchScore("asu", "Pasu")
	words := matchScore("reload pasu", "Pasu Synthetic Reload")
	none := matchScore("tracking", "Pasu")
	if !(exact > prefix && prefix > word && word > sub && sub > words && words > none) {
		t.Fatalf("unexpected order: exact=%v prefix=%v word=%v sub=%v words=%v none=%v", exact, prefix, word, sub, words, none)
	}
	if matchScore("1w4ts", "1w4ts reload") <= 0 || matchScore("", "x") != 0 {
		t.Fatal("edge cases")
	}
	if relevance(0.85, 10_000) <= relevance(0.85, 10) {
		t.Fatal("popularity should break ties")
	}
	if relevance(1, 0) <= relevance(0.85, 1_000_000) {
		t.Fatal("match quality must outweigh popularity")
	}
}

func TestPercentilesAndStanding(t *testing.T) {
	desc := []float64{100, 90, 80, 70, 60, 50, 40, 30, 20, 10}
	got := percentileScores(desc, []uint32{10, 50, 90, 99})
	if got[50] != 50 || got[10] != 10 || got[90] != 90 || got[99] != 100 {
		t.Fatalf("percentiles: %v", got)
	}
	bests := []store.PlayerBest{{UserHandle: "demo-a", Score: 100}, {UserHandle: "demo-b", Score: 80}, {UserHandle: "demo-c", Score: 80}, {UserHandle: "demo-d", Score: 10}}
	standing := viewerStanding(bests, "DEMO-C")
	if standing == nil || standing.Rank != 2 || standing.Percentile != 33.3 {
		t.Fatalf("standing: %+v", standing)
	}
	if viewerStanding(bests, "nobody") != nil {
		t.Fatal("unknown handle should have no standing")
	}
	if beatsShare(0, 1) != 100 {
		t.Fatal("a sole player beats everyone")
	}
}

func TestConsistencyAndTrend(t *testing.T) {
	if consistencyScore([]float64{100, 100, 100}) != 100 {
		t.Fatal("identical runs are perfectly consistent")
	}
	steady := consistencyScore([]float64{100, 98, 102, 101, 99})
	wild := consistencyScore([]float64{100, 40, 160, 20, 140})
	if !(steady > 90 && wild < steady) {
		t.Fatalf("steady=%v wild=%v", steady, wild)
	}
	if consistencyScore([]float64{1, 2}) != 0 {
		t.Fatal("too few runs")
	}
	// newest first: recent average 110 against previous 100
	if got := trendPercent([]float64{110, 110, 110, 110, 110, 100, 100, 100, 100, 100}); got != 10 {
		t.Fatalf("trend %v", got)
	}
	if trendPercent([]float64{1, 2, 3}) != 0 {
		t.Fatal("too few runs")
	}
}

func TestBenchmarkRankHistoryReplaysDays(t *testing.T) {
	scenarios := []historyScenario{
		{Name: "Synthetic Track", RankMaxes: []float64{100, 200, 300}},
		{Name: "Synthetic Click", RankMaxes: []float64{10, 20, 30}},
	}
	day := func(d int) time.Time { return time.Date(2026, 1, d, 12, 0, 0, 0, time.UTC) }
	runs := []historyRun{
		{ScenarioName: "synthetic track", Score: 150, PlayedAt: day(1)},
		{ScenarioName: "Synthetic Click", Score: 25, PlayedAt: day(2)},
		{ScenarioName: "Synthetic Click", Score: 5, PlayedAt: day(3)},   // no change, no point
		{ScenarioName: "Synthetic Track", Score: 310, PlayedAt: day(4)}, // track rank 3
		{ScenarioName: "Unrelated", Score: 9999, PlayedAt: day(5)},      // ignored
		{ScenarioName: "Synthetic Click", Score: 35, PlayedAt: day(6)},  // both rank 3
	}
	points := benchmarkRankHistory(scenarios, runs)
	if len(points) != 4 {
		t.Fatalf("points: %+v", points)
	}
	if points[0].OverallRankIndex != 0 || points[0].RankedScenarios != 1 {
		t.Fatalf("day 1: %+v", points[0])
	}
	if points[1].OverallRankIndex != 1 || points[1].AverageRankIndex != 1.5 {
		t.Fatalf("day 2: %+v", points[1])
	}
	if points[3].OverallRankIndex != 3 || points[3].Day != day(6).Truncate(24*time.Hour) {
		t.Fatalf("day 6: %+v", points[3])
	}
}

func TestRankMaxesFromThresholdsFillsGaps(t *testing.T) {
	got := rankMaxesFromThresholds([]kovaaksbenchmarks.BenchmarkThreshold{{RankIndex: 1, Score: 10}, {RankIndex: 3, Score: 30}})
	want := []float64{10, 30, 30}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if scenarioRankIndex(29, got) != 1 || scenarioRankIndex(30, got) != 3 {
		t.Fatal("rank from rebuilt thresholds")
	}
}

func TestCompareBestsCountsSharedScenarios(t *testing.T) {
	left := &store.ComparePlayerRecord{UserHandle: "demo-a", Bests: map[string]store.PlayerScenarioBest{
		"Synthetic A": {BestScore: 10, RunCount: 4}, "Synthetic B": {BestScore: 5, RunCount: 1}, "Only Left": {BestScore: 1},
	}}
	right := &store.ComparePlayerRecord{UserHandle: "demo-b", Bests: map[string]store.PlayerScenarioBest{
		"Synthetic A": {BestScore: 8, RunCount: 2}, "Synthetic B": {BestScore: 5, RunCount: 9}, "Only Right": {BestScore: 1},
	}}
	resp := compareBests(left, right)
	if len(resp.Shared) != 2 || resp.Wins != 1 || resp.Ties != 1 || resp.Losses != 0 {
		t.Fatalf("compare: %+v", resp)
	}
	if resp.Shared[0].ScenarioName != "Synthetic B" {
		t.Fatal("most played shared scenario first")
	}
}

func TestSortBenchmarkListPutsVisibleAndPopularFirst(t *testing.T) {
	items := []*hubv1.BenchmarkListItem{
		{BenchmarkName: "Hidden", Hidden: true, KovaaksPlayers: 1_000_000},
		{BenchmarkName: "Small", KovaaksPlayers: 50},
		{BenchmarkName: "Hub favourite", PlayerCount: 3},
		{BenchmarkName: "Big", KovaaksPlayers: 90_000},
	}
	sortBenchmarkList(items)
	order := []string{items[0].BenchmarkName, items[1].BenchmarkName, items[2].BenchmarkName, items[3].BenchmarkName}
	want := []string{"Hub favourite", "Big", "Small", "Hidden"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order %v want %v", order, want)
		}
	}
}

func TestSortBenchmarkLeaderboardTiesOnProgress(t *testing.T) {
	entries := []*hubv1.BenchmarkLeaderboardEntry{
		{UserHandle: "demo-c", OverallRankIndex: 3, BenchmarkProgress: 100},
		{UserHandle: "demo-a", OverallRankIndex: 5, BenchmarkProgress: 10},
		{UserHandle: "demo-b", OverallRankIndex: 3, BenchmarkProgress: 900},
	}
	sortBenchmarkLeaderboard(entries)
	if entries[0].UserHandle != "demo-a" || entries[1].UserHandle != "demo-b" {
		t.Fatalf("order: %s %s %s", entries[0].UserHandle, entries[1].UserHandle, entries[2].UserHandle)
	}
}

func TestRangeSince(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	since, name := rangeSince("30d", now)
	if since == nil || name != "30d" || !since.Equal(now.AddDate(0, 0, -30)) {
		t.Fatalf("30d: %v %s", since, name)
	}
	if since, name := rangeSince("bogus", now); since != nil || name != "all" {
		t.Fatal("unknown ranges mean all time")
	}
}

func TestMergeSearchResultsDropsDuplicateScenarios(t *testing.T) {
	local := []*hubv1.QuickSearchResult{{Kind: "scenario", ScenarioSlug: "synthetic-a"}}
	external := []*hubv1.QuickSearchResult{{Kind: "kovaaks_scenario", ScenarioSlug: "synthetic-a"}, {Kind: "kovaaks_scenario", ScenarioSlug: "synthetic-b"}}
	if got := mergeSearchResults(local, external); len(got) != 2 {
		t.Fatalf("merged %d", len(got))
	}
}

func TestLooksLikeSteam64(t *testing.T) {
	if !looksLikeSteam64(kovaaksbenchmarks.PlaceholderSteamID) || looksLikeSteam64("1234") || looksLikeSteam64("7656119x000000000") {
		t.Fatal("steam64 detection")
	}
}

func TestCapSearchResultsReservesKovaaksPlaces(t *testing.T) {
	var results []*hubv1.QuickSearchResult
	for i := 0; i < 30; i++ {
		results = append(results, &hubv1.QuickSearchResult{Kind: "scenario", Relevance: float64(100 - i)})
	}
	for i := 0; i < 10; i++ {
		results = append(results, &hubv1.QuickSearchResult{Kind: "kovaaks_player", Relevance: float64(10 - i)})
	}
	got := capSearchResults(results, 20, 6)
	external := 0
	for _, r := range got {
		if r.Kind == "kovaaks_player" {
			external++
		}
	}
	if len(got) != 20 || external != 6 || got[0].Relevance != 100 {
		t.Fatalf("got %d results, %d external", len(got), external)
	}
	if few := capSearchResults(results[:3], 20, 6); len(few) != 3 {
		t.Fatal("short lists are kept whole")
	}
}
