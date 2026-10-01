package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/veryCrunchy/aimmod-hub/api/internal/kovaaksbenchmarks"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

// storedBenchmarkCatalog returns the synced benchmark catalog from Postgres,
// cached briefly. It is empty until the background sync has run.
func (s *HubServer) storedBenchmarkCatalog(ctx context.Context) []store.KovaaksBenchmarkRecord {
	if s.store == nil {
		return nil
	}
	records, err := s.storedCatalog.get(ctx, "catalog", 2*time.Minute, func(ctx context.Context) ([]store.KovaaksBenchmarkRecord, error) {
		return s.store.ListKovaaksBenchmarkCatalog(ctx)
	}, 30*time.Minute)
	if err != nil {
		return nil
	}
	return records
}

func (s *HubServer) buildBenchmarkList(ctx context.Context) ([]*hubv1.BenchmarkListItem, error) {
	stored := s.storedBenchmarkCatalog(ctx)
	var catalog []*hubv1.BenchmarkListItem
	if len(stored) > 0 {
		catalog = make([]*hubv1.BenchmarkListItem, 0, len(stored))
		for _, record := range stored {
			hidden, reason := benchmarkQuality(record.Name, record.ScenarioCount, record.SyncedAt != nil, record.PlayerEstimate)
			item := &hubv1.BenchmarkListItem{
				BenchmarkId: record.BenchmarkID, BenchmarkName: record.Name,
				BenchmarkIconUrl: record.IconURL, BenchmarkAuthor: record.Author,
				BenchmarkType: record.BenchmarkType, ScenarioCount: uint32(record.ScenarioCount),
				Hidden: hidden, HiddenReason: reason,
			}
			if record.PlayerEstimate != nil && *record.PlayerEstimate > 0 {
				item.KovaaksPlayers = uint64(*record.PlayerEstimate)
			}
			catalog = append(catalog, item)
		}
	} else {
		live, err := s.benchmarkCatalog.get(ctx, "catalog", 15*time.Minute, func(ctx context.Context) ([]*hubv1.BenchmarkListItem, error) {
			items, err := s.benchmarks.ListCatalog(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]*hubv1.BenchmarkListItem, 0, len(items))
			for _, item := range items {
				out = append(out, &hubv1.BenchmarkListItem{
					BenchmarkId: item.BenchmarkID, BenchmarkName: item.BenchmarkName,
					BenchmarkIconUrl: item.BenchmarkIconURL, BenchmarkAuthor: item.BenchmarkAuthor,
					BenchmarkType: item.BenchmarkType,
				})
			}
			return out, nil
		}, time.Hour)
		if err != nil {
			return nil, err
		}
		catalog = make([]*hubv1.BenchmarkListItem, 0, len(live))
		for _, item := range live {
			hidden, reason := benchmarkQuality(item.BenchmarkName, 0, false, nil)
			copied := proto.Clone(item).(*hubv1.BenchmarkListItem)
			copied.Hidden, copied.HiddenReason = hidden, reason
			catalog = append(catalog, copied)
		}
	}
	counts := s.benchmarkCounts.snapshotAndRefresh(ctx, s.loadBenchmarkList)
	for _, item := range catalog {
		item.PlayerCount = counts[item.BenchmarkId]
	}
	sortBenchmarkList(catalog)
	return catalog, nil
}

// sortBenchmarkList puts visible benchmarks first, then AimMod players,
// then KovaaK's popularity, then name.
func sortBenchmarkList(items []*hubv1.BenchmarkListItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Hidden != b.Hidden {
			return !a.Hidden
		}
		if a.PlayerCount != b.PlayerCount {
			return a.PlayerCount > b.PlayerCount
		}
		if a.KovaaksPlayers != b.KovaaksPlayers {
			return a.KovaaksPlayers > b.KovaaksPlayers
		}
		return strings.ToLower(a.BenchmarkName) < strings.ToLower(b.BenchmarkName)
	})
}

// benchmarkMeta finds public metadata for a benchmark id.
func (s *HubServer) benchmarkMeta(ctx context.Context, benchmarkID uint32) *kovaaksbenchmarks.ProfileBenchmarkSummary {
	for _, record := range s.storedBenchmarkCatalog(ctx) {
		if record.BenchmarkID == benchmarkID {
			return &kovaaksbenchmarks.ProfileBenchmarkSummary{
				BenchmarkID: record.BenchmarkID, BenchmarkName: record.Name, BenchmarkIconURL: record.IconURL,
				BenchmarkAuthor: record.Author, BenchmarkType: record.BenchmarkType,
			}
		}
	}
	list, err := s.buildBenchmarkList(ctx)
	if err != nil {
		return nil
	}
	for _, item := range list {
		if item.BenchmarkId == benchmarkID {
			summary := benchmarkSummaryFromListItem(item)
			return &summary
		}
	}
	return nil
}

