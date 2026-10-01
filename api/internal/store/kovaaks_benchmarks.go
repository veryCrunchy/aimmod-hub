package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Public KovaaK's benchmark definitions, kept so pages can show thresholds,
// scenario lists and catalog quality without asking KovaaK's on every view.
// Nothing here is player data.
const kovaaksBenchmarkSchemaSQL = `
CREATE TABLE IF NOT EXISTS kovaaks_benchmarks (
  benchmark_id BIGINT PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  icon_url TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  benchmark_type TEXT NOT NULL DEFAULT '',
  ranks_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  scenario_count INT NOT NULL DEFAULT 0,
  player_estimate BIGINT,
  definition_synced_at TIMESTAMPTZ,
  definition_error TEXT NOT NULL DEFAULT '',
  listed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS kovaaks_benchmark_scenarios (
  benchmark_id BIGINT NOT NULL REFERENCES kovaaks_benchmarks(benchmark_id) ON DELETE CASCADE,
  position INT NOT NULL,
  category_name TEXT NOT NULL,
  scenario_name TEXT NOT NULL,
  leaderboard_id BIGINT NOT NULL DEFAULT 0,
  rank_maxes JSONB NOT NULL DEFAULT '[]'::jsonb,
  PRIMARY KEY (benchmark_id, position)
);

CREATE INDEX IF NOT EXISTS idx_kovaaks_benchmark_scenarios_name
  ON kovaaks_benchmark_scenarios (LOWER(scenario_name));
CREATE INDEX IF NOT EXISTS idx_kovaaks_benchmarks_synced
  ON kovaaks_benchmarks (definition_synced_at NULLS FIRST);
`

type KovaaksBenchmarkMeta struct {
	BenchmarkID   uint32
	Name          string
	IconURL       string
	Author        string
	BenchmarkType string
}

type KovaaksBenchmarkRank struct {
	RankIndex uint32 `json:"rankIndex"`
	RankName  string `json:"rankName"`
	IconURL   string `json:"iconUrl"`
	Color     string `json:"color"`
	FrameURL  string `json:"frameUrl"`
}

type KovaaksBenchmarkScenario struct {
	Position      int
	CategoryName  string
	ScenarioName  string
	LeaderboardID uint32
	RankMaxes     []float64
}

type KovaaksBenchmarkRecord struct {
	KovaaksBenchmarkMeta
	ScenarioCount  int
	PlayerEstimate *int64
	SyncedAt       *time.Time
	Ranks          []KovaaksBenchmarkRank
	Scenarios      []KovaaksBenchmarkScenario
}

