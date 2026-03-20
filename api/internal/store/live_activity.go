package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const liveActivityTTL = 90 * time.Second

type LiveActivityPayload struct {
	GameStateCode          int32    `json:"gameStateCode"`
	GameState              string   `json:"gameState"`
	Paused                 bool     `json:"paused"`
	ScenarioName           string   `json:"scenarioName,omitempty"`
	ScenarioType           string   `json:"scenarioType,omitempty"`
	ScenarioSubtype        string   `json:"scenarioSubtype,omitempty"`
	Score                  *float64 `json:"score,omitempty"`
	ScorePerMinute         *float64 `json:"scorePerMinute,omitempty"`
	AccuracyPct            *float64 `json:"accuracyPct,omitempty"`
	Kills                  *uint32  `json:"kills,omitempty"`
	ElapsedSecs            *float64 `json:"elapsedSecs,omitempty"`
	TimeRemainingSecs      *float64 `json:"timeRemainingSecs,omitempty"`
	QueueTimeRemainingSecs *float64 `json:"queueTimeRemainingSecs,omitempty"`
	RuntimeLoaded          bool     `json:"runtimeLoaded"`
	BridgeConnected        bool     `json:"bridgeConnected"`
}

type LiveActivityRecord struct {
	Active          bool      `json:"active"`
	UserHandle      string    `json:"userHandle,omitempty"`
	UserDisplayName string    `json:"userDisplayName,omitempty"`
	AvatarURL       string    `json:"avatarUrl,omitempty"`
	IsVerified      bool      `json:"isVerified,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt,omitempty"`
	LiveActivityPayload
}

func sanitizeLiveActivityPayload(payload LiveActivityPayload) LiveActivityPayload {
	clean := func(value string, maxLen int) string {
		value = strings.TrimSpace(value)
		if value == "" {
			return ""
		}
		var out []rune
		for _, r := range value {
			if r == '\n' || r == '\r' || r == '\t' {
				r = ' '
			}
			if r < 32 {
				continue
			}
			out = append(out, r)
			if len(out) >= maxLen {
				break
			}
		}
		return strings.TrimSpace(string(out))
	}

	payload.GameState = clean(payload.GameState, 80)
	payload.ScenarioName = clean(payload.ScenarioName, 160)
	payload.ScenarioType = clean(payload.ScenarioType, 80)
	payload.ScenarioSubtype = clean(payload.ScenarioSubtype, 80)
	return payload
}

func (s *Store) UpsertLiveActivity(ctx context.Context, userID int64, payload LiveActivityPayload) error {
	payload = sanitizeLiveActivityPayload(payload)
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode live activity payload: %w", err)
	}

	ttlSeconds := int(liveActivityTTL / time.Second)

	if _, err := s.pool.Exec(ctx, `
		INSERT INTO live_activities (user_id, payload_json, updated_at, expires_at)
		VALUES ($1, $2::jsonb, NOW(), NOW() + ($3 * INTERVAL '1 second'))
		ON CONFLICT (user_id) DO UPDATE SET
			payload_json = EXCLUDED.payload_json,
			updated_at = NOW(),
			expires_at = NOW() + ($3 * INTERVAL '1 second')
	`, userID, string(encoded), ttlSeconds); err != nil {
		return fmt.Errorf("upsert live activity: %w", err)
	}

	return nil
}

func (s *Store) DeleteLiveActivity(ctx context.Context, userID int64) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM live_activities WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete live activity: %w", err)
	}
	return nil
}

func (s *Store) GetLiveActivityByHandle(ctx context.Context, handle string) (LiveActivityRecord, error) {
	handle = strings.TrimSpace(strings.ToLower(handle))
	if handle == "" {
		return LiveActivityRecord{}, fmt.Errorf("profile handle is required")
	}

	var (
		record  LiveActivityRecord
		rawJSON []byte
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT
			hui.user_handle,
			hui.user_display_name,
			hui.avatar_url,
			hui.is_verified,
			la.updated_at,
			la.payload_json
		FROM live_activities la
		JOIN hub_user_identity hui ON hui.user_id = la.user_id
		WHERE (LOWER(hui.user_handle) = $1 OR LOWER(hui.external_id) = $1)
		  AND la.expires_at > NOW()
		LIMIT 1
	`, handle).Scan(
		&record.UserHandle,
		&record.UserDisplayName,
		&record.AvatarURL,
		&record.IsVerified,
		&record.UpdatedAt,
		&rawJSON,
	); err != nil {
		if err == pgx.ErrNoRows {
			return LiveActivityRecord{}, err
		}
		return LiveActivityRecord{}, fmt.Errorf("load live activity: %w", err)
	}

	if err := json.Unmarshal(rawJSON, &record.LiveActivityPayload); err != nil {
		return LiveActivityRecord{}, fmt.Errorf("decode live activity payload: %w", err)
	}
	record.Active = true
	return record, nil
}

func (s *Store) ListLiveActivities(ctx context.Context, limit int) ([]LiveActivityRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			hui.user_handle,
			hui.user_display_name,
			hui.avatar_url,
			hui.is_verified,
			la.updated_at,
			la.payload_json
		FROM live_activities la
		JOIN hub_user_identity hui ON hui.user_id = la.user_id
		WHERE la.expires_at > NOW()
		ORDER BY la.updated_at DESC, hui.user_handle ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list live activities: %w", err)
	}
	defer rows.Close()

	records := make([]LiveActivityRecord, 0, limit)
	for rows.Next() {
		var (
			record  LiveActivityRecord
			rawJSON []byte
		)
		if err := rows.Scan(
			&record.UserHandle,
			&record.UserDisplayName,
			&record.AvatarURL,
			&record.IsVerified,
			&record.UpdatedAt,
			&rawJSON,
		); err != nil {
			return nil, fmt.Errorf("scan live activity: %w", err)
		}
		if err := json.Unmarshal(rawJSON, &record.LiveActivityPayload); err != nil {
			return nil, fmt.Errorf("decode live activity payload: %w", err)
		}
		record.Active = true
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate live activities: %w", err)
	}

	return records, nil
}
