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
	IsVerified          bool
	RunCount            uint32
	ScenarioCount       uint32
	PrimaryScenarioType string
	AverageScore        float64
	AverageAccuracy     float64
	TopScenarios        []*hubv1.TopScenario
	RecentRuns          []*hubv1.RunPreview
	PersonalBests       []*hubv1.RunPreview
}

type LeaderboardRecord struct {
	Records   []*hubv1.RunPreview
	TopScores []*hubv1.RunPreview
}

type PlayerScenarioHistoryRecord struct {
	ScenarioName    string
	ScenarioSlug    string
	ScenarioType    string
	Runs            []*hubv1.RunPreview
	BestScore       float64
	AverageScore    float64
	BestAccuracy    float64
	AverageAccuracy float64
	RunCount        int32
}

type resolvedUserIdentity struct {
	UserID          int64
	UserExternalID  string
	UserHandle      string
	UserDisplayName string
}

func (s *Store) resolveUserIdentityByHandle(ctx context.Context, handle string) (resolvedUserIdentity, error) {
	handle = strings.TrimSpace(strings.ToLower(handle))
	if handle == "" {
		return resolvedUserIdentity{}, fmt.Errorf("profile handle is required")
	}

	var result resolvedUserIdentity
	if err := s.pool.QueryRow(ctx, `
		SELECT
			hui.user_id,
			hui.external_id,
			hui.user_handle,
			hui.user_display_name
		FROM hub_user_identity hui
		WHERE LOWER(hui.user_handle) = $1
		   OR LOWER(hui.external_id) = $1
		LIMIT 1
	`, handle).Scan(&result.UserID, &result.UserExternalID, &result.UserHandle, &result.UserDisplayName); err != nil {
		return resolvedUserIdentity{}, fmt.Errorf("load user identity by handle: %w", err)
	}
	return result, nil
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
	IsVerified          bool   `json:"isVerified"`
	RunCount            uint32 `json:"runCount"`
	ScenarioCount       uint32 `json:"scenarioCount"`
	PrimaryScenarioType string `json:"primaryScenarioType"`
}

type SearchRunRecord struct {
	PublicRunID     string    `json:"publicRunID"`
	SessionID       string    `json:"sessionID"`
	ScenarioSlug    string    `json:"scenarioSlug"`
	ScenarioName    string    `json:"scenarioName"`
	ScenarioType    string    `json:"scenarioType"`
	PlayedAt        time.Time `json:"playedAt"`
	Score           float64   `json:"score"`
	Accuracy        float64   `json:"accuracy"`
	DurationMS      uint64    `json:"durationMS"`
	UserHandle      string    `json:"userHandle"`
	UserDisplayName string    `json:"userDisplayName"`
	HasVideo        bool      `json:"hasVideo"`
	HasMousePath    bool      `json:"hasMousePath"`
	ReplayQuality   string    `json:"replayQuality"`
}

type SearchRecord struct {
	Query     string                 `json:"query"`
	Scenarios []SearchScenarioRecord `json:"scenarios"`
	Profiles  []SearchProfileRecord  `json:"profiles"`
	Runs      []SearchRunRecord      `json:"runs"`
	Replays   []SearchRunRecord      `json:"replays"`
}

type ReplayListRecord struct {
	Query        string            `json:"query"`
	ScenarioName string            `json:"scenarioName"`
	UserHandle   string            `json:"userHandle"`
	Items        []SearchRunRecord `json:"items"`
}

type AdminVersionBreakdown struct {
	Label    string `json:"label"`
	RunCount uint64 `json:"runCount"`
}

type AdminScenarioIssue struct {
	ScenarioName string `json:"scenarioName"`
	ScenarioSlug string `json:"scenarioSlug"`
	RunCount     uint64 `json:"runCount"`
}

type AdminRecentIngest struct {
	PublicRunID     string    `json:"publicRunId"`
	SessionID       string    `json:"sessionId"`
	SourceSessionID string    `json:"sourceSessionId"`
	ScenarioName    string    `json:"scenarioName"`
	ScenarioSlug    string    `json:"scenarioSlug"`
	ScenarioType    string    `json:"scenarioType"`
	UserHandle      string    `json:"userHandle"`
	UserDisplayName string    `json:"userDisplayName"`
	PlayedAt        time.Time `json:"playedAt"`
	IngestedAt      time.Time `json:"ingestedAt"`
	Score           float64   `json:"score"`
}

type AdminUserSyncHealth struct {
	UserHandle          string    `json:"userHandle"`
	UserDisplayName     string    `json:"userDisplayName"`
	RunCount            uint64    `json:"runCount"`
	UnknownTypeRuns     uint64    `json:"unknownTypeRuns"`
	MissingTimelineRuns uint64    `json:"missingTimelineRuns"`
	MissingContextRuns  uint64    `json:"missingContextRuns"`
	ZeroScoreRuns       uint64    `json:"zeroScoreRuns"`
	LastPlayedAt        time.Time `json:"lastPlayedAt"`
	LastIngestedAt      time.Time `json:"lastIngestedAt"`
}

type AdminIngestFailure struct {
	ID              uint64    `json:"id"`
	UserExternalID  string    `json:"userExternalId"`
	UserHandle      string    `json:"userHandle"`
	UserDisplayName string    `json:"userDisplayName"`
	SessionID       string    `json:"sessionId"`
	PublicRunID     string    `json:"publicRunId"`
	ScenarioName    string    `json:"scenarioName"`
	ErrorMessage    string    `json:"errorMessage"`
	CreatedAt       time.Time `json:"createdAt"`
}

type AdminUserRecentRun struct {
	PublicRunID  string    `json:"publicRunId"`
	ScenarioName string    `json:"scenarioName"`
	ScenarioSlug string    `json:"scenarioSlug"`
	ScenarioType string    `json:"scenarioType"`
	PlayedAt     time.Time `json:"playedAt"`
	Score        float64   `json:"score"`
	Accuracy     float64   `json:"accuracy"`
	DurationMS   uint64    `json:"durationMs"`
}

type AdminUserDetailRecord struct {
	UserHandle          string               `json:"userHandle"`
	UserDisplayName     string               `json:"userDisplayName"`
	RunCount            uint64               `json:"runCount"`
	ScenarioCount       uint64               `json:"scenarioCount"`
	UnknownTypeRuns     uint64               `json:"unknownTypeRuns"`
	MissingTimelineRuns uint64               `json:"missingTimelineRuns"`
	MissingContextRuns  uint64               `json:"missingContextRuns"`
	ZeroScoreRuns       uint64               `json:"zeroScoreRuns"`
	LastPlayedAt        time.Time            `json:"lastPlayedAt"`
	LastIngestedAt      time.Time            `json:"lastIngestedAt"`
	TopUnknownScenarios []AdminScenarioIssue `json:"topUnknownScenarios"`
	RecentFailures      []AdminIngestFailure `json:"recentFailures"`
	RecentRuns          []AdminUserRecentRun `json:"recentRuns"`
}

type AdminUserMetricsExportRun struct {
	PublicRunID         string          `json:"publicRunId"`
	SessionID           string          `json:"sessionId"`
	SourceSessionID     string          `json:"sourceSessionId"`
	ScenarioName        string          `json:"scenarioName"`
	ScenarioType        string          `json:"scenarioType"`
	PlayedAt            time.Time       `json:"playedAt"`
	Score               float64         `json:"score"`
	Accuracy            float64         `json:"accuracy"`
	DurationMS          uint64          `json:"durationMs"`
	AppVersion          string          `json:"appVersion"`
	SchemaVersion       uint32          `json:"schemaVersion"`
	TimelineSecondCount uint64          `json:"timelineSecondCount"`
	ContextWindowCount  uint64          `json:"contextWindowCount"`
	SummaryJSON         json.RawMessage `json:"summaryJson"`
	FeatureJSON         json.RawMessage `json:"featureJson"`
}

type AdminUserMetricsExport struct {
	ExportedAt      time.Time                   `json:"exportedAt"`
	Days            int                         `json:"days"`
	UserHandle      string                      `json:"userHandle"`
	UserDisplayName string                      `json:"userDisplayName"`
	RunCount        uint64                      `json:"runCount"`
	RecentFailures  []AdminIngestFailure        `json:"recentFailures"`
	Runs            []AdminUserMetricsExportRun `json:"runs"`
}

type AdminOverviewRecord struct {
	TotalRuns                uint64                  `json:"totalRuns"`
	TotalPlayers             uint64                  `json:"totalPlayers"`
	TotalScenarios           uint64                  `json:"totalScenarios"`
	UnknownTypeRuns          uint64                  `json:"unknownTypeRuns"`
	MissingSummaryRuns       uint64                  `json:"missingSummaryRuns"`
	MissingFeatureRuns       uint64                  `json:"missingFeatureRuns"`
	MissingTimelineRuns      uint64                  `json:"missingTimelineRuns"`
	MissingContextRuns       uint64                  `json:"missingContextRuns"`
	ZeroScoreRuns            uint64                  `json:"zeroScoreRuns"`
	MissingSourceSessionRuns uint64                  `json:"missingSourceSessionRuns"`
	AppVersions              []AdminVersionBreakdown `json:"appVersions"`
	SchemaVersions           []AdminVersionBreakdown `json:"schemaVersions"`
	TopUnknownScenarios      []AdminScenarioIssue    `json:"topUnknownScenarios"`
	RecentIngests            []AdminRecentIngest     `json:"recentIngests"`
	UserSyncHealth           []AdminUserSyncHealth   `json:"userSyncHealth"`
	RecentFailures           []AdminIngestFailure    `json:"recentFailures"`
}

func adminPlayedAfterClause(days int) (string, []any) {
	if days <= 0 {
		return "", nil
	}
	return " WHERE sr.played_at >= NOW() - ($1::int * INTERVAL '1 day') ", []any{days}
}

