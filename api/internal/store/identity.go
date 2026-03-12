package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type LinkedAccountIdentity struct {
	Provider          string
	ProviderAccountID string
	Username          string
	DisplayName       string
	AvatarURL         string
}

type matchedIdentityOwner struct {
	UserID   int64
	Verified bool
}

func normalizeLinkedIdentity(provider string, providerAccountID string, username string) (string, string) {
	normalizedProvider := strings.TrimSpace(strings.ToLower(provider))
	normalizedAccountID := strings.TrimSpace(providerAccountID)
	if normalizedAccountID == "" && normalizedProvider == "kovaaks" {
		normalizedAccountID = strings.TrimSpace(strings.ToLower(username))
	}
	return normalizedProvider, normalizedAccountID
}

func linkedIdentitiesFromRun(run IngestedRun) []LinkedAccountIdentity {
	identities := make([]LinkedAccountIdentity, 0, 2)

	if provider, accountID := normalizeLinkedIdentity("kovaaks", run.KovaaksUserID, run.KovaaksUsername); accountID != "" {
		identities = append(identities, LinkedAccountIdentity{
			Provider:          provider,
			ProviderAccountID: accountID,
			Username:          strings.TrimSpace(run.KovaaksUsername),
			DisplayName:       strings.TrimSpace(run.UserDisplayName),
			AvatarURL:         strings.TrimSpace(run.AvatarURL),
		})
	}

	if provider, accountID := normalizeLinkedIdentity("steam", run.SteamID, run.SteamDisplayName); accountID != "" {
		identities = append(identities, LinkedAccountIdentity{
			Provider:          provider,
			ProviderAccountID: accountID,
			Username:          strings.TrimSpace(run.SteamDisplayName),
			DisplayName:       strings.TrimSpace(run.SteamDisplayName),
			AvatarURL:         strings.TrimSpace(run.AvatarURL),
		})
	}

	deduped := make([]LinkedAccountIdentity, 0, len(identities))
	seen := make(map[string]struct{}, len(identities))
	for _, identity := range identities {
		key := identity.Provider + ":" + identity.ProviderAccountID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, identity)
	}
	return deduped
}

func preferredExternalID(run IngestedRun) string {
	if value := strings.TrimSpace(run.UserExternalID); value != "" {
		return value
	}
	if value := strings.TrimSpace(run.KovaaksUserID); value != "" {
		return "kovaaks:" + value
	}
	if value := strings.TrimSpace(run.SteamID); value != "" {
		return "steam:" + value
	}
	if value := strings.TrimSpace(strings.ToLower(run.KovaaksUsername)); value != "" {
		return "kovaaks:username:" + value
	}
	return ""
}

