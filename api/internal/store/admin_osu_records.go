package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

type AdminOsuRecordFilter struct {
	Search, Kind  string
	Limit, Offset int
}

type AdminOsuAccount struct {
	Provider  string    `json:"provider"`
	Username  string    `json:"username"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"createdAt"`
}

type AdminOsuPlayer struct {
	UserID            int64             `json:"userId"`
	Handle            string            `json:"handle"`
	DisplayName       string            `json:"displayName"`
	OsuUserID         int64             `json:"osuUserId"`
	Username          string            `json:"username"`
	Country           string            `json:"country"`
	CreatedAt         time.Time         `json:"createdAt"`
	ProfileUpdatedAt  *time.Time        `json:"profileUpdatedAt"`
	LastScoreAt       *time.Time        `json:"lastScoreAt"`
	Scores            int64             `json:"scores"`
	Public            int64             `json:"public"`
	Unlisted          int64             `json:"unlisted"`
	Private           int64             `json:"private"`
	Replays           int64             `json:"replays"`
	ReplayBytes       int64             `json:"replayBytes"`
	ActiveCredentials int64             `json:"activeCredentials"`
	LastCredentialUse *time.Time        `json:"lastCredentialUse"`
	Accounts          []AdminOsuAccount `json:"accounts"`
}

type AdminOsuPlayers struct {
	Items []AdminOsuPlayer `json:"items"`
	Total int64            `json:"total"`
}