// UpsertKovaaksBenchmarkCatalog stores public benchmark metadata. Definition
// columns are left alone so a catalog refresh never erases synced data.
func (s *Store) UpsertKovaaksBenchmarkCatalog(ctx context.Context, items []KovaaksBenchmarkMeta) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin benchmark catalog upsert: %w", err)
	}
	defer tx.Rollback(ctx)
	for _, item := range items {
		if item.BenchmarkID == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO kovaaks_benchmarks (benchmark_id, name, icon_url, author, benchmark_type, listed_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
			ON CONFLICT (benchmark_id) DO UPDATE SET
				name = EXCLUDED.name,
				icon_url = EXCLUDED.icon_url,
				author = EXCLUDED.author,
				benchmark_type = EXCLUDED.benchmark_type,
				listed_at = NOW()
		`, int64(item.BenchmarkID), strings.TrimSpace(item.Name), strings.TrimSpace(item.IconURL),
			strings.TrimSpace(item.Author), strings.TrimSpace(item.BenchmarkType)); err != nil {
			return fmt.Errorf("upsert benchmark %d: %w", item.BenchmarkID, err)
		}
	}
	return tx.Commit(ctx)
}

// SaveKovaaksBenchmarkDefinition replaces a benchmark's scenario list and
// rank ladder. playerEstimate may be nil when it could not be read.
func (s *Store) SaveKovaaksBenchmarkDefinition(
	ctx context.Context,
	benchmarkID uint32,
	ranks []KovaaksBenchmarkRank,
	scenarios []KovaaksBenchmarkScenario,
	playerEstimate *int64,
) error {
	ranksJSON, err := json.Marshal(ranks)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin benchmark definition save: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO kovaaks_benchmarks (benchmark_id, ranks_json, scenario_count, player_estimate, definition_synced_at, definition_error)
		VALUES ($1, $2, $3, $4, NOW(), '')
		ON CONFLICT (benchmark_id) DO UPDATE SET
			ranks_json = EXCLUDED.ranks_json,
			scenario_count = EXCLUDED.scenario_count,
			player_estimate = COALESCE(EXCLUDED.player_estimate, kovaaks_benchmarks.player_estimate),
			definition_synced_at = NOW(),
			definition_error = ''
	`, int64(benchmarkID), string(ranksJSON), len(scenarios), playerEstimate); err != nil {
		return fmt.Errorf("save benchmark definition: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM kovaaks_benchmark_scenarios WHERE benchmark_id = $1`, int64(benchmarkID)); err != nil {
		return fmt.Errorf("clear benchmark scenarios: %w", err)
	}
	for _, scenario := range scenarios {
		maxes, err := json.Marshal(scenario.RankMaxes)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO kovaaks_benchmark_scenarios (benchmark_id, position, category_name, scenario_name, leaderboard_id, rank_maxes)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, int64(benchmarkID), scenario.Position, scenario.CategoryName, scenario.ScenarioName, int64(scenario.LeaderboardID), string(maxes)); err != nil {
			return fmt.Errorf("save benchmark scenario: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// MarkKovaaksBenchmarkSyncFailed records a failed definition read so the
// syncer moves on and retries later.
func (s *Store) MarkKovaaksBenchmarkSyncFailed(ctx context.Context, benchmarkID uint32, reason string) error {
	if len(reason) > 200 {
		reason = reason[:200]
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE kovaaks_benchmarks
		SET definition_synced_at = NOW(), definition_error = $2
		WHERE benchmark_id = $1
	`, int64(benchmarkID), reason)
	return err
}

// ListKovaaksBenchmarksNeedingSync returns benchmark ids whose definition is
// missing or older than maxAge, oldest first.
func (s *Store) ListKovaaksBenchmarksNeedingSync(ctx context.Context, maxAge time.Duration, limit int) ([]uint32, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT benchmark_id FROM kovaaks_benchmarks
		WHERE definition_synced_at IS NULL
		   OR definition_synced_at < NOW() - make_interval(secs => $1)
		ORDER BY definition_synced_at NULLS FIRST, benchmark_id
		LIMIT $2
	`, maxAge.Seconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("list benchmarks needing sync: %w", err)
	}
	defer rows.Close()
	var ids []uint32
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, uint32(id))
	}
	return ids, rows.Err()
}

// ListKovaaksBenchmarkCatalog returns every stored benchmark with its
// quality signals but without scenario rows.
func (s *Store) ListKovaaksBenchmarkCatalog(ctx context.Context) ([]KovaaksBenchmarkRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT benchmark_id, name, icon_url, author, benchmark_type, scenario_count, player_estimate, definition_synced_at
		FROM kovaaks_benchmarks
		WHERE definition_error = '' OR definition_synced_at IS NULL OR scenario_count > 0
		ORDER BY benchmark_id
	`)
	if err != nil {
		return nil, fmt.Errorf("list benchmark catalog: %w", err)
	}
	defer rows.Close()
	var out []KovaaksBenchmarkRecord
	for rows.Next() {
		var record KovaaksBenchmarkRecord
		var id int64
		if err := rows.Scan(&id, &record.Name, &record.IconURL, &record.Author, &record.BenchmarkType,
			&record.ScenarioCount, &record.PlayerEstimate, &record.SyncedAt); err != nil {
			return nil, err
		}
		record.BenchmarkID = uint32(id)
		out = append(out, record)
	}
	return out, rows.Err()
}

// GetKovaaksBenchmark returns one benchmark with its ordered scenarios and
// rank ladder, or nil when it is not stored.
func (s *Store) GetKovaaksBenchmark(ctx context.Context, benchmarkID uint32) (*KovaaksBenchmarkRecord, error) {
	var record KovaaksBenchmarkRecord
	var id int64
	var ranksJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT benchmark_id, name, icon_url, author, benchmark_type, scenario_count, player_estimate, definition_synced_at, ranks_json
		FROM kovaaks_benchmarks WHERE benchmark_id = $1
	`, int64(benchmarkID)).Scan(&id, &record.Name, &record.IconURL, &record.Author, &record.BenchmarkType,
		&record.ScenarioCount, &record.PlayerEstimate, &record.SyncedAt, &ranksJSON)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, nil
		}
		return nil, fmt.Errorf("get benchmark: %w", err)
	}
	record.BenchmarkID = uint32(id)
	_ = json.Unmarshal(ranksJSON, &record.Ranks)
	rows, err := s.pool.Query(ctx, `
		SELECT position, category_name, scenario_name, leaderboard_id, rank_maxes
		FROM kovaaks_benchmark_scenarios WHERE benchmark_id = $1 ORDER BY position
	`, int64(benchmarkID))
	if err != nil {
		return nil, fmt.Errorf("get benchmark scenarios: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var scenario KovaaksBenchmarkScenario
		var leaderboardID int64
		var maxes []byte
		if err := rows.Scan(&scenario.Position, &scenario.CategoryName, &scenario.ScenarioName, &leaderboardID, &maxes); err != nil {
			return nil, err
		}
		scenario.LeaderboardID = uint32(leaderboardID)
		_ = json.Unmarshal(maxes, &scenario.RankMaxes)
		record.Scenarios = append(record.Scenarios, scenario)
	}
	return &record, rows.Err()
}