func benchmarkScenarioEntry(entry kovaaksbenchmarks.BenchmarkScenarioPage) *hubv1.BenchmarkScenarioEntry {
	thresholds := make([]*hubv1.BenchmarkThreshold, 0, len(entry.Thresholds))
	for _, threshold := range entry.Thresholds {
		thresholds = append(thresholds, benchmarkThreshold(threshold))
	}
	return &hubv1.BenchmarkScenarioEntry{
		ScenarioName:    entry.ScenarioName,
		ScenarioSlug:    slugifyScenarioName(entry.ScenarioName),
		CategoryName:    entry.CategoryName,
		Score:           entry.Score,
		LeaderboardRank: entry.LeaderboardRank,
		LeaderboardId:   entry.LeaderboardID,
		ScenarioRank:    benchmarkRankVisual(entry.ScenarioRank),
		Thresholds:      thresholds,
		ScoreSource:     entry.ScoreSource,
	}
}

// benchmarkHasParticipation reports whether a player has any score on a
// benchmark, from KovaaK's or from their own AimMod uploads. Every
// definition carries leaderboard ids, so those do not count.
func benchmarkHasParticipation(detail *kovaaksbenchmarks.BenchmarkDetail, localScores map[string]float64) bool {
	if detail == nil {
		return false
	}
	for _, category := range detail.Categories {
		for scenarioName, scenario := range category.Scenarios {
			if _, ok := localScores[scenarioName]; ok {
				return true
			}
			if scenario.Score > 0 || scenario.ScenarioRank > 0 || scenario.LeaderboardRank > 0 {
				return true
			}
		}
	}
	return false
}

func categoryPages(categories []kovaaksbenchmarks.BenchmarkCategoryPageRecord) []*hubv1.BenchmarkCategoryPage {
	out := make([]*hubv1.BenchmarkCategoryPage, 0, len(categories))
	for _, category := range categories {
		scenarios := make([]*hubv1.BenchmarkScenarioEntry, 0, len(category.Scenarios))
		for _, scenario := range category.Scenarios {
			scenarios = append(scenarios, benchmarkScenarioEntry(scenario))
		}
		out = append(out, &hubv1.BenchmarkCategoryPage{
			CategoryName:      category.CategoryName,
			CategoryRank:      category.CategoryRank,
			Scenarios:         scenarios,
			BenchmarkProgress: category.BenchmarkProgress,
		})
	}
	return out
}

func rankLadder(ranks []kovaaksbenchmarks.BenchmarkRankVisual) []*hubv1.BenchmarkRankVisual {
	out := make([]*hubv1.BenchmarkRankVisual, 0, len(ranks))
	for _, rank := range ranks {
		out = append(out, benchmarkRankVisual(rank))
	}
	return out
}