type AdminOsuBeatmap struct {
	Key         string     `json:"key"`
	OnlineID    int64      `json:"onlineId"`
	SetOnlineID int64      `json:"setOnlineId"`
	Title       string     `json:"title"`
	Artist      string     `json:"artist"`
	Creator     string     `json:"creator"`
	Version     string     `json:"version"`
	Ruleset     string     `json:"ruleset"`
	Stars       float64    `json:"stars"`
	BPM         float64    `json:"bpm"`
	LengthMS    int64      `json:"lengthMs"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	LastScoreAt *time.Time `json:"lastScoreAt"`
	Scores      int64      `json:"scores"`
	Players     int64      `json:"players"`
	Public      int64      `json:"public"`
	Unlisted    int64      `json:"unlisted"`
	Private     int64      `json:"private"`
	Replays     int64      `json:"replays"`
	ReplayBytes int64      `json:"replayBytes"`
}

type AdminOsuBeatmaps struct {
	Items []AdminOsuBeatmap `json:"items"`
	Total int64             `json:"total"`
}

const adminPlayersFrom = ` FROM hub_users h JOIN hub_user_identity u ON u.user_id = h.id
LEFT JOIN osu_profiles p ON p.user_id = h.id `
const adminPlayersWhere = ` WHERE ($1 = '' OR strpos(lower(concat_ws(' ', u.user_handle, u.user_display_name, p.username, p.osu_user_id::text)), lower($1)) > 0)
AND ($2 = '' OR ($2 = 'synced' AND p.user_id IS NOT NULL) OR ($2 = 'unsynced' AND p.user_id IS NULL)) `

func (s *Store) GetAdminOsuPlayers(ctx context.Context, f AdminOsuRecordFilter) (AdminOsuPlayers, error) {
	result := AdminOsuPlayers{Items: make([]AdminOsuPlayer, 0)}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if err = tx.QueryRow(ctx, `SELECT count(*)`+adminPlayersFrom+adminPlayersWhere, f.Search, f.Kind).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `SELECT h.id, u.user_handle, u.user_display_name,
COALESCE(p.osu_user_id, 0), COALESCE(p.username, ''), COALESCE(p.country_code, ''), h.created_at, p.updated_at,
(SELECT max(created_at) FROM osu_scores WHERE user_id = h.id),
(SELECT count(*) FROM osu_scores WHERE user_id = h.id),
(SELECT count(*) FROM osu_scores WHERE user_id = h.id AND visibility = 'public'),
(SELECT count(*) FROM osu_scores WHERE user_id = h.id AND visibility = 'unlisted'),
(SELECT count(*) FROM osu_scores WHERE user_id = h.id AND visibility = 'private'),
(SELECT count(*) FROM osu_replay_files r JOIN osu_scores s ON s.id = r.score_id WHERE s.user_id = h.id AND r.storage_key <> '' AND r.uploaded_at IS NOT NULL),
(SELECT COALESCE(sum(r.byte_size), 0) FROM osu_replay_files r JOIN osu_scores s ON s.id = r.score_id WHERE s.user_id = h.id AND r.storage_key <> '' AND r.uploaded_at IS NOT NULL),
(SELECT count(*) FROM upload_tokens WHERE user_id = h.id AND revoked_at IS NULL),
(SELECT max(last_used_at) FROM upload_tokens WHERE user_id = h.id AND revoked_at IS NULL),
COALESCE((SELECT jsonb_agg(jsonb_build_object('provider', provider, 'username', username, 'verified', verified, 'createdAt', created_at) ORDER BY provider) FROM linked_accounts WHERE user_id = h.id), '[]'::jsonb)
`+adminPlayersFrom+adminPlayersWhere+` ORDER BY h.created_at DESC, h.id DESC LIMIT $3 OFFSET $4`, f.Search, f.Kind, f.Limit, f.Offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var p AdminOsuPlayer
		var accounts []byte
		if err = rows.Scan(&p.UserID, &p.Handle, &p.DisplayName, &p.OsuUserID, &p.Username, &p.Country, &p.CreatedAt, &p.ProfileUpdatedAt, &p.LastScoreAt, &p.Scores, &p.Public, &p.Unlisted, &p.Private, &p.Replays, &p.ReplayBytes, &p.ActiveCredentials, &p.LastCredentialUse, &accounts); err != nil {
			return result, err
		}
		if err = json.Unmarshal(accounts, &p.Accounts); err != nil {
			return result, err
		}
		result.Items = append(result.Items, p)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

const adminBeatmapsFrom = ` FROM osu_beatmap_difficulties d JOIN osu_beatmap_sets b ON b.set_key = d.set_key `
const adminBeatmapsWhere = ` WHERE ($1 = '' OR strpos(lower(concat_ws(' ', b.title, b.artist, b.creator, d.version, d.online_id::text)), lower($1)) > 0)
AND ($2 = '' OR ($2 = 'online' AND d.online_id > 0) OR ($2 = 'local' AND d.online_id = 0)) `

func (s *Store) GetAdminOsuBeatmaps(ctx context.Context, f AdminOsuRecordFilter) (AdminOsuBeatmaps, error) {
	result := AdminOsuBeatmaps{Items: make([]AdminOsuBeatmap, 0)}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if err = tx.QueryRow(ctx, `SELECT count(*)`+adminBeatmapsFrom+adminBeatmapsWhere, f.Search, f.Kind).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `SELECT d.difficulty_key, d.online_id, b.online_id, b.title, b.artist, b.creator, d.version, d.ruleset, d.star_rating, d.bpm, d.length_ms, d.updated_at,
(SELECT max(created_at) FROM osu_scores WHERE difficulty_key = d.difficulty_key),
(SELECT count(*) FROM osu_scores WHERE difficulty_key = d.difficulty_key),
(SELECT count(DISTINCT user_id) FROM osu_scores WHERE difficulty_key = d.difficulty_key),
(SELECT count(*) FROM osu_scores WHERE difficulty_key = d.difficulty_key AND visibility = 'public'),
(SELECT count(*) FROM osu_scores WHERE difficulty_key = d.difficulty_key AND visibility = 'unlisted'),
(SELECT count(*) FROM osu_scores WHERE difficulty_key = d.difficulty_key AND visibility = 'private'),
(SELECT count(*) FROM osu_replay_files r JOIN osu_scores s ON s.id = r.score_id WHERE s.difficulty_key = d.difficulty_key AND r.storage_key <> '' AND r.uploaded_at IS NOT NULL),
(SELECT COALESCE(sum(r.byte_size), 0) FROM osu_replay_files r JOIN osu_scores s ON s.id = r.score_id WHERE s.difficulty_key = d.difficulty_key AND r.storage_key <> '' AND r.uploaded_at IS NOT NULL)
`+adminBeatmapsFrom+adminBeatmapsWhere+` ORDER BY d.updated_at DESC, d.difficulty_key LIMIT $3 OFFSET $4`, f.Search, f.Kind, f.Limit, f.Offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var b AdminOsuBeatmap
		if err = rows.Scan(&b.Key, &b.OnlineID, &b.SetOnlineID, &b.Title, &b.Artist, &b.Creator, &b.Version, &b.Ruleset, &b.Stars, &b.BPM, &b.LengthMS, &b.UpdatedAt, &b.LastScoreAt, &b.Scores, &b.Players, &b.Public, &b.Unlisted, &b.Private, &b.Replays, &b.ReplayBytes); err != nil {
			return result, err
		}
		result.Items = append(result.Items, b)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