func adminUserPlayedAfterClause(days int, argOffset int) (string, []any) {
	if days <= 0 {
		return "", nil
	}
	return fmt.Sprintf(" AND sr.played_at >= NOW() - ($%d::int * INTERVAL '1 day') ", argOffset), []any{days}
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

func (s *Store) GetAdminOverview(ctx context.Context, days int) (AdminOverviewRecord, error) {
	var record AdminOverviewRecord
	playedWhere, playedArgs := adminPlayedAfterClause(days)
	unknownWhere := " WHERE COALESCE(NULLIF(scenario_type, ''), 'Unknown') = 'Unknown' "
	if days > 0 {
		unknownWhere = " WHERE played_at >= NOW() - ($1::int * INTERVAL '1 day') AND COALESCE(NULLIF(scenario_type, ''), 'Unknown') = 'Unknown' "
	}

	if err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::bigint,
			COUNT(DISTINCT user_id)::bigint,
			COUNT(DISTINCT scenario_name)::bigint,
			COUNT(*) FILTER (WHERE COALESCE(NULLIF(scenario_type, ''), 'Unknown') = 'Unknown')::bigint,
			COUNT(*) FILTER (WHERE rs.session_id IS NULL)::bigint,
			COUNT(*) FILTER (WHERE rf.session_id IS NULL)::bigint,
			COUNT(*) FILTER (WHERE rt.session_id IS NULL)::bigint,
			COUNT(*) FILTER (WHERE rc.session_id IS NULL)::bigint,
			COUNT(*) FILTER (WHERE score <= 0)::bigint,
			COUNT(*) FILTER (WHERE COALESCE(NULLIF(source_session_id, ''), '') = '')::bigint
		FROM scenario_runs sr
		LEFT JOIN run_summaries rs ON rs.session_id = sr.session_id
		LEFT JOIN run_feature_sets rf ON rf.session_id = sr.session_id
		LEFT JOIN (
			SELECT DISTINCT session_id
			FROM run_timeline_seconds
		) rt ON rt.session_id = sr.session_id
		LEFT JOIN (
			SELECT DISTINCT session_id
			FROM run_context_windows
		) rc ON rc.session_id = sr.session_id
	`+playedWhere, playedArgs...).Scan(
		&record.TotalRuns,
		&record.TotalPlayers,
		&record.TotalScenarios,
		&record.UnknownTypeRuns,
		&record.MissingSummaryRuns,
		&record.MissingFeatureRuns,
		&record.MissingTimelineRuns,
		&record.MissingContextRuns,
		&record.ZeroScoreRuns,
		&record.MissingSourceSessionRuns,
	); err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("load admin overview aggregates: %w", err)
	}

	appVersions, err := s.pool.Query(ctx, `
		SELECT app_version, COUNT(*)::bigint
		FROM scenario_runs sr
	`+playedWhere+`
		GROUP BY app_version
		ORDER BY COUNT(*) DESC, app_version DESC
		LIMIT 12
	`, playedArgs...)
	if err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("load admin app versions: %w", err)
	}
	defer appVersions.Close()
	for appVersions.Next() {
		var item AdminVersionBreakdown
		if err := appVersions.Scan(&item.Label, &item.RunCount); err != nil {
			return AdminOverviewRecord{}, fmt.Errorf("scan admin app version: %w", err)
		}
		record.AppVersions = append(record.AppVersions, item)
	}
	if err := appVersions.Err(); err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("iterate admin app versions: %w", err)
	}

	schemaVersions, err := s.pool.Query(ctx, `
		SELECT schema_version::text, COUNT(*)::bigint
		FROM scenario_runs sr
	`+playedWhere+`
		GROUP BY schema_version
		ORDER BY COUNT(*) DESC, schema_version DESC
		LIMIT 12
	`, playedArgs...)
	if err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("load admin schema versions: %w", err)
	}
	defer schemaVersions.Close()
	for schemaVersions.Next() {
		var item AdminVersionBreakdown
		if err := schemaVersions.Scan(&item.Label, &item.RunCount); err != nil {
			return AdminOverviewRecord{}, fmt.Errorf("scan admin schema version: %w", err)
		}
		record.SchemaVersions = append(record.SchemaVersions, item)
	}
	if err := schemaVersions.Err(); err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("iterate admin schema versions: %w", err)
	}

	unknownScenarios, err := s.pool.Query(ctx, `
		SELECT
			scenario_name,
			COUNT(*)::bigint
		FROM scenario_runs sr
	`+unknownWhere+`
		GROUP BY scenario_name
		ORDER BY COUNT(*) DESC, scenario_name ASC
		LIMIT 20
	`, playedArgs...)
	if err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("load admin unknown scenarios: %w", err)
	}
	defer unknownScenarios.Close()
	for unknownScenarios.Next() {
		var item AdminScenarioIssue
		if err := unknownScenarios.Scan(&item.ScenarioName, &item.RunCount); err != nil {
			return AdminOverviewRecord{}, fmt.Errorf("scan admin unknown scenario: %w", err)
		}
		item.ScenarioSlug = slugifyScenarioName(item.ScenarioName)
		record.TopUnknownScenarios = append(record.TopUnknownScenarios, item)
	}
	if err := unknownScenarios.Err(); err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("iterate admin unknown scenarios: %w", err)
	}

	recentIngests, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(sr.public_run_id, sr.session_id),
			sr.session_id,
			COALESCE(sr.source_session_id, ''),
			sr.scenario_name,
			COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown'),
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id)),
			sr.played_at,
			sr.created_at,
			sr.score
		FROM scenario_runs sr
		JOIN hub_users hu ON hu.id = sr.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
	`+playedWhere+`
		ORDER BY sr.created_at DESC, sr.played_at DESC
		LIMIT 40
	`, playedArgs...)
	if err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("load admin recent ingests: %w", err)
	}
	defer recentIngests.Close()
	for recentIngests.Next() {
		var item AdminRecentIngest
		if err := recentIngests.Scan(
			&item.PublicRunID,
			&item.SessionID,
			&item.SourceSessionID,
			&item.ScenarioName,
			&item.ScenarioType,
			&item.UserHandle,
			&item.UserDisplayName,
			&item.PlayedAt,
			&item.IngestedAt,
			&item.Score,
		); err != nil {
			return AdminOverviewRecord{}, fmt.Errorf("scan admin recent ingest: %w", err)
		}
		item.ScenarioSlug = slugifyScenarioName(item.ScenarioName)
		record.RecentIngests = append(record.RecentIngests, item)
	}
	if err := recentIngests.Err(); err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("iterate admin recent ingests: %w", err)
	}

	userHealthRows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id)),
			COUNT(sr.*)::bigint,
			COUNT(*) FILTER (WHERE COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown')::bigint,
			COUNT(*) FILTER (WHERE rt.session_id IS NULL)::bigint,
			COUNT(*) FILTER (WHERE rc.session_id IS NULL)::bigint,
			COUNT(*) FILTER (WHERE sr.score <= 0)::bigint,
			MAX(sr.played_at),
			MAX(sr.created_at)
		FROM hub_users hu
		JOIN scenario_runs sr ON sr.user_id = hu.id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		LEFT JOIN (
			SELECT DISTINCT session_id
			FROM run_timeline_seconds
		) rt ON rt.session_id = sr.session_id
		LEFT JOIN (
			SELECT DISTINCT session_id
			FROM run_context_windows
		) rc ON rc.session_id = sr.session_id
	`+playedWhere+`
		GROUP BY hu.id, COALESCE(la.username, hu.external_id), COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id))
		ORDER BY COUNT(*) FILTER (WHERE COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown') DESC,
		         COUNT(*) FILTER (WHERE rt.session_id IS NULL) DESC,
		         MAX(sr.created_at) DESC
		LIMIT 20
	`, playedArgs...)
	if err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("load admin user sync health: %w", err)
	}
	defer userHealthRows.Close()
	for userHealthRows.Next() {
		var item AdminUserSyncHealth
		if err := userHealthRows.Scan(
			&item.UserHandle,
			&item.UserDisplayName,
			&item.RunCount,
			&item.UnknownTypeRuns,
			&item.MissingTimelineRuns,
			&item.MissingContextRuns,
			&item.ZeroScoreRuns,
			&item.LastPlayedAt,
			&item.LastIngestedAt,
		); err != nil {
			return AdminOverviewRecord{}, fmt.Errorf("scan admin user sync health: %w", err)
		}
		record.UserSyncHealth = append(record.UserSyncHealth, item)
	}
	if err := userHealthRows.Err(); err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("iterate admin user sync health: %w", err)
	}

	failureRows, err := s.pool.Query(ctx, `
		SELECT
			f.id,
			f.user_external_id,
			COALESCE(la.username, hu.external_id, ''),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id, '')),
			f.session_id,
			COALESCE(sr.public_run_id, ''),
			f.scenario_name,
			f.error_message,
			f.created_at
		FROM ingest_failures f
		LEFT JOIN hub_users hu ON LOWER(hu.external_id) = LOWER(f.user_external_id)
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		LEFT JOIN scenario_runs sr ON sr.user_id = hu.id
			AND (sr.source_session_id = f.session_id OR sr.session_id = f.session_id)
		ORDER BY created_at DESC
		LIMIT 50
	`)
	if err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("load admin ingest failures: %w", err)
	}
	defer failureRows.Close()
	for failureRows.Next() {
		var item AdminIngestFailure
		if err := failureRows.Scan(
			&item.ID,
			&item.UserExternalID,
			&item.UserHandle,
			&item.UserDisplayName,
			&item.SessionID,
			&item.PublicRunID,
			&item.ScenarioName,
			&item.ErrorMessage,
			&item.CreatedAt,
		); err != nil {
			return AdminOverviewRecord{}, fmt.Errorf("scan admin ingest failure: %w", err)
		}
		record.RecentFailures = append(record.RecentFailures, item)
	}
	if err := failureRows.Err(); err != nil {
		return AdminOverviewRecord{}, fmt.Errorf("iterate admin ingest failures: %w", err)
	}

	return record, nil
}

func (s *Store) GetAdminUserDetail(ctx context.Context, handle string, days int) (AdminUserDetailRecord, error) {
	handle = strings.TrimSpace(strings.ToLower(handle))
	if handle == "" {
		return AdminUserDetailRecord{}, fmt.Errorf("user handle is required")
	}

	var detail AdminUserDetailRecord
	var userID int64
	if err := s.pool.QueryRow(ctx, `
		SELECT
			hu.id,
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id))
		FROM hub_users hu
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
		   OR LOWER(hu.external_id) = $1
		LIMIT 1
	`, handle).Scan(&userID, &detail.UserHandle, &detail.UserDisplayName); err != nil {
		return AdminUserDetailRecord{}, fmt.Errorf("load admin user detail: %w", err)
	}

	userWhere, userArgs := adminUserPlayedAfterClause(days, 2)

	if err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::bigint,
			COUNT(DISTINCT sr.scenario_name)::bigint,
			COUNT(*) FILTER (WHERE COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown')::bigint,
			COUNT(*) FILTER (WHERE rt.session_id IS NULL)::bigint,
			COUNT(*) FILTER (WHERE rc.session_id IS NULL)::bigint,
			COUNT(*) FILTER (WHERE sr.score <= 0)::bigint,
			MAX(sr.played_at),
			MAX(sr.created_at)
		FROM scenario_runs sr
		LEFT JOIN (
			SELECT DISTINCT session_id
			FROM run_timeline_seconds
		) rt ON rt.session_id = sr.session_id
		LEFT JOIN (
			SELECT DISTINCT session_id
			FROM run_context_windows
		) rc ON rc.session_id = sr.session_id
		WHERE sr.user_id = $1
	`+userWhere, append([]any{userID}, userArgs...)...).Scan(
		&detail.RunCount,
		&detail.ScenarioCount,
		&detail.UnknownTypeRuns,
		&detail.MissingTimelineRuns,
		&detail.MissingContextRuns,
		&detail.ZeroScoreRuns,
		&detail.LastPlayedAt,
		&detail.LastIngestedAt,
	); err != nil {
		return AdminUserDetailRecord{}, fmt.Errorf("load admin user detail aggregates: %w", err)
	}

	unknownRows, err := s.pool.Query(ctx, `
		SELECT
			sr.scenario_name,
			COUNT(*)::bigint
		FROM scenario_runs sr
		WHERE sr.user_id = $1
		  AND COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown'
	`+userWhere+`
		GROUP BY sr.scenario_name
		ORDER BY COUNT(*) DESC, sr.scenario_name ASC
		LIMIT 12
	`, append([]any{userID}, userArgs...)...)
	if err != nil {
		return AdminUserDetailRecord{}, fmt.Errorf("load admin user unknown scenarios: %w", err)
	}
	defer unknownRows.Close()
	for unknownRows.Next() {
		var item AdminScenarioIssue
		if err := unknownRows.Scan(&item.ScenarioName, &item.RunCount); err != nil {
			return AdminUserDetailRecord{}, fmt.Errorf("scan admin user unknown scenario: %w", err)
		}
		item.ScenarioSlug = slugifyScenarioName(item.ScenarioName)
		detail.TopUnknownScenarios = append(detail.TopUnknownScenarios, item)
	}
	if err := unknownRows.Err(); err != nil {
		return AdminUserDetailRecord{}, fmt.Errorf("iterate admin user unknown scenarios: %w", err)
	}

	failureRows, err := s.pool.Query(ctx, `
		SELECT
			f.id,
			f.user_external_id,
			COALESCE(la.username, hu.external_id, ''),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id, '')),
			f.session_id,
			COALESCE(sr.public_run_id, ''),
			f.scenario_name,
			f.error_message,
			f.created_at
		FROM ingest_failures f
		JOIN hub_users hu ON LOWER(hu.external_id) = LOWER(f.user_external_id)
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		LEFT JOIN scenario_runs sr ON sr.user_id = hu.id
			AND (sr.source_session_id = f.session_id OR sr.session_id = f.session_id)
		WHERE LOWER(user_external_id) = (
			SELECT LOWER(external_id) FROM hub_users WHERE id = $1
		)
		ORDER BY created_at DESC
		LIMIT 20
	`, userID)
	if err != nil {
		return AdminUserDetailRecord{}, fmt.Errorf("load admin user failures: %w", err)
	}
	defer failureRows.Close()
	for failureRows.Next() {
		var item AdminIngestFailure
		if err := failureRows.Scan(
			&item.ID,
			&item.UserExternalID,
			&item.UserHandle,
			&item.UserDisplayName,
			&item.SessionID,
			&item.PublicRunID,
			&item.ScenarioName,
			&item.ErrorMessage,
			&item.CreatedAt,
		); err != nil {
			return AdminUserDetailRecord{}, fmt.Errorf("scan admin user failure: %w", err)
		}
		detail.RecentFailures = append(detail.RecentFailures, item)
	}
	if err := failureRows.Err(); err != nil {
		return AdminUserDetailRecord{}, fmt.Errorf("iterate admin user failures: %w", err)
	}

	runRows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(sr.public_run_id, sr.session_id),
			sr.scenario_name,
			COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown'),
			sr.played_at,
			sr.score,
			sr.accuracy,
			sr.duration_ms
		FROM scenario_runs sr
		WHERE sr.user_id = $1
	`+userWhere+`
		ORDER BY sr.played_at DESC, sr.created_at DESC
		LIMIT 20
	`, append([]any{userID}, userArgs...)...)
	if err != nil {
		return AdminUserDetailRecord{}, fmt.Errorf("load admin user recent runs: %w", err)
	}
	defer runRows.Close()
	for runRows.Next() {
		var item AdminUserRecentRun
		if err := runRows.Scan(
			&item.PublicRunID,
			&item.ScenarioName,
			&item.ScenarioType,
			&item.PlayedAt,
			&item.Score,
			&item.Accuracy,
			&item.DurationMS,
		); err != nil {
			return AdminUserDetailRecord{}, fmt.Errorf("scan admin user recent run: %w", err)
		}
		item.ScenarioSlug = slugifyScenarioName(item.ScenarioName)
		detail.RecentRuns = append(detail.RecentRuns, item)
	}
	if err := runRows.Err(); err != nil {
		return AdminUserDetailRecord{}, fmt.Errorf("iterate admin user recent runs: %w", err)
	}

	return detail, nil
}

func (s *Store) GetAdminUserMetricsExport(ctx context.Context, handle string, days int) (AdminUserMetricsExport, error) {
	handle = strings.TrimSpace(strings.ToLower(handle))
	if handle == "" {
		return AdminUserMetricsExport{}, fmt.Errorf("user handle is required")
	}

	detail, err := s.GetAdminUserDetail(ctx, handle, days)
	if err != nil {
		return AdminUserMetricsExport{}, err
	}

	var userID int64
	if err := s.pool.QueryRow(ctx, `
		SELECT hu.id
		FROM hub_users hu
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
		   OR LOWER(hu.external_id) = $1
		LIMIT 1
	`, handle).Scan(&userID); err != nil {
		return AdminUserMetricsExport{}, fmt.Errorf("load admin export user: %w", err)
	}

	userWhere, userArgs := adminUserPlayedAfterClause(days, 2)
	rows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(sr.public_run_id, sr.session_id),
			sr.session_id,
			COALESCE(sr.source_session_id, ''),
			sr.scenario_name,
			COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown'),
			sr.played_at,
			sr.score,
			sr.accuracy,
			sr.duration_ms,
			COALESCE(rs.app_version, ''),
			COALESCE(rs.schema_version, 0),
			COALESCE(rt.timeline_count, 0),
			COALESCE(rc.context_count, 0),
			COALESCE(rs.summary_json, '{}'::jsonb)::text,
			COALESCE(rf.feature_json, '{}'::jsonb)::text
		FROM scenario_runs sr
		LEFT JOIN run_summaries rs ON rs.session_id = sr.session_id
		LEFT JOIN run_feature_sets rf ON rf.session_id = sr.session_id
		LEFT JOIN (
			SELECT session_id, COUNT(*)::bigint AS timeline_count
			FROM run_timeline_seconds
			GROUP BY session_id
		) rt ON rt.session_id = sr.session_id
		LEFT JOIN (
			SELECT session_id, COUNT(*)::bigint AS context_count
			FROM run_context_windows
			GROUP BY session_id
		) rc ON rc.session_id = sr.session_id
		WHERE sr.user_id = $1
	`+userWhere+`
		ORDER BY sr.played_at DESC, sr.created_at DESC
	`, append([]any{userID}, userArgs...)...)
	if err != nil {
		return AdminUserMetricsExport{}, fmt.Errorf("load admin user metrics export runs: %w", err)
	}
	defer rows.Close()

	export := AdminUserMetricsExport{
		ExportedAt:      time.Now().UTC(),
		Days:            days,
		UserHandle:      detail.UserHandle,
		UserDisplayName: detail.UserDisplayName,
		RunCount:        detail.RunCount,
		RecentFailures:  detail.RecentFailures,
	}

	for rows.Next() {
		var item AdminUserMetricsExportRun
		var schemaVersion uint64
		var timelineCount uint64
		var contextCount uint64
		var summaryText string
		var featureText string
		if err := rows.Scan(
			&item.PublicRunID,
			&item.SessionID,
			&item.SourceSessionID,
			&item.ScenarioName,
			&item.ScenarioType,
			&item.PlayedAt,
			&item.Score,
			&item.Accuracy,
			&item.DurationMS,
			&item.AppVersion,
			&schemaVersion,
			&timelineCount,
			&contextCount,
			&summaryText,
			&featureText,
		); err != nil {
			return AdminUserMetricsExport{}, fmt.Errorf("scan admin user metrics export run: %w", err)
		}
		item.SchemaVersion = uint32(schemaVersion)
		item.TimelineSecondCount = timelineCount
		item.ContextWindowCount = contextCount
		item.SummaryJSON = json.RawMessage(summaryText)
		item.FeatureJSON = json.RawMessage(featureText)
		export.Runs = append(export.Runs, item)
	}
	if err := rows.Err(); err != nil {
		return AdminUserMetricsExport{}, fmt.Errorf("iterate admin user metrics export runs: %w", err)
	}

	return export, nil
}

