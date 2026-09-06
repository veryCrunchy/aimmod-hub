package store

import (
	"context"
	"encoding/json"
	"time"
)

type IndexedOsuScore struct {
	ID        int64
	UserID    int64
	Mode      string
	PlayedAt  time.Time
	HasReplay bool
	Item      json.RawMessage
}

func (s *Store) SaveIndexedOsuScores(ctx context.Context, items []IndexedOsuScore) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, item := range items {
		if item.ID <= 0 || item.UserID <= 0 || item.PlayedAt.IsZero() {
			continue
		}
		_, err = tx.Exec(ctx, `INSERT INTO osu_public_score_index(score_id,osu_user_id,ruleset,played_at,has_replay,item) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(score_id) DO UPDATE SET item=EXCLUDED.item,has_replay=EXCLUDED.has_replay,updated_at=NOW()`, item.ID, item.UserID, item.Mode, item.PlayedAt, item.HasReplay, item.Item)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) ListIndexedOsuScores(ctx context.Context, limit int, replaysOnly bool) ([]json.RawMessage, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT item FROM osu_public_score_index WHERE ($2=false OR has_replay) ORDER BY played_at DESC,score_id DESC LIMIT $1`, limit, replaysOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var item json.RawMessage
		if err = rows.Scan(&item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) ClaimOsuScoreIndexPlayer(ctx context.Context) (int64, string, error) {
	var id int64
	var mode string
	err := s.pool.QueryRow(ctx, `UPDATE osu_public_players SET scores_next_refresh=NOW()+INTERVAL '2 minutes' WHERE (osu_user_id,ruleset)=(SELECT osu_user_id,ruleset FROM osu_public_players WHERE scores_next_refresh<=NOW() ORDER BY scores_next_refresh,osu_user_id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING osu_user_id,ruleset`).Scan(&id, &mode)
	return id, mode, err
}
func (s *Store) FinishOsuScoreIndexPlayer(ctx context.Context, id int64, mode string) error {
	_, err := s.pool.Exec(ctx, `UPDATE osu_public_players SET scores_next_refresh=NOW()+INTERVAL '12 hours' WHERE osu_user_id=$1 AND ruleset=$2`, id, mode)
	return err
}

func (s *Store) GetIndexedOsuScore(ctx context.Context, id int64) (OsuPublicReplay, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, `SELECT item FROM osu_public_score_index WHERE score_id=$1`, id).Scan(&data)
	if err != nil {
		return OsuPublicReplay{}, err
	}
	var item OsuPublicReplay
	err = json.Unmarshal(data, &item)
	return item, err
}
