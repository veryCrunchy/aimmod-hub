package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament"
	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament/bracket"
)

// Uses an explicitly supplied test database; the test creates and drops its own schema.
func TestTournamentPersistence(t *testing.T) {
	url := os.Getenv("AIMMOD_TOURNAMENT_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("AIMMOD_TOURNAMENT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("tournament_test_%d", time.Now().UnixNano())
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	if _, err = pool.Exec(ctx, "SET search_path TO "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	s := &Store{pool: pool}
	if err = s.ensureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	users := make([]int64, 4)
	for i := range users {
		handle := fmt.Sprintf("tournament-player-%d", i)
		if err := pool.QueryRow(ctx, `INSERT INTO hub_users(external_id, profile_handle) VALUES($1, $1) RETURNING id`, handle).Scan(&users[i]); err != nil {
			t.Fatal(err)
		}
	}
	org := tournament.Actor{UserRef: tournament.UserRef{UserID: users[0], Handle: "tournament-player-0"}}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	spec := tournament.Spec{Name: "Synthetic Persist", Format: bracket.SingleElimination, Seeding: tournament.SeedManual,
		Ruleset: tournament.Ruleset{BestOf: 1, Pool: []tournament.PoolScenario{{Name: "Synthetic A"}}}}
	a, err := tournament.New("t_persist_a", spec, org, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTournament(ctx, a); err != nil {
		t.Fatal(err)
	}
	b, _ := tournament.New("t_persist_b", spec, org, now.Add(time.Second))
	if err := s.CreateTournament(ctx, b); err != nil {
		t.Fatal(err)
	}
	if a.Slug != "synthetic-persist" || b.Slug != "synthetic-persist-2" {
		t.Fatalf("slugs %s %s", a.Slug, b.Slug)
	}
	loaded, err := s.LoadTournament(ctx, "synthetic-persist")
	if err != nil || loaded.ID != a.ID || loaded.Version != 1 {
		t.Fatalf("load by slug: %v %+v", err, loaded)
	}
	if err := loaded.Advance(org, tournament.OpenRegistration, nil, 0, now); err != nil {
		t.Fatal(err)
	}
	for _, u := range users[1:] {
		if _, err := loaded.Register(tournament.Actor{UserRef: tournament.UserRef{UserID: u, Handle: fmt.Sprint(u)}}, now); err != nil {
			t.Fatal(err)
		}
	}
	stale, _ := s.LoadTournament(ctx, a.ID)
	if err := s.SaveTournament(ctx, loaded, now); err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 2 {
		t.Fatalf("version %d", loaded.Version)
	}
	if err := s.SaveTournament(ctx, stale, now); !errors.Is(err, ErrTournamentConflict) {
		t.Fatalf("stale save: %v", err)
	}
	// Membership drives "mine"; drafts only appear there.
	mine, _, err := s.ListTournaments(ctx, TournamentFilter{MemberUserID: users[2]})
	if err != nil || len(mine) != 1 || mine[0].ID != a.ID {
		t.Fatalf("mine %v %d", err, len(mine))
	}
	public, _, err := s.ListTournaments(ctx, TournamentFilter{})
	if err != nil || len(public) != 1 {
		t.Fatalf("public list hides drafts: %v %d", err, len(public))
	}
	orgList, _, _ := s.ListTournaments(ctx, TournamentFilter{MemberUserID: users[0]})
	if len(orgList) != 2 {
		t.Fatalf("organiser list %d", len(orgList))
	}
	// Paging.
	page, next, err := s.ListTournaments(ctx, TournamentFilter{MemberUserID: users[0], Limit: 1})
	if err != nil || len(page) != 1 || next == "" || page[0].ID != b.ID {
		t.Fatalf("page 1 %v %q", err, next)
	}
	page, next, err = s.ListTournaments(ctx, TournamentFilter{MemberUserID: users[0], Limit: 1, Cursor: next})
	if err != nil || len(page) != 1 || next != "" || page[0].ID != a.ID {
		t.Fatalf("page 2 %v %q", err, next)
	}
	if _, _, err := s.ListTournaments(ctx, TournamentFilter{Cursor: "junk"}); err == nil {
		t.Fatal("junk cursor accepted")
	}
	if _, err := s.LoadTournament(ctx, "missing"); !errors.Is(err, ErrTournamentNotFound) {
		t.Fatal(err)
	}
	// Personal bests for stat seeding.
	if _, err := pool.Exec(ctx, `INSERT INTO scenario_runs(session_id, user_id, app_version, schema_version, scenario_name, score, played_at)
		VALUES ('s1', $1, 'test', 1, 'Synthetic A', 100, NOW()), ('s2', $1, 'test', 1, 'synthetic a', 150, NOW()), ('s3', $2, 'test', 1, 'Other', 900, NOW())`, users[1], users[2]); err != nil {
		t.Fatal(err)
	}
	best, err := s.ScenarioPersonalBests(ctx, users, "Synthetic A")
	if err != nil || len(best) != 1 || best[users[1]] != 150 {
		t.Fatalf("personal bests %v %v", best, err)
	}
	u, err := s.TournamentUserByHandle(ctx, "TOURNAMENT-PLAYER-3")
	if err != nil || u.UserID != users[3] {
		t.Fatalf("handle lookup %+v %v", u, err)
	}
}