func (s *HubServer) GetBenchmarkPage(
	ctx context.Context,
	req *connect.Request[hubv1.GetBenchmarkPageRequest],
) (*connect.Response[hubv1.GetBenchmarkPageResponse], error) {
	handle := strings.TrimSpace(req.Msg.GetHandle())
	steamID := strings.TrimSpace(req.Msg.GetSteamId())
	benchmarkID := req.Msg.GetBenchmarkId()
	if benchmarkID == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("benchmark_id is required"))
	}
	if handle == "" && steamID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("handle or steam_id is required"))
	}
	if handle == "" {
		return s.kovaaksPlayerBenchmarkPage(ctx, steamID, benchmarkID)
	}

	profile, err := s.store.GetProfileMeta(ctx, handle)
	if err != nil || profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("profile not found"))
	}
	identity, err := s.store.GetBenchmarkIdentityByHandle(ctx, handle)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if strings.TrimSpace(identity.SteamID) == "" {
		// Ranks come only from a linked account; there is nothing to show.
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("this profile has no linked Steam account"))
	}

	summary := s.benchmarkMeta(ctx, benchmarkID)
	if summary == nil {
		preloaded, _ := s.benchmarks.ListPlayerBenchmarks(ctx, identity.KovaaksUsername)
		for i := range preloaded {
			if preloaded[i].BenchmarkID == benchmarkID {
				summary = &preloaded[i]
				break
			}
		}
	}
	if summary == nil {
		summary = &kovaaksbenchmarks.ProfileBenchmarkSummary{BenchmarkID: benchmarkID}
	}

	// The player's own AimMod uploads may beat what KovaaK's shows (for
	// example when KovaaK's suppresses their leaderboard entries).
	localScores, _ := s.store.GetBestScoresByHandle(ctx, handle)
	detail, err := s.benchmarks.GetBenchmarkDetail(ctx, benchmarkID, identity.SteamID)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if detail == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("benchmark detail not found"))
	}
	categories := kovaaksbenchmarks.BuildCategoryPages(detail, kovaaksbenchmarks.PageOptions{LocalScores: localScores, ComputeRanks: true})
	overallRank := kovaaksbenchmarks.OverallRankFromCategories(categories, detail.Ranks)

	return connect.NewResponse(&hubv1.GetBenchmarkPageResponse{
		UserHandle:        profile.Handle,
		UserDisplayName:   profile.DisplayName,
		BenchmarkId:       summary.BenchmarkID,
		BenchmarkName:     summary.BenchmarkName,
		BenchmarkIconUrl:  summary.BenchmarkIconURL,
		BenchmarkAuthor:   summary.BenchmarkAuthor,
		BenchmarkType:     summary.BenchmarkType,
		OverallRank:       benchmarkRankVisual(overallRank),
		Categories:        categoryPages(categories),
		Ranks:             rankLadder(detail.Ranks),
		BenchmarkProgress: detail.BenchmarkProgress,
		RankHistory:       s.rankHistory(ctx, handle, categories),
		AimmodHandle:      profile.Handle,
	}), nil
}

// rankHistory replays the player's own AimMod uploads against the
// benchmark's thresholds.
func (s *HubServer) rankHistory(ctx context.Context, handle string, categories []kovaaksbenchmarks.BenchmarkCategoryPageRecord) []*hubv1.BenchmarkRankHistoryPoint {
	var scenarios []historyScenario
	var names []string
	for _, category := range categories {
		for _, scenario := range category.Scenarios {
			scenarios = append(scenarios, historyScenario{Name: scenario.ScenarioName, RankMaxes: rankMaxesFromThresholds(scenario.Thresholds)})
			names = append(names, scenario.ScenarioName)
		}
	}
	runs, err := s.store.PlayerRunsForScenarios(ctx, handle, names)
	if err != nil || len(runs) == 0 {
		return nil
	}
	history := make([]historyRun, 0, len(runs))
	for _, run := range runs {
		history = append(history, historyRun{ScenarioName: run.ScenarioName, Score: run.Score, PlayedAt: run.PlayedAt})
	}
	points := benchmarkRankHistory(scenarios, history)
	out := make([]*hubv1.BenchmarkRankHistoryPoint, 0, len(points))
	for _, point := range points {
		out = append(out, &hubv1.BenchmarkRankHistoryPoint{
			DateIso:          point.Day.Format("2006-01-02"),
			OverallRankIndex: point.OverallRankIndex,
			AverageRankIndex: point.AverageRankIndex,
			RankedScenarios:  point.RankedScenarios,
		})
	}
	return out
}

// rankMaxesFromThresholds rebuilds a rank_maxes array (index n = minimum for
// rank n+1) from threshold records, which may skip placeholder ranks.
func rankMaxesFromThresholds(thresholds []kovaaksbenchmarks.BenchmarkThreshold) []float64 {
	if len(thresholds) == 0 {
		return nil
	}
	maxIndex := uint32(0)
	for _, threshold := range thresholds {
		if threshold.RankIndex > maxIndex {
			maxIndex = threshold.RankIndex
		}
	}
	maxes := make([]float64, maxIndex)
	for i := range maxes {
		maxes[i] = -1
	}
	for _, threshold := range thresholds {
		if threshold.RankIndex > 0 {
			maxes[threshold.RankIndex-1] = threshold.Score
		}
	}
	// Fill gaps with the next defined threshold so rank order holds.
	next := maxes[len(maxes)-1]
	for i := len(maxes) - 1; i >= 0; i-- {
		if maxes[i] < 0 {
			maxes[i] = next
		} else {
			next = maxes[i]
		}
	}
	return maxes
}