func ensureHubUserTx(ctx context.Context, tx pgx.Tx, externalID string) (int64, error) {
	userID, _, err := insertHubUserTx(ctx, tx, externalID)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func loadUserExternalIDTx(ctx context.Context, tx pgx.Tx, userID int64) (string, error) {
	var externalID string
	if err := tx.QueryRow(ctx, `
		SELECT external_id
		FROM hub_users
		WHERE id = $1
	`, userID).Scan(&externalID); err != nil {
		return "", fmt.Errorf("load hub user external id: %w", err)
	}
	return externalID, nil
}

func loadIdentityOwnerTx(
	ctx context.Context,
	tx pgx.Tx,
	identity LinkedAccountIdentity,
) (matchedIdentityOwner, bool, error) {
	var owner matchedIdentityOwner
	err := tx.QueryRow(ctx, `
		SELECT user_id, verified
		FROM linked_accounts
		WHERE provider = $1 AND provider_account_id = $2
		FOR UPDATE
	`, identity.Provider, identity.ProviderAccountID).Scan(&owner.UserID, &owner.Verified)
	if err == pgx.ErrNoRows {
		return matchedIdentityOwner{}, false, nil
	}
	if err != nil {
		return matchedIdentityOwner{}, false, fmt.Errorf("load linked account owner: %w", err)
	}
	return owner, true, nil
}

func mergeUsersTx(ctx context.Context, tx pgx.Tx, targetUserID, sourceUserID int64) error {
	if targetUserID == 0 || sourceUserID == 0 || targetUserID == sourceUserID {
		return nil
	}

	rows, err := tx.Query(ctx, `
		SELECT id, provider
		FROM linked_accounts
		WHERE user_id = $1
		FOR UPDATE
	`, sourceUserID)
	if err != nil {
		return fmt.Errorf("load source linked accounts: %w", err)
	}
	defer rows.Close()

	type linkedAccountRow struct {
		ID       int64
		Provider string
	}
	var accounts []linkedAccountRow
	for rows.Next() {
		var row linkedAccountRow
		if err := rows.Scan(&row.ID, &row.Provider); err != nil {
			return fmt.Errorf("scan source linked account: %w", err)
		}
		accounts = append(accounts, row)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate source linked accounts: %w", err)
	}

	for _, account := range accounts {
		var targetAccountID int64
		err := tx.QueryRow(ctx, `
			SELECT id
			FROM linked_accounts
			WHERE user_id = $1 AND provider = $2
			FOR UPDATE
		`, targetUserID, account.Provider).Scan(&targetAccountID)
		if err == pgx.ErrNoRows {
			if _, err := tx.Exec(ctx, `
				UPDATE linked_accounts
				SET user_id = $1, updated_at = NOW()
				WHERE id = $2
			`, targetUserID, account.ID); err != nil {
				return fmt.Errorf("transfer linked account: %w", err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("load target linked account: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE linked_accounts AS target
			SET username = CASE
					WHEN NULLIF(target.username, '') IS NULL THEN source.username
					ELSE target.username
				END,
				display_name = CASE
					WHEN NULLIF(target.display_name, '') IS NULL THEN source.display_name
					ELSE target.display_name
				END,
				avatar_url = CASE
					WHEN NULLIF(target.avatar_url, '') IS NULL THEN source.avatar_url
					ELSE target.avatar_url
				END,
				verified = target.verified OR source.verified,
				updated_at = NOW()
			FROM linked_accounts AS source
			WHERE target.id = $1 AND source.id = $2
		`, targetAccountID, account.ID); err != nil {
			return fmt.Errorf("merge linked account: %w", err)
		}

		if _, err := tx.Exec(ctx, `DELETE FROM linked_accounts WHERE id = $1`, account.ID); err != nil {
			return fmt.Errorf("delete merged linked account: %w", err)
		}
	}

	for _, query := range []string{
		`DELETE FROM scenario_runs AS src
		  USING scenario_runs AS dst
		  WHERE src.user_id = $2
		    AND dst.user_id = $1
		    AND (
		      (src.source_session_id IS NOT NULL AND dst.source_session_id = src.source_session_id)
		      OR dst.session_id = src.session_id
		    )`,
		`UPDATE scenario_runs SET user_id = $1 WHERE user_id = $2`,
		`UPDATE upload_tokens SET user_id = $1 WHERE user_id = $2`,
		`UPDATE auth_sessions SET user_id = $1 WHERE user_id = $2`,
		`UPDATE device_link_requests SET user_id = $1 WHERE user_id = $2`,
	} {
		if _, err := tx.Exec(ctx, query, targetUserID, sourceUserID); err != nil {
			return fmt.Errorf("transfer user records: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM hub_users WHERE id = $1`, sourceUserID); err != nil {
		return fmt.Errorf("delete merged source user: %w", err)
	}

	return nil
}

func upsertLinkedIdentityTx(
	ctx context.Context,
	tx pgx.Tx,
	targetUserID int64,
	identity LinkedAccountIdentity,
	verified bool,
) error {
	if identity.Provider == "" || identity.ProviderAccountID == "" {
		return nil
	}

	owner, found, err := loadIdentityOwnerTx(ctx, tx, identity)
	if err != nil {
		return err
	}
	if found && owner.UserID != targetUserID {
		if owner.Verified {
			return fmt.Errorf("%s account is already verified on another profile", identity.Provider)
		}
		if err := mergeUsersTx(ctx, tx, targetUserID, owner.UserID); err != nil {
			return err
		}
	}

	var existingTargetRowID int64
	var existingTargetAccountID string
	var existingTargetVerified bool
	err = tx.QueryRow(ctx, `
		SELECT id, provider_account_id, verified
		FROM linked_accounts
		WHERE user_id = $1 AND provider = $2
		FOR UPDATE
	`, targetUserID, identity.Provider).Scan(&existingTargetRowID, &existingTargetAccountID, &existingTargetVerified)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("load target provider link: %w", err)
	}
	if err == nil && existingTargetAccountID != identity.ProviderAccountID {
		if existingTargetVerified && verified {
			return fmt.Errorf("%s provider is already linked to a different verified account", identity.Provider)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE linked_accounts
			SET provider_account_id = $2,
				username = $3,
				display_name = $4,
				avatar_url = $5,
				verified = verified OR $6,
				updated_at = NOW()
			WHERE id = $1
		`, existingTargetRowID, identity.ProviderAccountID, identity.Username, identity.DisplayName, identity.AvatarURL, verified); err != nil {
			return fmt.Errorf("replace target provider link: %w", err)
		}
		return nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO linked_accounts (
			user_id,
			provider,
			provider_account_id,
			username,
			display_name,
			avatar_url,
			verified
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (provider, provider_account_id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			verified = linked_accounts.verified OR EXCLUDED.verified,
			updated_at = NOW()
	`, targetUserID, identity.Provider, identity.ProviderAccountID, identity.Username, identity.DisplayName, identity.AvatarURL, verified); err != nil {
		return fmt.Errorf("upsert linked account identity: %w", err)
	}

	return nil
}

func resolveIngestUserTx(
	ctx context.Context,
	tx pgx.Tx,
	run IngestedRun,
	authUser *AuthUser,
) (int64, string, error) {
	identities := linkedIdentitiesFromRun(run)

	if authUser != nil {
		for _, identity := range identities {
			if err := upsertLinkedIdentityTx(ctx, tx, authUser.UserID, identity, true); err != nil {
				return 0, "", err
			}
		}
		externalID := strings.TrimSpace(authUser.UserExternalID)
		if externalID == "" {
			var err error
			externalID = preferredExternalID(run)
			if externalID == "" {
				externalID = fmt.Sprintf("user:%d", authUser.UserID)
			}
			if _, err = tx.Exec(ctx, `
				UPDATE hub_users
				SET external_id = $2, updated_at = NOW()
				WHERE id = $1
			`, authUser.UserID, externalID); err != nil {
				return 0, "", fmt.Errorf("update authenticated user external id: %w", err)
			}
		}
		if _, err := ensureProfileHandleTx(ctx, tx, authUser.UserID, externalID); err != nil {
			return 0, "", err
		}
		return authUser.UserID, externalID, nil
	}

	if len(identities) == 0 {
		externalID := preferredExternalID(run)
		if externalID == "" {
			return 0, "", fmt.Errorf("user identity is required")
		}
		userID, err := ensureHubUserTx(ctx, tx, externalID)
		if err != nil {
			return 0, "", err
		}
		return userID, externalID, nil
	}

	seenUsers := make(map[int64]bool)
	var matched []matchedIdentityOwner
	for _, identity := range identities {
		owner, found, err := loadIdentityOwnerTx(ctx, tx, identity)
		if err != nil {
			return 0, "", err
		}
		if !found {
			continue
		}
		if _, ok := seenUsers[owner.UserID]; ok {
			continue
		}
		seenUsers[owner.UserID] = owner.Verified
		matched = append(matched, owner)
	}

	var finalUserID int64
	for _, owner := range matched {
		if owner.Verified {
			return 0, "", fmt.Errorf("identity belongs to a verified profile; link your AimMod account to upload")
		}
	}
	if finalUserID == 0 && len(matched) > 0 {
		finalUserID = matched[0].UserID
	}
	if finalUserID == 0 {
		externalID := preferredExternalID(run)
		if externalID == "" {
			return 0, "", fmt.Errorf("user identity is required")
		}
		userID, err := ensureHubUserTx(ctx, tx, externalID)
		if err != nil {
			return 0, "", err
		}
		finalUserID = userID
	}

	for _, owner := range matched {
		if owner.UserID == finalUserID {
			continue
		}
		if err := mergeUsersTx(ctx, tx, finalUserID, owner.UserID); err != nil {
			return 0, "", err
		}
	}

	for _, identity := range identities {
		if err := upsertLinkedIdentityTx(ctx, tx, finalUserID, identity, false); err != nil {
			return 0, "", err
		}
	}

	externalID, err := loadUserExternalIDTx(ctx, tx, finalUserID)
	if err != nil {
		return 0, "", err
	}
	if _, err := ensureProfileHandleTx(ctx, tx, finalUserID, externalID); err != nil {
		return 0, "", err
	}
	return finalUserID, externalID, nil
}
