package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament"
)

// Tournaments are stored as one JSON document per event (the aggregate in
// package tournament) with an optimistic version, plus a membership index
// for "my tournaments" lookups. Events are small (at most 256 entrants and
// about 500 matches), so loading and saving whole keeps every rule in one
// place and every change atomic.
const tournamentSchemaSQL = `
CREATE TABLE IF NOT EXISTS tournaments (
  id TEXT PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  format TEXT NOT NULL,
  organiser_user_id BIGINT NOT NULL REFERENCES hub_users(id) ON DELETE CASCADE,
  starts_at TIMESTAMPTZ NULL,
  entrant_count INTEGER NOT NULL DEFAULT 0,
  version BIGINT NOT NULL,
  doc JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tournaments_status_created ON tournaments(status, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_tournaments_created ON tournaments(created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS tournament_members (
  tournament_id TEXT NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES hub_users(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  PRIMARY KEY (tournament_id, user_id, role)
);
CREATE INDEX IF NOT EXISTS idx_tournament_members_user ON tournament_members(user_id, tournament_id);
`

var (
	ErrTournamentNotFound = errors.New("tournament not found")
	// ErrTournamentConflict: someone saved a newer version first; load and retry.
	ErrTournamentConflict = errors.New("tournament changed concurrently")
)

func (s *Store) ensureTournamentSchema(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, tournamentSchemaSQL); err != nil {
		return fmt.Errorf("ensure tournament schema: %w", err)
	}
	return nil
}

// pgxExecer is satisfied by the pool and by transactions.
type pgxExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// CreateTournament stores a new tournament, picking a free slug.
func (s *Store) CreateTournament(ctx context.Context, t *tournament.Tournament) error {
	base := t.Slug
	for attempt := 0; attempt < 20; attempt++ {
		slug := base
		if attempt > 0 {
			slug = base + "-" + strconv.Itoa(attempt+1)
		}
		t.Slug = slug
		doc, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("encode tournament: %w", err)
		}
		tag, err := s.pool.Exec(ctx, `
			INSERT INTO tournaments (id, slug, name, status, format, organiser_user_id, starts_at, entrant_count, version, doc, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
			ON CONFLICT (slug) DO NOTHING
		`, t.ID, slug, t.Spec.Name, string(t.Status), string(t.Spec.Format), t.Organiser.UserID, t.Spec.StartsAt, len(t.Entrants), t.Version, string(doc), t.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert tournament: %w", err)
		}
		if tag.RowsAffected() == 1 {
			return s.writeTournamentMembers(ctx, s.pool, t)
		}
	}
	return fmt.Errorf("no free slug for %q", base)
}

func (s *Store) writeTournamentMembers(ctx context.Context, q pgxExecer, t *tournament.Tournament) error {
	if _, err := q.Exec(ctx, `DELETE FROM tournament_members WHERE tournament_id = $1`, t.ID); err != nil {
		return fmt.Errorf("clear tournament members: %w", err)
	}
	ids, roles := []int64{t.Organiser.UserID}, []string{"organiser"}
	for _, st := range t.Staff {
		ids, roles = append(ids, st.User.UserID), append(roles, string(st.Role))
	}
	for _, e := range t.Entrants {
		if e.Active() {
			ids, roles = append(ids, e.User.UserID), append(roles, "entrant")
		}
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO tournament_members (tournament_id, user_id, role)
		SELECT $1, u, r FROM UNNEST($2::BIGINT[], $3::TEXT[]) AS m(u, r)
		WHERE EXISTS (SELECT 1 FROM hub_users WHERE id = u)
		ON CONFLICT DO NOTHING
	`, t.ID, ids, roles); err != nil {
		return fmt.Errorf("write tournament members: %w", err)
	}
	return nil
}

// LoadTournament by id or slug.
func (s *Store) LoadTournament(ctx context.Context, idOrSlug string) (*tournament.Tournament, error) {
	var doc []byte
	var version int64
	err := s.pool.QueryRow(ctx, `SELECT doc, version FROM tournaments WHERE id = $1 OR slug = LOWER($1) LIMIT 1`, strings.TrimSpace(idOrSlug)).Scan(&doc, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTournamentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load tournament: %w", err)
	}
	var t tournament.Tournament
	if err := json.Unmarshal(doc, &t); err != nil {
		return nil, fmt.Errorf("decode tournament: %w", err)
	}
	t.Version = version
	return &t, nil
}

// SaveTournament writes t if nobody saved since it was loaded (t.Version)
// and bumps the version.
func (s *Store) SaveTournament(ctx context.Context, t *tournament.Tournament, now time.Time) error {
	expected := t.Version
	t.Version = expected + 1
	t.UpdatedAt = now
	doc, err := json.Marshal(t)
	if err != nil {
		t.Version = expected
		return fmt.Errorf("encode tournament: %w", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Version = expected
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	active := 0
	for _, e := range t.Entrants {
		if e.Active() {
			active++
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE tournaments
		SET name = $3, status = $4, format = $5, starts_at = $6, entrant_count = $7, version = $8, doc = $9, updated_at = $10
		WHERE id = $1 AND version = $2
	`, t.ID, expected, t.Spec.Name, string(t.Status), string(t.Spec.Format), t.Spec.StartsAt, active, t.Version, string(doc), now)
	if err != nil {
		t.Version = expected
		return fmt.Errorf("save tournament: %w", err)
	}
	if tag.RowsAffected() != 1 {
		t.Version = expected
		return ErrTournamentConflict
	}
	if err := s.writeTournamentMembers(ctx, tx, t); err != nil {
		t.Version = expected
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		t.Version = expected
		return fmt.Errorf("commit tournament: %w", err)
	}
	return nil
}