// BenchmarkScenarioMembership says a scenario is part of a benchmark.
type BenchmarkScenarioMembership struct {
	BenchmarkID   uint32
	BenchmarkName string
	IconURL       string
	CategoryName  string
	ScenarioName  string
	LeaderboardID uint32
	RankMaxes     []float64
	Ranks         []KovaaksBenchmarkRank
}

// ListBenchmarksContainingScenarios returns, for each named scenario, the
// stored benchmarks that include it (matched case-insensitively).
func (s *Store) ListBenchmarksContainingScenarios(ctx context.Context, scenarioNames []string) ([]BenchmarkScenarioMembership, error) {
	if len(scenarioNames) == 0 {
		return nil, nil
	}
	lowered := make([]string, 0, len(scenarioNames))
	for _, name := range scenarioNames {
		if trimmed := strings.ToLower(strings.TrimSpace(name)); trimmed != "" {
			lowered = append(lowered, trimmed)
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT kb.benchmark_id, kb.name, kb.icon_url, kbs.category_name, kbs.scenario_name, kbs.leaderboard_id, kbs.rank_maxes, kb.ranks_json
		FROM kovaaks_benchmark_scenarios kbs
		JOIN kovaaks_benchmarks kb ON kb.benchmark_id = kbs.benchmark_id
		WHERE LOWER(kbs.scenario_name) = ANY($1)
		ORDER BY kb.player_estimate DESC NULLS LAST, kb.benchmark_id, kbs.position
	`, lowered)
	if err != nil {
		return nil, fmt.Errorf("list benchmarks containing scenarios: %w", err)
	}
	defer rows.Close()
	var out []BenchmarkScenarioMembership
	for rows.Next() {
		var m BenchmarkScenarioMembership
		var id, leaderboardID int64
		var maxes, ranks []byte
		if err := rows.Scan(&id, &m.BenchmarkName, &m.IconURL, &m.CategoryName, &m.ScenarioName, &leaderboardID, &maxes, &ranks); err != nil {
			return nil, err
		}
		m.BenchmarkID = uint32(id)
		m.LeaderboardID = uint32(leaderboardID)
		_ = json.Unmarshal(maxes, &m.RankMaxes)
		_ = json.Unmarshal(ranks, &m.Ranks)
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListBenchmarksByLeaderboardID returns stored benchmarks that include the
// given KovaaK's leaderboard.
func (s *Store) ListBenchmarksByLeaderboardID(ctx context.Context, leaderboardID uint32) ([]BenchmarkScenarioMembership, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT kb.benchmark_id, kb.name, kbs.category_name, kbs.scenario_name
		FROM kovaaks_benchmark_scenarios kbs
		JOIN kovaaks_benchmarks kb ON kb.benchmark_id = kbs.benchmark_id
		WHERE kbs.leaderboard_id = $1
		ORDER BY kb.player_estimate DESC NULLS LAST
		LIMIT 5
	`, int64(leaderboardID))
	if err != nil {
		return nil, fmt.Errorf("list benchmarks by leaderboard: %w", err)
	}
	defer rows.Close()
	var out []BenchmarkScenarioMembership
	for rows.Next() {
		var m BenchmarkScenarioMembership
		var id int64
		if err := rows.Scan(&id, &m.BenchmarkName, &m.CategoryName, &m.ScenarioName); err != nil {
			return nil, err
		}
		m.BenchmarkID = uint32(id)
		m.LeaderboardID = leaderboardID
		out = append(out, m)
	}
	return out, rows.Err()
}
