package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type AuthUser struct {
	UserID         int64  `json:"userId"`
	UserExternalID string `json:"userExternalId"`
	DiscordUserID  string `json:"discordUserId"`
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	AvatarURL      string `json:"avatarUrl"`
	IsAdmin        bool   `json:"isAdmin,omitempty"`
}

type UploadTokenRecord struct {
	ID         int64      `json:"id"`
	Label      string     `json:"label"`
	LastFour   string     `json:"lastFour"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

type DeviceLinkRequest struct {
	DeviceCode string
	UserCode   string
	Label      string
	ExpiresAt  time.Time
}

type DeviceLinkPollResult struct {
	Status      string
	ExpiresAt   time.Time
	User        *AuthUser
	UploadToken string
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomUserCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	var b strings.Builder
	for idx, value := range buf {
		if idx == 4 {
			b.WriteByte('-')
		}
		b.WriteByte(alphabet[int(value)%len(alphabet)])
	}
	return b.String(), nil
}

func (s *Store) CreateAuthState(ctx context.Context, returnTo string) (string, error) {
	state, err := randomHex(24)
	if err != nil {
		return "", fmt.Errorf("generate auth state: %w", err)
	}
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO auth_states (state, return_to, expires_at)
		VALUES ($1, $2, $3)
	`, state, returnTo, expiresAt); err != nil {
		return "", fmt.Errorf("insert auth state: %w", err)
	}
	return state, nil
}

