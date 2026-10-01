package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs against an explicitly supplied test database in its own schema.
func newIntegrationStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("AIMMOD_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("AIMMOD_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("kovaaks_stats_test_%d", time.Now().UnixNano())
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		pool.Close()
	})
	if _, err = pool.Exec(ctx, "SET search_path TO "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	s := &Store{pool: pool}
	if err = s.ensureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	return s
}

func saveSyntheticRun(t *testing.T, s *Store, player, steamID, scenario string, score float64, playedAt time.Time, n int) {
	t.Helper()
	run := IngestedRun{
		AppVersion: "test", SchemaVersion: 1,
		UserExternalID: "synthetic:" + player, UserDisplayName: "Synthetic " + player,
		SteamID: steamID, SteamDisplayName: steamDisplay(steamID, player),
		SessionID:    fmt.Sprintf("%s-%s-%d", player, scenario, n),
		ScenarioName: scenario, ScenarioType: "Tracking",
		Score: score, Accuracy: 50 + float64(n%10), DurationMS: 60000, PlayedAt: playedAt,
		SummaryJSON: []byte(`{}`), FeatureJSON: []byte(`{}`),
	}
	if err := s.SaveIngestedRun(context.Background(), run, nil); err != nil {
		t.Fatal(err)
	}
}

func steamDisplay(steamID, player string) string {
	if steamID == "" {
		return ""
	}
	return "Synthetic " + player
}