func (s *Store) RepairScenarioTypes(ctx context.Context) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin repair scenario types transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var updated int64

	steps := []string{
		`
		UPDATE run_summaries
		SET summary_json = jsonb_set(
		    summary_json,
		    '{scenarioType}',
		    to_jsonb(
		        CASE summary_json->>'scenarioType'
		            WHEN 'OneShotClicking' THEN 'StaticClicking'
		            WHEN 'ReactiveClicking' THEN 'DynamicClicking'
		            WHEN 'MultiHitClicking' THEN 'TargetSwitching'
		            ELSE summary_json->>'scenarioType'
		        END
		    ),
		    true
		)
		WHERE summary_json ? 'scenarioType'
		  AND summary_json->>'scenarioType' IN ('OneShotClicking', 'ReactiveClicking', 'MultiHitClicking')
		`,
		`
		UPDATE scenario_runs
		SET scenario_type = CASE scenario_type
		    WHEN 'OneShotClicking' THEN 'StaticClicking'
		    WHEN 'ReactiveClicking' THEN 'DynamicClicking'
		    WHEN 'MultiHitClicking' THEN 'TargetSwitching'
		    ELSE scenario_type
		END,
		    updated_at = NOW()
		WHERE scenario_type IN ('OneShotClicking', 'ReactiveClicking', 'MultiHitClicking')
		`,
		`
		UPDATE scenario_runs sr
		SET scenario_type = CASE NULLIF(BTRIM(rs.summary_json->>'scenarioType'), '')
		    WHEN 'OneShotClicking' THEN 'StaticClicking'
		    WHEN 'ReactiveClicking' THEN 'DynamicClicking'
		    WHEN 'MultiHitClicking' THEN 'TargetSwitching'
		    ELSE NULLIF(BTRIM(rs.summary_json->>'scenarioType'), '')
		END,
		    updated_at = NOW()
		FROM run_summaries rs
		WHERE sr.session_id = rs.session_id
		  AND COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown'
		  AND NULLIF(BTRIM(rs.summary_json->>'scenarioType'), '') IS NOT NULL
		  AND BTRIM(rs.summary_json->>'scenarioType') <> 'Unknown'
		`,
		`
		WITH dominant AS (
			SELECT scenario_name, scenario_type
			FROM (
				SELECT
					scenario_name,
					scenario_type,
					ROW_NUMBER() OVER (
						PARTITION BY scenario_name
						ORDER BY COUNT(*) DESC, scenario_type ASC
					) AS rn
				FROM scenario_runs
				WHERE COALESCE(NULLIF(scenario_type, ''), 'Unknown') <> 'Unknown'
				GROUP BY scenario_name, scenario_type
			) ranked
			WHERE rn = 1
		)
		UPDATE scenario_runs sr
		SET scenario_type = dominant.scenario_type,
		    updated_at = NOW()
		FROM dominant
		WHERE sr.scenario_name = dominant.scenario_name
		  AND COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown'
		`,
		`
		UPDATE scenario_runs sr
		SET scenario_type = CASE
		    WHEN regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%targetswitch%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%switchingspheres%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%switchinghumanoid%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%controlts%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%eddiets%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%dotts%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%driftts%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%flyts%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%bouncets%'
		        THEN 'TargetSwitching'
		    WHEN regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%pasu%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%popcorn%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%airangelic%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%bounce%'
		        THEN 'DynamicClicking'
		    WHEN regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%variousstatic%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%clickingstatic%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%tilefrenzy%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%sixshot%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%1w%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%ww%'
		        THEN 'StaticClicking'
		    WHEN regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%tracking%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%whisphere%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%silo%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%smooth%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%controlsphere%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%centering%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%distancetrack%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%movementtracking%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%ground%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%pgt%'
		        THEN 'Tracking'
		    WHEN COALESCE((rs.summary_json->>'csvKills')::double precision, 0) > 0
		         AND COALESCE((rs.summary_json->>'csvDamageDone')::double precision, 0) > 0
		        THEN 'TargetSwitching'
		    WHEN COALESCE((rs.summary_json->>'csvKills')::double precision, 0) > 0
		         AND (
		            COALESCE((rs.summary_json->>'csvAvgTtk')::double precision, 0) >= 0.45
		            OR (
		                COALESCE((rs.summary_json->>'killsPerSecond')::double precision, 0) > 0
		                AND COALESCE((rs.summary_json->>'killsPerSecond')::double precision, 0) <= 2.25
		            )
		         )
		        THEN 'DynamicClicking'
		    WHEN COALESCE((rs.summary_json->>'csvKills')::double precision, 0) > 0
		        THEN 'StaticClicking'
		    ELSE 'Tracking'
		END,
		    updated_at = NOW()
		FROM run_summaries rs
		WHERE sr.session_id = rs.session_id
		  AND COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown'
		  AND (
		    LOWER(sr.scenario_name) LIKE '%track%'
		    OR LOWER(sr.scenario_name) LIKE '%sphere%'
		    OR LOWER(sr.scenario_name) LIKE '%smooth%'
		    OR LOWER(sr.scenario_name) LIKE '%air %'
		    OR LOWER(sr.scenario_name) LIKE 'air_%'
		    OR LOWER(sr.scenario_name) LIKE '%control%'
		    OR COALESCE((rs.summary_json->>'csvKills')::double precision, 0) > 0
		    OR COALESCE((rs.summary_json->>'csvAvgTtk')::double precision, 0) >= 5.0
		  )
		`,
	}

	for _, stmt := range steps {
		tag, execErr := tx.Exec(ctx, stmt)
		if execErr != nil {
			return 0, fmt.Errorf("repair scenario types: %w", execErr)
		}
		updated += tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit repair scenario types: %w", err)
	}
	return updated, nil
}

