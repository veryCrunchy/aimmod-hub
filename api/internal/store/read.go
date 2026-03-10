package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

type RunRecord struct {
	PublicRunID     string
	SourceSessionID string
	SessionID       string
	ScenarioName    string
	ScenarioType    string
	PlayedAt        time.Time
	Score           float64
	Accuracy        float64
	DurationMS      uint64
	UserHandle      string
	UserDisplayName string
	AvatarURL       string
	Summary         map[string]*hubv1.SessionSummaryValue
	FeatureSet      map[string]*hubv1.SessionSummaryValue
	Timeline        []*hubv1.TimelineSecond
	ContextWindows  []*hubv1.ContextWindow
	ScenarioRuns    []*hubv1.RunPreview
}

type ScenarioPageRecord struct {
	ScenarioName      string
	ScenarioSlug      string
	ScenarioType      string
	RunCount          uint32
	BestScore         float64
	AverageScore      float64
	AverageAccuracy   float64
	AverageDurationMS uint64
	RecentRuns        []*hubv1.RunPreview
	TopRuns           []*hubv1.RunPreview
	ScoreDistribution []*hubv1.ScoreBin
}

type ProfileRecord struct {
	UserExternalID      string
	UserHandle          string
	UserDisplayName     string
	AvatarURL           string
	RunCount            uint32
	ScenarioCount       uint32
	PrimaryScenarioType string
	AverageScore        float64
	AverageAccuracy     float64
	TopScenarios        []*hubv1.TopScenario
	RecentRuns          []*hubv1.RunPreview
	PersonalBests       []*hubv1.RunPreview
}

type OverviewRecord struct {
	TotalRuns      uint32
	TotalScenarios uint32
	TotalPlayers   uint32
	RecentRuns     []*hubv1.RunPreview
	TopScenarios   []*hubv1.TopScenario
	ActiveProfiles []*hubv1.CommunityProfilePreview
}

type SearchScenarioRecord struct {
	ScenarioName string `json:"scenarioName"`
	ScenarioSlug string `json:"scenarioSlug"`
	ScenarioType string `json:"scenarioType"`
	RunCount     uint32 `json:"runCount"`
}

type SearchProfileRecord struct {
	UserHandle          string `json:"userHandle"`
	UserDisplayName     string `json:"userDisplayName"`
	AvatarURL           string `json:"avatarURL"`
	RunCount            uint32 `json:"runCount"`
	ScenarioCount       uint32 `json:"scenarioCount"`
	PrimaryScenarioType string `json:"primaryScenarioType"`
}

type SearchRunRecord struct {
	PublicRunID     string    `json:"publicRunID"`
	SessionID       string    `json:"sessionID"`
	ScenarioName    string    `json:"scenarioName"`
	ScenarioType    string    `json:"scenarioType"`
	PlayedAt        time.Time `json:"playedAt"`
	Score           float64   `json:"score"`
	Accuracy        float64   `json:"accuracy"`
	DurationMS      uint64    `json:"durationMS"`
	UserHandle      string    `json:"userHandle"`
	UserDisplayName string    `json:"userDisplayName"`
}

type SearchRecord struct {
	Query     string                `json:"query"`
	Scenarios []SearchScenarioRecord `json:"scenarios"`
	Profiles  []SearchProfileRecord  `json:"profiles"`
	Runs      []SearchRunRecord      `json:"runs"`
}

func slugifyScenarioName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '\'' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func summaryJSONToMap(raw []byte) (map[string]*hubv1.SessionSummaryValue, error) {
	if len(raw) == 0 {
		return map[string]*hubv1.SessionSummaryValue{}, nil
	}

	decoded := map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode summary json: %w", err)
	}

	out := make(map[string]*hubv1.SessionSummaryValue, len(decoded))
	for key, value := range decoded {
		switch typed := value.(type) {
		case string:
			out[key] = &hubv1.SessionSummaryValue{
				Kind: &hubv1.SessionSummaryValue_StringValue{StringValue: typed},
			}
		case bool:
			out[key] = &hubv1.SessionSummaryValue{
				Kind: &hubv1.SessionSummaryValue_BoolValue{BoolValue: typed},
			}
		case float64:
			out[key] = &hubv1.SessionSummaryValue{
				Kind: &hubv1.SessionSummaryValue_NumberValue{NumberValue: typed},
			}
		}
	}
	return out, nil
}

