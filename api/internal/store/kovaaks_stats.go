package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ResolveScenarioName maps a hub scenario slug to its stored scenario name.
func (s *Store) ResolveScenarioName(ctx context.Context, slug string) (string, error) {
	return s.resolveScenarioNameBySlug(ctx, slug)
}

// ScenarioSlug is the hub slug for a scenario name.
func ScenarioSlug(name string) string {
	return slugifyScenarioName(name)
}

// escapeLike escapes LIKE wildcards so user input is matched literally.
func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

type ScenarioLeaderboardQuery struct {
	ScenarioName string
	Since        *time.Time
	Query        string
	Sort         string // score, accuracy, recent, runs
	LinkedOnly   bool
	Offset       int
	Limit        int
	AroundHandle string
}

type ScenarioLeaderboardRow struct {
	Rank            uint32
	UserHandle      string
	DisplayName     string
	AvatarURL       string
	SteamID         string
	KovaaksUsername string
	Score           float64
	Accuracy        float64
	PlayedAt        time.Time
	RunID           string
	RunCount        uint32
	IsLinked        bool
}

type ScenarioLeaderboardResult struct {
	Rows          []ScenarioLeaderboardRow
	Total         uint32
	Offset        int
	HighlightRank uint32
}

func leaderboardOrder(sort string) string {
	switch sort {
	case "accuracy":
		return "accuracy DESC, score DESC, user_handle"
	case "recent":
		return "last_played DESC, score DESC, user_handle"
	case "runs":
		return "run_count DESC, score DESC, user_handle"
	default:
		return "score DESC, played_at ASC, user_handle"
	}
}

const scenarioLeaderboardCTE = `
WITH best AS (
	SELECT DISTINCT ON (sr.user_id)
		sr.user_id, sr.score, sr.accuracy, sr.played_at,
		COALESCE(sr.public_run_id, sr.session_id) AS run_id
	FROM scenario_runs sr
	WHERE sr.scenario_name = $1 AND ($2::timestamptz IS NULL OR sr.played_at >= $2::timestamptz)
	ORDER BY sr.user_id, sr.score DESC, sr.played_at ASC
), agg AS (
	SELECT user_id, COUNT(*)::int AS run_count, MAX(played_at) AS last_played
	FROM scenario_runs
	WHERE scenario_name = $1 AND ($2::timestamptz IS NULL OR played_at >= $2::timestamptz)
	GROUP BY user_id
), ranked AS (
	SELECT
		b.user_id, b.score, b.accuracy, b.played_at, b.run_id, a.run_count, a.last_played,
		hui.user_handle, hui.user_display_name, hui.avatar_url,
		COALESCE(steam.provider_account_id, '') AS steam_id,
		COALESCE(NULLIF(kv.username, ''), '') AS kovaaks_username,
		(steam.user_id IS NOT NULL OR kv.user_id IS NOT NULL) AS is_linked,
		RANK() OVER (ORDER BY b.score DESC) AS score_rank
	FROM best b
	JOIN agg a ON a.user_id = b.user_id
	JOIN hub_user_identity hui ON hui.user_id = b.user_id
	LEFT JOIN linked_accounts steam ON steam.user_id = b.user_id AND steam.provider = 'steam'
	LEFT JOIN linked_accounts kv ON kv.user_id = b.user_id AND kv.provider = 'kovaaks'
	WHERE (NOT $3::boolean OR steam.user_id IS NOT NULL OR kv.user_id IS NOT NULL)
), filtered AS (
	SELECT * FROM ranked
	WHERE $4::text = ''
	   OR user_handle ILIKE '%' || $4::text || '%'
	   OR user_display_name ILIKE '%' || $4::text || '%'
)
`

