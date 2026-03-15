package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS hub_users (
  id BIGSERIAL PRIMARY KEY,
  external_id TEXT NOT NULL UNIQUE,
  aimmod_user_id TEXT NOT NULL DEFAULT '',
	profile_handle TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scenario_runs (
  session_id TEXT PRIMARY KEY,
  source_session_id TEXT,
  public_run_id TEXT,
  user_id BIGINT NOT NULL REFERENCES hub_users(id) ON DELETE CASCADE,
  app_version TEXT NOT NULL,
  schema_version INTEGER NOT NULL,
  scenario_name TEXT NOT NULL,
  scenario_type TEXT NOT NULL DEFAULT '',
  score DOUBLE PRECISION NOT NULL DEFAULT 0,
  accuracy DOUBLE PRECISION NOT NULL DEFAULT 0,
  duration_ms BIGINT NOT NULL DEFAULT 0,
  played_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scenario_runs_user_played_at
  ON scenario_runs(user_id, played_at DESC);

CREATE TABLE IF NOT EXISTS run_summaries (
  session_id TEXT PRIMARY KEY REFERENCES scenario_runs(session_id) ON DELETE CASCADE,
  summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS run_feature_sets (
  session_id TEXT PRIMARY KEY REFERENCES scenario_runs(session_id) ON DELETE CASCADE,
  feature_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS run_timeline_seconds (
  session_id TEXT NOT NULL REFERENCES scenario_runs(session_id) ON DELETE CASCADE,
  t_sec INTEGER NOT NULL,
  score DOUBLE PRECISION NOT NULL DEFAULT 0,
  accuracy DOUBLE PRECISION NOT NULL DEFAULT 0,
  damage_eff DOUBLE PRECISION NOT NULL DEFAULT 0,
  spm DOUBLE PRECISION NOT NULL DEFAULT 0,
  shots INTEGER NOT NULL DEFAULT 0,
  hits INTEGER NOT NULL DEFAULT 0,
  kills INTEGER NOT NULL DEFAULT 0,
  paused BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (session_id, t_sec)
);

CREATE TABLE IF NOT EXISTS run_context_windows (
  session_id TEXT NOT NULL REFERENCES scenario_runs(session_id) ON DELETE CASCADE,
  ordinal INTEGER NOT NULL,
  start_ms BIGINT NOT NULL,
  end_ms BIGINT NOT NULL,
  window_type TEXT NOT NULL DEFAULT '',
  label TEXT NOT NULL DEFAULT '',
  feature_summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  coaching_tags TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (session_id, ordinal)
);

CREATE TABLE IF NOT EXISTS linked_accounts (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES hub_users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  provider_account_id TEXT NOT NULL,
  username TEXT NOT NULL DEFAULT '',
  display_name TEXT NOT NULL DEFAULT '',
  avatar_url TEXT NOT NULL DEFAULT '',
  verified BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (provider, provider_account_id),
  UNIQUE (user_id, provider)
);

CREATE TABLE IF NOT EXISTS auth_states (
  state TEXT PRIMARY KEY,
  return_to TEXT NOT NULL DEFAULT '/account',
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS auth_sessions (
  session_id TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES hub_users(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS upload_tokens (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES hub_users(id) ON DELETE CASCADE,
  label TEXT NOT NULL DEFAULT 'Desktop app',
  token_hash TEXT NOT NULL UNIQUE,
  last_four TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_used_at TIMESTAMPTZ NULL,
  revoked_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS device_link_requests (
  device_code TEXT PRIMARY KEY,
  user_code TEXT NOT NULL UNIQUE,
  label TEXT NOT NULL DEFAULT 'AimMod desktop',
  status TEXT NOT NULL DEFAULT 'pending',
  user_id BIGINT NULL REFERENCES hub_users(id) ON DELETE SET NULL,
  upload_token_id BIGINT NULL REFERENCES upload_tokens(id) ON DELETE SET NULL,
  upload_token_plaintext TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  approved_at TIMESTAMPTZ NULL,
  last_polled_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS ingest_failures (
  id BIGSERIAL PRIMARY KEY,
  user_external_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  scenario_name TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS replay_media_assets (
  session_id TEXT PRIMARY KEY REFERENCES scenario_runs(session_id) ON DELETE CASCADE,
  quality TEXT NOT NULL DEFAULT 'standard',
  storage_key TEXT NOT NULL,
  content_type TEXT NOT NULL DEFAULT 'video/mp4',
  byte_size BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS run_mouse_paths (
  session_id TEXT PRIMARY KEY REFERENCES scenario_runs(session_id) ON DELETE CASCADE,
  point_count INTEGER NOT NULL DEFAULT 0,
  duration_ms BIGINT NOT NULL DEFAULT 0,
  playback_offset_ms BIGINT NOT NULL DEFAULT 0,
  video_offset_ms BIGINT NOT NULL DEFAULT 0,
  path_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  hit_timestamps_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_replay_media_assets_storage_key
  ON replay_media_assets(storage_key);

CREATE INDEX IF NOT EXISTS idx_ingest_failures_created_at
  ON ingest_failures(created_at DESC);

CREATE TABLE IF NOT EXISTS kovaaks_user_cache (
  steam_id TEXT PRIMARY KEY,
  kovaaks_username TEXT NOT NULL DEFAULT '',
  steam_display_name TEXT NOT NULL DEFAULT '',
  avatar_url TEXT NOT NULL DEFAULT '',
  country TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_kovaaks_user_cache_username
  ON kovaaks_user_cache (LOWER(kovaaks_username));

CREATE INDEX IF NOT EXISTS idx_kovaaks_user_cache_display_name
  ON kovaaks_user_cache (LOWER(steam_display_name));
`

const hubUserIdentityViewSQL = `
CREATE OR REPLACE VIEW hub_user_identity AS
SELECT
  hu.id AS user_id,
  hu.external_id,
	COALESCE(NULLIF(hu.profile_handle, ''), COALESCE(NULLIF(best.username, ''), hu.external_id)) AS user_handle,
  COALESCE(NULLIF(best.display_name, ''), COALESCE(NULLIF(best.username, ''), hu.external_id)) AS user_display_name,
  COALESCE(best.avatar_url, '') AS avatar_url,
  COALESCE(best.verified, FALSE) AS is_verified
FROM hub_users hu
LEFT JOIN LATERAL (
  SELECT
    la.username,
    la.display_name,
    la.avatar_url,
    la.verified
  FROM linked_accounts la
  WHERE la.user_id = hu.id
  ORDER BY
    CASE
      WHEN la.verified AND la.provider = 'kovaaks' THEN 1
      WHEN la.verified AND la.provider = 'steam' THEN 2
      WHEN la.verified AND la.provider = 'discord' THEN 3
      WHEN NOT la.verified AND la.provider = 'kovaaks' THEN 4
      WHEN NOT la.verified AND la.provider = 'steam' THEN 5
      WHEN la.provider = 'discord' THEN 6
      ELSE 7
    END,
    la.updated_at DESC,
    la.id DESC
  LIMIT 1
) AS best ON TRUE;
`

type Store struct {
	pool *pgxpool.Pool

	scenarioSlugMu    sync.RWMutex
	scenarioSlugCache scenarioSlugCache
}

type scenarioSlugCache struct {
	expiresAt time.Time
	bySlug    map[string]string
}

type IngestedRun struct {
	AppVersion       string
	SchemaVersion    uint32
	UserExternalID   string
	KovaaksUserID    string
	KovaaksUsername  string
	UserDisplayName  string
	AvatarURL        string
	SteamID          string
	SteamDisplayName string
	SessionID        string
	ScenarioName     string
	ScenarioType     string
	Score            float64
	Accuracy         float64
	DurationMS       uint64
	PlayedAt         time.Time
	SummaryJSON      []byte
	FeatureJSON      []byte
	Timeline         []TimelineSecond
	ContextWindows   []ContextWindow
}

type TimelineSecond struct {
	Second    uint32
	Score     float64
	Accuracy  float64
	DamageEff float64
	SPM       float64
	Shots     uint32
	Hits      uint32
	Kills     uint32
	Paused    bool
}

type ContextWindow struct {
	StartMS            uint64
	EndMS              uint64
	WindowType         string
	Label              string
	FeatureSummaryJSON []byte
	CoachingTags       []string
}

type DiscordLink struct {
	UserExternalID string
	DiscordUserID  string
	Username       string
	GlobalName     string
	AvatarURL      string
}

type IngestFailureRecord struct {
	UserExternalID string
	SessionID      string
	ScenarioName   string
	ErrorMessage   string
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	cfg.MaxConns = 8
	cfg.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	store := &Store{pool: pool}
	if err := store.ensureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) ensureSchema(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
		ALTER TABLE hub_users ADD COLUMN IF NOT EXISTS aimmod_user_id TEXT NOT NULL DEFAULT '';
		ALTER TABLE hub_users ADD COLUMN IF NOT EXISTS profile_handle TEXT NOT NULL DEFAULT '';
		ALTER TABLE linked_accounts ADD COLUMN IF NOT EXISTS verified BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE scenario_runs ADD COLUMN IF NOT EXISTS source_session_id TEXT;
		ALTER TABLE scenario_runs ADD COLUMN IF NOT EXISTS public_run_id TEXT;
		ALTER TABLE run_mouse_paths ADD COLUMN IF NOT EXISTS playback_offset_ms BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE run_mouse_paths ADD COLUMN IF NOT EXISTS video_offset_ms BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE run_mouse_paths ADD COLUMN IF NOT EXISTS hit_timestamps_json JSONB NOT NULL DEFAULT '[]'::jsonb;
		CREATE UNIQUE INDEX IF NOT EXISTS idx_scenario_runs_public_run_id
			ON scenario_runs(public_run_id)
			WHERE public_run_id IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS idx_scenario_runs_user_source_session_id
			ON scenario_runs(user_id, source_session_id)
			WHERE source_session_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_scenario_runs_played_at
			ON scenario_runs(played_at DESC);
		CREATE INDEX IF NOT EXISTS idx_scenario_runs_scenario_name_played_at
			ON scenario_runs(scenario_name, played_at DESC, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_scenario_runs_scenario_name_score_played_at
			ON scenario_runs(scenario_name, score DESC, played_at DESC);
		CREATE INDEX IF NOT EXISTS idx_scenario_runs_user_scenario_name_played_at
			ON scenario_runs(user_id, scenario_name, played_at DESC);
		CREATE INDEX IF NOT EXISTS idx_scenario_runs_scenario_type_score_played_at
			ON scenario_runs(scenario_type, score DESC, played_at DESC);
	`); err != nil {
		return fmt.Errorf("ensure scenario run identifiers: %w", err)
	}
	if err := s.backfillAimmodUserIDs(ctx); err != nil {
		return fmt.Errorf("backfill aimmod user ids: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_hub_users_aimmod_user_id
			ON hub_users (aimmod_user_id)
			WHERE aimmod_user_id <> '';
	`); err != nil {
		return fmt.Errorf("ensure aimmod user id index: %w", err)
	}
	if err := s.backfillProfileHandles(ctx); err != nil {
		return fmt.Errorf("backfill profile handles: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_hub_users_profile_handle_ci
			ON hub_users (LOWER(profile_handle))
			WHERE profile_handle <> '';
	`); err != nil {
		return fmt.Errorf("ensure profile handle index: %w", err)
	}
	if _, err := s.pool.Exec(ctx, hubUserIdentityViewSQL); err != nil {
		return fmt.Errorf("ensure hub user identity view: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
		DELETE FROM ingest_failures
		WHERE created_at < NOW() - INTERVAL '30 days'
	`); err != nil {
		return fmt.Errorf("trim ingest failures: %w", err)
	}
	return nil
}

func makePublicRunID(userExternalID string, sourceSessionID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(strings.ToLower(userExternalID)) + ":" + strings.TrimSpace(sourceSessionID)))
	return "run_" + hex.EncodeToString(sum[:16])
}

func (s *Store) SaveIngestedRun(ctx context.Context, run IngestedRun, authUser *AuthUser) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	userID, resolvedExternalID, err := resolveIngestUserTx(ctx, tx, run, authUser)
	if err != nil {
		return err
	}

	storedRunID := makePublicRunID(resolvedExternalID, run.SessionID)

	if _, err := tx.Exec(ctx, `
		INSERT INTO scenario_runs (
			session_id,
			source_session_id,
			public_run_id,
			user_id,
			app_version,
			schema_version,
			scenario_name,
			scenario_type,
			score,
			accuracy,
			duration_ms,
			played_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (session_id) DO UPDATE SET
			session_id = EXCLUDED.session_id,
			source_session_id = EXCLUDED.source_session_id,
			public_run_id = EXCLUDED.public_run_id,
			user_id = EXCLUDED.user_id,
			app_version = EXCLUDED.app_version,
			schema_version = EXCLUDED.schema_version,
			scenario_name = EXCLUDED.scenario_name,
			scenario_type = EXCLUDED.scenario_type,
			score = EXCLUDED.score,
			accuracy = EXCLUDED.accuracy,
			duration_ms = EXCLUDED.duration_ms,
			played_at = EXCLUDED.played_at,
			updated_at = NOW()
	`, storedRunID, run.SessionID, storedRunID, userID, run.AppVersion, int32(run.SchemaVersion), run.ScenarioName, run.ScenarioType, run.Score, run.Accuracy, int64(run.DurationMS), run.PlayedAt); err != nil {
		return fmt.Errorf("upsert scenario run: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO run_summaries (session_id, summary_json)
		VALUES ($1, $2::jsonb)
		ON CONFLICT (session_id) DO UPDATE SET
			summary_json = EXCLUDED.summary_json,
			updated_at = NOW()
	`, storedRunID, string(run.SummaryJSON)); err != nil {
		return fmt.Errorf("upsert run summary: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO run_feature_sets (session_id, feature_json)
		VALUES ($1, $2::jsonb)
		ON CONFLICT (session_id) DO UPDATE SET
			feature_json = EXCLUDED.feature_json,
			updated_at = NOW()
	`, storedRunID, string(run.FeatureJSON)); err != nil {
		return fmt.Errorf("upsert run feature set: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM run_timeline_seconds WHERE session_id = $1`, storedRunID); err != nil {
		return fmt.Errorf("clear run timeline: %w", err)
	}
	for _, second := range run.Timeline {
		if _, err := tx.Exec(ctx, `
			INSERT INTO run_timeline_seconds (
				session_id,
				t_sec,
				score,
				accuracy,
				damage_eff,
				spm,
				shots,
				hits,
				kills,
				paused
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, storedRunID, int32(second.Second), second.Score, second.Accuracy, second.DamageEff, second.SPM, int32(second.Shots), int32(second.Hits), int32(second.Kills), second.Paused); err != nil {
			return fmt.Errorf("insert run timeline second: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM run_context_windows WHERE session_id = $1`, storedRunID); err != nil {
		return fmt.Errorf("clear run context windows: %w", err)
	}
	for idx, window := range run.ContextWindows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO run_context_windows (
				session_id,
				ordinal,
				start_ms,
				end_ms,
				window_type,
				label,
				feature_summary_json,
				coaching_tags
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
		`, storedRunID, idx, int64(window.StartMS), int64(window.EndMS), window.WindowType, window.Label, string(window.FeatureSummaryJSON), window.CoachingTags); err != nil {
			return fmt.Errorf("insert run context window: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (s *Store) LinkDiscordAccount(ctx context.Context, link DiscordLink) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var userID int64
	if userID, _, err = insertHubUserTx(ctx, tx, link.UserExternalID); err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM linked_accounts
		WHERE user_id = $1 AND provider = 'discord'
	`, userID); err != nil {
		return fmt.Errorf("clear existing discord link: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO linked_accounts (
			user_id,
			provider,
			provider_account_id,
			username,
			display_name,
			avatar_url,
			verified
		)
		VALUES ($1, 'discord', $2, $3, $4, $5, TRUE)
		ON CONFLICT (provider, provider_account_id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			verified = TRUE,
			updated_at = NOW()
	`, userID, link.DiscordUserID, link.Username, link.GlobalName, link.AvatarURL); err != nil {
		return fmt.Errorf("upsert discord link: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func SummaryMapToJSON(summary map[string]*hubv1.SessionSummaryValue) ([]byte, error) {
	if len(summary) == 0 {
		return []byte(`{}`), nil
	}

	jsonSummary := make(map[string]any, len(summary))
	for key, value := range summary {
		if value == nil {
			jsonSummary[key] = nil
			continue
		}
		switch kind := value.Kind.(type) {
		case *hubv1.SessionSummaryValue_StringValue:
			jsonSummary[key] = kind.StringValue
		case *hubv1.SessionSummaryValue_NumberValue:
			jsonSummary[key] = kind.NumberValue
		case *hubv1.SessionSummaryValue_BoolValue:
			jsonSummary[key] = kind.BoolValue
		default:
			jsonSummary[key] = nil
		}
	}

	payload, err := json.Marshal(jsonSummary)
	if err != nil {
		return nil, fmt.Errorf("marshal summary: %w", err)
	}
	return payload, nil
}

func (s *Store) RecordIngestFailure(ctx context.Context, failure IngestFailureRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ingest_failures (
			user_external_id,
			session_id,
			scenario_name,
			error_message
		)
		VALUES ($1, $2, $3, $4)
	`, strings.TrimSpace(failure.UserExternalID), strings.TrimSpace(failure.SessionID), strings.TrimSpace(failure.ScenarioName), strings.TrimSpace(failure.ErrorMessage))
	if err != nil {
		return fmt.Errorf("record ingest failure: %w", err)
	}
	return nil
}

func (s *Store) ClearIngestFailures(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM ingest_failures`)
	if err != nil {
		return 0, fmt.Errorf("clear ingest failures: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (s *Store) ClearIngestFailuresForUser(ctx context.Context, handle string) (int64, error) {
	handle = strings.TrimSpace(strings.ToLower(handle))
	if handle == "" {
		return 0, fmt.Errorf("user handle is required")
	}

	tag, err := s.pool.Exec(ctx, `
		DELETE FROM ingest_failures
		WHERE LOWER(user_external_id) = (
			SELECT LOWER(hui.external_id)
			FROM hub_user_identity hui
			WHERE LOWER(hui.user_handle) = $1
			   OR LOWER(hui.external_id) = $1
			LIMIT 1
		)
	`, handle)
	if err != nil {
		return 0, fmt.Errorf("clear ingest failures for user: %w", err)
	}
	return tag.RowsAffected(), nil
}
