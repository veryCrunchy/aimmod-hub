package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	OsuVisibilityPrivate  = "private"
	OsuVisibilityUnlisted = "unlisted"
	OsuVisibilityPublic   = "public"
)

type OsuProfileInput struct {
	OsuUserID         int64    `json:"osuUserId"`
	Username          string   `json:"username"`
	CountryCode       string   `json:"countryCode"`
	AvatarURL         string   `json:"avatarUrl"`
	GlobalRank        *int64   `json:"globalRank,omitempty"`
	PerformancePoints *float64 `json:"performancePoints,omitempty"`
	PlayCount         int64    `json:"playCount"`
	PlayTimeSeconds   int64    `json:"playTimeSeconds"`
}

type OsuBeatmapSetInput struct {
	SetKey   string `json:"setKey"`
	OnlineID int64  `json:"onlineId"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Creator  string `json:"creator"`
	Source   string `json:"source"`
	CoverURL string `json:"coverUrl"`
}

type OsuBeatmapDifficultyInput struct {
	DifficultyKey     string  `json:"difficultyKey"`
	SetKey            string  `json:"setKey"`
	OnlineID          int64   `json:"onlineId"`
	Checksum          string  `json:"checksum"`
	Version           string  `json:"version"`
	Ruleset           string  `json:"ruleset"`
	StarRating        float64 `json:"starRating"`
	BPM               float64 `json:"bpm"`
	LengthMS          int64   `json:"lengthMs"`
	CircleSize        float64 `json:"circleSize"`
	ApproachRate      float64 `json:"approachRate"`
	OverallDifficulty float64 `json:"overallDifficulty"`
	DrainRate         float64 `json:"drainRate"`
	MaxCombo          int     `json:"maxCombo"`
}

type OsuScoreInput struct {
	ClientScoreID     string    `json:"clientScoreId"`
	OnlineScoreID     int64     `json:"onlineScoreId"`
	PlayedAt          time.Time `json:"playedAt"`
	TotalScore        int64     `json:"totalScore"`
	PerformancePoints *float64  `json:"performancePoints,omitempty"`
	Accuracy          float64   `json:"accuracy"`
	MaxCombo          int       `json:"maxCombo"`
	Count300          int       `json:"count300"`
	Count100          int       `json:"count100"`
	Count50           int       `json:"count50"`
	CountMiss         int       `json:"countMiss"`
	Mods              []string  `json:"mods"`
	Passed            bool      `json:"passed"`
}

type OsuReplayInput struct {
	SHA256         string `json:"sha256"`
	ClientFilename string `json:"clientFilename"`
	UploadFile     bool   `json:"uploadFile"`
}

type OsuAnalysisInput struct {
	SchemaVersion int             `json:"schemaVersion"`
	EngineVersion string          `json:"engineVersion"`
	Payload       json.RawMessage `json:"payload"`
}

type OsuSyncInput struct {
	SchemaVersion  int                       `json:"schemaVersion"`
	ClientUploadID string                    `json:"clientUploadId"`
	ContentHash    string                    `json:"contentHash"`
	Visibility     string                    `json:"visibility"`
	Profile        OsuProfileInput           `json:"profile"`
	BeatmapSet     OsuBeatmapSetInput        `json:"beatmapSet"`
	Difficulty     OsuBeatmapDifficultyInput `json:"difficulty"`
	Score          OsuScoreInput             `json:"score"`
	Replay         *OsuReplayInput           `json:"replay,omitempty"`
	Analysis       *OsuAnalysisInput         `json:"analysis,omitempty"`
}

type OsuSyncResult struct {
	ShareID              string `json:"shareId"`
	Visibility           string `json:"visibility"`
	Created              bool   `json:"created"`
	ReplayUploadRequired bool   `json:"replayUploadRequired"`
}

type OsuReplayUploadTarget struct {
	ScoreID      int64
	ShareID      string
	Visibility   string
	ReplaySHA256 string
	StorageKey   string
	ByteSize     int64
}

type OsuPublicReplay struct {
	ShareID           string          `json:"shareId"`
	Visibility        string          `json:"visibility"`
	HubHandle         string          `json:"hubHandle"`
	HubDisplayName    string          `json:"hubDisplayName"`
	OsuUserID         int64           `json:"osuUserId"`
	OsuUsername       string          `json:"osuUsername"`
	CountryCode       string          `json:"countryCode"`
	AvatarURL         string          `json:"avatarUrl"`
	BeatmapSetID      int64           `json:"beatmapSetId"`
	BeatmapID         int64           `json:"beatmapId"`
	Title             string          `json:"title"`
	Artist            string          `json:"artist"`
	Creator           string          `json:"creator"`
	CoverURL          string          `json:"coverUrl"`
	Difficulty        string          `json:"difficulty"`
	Ruleset           string          `json:"ruleset"`
	StarRating        float64         `json:"starRating"`
	BPM               float64         `json:"bpm"`
	LengthMS          int64           `json:"lengthMs"`
	PlayedAt          time.Time       `json:"playedAt"`
	TotalScore        int64           `json:"totalScore"`
	PerformancePoints *float64        `json:"performancePoints,omitempty"`
	Accuracy          float64         `json:"accuracy"`
	MaxCombo          int             `json:"maxCombo"`
	Count300          int             `json:"count300"`
	Count100          int             `json:"count100"`
	Count50           int             `json:"count50"`
	CountMiss         int             `json:"countMiss"`
	Mods              []string        `json:"mods"`
	Passed            bool            `json:"passed"`
	HasReplayFile     bool            `json:"hasReplayFile"`
	AnalysisSchema    int             `json:"analysisSchema"`
	AnalysisEngine    string          `json:"analysisEngine"`
	Analysis          json.RawMessage `json:"analysis,omitempty"`
}

type OsuPublicProfile struct {
	HubHandle         string            `json:"hubHandle"`
	HubDisplayName    string            `json:"hubDisplayName"`
	OsuUserID         int64             `json:"osuUserId"`
	OsuUsername       string            `json:"osuUsername"`
	CountryCode       string            `json:"countryCode"`
	AvatarURL         string            `json:"avatarUrl"`
	GlobalRank        *int64            `json:"globalRank,omitempty"`
	PerformancePoints *float64          `json:"performancePoints,omitempty"`
	PlayCount         int64             `json:"playCount"`
	PlayTimeSeconds   int64             `json:"playTimeSeconds"`
	SharedReplayCount int               `json:"sharedReplayCount"`
	RecentReplays     []OsuPublicReplay `json:"recentReplays"`
}

func makeOsuShareID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate osu share id: %w", err)
	}
	return "osu_" + hex.EncodeToString(value), nil
}

func (s *Store) SaveOsuSync(ctx context.Context, userID int64, input OsuSyncInput) (OsuSyncResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OsuSyncResult{}, fmt.Errorf("begin osu sync: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO osu_profiles (
			user_id, osu_user_id, username, country_code, avatar_url, global_rank,
			performance_points, play_count, play_time_seconds
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (user_id) DO UPDATE SET
			osu_user_id = EXCLUDED.osu_user_id,
			username = EXCLUDED.username,
			country_code = EXCLUDED.country_code,
			avatar_url = EXCLUDED.avatar_url,
			global_rank = EXCLUDED.global_rank,
			performance_points = EXCLUDED.performance_points,
			play_count = EXCLUDED.play_count,
			play_time_seconds = EXCLUDED.play_time_seconds,
			updated_at = NOW()
	`, userID, input.Profile.OsuUserID, input.Profile.Username, input.Profile.CountryCode,
		input.Profile.AvatarURL, input.Profile.GlobalRank, input.Profile.PerformancePoints,
		input.Profile.PlayCount, input.Profile.PlayTimeSeconds); err != nil {
		return OsuSyncResult{}, fmt.Errorf("upsert osu profile: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO osu_beatmap_sets (set_key, online_id, title, artist, creator, source, cover_url)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (set_key) DO UPDATE SET
			online_id = EXCLUDED.online_id,
			title = EXCLUDED.title,
			artist = EXCLUDED.artist,
			creator = EXCLUDED.creator,
			source = EXCLUDED.source,
			cover_url = EXCLUDED.cover_url,
			updated_at = NOW()
	`, input.BeatmapSet.SetKey, input.BeatmapSet.OnlineID, input.BeatmapSet.Title,
		input.BeatmapSet.Artist, input.BeatmapSet.Creator, input.BeatmapSet.Source,
		input.BeatmapSet.CoverURL); err != nil {
		return OsuSyncResult{}, fmt.Errorf("upsert osu beatmap set: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO osu_beatmap_difficulties (
			difficulty_key, set_key, online_id, checksum, version, ruleset, star_rating,
			bpm, length_ms, circle_size, approach_rate, overall_difficulty, drain_rate, max_combo
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (difficulty_key) DO UPDATE SET
			set_key = EXCLUDED.set_key,
			online_id = EXCLUDED.online_id,
			checksum = EXCLUDED.checksum,
			version = EXCLUDED.version,
			ruleset = EXCLUDED.ruleset,
			star_rating = EXCLUDED.star_rating,
			bpm = EXCLUDED.bpm,
			length_ms = EXCLUDED.length_ms,
			circle_size = EXCLUDED.circle_size,
			approach_rate = EXCLUDED.approach_rate,
			overall_difficulty = EXCLUDED.overall_difficulty,
			drain_rate = EXCLUDED.drain_rate,
			max_combo = EXCLUDED.max_combo,
			updated_at = NOW()
	`, input.Difficulty.DifficultyKey, input.Difficulty.SetKey, input.Difficulty.OnlineID,
		input.Difficulty.Checksum, input.Difficulty.Version, input.Difficulty.Ruleset,
		input.Difficulty.StarRating, input.Difficulty.BPM, input.Difficulty.LengthMS,
		input.Difficulty.CircleSize, input.Difficulty.ApproachRate,
		input.Difficulty.OverallDifficulty, input.Difficulty.DrainRate,
		input.Difficulty.MaxCombo); err != nil {
		return OsuSyncResult{}, fmt.Errorf("upsert osu difficulty: %w", err)
	}

	var scoreID int64
	var existingHash string
	var shareID string
	created := false
	err = tx.QueryRow(ctx, `
		SELECT id, content_hash, share_id
		FROM osu_scores
		WHERE user_id = $1 AND (client_score_id = $2 OR content_hash = $3)
		ORDER BY CASE WHEN client_score_id = $2 THEN 0 ELSE 1 END
		LIMIT 1
		FOR UPDATE
	`, userID, input.Score.ClientScoreID, input.ContentHash).Scan(&scoreID, &existingHash, &shareID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return OsuSyncResult{}, fmt.Errorf("load existing osu score: %w", err)
	}
	if err == nil {
		if !strings.EqualFold(existingHash, input.ContentHash) {
			return OsuSyncResult{}, fmt.Errorf("client score id is already associated with different content")
		}
		if _, err := tx.Exec(ctx, `UPDATE osu_scores SET visibility = $1, updated_at = NOW() WHERE id = $2`, input.Visibility, scoreID); err != nil {
			return OsuSyncResult{}, fmt.Errorf("update osu score visibility: %w", err)
		}
	} else {
		shareID, err = makeOsuShareID()
		if err != nil {
			return OsuSyncResult{}, err
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO osu_scores (
				user_id, client_score_id, content_hash, difficulty_key, online_score_id,
				played_at, total_score, performance_points, accuracy, max_combo,
				count_300, count_100, count_50, count_miss, mods, passed, visibility, share_id
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
			RETURNING id
		`, userID, input.Score.ClientScoreID, input.ContentHash, input.Difficulty.DifficultyKey,
			input.Score.OnlineScoreID, input.Score.PlayedAt, input.Score.TotalScore,
			input.Score.PerformancePoints, input.Score.Accuracy, input.Score.MaxCombo,
			input.Score.Count300, input.Score.Count100, input.Score.Count50,
			input.Score.CountMiss, input.Score.Mods, input.Score.Passed,
			input.Visibility, shareID).Scan(&scoreID)
		if err != nil {
			return OsuSyncResult{}, fmt.Errorf("insert osu score: %w", err)
		}
		created = true
	}

	if input.Replay != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO osu_replay_files (score_id, replay_sha256, client_filename)
			VALUES ($1,$2,$3)
			ON CONFLICT (score_id) DO UPDATE SET
				storage_key = CASE WHEN osu_replay_files.replay_sha256 = EXCLUDED.replay_sha256 THEN osu_replay_files.storage_key ELSE '' END,
				byte_size = CASE WHEN osu_replay_files.replay_sha256 = EXCLUDED.replay_sha256 THEN osu_replay_files.byte_size ELSE 0 END,
				uploaded_at = CASE WHEN osu_replay_files.replay_sha256 = EXCLUDED.replay_sha256 THEN osu_replay_files.uploaded_at ELSE NULL END,
				replay_sha256 = EXCLUDED.replay_sha256,
				client_filename = EXCLUDED.client_filename,
				updated_at = NOW()
		`, scoreID, input.Replay.SHA256, input.Replay.ClientFilename); err != nil {
			return OsuSyncResult{}, fmt.Errorf("upsert osu replay metadata: %w", err)
		}
	}

	if input.Analysis != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO osu_replay_analyses (score_id, schema_version, engine_version, analysis_json)
			VALUES ($1,$2,$3,$4::jsonb)
			ON CONFLICT (score_id) DO UPDATE SET
				schema_version = EXCLUDED.schema_version,
				engine_version = EXCLUDED.engine_version,
				analysis_json = EXCLUDED.analysis_json,
				updated_at = NOW()
		`, scoreID, input.Analysis.SchemaVersion, input.Analysis.EngineVersion, string(input.Analysis.Payload)); err != nil {
			return OsuSyncResult{}, fmt.Errorf("upsert osu replay analysis: %w", err)
		}
	}

	replayUploadRequired := false
	if input.Replay != nil && input.Replay.UploadFile {
		var storedHash string
		var storedBytes int64
		if err := tx.QueryRow(ctx, `SELECT replay_sha256, byte_size FROM osu_replay_files WHERE score_id = $1`, scoreID).Scan(&storedHash, &storedBytes); err != nil {
			return OsuSyncResult{}, fmt.Errorf("check osu replay upload: %w", err)
		}
		replayUploadRequired = !strings.EqualFold(storedHash, input.Replay.SHA256) || storedBytes <= 0
	}

	if err := tx.Commit(ctx); err != nil {
		return OsuSyncResult{}, fmt.Errorf("commit osu sync: %w", err)
	}
	return OsuSyncResult{ShareID: shareID, Visibility: input.Visibility, Created: created, ReplayUploadRequired: replayUploadRequired}, nil
}

func (s *Store) GetOsuReplayUploadTarget(ctx context.Context, userID int64, shareID string) (OsuReplayUploadTarget, error) {
	var target OsuReplayUploadTarget
	if err := s.pool.QueryRow(ctx, `
		SELECT s.id, s.share_id, s.visibility, f.replay_sha256, f.storage_key, f.byte_size
		FROM osu_scores s
		JOIN osu_replay_files f ON f.score_id = s.id
		WHERE s.user_id = $1 AND s.share_id = $2
	`, userID, shareID).Scan(&target.ScoreID, &target.ShareID, &target.Visibility,
		&target.ReplaySHA256, &target.StorageKey, &target.ByteSize); err != nil {
		return OsuReplayUploadTarget{}, fmt.Errorf("load osu replay upload target: %w", err)
	}
	return target, nil
}

func (s *Store) CompleteOsuReplayUpload(ctx context.Context, scoreID int64, storageKey string, byteSize int64) error {
	if _, err := s.pool.Exec(ctx, `
		UPDATE osu_replay_files
		SET storage_key = $1, byte_size = $2, uploaded_at = NOW(), updated_at = NOW()
		WHERE score_id = $3
	`, storageKey, byteSize, scoreID); err != nil {
		return fmt.Errorf("complete osu replay upload: %w", err)
	}
	return nil
}

func (s *Store) GetOsuPublicReplay(ctx context.Context, shareID string) (OsuPublicReplay, error) {
	row := s.pool.QueryRow(ctx, osuPublicReplaySelect+`
		WHERE s.share_id = $1 AND s.visibility IN ('public', 'unlisted')
	`, shareID)
	return scanOsuPublicReplay(row)
}

func (s *Store) ListOsuCommunity(ctx context.Context, limit int) ([]OsuPublicReplay, error) {
	rows, err := s.pool.Query(ctx, osuPublicReplayListSelect+`
		WHERE s.visibility = 'public'
		ORDER BY s.played_at DESC, s.id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list osu community: %w", err)
	}
	defer rows.Close()
	return collectOsuPublicReplays(rows)
}