func runPreviewFromRecord(record RunRecord) *hubv1.RunPreview {
	return &hubv1.RunPreview{
		SessionId:       record.SessionID,
		ScenarioName:    record.ScenarioName,
		ScenarioType:    record.ScenarioType,
		PlayedAtIso:     record.PlayedAt.UTC().Format(time.RFC3339),
		Score:           record.Score,
		Accuracy:        record.Accuracy,
		DurationMs:      record.DurationMS,
		UserHandle:      record.UserHandle,
		UserDisplayName: record.UserDisplayName,
		RunId:           record.PublicRunID,
	}
}

func (s *Store) GetOverview(ctx context.Context) (OverviewRecord, error) {
	var record OverviewRecord
	if err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::bigint,
			COUNT(DISTINCT scenario_name)::bigint,
			COUNT(DISTINCT user_id)::bigint
		FROM scenario_runs
	`).Scan(&record.TotalRuns, &record.TotalScenarios, &record.TotalPlayers); err != nil {
		return OverviewRecord{}, fmt.Errorf("load overview aggregates: %w", err)
	}

	recentRuns, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(sr.public_run_id, sr.session_id),
			sr.session_id,
			sr.scenario_name,
			sr.scenario_type,
			sr.played_at,
			sr.score,
			sr.accuracy,
			sr.duration_ms,
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id))
		FROM scenario_runs sr
		JOIN hub_users hu ON hu.id = sr.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		ORDER BY sr.played_at DESC, sr.created_at DESC
		LIMIT 50
	`)
	if err != nil {
		return OverviewRecord{}, fmt.Errorf("load overview recent runs: %w", err)
	}
	defer recentRuns.Close()
	for recentRuns.Next() {
		var run RunRecord
		if err := recentRuns.Scan(
			&run.PublicRunID,
			&run.SessionID,
			&run.ScenarioName,
			&run.ScenarioType,
			&run.PlayedAt,
			&run.Score,
			&run.Accuracy,
			&run.DurationMS,
			&run.UserHandle,
			&run.UserDisplayName,
		); err != nil {
			return OverviewRecord{}, fmt.Errorf("scan overview recent run: %w", err)
		}
		record.RecentRuns = append(record.RecentRuns, runPreviewFromRecord(run))
	}
	if err := recentRuns.Err(); err != nil {
		return OverviewRecord{}, fmt.Errorf("iterate overview recent runs: %w", err)
	}

	topScenarios, err := s.pool.Query(ctx, `
		SELECT
			scenario_name,
			COALESCE(NULLIF(scenario_type, ''), 'Unknown'),
			COUNT(*) AS run_count
		FROM scenario_runs
		GROUP BY 1, 2
		ORDER BY run_count DESC, scenario_name ASC
		LIMIT 30
	`)
	if err != nil {
		return OverviewRecord{}, fmt.Errorf("load overview top scenarios: %w", err)
	}
	defer topScenarios.Close()
	for topScenarios.Next() {
		topScenario := &hubv1.TopScenario{}
		if err := topScenarios.Scan(&topScenario.ScenarioName, &topScenario.ScenarioType, &topScenario.RunCount); err != nil {
			return OverviewRecord{}, fmt.Errorf("scan overview top scenario: %w", err)
		}
		topScenario.ScenarioSlug = slugifyScenarioName(topScenario.ScenarioName)
		record.TopScenarios = append(record.TopScenarios, topScenario)
	}
	if err := topScenarios.Err(); err != nil {
		return OverviewRecord{}, fmt.Errorf("iterate overview top scenarios: %w", err)
	}

	activeProfiles, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(la.username, hu.external_id) AS user_handle,
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id)) AS user_display_name,
			COALESCE(la.avatar_url, '') AS avatar_url,
			COUNT(sr.*)::bigint AS run_count,
			COUNT(DISTINCT sr.scenario_name)::bigint AS scenario_count,
			COALESCE((
				SELECT COALESCE(NULLIF(sr2.scenario_type, ''), 'Unknown')
				FROM scenario_runs sr2
				WHERE sr2.user_id = hu.id
				GROUP BY COALESCE(NULLIF(sr2.scenario_type, ''), 'Unknown')
				ORDER BY COUNT(*) DESC, COALESCE(NULLIF(sr2.scenario_type, ''), 'Unknown') ASC
				LIMIT 1
			), 'Unknown') AS primary_scenario_type
		FROM hub_users hu
		JOIN scenario_runs sr ON sr.user_id = hu.id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		GROUP BY hu.id, user_handle, user_display_name, avatar_url
		ORDER BY run_count DESC, user_display_name ASC
		LIMIT 30
	`)
	if err != nil {
		return OverviewRecord{}, fmt.Errorf("load overview active profiles: %w", err)
	}
	defer activeProfiles.Close()
	for activeProfiles.Next() {
		profile := &hubv1.CommunityProfilePreview{}
		if err := activeProfiles.Scan(
			&profile.UserHandle,
			&profile.UserDisplayName,
			&profile.AvatarUrl,
			&profile.RunCount,
			&profile.ScenarioCount,
			&profile.PrimaryScenarioType,
		); err != nil {
			return OverviewRecord{}, fmt.Errorf("scan overview active profile: %w", err)
		}
		record.ActiveProfiles = append(record.ActiveProfiles, profile)
	}
	if err := activeProfiles.Err(); err != nil {
		return OverviewRecord{}, fmt.Errorf("iterate overview active profiles: %w", err)
	}

	return record, nil
}

func (s *Store) GetRun(ctx context.Context, sessionID string) (RunRecord, error) {
	var record RunRecord
	var summaryJSON []byte
	var featureJSON []byte
	if err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(sr.public_run_id, sr.session_id),
			COALESCE(sr.source_session_id, sr.session_id),
			sr.session_id,
			sr.scenario_name,
			sr.scenario_type,
			sr.played_at,
			sr.score,
			sr.accuracy,
			sr.duration_ms,
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id)),
			COALESCE(la.avatar_url, ''),
			COALESCE(rs.summary_json::text, '{}'),
			COALESCE(rf.feature_json::text, '{}')
		FROM scenario_runs sr
		JOIN hub_users hu ON hu.id = sr.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		LEFT JOIN run_summaries rs ON rs.session_id = sr.session_id
		LEFT JOIN run_feature_sets rf ON rf.session_id = sr.session_id
		WHERE sr.public_run_id = $1 OR sr.session_id = $1
	`, sessionID).Scan(
		&record.PublicRunID,
		&record.SourceSessionID,
		&record.SessionID,
		&record.ScenarioName,
		&record.ScenarioType,
		&record.PlayedAt,
		&record.Score,
		&record.Accuracy,
		&record.DurationMS,
		&record.UserHandle,
		&record.UserDisplayName,
		&record.AvatarURL,
		&summaryJSON,
		&featureJSON,
	); err != nil {
		return RunRecord{}, fmt.Errorf("load run: %w", err)
	}

	var err error
	record.Summary, err = summaryJSONToMap(summaryJSON)
	if err != nil {
		return RunRecord{}, err
	}
	record.FeatureSet, err = summaryJSONToMap(featureJSON)
	if err != nil {
		return RunRecord{}, err
	}

	timelineRows, err := s.pool.Query(ctx, `
		SELECT t_sec, score, accuracy, damage_eff, spm, shots, hits, kills, paused
		FROM run_timeline_seconds
		WHERE session_id = $1
		ORDER BY t_sec ASC
	`, sessionID)
	if err != nil {
		return RunRecord{}, fmt.Errorf("load run timeline: %w", err)
	}
	defer timelineRows.Close()
	for timelineRows.Next() {
		point := &hubv1.TimelineSecond{}
		if err := timelineRows.Scan(
			&point.TSec,
			&point.Score,
			&point.Accuracy,
			&point.DamageEff,
			&point.Spm,
			&point.Shots,
			&point.Hits,
			&point.Kills,
			&point.Paused,
		); err != nil {
			return RunRecord{}, fmt.Errorf("scan run timeline: %w", err)
		}
		record.Timeline = append(record.Timeline, point)
	}
	if err := timelineRows.Err(); err != nil {
		return RunRecord{}, fmt.Errorf("iterate run timeline: %w", err)
	}

	windowRows, err := s.pool.Query(ctx, `
		SELECT start_ms, end_ms, window_type, label, feature_summary_json::text, coaching_tags
		FROM run_context_windows
		WHERE session_id = $1
		ORDER BY ordinal ASC
	`, sessionID)
	if err != nil {
		return RunRecord{}, fmt.Errorf("load run context windows: %w", err)
	}
	defer windowRows.Close()
	for windowRows.Next() {
		var featureSummaryJSON []byte
		var tags []string
		window := &hubv1.ContextWindow{}
		if err := windowRows.Scan(
			&window.StartMs,
			&window.EndMs,
			&window.WindowType,
			&window.Label,
			&featureSummaryJSON,
			&tags,
		); err != nil {
			return RunRecord{}, fmt.Errorf("scan run context window: %w", err)
		}
		window.FeatureSummary, err = summaryJSONToMap(featureSummaryJSON)
		if err != nil {
			return RunRecord{}, err
		}
		window.CoachingTags = tags
		record.ContextWindows = append(record.ContextWindows, window)
	}
	if err := windowRows.Err(); err != nil {
		return RunRecord{}, fmt.Errorf("iterate run context windows: %w", err)
	}

	relatedRows, err := s.pool.Query(ctx, `
	    SELECT
	        COALESCE(sr.public_run_id, sr.session_id),
	        sr.session_id,
	        sr.scenario_name,
	        sr.scenario_type,
	        sr.played_at,
	        sr.score,
	        sr.accuracy,
	        sr.duration_ms,
	        COALESCE(la.username, hu.external_id),
	        COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id))
	    FROM scenario_runs sr
	    JOIN hub_users hu ON hu.id = sr.user_id
	    LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
	    WHERE sr.scenario_name = $1
	      AND sr.session_id != $2
	    ORDER BY sr.score DESC, sr.played_at DESC
	    LIMIT 10
	`, record.ScenarioName, record.SessionID)
	if err != nil {
		return RunRecord{}, fmt.Errorf("load scenario runs: %w", err)
	}
	defer relatedRows.Close()
	for relatedRows.Next() {
		var rel RunRecord
		if err := relatedRows.Scan(
			&rel.PublicRunID,
			&rel.SessionID,
			&rel.ScenarioName,
			&rel.ScenarioType,
			&rel.PlayedAt,
			&rel.Score,
			&rel.Accuracy,
			&rel.DurationMS,
			&rel.UserHandle,
			&rel.UserDisplayName,
		); err != nil {
			return RunRecord{}, fmt.Errorf("scan scenario run: %w", err)
		}
		record.ScenarioRuns = append(record.ScenarioRuns, runPreviewFromRecord(rel))
	}
	if err := relatedRows.Err(); err != nil {
		return RunRecord{}, fmt.Errorf("iterate scenario runs: %w", err)
	}

	return record, nil
}

