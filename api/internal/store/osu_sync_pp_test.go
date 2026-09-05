package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOsuReuploadRepairsOnlyMissingPP(t *testing.T) {
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
	schema := fmt.Sprintf("osu_pp_repair_%d", time.Now().UnixNano())
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
	for _, sql := range []string{schemaSQL, hubUserIdentityViewSQL, `INSERT INTO hub_users (id, external_id, profile_handle) VALUES (1,'test','player')`} {
		if _, err = isolated.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	s := &Store{pool: isolated}
	input := OsuSyncInput{ContentHash: strings.Repeat("a", 64), Visibility: "public", Profile: OsuProfileInput{OsuUserID: 7, Username: "player"},
		BeatmapSet: OsuBeatmapSetInput{SetKey: "set", Title: "Map"}, Difficulty: OsuBeatmapDifficultyInput{DifficultyKey: "diff", SetKey: "set", Ruleset: "osu"},
		Score: OsuScoreInput{ClientScoreID: "score", OnlineScoreID: 42, PlayedAt: time.Now(), Mods: []string{}}}
	first, err := s.SaveOsuSync(ctx, 1, input)
	if err != nil {
		t.Fatal(err)
	}
	pp := 123.5
	input.Score.PerformancePoints = &pp
	second, err := s.SaveOsuSync(ctx, 1, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ShareID != second.ShareID || second.Created {
		t.Fatal("reupload created duplicate")
	}
	for _, next := range []*float64{nil, new(float64)} {
		input.Score.PerformancePoints = next
		if _, err = s.SaveOsuSync(ctx, 1, input); err != nil {
			t.Fatal(err)
		}
		replay, err := s.GetOsuPublicReplay(ctx, first.ShareID)
		if err != nil {
			t.Fatal(err)
		}
		if replay.PerformancePoints == nil || *replay.PerformancePoints != pp || replay.OnlineScoreID != 42 {
			t.Fatalf("repair/projection %+v", replay)
		}
	}
	t.Run("NumericOsuProfileRetainsOnlyPublicUploadsAndDoesNotMatchNumericHandle", func(t *testing.T) {
		_, err := isolated.Exec(ctx, `
			INSERT INTO hub_users (id,external_id,profile_handle) VALUES (2,'numeric-handle','7'),(3,'hidden-profile','hidden');
			INSERT INTO osu_profiles (user_id,osu_user_id,username) VALUES (2,88,'Different player'),(3,99,'Hidden player');
			INSERT INTO osu_scores (user_id,client_score_id,content_hash,difficulty_key,played_at,visibility,share_id) VALUES
			(1,'private','private','diff',NOW(),'private','private-share'),
			(1,'unlisted','unlisted','diff',NOW(),'unlisted','unlisted-share'),
			(2,'numeric','numeric','diff',NOW(),'public','numeric-share'),
			(3,'hidden','hidden','diff',NOW(),'private','hidden-share');
		`)
		if err != nil {
			t.Fatal(err)
		}
		profile, err := s.GetOsuPublicProfileByOsuUserID(ctx, 7, 100)
		if err != nil {
			t.Fatal(err)
		}
		if profile.HubHandle != "player" || profile.OsuUserID != 7 || profile.SharedReplayCount != 1 || len(profile.RecentReplays) != 1 || profile.RecentReplays[0].ShareID != first.ShareID {
			t.Fatalf("wrong public profile %+v", profile)
		}
		numericHandle, err := s.GetOsuPublicProfile(ctx, "7", 100)
		if err != nil || numericHandle.OsuUserID != 88 {
			t.Fatalf("existing numeric handle changed: %+v %v", numericHandle, err)
		}
		if hidden, err := s.GetOsuPublicProfileByOsuUserID(ctx, 99, 100); err == nil || hidden.HubHandle != "" {
			t.Fatalf("private-only profile disclosed: %+v %v", hidden, err)
		}
	})
}