func (s *Store) GetOsuPublicProfile(ctx context.Context, handle string, limit int) (OsuPublicProfile, error) {
	var profile OsuPublicProfile
	if err := s.pool.QueryRow(ctx, `
		SELECT hui.user_handle, hui.user_display_name, p.osu_user_id, p.username,
			p.country_code, p.avatar_url, p.global_rank, p.performance_points,
			p.play_count, p.play_time_seconds,
			COUNT(s.id) FILTER (WHERE s.visibility = 'public')::integer
		FROM hub_user_identity hui
		JOIN osu_profiles p ON p.user_id = hui.user_id
		LEFT JOIN osu_scores s ON s.user_id = hui.user_id
		WHERE LOWER(hui.user_handle) = LOWER($1)
		GROUP BY hui.user_handle, hui.user_display_name, p.osu_user_id, p.username,
			p.country_code, p.avatar_url, p.global_rank, p.performance_points,
			p.play_count, p.play_time_seconds
		HAVING COUNT(s.id) FILTER (WHERE s.visibility = 'public') > 0
	`, handle).Scan(&profile.HubHandle, &profile.HubDisplayName, &profile.OsuUserID,
		&profile.OsuUsername, &profile.CountryCode, &profile.AvatarURL, &profile.GlobalRank,
		&profile.PerformancePoints, &profile.PlayCount, &profile.PlayTimeSeconds,
		&profile.SharedReplayCount); err != nil {
		return OsuPublicProfile{}, fmt.Errorf("load osu profile: %w", err)
	}

	rows, err := s.pool.Query(ctx, osuPublicReplayListSelect+`
		WHERE s.visibility = 'public' AND LOWER(hui.user_handle) = LOWER($1)
		ORDER BY s.played_at DESC, s.id DESC
		LIMIT $2
	`, handle, limit)
	if err != nil {
		return OsuPublicProfile{}, fmt.Errorf("list osu profile replays: %w", err)
	}
	defer rows.Close()
	profile.RecentReplays, err = collectOsuPublicReplays(rows)
	return profile, err
}