func (s *Store) GetScenarioPage(ctx context.Context, slug string) (ScenarioPageRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT scenario_name
		FROM scenario_runs
		ORDER BY scenario_name ASC
	`)
	if err != nil {
		return ScenarioPageRecord{}, fmt.Errorf("list scenarios: %w", err)
	}
	defer rows.Close()

	scenarioName := ""
	for rows.Next() {
		var candidate string
		if err := rows.Scan(&candidate); err != nil {
			return ScenarioPageRecord{}, fmt.Errorf("scan scenario name: %w", err)
		}
		if slugifyScenarioName(candidate) == slug {
			scenarioName = candidate
			break
		}
	}
	if err := rows.Err(); err != nil {
		return ScenarioPageRecord{}, fmt.Errorf("iterate scenarios: %w", err)
	}
	if scenarioName == "" {
		return ScenarioPageRecord{}, fmt.Errorf("scenario not found")
	}

	var record ScenarioPageRecord
	record.ScenarioName = scenarioName
	record.ScenarioSlug = slugifyScenarioName(scenarioName)
	if err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE((
				SELECT COALESCE(NULLIF(sr2.scenario_type, ''), 'Unknown')
				FROM scenario_runs sr2
				WHERE sr2.scenario_name = $1
				GROUP BY COALESCE(NULLIF(sr2.scenario_type, ''), 'Unknown')
				ORDER BY COUNT(*) DESC, COALESCE(NULLIF(sr2.scenario_type, ''), 'Unknown') ASC
				LIMIT 1
			), 'Unknown'),
			COUNT(*),
			COALESCE(MAX(score), 0),
			COALESCE(AVG(score), 0),
			COALESCE(AVG(accuracy), 0),
			COALESCE(ROUND(AVG(duration_ms))::bigint, 0)
		FROM scenario_runs
		WHERE scenario_name = $1
	`, scenarioName).Scan(
		&record.ScenarioType,
		&record.RunCount,
		&record.BestScore,
		&record.AverageScore,
		&record.AverageAccuracy,
		&record.AverageDurationMS,
	); err != nil {
		return ScenarioPageRecord{}, fmt.Errorf("load scenario aggregates: %w", err)
	}

	recentRows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(sr.public_run_id, sr.session_id),
			sr.session_id,
			sr.scenario_name,
			sr.scenario_type,
			sr.played_at,
			sr.score,
			sr.accuracy,
			sr.duration_ms,
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id))
		FROM scenario_runs sr
		JOIN hub_users hu ON hu.id = sr.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE sr.scenario_name = $1
		ORDER BY sr.played_at DESC, sr.created_at DESC
		LIMIT 50
	`, scenarioName)
	if err != nil {
		return ScenarioPageRecord{}, fmt.Errorf("load scenario recent runs: %w", err)
	}
	defer recentRows.Close()
	for recentRows.Next() {
		var run RunRecord
		if err := recentRows.Scan(
			&run.PublicRunID,
			&run.SessionID,
			&run.ScenarioName,
			&run.ScenarioType,
			&run.PlayedAt,
			&run.Score,
			&run.Accuracy,
			&run.DurationMS,
			&run.UserHandle,
			&run.UserDisplayName,
		); err != nil {
			return ScenarioPageRecord{}, fmt.Errorf("scan scenario recent run: %w", err)
		}
		record.RecentRuns = append(record.RecentRuns, runPreviewFromRecord(run))
	}
	if err := recentRows.Err(); err != nil {
		return ScenarioPageRecord{}, fmt.Errorf("iterate scenario recent runs: %w", err)
	}

	topRows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(sr.public_run_id, sr.session_id),
			sr.session_id,
			sr.scenario_name,
			sr.scenario_type,
			sr.played_at,
			sr.score,
			sr.accuracy,
			sr.duration_ms,
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id))
		FROM scenario_runs sr
		JOIN hub_users hu ON hu.id = sr.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE sr.scenario_name = $1
		ORDER BY sr.score DESC, sr.played_at DESC
		LIMIT 50
	`, scenarioName)
	if err != nil {
		return ScenarioPageRecord{}, fmt.Errorf("load scenario top runs: %w", err)
	}
	defer topRows.Close()
	for topRows.Next() {
		var run RunRecord
		if err := topRows.Scan(
			&run.PublicRunID,
			&run.SessionID,
			&run.ScenarioName,
			&run.ScenarioType,
			&run.PlayedAt,
			&run.Score,
			&run.Accuracy,
			&run.DurationMS,
			&run.UserHandle,
			&run.UserDisplayName,
		); err != nil {
			return ScenarioPageRecord{}, fmt.Errorf("scan scenario top run: %w", err)
		}
		record.TopRuns = append(record.TopRuns, runPreviewFromRecord(run))
	}
	if err := topRows.Err(); err != nil {
		return ScenarioPageRecord{}, fmt.Errorf("iterate scenario top runs: %w", err)
	}

	// Compute score distribution histogram
	scoreRows, err := s.pool.Query(ctx, `
	    SELECT score FROM scenario_runs WHERE scenario_name = $1 AND score > 0
	`, scenarioName)
	if err != nil {
		return ScenarioPageRecord{}, fmt.Errorf("load score distribution: %w", err)
	}
	defer scoreRows.Close()
	var allScores []float64
	for scoreRows.Next() {
		var score float64
		if err := scoreRows.Scan(&score); err != nil {
			return ScenarioPageRecord{}, fmt.Errorf("scan score: %w", err)
		}
		allScores = append(allScores, score)
	}
	if err := scoreRows.Err(); err != nil {
		return ScenarioPageRecord{}, fmt.Errorf("iterate scores: %w", err)
	}
	if len(allScores) >= 3 {
		record.ScoreDistribution = computeScoreHistogram(allScores, 10)
	}

	return record, nil
}

