package store

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Sitemap queries use the same public records as the page APIs. Each row is a
// list of path segments, escaped only after querying so names remain intact.
// No replay files, private scores or unlisted share links are published here.
const scenarioSitemapSlug = `trim(both '-' from regexp_replace(regexp_replace(lower(trim(sr.scenario_name)), '[^a-z0-9 _''.-]', '', 'g'), '[- _''.]+', '-', 'g'))`

var sitemapQueries = map[string]string{
	"profiles":            `SELECT ARRAY['profiles', hui.user_handle] AS segments FROM hub_user_identity hui WHERE trim(hui.user_handle) <> ''`,
	"runs":                `SELECT ARRAY['runs', COALESCE(NULLIF(public_run_id, ''), session_id)] AS segments FROM scenario_runs`,
	"scenarios":           `SELECT DISTINCT ARRAY['scenarios', ` + scenarioSitemapSlug + `] AS segments FROM scenario_runs sr WHERE ` + scenarioSitemapSlug + ` <> ''`,
	"player-scenarios":    `SELECT DISTINCT ARRAY['profiles', hui.user_handle, 'scenarios', ` + scenarioSitemapSlug + `] AS segments FROM scenario_runs sr JOIN hub_user_identity hui ON hui.user_id = sr.user_id WHERE trim(hui.user_handle) <> '' AND ` + scenarioSitemapSlug + ` <> ''`,
	"profile-benchmarks":  `SELECT ARRAY['profiles', hui.user_handle, 'benchmarks'] AS segments FROM hub_user_identity hui WHERE trim(hui.user_handle) <> '' AND EXISTS (SELECT 1 FROM linked_accounts a WHERE a.user_id = hui.user_id AND a.provider = 'steam' AND trim(a.provider_account_id) <> '')`,
	"osu-profiles":        `SELECT ARRAY['osu', 'profiles', hui.user_handle] AS segments FROM hub_user_identity hui JOIN osu_profiles p ON p.user_id = hui.user_id WHERE trim(hui.user_handle) <> '' AND EXISTS (SELECT 1 FROM osu_scores s WHERE s.user_id = hui.user_id AND s.visibility = 'public')`,
	"osu-public-players":  `SELECT DISTINCT ARRAY['osu','profiles',osu_user_id::text] AS segments FROM osu_public_players WHERE updated_at>NOW()-INTERVAL '30 days'`,
	"osu-replays":         `SELECT ARRAY['osu', 'replays', s.share_id] AS segments FROM osu_scores s JOIN osu_profiles p ON p.user_id = s.user_id JOIN osu_beatmap_difficulties d ON d.difficulty_key = s.difficulty_key JOIN osu_beatmap_sets b ON b.set_key = d.set_key WHERE s.visibility = 'public'`,
	"external-profiles":   `SELECT ARRAY['u', steam_id] AS segments FROM kovaaks_user_cache WHERE trim(steam_id) <> ''`,
	"osu-public-scores":   `SELECT ARRAY['osu','scores',score_id::text] AS segments FROM osu_public_score_index`,
	"osu-scores":          `SELECT DISTINCT ARRAY['osu', 'scores', online_score_id::text] AS segments FROM osu_scores WHERE visibility = 'public' AND online_score_id > 0`,
	"benchmarks":          `SELECT DISTINCT ARRAY['benchmarks', benchmark_id::text] AS segments FROM sitemap_benchmarks`,
	"player-benchmarks":   `SELECT DISTINCT ARRAY['profiles', hui.user_handle, 'benchmarks', b.benchmark_id::text] AS segments FROM hub_user_identity hui JOIN linked_accounts steam ON steam.user_id = hui.user_id AND steam.provider = 'steam' LEFT JOIN linked_accounts k ON k.user_id = hui.user_id AND k.provider = 'kovaaks' JOIN sitemap_benchmarks b ON b.username = lower(COALESCE(NULLIF(trim(k.username), ''), NULLIF(trim(k.display_name), ''), k.provider_account_id, hui.user_handle)) WHERE trim(steam.provider_account_id) <> '' AND trim(hui.user_handle) <> ''`,
	"external-benchmarks": `SELECT DISTINCT ARRAY['u', c.steam_id, 'benchmarks', b.benchmark_id::text] AS segments FROM kovaaks_user_cache c JOIN sitemap_benchmarks b ON b.username = lower(trim(c.kovaaks_username)) WHERE trim(c.steam_id) <> ''`,
	"kovaaks-profiles":    `SELECT DISTINCT ARRAY['u', 'kovaaks', b.username] AS segments FROM sitemap_benchmarks b WHERE NOT EXISTS (SELECT 1 FROM kovaaks_user_cache c WHERE lower(trim(c.kovaaks_username)) = b.username AND trim(c.steam_id) <> '')`,
}

// Replace only after a successful provider response. Failed requests leave the
// last known public catalog available for discovery.
func (s *Store) SaveSitemapBenchmarks(ctx context.Context, username string, ids []uint32) error {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize replacement for the same public player if provider requests race.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, username); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM sitemap_benchmarks WHERE username = $1`, username); err != nil {
		return err
	}
	values := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			values = append(values, int64(id))
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sitemap_benchmarks (username, benchmark_id) SELECT $1, id FROM unnest($2::bigint[]) AS id ON CONFLICT DO NOTHING`, username, values); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SitemapCounts(ctx context.Context) (map[string]int64, error) {
	counts := make(map[string]int64, len(sitemapQueries))
	for kind, query := range sitemapQueries {
		var count int64
		if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM ("+query+") entries").Scan(&count); err != nil {
			return nil, fmt.Errorf("count sitemap %s: %w", kind, err)
		}
		counts[kind] = count
	}
	return counts, nil
}

func (s *Store) SitemapPaths(ctx context.Context, kind string, limit, offset int) ([]string, error) {
	query, ok := sitemapQueries[kind]
	if !ok || limit < 1 || limit > 5000 || offset < 0 {
		return nil, fmt.Errorf("invalid sitemap page")
	}
	rows, err := s.pool.Query(ctx, "SELECT segments FROM ("+query+") entries ORDER BY segments COLLATE \"C\" LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list sitemap %s: %w", kind, err)
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var segments []string
		if err := rows.Scan(&segments); err != nil {
			return nil, err
		}
		for i, segment := range segments {
			segments[i] = url.PathEscape(segment)
		}
		paths = append(paths, "/"+strings.Join(segments, "/"))
	}
	return paths, rows.Err()
}