func (s *Store) GetOsuReplayFile(ctx context.Context, shareID string) (string, string, int64, error) {
	var storageKey, contentType string
	var byteSize int64
	if err := s.pool.QueryRow(ctx, `
		SELECT f.storage_key, f.content_type, f.byte_size
		FROM osu_replay_files f
		JOIN osu_scores s ON s.id = f.score_id
		WHERE s.share_id = $1
		  AND s.visibility IN ('public', 'unlisted')
		  AND f.storage_key <> ''
		  AND f.byte_size > 0
	`, shareID).Scan(&storageKey, &contentType, &byteSize); err != nil {
		return "", "", 0, fmt.Errorf("load osu replay file: %w", err)
	}
	return storageKey, contentType, byteSize, nil
}

const osuPublicReplayColumns = `
	SELECT s.share_id, s.visibility, hui.user_handle, hui.user_display_name,
		p.osu_user_id, p.username, p.country_code, p.avatar_url,
		bs.online_id, d.online_id, bs.title, bs.artist, bs.creator, bs.cover_url,
		d.version, d.ruleset, d.star_rating, d.bpm, d.length_ms,
		s.played_at, s.total_score, s.performance_points, s.accuracy, s.max_combo,
		s.count_300, s.count_100, s.count_50, s.count_miss, s.mods, s.passed,
		COALESCE(f.byte_size, 0) > 0
`

