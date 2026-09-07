package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

// Use an explicitly supplied test database. Every test creates and removes its own schema.
func TestTrainingDatabasePersistenceAndPrivacy(t *testing.T) {
	url := os.Getenv("AIMMOD_TRAINING_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("AIMMOD_TRAINING_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("training_test_%d", time.Now().UnixNano())
	cfg.MaxConns = 1
	// Also supports development databases that multiplex clients over one PostgreSQL session.
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
	var first, second int64
	if err = pool.QueryRow(ctx, `INSERT INTO hub_users(external_id,profile_handle) VALUES('training-player-a','training-player-a') RETURNING id`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO hub_users(external_id,profile_handle) VALUES('training-player-b','training-player-b') RETURNING id`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	session := trainingFixture()
	if err = s.SaveTraining(ctx, first, []TrainingSession{session}); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveTraining(ctx, first, []TrainingSession{session}); err != nil {
		t.Fatal("retry failed", err)
	}
	own, _, err := s.ListTraining(ctx, "", first, 30, "")
	if err != nil || len(own) != 1 {
		t.Fatal("retry duplicated or lost session", len(own), err)
	}
	public, _, err := s.ListTraining(ctx, "training-player-a", 0, 30, "")
	if err != nil || len(public) != 0 {
		t.Fatal("private session leaked", err)
	}
	found, err := s.SetTrainingVisibility(ctx, second, session.ID, "public")
	if err != nil || found {
		t.Fatal("another owner changed visibility")
	}
	found, err = s.SetTrainingVisibility(ctx, first, session.ID, "public")
	if err != nil || !found {
		t.Fatal("share failed", err)
	}
	public, _, err = s.ListTraining(ctx, "training-player-a", 0, 30, "steady")
	if err != nil || len(public) != 1 || public[0].Visibility != "public" {
		t.Fatal("public query failed", err)
	}
	other, _, err := s.ListTraining(ctx, "", second, 30, "")
	if err != nil || len(other) != 0 {
		t.Fatal("owner query crossed accounts", err)
	}
	bad := session
	bad.Hits--
	extra := session
	extra.ID = "00000000-0000-4000-8000-000000000002"
	if err = s.SaveTraining(ctx, first, []TrainingSession{extra, bad}); !errors.Is(err, ErrTrainingConflict) {
		t.Fatal("missing immutable conflict", err)
	}
	own, _, _ = s.ListTraining(ctx, "", first, 30, "")
	if len(own) != 1 {
		t.Fatal("batch failed to roll back")
	}
	if _, err = s.SetTrainingVisibility(ctx, first, session.ID, "private"); err != nil {
		t.Fatal(err)
	}
	session.Visibility = "public"
	if err = s.SaveTraining(ctx, first, []TrainingSession{session}); err != nil {
		t.Fatal(err)
	}
	public, _, _ = s.ListTraining(ctx, "training-player-a", 0, 30, "")
	if len(public) != 0 {
		t.Fatal("retry republished hidden session")
	}
}