func computeScoreHistogram(scores []float64, bins int) []*hubv1.ScoreBin {
	if len(scores) == 0 || bins <= 0 {
		return nil
	}
	minScore, maxScore := scores[0], scores[0]
	for _, s := range scores[1:] {
		if s < minScore {
			minScore = s
		}
		if s > maxScore {
			maxScore = s
		}
	}
	if maxScore <= minScore {
		return nil
	}
	binSize := (maxScore - minScore) / float64(bins)
	result := make([]*hubv1.ScoreBin, bins)
	for i := range result {
		result[i] = &hubv1.ScoreBin{
			Lo: minScore + float64(i)*binSize,
			Hi: minScore + float64(i+1)*binSize,
		}
	}
	for _, s := range scores {
		idx := int((s - minScore) / binSize)
		if idx >= bins {
			idx = bins - 1
		}
		if idx < 0 {
			idx = 0
		}
		result[idx].Count++
	}
	return result
}

func (s *Store) GetProfile(ctx context.Context, handle string) (ProfileRecord, error) {
	handle = strings.TrimSpace(strings.ToLower(handle))
	if handle == "" {
		return ProfileRecord{}, fmt.Errorf("profile handle is required")
	}

	var record ProfileRecord
	var userID int64
	if err := s.pool.QueryRow(ctx, `
		SELECT
			hu.id,
			hu.external_id,
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id)),
			COALESCE(la.avatar_url, '')
		FROM hub_users hu
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
		   OR LOWER(hu.external_id) = $1
		LIMIT 1
	`, handle).Scan(
		&userID,
		&record.UserExternalID,
		&record.UserHandle,
		&record.UserDisplayName,
		&record.AvatarURL,
	); err != nil {
		return ProfileRecord{}, fmt.Errorf("load profile: %w", err)
	}

	if err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(DISTINCT scenario_name),
			COALESCE(AVG(score), 0),
			COALESCE(AVG(accuracy), 0)
		FROM scenario_runs
		WHERE user_id = $1
	`, userID).Scan(
		&record.RunCount,
		&record.ScenarioCount,
		&record.AverageScore,
		&record.AverageAccuracy,
	); err != nil {
		return ProfileRecord{}, fmt.Errorf("load profile aggregates: %w", err)
	}

	typeRows, err := s.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(scenario_type, ''), 'Unknown') AS scenario_type, COUNT(*) AS c
		FROM scenario_runs
		WHERE user_id = $1
		GROUP BY 1
		ORDER BY c DESC, scenario_type ASC
		LIMIT 1
	`, userID)
	if err != nil {
		return ProfileRecord{}, fmt.Errorf("load profile scenario type: %w", err)
	}
	defer typeRows.Close()
	if typeRows.Next() {
		var count int64
		if err := typeRows.Scan(&record.PrimaryScenarioType, &count); err != nil {
			return ProfileRecord{}, fmt.Errorf("scan profile scenario type: %w", err)
		}
	}

	topRows, err := s.pool.Query(ctx, `
		SELECT
			scenario_name,
			COALESCE(NULLIF(scenario_type, ''), 'Unknown'),
			COUNT(*) AS run_count
		FROM scenario_runs
		WHERE user_id = $1
		GROUP BY 1, 2
		ORDER BY run_count DESC, scenario_name ASC
		LIMIT 30
	`, userID)
	if err != nil {
		return ProfileRecord{}, fmt.Errorf("load top scenarios: %w", err)
	}
	defer topRows.Close()
	for topRows.Next() {
		var scenarioName string
		topScenario := &hubv1.TopScenario{}
		if err := topRows.Scan(&scenarioName, &topScenario.ScenarioType, &topScenario.RunCount); err != nil {
			return ProfileRecord{}, fmt.Errorf("scan top scenario: %w", err)
		}
		topScenario.ScenarioName = scenarioName
		topScenario.ScenarioSlug = slugifyScenarioName(scenarioName)
		record.TopScenarios = append(record.TopScenarios, topScenario)
	}
	if err := topRows.Err(); err != nil {
		return ProfileRecord{}, fmt.Errorf("iterate top scenarios: %w", err)
	}

	recentRows, err := s.pool.Query(ctx, `
		SELECT COALESCE(public_run_id, session_id), session_id, scenario_name, scenario_type, played_at, score, accuracy, duration_ms
		FROM scenario_runs
		WHERE user_id = $1
		ORDER BY played_at DESC, created_at DESC
		LIMIT 50
	`, userID)
	if err != nil {
		return ProfileRecord{}, fmt.Errorf("load recent profile runs: %w", err)
	}
	defer recentRows.Close()
	for recentRows.Next() {
		var run RunRecord
		run.UserHandle = record.UserHandle
		run.UserDisplayName = record.UserDisplayName
		if err := recentRows.Scan(
			&run.PublicRunID,
			&run.SessionID,
			&run.ScenarioName,
			&run.ScenarioType,
			&run.PlayedAt,
			&run.Score,
			&run.Accuracy,
			&run.DurationMS,
		); err != nil {
			return ProfileRecord{}, fmt.Errorf("scan recent profile run: %w", err)
		}
		record.RecentRuns = append(record.RecentRuns, runPreviewFromRecord(run))
	}
	if err := recentRows.Err(); err != nil {
		return ProfileRecord{}, fmt.Errorf("iterate recent profile runs: %w", err)
	}

	bestsRows, err := s.pool.Query(ctx, `
	    SELECT DISTINCT ON (sr.scenario_name)
	        COALESCE(sr.public_run_id, sr.session_id),
	        sr.session_id,
	        sr.scenario_name,
	        sr.scenario_type,
	        sr.played_at,
	        sr.score,
	        sr.accuracy,
	        sr.duration_ms
	    FROM scenario_runs sr
	    WHERE sr.user_id = $1
	    ORDER BY sr.scenario_name, sr.score DESC, sr.played_at DESC
	    LIMIT 50
	`, userID)
	if err != nil {
		return ProfileRecord{}, fmt.Errorf("load personal bests: %w", err)
	}
	defer bestsRows.Close()
	for bestsRows.Next() {
		var run RunRecord
		run.UserHandle = record.UserHandle
		run.UserDisplayName = record.UserDisplayName
		if err := bestsRows.Scan(
			&run.PublicRunID,
			&run.SessionID,
			&run.ScenarioName,
			&run.ScenarioType,
			&run.PlayedAt,
			&run.Score,
			&run.Accuracy,
			&run.DurationMS,
		); err != nil {
			return ProfileRecord{}, fmt.Errorf("scan personal best: %w", err)
		}
		record.PersonalBests = append(record.PersonalBests, runPreviewFromRecord(run))
	}
	if err := bestsRows.Err(); err != nil {
		return ProfileRecord{}, fmt.Errorf("iterate personal bests: %w", err)
	}

	sort.SliceStable(record.TopScenarios, func(i, j int) bool {
		if record.TopScenarios[i].RunCount == record.TopScenarios[j].RunCount {
			return record.TopScenarios[i].ScenarioName < record.TopScenarios[j].ScenarioName
		}
		return record.TopScenarios[i].RunCount > record.TopScenarios[j].RunCount
	})

	return record, nil
}