// kovaaksPlayerBenchmarkPage shows a benchmark sheet for any KovaaK's player
// from KovaaK's public data only.
func (s *HubServer) kovaaksPlayerBenchmarkPage(ctx context.Context, steamID string, benchmarkID uint32) (*connect.Response[hubv1.GetBenchmarkPageResponse], error) {
	if !looksLikeSteam64(steamID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("steam_id must be a Steam64 id"))
	}
	detail, err := s.benchmarks.GetBenchmarkDetail(ctx, benchmarkID, steamID)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if detail == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("benchmark not found"))
	}
	summary := s.benchmarkMeta(ctx, benchmarkID)
	if summary == nil {
		summary = &kovaaksbenchmarks.ProfileBenchmarkSummary{BenchmarkID: benchmarkID}
	}
	categories := kovaaksbenchmarks.BuildCategoryPages(detail, kovaaksbenchmarks.PageOptions{ComputeRanks: true})
	overallRank := kovaaksbenchmarks.OverallRankFromCategories(categories, detail.Ranks)

	resp := &hubv1.GetBenchmarkPageResponse{
		BenchmarkId:       benchmarkID,
		BenchmarkName:     summary.BenchmarkName,
		BenchmarkIconUrl:  summary.BenchmarkIconURL,
		BenchmarkAuthor:   summary.BenchmarkAuthor,
		BenchmarkType:     summary.BenchmarkType,
		OverallRank:       benchmarkRankVisual(overallRank),
		Categories:        categoryPages(categories),
		Ranks:             rankLadder(detail.Ranks),
		BenchmarkProgress: detail.BenchmarkProgress,
		IsKovaaksOnly:     true,
		SteamId:           steamID,
	}
	if linked, err := s.store.GetBenchmarkIdentityBySteamId(ctx, steamID); err == nil && linked != nil {
		resp.AimmodHandle = linked.UserHandle
		resp.IsKovaaksOnly = false
	}
	if cached, err := s.store.GetKovaaksUserBySteamId(ctx, steamID); err == nil && cached != nil {
		resp.KovaaksUsername = cached.Username
		resp.UserDisplayName = firstNonEmpty(cached.DisplayName, cached.Username)
		resp.AvatarUrl = cached.AvatarURL
	}
	return connect.NewResponse(resp), nil
}

func looksLikeSteam64(value string) bool {
	if len(value) != 17 || !strings.HasPrefix(value, "7656119") {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s *HubServer) fetchProfileBenchmarks(ctx context.Context, handle string) ([]*hubv1.BenchmarkSummary, []kovaaksbenchmarks.ProfileBenchmarkSummary, error) {
	identity, err := s.store.GetBenchmarkIdentityByHandle(ctx, handle)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(identity.KovaaksUsername) == "" && strings.TrimSpace(identity.SteamID) == "" {
		// Without a linked KovaaK's or Steam account there are no ranks to look up.
		return nil, nil, nil
	}

	items, listErr := s.benchmarks.ListPlayerBenchmarks(ctx, identity.KovaaksUsername)
	if strings.TrimSpace(identity.SteamID) == "" {
		if listErr != nil {
			return nil, nil, listErr
		}
		out := make([]*hubv1.BenchmarkSummary, 0, len(items))
		for _, item := range items {
			out = append(out, benchmarkSummary(item))
		}
		return out, items, nil
	}

	localScores, _ := s.store.GetBestScoresByHandle(ctx, handle)

	// Candidates: benchmarks KovaaK's already ranks the player on, plus
	// benchmarks that contain a scenario the player uploaded through AimMod.
	// Checking only these keeps provider requests proportional to the
	// player's activity instead of the whole catalog.
	candidates := make(map[uint32]kovaaksbenchmarks.ProfileBenchmarkSummary)
	for _, item := range items {
		candidates[item.BenchmarkID] = item
	}
	if len(localScores) > 0 {
		names := make([]string, 0, len(localScores))
		for name := range localScores {
			names = append(names, name)
		}
		if memberships, err := s.store.ListBenchmarksContainingScenarios(ctx, names); err == nil {
			for _, m := range memberships {
				if _, ok := candidates[m.BenchmarkID]; ok {
					continue
				}
				if meta := s.benchmarkMeta(ctx, m.BenchmarkID); meta != nil {
					candidates[m.BenchmarkID] = *meta
				}
			}
		}
	}
	if len(candidates) == 0 && listErr != nil {
		return nil, nil, listErr
	}

	type benchmarkResult struct {
		summary kovaaksbenchmarks.ProfileBenchmarkSummary
		rank    kovaaksbenchmarks.BenchmarkRankVisual
	}
	results := make(chan benchmarkResult, len(candidates))
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup
	for _, item := range candidates {
		wg.Add(1)
		go func(item kovaaksbenchmarks.ProfileBenchmarkSummary) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			detail, err := s.benchmarks.GetBenchmarkDetail(ctx, item.BenchmarkID, identity.SteamID)
			if err != nil || !benchmarkHasParticipation(detail, localScores) {
				return
			}
			categories := kovaaksbenchmarks.BuildCategoryPages(detail, kovaaksbenchmarks.PageOptions{LocalScores: localScores, ComputeRanks: true})
			rank := kovaaksbenchmarks.OverallRankFromCategories(categories, detail.Ranks)
			if rank.RankName == "" && strings.TrimSpace(item.OverallRankName) != "" {
				rank = kovaaksbenchmarks.BenchmarkRankVisual{RankName: item.OverallRankName, IconURL: item.OverallRankIcon, Color: item.OverallRankColor}
			}
			results <- benchmarkResult{summary: item, rank: rank}
		}(item)
	}
	go func() { wg.Wait(); close(results) }()

	computed := make([]benchmarkResult, 0, len(candidates))
	for result := range results {
		computed = append(computed, result)
	}
	sort.Slice(computed, func(i, j int) bool {
		if computed[i].summary.BenchmarkName != computed[j].summary.BenchmarkName {
			return computed[i].summary.BenchmarkName < computed[j].summary.BenchmarkName
		}
		return computed[i].summary.BenchmarkID < computed[j].summary.BenchmarkID
	})
	out := make([]*hubv1.BenchmarkSummary, 0, len(computed))
	preloaded := make([]kovaaksbenchmarks.ProfileBenchmarkSummary, 0, len(computed))
	for _, item := range computed {
		out = append(out, benchmarkSummaryWithRank(item.summary, item.rank))
		preloaded = append(preloaded, item.summary)
	}
	return out, preloaded, nil
}

