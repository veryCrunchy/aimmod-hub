package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type BenchmarkUserIdentity struct {
	UserHandle      string
	DisplayName     string
	AvatarURL       string
	SteamID         string
	KovaaksUsername string
}

// ListUsersWithBenchmarkIdentity returns all hub users who have a steam linked account
// and a profile handle, along with their Kovaaks username if available.
func (s *Store) ListUsersWithBenchmarkIdentity(ctx context.Context) ([]BenchmarkUserIdentity, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			hui.user_handle,
			hui.user_display_name,
			COALESCE(steam.avatar_url, hui.avatar_url, '') AS avatar_url,
			steam.provider_account_id AS steam_id,
			COALESCE(NULLIF(TRIM(kovaaks.username), ''), NULLIF(TRIM(kovaaks.display_name), ''), kovaaks.provider_account_id, hui.user_handle) AS kovaaks_username
		FROM hub_user_identity hui
		JOIN linked_accounts steam
			ON steam.user_id = hui.user_id AND steam.provider = 'steam'
		LEFT JOIN linked_accounts kovaaks
			ON kovaaks.user_id = hui.user_id AND kovaaks.provider = 'kovaaks'
		WHERE TRIM(steam.provider_account_id) != ''
		ORDER BY hui.user_handle
	`)
	if err != nil {
		return nil, fmt.Errorf("list users with benchmark identity: %w", err)
	}
	defer rows.Close()

	var result []BenchmarkUserIdentity
	for rows.Next() {
		var r BenchmarkUserIdentity
		if err := rows.Scan(&r.UserHandle, &r.DisplayName, &r.AvatarURL, &r.SteamID, &r.KovaaksUsername); err != nil {
			return nil, fmt.Errorf("scan benchmark user identity: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate benchmark user identities: %w", err)
	}
	return result, nil
}

type BenchmarkIdentityRecord struct {
	KovaaksUsername string
	SteamID         string
}

func (s *Store) GetBenchmarkIdentityByHandle(ctx context.Context, handle string) (BenchmarkIdentityRecord, error) {
	var result BenchmarkIdentityRecord
	resolvedUser, err := s.resolveUserIdentityByHandle(ctx, handle)
	if err != nil {
		return result, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT provider, provider_account_id, username, display_name
		FROM linked_accounts
		WHERE user_id = $1
	`, resolvedUser.UserID)
	if err != nil {
		return result, fmt.Errorf("load benchmark identity: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var provider string
		var providerAccountID string
		var username string
		var displayName string
		if err := rows.Scan(&provider, &providerAccountID, &username, &displayName); err != nil {
			return result, fmt.Errorf("scan benchmark identity: %w", err)
		}
		switch provider {
		case "kovaaks":
			if result.KovaaksUsername == "" {
				if username != "" {
					result.KovaaksUsername = username
				} else if displayName != "" {
					result.KovaaksUsername = displayName
				} else {
					result.KovaaksUsername = providerAccountID
				}
			}
		case "steam":
			if result.SteamID == "" {
				result.SteamID = providerAccountID
			}
		}
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate benchmark identity: %w", err)
	}

	if result.KovaaksUsername == "" {
		result.KovaaksUsername = resolvedUser.UserHandle
	}
	return result, nil
}

// GetBenchmarkIdentityByKovaaksUsername returns the BenchmarkUserIdentity for the hub user
// whose KovaaK's linked account username or display name matches the given value.
// Returns nil, nil if no such user exists.
func (s *Store) GetBenchmarkIdentityByKovaaksUsername(ctx context.Context, username string) (*BenchmarkUserIdentity, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, nil
	}
	row := s.pool.QueryRow(ctx, `
		SELECT
			hui.user_handle,
			hui.user_display_name,
			COALESCE(steam.avatar_url, hui.avatar_url, '') AS avatar_url,
			COALESCE(NULLIF(TRIM(steam.provider_account_id), ''), '') AS steam_id,
			COALESCE(NULLIF(TRIM(kovaaks.username), ''), NULLIF(TRIM(kovaaks.display_name), ''), kovaaks.provider_account_id, hui.user_handle) AS kovaaks_username
		FROM hub_user_identity hui
		LEFT JOIN linked_accounts steam
			ON steam.user_id = hui.user_id AND steam.provider = 'steam'
		JOIN linked_accounts kovaaks
			ON kovaaks.user_id = hui.user_id AND kovaaks.provider = 'kovaaks'
		WHERE LOWER(TRIM(kovaaks.username)) = LOWER($1)
		   OR LOWER(TRIM(kovaaks.display_name)) = LOWER($1)
		LIMIT 1
	`, username)
	var r BenchmarkUserIdentity
	if err := row.Scan(&r.UserHandle, &r.DisplayName, &r.AvatarURL, &r.SteamID, &r.KovaaksUsername); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get benchmark identity by kovaaks username: %w", err)
	}
	return &r, nil
}

// GetBenchmarkIdentityBySteamId returns the BenchmarkUserIdentity for the hub user
// who has linked the given Steam ID. Returns nil, nil if no AimMod user has that Steam ID.
func (s *Store) GetBenchmarkIdentityBySteamId(ctx context.Context, steamId string) (*BenchmarkUserIdentity, error) {
	steamId = strings.TrimSpace(steamId)
	if steamId == "" {
		return nil, nil
	}
	row := s.pool.QueryRow(ctx, `
		SELECT
			hui.user_handle,
			hui.user_display_name,
			COALESCE(steam.avatar_url, hui.avatar_url, '') AS avatar_url,
			steam.provider_account_id AS steam_id,
			COALESCE(NULLIF(TRIM(kovaaks.username), ''), NULLIF(TRIM(kovaaks.display_name), ''), kovaaks.provider_account_id, hui.user_handle) AS kovaaks_username
		FROM hub_user_identity hui
		JOIN linked_accounts steam
			ON steam.user_id = hui.user_id AND steam.provider = 'steam'
		LEFT JOIN linked_accounts kovaaks
			ON kovaaks.user_id = hui.user_id AND kovaaks.provider = 'kovaaks'
		WHERE TRIM(steam.provider_account_id) = $1
		LIMIT 1
	`, steamId)
	var r BenchmarkUserIdentity
	if err := row.Scan(&r.UserHandle, &r.DisplayName, &r.AvatarURL, &r.SteamID, &r.KovaaksUsername); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get benchmark identity by steam id: %w", err)
	}
	return &r, nil
}