func TestKovaaksStatsQueries(t *testing.T) {
	s := newIntegrationStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -60)

	saveSyntheticRun(t, s, "alpha", "76561190000000011", "Synthetic Track", 900, old, 1)
	saveSyntheticRun(t, s, "alpha", "76561190000000011", "Synthetic Track", 700, now.Add(-time.Hour), 2)
	saveSyntheticRun(t, s, "bravo", "", "Synthetic Track", 800, now.Add(-2*time.Hour), 1)
	saveSyntheticRun(t, s, "bravo", "", "Synthetic Track", 820, now.Add(-time.Hour), 2)
	saveSyntheticRun(t, s, "charlie", "76561190000000013", "Synthetic Track", 600, now.Add(-3*time.Hour), 1)
	saveSyntheticRun(t, s, "charlie", "76561190000000013", "Synthetic Click", 50, now.Add(-3*time.Hour), 2)
	saveSyntheticRun(t, s, "alpha", "76561190000000011", "Synthetic Click", 40, now.Add(-3*time.Hour), 3)

	var handles []string
	rows, err := s.pool.Query(ctx, `SELECT user_handle FROM hub_user_identity ORDER BY user_handle`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var h string
		_ = rows.Scan(&h)
		handles = append(handles, h)
	}
	rows.Close()
	if len(handles) != 3 {
		t.Fatalf("handles: %v", handles)
	}
	alpha := handleFor(t, s, "synthetic:alpha")

	all, err := s.ScenarioLeaderboard(ctx, ScenarioLeaderboardQuery{ScenarioName: "Synthetic Track", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 3 || len(all.Rows) != 2 || all.Rows[0].Score != 900 || all.Rows[0].Rank != 1 || all.Rows[0].RunCount != 2 || !all.Rows[0].IsLinked {
		t.Fatalf("all time board: %+v", all)
	}

	since := now.AddDate(0, 0, -7)
	recent, err := s.ScenarioLeaderboard(ctx, ScenarioLeaderboardQuery{ScenarioName: "Synthetic Track", Since: &since, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if recent.Rows[0].Score != 820 || recent.Rows[1].Score != 700 {
		t.Fatalf("7 day board should drop the old best: %+v", recent.Rows)
	}

	linked, err := s.ScenarioLeaderboard(ctx, ScenarioLeaderboardQuery{ScenarioName: "Synthetic Track", LinkedOnly: true, Limit: 10})
	if err != nil || linked.Total != 2 {
		t.Fatalf("linked only: %+v %v", linked, err)
	}

	jump, err := s.ScenarioLeaderboard(ctx, ScenarioLeaderboardQuery{ScenarioName: "Synthetic Track", Limit: 1, AroundHandle: handleFor(t, s, "synthetic:charlie")})
	if err != nil || jump.Offset != 2 || jump.HighlightRank != 3 || jump.Rows[0].Score != 600 {
		t.Fatalf("jump to player: %+v %v", jump, err)
	}

	filtered, err := s.ScenarioLeaderboard(ctx, ScenarioLeaderboardQuery{ScenarioName: "Synthetic Track", Query: "bra%", Limit: 10})
	if err != nil || filtered.Total != 0 {
		t.Fatalf("LIKE wildcards must be literal: %+v %v", filtered, err)
	}

	bests, err := s.ScenarioPlayerBests(ctx, "Synthetic Track")
	if err != nil || len(bests) != 3 || bests[0].Score != 900 {
		t.Fatalf("bests: %+v %v", bests, err)
	}

	resolved, stats, err := s.PlayerScenarioStats(ctx, alpha, nil)
	if err != nil || resolved != alpha || len(stats) != 2 {
		t.Fatalf("stats: %v %+v %v", resolved, stats, err)
	}
	var track PlayerScenarioStatRecord
	for _, stat := range stats {
		if stat.ScenarioName == "Synthetic Track" {
			track = stat
		}
	}
	if track.BestScore != 900 || track.HubRank != 1 || track.HubPlayers != 3 || track.HubBelow != 2 || len(track.RecentScores) != 2 || track.RecentScores[0] != 700 {
		t.Fatalf("track stat: %+v", track)
	}

	activity, err := s.PlayerActivity(ctx, alpha, now.AddDate(0, 0, -90))
	if err != nil || len(activity) < 2 {
		t.Fatalf("activity: %+v %v", activity, err)
	}

	left, err := s.ComparePlayerBests(ctx, alpha)
	if err != nil || len(left.Bests) != 2 || left.RunCount != 3 {
		t.Fatalf("compare: %+v %v", left, err)
	}

	points, err := s.PlayerRunsForScenarios(ctx, alpha, []string{"synthetic track"})
	if err != nil || len(points) != 2 || !points[0].PlayedAt.Before(points[1].PlayedAt) {
		t.Fatalf("runs for scenarios: %+v %v", points, err)
	}

	index, err := s.SearchIndexSnapshot(ctx, time.Minute)
	if err != nil || len(index.Scenarios) != 2 || len(index.Profiles) != 3 || index.Scenarios[0].ScenarioName != "Synthetic Track" {
		t.Fatalf("search index: %+v %v", index, err)
	}
}

func handleFor(t *testing.T, s *Store, externalID string) string {
	t.Helper()
	var handle string
	if err := s.pool.QueryRow(context.Background(), `SELECT user_handle FROM hub_user_identity WHERE external_id = $1`, externalID).Scan(&handle); err != nil {
		t.Fatal(err)
	}
	return handle
}

func TestKovaaksBenchmarkDefinitions(t *testing.T) {
	s := newIntegrationStore(t)
	ctx := context.Background()
	if err := s.UpsertKovaaksBenchmarkCatalog(ctx, []KovaaksBenchmarkMeta{
		{BenchmarkID: 5, Name: "Synthetic Benchmark", Author: "demo"},
		{BenchmarkID: 6, Name: "Synthetic Empty"},
	}); err != nil {
		t.Fatal(err)
	}
	pending, err := s.ListKovaaksBenchmarksNeedingSync(ctx, time.Hour, 10)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending: %v %v", pending, err)
	}
	players := int64(4321)
	ranks := []KovaaksBenchmarkRank{{RankIndex: 0, RankName: "No Rank"}, {RankIndex: 1, RankName: "Bronze"}}
	scenarios := []KovaaksBenchmarkScenario{
		{Position: 0, CategoryName: "Clicking", ScenarioName: "Synthetic Click", LeaderboardID: 70, RankMaxes: []float64{10}},
		{Position: 1, CategoryName: "Tracking", ScenarioName: "Synthetic Track", LeaderboardID: 71, RankMaxes: []float64{100}},
	}
	if err := s.SaveKovaaksBenchmarkDefinition(ctx, 5, ranks, scenarios, &players); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkKovaaksBenchmarkSyncFailed(ctx, 6, "definition unavailable"); err != nil {
		t.Fatal(err)
	}
	// A catalog refresh must not erase definition data.
	if err := s.UpsertKovaaksBenchmarkCatalog(ctx, []KovaaksBenchmarkMeta{{BenchmarkID: 5, Name: "Synthetic Benchmark S2"}}); err != nil {
		t.Fatal(err)
	}
	record, err := s.GetKovaaksBenchmark(ctx, 5)
	if err != nil || record == nil || record.Name != "Synthetic Benchmark S2" || record.ScenarioCount != 2 || *record.PlayerEstimate != 4321 || len(record.Scenarios) != 2 || len(record.Ranks) != 2 {
		t.Fatalf("record: %+v %v", record, err)
	}
	if record.Scenarios[1].ScenarioName != "Synthetic Track" || record.Scenarios[1].RankMaxes[0] != 100 {
		t.Fatalf("scenario order: %+v", record.Scenarios)
	}
	memberships, err := s.ListBenchmarksContainingScenarios(ctx, []string{"SYNTHETIC TRACK"})
	if err != nil || len(memberships) != 1 || memberships[0].BenchmarkID != 5 || memberships[0].LeaderboardID != 71 {
		t.Fatalf("memberships: %+v %v", memberships, err)
	}
	byBoard, err := s.ListBenchmarksByLeaderboardID(ctx, 70)
	if err != nil || len(byBoard) != 1 {
		t.Fatalf("by leaderboard: %+v %v", byBoard, err)
	}
	catalog, err := s.ListKovaaksBenchmarkCatalog(ctx)
	if err != nil || len(catalog) != 1 || catalog[0].BenchmarkID != 5 {
		t.Fatalf("failed definitions with no scenarios leave the catalog: %+v %v", catalog, err)
	}
	pending, _ = s.ListKovaaksBenchmarksNeedingSync(ctx, time.Hour, 10)
	if len(pending) != 0 {
		t.Fatalf("synced benchmarks are not pending: %v", pending)
	}
	if missing, err := s.GetKovaaksBenchmark(ctx, 999); err != nil || missing != nil {
		t.Fatalf("missing benchmark: %+v %v", missing, err)
	}
}