const osuPublicReplayJoins = `
	FROM osu_scores s
	JOIN hub_user_identity hui ON hui.user_id = s.user_id
	JOIN osu_profiles p ON p.user_id = s.user_id
	JOIN osu_beatmap_difficulties d ON d.difficulty_key = s.difficulty_key
	JOIN osu_beatmap_sets bs ON bs.set_key = d.set_key
	LEFT JOIN osu_replay_files f ON f.score_id = s.id
	LEFT JOIN osu_replay_analyses a ON a.score_id = s.id
`

const osuPublicReplaySelect = osuPublicReplayColumns + `,
		COALESCE(a.schema_version, 0), COALESCE(a.engine_version, ''),
		COALESCE(a.analysis_json, '{}'::jsonb)::text
` + osuPublicReplayJoins

// Feed and profile lists intentionally omit exact judgements. A single analysis can
// approach the request limit; details are loaded only on the individual share page.
const osuPublicReplayListSelect = osuPublicReplayColumns + `,
		0, '', '{}'::text
` + osuPublicReplayJoins

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOsuPublicReplay(row rowScanner) (OsuPublicReplay, error) {
	var replay OsuPublicReplay
	var analysisText string
	if err := row.Scan(&replay.ShareID, &replay.Visibility, &replay.HubHandle,
		&replay.HubDisplayName, &replay.OsuUserID, &replay.OsuUsername,
		&replay.CountryCode, &replay.AvatarURL, &replay.BeatmapSetID,
		&replay.BeatmapID, &replay.Title, &replay.Artist, &replay.Creator,
		&replay.CoverURL, &replay.Difficulty, &replay.Ruleset,
		&replay.StarRating, &replay.BPM, &replay.LengthMS, &replay.PlayedAt,
		&replay.TotalScore, &replay.PerformancePoints, &replay.Accuracy,
		&replay.MaxCombo, &replay.Count300, &replay.Count100, &replay.Count50,
		&replay.CountMiss, &replay.Mods, &replay.Passed, &replay.HasReplayFile,
		&replay.AnalysisSchema, &replay.AnalysisEngine, &analysisText); err != nil {
		return OsuPublicReplay{}, fmt.Errorf("scan osu public replay: %w", err)
	}
	replay.Analysis = json.RawMessage(analysisText)
	return replay, nil
}

func collectOsuPublicReplays(rows pgx.Rows) ([]OsuPublicReplay, error) {
	replays := make([]OsuPublicReplay, 0)
	for rows.Next() {
		replay, err := scanOsuPublicReplay(rows)
		if err != nil {
			return nil, err
		}
		replays = append(replays, replay)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate osu public replays: %w", err)
	}
	return replays, nil
}
