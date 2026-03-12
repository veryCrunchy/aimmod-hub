package store

import (
	"context"
	"fmt"
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