func (s *Store) ConsumeAuthState(ctx context.Context, state string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var returnTo string
	var expiresAt time.Time
	if err := tx.QueryRow(ctx, `
		SELECT return_to, expires_at
		FROM auth_states
		WHERE state = $1
	`, state).Scan(&returnTo, &expiresAt); err != nil {
		return "", fmt.Errorf("load auth state: %w", err)
	}
	if expiresAt.Before(time.Now().UTC()) {
		return "", fmt.Errorf("auth state expired")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM auth_states WHERE state = $1`, state); err != nil {
		return "", fmt.Errorf("delete auth state: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit auth state: %w", err)
	}
	return returnTo, nil
}

func (s *Store) UpsertDiscordUser(ctx context.Context, discordID, username, displayName, avatarURL string) (AuthUser, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AuthUser{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var user AuthUser
	err = tx.QueryRow(ctx, `
		SELECT hu.id, hu.external_id, la.provider_account_id, la.username, la.display_name, la.avatar_url
		FROM linked_accounts la
		JOIN hub_users hu ON hu.id = la.user_id
		WHERE la.provider = 'discord' AND la.provider_account_id = $1
	`, discordID).Scan(&user.UserID, &user.UserExternalID, &user.DiscordUserID, &user.Username, &user.DisplayName, &user.AvatarURL)
	if err != nil {
		userExternalID := "discord:" + discordID
		if err := tx.QueryRow(ctx, `
			INSERT INTO hub_users (external_id)
			VALUES ($1)
			RETURNING id, external_id
		`, userExternalID).Scan(&user.UserID, &user.UserExternalID); err != nil {
			return AuthUser{}, fmt.Errorf("create hub user: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE hub_users SET updated_at = NOW() WHERE id = $1`, user.UserID); err != nil {
			return AuthUser{}, fmt.Errorf("touch hub user: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO linked_accounts (
			user_id,
			provider,
			provider_account_id,
			username,
			display_name,
			avatar_url
		)
		VALUES ($1, 'discord', $2, $3, $4, $5)
		ON CONFLICT (provider, provider_account_id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = NOW()
	`, user.UserID, discordID, username, displayName, avatarURL); err != nil {
		return AuthUser{}, fmt.Errorf("upsert discord account: %w", err)
	}

	user.DiscordUserID = discordID
	user.Username = username
	user.DisplayName = displayName
	user.AvatarURL = avatarURL

	if err := tx.Commit(ctx); err != nil {
		return AuthUser{}, fmt.Errorf("commit discord user: %w", err)
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64) (string, error) {
	sessionID, err := randomHex(32)
	if err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO auth_sessions (session_id, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, sessionID, userID, expiresAt); err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}
	return sessionID, nil
}

func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE session_id = $1`, sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) GetUserBySession(ctx context.Context, sessionID string) (AuthUser, error) {
	var user AuthUser
	var expiresAt time.Time
	if err := s.pool.QueryRow(ctx, `
		SELECT
			hu.id,
			hu.external_id,
			la.provider_account_id,
			la.username,
			la.display_name,
			la.avatar_url,
			s.expires_at
		FROM auth_sessions s
		JOIN hub_users hu ON hu.id = s.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE s.session_id = $1
	`, sessionID).Scan(&user.UserID, &user.UserExternalID, &user.DiscordUserID, &user.Username, &user.DisplayName, &user.AvatarURL, &expiresAt); err != nil {
		return AuthUser{}, fmt.Errorf("load session user: %w", err)
	}
	if expiresAt.Before(time.Now().UTC()) {
		_ = s.DeleteSession(ctx, sessionID)
		return AuthUser{}, fmt.Errorf("session expired")
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET last_seen_at = NOW()
		WHERE session_id = $1
	`, sessionID); err != nil {
		return AuthUser{}, fmt.Errorf("touch session: %w", err)
	}
	return user, nil
}

func (s *Store) CreateUploadToken(ctx context.Context, userID int64, label string) (UploadTokenRecord, string, error) {
	return insertUploadTokenTx(ctx, s.pool, userID, label)
}

func insertUploadTokenTx(ctx context.Context, tx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID int64, label string) (UploadTokenRecord, string, error) {
	rawSecret, err := randomHex(24)
	if err != nil {
		return UploadTokenRecord{}, "", fmt.Errorf("generate token: %w", err)
	}
	rawToken := "amh_" + rawSecret
	lastFour := rawToken
	if len(lastFour) > 4 {
		lastFour = lastFour[len(lastFour)-4:]
	}

	var record UploadTokenRecord
	if err := tx.QueryRow(ctx, `
		INSERT INTO upload_tokens (user_id, label, token_hash, last_four)
		VALUES ($1, $2, $3, $4)
		RETURNING id, label, last_four, created_at, last_used_at
	`, userID, label, tokenHash(rawToken), lastFour).Scan(&record.ID, &record.Label, &record.LastFour, &record.CreatedAt, &record.LastUsedAt); err != nil {
		return UploadTokenRecord{}, "", fmt.Errorf("insert upload token: %w", err)
	}
	return record, rawToken, nil
}

func (s *Store) ListUploadTokens(ctx context.Context, userID int64) ([]UploadTokenRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, label, last_four, created_at, last_used_at
		FROM upload_tokens
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list upload tokens: %w", err)
	}
	defer rows.Close()

	var records []UploadTokenRecord
	for rows.Next() {
		var record UploadTokenRecord
		if err := rows.Scan(&record.ID, &record.Label, &record.LastFour, &record.CreatedAt, &record.LastUsedAt); err != nil {
			return nil, fmt.Errorf("scan upload token: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate upload tokens: %w", err)
	}
	return records, nil
}

func (s *Store) RevokeUploadToken(ctx context.Context, userID, tokenID int64) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE upload_tokens
		SET revoked_at = NOW()
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
	`, tokenID, userID)
	if err != nil {
		return fmt.Errorf("revoke upload token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("upload token not found")
	}
	return nil
}

func (s *Store) GetUserByUploadToken(ctx context.Context, rawToken string) (AuthUser, error) {
	token := strings.TrimSpace(strings.TrimPrefix(rawToken, "Bearer "))
	if token == "" {
		return AuthUser{}, fmt.Errorf("missing token")
	}

	var user AuthUser
	if err := s.pool.QueryRow(ctx, `
		SELECT
			hu.id,
			hu.external_id,
			la.provider_account_id,
			la.username,
			la.display_name,
			la.avatar_url
		FROM upload_tokens t
		JOIN hub_users hu ON hu.id = t.user_id
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE t.token_hash = $1 AND t.revoked_at IS NULL
	`, tokenHash(token)).Scan(&user.UserID, &user.UserExternalID, &user.DiscordUserID, &user.Username, &user.DisplayName, &user.AvatarURL); err != nil {
		return AuthUser{}, fmt.Errorf("load upload token user: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE upload_tokens SET last_used_at = NOW() WHERE token_hash = $1`, tokenHash(token)); err != nil {
		return AuthUser{}, fmt.Errorf("touch upload token: %w", err)
	}
	return user, nil
}

func (s *Store) CreateDeviceLinkRequest(ctx context.Context, label string) (DeviceLinkRequest, error) {
	deviceCode, err := randomHex(32)
	if err != nil {
		return DeviceLinkRequest{}, fmt.Errorf("generate device code: %w", err)
	}
	if strings.TrimSpace(label) == "" {
		label = "AimMod desktop"
	}

	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	for attempts := 0; attempts < 12; attempts++ {
		userCode, err := randomUserCode()
		if err != nil {
			return DeviceLinkRequest{}, fmt.Errorf("generate user code: %w", err)
		}
		tag, err := s.pool.Exec(ctx, `
			INSERT INTO device_link_requests (device_code, user_code, label, status, expires_at)
			VALUES ($1, $2, $3, 'pending', $4)
			ON CONFLICT (user_code) DO NOTHING
		`, deviceCode, userCode, label, expiresAt)
		if err != nil {
			return DeviceLinkRequest{}, fmt.Errorf("insert device link request: %w", err)
		}
		if tag.RowsAffected() == 1 {
			return DeviceLinkRequest{
				DeviceCode: deviceCode,
				UserCode:   userCode,
				Label:      label,
				ExpiresAt:  expiresAt,
			}, nil
		}
	}

	return DeviceLinkRequest{}, fmt.Errorf("could not allocate unique device link code")
}

func (s *Store) ApproveDeviceLink(ctx context.Context, userID int64, userCode string) (DeviceLinkPollResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeviceLinkPollResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	normalizedCode := strings.ToUpper(strings.TrimSpace(userCode))
	var result DeviceLinkPollResult
	var label string
	if err := tx.QueryRow(ctx, `
		SELECT status, expires_at, label
		FROM device_link_requests
		WHERE user_code = $1
		FOR UPDATE
	`, normalizedCode).Scan(&result.Status, &result.ExpiresAt, &label); err != nil {
		return DeviceLinkPollResult{}, fmt.Errorf("load device link request: %w", err)
	}

	if result.ExpiresAt.Before(time.Now().UTC()) {
		if _, err := tx.Exec(ctx, `
			UPDATE device_link_requests
			SET status = 'expired'
			WHERE user_code = $1
		`, normalizedCode); err != nil {
			return DeviceLinkPollResult{}, fmt.Errorf("expire device link request: %w", err)
		}
		result.Status = "expired"
		return result, nil
	}
	if result.Status == "approved" {
		return result, nil
	}
	if result.Status != "pending" {
		return DeviceLinkPollResult{}, fmt.Errorf("device link request is %s", result.Status)
	}

	record, rawToken, err := insertUploadTokenTx(ctx, tx, userID, label)
	if err != nil {
		return DeviceLinkPollResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE device_link_requests
		SET status = 'approved',
		    user_id = $2,
		    upload_token_id = $3,
		    upload_token_plaintext = $4,
		    approved_at = NOW()
		WHERE user_code = $1
	`, normalizedCode, userID, record.ID, rawToken); err != nil {
		return DeviceLinkPollResult{}, fmt.Errorf("approve device link request: %w", err)
	}

	var user AuthUser
	if err := tx.QueryRow(ctx, `
		SELECT
			hu.id,
			hu.external_id,
			COALESCE(la.provider_account_id, ''),
			COALESCE(la.username, ''),
			COALESCE(la.display_name, ''),
			COALESCE(la.avatar_url, '')
		FROM hub_users hu
		LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
		WHERE hu.id = $1
	`, userID).Scan(&user.UserID, &user.UserExternalID, &user.DiscordUserID, &user.Username, &user.DisplayName, &user.AvatarURL); err != nil {
		return DeviceLinkPollResult{}, fmt.Errorf("load approved device link user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return DeviceLinkPollResult{}, fmt.Errorf("commit device link approval: %w", err)
	}

	result.Status = "approved"
	result.User = &user
	result.UploadToken = rawToken
	return result, nil
}

func (s *Store) PollDeviceLink(ctx context.Context, deviceCode string) (DeviceLinkPollResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeviceLinkPollResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	normalizedCode := strings.TrimSpace(deviceCode)
	var result DeviceLinkPollResult
	var userID *int64
	if err := tx.QueryRow(ctx, `
		SELECT status, expires_at, user_id, COALESCE(upload_token_plaintext, '')
		FROM device_link_requests
		WHERE device_code = $1
		FOR UPDATE
	`, normalizedCode).Scan(&result.Status, &result.ExpiresAt, &userID, &result.UploadToken); err != nil {
		return DeviceLinkPollResult{}, fmt.Errorf("load device link request: %w", err)
	}

	if result.ExpiresAt.Before(time.Now().UTC()) && result.Status == "pending" {
		result.Status = "expired"
		if _, err := tx.Exec(ctx, `
			UPDATE device_link_requests
			SET status = 'expired'
			WHERE device_code = $1
		`, normalizedCode); err != nil {
			return DeviceLinkPollResult{}, fmt.Errorf("expire device link request: %w", err)
		}
	}

	if result.Status == "approved" && userID != nil {
		var user AuthUser
		if err := tx.QueryRow(ctx, `
			SELECT
				hu.id,
				hu.external_id,
				COALESCE(la.provider_account_id, ''),
				COALESCE(la.username, ''),
				COALESCE(la.display_name, ''),
				COALESCE(la.avatar_url, '')
			FROM hub_users hu
			LEFT JOIN linked_accounts la ON la.user_id = hu.id AND la.provider = 'discord'
			WHERE hu.id = $1
		`, *userID).Scan(&user.UserID, &user.UserExternalID, &user.DiscordUserID, &user.Username, &user.DisplayName, &user.AvatarURL); err != nil {
			return DeviceLinkPollResult{}, fmt.Errorf("load device link user: %w", err)
		}
		result.User = &user
	}

	if _, err := tx.Exec(ctx, `
		UPDATE device_link_requests
		SET last_polled_at = NOW()
		WHERE device_code = $1
	`, normalizedCode); err != nil {
		return DeviceLinkPollResult{}, fmt.Errorf("touch device link request: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return DeviceLinkPollResult{}, fmt.Errorf("commit device link poll: %w", err)
	}
	return result, nil
}
