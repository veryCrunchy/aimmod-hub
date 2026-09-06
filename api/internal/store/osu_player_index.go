package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Public provider records are independent of Hub identities and private uploads.
func (s *Store) SaveIndexedOsuPlayers(ctx context.Context, mode string, players []OsuPublicProfile) error {
	if mode != "osu" && mode != "taiko" && mode != "fruits" && mode != "mania" {
		return fmt.Errorf("invalid ruleset")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, p := range players {
		if p.OsuUserID <= 0 || strings.TrimSpace(p.OsuUsername) == "" {
			continue
		}
		p.HubHandle = ""
		p.HubDisplayName = ""
		p.SharedReplayCount = 0
		p.RecentReplays = []OsuPublicReplay{}
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		// Search results contain identity only; do not erase previously observed statistics.
		if p.PerformancePoints != nil && p.GlobalRank == nil {
			var fields map[string]any
			if err = json.Unmarshal(data, &fields); err != nil {
				return err
			}
			fields["globalRank"] = nil
			data, err = json.Marshal(fields)
			if err != nil {
				return err
			}
		}
		if p.PerformancePoints == nil {
			var fields map[string]any
			if err = json.Unmarshal(data, &fields); err != nil {
				return err
			}
			for _, key := range []string{"globalRank", "performancePoints", "playCount", "playTimeSeconds"} {
				delete(fields, key)
			}
			data, err = json.Marshal(fields)
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO osu_public_players(osu_user_id,ruleset,username,profile) VALUES($1,$2,$3,$4)
   ON CONFLICT(osu_user_id,ruleset) DO UPDATE SET username=EXCLUDED.username,profile=osu_public_players.profile || EXCLUDED.profile,updated_at=NOW()`, p.OsuUserID, mode, p.OsuUsername, data)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) GetIndexedOsuProfile(ctx context.Context, identifier, mode string) (OsuPublicProfile, error) {
	var data []byte
	id, _ := strconv.ParseInt(identifier, 10, 64)
	err := s.pool.QueryRow(ctx, `SELECT profile FROM osu_public_players WHERE ($2='' OR ruleset=$2) AND updated_at>NOW()-INTERVAL '30 days' AND (($3>0 AND osu_user_id=$3) OR ($3=0 AND lower(username)=lower($1))) ORDER BY (ruleset='osu') DESC,updated_at DESC LIMIT 1`, identifier, mode, id).Scan(&data)
	if err != nil {
		return OsuPublicProfile{}, err
	}
	var p OsuPublicProfile
	err = json.Unmarshal(data, &p)
	return p, err
}

func (s *Store) ListIndexedOsuPlayers(ctx context.Context, mode, query string, page int) ([]OsuPublicProfile, error) {
	rows, err := s.pool.Query(ctx, `SELECT profile FROM osu_public_players WHERE ruleset=$1 AND updated_at>NOW()-INTERVAL '30 days' AND ($2='' OR strpos(lower(username),lower($2))>0 OR osu_user_id::text=$2) ORDER BY NULLIF(profile->>'globalRank','')::bigint NULLS LAST, lower(username),osu_user_id LIMIT 51 OFFSET $3`, mode, query, (page-1)*50)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []OsuPublicProfile{}
	for rows.Next() {
		var data []byte
		var p OsuPublicProfile
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &p); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (s *Store) ClaimOsuPlayerIndexPage(ctx context.Context) (string, int, error) {
	var mode string
	var page int
	err := s.pool.QueryRow(ctx, `UPDATE osu_player_index_progress SET next_attempt=NOW()+INTERVAL '2 minutes' WHERE ruleset=(SELECT ruleset FROM osu_player_index_progress WHERE next_attempt<=NOW() ORDER BY next_attempt,(ruleset='osu') DESC,ruleset FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING ruleset,page`).Scan(&mode, &page)
	return mode, page, err
}
func (s *Store) FinishOsuPlayerIndexPage(ctx context.Context, mode string, page, next int) error {
	delay := 60
	if next == 0 {
		next = 1
		delay = 21600
	}
	_, err := s.pool.Exec(ctx, `UPDATE osu_player_index_progress SET page=$3,next_attempt=NOW()+($4*INTERVAL '1 second') WHERE ruleset=$1 AND page=$2`, mode, page, next, delay)
	return err
}
