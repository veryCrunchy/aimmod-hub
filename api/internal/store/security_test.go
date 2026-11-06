package store

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestIdentityAndRetryDatabase(t *testing.T) {
	dsn := os.Getenv("AIMMOD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AIMMOD_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("security_test_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	isolated, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer isolated.Close()
	s := &Store{pool: isolated}
	if err = s.ensureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = isolated.Exec(ctx, `INSERT INTO hub_users(id,external_id,profile_handle) VALUES(1,'synthetic-one','synthetic-one'),(2,'synthetic-two','synthetic-two')`); err != nil {
		t.Fatal(err)
	}
	tx, err := isolated.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = resolveIngestUserTx(ctx, tx, IngestedRun{SteamID: "00000000000000001"}, &AuthUser{UserID: 1, UserExternalID: "synthetic-one"})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var verified bool
	if err = isolated.QueryRow(ctx, `SELECT verified FROM linked_accounts WHERE user_id=1 AND provider='steam'`).Scan(&verified); err != nil || verified {
		t.Fatalf("forged identity verified: %v", err)
	}
	if err = s.VerifySteamIdentity(ctx, 2, "00000000000000001"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = isolated.QueryRow(ctx, `SELECT count(*) FROM hub_users`).Scan(&count); err != nil || count != 2 {
		t.Fatal("accounts merged", err)
	}
	if err = s.ensureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err = isolated.QueryRow(ctx, `SELECT verified FROM linked_accounts WHERE user_id=2 AND provider='steam'`).Scan(&verified); err != nil || !verified {
		t.Fatal("proof lost on migration", err)
	}
	if err = s.UpsertKovaaksBenchmarkCatalog(ctx, []KovaaksBenchmarkMeta{{BenchmarkID: 1}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 15; i++ {
		if err = s.MarkKovaaksBenchmarkSyncFailed(ctx, 1, "temporary failure"); err != nil {
			t.Fatal(err)
		}
	}
	var success *time.Time
	var delay float64
	if err = isolated.QueryRow(ctx, `SELECT definition_synced_at, EXTRACT(EPOCH FROM next_retry-last_attempt) FROM kovaaks_benchmarks WHERE benchmark_id=1`).Scan(&success, &delay); err != nil || success != nil || delay > 21600 || delay < 60 {
		t.Fatal("invalid retry state", err, delay)
	}
	ids, err := s.ListKovaaksBenchmarksNeedingSync(ctx, time.Hour, 20)
	if err != nil || len(ids) != 0 {
		t.Fatal("retry selected too early", err)
	}
	if _, err = isolated.Exec(ctx, `UPDATE kovaaks_benchmarks SET next_retry=NOW()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	ids, err = s.ListKovaaksBenchmarksNeedingSync(ctx, time.Hour, 20)
	if err != nil || len(ids) != 1 {
		t.Fatal("retry not selected", err)
	}
	if err = s.SaveKovaaksBenchmarkDefinition(ctx, 1, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	ids, err = s.ListKovaaksBenchmarksNeedingSync(ctx, time.Hour, 20)
	if err != nil || len(ids) != 0 {
		t.Fatal("successful definition retried", err)
	}
}