type TournamentFilter struct {
	Statuses []string
	// MemberUserID: only tournaments this user entered or helps run
	// (drafts included); otherwise drafts are never listed.
	MemberUserID int64
	Limit        int
	Cursor string
}

// ListTournaments, newest first. The cursor is "<unix nanos>:<id>" of the last row.
func (s *Store) ListTournaments(ctx context.Context, f TournamentFilter) ([]*tournament.Tournament, string, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 30
	}
	args := []any{}
	where := []string{"TRUE"}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if len(f.Statuses) > 0 {
		where = append(where, "t.status = ANY("+arg(f.Statuses)+"::TEXT[])")
	}
	if f.MemberUserID != 0 {
		where = append(where, "EXISTS (SELECT 1 FROM tournament_members m WHERE m.tournament_id = t.id AND m.user_id = "+arg(f.MemberUserID)+")")
	} else {
		where = append(where, "t.status <> 'draft'")
	}
	if f.Cursor != "" {
		parts := strings.SplitN(f.Cursor, ":", 2)
		nanos, err := strconv.ParseInt(parts[0], 10, 64)
		if len(parts) != 2 || err != nil {
			return nil, "", fmt.Errorf("invalid cursor")
		}
		at := time.Unix(0, nanos).UTC()
		where = append(where, "(t.created_at, t.id) < ("+arg(at)+", "+arg(parts[1])+")")
	}
	query := `SELECT doc, version, created_at FROM tournaments t WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY t.created_at DESC, t.id DESC LIMIT ` + arg(f.Limit+1)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list tournaments: %w", err)
	}
	defer rows.Close()
	var out []*tournament.Tournament
	var created []time.Time
	for rows.Next() {
		var doc []byte
		var version int64
		var at time.Time
		if err := rows.Scan(&doc, &version, &at); err != nil {
			return nil, "", fmt.Errorf("scan tournament: %w", err)
		}
		var t tournament.Tournament
		if err := json.Unmarshal(doc, &t); err != nil {
			return nil, "", fmt.Errorf("decode tournament: %w", err)
		}
		t.Version = version
		out = append(out, &t)
		created = append(created, at)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > f.Limit {
		out = out[:f.Limit]
		last := out[len(out)-1]
		next = strconv.FormatInt(created[f.Limit-1].UnixNano(), 10) + ":" + last.ID
	}
	return out, next, nil
}

// ScenarioPersonalBests: each user's best uploaded score on a scenario.
func (s *Store) ScenarioPersonalBests(ctx context.Context, userIDs []int64, scenario string) (map[int64]float64, error) {
	out := map[int64]float64{}
	if len(userIDs) == 0 || strings.TrimSpace(scenario) == "" {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT user_id, MAX(score)
		FROM scenario_runs
		WHERE user_id = ANY($1::BIGINT[]) AND LOWER(scenario_name) = LOWER($2)
		GROUP BY user_id
	`, userIDs, strings.TrimSpace(scenario))
	if err != nil {
		return nil, fmt.Errorf("scenario personal bests: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var best float64
		if err := rows.Scan(&id, &best); err != nil {
			return nil, err
		}
		out[id] = best
	}
	return out, rows.Err()
}

// TournamentUser is the public identity of a Hub user, for staff and invites.
type TournamentUser struct {
	UserID      int64
	ExternalID  string
	Handle      string
	DisplayName string
	AvatarURL   string
	SteamID     string
}

func (s *Store) TournamentUserByHandle(ctx context.Context, handle string) (TournamentUser, error) {
	var u TournamentUser
	err := s.pool.QueryRow(ctx, `
		SELECT hui.user_id, hu.external_id, hui.user_handle, hui.user_display_name, COALESCE(hui.avatar_url, ''),
			COALESCE(steam.provider_account_id, '')
		FROM hub_user_identity hui
		JOIN hub_users hu ON hu.id = hui.user_id
		LEFT JOIN linked_accounts steam ON steam.user_id = hui.user_id AND steam.provider = 'steam'
		WHERE LOWER(hui.user_handle) = LOWER($1)
		LIMIT 1
	`, strings.TrimSpace(handle)).Scan(&u.UserID, &u.ExternalID, &u.Handle, &u.DisplayName, &u.AvatarURL, &u.SteamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrTournamentNotFound
	}
	return u, err
}