func (s *HubServer) GetBenchmarkLeaderboard(
	ctx context.Context,
	req *connect.Request[hubv1.GetBenchmarkLeaderboardRequest],
) (*connect.Response[hubv1.GetBenchmarkLeaderboardResponse], error) {
	benchmarkID := req.Msg.GetBenchmarkId()
	if benchmarkID == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("benchmark_id is required"))
	}
	users, err := s.store.ListUsersWithBenchmarkIdentity(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	type fetchResult struct {
		entry *hubv1.BenchmarkLeaderboardEntry
	}
	results := make(chan fetchResult, len(users))
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup
	for _, u := range users {
		wg.Add(1)
		go func(u store.BenchmarkUserIdentity) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			detail, err := s.benchmarks.GetBenchmarkDetail(ctx, benchmarkID, u.SteamID)
			if err != nil || detail == nil || detail.OverallRank == 0 {
				return
			}
			rv := kovaaksbenchmarks.RankVisualFromDetail(detail, detail.OverallRank)
			results <- fetchResult{entry: &hubv1.BenchmarkLeaderboardEntry{
				UserHandle:         u.UserHandle,
				DisplayName:        u.DisplayName,
				AvatarUrl:          u.AvatarURL,
				OverallRankName:    rv.RankName,
				OverallRankIconUrl: rv.IconURL,
				OverallRankIndex:   detail.OverallRank,
				BenchmarkProgress:  detail.BenchmarkProgress,
				OverallRankColor:   rv.Color,
			}}
		}(u)
	}
	go func() { wg.Wait(); close(results) }()

	var entries []*hubv1.BenchmarkLeaderboardEntry
	for r := range results {
		entries = append(entries, r.entry)
	}
	sortBenchmarkLeaderboard(entries)

	resp := &hubv1.GetBenchmarkLeaderboardResponse{BenchmarkId: benchmarkID, Entries: entries}
	if meta := s.benchmarkMeta(ctx, benchmarkID); meta != nil {
		resp.BenchmarkName = meta.BenchmarkName
		resp.BenchmarkIconUrl = meta.BenchmarkIconURL
	}
	return connect.NewResponse(resp), nil
}

// sortBenchmarkLeaderboard orders by rank, then KovaaK's progress, then name.
func sortBenchmarkLeaderboard(entries []*hubv1.BenchmarkLeaderboardEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.OverallRankIndex != b.OverallRankIndex {
			return a.OverallRankIndex > b.OverallRankIndex
		}
		if a.BenchmarkProgress != b.BenchmarkProgress {
			return a.BenchmarkProgress > b.BenchmarkProgress
		}
		return strings.ToLower(a.UserHandle) < strings.ToLower(b.UserHandle)
	})
}