func (s *Store) RepairScenarioTypesForUser(ctx context.Context, handle string) (int64, error) {
	handle = strings.TrimSpace(strings.ToLower(handle))
	if handle == "" {
		return 0, fmt.Errorf("user handle is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin repair scenario types for user transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	resolveUserCTE := `
		WITH target_user AS (
			SELECT hu.id
			FROM hub_users hu
			LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
			WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
			   OR LOWER(hu.external_id) = $1
			LIMIT 1
		)
	`

	var updated int64
	steps := []string{
		resolveUserCTE + `
		UPDATE run_summaries rs
		SET summary_json = jsonb_set(
		    summary_json,
		    '{scenarioType}',
		    to_jsonb(
		        CASE summary_json->>'scenarioType'
		            WHEN 'OneShotClicking' THEN 'StaticClicking'
		            WHEN 'ReactiveClicking' THEN 'DynamicClicking'
		            WHEN 'MultiHitClicking' THEN 'TargetSwitching'
		            ELSE summary_json->>'scenarioType'
		        END
		    ),
		    true
		)
		FROM scenario_runs sr, target_user tu
		WHERE rs.session_id = sr.session_id
		  AND sr.user_id = tu.id
		  AND rs.summary_json ? 'scenarioType'
		  AND rs.summary_json->>'scenarioType' IN ('OneShotClicking', 'ReactiveClicking', 'MultiHitClicking')
		`,
		resolveUserCTE + `
		UPDATE scenario_runs
		SET scenario_type = CASE scenario_type
		    WHEN 'OneShotClicking' THEN 'StaticClicking'
		    WHEN 'ReactiveClicking' THEN 'DynamicClicking'
		    WHEN 'MultiHitClicking' THEN 'TargetSwitching'
		    ELSE scenario_type
		END,
		    updated_at = NOW()
		WHERE user_id = (SELECT id FROM target_user)
		  AND scenario_type IN ('OneShotClicking', 'ReactiveClicking', 'MultiHitClicking')
		`,
		resolveUserCTE + `
		UPDATE scenario_runs sr
		SET scenario_type = CASE NULLIF(BTRIM(rs.summary_json->>'scenarioType'), '')
		    WHEN 'OneShotClicking' THEN 'StaticClicking'
		    WHEN 'ReactiveClicking' THEN 'DynamicClicking'
		    WHEN 'MultiHitClicking' THEN 'TargetSwitching'
		    ELSE NULLIF(BTRIM(rs.summary_json->>'scenarioType'), '')
		END,
		    updated_at = NOW()
		FROM run_summaries rs, target_user tu
		WHERE sr.session_id = rs.session_id
		  AND sr.user_id = tu.id
		  AND COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown'
		  AND NULLIF(BTRIM(rs.summary_json->>'scenarioType'), '') IS NOT NULL
		  AND BTRIM(rs.summary_json->>'scenarioType') <> 'Unknown'
		`,
		resolveUserCTE + `
		, dominant AS (
			SELECT scenario_name, scenario_type
			FROM (
				SELECT
					scenario_name,
					scenario_type,
					ROW_NUMBER() OVER (
						PARTITION BY scenario_name
						ORDER BY COUNT(*) DESC, scenario_type ASC
					) AS rn
				FROM scenario_runs
				WHERE COALESCE(NULLIF(scenario_type, ''), 'Unknown') <> 'Unknown'
				GROUP BY scenario_name, scenario_type
			) ranked
			WHERE rn = 1
		)
		UPDATE scenario_runs sr
		SET scenario_type = dominant.scenario_type,
		    updated_at = NOW()
		FROM dominant, target_user tu
		WHERE sr.user_id = tu.id
		  AND sr.scenario_name = dominant.scenario_name
		  AND COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown'
		`,
		resolveUserCTE + `
		UPDATE scenario_runs sr
		SET scenario_type = CASE
		    WHEN regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%targetswitch%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%switchingspheres%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%switchinghumanoid%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%controlts%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%eddiets%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%dotts%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%driftts%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%flyts%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%bouncets%'
		        THEN 'TargetSwitching'
		    WHEN regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%pasu%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%popcorn%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%airangelic%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%bounce%'
		        THEN 'DynamicClicking'
		    WHEN regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%variousstatic%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%clickingstatic%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%tilefrenzy%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%sixshot%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%1w%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%ww%'
		        THEN 'StaticClicking'
		    WHEN regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%tracking%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%whisphere%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%silo%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%smooth%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%controlsphere%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%centering%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%distancetrack%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%movementtracking%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%ground%'
		         OR regexp_replace(lower(sr.scenario_name), '[^a-z0-9]+', '', 'g') LIKE '%pgt%'
		        THEN 'Tracking'
		    WHEN COALESCE((rs.summary_json->>'csvKills')::double precision, 0) > 0
		         AND COALESCE((rs.summary_json->>'csvDamageDone')::double precision, 0) > 0
		        THEN 'TargetSwitching'
		    WHEN COALESCE((rs.summary_json->>'csvKills')::double precision, 0) > 0
		         AND (
		            COALESCE((rs.summary_json->>'csvAvgTtk')::double precision, 0) >= 0.45
		            OR (
		                COALESCE((rs.summary_json->>'killsPerSecond')::double precision, 0) > 0
		                AND COALESCE((rs.summary_json->>'killsPerSecond')::double precision, 0) <= 2.25
		            )
		         )
		        THEN 'DynamicClicking'
		    WHEN COALESCE((rs.summary_json->>'csvKills')::double precision, 0) > 0
		        THEN 'StaticClicking'
		    ELSE 'Tracking'
		END,
		    updated_at = NOW()
		FROM run_summaries rs, target_user tu
		WHERE sr.session_id = rs.session_id
		  AND sr.user_id = tu.id
		  AND COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown'
		  AND (
		    LOWER(sr.scenario_name) LIKE '%track%'
		    OR LOWER(sr.scenario_name) LIKE '%sphere%'
		    OR LOWER(sr.scenario_name) LIKE '%smooth%'
		    OR LOWER(sr.scenario_name) LIKE '%air %'
		    OR LOWER(sr.scenario_name) LIKE 'air_%'
		    OR LOWER(sr.scenario_name) LIKE '%control%'
		    OR COALESCE((rs.summary_json->>'csvKills')::double precision, 0) > 0
		    OR COALESCE((rs.summary_json->>'csvAvgTtk')::double precision, 0) >= 5.0
		  )
		`,
	}

	for _, stmt := range steps {
		tag, execErr := tx.Exec(ctx, stmt, handle)
		if execErr != nil {
			return 0, fmt.Errorf("repair scenario types for user: %w", execErr)
		}
		updated += tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit repair scenario types for user: %w", err)
	}
	return updated, nil
}

func (s *Store) RepairRunMetrics(ctx context.Context, handle string) (int64, error) {
	args := []any{}
	userFilter := ""
	if trimmed := strings.TrimSpace(strings.ToLower(handle)); trimmed != "" {
		userFilter = `
		  AND sr.user_id = (
			SELECT hu.id
			FROM hub_users hu
			LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
			WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
			   OR LOWER(hu.external_id) = $1
			LIMIT 1
		  )
		`
		args = append(args, trimmed)
	}

	tag, err := s.pool.Exec(ctx, `
		WITH timeline_rollup AS (
			SELECT
				session_id,
				MAX(score) AS max_score,
				MAX(shots) AS max_shots,
				MAX(hits) AS max_hits
			FROM run_timeline_seconds
			GROUP BY session_id
		)
		UPDATE scenario_runs sr
		SET
			score = CASE
				WHEN sr.score > 0 THEN sr.score
				ELSE COALESCE(
					NULLIF((rs.summary_json->>'scoreTotal')::double precision, 0),
					NULLIF((rs.summary_json->>'scoreTotalDerived')::double precision, 0),
					NULLIF(tr.max_score, 0),
					NULLIF((rs.summary_json->>'csvScore')::double precision, 0),
					sr.score
				)
			END,
			accuracy = CASE
				WHEN sr.accuracy > 1.0 THEN sr.accuracy
				WHEN COALESCE(tr.max_shots, 0) > 0 THEN (tr.max_hits::double precision / tr.max_shots::double precision) * 100.0
				WHEN COALESCE(NULLIF((rs.summary_json->>'accuracyPct')::double precision, 0), 0) > 1.0 THEN (rs.summary_json->>'accuracyPct')::double precision
				WHEN COALESCE(NULLIF((rs.summary_json->>'panelAccuracyPct')::double precision, 0), 0) > 1.0 THEN (rs.summary_json->>'panelAccuracyPct')::double precision
				WHEN COALESCE(NULLIF((rs.summary_json->>'csvAccuracy')::double precision, 0), 0) > 1.0 THEN (rs.summary_json->>'csvAccuracy')::double precision
				WHEN COALESCE(NULLIF((rs.summary_json->>'csvAccuracy')::double precision, 0), 0) > 0 THEN ((rs.summary_json->>'csvAccuracy')::double precision) * 100.0
				ELSE sr.accuracy
			END,
			duration_ms = CASE
				WHEN sr.duration_ms > 0 THEN sr.duration_ms
				ELSE COALESCE(
					NULLIF(ROUND(COALESCE((rs.summary_json->>'csvDurationSecs')::double precision, 0) * 1000.0)::bigint, 0),
					sr.duration_ms
				)
			END,
			updated_at = NOW()
		FROM run_summaries rs
		LEFT JOIN timeline_rollup tr ON tr.session_id = rs.session_id
		WHERE sr.session_id = rs.session_id
		  AND (
			sr.score <= 0
			OR sr.accuracy <= 1.0
			OR sr.duration_ms <= 0
		  )
		  `+userFilter+`
	`, args...)
	if err != nil {
		return 0, fmt.Errorf("repair run metrics: %w", err)
	}
	return tag.RowsAffected(), nil
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
			hui.user_handle,
			hui.user_display_name
		FROM scenario_runs sr
		JOIN hub_user_identity hui ON hui.user_id = sr.user_id
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
			hui.user_handle,
			hui.user_display_name,
			hui.avatar_url,
			hui.is_verified,
			COUNT(sr.*)::bigint AS run_count,
			COUNT(DISTINCT sr.scenario_name)::bigint AS scenario_count,
			COALESCE((
				SELECT COALESCE(NULLIF(sr2.scenario_type, ''), 'Unknown')
				FROM scenario_runs sr2
				WHERE sr2.user_id = hui.user_id
				GROUP BY COALESCE(NULLIF(sr2.scenario_type, ''), 'Unknown')
				ORDER BY COUNT(*) DESC, COALESCE(NULLIF(sr2.scenario_type, ''), 'Unknown') ASC
				LIMIT 1
			), 'Unknown') AS primary_scenario_type
		FROM hub_user_identity hui
		JOIN scenario_runs sr ON sr.user_id = hui.user_id
		GROUP BY hui.user_id, hui.user_handle, hui.user_display_name, hui.avatar_url, hui.is_verified
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
			&profile.IsVerified,
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
			hui.user_handle,
			hui.user_display_name,
			hui.avatar_url,
			COALESCE(rs.summary_json::text, '{}'),
			COALESCE(rf.feature_json::text, '{}')
		FROM scenario_runs sr
		JOIN hub_user_identity hui ON hui.user_id = sr.user_id
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
			hui.user_id,
			hui.external_id,
			hui.user_handle,
			hui.user_display_name,
			hui.avatar_url,
			hui.is_verified
		FROM hub_user_identity hui
		WHERE LOWER(hui.user_handle) = $1
		   OR LOWER(hui.external_id) = $1
		LIMIT 1
	`, handle).Scan(
		&userID,
		&record.UserExternalID,
		&record.UserHandle,
		&record.UserDisplayName,
		&record.AvatarURL,
		&record.IsVerified,
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
			hui.user_handle,
			hui.user_display_name,
			hui.avatar_url,
			hui.is_verified,
			COUNT(sr.*)::bigint AS run_count,
			COUNT(DISTINCT sr.scenario_name)::bigint AS scenario_count,
			COALESCE((
				SELECT NULLIF(sr2.scenario_type, '')
				FROM scenario_runs sr2
				WHERE sr2.user_id = hui.user_id
				  AND NULLIF(sr2.scenario_type, '') IS NOT NULL
				GROUP BY sr2.scenario_type
				ORDER BY COUNT(*) DESC, sr2.scenario_type ASC
				LIMIT 1
			), '') AS primary_scenario_type
		FROM hub_user_identity hui
		JOIN scenario_runs sr ON sr.user_id = hui.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hui.user_id
		WHERE hui.user_handle ILIKE $1
		   OR hui.user_display_name ILIKE $1
		   OR hui.external_id ILIKE $1
		   OR la.username ILIKE $1
		   OR la.display_name ILIKE $1
		   OR la.provider_account_id ILIKE $1
		GROUP BY hui.user_id, hui.user_handle, hui.user_display_name, hui.avatar_url, hui.is_verified
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
			&item.IsVerified,
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
			hui.user_handle,
			hui.user_display_name,
			EXISTS(SELECT 1 FROM replay_media_assets rma WHERE rma.session_id = sr.session_id),
			EXISTS(SELECT 1 FROM run_mouse_paths rmp WHERE rmp.session_id = sr.session_id),
			COALESCE((SELECT rma.quality FROM replay_media_assets rma WHERE rma.session_id = sr.session_id LIMIT 1), '')
		FROM scenario_runs sr
		JOIN hub_user_identity hui ON hui.user_id = sr.user_id
		WHERE sr.scenario_name ILIKE $1
		   OR hui.user_handle ILIKE $1
		   OR hui.user_display_name ILIKE $1
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
			&item.HasVideo,
			&item.HasMousePath,
			&item.ReplayQuality,
		); err != nil {
			return SearchRecord{}, fmt.Errorf("scan search run: %w", err)
		}
		item.ScenarioSlug = slugifyScenarioName(item.ScenarioName)
		record.Runs = append(record.Runs, item)
	}
	if err := runRows.Err(); err != nil {
		return SearchRecord{}, fmt.Errorf("iterate search runs: %w", err)
	}

	replayRows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(sr.public_run_id, sr.session_id),
			sr.session_id,
			sr.scenario_name,
			sr.scenario_type,
			sr.played_at,
			sr.score,
			sr.accuracy,
			sr.duration_ms,
			hui.user_handle,
			hui.user_display_name,
			EXISTS(SELECT 1 FROM replay_media_assets rma WHERE rma.session_id = sr.session_id),
			EXISTS(SELECT 1 FROM run_mouse_paths rmp WHERE rmp.session_id = sr.session_id),
			COALESCE((SELECT rma.quality FROM replay_media_assets rma WHERE rma.session_id = sr.session_id LIMIT 1), '')
		FROM scenario_runs sr
		JOIN hub_user_identity hui ON hui.user_id = sr.user_id
		WHERE (
			EXISTS(SELECT 1 FROM replay_media_assets rma WHERE rma.session_id = sr.session_id)
			OR EXISTS(SELECT 1 FROM run_mouse_paths rmp WHERE rmp.session_id = sr.session_id)
		)
		  AND (
			sr.scenario_name ILIKE $1
			OR hui.user_handle ILIKE $1
			OR hui.user_display_name ILIKE $1
			OR sr.public_run_id ILIKE $1
			OR sr.session_id ILIKE $1
		)
		ORDER BY sr.played_at DESC, sr.created_at DESC
		LIMIT 20
	`, pattern)
	if err != nil {
		return SearchRecord{}, fmt.Errorf("search replays: %w", err)
	}
	defer replayRows.Close()
	for replayRows.Next() {
		var item SearchRunRecord
		if err := replayRows.Scan(
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
			&item.HasVideo,
			&item.HasMousePath,
			&item.ReplayQuality,
		); err != nil {
			return SearchRecord{}, fmt.Errorf("scan search replay: %w", err)
		}
		item.ScenarioSlug = slugifyScenarioName(item.ScenarioName)
		record.Replays = append(record.Replays, item)
	}
	if err := replayRows.Err(); err != nil {
		return SearchRecord{}, fmt.Errorf("iterate search replays: %w", err)
	}

	return record, nil
}

func (s *Store) ListReplays(
	ctx context.Context,
	query string,
	scenarioName string,
	userHandle string,
	limit int,
) (ReplayListRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	record := ReplayListRecord{
		Query:        strings.TrimSpace(query),
		ScenarioName: strings.TrimSpace(scenarioName),
		UserHandle:   strings.TrimSpace(userHandle),
		Items:        []SearchRunRecord{},
	}

	args := []any{}
	clauses := []string{
		`(
			EXISTS(SELECT 1 FROM replay_media_assets rma WHERE rma.session_id = sr.session_id)
			OR EXISTS(SELECT 1 FROM run_mouse_paths rmp WHERE rmp.session_id = sr.session_id)
		)`,
	}

	if record.Query != "" {
		args = append(args, "%"+record.Query+"%")
		idx := len(args)
		clauses = append(clauses, fmt.Sprintf(`(
			sr.scenario_name ILIKE $%d
			OR COALESCE(la.username, hu.external_id) ILIKE $%d
			OR COALESCE(NULLIF(la.display_name, ''), '') ILIKE $%d
			OR sr.public_run_id ILIKE $%d
			OR sr.session_id ILIKE $%d
		)`, idx, idx, idx, idx, idx))
	}

	if record.ScenarioName != "" {
		args = append(args, record.ScenarioName)
		clauses = append(clauses, fmt.Sprintf("sr.scenario_name = $%d", len(args)))
	}

	if record.UserHandle != "" {
		args = append(args, strings.ToLower(record.UserHandle))
		clauses = append(clauses, fmt.Sprintf("LOWER(COALESCE(la.username, hu.external_id)) = $%d", len(args)))
	}

	args = append(args, limit)
	rows, err := s.pool.Query(ctx, `
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
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id)),
			EXISTS(SELECT 1 FROM replay_media_assets rma WHERE rma.session_id = sr.session_id),
			EXISTS(SELECT 1 FROM run_mouse_paths rmp WHERE rmp.session_id = sr.session_id),
			COALESCE((SELECT rma.quality FROM replay_media_assets rma WHERE rma.session_id = sr.session_id LIMIT 1), '')
		FROM scenario_runs sr
		JOIN hub_users hu ON hu.id = sr.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE `+strings.Join(clauses, " AND ")+`
		ORDER BY sr.played_at DESC, sr.created_at DESC
		LIMIT $`+fmt.Sprintf("%d", len(args)), args...)
	if err != nil {
		return ReplayListRecord{}, fmt.Errorf("list replays: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item SearchRunRecord
		if err := rows.Scan(
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
			&item.HasVideo,
			&item.HasMousePath,
			&item.ReplayQuality,
		); err != nil {
			return ReplayListRecord{}, fmt.Errorf("scan replay listing: %w", err)
		}
		item.ScenarioSlug = slugifyScenarioName(item.ScenarioName)
		record.Items = append(record.Items, item)
	}
	if err := rows.Err(); err != nil {
		return ReplayListRecord{}, fmt.Errorf("iterate replay listing: %w", err)
	}

	return record, nil
}

func (s *Store) GetLeaderboard(ctx context.Context, scenarioType string) (LeaderboardRecord, error) {
	var result LeaderboardRecord
	result.Records = []*hubv1.RunPreview{}
	result.TopScores = []*hubv1.RunPreview{}

	// Records: best score per scenario (DISTINCT ON scenario_name)
	recordRows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (sr.scenario_name)
			sr.scenario_name, sr.scenario_type, sr.score, sr.accuracy,
			sr.duration_ms, sr.played_at,
			COALESCE(sr.public_run_id, sr.session_id), sr.session_id,
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id))
		FROM scenario_runs sr
		JOIN hub_users hu ON hu.id = sr.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE ($1 = '' OR sr.scenario_type = $1)
		ORDER BY sr.scenario_name, sr.score DESC
	`, scenarioType)
	if err != nil {
		return result, err
	}
	defer recordRows.Close()
	for recordRows.Next() {
		var rp hubv1.RunPreview
		var durationMs int64
		var playedAt time.Time
		if err := recordRows.Scan(
			&rp.ScenarioName, &rp.ScenarioType, &rp.Score, &rp.Accuracy,
			&durationMs, &playedAt,
			&rp.RunId, &rp.SessionId,
			&rp.UserHandle, &rp.UserDisplayName,
		); err != nil {
			continue
		}
		rp.DurationMs = uint64(durationMs)
		rp.PlayedAtIso = playedAt.UTC().Format(time.RFC3339)
		result.Records = append(result.Records, &rp)
	}

	// Top scores: overall top 100, ordered by score DESC
	topRows, err := s.pool.Query(ctx, `
		SELECT
			sr.scenario_name, sr.scenario_type, sr.score, sr.accuracy,
			sr.duration_ms, sr.played_at,
			COALESCE(sr.public_run_id, sr.session_id), sr.session_id,
			COALESCE(la.username, hu.external_id),
			COALESCE(NULLIF(la.display_name, ''), COALESCE(la.username, hu.external_id))
		FROM scenario_runs sr
		JOIN hub_users hu ON hu.id = sr.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE ($1 = '' OR sr.scenario_type = $1)
		ORDER BY sr.score DESC
		LIMIT 100
	`, scenarioType)
	if err != nil {
		return result, err
	}
	defer topRows.Close()
	for topRows.Next() {
		var rp hubv1.RunPreview
		var durationMs int64
		var playedAt time.Time
		if err := topRows.Scan(
			&rp.ScenarioName, &rp.ScenarioType, &rp.Score, &rp.Accuracy,
			&durationMs, &playedAt,
			&rp.RunId, &rp.SessionId,
			&rp.UserHandle, &rp.UserDisplayName,
		); err != nil {
			continue
		}
		rp.DurationMs = uint64(durationMs)
		rp.PlayedAtIso = playedAt.UTC().Format(time.RFC3339)
		result.TopScores = append(result.TopScores, &rp)
	}

	return result, nil
}