// ScenarioLeaderboard ranks AimMod players by their best score on one
// scenario, with optional time range, name filter, linked filter and sorting.
// Rank always reflects score order within the chosen range.
func (s *Store) ScenarioLeaderboard(ctx context.Context, q ScenarioLeaderboardQuery) (ScenarioLeaderboardResult, error) {
	var result ScenarioLeaderboardResult
	if strings.TrimSpace(q.ScenarioName) == "" {
		return result, fmt.Errorf("scenario name is required")
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 50
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	order := leaderboardOrder(q.Sort)
	filter := escapeLike(strings.TrimSpace(q.Query))
	args := []any{q.ScenarioName, q.Since, q.LinkedOnly, filter}

	if handle := strings.TrimSpace(q.AroundHandle); handle != "" {
		var position int
		var scoreRank int
		err := s.pool.QueryRow(ctx, scenarioLeaderboardCTE+`
			SELECT pos, score_rank FROM (
				SELECT user_handle, score_rank, ROW_NUMBER() OVER (ORDER BY `+order+`) AS pos FROM filtered
			) x WHERE LOWER(user_handle) = LOWER($5)
		`, append(args, handle)...).Scan(&position, &scoreRank)
		if err == nil && position > 0 {
			q.Offset = ((position - 1) / q.Limit) * q.Limit
			result.HighlightRank = uint32(scoreRank)
		}
	}

	rows, err := s.pool.Query(ctx, scenarioLeaderboardCTE+`
		SELECT score_rank, user_handle, user_display_name, avatar_url, steam_id, kovaaks_username,
		       score, accuracy, played_at, run_id, run_count, is_linked, COUNT(*) OVER () AS total
		FROM filtered
		ORDER BY `+order+`
		LIMIT $5 OFFSET $6
	`, append(args, q.Limit, q.Offset)...)
	if err != nil {
		return result, fmt.Errorf("scenario leaderboard: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row ScenarioLeaderboardRow
		var rank int64
		var runCount int
		var total int64
		if err := rows.Scan(&rank, &row.UserHandle, &row.DisplayName, &row.AvatarURL, &row.SteamID, &row.KovaaksUsername,
			&row.Score, &row.Accuracy, &row.PlayedAt, &row.RunID, &runCount, &row.IsLinked, &total); err != nil {
			return result, fmt.Errorf("scan scenario leaderboard: %w", err)
		}
		row.Rank = uint32(rank)
		row.RunCount = uint32(runCount)
		result.Total = uint32(total)
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if len(result.Rows) == 0 && q.Offset > 0 {
		// Past the end: report the total so the client can clamp its page.
		_ = s.pool.QueryRow(ctx, scenarioLeaderboardCTE+`SELECT COUNT(*) FROM filtered`, args...).Scan(&result.Total)
	}
	result.Offset = q.Offset
	return result, nil
}

type PlayerBest struct {
	UserHandle string
	Score      float64
	RunCount   uint32
}

// ScenarioPlayerBests returns every AimMod player's best score on a
// scenario, best first.
func (s *Store) ScenarioPlayerBests(ctx context.Context, scenarioName string) ([]PlayerBest, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT hui.user_handle, b.best, b.runs
		FROM (
			SELECT user_id, MAX(score) AS best, COUNT(*)::int AS runs
			FROM scenario_runs WHERE scenario_name = $1
			GROUP BY user_id
		) b
		JOIN hub_user_identity hui ON hui.user_id = b.user_id
		ORDER BY b.best DESC
	`, scenarioName)
	if err != nil {
		return nil, fmt.Errorf("scenario player bests: %w", err)
	}
	defer rows.Close()
	var out []PlayerBest
	for rows.Next() {
		var best PlayerBest
		var runs int
		if err := rows.Scan(&best.UserHandle, &best.Score, &runs); err != nil {
			return nil, err
		}
		best.RunCount = uint32(runs)
		out = append(out, best)
	}
	return out, rows.Err()
}

type PlayerScenarioStatRecord struct {
	ScenarioName    string
	ScenarioType    string
	RunCount        uint32
	BestScore       float64
	AverageScore    float64
	BestAccuracy    float64
	BestPlayedAt    time.Time
	LastPlayedAt    time.Time
	TotalDurationMS uint64
	RecentScores    []float64 // newest first
	HubRank         uint32
	HubPlayers      uint32
	HubBelow        uint32
}

type ActivityDayRecord struct {
	Date       time.Time
	RunCount   uint32
	DurationMS uint64
}

type resolvedHandle struct {
	UserID     int64
	UserHandle string
}

func (s *Store) userIDByHandle(ctx context.Context, handle string) (resolvedHandle, error) {
	var out resolvedHandle
	err := s.pool.QueryRow(ctx, `
		SELECT user_id, user_handle FROM hub_user_identity
		WHERE LOWER(user_handle) = LOWER($1) OR LOWER(external_id) = LOWER($1)
		ORDER BY (LOWER(user_handle) = LOWER($1)) DESC
		LIMIT 1
	`, strings.TrimSpace(handle)).Scan(&out.UserID, &out.UserHandle)
	if err != nil {
		return out, fmt.Errorf("profile not found")
	}
	return out, nil
}

// PlayerScenarioStats summarises every scenario a player has uploaded, with
// AimMod-wide standing computed from all-time bests.
func (s *Store) PlayerScenarioStats(ctx context.Context, handle string, since *time.Time) (string, []PlayerScenarioStatRecord, error) {
	user, err := s.userIDByHandle(ctx, handle)
	if err != nil {
		return "", nil, err
	}
	rows, err := s.pool.Query(ctx, `
		WITH runs AS (
			SELECT scenario_name, COALESCE(NULLIF(scenario_type, ''), 'Unknown') AS scenario_type,
			       score, accuracy, played_at, duration_ms,
			       ROW_NUMBER() OVER (PARTITION BY scenario_name ORDER BY played_at DESC) AS rn
			FROM scenario_runs
			WHERE user_id = $1 AND ($2::timestamptz IS NULL OR played_at >= $2::timestamptz)
		)
		SELECT scenario_name,
		       MODE() WITHIN GROUP (ORDER BY scenario_type),
		       COUNT(*)::int, MAX(score), AVG(score), MAX(accuracy),
		       (ARRAY_AGG(played_at ORDER BY score DESC, played_at ASC))[1],
		       MAX(played_at), COALESCE(SUM(duration_ms), 0)::bigint,
		       ARRAY_AGG(score ORDER BY played_at DESC) FILTER (WHERE rn <= 20)
		FROM runs
		GROUP BY scenario_name
		ORDER BY MAX(played_at) DESC
	`, user.UserID, since)
	if err != nil {
		return "", nil, fmt.Errorf("player scenario stats: %w", err)
	}
	defer rows.Close()
	var out []PlayerScenarioStatRecord
	names := []string{}
	for rows.Next() {
		var rec PlayerScenarioStatRecord
		var runs int
		var duration int64
		if err := rows.Scan(&rec.ScenarioName, &rec.ScenarioType, &runs, &rec.BestScore, &rec.AverageScore, &rec.BestAccuracy,
			&rec.BestPlayedAt, &rec.LastPlayedAt, &duration, &rec.RecentScores); err != nil {
			return "", nil, fmt.Errorf("scan player scenario stat: %w", err)
		}
		rec.RunCount = uint32(runs)
		rec.TotalDurationMS = uint64(max(duration, 0))
		out = append(out, rec)
		names = append(names, rec.ScenarioName)
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if len(names) == 0 {
		return user.UserHandle, out, nil
	}

	standing, err := s.pool.Query(ctx, `
		WITH best AS (
			SELECT scenario_name, user_id, MAX(score) AS best
			FROM scenario_runs WHERE scenario_name = ANY($1)
			GROUP BY scenario_name, user_id
		)
		SELECT b.scenario_name, COUNT(*)::int,
		       COUNT(*) FILTER (WHERE b.best > m.best)::int,
		       COUNT(*) FILTER (WHERE b.best < m.best)::int
		FROM best b
		JOIN best m ON m.scenario_name = b.scenario_name AND m.user_id = $2
		GROUP BY b.scenario_name
	`, names, user.UserID)
	if err != nil {
		return "", nil, fmt.Errorf("player scenario standing: %w", err)
	}
	defer standing.Close()
	byName := map[string]int{}
	for i := range out {
		byName[out[i].ScenarioName] = i
	}
	for standing.Next() {
		var name string
		var players, better, below int
		if err := standing.Scan(&name, &players, &better, &below); err != nil {
			return "", nil, err
		}
		if i, ok := byName[name]; ok {
			out[i].HubPlayers = uint32(players)
			out[i].HubRank = uint32(better + 1)
			out[i].HubBelow = uint32(below)
		}
	}
	return user.UserHandle, out, standing.Err()
}

// PlayerActivity returns runs and time played per UTC day since a date.
func (s *Store) PlayerActivity(ctx context.Context, handle string, since time.Time) ([]ActivityDayRecord, error) {
	user, err := s.userIDByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DATE_TRUNC('day', played_at AT TIME ZONE 'UTC') AS day, COUNT(*)::int, COALESCE(SUM(duration_ms), 0)::bigint
		FROM scenario_runs
		WHERE user_id = $1 AND played_at >= $2
		GROUP BY day ORDER BY day
	`, user.UserID, since)
	if err != nil {
		return nil, fmt.Errorf("player activity: %w", err)
	}
	defer rows.Close()
	var out []ActivityDayRecord
	for rows.Next() {
		var day ActivityDayRecord
		var runs int
		var duration int64
		if err := rows.Scan(&day.Date, &runs, &duration); err != nil {
			return nil, err
		}
		day.RunCount = uint32(runs)
		day.DurationMS = uint64(max(duration, 0))
		out = append(out, day)
	}
	return out, rows.Err()
}

type PlayerScenarioBest struct {
	ScenarioName string
	ScenarioType string
	BestScore    float64
	RunCount     uint32
}

type ComparePlayerRecord struct {
	UserHandle  string
	DisplayName string
	AvatarURL   string
	Bests       map[string]PlayerScenarioBest
	RunCount    uint32
}

// ComparePlayerBests loads one player's best score per scenario.
func (s *Store) ComparePlayerBests(ctx context.Context, handle string) (*ComparePlayerRecord, error) {
	user, err := s.userIDByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	record := &ComparePlayerRecord{Bests: map[string]PlayerScenarioBest{}}
	if err := s.pool.QueryRow(ctx, `
		SELECT user_handle, user_display_name, avatar_url FROM hub_user_identity WHERE user_id = $1
	`, user.UserID).Scan(&record.UserHandle, &record.DisplayName, &record.AvatarURL); err != nil {
		return nil, fmt.Errorf("compare identity: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT scenario_name, MODE() WITHIN GROUP (ORDER BY COALESCE(NULLIF(scenario_type, ''), 'Unknown')),
		       MAX(score), COUNT(*)::int
		FROM scenario_runs WHERE user_id = $1
		GROUP BY scenario_name
	`, user.UserID)
	if err != nil {
		return nil, fmt.Errorf("compare bests: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var best PlayerScenarioBest
		var runs int
		if err := rows.Scan(&best.ScenarioName, &best.ScenarioType, &best.BestScore, &runs); err != nil {
			return nil, err
		}
		best.RunCount = uint32(runs)
		record.RunCount += best.RunCount
		record.Bests[best.ScenarioName] = best
	}
	return record, rows.Err()
}

type RunPoint struct {
	ScenarioName string
	Score        float64
	PlayedAt     time.Time
}

// PlayerRunsForScenarios returns a player's runs on the named scenarios
// (matched case-insensitively), oldest first.
func (s *Store) PlayerRunsForScenarios(ctx context.Context, handle string, names []string) ([]RunPoint, error) {
	if len(names) == 0 {
		return nil, nil
	}
	user, err := s.userIDByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	lowered := make([]string, 0, len(names))
	for _, name := range names {
		lowered = append(lowered, strings.ToLower(strings.TrimSpace(name)))
	}
	rows, err := s.pool.Query(ctx, `
		SELECT scenario_name, score, played_at FROM scenario_runs
		WHERE user_id = $1 AND LOWER(scenario_name) = ANY($2)
		ORDER BY played_at ASC
		LIMIT 20000
	`, user.UserID, lowered)
	if err != nil {
		return nil, fmt.Errorf("player benchmark runs: %w", err)
	}
	defer rows.Close()
	var out []RunPoint
	for rows.Next() {
		var point RunPoint
		if err := rows.Scan(&point.ScenarioName, &point.Score, &point.PlayedAt); err != nil {
			return nil, err
		}
		out = append(out, point)
	}
	return out, rows.Err()
}

// Search index: a short-lived in-memory copy of scenario and profile names so
// the search palette can rank results on every keystroke without table scans.

type SearchScenarioEntry struct {
	ScenarioName string
	ScenarioSlug string
	ScenarioType string
	RunCount     uint32
	PlayerCount  uint32
}

type SearchProfileEntry struct {
	UserHandle      string
	DisplayName     string
	AvatarURL       string
	KovaaksUsername string
	RunCount        uint32
	IsVerified      bool
}

type SearchIndex struct {
	Scenarios []SearchScenarioEntry
	Profiles  []SearchProfileEntry
	BuiltAt   time.Time
}

type searchIndexCache struct {
	mu    sync.Mutex
	index *SearchIndex
}

// SearchIndexSnapshot returns a cached search index no older than ttl.
func (s *Store) SearchIndexSnapshot(ctx context.Context, ttl time.Duration) (*SearchIndex, error) {
	s.searchIndex.mu.Lock()
	defer s.searchIndex.mu.Unlock()
	if s.searchIndex.index != nil && time.Since(s.searchIndex.index.BuiltAt) < ttl {
		return s.searchIndex.index, nil
	}
	index := &SearchIndex{BuiltAt: time.Now()}
	rows, err := s.pool.Query(ctx, `
		SELECT scenario_name,
		       MODE() WITHIN GROUP (ORDER BY COALESCE(NULLIF(scenario_type, ''), 'Unknown')),
		       COUNT(*)::int, COUNT(DISTINCT user_id)::int
		FROM scenario_runs
		GROUP BY scenario_name
	`)
	if err != nil {
		return nil, fmt.Errorf("search index scenarios: %w", err)
	}
	for rows.Next() {
		var entry SearchScenarioEntry
		var runs, players int
		if err := rows.Scan(&entry.ScenarioName, &entry.ScenarioType, &runs, &players); err != nil {
			rows.Close()
			return nil, err
		}
		entry.ScenarioSlug = slugifyScenarioName(entry.ScenarioName)
		entry.RunCount, entry.PlayerCount = uint32(runs), uint32(players)
		index.Scenarios = append(index.Scenarios, entry)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	profiles, err := s.pool.Query(ctx, `
		SELECT hui.user_handle, hui.user_display_name, hui.avatar_url, hui.is_verified,
		       COALESCE(kv.username, ''), COALESCE(r.runs, 0)::int
		FROM hub_user_identity hui
		LEFT JOIN linked_accounts kv ON kv.user_id = hui.user_id AND kv.provider = 'kovaaks'
		LEFT JOIN (SELECT user_id, COUNT(*) AS runs FROM scenario_runs GROUP BY user_id) r ON r.user_id = hui.user_id
		WHERE TRIM(hui.user_handle) <> ''
	`)
	if err != nil {
		return nil, fmt.Errorf("search index profiles: %w", err)
	}
	defer profiles.Close()
	for profiles.Next() {
		var entry SearchProfileEntry
		var runs int
		if err := profiles.Scan(&entry.UserHandle, &entry.DisplayName, &entry.AvatarURL, &entry.IsVerified, &entry.KovaaksUsername, &runs); err != nil {
			return nil, err
		}
		entry.RunCount = uint32(runs)
		index.Profiles = append(index.Profiles, entry)
	}
	if err := profiles.Err(); err != nil {
		return nil, err
	}
	sort.Slice(index.Scenarios, func(i, j int) bool { return index.Scenarios[i].RunCount > index.Scenarios[j].RunCount })
	s.searchIndex.index = index
	return index, nil
}
