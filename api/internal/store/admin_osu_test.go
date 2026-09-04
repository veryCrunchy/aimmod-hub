package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAdminOsuDatabase(t *testing.T) {
	dsn := os.Getenv("AIMMOD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AIMMOD_TEST_DATABASE_URL is required for PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("admin_osu_test_%d", time.Now().UnixNano())
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
	for _, sql := range []string{schemaSQL, hubUserIdentityViewSQL, `
INSERT INTO hub_users (id, external_id, profile_handle) VALUES (1, 'test-user', 'player');
INSERT INTO osu_profiles (user_id, osu_user_id, username) VALUES (1, 12, 'Player');
INSERT INTO linked_accounts (user_id, provider, provider_account_id) VALUES (1, 'discord', 'SECRET');
INSERT INTO upload_tokens (user_id, token_hash, last_four) VALUES (1, 'SECRET', 'CRET');
INSERT INTO device_link_requests (device_code, user_code, expires_at) VALUES ('SECRET1', 'CODE1', NOW() + interval '1 hour'), ('SECRET2', 'CODE2', NOW() - interval '1 hour');
INSERT INTO osu_beatmap_sets (set_key, title) VALUES ('set', 'Map 100%');
INSERT INTO osu_beatmap_difficulties (difficulty_key, set_key, version) VALUES ('diff', 'set', 'Hard');
INSERT INTO osu_scores (id, user_id, client_score_id, content_hash, difficulty_key, played_at, visibility, share_id)
SELECT i, 1, i::text, i::text, 'diff', NOW(), CASE WHEN i <= 10 THEN 'public' WHEN i <= 20 THEN 'unlisted' ELSE 'private' END, 'SECRET-share-' || i FROM generate_series(1, 28) i;
INSERT INTO osu_replay_files (score_id, replay_sha256, storage_key, byte_size, uploaded_at)
SELECT i, 'SECRET-hash', CASE WHEN i <= 10 THEN 'SECRET-path-' || i ELSE '' END, CASE WHEN i <= 10 THEN 1024 ELSE 0 END, CASE WHEN i <= 10 THEN NOW() ELSE NULL END FROM generate_series(1,20) i;
`} {
		if _, err = isolated.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	s := &Store{pool: isolated}
	result, err := s.GetAdminOsuOverview(ctx, AdminOsuFilter{Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 28 || len(result.Items) != 25 || result.Items[0].ID != 28 {
		t.Fatalf("incorrect pagination: %+v", result)
	}
	want := AdminOsuSummary{Scores: 28, Public: 10, Unlisted: 10, Private: 8, Uploaded: 10, Pending: 10, ReplayBytes: 10240, Profiles: 1, PublicProfiles: 1, ActiveCredentials: 1, ConnectedAccounts: 1, PendingDevices: 1}
	if result.Summary != want {
		t.Fatalf("summary: %+v, want %+v", result.Summary, want)
	}
	payload, _ := json.Marshal(result)
	if strings.Contains(string(payload), "SECRET") {
		t.Fatal("sensitive data leaked")
	}
	for _, tc := range []struct {
		filter AdminOsuFilter
		total  int64
		length int
	}{
		{AdminOsuFilter{Limit: 25, Offset: 25}, 28, 3},
		{AdminOsuFilter{Limit: 25, Visibility: "private", Status: "none"}, 8, 8},
		{AdminOsuFilter{Limit: 25, Status: "pending"}, 10, 10},
		{AdminOsuFilter{Limit: 25, Status: "uploaded", Search: "pLaYeR"}, 10, 10},
		{AdminOsuFilter{Limit: 25, Search: "%"}, 28, 25},
		{AdminOsuFilter{Limit: 25, Search: "' OR 1=1 --"}, 0, 0},
		{AdminOsuFilter{Limit: 25, Offset: 100}, 28, 0},
	} {
		got, err := s.GetAdminOsuOverview(ctx, tc.filter)
		if err != nil {
			t.Fatal(err)
		}
		if got.Total != tc.total || len(got.Items) != tc.length {
			t.Fatalf("filter %+v: total %d length %d", tc.filter, got.Total, len(got.Items))
		}
	}
}
