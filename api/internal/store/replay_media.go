package store

import (
	"context"
	"fmt"
	"encoding/json"
	"strings"
)

type ReplayMediaMeta struct {
	PublicRunID string `json:"publicRunId"`
	Quality     string `json:"quality"`
	StorageKey  string `json:"storageKey"`
	ContentType string `json:"contentType"`
	ByteSize    int64  `json:"byteSize"`
}

type MousePathPoint struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Timestamp uint64  `json:"timestampMs"`
	IsClick   bool    `json:"isClick"`
}

type ReplayMediaUploadTarget struct {
	StoredSessionID string
	PublicRunID     string
}

func normalizeReplayMediaQuality(value string) string {
	switch strings.TrimSpace(value) {
	case "high", "ultra":
		return strings.TrimSpace(value)
	default:
		return "standard"
	}
}

func (s *Store) GetReplayMediaUploadTarget(ctx context.Context, userID int64, sessionID string) (ReplayMediaUploadTarget, error) {
	var target ReplayMediaUploadTarget
	if err := s.pool.QueryRow(ctx, `
		SELECT session_id, public_run_id
		FROM scenario_runs
		WHERE user_id = $1
		  AND ($2 = session_id OR $2 = source_session_id)
	`, userID, sessionID).Scan(&target.StoredSessionID, &target.PublicRunID); err != nil {
		return ReplayMediaUploadTarget{}, fmt.Errorf("load replay media target: %w", err)
	}
	return target, nil
}

func (s *Store) UpsertReplayMediaAsset(
	ctx context.Context,
	sessionID string,
	quality string,
	storageKey string,
	contentType string,
	byteSize int64,
) error {
	quality = normalizeReplayMediaQuality(quality)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO replay_media_assets (
			session_id,
			quality,
			storage_key,
			content_type,
			byte_size
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (session_id) DO UPDATE SET
			quality = EXCLUDED.quality,
			storage_key = EXCLUDED.storage_key,
			content_type = EXCLUDED.content_type,
			byte_size = EXCLUDED.byte_size,
			updated_at = NOW()
	`, sessionID, quality, storageKey, contentType, byteSize); err != nil {
		return fmt.Errorf("upsert replay media asset: %w", err)
	}
	return nil
}

func (s *Store) GetReplayMediaMeta(ctx context.Context, runID string) (ReplayMediaMeta, error) {
	var meta ReplayMediaMeta
	if err := s.pool.QueryRow(ctx, `
		SELECT
			sr.public_run_id,
			rma.quality,
			rma.storage_key,
			rma.content_type,
			rma.byte_size
		FROM replay_media_assets rma
		JOIN scenario_runs sr ON sr.session_id = rma.session_id
		WHERE sr.public_run_id = $1 OR sr.session_id = $1
	`, runID).Scan(
		&meta.PublicRunID,
		&meta.Quality,
		&meta.StorageKey,
		&meta.ContentType,
		&meta.ByteSize,
	); err != nil {
		return ReplayMediaMeta{}, fmt.Errorf("load replay media meta: %w", err)
	}
	return meta, nil
}

func (s *Store) DeleteReplayMediaAssetForUser(ctx context.Context, userID int64, runID string) (string, error) {
	var storageKey string
	if err := s.pool.QueryRow(ctx, `
		DELETE FROM replay_media_assets rma
		USING scenario_runs sr
		WHERE rma.session_id = sr.session_id
		  AND sr.user_id = $1
		  AND ($2 = sr.public_run_id OR $2 = sr.session_id OR $2 = sr.source_session_id)
		RETURNING rma.storage_key
	`, userID, runID).Scan(&storageKey); err != nil {
		return "", fmt.Errorf("delete replay media asset: %w", err)
	}
	return storageKey, nil
}

func (s *Store) UpsertMousePath(
	ctx context.Context,
	sessionID string,
	points []MousePathPoint,
) error {
	pointCount := len(points)
	var durationMS uint64
	if pointCount > 0 {
		durationMS = points[pointCount-1].Timestamp
	}
	payload, err := json.Marshal(points)
	if err != nil {
		return fmt.Errorf("marshal mouse path: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO run_mouse_paths (
			session_id,
			point_count,
			duration_ms,
			path_json
		)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (session_id) DO UPDATE SET
			point_count = EXCLUDED.point_count,
			duration_ms = EXCLUDED.duration_ms,
			path_json = EXCLUDED.path_json,
			updated_at = NOW()
	`, sessionID, pointCount, int64(durationMS), string(payload)); err != nil {
		return fmt.Errorf("upsert mouse path: %w", err)
	}
	return nil
}

func (s *Store) GetMousePath(ctx context.Context, runID string) ([]MousePathPoint, error) {
	var payload []byte
	if err := s.pool.QueryRow(ctx, `
		SELECT rmp.path_json::text
		FROM run_mouse_paths rmp
		JOIN scenario_runs sr ON sr.session_id = rmp.session_id
		WHERE sr.public_run_id = $1 OR sr.session_id = $1 OR sr.source_session_id = $1
	`, runID).Scan(&payload); err != nil {
		return nil, fmt.Errorf("load mouse path: %w", err)
	}
	var points []MousePathPoint
	if err := json.Unmarshal(payload, &points); err != nil {
		return nil, fmt.Errorf("decode mouse path: %w", err)
	}
	return points, nil
}
