package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type KovaaksUserCacheEntry struct {
	SteamID     string
	Username    string
	DisplayName string
	AvatarURL   string
	Country     string
}

// UpsertKovaaksUsers inserts or updates KovaaK's player records into the
// kovaaks_user_cache table. Called after every KovaaK's search API response
// so the cache grows naturally over time.
func (s *Store) UpsertKovaaksUsers(ctx context.Context, users []KovaaksUserCacheEntry) error {
	if len(users) == 0 {
		return nil
	}
	for _, u := range users {
		if u.SteamID == "" {
			continue
		}
		_, err := s.pool.Exec(ctx, `
			INSERT INTO kovaaks_user_cache
				(steam_id, kovaaks_username, steam_display_name, avatar_url, country, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
			ON CONFLICT (steam_id) DO UPDATE SET
				kovaaks_username   = EXCLUDED.kovaaks_username,
				steam_display_name = EXCLUDED.steam_display_name,
				avatar_url         = EXCLUDED.avatar_url,
				country            = EXCLUDED.country,
				updated_at         = NOW()
		`, u.SteamID, u.Username, u.DisplayName, u.AvatarURL, u.Country)
		if err != nil {
			return fmt.Errorf("upsert kovaaks user cache: %w", err)
		}
	}
	return nil
}

// GetKovaaksUserBySteamId looks up a single cached KovaaK's player by their
// Steam64 ID. Returns nil (no error) when the Steam ID is not in the cache.
func (s *Store) GetKovaaksUserBySteamId(ctx context.Context, steamId string) (*KovaaksUserCacheEntry, error) {
	if steamId == "" {
		return nil, nil
	}
	var e KovaaksUserCacheEntry
	err := s.pool.QueryRow(ctx, `
		SELECT steam_id, kovaaks_username, steam_display_name, avatar_url, country
		FROM kovaaks_user_cache
		WHERE steam_id = $1
	`, steamId).Scan(&e.SteamID, &e.Username, &e.DisplayName, &e.AvatarURL, &e.Country)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get kovaaks user by steam id: %w", err)
	}
	return &e, nil
}

// SearchKovaaksUserCache returns cached KovaaK's players whose username or
// Steam display name contains the query (case-insensitive). Results are
// ordered: exact username match first, then prefix matches, then contains.
func (s *Store) SearchKovaaksUserCache(ctx context.Context, query string, limit int) ([]KovaaksUserCacheEntry, error) {
	if query == "" || limit <= 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT steam_id, kovaaks_username, steam_display_name, avatar_url, country
		FROM kovaaks_user_cache
		WHERE LOWER(kovaaks_username)   LIKE '%' || LOWER($1) || '%'
		   OR LOWER(steam_display_name) LIKE '%' || LOWER($1) || '%'
		ORDER BY
			CASE
				WHEN LOWER(kovaaks_username) = LOWER($1)            THEN 0
				WHEN LOWER(kovaaks_username) LIKE LOWER($1) || '%'  THEN 1
				WHEN LOWER(steam_display_name) LIKE LOWER($1) || '%' THEN 2
				ELSE 3
			END,
			kovaaks_username ASC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search kovaaks user cache: %w", err)
	}
	defer rows.Close()

	var results []KovaaksUserCacheEntry
	for rows.Next() {
		var e KovaaksUserCacheEntry
		if err := rows.Scan(&e.SteamID, &e.Username, &e.DisplayName, &e.AvatarURL, &e.Country); err != nil {
			return nil, fmt.Errorf("scan kovaaks user cache: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}
