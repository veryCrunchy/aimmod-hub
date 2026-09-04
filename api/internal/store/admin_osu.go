package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type AdminOsuFilter struct {
	Search, Visibility, Status string
	Limit, Offset              int
}

type AdminOsuSummary struct {
	Scores            int64 `json:"scores"`
	Public            int64 `json:"public"`
	Unlisted          int64 `json:"unlisted"`
	Private           int64 `json:"private"`
	Uploaded          int64 `json:"uploaded"`
	Pending           int64 `json:"pending"`
	ReplayBytes       int64 `json:"replayBytes"`
	Profiles          int64 `json:"profiles"`
	PublicProfiles    int64 `json:"publicProfiles"`
	ActiveCredentials int64 `json:"activeCredentials"`
	ConnectedAccounts int64 `json:"connectedAccounts"`
	PendingDevices    int64 `json:"pendingDevices"`
}

// This projection deliberately excludes replay payloads, file paths and credentials.
type AdminOsuShare struct {
	ID         int64      `json:"id"`
	Handle     string     `json:"handle"`
	Username   string     `json:"username"`
	Title      string     `json:"title"`
	Difficulty string     `json:"difficulty"`
	Visibility string     `json:"visibility"`
	Status     string     `json:"status"`
	ByteSize   int64      `json:"byteSize"`
	CreatedAt  time.Time  `json:"createdAt"`
	UploadedAt *time.Time `json:"uploadedAt"`
}

type AdminOsuOverview struct {
	Summary AdminOsuSummary `json:"summary"`
	Items   []AdminOsuShare `json:"items"`
	Total   int64           `json:"total"`
}

const adminOsuFrom = ` FROM osu_scores s
JOIN hub_user_identity u ON u.user_id = s.user_id
LEFT JOIN osu_profiles p ON p.user_id = s.user_id
JOIN osu_beatmap_difficulties d ON d.difficulty_key = s.difficulty_key
JOIN osu_beatmap_sets b ON b.set_key = d.set_key
LEFT JOIN osu_replay_files f ON f.score_id = s.id `

const adminOsuStatus = `CASE WHEN f.score_id IS NULL THEN 'none' WHEN f.storage_key <> '' AND f.uploaded_at IS NOT NULL THEN 'uploaded' ELSE 'pending' END`

const adminOsuWhere = ` WHERE ($1 = '' OR strpos(lower(concat_ws(' ', u.user_handle, p.username, b.title, d.version)), lower($1)) > 0)
AND ($2 = '' OR s.visibility = $2)
AND ($3 = '' OR (` + adminOsuStatus + `) = $3)`

func (s *Store) GetAdminOsuOverview(ctx context.Context, filter AdminOsuFilter) (AdminOsuOverview, error) {
	result := AdminOsuOverview{Items: make([]AdminOsuShare, 0)}
	// Keep totals and pagination consistent while uploads arrive.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE s.visibility = 'public'),
count(*) FILTER (WHERE s.visibility = 'unlisted'), count(*) FILTER (WHERE s.visibility = 'private'),
count(*) FILTER (WHERE f.storage_key <> '' AND f.uploaded_at IS NOT NULL),
count(*) FILTER (WHERE f.score_id IS NOT NULL AND (f.storage_key = '' OR f.uploaded_at IS NULL)),
COALESCE(sum(f.byte_size) FILTER (WHERE f.storage_key <> '' AND f.uploaded_at IS NOT NULL), 0),
(SELECT count(*) FROM osu_profiles), count(DISTINCT s.user_id) FILTER (WHERE s.visibility = 'public'),
(SELECT count(*) FROM upload_tokens WHERE revoked_at IS NULL),
(SELECT count(*) FROM linked_accounts),
(SELECT count(*) FROM device_link_requests WHERE status = 'pending' AND expires_at > NOW())
FROM osu_scores s LEFT JOIN osu_replay_files f ON f.score_id = s.id`).Scan(
		&result.Summary.Scores, &result.Summary.Public, &result.Summary.Unlisted, &result.Summary.Private,
		&result.Summary.Uploaded, &result.Summary.Pending, &result.Summary.ReplayBytes,
		&result.Summary.Profiles, &result.Summary.PublicProfiles, &result.Summary.ActiveCredentials,
		&result.Summary.ConnectedAccounts, &result.Summary.PendingDevices)
	if err != nil {
		return result, err
	}
	args := []any{filter.Search, filter.Visibility, filter.Status}
	if err = tx.QueryRow(ctx, `SELECT count(*)`+adminOsuFrom+adminOsuWhere, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `SELECT s.id, u.user_handle, COALESCE(p.username, ''), b.title, d.version,
s.visibility, `+adminOsuStatus+`, COALESCE(f.byte_size, 0), s.created_at, f.uploaded_at`+adminOsuFrom+adminOsuWhere+` ORDER BY s.created_at DESC, s.id DESC LIMIT $4 OFFSET $5`, append(args, filter.Limit, filter.Offset)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item AdminOsuShare
		if err = rows.Scan(&item.ID, &item.Handle, &item.Username, &item.Title, &item.Difficulty, &item.Visibility, &item.Status, &item.ByteSize, &item.CreatedAt, &item.UploadedAt); err != nil {
			return result, err
		}
		result.Items = append(result.Items, item)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