func (s *Store) GetPlayerScenarioHistory(ctx context.Context, handle, slug string) (PlayerScenarioHistoryRecord, error) {
	var result PlayerScenarioHistoryRecord
	result.Runs = []*hubv1.RunPreview{}
	resolvedUser, err := s.resolveUserIdentityByHandle(ctx, handle)
	if err != nil {
		return result, err
	}

	// Resolve scenario name from slug
	nameRows, err := s.pool.Query(ctx, `
		SELECT DISTINCT scenario_name
		FROM scenario_runs
		ORDER BY scenario_name ASC
	`)
	if err != nil {
		return result, err
	}
	defer nameRows.Close()
	scenarioName := ""
	for nameRows.Next() {
		var candidate string
		if err := nameRows.Scan(&candidate); err != nil {
			continue
		}
		if slugifyScenarioName(candidate) == slug {
			scenarioName = candidate
			break
		}
	}
	if scenarioName == "" {
		return result, fmt.Errorf("scenario not found")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			sr.scenario_name, sr.scenario_type,
			sr.score, sr.accuracy, sr.duration_ms, sr.played_at,
			COALESCE(sr.public_run_id, sr.session_id), sr.session_id
		FROM scenario_runs sr
		WHERE sr.user_id = $1
		  AND sr.scenario_name = $2
		ORDER BY sr.played_at ASC
		LIMIT 500
	`, resolvedUser.UserID, scenarioName)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	var totalScore, totalAccuracy float64
	for rows.Next() {
		var rp hubv1.RunPreview
		var durationMs int64
		var playedAt time.Time
		if err := rows.Scan(
			&rp.ScenarioName, &rp.ScenarioType,
			&rp.Score, &rp.Accuracy, &durationMs, &playedAt,
			&rp.RunId, &rp.SessionId,
		); err != nil {
			continue
		}
		rp.UserHandle = resolvedUser.UserHandle
		rp.UserDisplayName = resolvedUser.UserDisplayName
		rp.DurationMs = uint64(durationMs)
		rp.PlayedAtIso = playedAt.UTC().Format(time.RFC3339)
		result.Runs = append(result.Runs, &rp)
		if result.ScenarioName == "" {
			result.ScenarioName = rp.ScenarioName
			result.ScenarioSlug = slug
			result.ScenarioType = rp.ScenarioType
		}
		if rp.Score > result.BestScore {
			result.BestScore = rp.Score
		}
		if rp.Accuracy > result.BestAccuracy {
			result.BestAccuracy = rp.Accuracy
		}
		totalScore += rp.Score
		totalAccuracy += rp.Accuracy
	}
	result.RunCount = int32(len(result.Runs))
	if result.RunCount > 0 {
		result.AverageScore = totalScore / float64(result.RunCount)
		result.AverageAccuracy = totalAccuracy / float64(result.RunCount)
	}
	return result, nil
}

type AimProfileRecord struct {
	UserHandle        string
	UserDisplayName   string
	TypeBands         []*hubv1.TypeProfileBand
	OverallAccuracy   float64
	OverallPercentile float64
	TotalRunCount     int32
	StrongestType     string
	MostPracticedType string
}

func (s *Store) GetAimProfile(ctx context.Context, handle string) (AimProfileRecord, error) {
	var result AimProfileRecord
	result.TypeBands = []*hubv1.TypeProfileBand{}
	resolvedUser, err := s.resolveUserIdentityByHandle(ctx, handle)
	if err != nil {
		return result, err
	}
	result.UserHandle = resolvedUser.UserHandle
	result.UserDisplayName = resolvedUser.UserDisplayName

	// Per-type stats for the player
	rows, err := s.pool.Query(ctx, `
		WITH player_stats AS (
			SELECT
				sr.scenario_type,
				COUNT(*) AS run_count,
				AVG(sr.accuracy) AS avg_accuracy,
				AVG(sr.score) AS avg_score,
				MAX(sr.score) AS best_score
			FROM scenario_runs sr
			WHERE sr.user_id = $1
			  AND sr.scenario_type NOT IN ('', 'Unknown')
			GROUP BY sr.scenario_type
		),
		community_stats AS (
			SELECT
				scenario_type,
				COUNT(*) AS total_runs,
				AVG(accuracy) AS community_avg_accuracy,
				AVG(score) AS community_avg_score
			FROM scenario_runs
			WHERE scenario_type NOT IN ('', 'Unknown')
			GROUP BY scenario_type
		)
		SELECT
			p.scenario_type,
			p.run_count,
			p.avg_accuracy,
			p.avg_score,
			p.best_score,
			c.community_avg_accuracy,
			c.community_avg_score,
			(
				SELECT COUNT(*) * 100.0 / NULLIF(c.total_runs, 0)
				FROM scenario_runs r
				WHERE r.scenario_type = p.scenario_type
				  AND r.accuracy < p.avg_accuracy
			) AS accuracy_percentile
		FROM player_stats p
		JOIN community_stats c ON c.scenario_type = p.scenario_type
		ORDER BY p.run_count DESC
	`, resolvedUser.UserID)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	var totalRuns int32
	var totalAccWeighted float64
	var bestPct float64
	var bestPctType string
	var mostRunsType string
	var mostRuns int32

	for rows.Next() {
		var band hubv1.TypeProfileBand
		var pct float64
		if err := rows.Scan(
			&band.ScenarioType,
			&band.RunCount,
			&band.AvgAccuracy,
			&band.AvgScore,
			&band.BestScore,
			&band.CommunityAvgAccuracy,
			&band.CommunityAvgScore,
			&pct,
		); err != nil {
			continue
		}
		band.AccuracyPercentile = pct
		result.TypeBands = append(result.TypeBands, &band)
		totalRuns += band.RunCount
		totalAccWeighted += band.AvgAccuracy * float64(band.RunCount)
		if pct > bestPct {
			bestPct = pct
			bestPctType = band.ScenarioType
		}
		if band.RunCount > mostRuns {
			mostRuns = band.RunCount
			mostRunsType = band.ScenarioType
		}
	}

	result.TotalRunCount = totalRuns
	if totalRuns > 0 {
		result.OverallAccuracy = totalAccWeighted / float64(totalRuns)
	}
	result.StrongestType = bestPctType
	result.MostPracticedType = mostRunsType

	// Overall percentile: what fraction of all runs have lower accuracy than player's overall avg
	if result.OverallAccuracy > 0 {
		err = s.pool.QueryRow(ctx, `
			SELECT COUNT(*) * 100.0 / NULLIF((SELECT COUNT(*) FROM scenario_runs WHERE scenario_type NOT IN ('', 'Unknown')), 0)
			FROM scenario_runs
			WHERE scenario_type NOT IN ('', 'Unknown')
			  AND accuracy < $1
		`, result.OverallAccuracy).Scan(&result.OverallPercentile)
		if err != nil {
			result.OverallPercentile = 0
		}
	}

	// Fetch avg smoothness per type from feature_json JSONB
	smoothRows, err := s.pool.Query(ctx, `
		SELECT
			sr.scenario_type,
			AVG((rfs.feature_json->>'smoothnessComposite')::float) AS avg_smoothness
		FROM scenario_runs sr
		JOIN run_feature_sets rfs ON rfs.session_id = sr.session_id
		WHERE sr.user_id = $1
		  AND sr.scenario_type NOT IN ('', 'Unknown')
		  AND rfs.feature_json->>'smoothnessComposite' IS NOT NULL
		  AND (rfs.feature_json->>'smoothnessComposite')::float > 0
		GROUP BY sr.scenario_type
	`, resolvedUser.UserID)
	if err == nil {
		defer smoothRows.Close()
		smoothMap := map[string]float64{}
		for smoothRows.Next() {
			var t string
			var v float64
			if smoothRows.Scan(&t, &v) == nil {
				smoothMap[t] = v
			}
		}
		for _, band := range result.TypeBands {
			if v, ok := smoothMap[band.ScenarioType]; ok {
				band.AvgSmoothness = v
			}
		}
	}

	return result, nil
}

// ── Aim Fingerprint ────────────────────────────────────────────────────────────

type fpDist struct{ median, p25, p75 float64 }

func fpPercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted))*p/100+0.9999) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func fpDist3(vals []float64) fpDist {
	if len(vals) == 0 {
		return fpDist{}
	}
	s2 := make([]float64, len(vals))
	copy(s2, vals)
	sort.Float64s(s2)
	return fpDist{
		median: fpPercentile(s2, 50),
		p25:    fpPercentile(s2, 25),
		p75:    fpPercentile(s2, 75),
	}
}

func fpClamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func fpScale(v, mn, mx float64) float64 {
	if mx <= mn {
		return 0
	}
	return fpClamp((v-mn)/(mx-mn)*100, 0, 100)
}

func fpInverse(v, good, bad float64) float64 { return 100 - fpScale(v, good, bad) }

func fpVolatility(iqr, span float64) float64 {
	if span <= 0 {
		return 0
	}
	return fpClamp((iqr/span)*100, 0, 100)
}

func fpWeighted(parts [][2]float64) int32 {
	total, weighted := 0.0, 0.0
	for _, p := range parts {
		total += p[1]
		weighted += p[0] * p[1]
	}
	if total <= 0 {
		return 0
	}
	return int32(fpClamp(weighted/total+0.5, 0, 100))
}

func fpMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func fpRound(v float64) int32 { return int32(fpClamp(v+0.5, 0, 100)) }

func isTrackingType(t string) bool {
	return t == "PureTracking" || strings.Contains(t, "Tracking")
}

func classifyAimStyle(precision, speed, control, consistency, decisiveness, rhythm int32, isTracking bool) (name, tagline, color, description, focus string) {
	if isTracking {
		switch {
		case precision > 70 && consistency > 70 && rhythm > 70:
			return "The Rail", "Locked on and flowing", "#00f5a0",
				"Your tracking is smooth, consistent, and precise — you stay on target with minimal wobble and even speed.",
				"Faster target variants, smaller hitbox scenarios, long-session endurance"
		case speed > 65 && consistency < 50:
			return "The Sprinter", "Fast but choppy", "#ff6b6b",
				"You can keep up with fast targets but your speed is uneven — you accelerate and decelerate in bursts instead of flowing continuously.",
				"Smooth-tracking drills, large target slow-tracking, constant-speed follow scenarios"
		case control > 70 && precision > 65 && rhythm > 60:
			return "The Orbiter", "Smooth and controlled", "#00b4ff",
				"You maintain clean, controlled contact with targets and rarely overshoot. Your movement flows well.",
				"Speed-ramp drills, reactive tracking, target-leading practice"
		case speed > 60 && control > 55 && decisiveness > 60:
			return "The Overtaker", "Aggressive and reactive", "#ffd700",
				"You chase targets hard and react fast — your instincts are sharp.",
				"Strafing target scenarios, smooth acceleration drills, reduce overcorrections"
		case consistency > 65 && speed < 40:
			return "The Anchor", "Steady but slow", "#a78bfa",
				"Your tracking is mechanically consistent and clean, but you struggle when targets accelerate or change direction.",
				"Dynamic tracking scenarios, speed-increasing variants, reaction-based targets"
		default:
			return "The Foundation Builder", "Building tracking fundamentals", "#ffd700",
				"Your tracking mechanics are still developing. Focus on staying on target continuously and matching target speed evenly.",
				"Beginner tracking scenarios, large slow targets, smooth-follow drills"
		}
	}
	switch {
	case speed > 65 && control < 40:
		return "The Aggressor", "Raw speed, needs refinement", "#ff6b6b",
			"You move fast and commit hard, but overshoot often. Your instincts are strong — channel that aggression into deliberate deceleration.",
			"Deceleration drills, close-range flick scenarios, overshooting correction"
	case precision > 70 && control > 65 && speed < 50:
		return "The Surgeon", "Clean and controlled", "#00f5a0",
			"Your mouse movement is exceptionally clean. You rarely miss, but you're playing conservatively. Match that precision at higher speed.",
			"Reactive scenarios, tempo drills, increasing flick distance"
	case consistency > 70 && rhythm > 70:
		return "The Metronome", "Mechanically reliable", "#00b4ff",
			"Extremely consistent mechanics with a reliable click rhythm. This repeatability is your foundation.",
			"Difficulty escalation, novel scenario types to raise your ceiling"
	case decisiveness > 70 && precision < 55:
		return "The Gambler", "Confident but imprecise", "#ffd700",
			"You commit fast and trust your instincts — great for reaction time. But shots sometimes fire before fully acquiring the target.",
			"Micro-adjustment training, precision clicking, accuracy-first drills"
	case precision > 65 && consistency > 65:
		return "The Technician", "Solid all-around mechanics", "#a78bfa",
			"A well-rounded, technically sound aimer with strong precision and consistency.",
			"Reactive flick scenarios, head-tracking, increasing pace"
	default:
		return "The Foundation Builder", "Developing core mechanics", "#ffd700",
			"Your aim style is still taking shape. Focus on fundamentals: reduce jitter, clean up movement paths, and build consistent click timing.",
			"Tracking basics, precision clicking, click timing trainers"
	}
}

func (s *Store) GetAimFingerprint(ctx context.Context, handle string) (*hubv1.AimFingerprint, error) {
	// Validate user exists
	resolvedUser, err := s.resolveUserIdentityByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}

	// Fetch smoothness fields for recent sessions (up to 300)
	rows, err := s.pool.Query(ctx, `
		SELECT
			(rfs.feature_json->>'smoothnessJitter')::float,
			(rfs.feature_json->>'smoothnessOvershootRate')::float,
			(rfs.feature_json->>'smoothnessVelocityStd')::float,
			(rfs.feature_json->>'smoothnessPathEfficiency')::float,
			(rfs.feature_json->>'smoothnessAvgSpeed')::float,
			(rfs.feature_json->>'smoothnessClickTimingCv')::float,
			(rfs.feature_json->>'smoothnessCorrectionRatio')::float,
			(rfs.feature_json->>'smoothnessDirectionalBias')::float,
			sr.scenario_type
		FROM scenario_runs sr
		JOIN run_feature_sets rfs ON rfs.session_id = sr.session_id
		WHERE sr.user_id = $1
		  AND rfs.feature_json->>'smoothnessJitter' IS NOT NULL
		ORDER BY sr.played_at DESC
		LIMIT 300
	`, resolvedUser.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type row struct {
		jitter, overshootRate, velocityStd, pathEfficiency        float64
		avgSpeed, clickTimingCv, correctionRatio, directionalBias float64
		scenarioType                                              string
	}

	var allRows []row
	typeCounts := map[string]int{}
	for rows.Next() {
		var r row
		if err := rows.Scan(
			&r.jitter, &r.overshootRate, &r.velocityStd, &r.pathEfficiency,
			&r.avgSpeed, &r.clickTimingCv, &r.correctionRatio, &r.directionalBias,
			&r.scenarioType,
		); err != nil {
			continue
		}
		allRows = append(allRows, r)
		if r.scenarioType != "" && r.scenarioType != "Unknown" {
			typeCounts[r.scenarioType]++
		}
	}

	if len(allRows) == 0 {
		return nil, fmt.Errorf("no smoothness data for %s", handle)
	}

	// Dominant scenario type
	dominantType := "Unknown"
	bestCount := 0
	for t, c := range typeCounts {
		if c > bestCount {
			bestCount = c
			dominantType = t
		}
	}
	tracking := isTrackingType(dominantType)

	// Build metric slices
	jitterVals := make([]float64, len(allRows))
	overshootVals := make([]float64, len(allRows))
	velStdVals := make([]float64, len(allRows))
	pathEffVals := make([]float64, len(allRows))
	avgSpeedVals := make([]float64, len(allRows))
	clickCVVals := make([]float64, len(allRows))
	correctionVals := make([]float64, len(allRows))
	dirBiasVals := make([]float64, len(allRows))
	for i, r := range allRows {
		jitterVals[i] = r.jitter
		overshootVals[i] = r.overshootRate
		velStdVals[i] = r.velocityStd
		pathEffVals[i] = r.pathEfficiency
		avgSpeedVals[i] = r.avgSpeed
		clickCVVals[i] = r.clickTimingCv
		correctionVals[i] = r.correctionRatio
		dirBiasVals[i] = r.directionalBias
	}

	jitter := fpDist3(jitterVals)
	overshoot := fpDist3(overshootVals)
	velStd := fpDist3(velStdVals)
	pathEff := fpDist3(pathEffVals)
	avgSpd := fpDist3(avgSpeedVals)
	clickCV := fpDist3(clickCVVals)
	correction := fpDist3(correctionVals)
	dirBias := fpDist3(dirBiasVals)

	// Compute axis scores
	precision := fpWeighted([][2]float64{
		{fpScale(pathEff.median, 0.86, 0.985), 0.65},
		{fpInverse(jitter.p75, 0.14, 0.45), 0.35},
	})
	speedMin, speedMax := 450.0, 2300.0
	if tracking {
		speedMin, speedMax = 650.0, 2600.0
	}
	speed := fpRound(fpScale(avgSpd.median, speedMin, speedMax))
	control := fpWeighted([][2]float64{
		{fpInverse(fpMax(overshoot.median, overshoot.p75*0.75), 0.00005, 0.0045), 0.4},
		{fpInverse(fpMax(correction.median, correction.p75*0.85), 0.10, 0.42), 0.4},
		{fpScale(pathEff.median, 0.88, 0.98), 0.15},
		{fpInverse(fpMax(dirBias.median, dirBias.p75*0.8), 0.0, 0.08), 0.05},
	})
	consistency := fpWeighted([][2]float64{
		{fpInverse(fpMax(velStd.median, velStd.p75*0.85), 0.18, 0.9), 0.8},
		{fpInverse(jitter.median, 0.12, 0.42), 0.2},
	})
	decisiveness := fpWeighted([][2]float64{
		{fpInverse(fpMax(correction.median, correction.p75*0.8), 0.08, 0.38), 0.85},
		{fpInverse(fpMax(dirBias.median, dirBias.p75), 0.0, 0.08), 0.15},
	})
	var rhythm int32
	if tracking {
		rhythm = fpWeighted([][2]float64{
			{fpInverse(fpMax(velStd.median, velStd.p75), 0.18, 0.95), 0.7},
			{fpInverse(jitter.median, 0.12, 0.42), 0.3},
		})
	} else {
		rhythm = fpWeighted([][2]float64{
			{fpInverse(fpMax(clickCV.median, clickCV.p75*0.9), 0.03, 0.28), 0.8},
			{fpInverse(fpMax(correction.median, correction.p75*0.85), 0.08, 0.4), 0.2},
		})
	}

	rhythmLabel := "Rhythm"
	if tracking {
		rhythmLabel = "Flow"
	}

	// Volatility per axis
	precisionVol := int32((fpVolatility(jitter.p75-jitter.p25, 0.18) + fpVolatility(pathEff.p75-pathEff.p25, 0.2)) / 2)
	speedVol := int32(fpVolatility(avgSpd.p75-avgSpd.p25, 420))
	controlVol := int32((fpVolatility(overshoot.p75-overshoot.p25, 0.0045) + fpVolatility(correction.p75-correction.p25, 0.22) + fpVolatility(dirBias.p75-dirBias.p25, 0.08)) / 3)
	consistencyVol := int32(fpVolatility(velStd.p75-velStd.p25, 0.24))
	decisiveVol := int32((fpVolatility(correction.p75-correction.p25, 0.22) + fpVolatility(dirBias.p75-dirBias.p25, 0.08)) / 2)
	rhythmIQR := velStd.p75 - velStd.p25
	rhythmSpan := 0.24
	if !tracking {
		rhythmIQR = clickCV.p75 - clickCV.p25
		rhythmSpan = 0.3
	}
	rhythmVol := int32(fpVolatility(rhythmIQR, rhythmSpan))

	axes := []*hubv1.AimFingerprintAxis{
		{Key: "precision", Label: "Precision", Value: precision, Volatility: precisionVol},
		{Key: "speed", Label: "Speed", Value: speed, Volatility: speedVol},
		{Key: "control", Label: "Control", Value: control, Volatility: controlVol},
		{Key: "consistency", Label: "Consistency", Value: consistency, Volatility: consistencyVol},
		{Key: "decisiveness", Label: "Decisiveness", Value: decisiveness, Volatility: decisiveVol},
		{Key: "rhythm", Label: rhythmLabel, Value: rhythm, Volatility: rhythmVol},
	}

	styleName, styleTagline, styleColor, styleDesc, styleFocus := classifyAimStyle(
		precision, speed, control, consistency, decisiveness, rhythm, tracking,
	)

	return &hubv1.AimFingerprint{
		Precision:            precision,
		Speed:                speed,
		Control:              control,
		Consistency:          consistency,
		Decisiveness:         decisiveness,
		Rhythm:               rhythm,
		RhythmLabel:          rhythmLabel,
		SessionCount:         int32(len(allRows)),
		Axes:                 axes,
		StyleName:            styleName,
		StyleTagline:         styleTagline,
		StyleColor:           styleColor,
		StyleDescription:     styleDesc,
		StyleFocus:           styleFocus,
		DominantScenarioType: dominantType,
	}, nil
}

func (s *Store) ReclassifyTracking(ctx context.Context) (pgconn.CommandTag, error) {
	if _, err := s.pool.Exec(ctx, `
	    UPDATE run_summaries
	    SET summary_json = jsonb_set(
	        summary_json,
	        '{scenarioType}',
	        to_jsonb(
	            CASE summary_json->>'scenarioType'
	                WHEN 'OneShotClicking' THEN 'StaticClicking'
	                WHEN 'ReactiveClicking' THEN 'DynamicClicking'
	                WHEN 'MultiHitClicking' THEN 'TargetSwitching'
	                ELSE summary_json->>'scenarioType'
	            END
	        ),
	        true
	    )
	    WHERE summary_json ? 'scenarioType'
	      AND summary_json->>'scenarioType' IN ('OneShotClicking', 'ReactiveClicking', 'MultiHitClicking')
	`); err != nil {
		return pgconn.CommandTag{}, err
	}

	if _, err := s.pool.Exec(ctx, `
	    UPDATE scenario_runs
	    SET scenario_type = CASE scenario_type
	        WHEN 'OneShotClicking' THEN 'StaticClicking'
	        WHEN 'ReactiveClicking' THEN 'DynamicClicking'
	        WHEN 'MultiHitClicking' THEN 'TargetSwitching'
	        ELSE scenario_type
	    END,
	    updated_at = NOW()
	    WHERE scenario_type IN ('OneShotClicking', 'ReactiveClicking', 'MultiHitClicking')
	`); err != nil {
		return pgconn.CommandTag{}, err
	}

	if _, err := s.pool.Exec(ctx, `
	    UPDATE scenario_runs sr
	    SET scenario_type = CASE NULLIF(rs.summary_json->>'scenarioType', '')
	        WHEN 'OneShotClicking' THEN 'StaticClicking'
	        WHEN 'ReactiveClicking' THEN 'DynamicClicking'
	        WHEN 'MultiHitClicking' THEN 'TargetSwitching'
	        ELSE NULLIF(rs.summary_json->>'scenarioType', '')
	    END,
	        updated_at = NOW()
	    FROM run_summaries rs
	    WHERE sr.session_id = rs.session_id
	      AND sr.scenario_type IN ('Unknown', '')
	      AND NULLIF(rs.summary_json->>'scenarioType', '') IS NOT NULL
	      AND rs.summary_json->>'scenarioType' <> 'Unknown'
	`); err != nil {
		return pgconn.CommandTag{}, err
	}

	if _, err := s.pool.Exec(ctx, `
	    WITH dominant AS (
	        SELECT scenario_name, scenario_type
	        FROM (
	            SELECT
	                scenario_name,
	                scenario_type,
	                ROW_NUMBER() OVER (
	                    PARTITION BY scenario_name
	                    ORDER BY COUNT(*) DESC, scenario_type ASC
	                ) AS rank
	            FROM scenario_runs
	            WHERE COALESCE(NULLIF(scenario_type, ''), 'Unknown') <> 'Unknown'
	            GROUP BY scenario_name, scenario_type
	        ) ranked
	        WHERE rank = 1
	    )
	    UPDATE scenario_runs sr
	    SET scenario_type = dominant.scenario_type,
	        updated_at = NOW()
	    FROM dominant
	    WHERE sr.scenario_name = dominant.scenario_name
	      AND COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown'
	`); err != nil {
		return pgconn.CommandTag{}, err
	}

	return s.pool.Exec(ctx, `
	    UPDATE scenario_runs sr
	    SET scenario_type = CASE
	        WHEN COALESCE((rs.summary_json->>'csvKills')::float, 0) > 0
	             AND COALESCE((rs.summary_json->>'csvDamageDone')::float, 0) > 0
	            THEN 'TargetSwitching'
	        WHEN COALESCE((rs.summary_json->>'csvKills')::float, 0) > 0
	             AND (
	                COALESCE((rs.summary_json->>'csvAvgTtk')::float, 0) >= 0.45
	                OR (
	                    COALESCE((rs.summary_json->>'killsPerSecond')::float, 0) > 0
	                    AND COALESCE((rs.summary_json->>'killsPerSecond')::float, 0) <= 2.25
	                )
	             )
	            THEN 'DynamicClicking'
	        WHEN COALESCE((rs.summary_json->>'csvKills')::float, 0) > 0
	            THEN 'StaticClicking'
	        ELSE 'Tracking'
	    END,
	        updated_at = NOW()
	    FROM run_summaries rs
	    WHERE sr.session_id = rs.session_id
	      AND sr.scenario_type IN ('Unknown', '')
	      AND (
	        COALESCE((rs.summary_json->>'csvKills')::float, 0) > 0
	        OR (rs.summary_json->>'csvAvgTtk')::float >= 5.0
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

func (s *Store) ReclassifyTrackingForUser(ctx context.Context, handle string) (pgconn.CommandTag, error) {
	handle = strings.TrimSpace(strings.ToLower(handle))
	if handle == "" {
		return pgconn.CommandTag{}, fmt.Errorf("user handle is required")
	}

	if _, err := s.pool.Exec(ctx, `
	    UPDATE run_summaries rs
	    SET summary_json = jsonb_set(
	        summary_json,
	        '{scenarioType}',
	        to_jsonb(
	            CASE summary_json->>'scenarioType'
	                WHEN 'OneShotClicking' THEN 'StaticClicking'
	                WHEN 'ReactiveClicking' THEN 'DynamicClicking'
	                WHEN 'MultiHitClicking' THEN 'TargetSwitching'
	                ELSE summary_json->>'scenarioType'
	            END
	        ),
	        true
	    )
	    FROM scenario_runs sr
	    WHERE rs.session_id = sr.session_id
	      AND sr.user_id = (
	        SELECT hu.id
	        FROM hub_users hu
	        LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
	        WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
	           OR LOWER(hu.external_id) = $1
	        LIMIT 1
	      )
	      AND rs.summary_json ? 'scenarioType'
	      AND rs.summary_json->>'scenarioType' IN ('OneShotClicking', 'ReactiveClicking', 'MultiHitClicking')
	`, handle); err != nil {
		return pgconn.CommandTag{}, err
	}

	if _, err := s.pool.Exec(ctx, `
	    UPDATE scenario_runs
	    SET scenario_type = CASE scenario_type
	        WHEN 'OneShotClicking' THEN 'StaticClicking'
	        WHEN 'ReactiveClicking' THEN 'DynamicClicking'
	        WHEN 'MultiHitClicking' THEN 'TargetSwitching'
	        ELSE scenario_type
	    END,
	    updated_at = NOW()
	    WHERE user_id = (
	        SELECT hu.id
	        FROM hub_users hu
	        LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
	        WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
	           OR LOWER(hu.external_id) = $1
	        LIMIT 1
	    )
	      AND scenario_type IN ('OneShotClicking', 'ReactiveClicking', 'MultiHitClicking')
	`, handle); err != nil {
		return pgconn.CommandTag{}, err
	}

	if _, err := s.pool.Exec(ctx, `
	    UPDATE scenario_runs sr
	    SET scenario_type = CASE NULLIF(rs.summary_json->>'scenarioType', '')
	        WHEN 'OneShotClicking' THEN 'StaticClicking'
	        WHEN 'ReactiveClicking' THEN 'DynamicClicking'
	        WHEN 'MultiHitClicking' THEN 'TargetSwitching'
	        ELSE NULLIF(rs.summary_json->>'scenarioType', '')
	    END,
	        updated_at = NOW()
	    FROM run_summaries rs
	    WHERE sr.session_id = rs.session_id
	      AND sr.user_id = (
	        SELECT hu.id
	        FROM hub_users hu
	        LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
	        WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
	           OR LOWER(hu.external_id) = $1
	        LIMIT 1
	      )
	      AND sr.scenario_type IN ('Unknown', '')
	      AND NULLIF(rs.summary_json->>'scenarioType', '') IS NOT NULL
	      AND rs.summary_json->>'scenarioType' <> 'Unknown'
	`, handle); err != nil {
		return pgconn.CommandTag{}, err
	}

	if _, err := s.pool.Exec(ctx, `
	    WITH dominant AS (
	        SELECT scenario_name, scenario_type
	        FROM (
	            SELECT
	                scenario_name,
	                scenario_type,
	                ROW_NUMBER() OVER (
	                    PARTITION BY scenario_name
	                    ORDER BY COUNT(*) DESC, scenario_type ASC
	                ) AS rank
	            FROM scenario_runs
	            WHERE user_id = (
	                SELECT hu.id
	                FROM hub_users hu
	                LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
	                WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
	                   OR LOWER(hu.external_id) = $1
	                LIMIT 1
	            )
	              AND COALESCE(NULLIF(scenario_type, ''), 'Unknown') <> 'Unknown'
	            GROUP BY scenario_name, scenario_type
	        ) ranked
	        WHERE rank = 1
	    )
	    UPDATE scenario_runs sr
	    SET scenario_type = dominant.scenario_type,
	        updated_at = NOW()
	    FROM dominant
	    WHERE sr.user_id = (
	        SELECT hu.id
	        FROM hub_users hu
	        LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
	        WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
	           OR LOWER(hu.external_id) = $1
	        LIMIT 1
	    )
	      AND sr.scenario_name = dominant.scenario_name
	      AND COALESCE(NULLIF(sr.scenario_type, ''), 'Unknown') = 'Unknown'
	`, handle); err != nil {
		return pgconn.CommandTag{}, err
	}

	return s.pool.Exec(ctx, `
	    UPDATE scenario_runs sr
	    SET scenario_type = CASE
	        WHEN COALESCE((rs.summary_json->>'csvKills')::float, 0) > 0
	             AND COALESCE((rs.summary_json->>'csvDamageDone')::float, 0) > 0
	            THEN 'TargetSwitching'
	        WHEN COALESCE((rs.summary_json->>'csvKills')::float, 0) > 0
	             AND (
	                COALESCE((rs.summary_json->>'csvAvgTtk')::float, 0) >= 0.45
	                OR (
	                    COALESCE((rs.summary_json->>'killsPerSecond')::float, 0) > 0
	                    AND COALESCE((rs.summary_json->>'killsPerSecond')::float, 0) <= 2.25
	                )
	             )
	            THEN 'DynamicClicking'
	        WHEN COALESCE((rs.summary_json->>'csvKills')::float, 0) > 0
	            THEN 'StaticClicking'
	        ELSE 'Tracking'
	    END,
	        updated_at = NOW()
	    FROM run_summaries rs
	    WHERE sr.session_id = rs.session_id
	      AND sr.user_id = (
	        SELECT hu.id
	        FROM hub_users hu
	        LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
	        WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
	           OR LOWER(hu.external_id) = $1
	        LIMIT 1
	      )
	      AND sr.scenario_type IN ('Unknown', '')
	      AND (
	        COALESCE((rs.summary_json->>'csvKills')::float, 0) > 0
	        OR (rs.summary_json->>'csvAvgTtk')::float >= 5.0
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
	`, handle)
}

func (s *Store) GetAdminFailures(ctx context.Context, handle string, limit int) ([]AdminIngestFailure, error) {
	query := `
		SELECT
			id,
			user_external_id,
			session_id,
			scenario_name,
			error_message,
			created_at
		FROM ingest_failures
	`
	args := []any{}
	where := ""
	if trimmed := strings.TrimSpace(strings.ToLower(handle)); trimmed != "" {
		where = `
		WHERE LOWER(user_external_id) = (
			SELECT LOWER(hu.external_id)
			FROM hub_users hu
			LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
			WHERE LOWER(COALESCE(la.username, hu.external_id)) = $1
			   OR LOWER(hu.external_id) = $1
			LIMIT 1
		)
		`
		args = append(args, trimmed)
	}
	query += where + ` ORDER BY created_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load admin failures: %w", err)
	}
	defer rows.Close()

	failures := []AdminIngestFailure{}
	for rows.Next() {
		var item AdminIngestFailure
		if err := rows.Scan(
			&item.ID,
			&item.UserExternalID,
			&item.SessionID,
			&item.ScenarioName,
			&item.ErrorMessage,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan admin failure: %w", err)
		}
		failures = append(failures, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin failures: %w", err)
	}
	return failures, nil
}