func (s *Store) Search(ctx context.Context, query string) (SearchRecord, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return SearchRecord{Query: query}, nil
	}

	record := SearchRecord{Query: query}
	pattern := "%" + query + "%"

	scenarioRows, err := s.pool.Query(ctx, `
		SELECT
			sr.scenario_name,
			COALESCE((
				SELECT NULLIF(sr2.scenario_type, '')
				FROM scenario_runs sr2
				WHERE sr2.scenario_name = sr.scenario_name
				  AND NULLIF(sr2.scenario_type, '') IS NOT NULL
				GROUP BY sr2.scenario_type
				ORDER BY COUNT(*) DESC, sr2.scenario_type ASC
				LIMIT 1
			), ''),
			COUNT(*)::bigint
		FROM scenario_runs sr
		WHERE sr.scenario_name ILIKE $1
		GROUP BY sr.scenario_name
		ORDER BY COUNT(*) DESC, sr.scenario_name ASC
		LIMIT 12
	`, pattern)
	if err != nil {
		return SearchRecord{}, fmt.Errorf("search scenarios: %w", err)
	}
	defer scenarioRows.Close()
	for scenarioRows.Next() {
		var item SearchScenarioRecord
		if err := scenarioRows.Scan(&item.ScenarioName, &item.ScenarioType, &item.RunCount); err != nil {
			return SearchRecord{}, fmt.Errorf("scan search scenario: %w", err)
		}
		item.ScenarioSlug = slugifyScenarioName(item.ScenarioName)
		record.Scenarios = append(record.Scenarios, item)
	}
	if err := scenarioRows.Err(); err != nil {
		return SearchRecord{}, fmt.Errorf("iterate search scenarios: %w", err)
	}

	profileRows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(la.username, hu.external_id) AS user_handle,
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id)) AS user_display_name,
			COALESCE(la.avatar_url, '') AS avatar_url,
			COUNT(sr.*)::bigint AS run_count,
			COUNT(DISTINCT sr.scenario_name)::bigint AS scenario_count,
			COALESCE((
				SELECT NULLIF(sr2.scenario_type, '')
				FROM scenario_runs sr2
				WHERE sr2.user_id = hu.id
				  AND NULLIF(sr2.scenario_type, '') IS NOT NULL
				GROUP BY sr2.scenario_type
				ORDER BY COUNT(*) DESC, sr2.scenario_type ASC
				LIMIT 1
			), '') AS primary_scenario_type
		FROM hub_users hu
		JOIN scenario_runs sr ON sr.user_id = hu.id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE COALESCE(la.username, hu.external_id) ILIKE $1
		   OR COALESCE(NULLIF(la.display_name, ''), '') ILIKE $1
		   OR hu.external_id ILIKE $1
		GROUP BY hu.id, user_handle, user_display_name, avatar_url
		ORDER BY run_count DESC, user_display_name ASC
		LIMIT 12
	`, pattern)
	if err != nil {
		return SearchRecord{}, fmt.Errorf("search profiles: %w", err)
	}
	defer profileRows.Close()
	for profileRows.Next() {
		var item SearchProfileRecord
		if err := profileRows.Scan(
			&item.UserHandle,
			&item.UserDisplayName,
			&item.AvatarURL,
			&item.RunCount,
			&item.ScenarioCount,
			&item.PrimaryScenarioType,
		); err != nil {
			return SearchRecord{}, fmt.Errorf("scan search profile: %w", err)
		}
		record.Profiles = append(record.Profiles, item)
	}
	if err := profileRows.Err(); err != nil {
		return SearchRecord{}, fmt.Errorf("iterate search profiles: %w", err)
	}

	runRows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(sr.public_run_id, sr.session_id),
			sr.session_id,
			sr.scenario_name,
			sr.scenario_type,
			sr.played_at,
			sr.score,
			sr.accuracy,
			sr.duration_ms,
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id))
		FROM scenario_runs sr
		JOIN hub_users hu ON hu.id = sr.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE sr.scenario_name ILIKE $1
		   OR COALESCE(la.username, hu.external_id) ILIKE $1
		   OR COALESCE(NULLIF(la.display_name, ''), '') ILIKE $1
		   OR sr.public_run_id ILIKE $1
		   OR sr.session_id ILIKE $1
		ORDER BY sr.played_at DESC, sr.created_at DESC
		LIMIT 20
	`, pattern)
	if err != nil {
		return SearchRecord{}, fmt.Errorf("search runs: %w", err)
	}
	defer runRows.Close()
	for runRows.Next() {
		var item SearchRunRecord
		if err := runRows.Scan(
			&item.PublicRunID,
			&item.SessionID,
			&item.ScenarioName,
			&item.ScenarioType,
			&item.PlayedAt,
			&item.Score,
			&item.Accuracy,
			&item.DurationMS,
			&item.UserHandle,
			&item.UserDisplayName,
		); err != nil {
			return SearchRecord{}, fmt.Errorf("scan search run: %w", err)
		}
		record.Runs = append(record.Runs, item)
	}
	if err := runRows.Err(); err != nil {
		return SearchRecord{}, fmt.Errorf("iterate search runs: %w", err)
	}

	return record, nil
}

func (s *Store) ReclassifyTracking(ctx context.Context) (pgconn.CommandTag, error) {
	return s.pool.Exec(ctx, `
	    UPDATE scenario_runs sr
	    SET scenario_type = 'Tracking'
	    FROM run_summaries rs
	    WHERE sr.session_id = rs.session_id
	      AND sr.scenario_type IN ('MultiHitClicking', 'Unknown', '')
	      AND (
	        (rs.summary_json->>'csvAvgTtk')::float >= 5.0
	        OR (
	          (rs.summary_json->>'killsPerSecond')::float > 0
	          AND (sr.duration_ms::float / 1000.0) > 0
	          AND (rs.summary_json->>'damageDone')::float
	            / NULLIF(
	                (rs.summary_json->>'killsPerSecond')::float * (sr.duration_ms::float / 1000.0),
	                0
	              ) < 0.5
	        )
	      )
	`)
}
