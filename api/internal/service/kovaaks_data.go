package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/veryCrunchy/aimmod-hub/api/internal/kovaaksbenchmarks"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

const optionalProviderTimeout = 1500 * time.Millisecond

var scenarioPercentiles = []uint32{10, 25, 50, 75, 90, 99}

func isoTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// ── Scenario page ────────────────────────────────────────────────────────────

func (s *HubServer) GetScenarioPage(
	ctx context.Context,
	req *connect.Request[hubv1.GetScenarioPageRequest],
) (*connect.Response[hubv1.GetScenarioPageResponse], error) {
	slug := strings.TrimSpace(req.Msg.GetSlug())
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	page, err := s.store.GetScenarioPage(ctx, slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	resp := &hubv1.GetScenarioPageResponse{
		ScenarioName:      page.ScenarioName,
		ScenarioSlug:      page.ScenarioSlug,
		ScenarioType:      page.ScenarioType,
		RunCount:          page.RunCount,
		BestScore:         page.BestScore,
		AverageScore:      page.AverageScore,
		AverageAccuracy:   page.AverageAccuracy,
		AverageDurationMs: page.AverageDurationMS,
		RecentRuns:        page.RecentRuns,
		TopRuns:           page.TopRuns,
		ScoreDistribution: page.ScoreDistribution,
		PlayerCount:       page.PlayerCount,
		RunsLast_7Days:    page.RunsLast7Days,
	}

	if bests, err := s.store.ScenarioPlayerBests(ctx, page.ScenarioName); err == nil && len(bests) > 0 {
		scores := make([]float64, len(bests))
		for i, best := range bests {
			scores[i] = best.Score
		}
		if len(bests) >= 3 {
			values := percentileScores(scores, scenarioPercentiles)
			for _, p := range scenarioPercentiles {
				resp.Percentiles = append(resp.Percentiles, &hubv1.ScorePercentile{Percentile: p, Score: values[p]})
			}
		}
		if handle := strings.TrimSpace(req.Msg.GetHandle()); handle != "" {
			resp.Viewer = viewerStanding(bests, handle)
		}
	}

	if memberships, err := s.store.ListBenchmarksContainingScenarios(ctx, []string{page.ScenarioName}); err == nil {
		seen := map[uint32]bool{}
		for _, m := range memberships {
			if seen[m.BenchmarkID] {
				continue
			}
			seen[m.BenchmarkID] = true
			ranks := make([]kovaaksbenchmarks.BenchmarkRankVisual, 0, len(m.Ranks))
			for _, r := range m.Ranks {
				ranks = append(ranks, kovaaksbenchmarks.BenchmarkRankVisual{RankIndex: r.RankIndex, RankName: r.RankName, IconURL: r.IconURL, Color: r.Color, FrameURL: r.FrameURL})
			}
			membership := &hubv1.ScenarioBenchmarkMembership{
				BenchmarkId: m.BenchmarkID, BenchmarkName: m.BenchmarkName, BenchmarkIconUrl: m.IconURL, CategoryName: m.CategoryName,
			}
			for _, threshold := range kovaaksbenchmarks.ThresholdsFor(ranks, m.RankMaxes) {
				membership.Thresholds = append(membership.Thresholds, benchmarkThreshold(threshold))
			}
			if resp.KovaaksLeaderboardId == 0 && m.LeaderboardID > 0 {
				resp.KovaaksLeaderboardId = m.LeaderboardID
			}
			resp.Benchmarks = append(resp.Benchmarks, membership)
			if len(resp.Benchmarks) >= 12 {
				break
			}
		}
	}

	providerCtx, cancel := context.WithTimeout(ctx, optionalProviderTimeout)
	defer cancel()
	if item, err := s.benchmarks.FindScenario(providerCtx, page.ScenarioName); err == nil && item != nil {
		resp.KovaaksLeaderboardId = item.LeaderboardID
		resp.KovaaksPlays = item.Plays
		resp.KovaaksEntries = item.Entries
	}
	return connect.NewResponse(resp), nil
}

// viewerStanding finds a player's rank and share beaten in a best-first list.
func viewerStanding(bests []store.PlayerBest, handle string) *hubv1.PlayerScenarioStanding {
	for i, best := range bests {
		if !strings.EqualFold(best.UserHandle, handle) {
			continue
		}
		rank := 1
		for j := 0; j < i; j++ {
			if bests[j].Score > best.Score {
				rank++
			}
		}
		below := 0
		for _, other := range bests[i+1:] {
			if other.Score < best.Score {
				below++
			}
		}
		return &hubv1.PlayerScenarioStanding{
			UserHandle: best.UserHandle,
			BestScore:  best.Score,
			Rank:       uint32(rank),
			Percentile: beatsShare(uint32(below), uint32(len(bests))),
			RunCount:   best.RunCount,
		}
	}
	return nil
}

// ── Scenario leaderboards ────────────────────────────────────────────────────

func (s *HubServer) GetScenarioLeaderboard(
	ctx context.Context,
	req *connect.Request[hubv1.GetScenarioLeaderboardRequest],
) (*connect.Response[hubv1.GetScenarioLeaderboardResponse], error) {
	msg := req.Msg
	pageSize := msg.GetPageSize()
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	name := strings.TrimSpace(msg.GetScenarioName())
	if name == "" && strings.TrimSpace(msg.GetScenarioSlug()) != "" {
		resolved, err := s.store.ResolveScenarioName(ctx, msg.GetScenarioSlug())
		if err == nil {
			name = resolved
		}
	}

	if strings.EqualFold(msg.GetSource(), "kovaaks") {
		return s.kovaaksScenarioLeaderboard(ctx, name, msg.GetLeaderboardId(), msg.GetPage(), pageSize, msg.GetAroundSteamId(), msg.GetAroundHandle())
	}
	if name == "" {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("scenario not found"))
	}
	since, rangeName := rangeSince(msg.GetRange(), time.Now())
	sortName := strings.ToLower(strings.TrimSpace(msg.GetSort()))
	switch sortName {
	case "accuracy", "recent", "runs":
	default:
		sortName = "score"
	}
	result, err := s.store.ScenarioLeaderboard(ctx, store.ScenarioLeaderboardQuery{
		ScenarioName: name,
		Since:        since,
		Query:        msg.GetQuery(),
		Sort:         sortName,
		LinkedOnly:   msg.GetLinkedOnly(),
		Offset:       int(msg.GetPage() * pageSize),
		Limit:        int(pageSize),
		AroundHandle: msg.GetAroundHandle(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &hubv1.GetScenarioLeaderboardResponse{
		ScenarioName:  name,
		ScenarioSlug:  store.ScenarioSlug(name),
		Source:        "aimmod",
		Total:         result.Total,
		Page:          uint32(result.Offset) / pageSize,
		PageSize:      pageSize,
		HighlightRank: result.HighlightRank,
		Range:         rangeName,
		Sort:          sortName,
	}
	for _, row := range result.Rows {
		resp.Entries = append(resp.Entries, &hubv1.ScenarioLeaderboardEntry{
			Rank:           row.Rank,
			UserHandle:     row.UserHandle,
			DisplayName:    row.DisplayName,
			AvatarUrl:      row.AvatarURL,
			Score:          row.Score,
			Accuracy:       row.Accuracy,
			PlayedAtIso:    isoTime(row.PlayedAt),
			RunId:          row.RunID,
			RunCount:       row.RunCount,
			IsAimmodPlayer: true,
			IsLinked:       row.IsLinked,
		})
	}
	return connect.NewResponse(resp), nil
}

// kovaaksScenarioLeaderboard pages through KovaaK's global board. "Jump to
// me" uses the player's leaderboard rank from their linked Steam account.
func (s *HubServer) kovaaksScenarioLeaderboard(
	ctx context.Context, name string, leaderboardID, page, pageSize uint32, aroundSteamID, aroundHandle string,
) (*connect.Response[hubv1.GetScenarioLeaderboardResponse], error) {
	if leaderboardID == 0 && name != "" {
		if item, err := s.benchmarks.FindScenario(ctx, name); err == nil && item != nil {
			leaderboardID = item.LeaderboardID
		}
	}
	if leaderboardID == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("this scenario has no KovaaK's leaderboard"))
	}
	if aroundSteamID == "" && aroundHandle != "" {
		if identity, err := s.store.GetBenchmarkIdentityByHandle(ctx, aroundHandle); err == nil {
			aroundSteamID = identity.SteamID
		}
	}
	var highlight uint32
	if aroundSteamID != "" && looksLikeSteam64(aroundSteamID) {
		if rank := s.kovaaksLeaderboardRank(ctx, leaderboardID, aroundSteamID); rank > 0 {
			highlight = rank
			page = (rank - 1) / pageSize
		}
	}
	board, err := s.benchmarks.GetScenarioLeaderboard(ctx, leaderboardID, page, pageSize)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	resp := &hubv1.GetScenarioLeaderboardResponse{
		ScenarioName:  name,
		ScenarioSlug:  store.ScenarioSlug(name),
		LeaderboardId: leaderboardID,
		Source:        "kovaaks",
		Total:         board.Total,
		Page:          board.Page,
		PageSize:      board.PageSize,
		HighlightRank: highlight,
		Range:         "all",
		Sort:          "score",
	}
	steamIDs := make([]string, 0, len(board.Entries))
	for _, entry := range board.Entries {
		steamIDs = append(steamIDs, entry.SteamID)
	}
	linked := s.linkedHandlesBySteamID(ctx, steamIDs)
	for _, entry := range board.Entries {
		row := &hubv1.ScenarioLeaderboardEntry{
			Rank:            entry.Rank,
			SteamId:         entry.SteamID,
			KovaaksUsername: entry.KovaaksUsername,
			DisplayName:     firstNonEmpty(entry.SteamName, entry.KovaaksUsername),
			Country:         entry.Country,
			Score:           entry.Score,
			Cm360:           entry.CM360,
			PlayedAtIso:     isoTime(entry.PlayedAt),
		}
		if handle, ok := linked[entry.SteamID]; ok {
			row.UserHandle = handle
			row.IsAimmodPlayer = true
			row.IsLinked = true
		}
		resp.Entries = append(resp.Entries, row)
	}
	return connect.NewResponse(resp), nil
}

// kovaaksLeaderboardRank finds a player's rank on a KovaaK's leaderboard
// through the benchmark detail of any stored benchmark that contains it.
func (s *HubServer) kovaaksLeaderboardRank(ctx context.Context, leaderboardID uint32, steamID string) uint32 {
	memberships, err := s.store.ListBenchmarksByLeaderboardID(ctx, leaderboardID)
	if err != nil || len(memberships) == 0 {
		return 0
	}
	detail, err := s.benchmarks.GetBenchmarkDetail(ctx, memberships[0].BenchmarkID, steamID)
	if err != nil || detail == nil {
		return 0
	}
	for _, category := range detail.Categories {
		for _, scenario := range category.Scenarios {
			if scenario.LeaderboardID == leaderboardID {
				return scenario.LeaderboardRank
			}
		}
	}
	return 0
}

// linkedHandlesBySteamID maps Steam ids to AimMod handles for accounts their
// owners linked. Unlinked players are simply absent.
func (s *HubServer) linkedHandlesBySteamID(ctx context.Context, steamIDs []string) map[string]string {
	out := map[string]string{}
	if len(steamIDs) == 0 {
		return out
	}
	users, err := s.store.ListUsersWithBenchmarkIdentity(ctx)
	if err != nil {
		return out
	}
	wanted := map[string]bool{}
	for _, id := range steamIDs {
		wanted[id] = true
	}
	for _, user := range users {
		if wanted[user.SteamID] {
			out[user.SteamID] = user.UserHandle
		}
	}
	return out
}

// ── Quick search ─────────────────────────────────────────────────────────────

func (s *HubServer) QuickSearch(
	ctx context.Context,
	req *connect.Request[hubv1.QuickSearchRequest],
) (*connect.Response[hubv1.QuickSearchResponse], error) {
	query := strings.TrimSpace(req.Msg.GetQuery())
	limit := int(req.Msg.GetLimit())
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	resp := &hubv1.QuickSearchResponse{Query: query}
	if len([]rune(query)) < 1 {
		return connect.NewResponse(resp), nil
	}
	if len(query) > 80 {
		query = query[:80]
	}
	key := strings.ToLower(query)

	local, err := s.searchResults.get(ctx, key, 30*time.Second, func(ctx context.Context) ([]*hubv1.QuickSearchResult, error) {
		return s.localSearch(ctx, query)
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	results := append([]*hubv1.QuickSearchResult(nil), local...)

	if req.Msg.GetIncludeKovaaks() && len([]rune(query)) >= 3 {
		providerCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		external, err := s.externalSearch.get(providerCtx, key, 10*time.Minute, func(ctx context.Context) ([]*hubv1.QuickSearchResult, error) {
			return s.kovaaksSearch(ctx, query), nil
		})
		cancel()
		if err == nil {
			resp.KovaaksIncluded = true
			results = mergeSearchResults(results, external)
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Relevance > results[j].Relevance })
	if len(results) > limit {
		results = results[:limit]
	}
	resp.Results = results
	return connect.NewResponse(resp), nil
}

func (s *HubServer) localSearch(ctx context.Context, query string) ([]*hubv1.QuickSearchResult, error) {
	index, err := s.store.SearchIndexSnapshot(ctx, 2*time.Minute)
	if err != nil {
		return nil, err
	}
	var out []*hubv1.QuickSearchResult
	for _, profile := range index.Profiles {
		match := max(matchScore(query, profile.UserHandle), matchScore(query, profile.DisplayName), 0.9*matchScore(query, profile.KovaaksUsername))
		if match == 0 {
			continue
		}
		out = append(out, &hubv1.QuickSearchResult{
			Kind:       "player",
			Title:      firstNonEmpty(profile.DisplayName, profile.UserHandle),
			Subtitle:   "@" + profile.UserHandle,
			ImageUrl:   profile.AvatarURL,
			Relevance:  relevance(match, uint64(profile.RunCount)) + 6,
			UserHandle: profile.UserHandle,
			Count:      uint64(profile.RunCount),
		})
	}
	for _, scenario := range index.Scenarios {
		match := matchScore(query, scenario.ScenarioName)
		if match == 0 {
			continue
		}
		out = append(out, &hubv1.QuickSearchResult{
			Kind:         "scenario",
			Title:        scenario.ScenarioName,
			Badge:        scenario.ScenarioType,
			Relevance:    relevance(match, uint64(scenario.RunCount)) + 4,
			ScenarioSlug: scenario.ScenarioSlug,
			ScenarioName: scenario.ScenarioName,
			Count:        uint64(scenario.RunCount),
		})
	}
	if list, err := s.buildBenchmarkList(ctx); err == nil {
		for _, item := range list {
			match := max(matchScore(query, item.BenchmarkName), 0.6*matchScore(query, item.BenchmarkAuthor))
			if match == 0 {
				continue
			}
			score := relevance(match, item.KovaaksPlayers+uint64(item.PlayerCount)*100)
			if item.Hidden {
				score -= 40
			}
			subtitle := ""
			if item.BenchmarkAuthor != "" {
				subtitle = "by " + item.BenchmarkAuthor
			}
			out = append(out, &hubv1.QuickSearchResult{
				Kind:        "benchmark",
				Title:       item.BenchmarkName,
				Subtitle:    subtitle,
				ImageUrl:    item.BenchmarkIconUrl,
				Relevance:   score,
				BenchmarkId: item.BenchmarkId,
				Count:       item.KovaaksPlayers,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Relevance > out[j].Relevance })
	if len(out) > 40 {
		out = out[:40]
	}
	return out, nil
}

// kovaaksSearch asks KovaaK's for players and scenarios. Failures return
// whatever succeeded; search must never break because KovaaK's is slow.
func (s *HubServer) kovaaksSearch(ctx context.Context, query string) []*hubv1.QuickSearchResult {
	var mu sync.Mutex
	var out []*hubv1.QuickSearchResult
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		users, err := s.benchmarks.SearchUsers(ctx, query, 8)
		if err != nil {
			return
		}
		entries := make([]store.KovaaksUserCacheEntry, 0, len(users))
		for _, user := range users {
			entries = append(entries, store.KovaaksUserCacheEntry{SteamID: user.SteamID, Username: user.Username, DisplayName: user.DisplayName, AvatarURL: user.AvatarURL, Country: user.Country})
		}
		_ = s.store.UpsertKovaaksUsers(ctx, entries)
		mu.Lock()
		defer mu.Unlock()
		for _, user := range users {
			if user.SteamID == "" {
				continue
			}
			match := max(matchScore(query, user.Username), matchScore(query, user.DisplayName))
			if match == 0 {
				match = 0.3
			}
			subtitle := "KovaaK's player"
			if user.DisplayName != "" && !strings.EqualFold(user.DisplayName, user.Username) {
				subtitle = user.DisplayName + " · KovaaK's player"
			}
			out = append(out, &hubv1.QuickSearchResult{
				Kind: "kovaaks_player", Title: firstNonEmpty(user.Username, user.DisplayName), Subtitle: subtitle,
				ImageUrl: user.AvatarURL, Relevance: relevance(match, 0), SteamId: user.SteamID, Country: strings.ToLower(user.Country),
			})
		}
	}()
	go func() {
		defer wg.Done()
		page, err := s.benchmarks.SearchScenarios(ctx, query, 0, 8)
		if err != nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		for _, item := range page.Items {
			match := matchScore(query, item.ScenarioName)
			if match == 0 {
				match = 0.3
			}
			out = append(out, &hubv1.QuickSearchResult{
				Kind: "kovaaks_scenario", Title: item.ScenarioName, Subtitle: "KovaaK's scenario", Badge: item.AimType,
				Relevance: relevance(match, item.Entries) - 2, ScenarioName: item.ScenarioName,
				ScenarioSlug: store.ScenarioSlug(item.ScenarioName), Count: item.Entries,
			})
		}
	}()
	wg.Wait()
	return out
}

// mergeSearchResults adds external results that do not duplicate a local
// scenario (by slug) or an AimMod player.
func mergeSearchResults(local, external []*hubv1.QuickSearchResult) []*hubv1.QuickSearchResult {
	slugs := map[string]bool{}
	for _, item := range local {
		if item.Kind == "scenario" {
			slugs[item.ScenarioSlug] = true
		}
	}
	out := append([]*hubv1.QuickSearchResult(nil), local...)
	for _, item := range external {
		if item.Kind == "kovaaks_scenario" && slugs[item.ScenarioSlug] {
			continue
		}
		out = append(out, item)
	}
	return out
}

// ── Player scenario stats and comparison ────────────────────────────────────

func (s *HubServer) GetPlayerScenarioStats(
	ctx context.Context,
	req *connect.Request[hubv1.GetPlayerScenarioStatsRequest],
) (*connect.Response[hubv1.GetPlayerScenarioStatsResponse], error) {
	handle := strings.TrimSpace(req.Msg.GetHandle())
	if handle == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("handle is required"))
	}
	since, rangeName := rangeSince(req.Msg.GetRange(), time.Now())
	resolved, records, err := s.store.PlayerScenarioStats(ctx, handle, since)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	resp := &hubv1.GetPlayerScenarioStatsResponse{UserHandle: resolved, Range: rangeName}
	for _, record := range records {
		recent := append([]float64(nil), record.RecentScores...)
		oldestFirst := make([]float64, len(recent))
		for i := range recent {
			oldestFirst[len(recent)-1-i] = recent[i]
		}
		resp.Scenarios = append(resp.Scenarios, &hubv1.PlayerScenarioStat{
			ScenarioName:     record.ScenarioName,
			ScenarioSlug:     store.ScenarioSlug(record.ScenarioName),
			ScenarioType:     record.ScenarioType,
			RunCount:         record.RunCount,
			BestScore:        record.BestScore,
			AverageScore:     record.AverageScore,
			BestAccuracy:     record.BestAccuracy,
			BestPlayedAtIso:  isoTime(record.BestPlayedAt),
			LastPlayedAtIso:  isoTime(record.LastPlayedAt),
			Consistency:      consistencyScore(recent),
			TrendPct:         trendPercent(recent),
			Percentile:       beatsShare(record.HubBelow, record.HubPlayers),
			HubRank:          record.HubRank,
			HubPlayers:       record.HubPlayers,
			RecentScores:     oldestFirst,
			TotalDurationMs:  record.TotalDurationMS,
		})
	}
	if days, err := s.store.PlayerActivity(ctx, resolved, time.Now().AddDate(0, 0, -182)); err == nil {
		for _, day := range days {
			resp.Activity = append(resp.Activity, &hubv1.ActivityDay{
				DateIso: day.Date.Format("2006-01-02"), RunCount: day.RunCount, DurationMs: day.DurationMS,
			})
		}
	}
	return connect.NewResponse(resp), nil
}

func (s *HubServer) ComparePlayers(
	ctx context.Context,
	req *connect.Request[hubv1.ComparePlayersRequest],
) (*connect.Response[hubv1.ComparePlayersResponse], error) {
	a := strings.TrimSpace(req.Msg.GetHandle())
	b := strings.TrimSpace(req.Msg.GetOtherHandle())
	if a == "" || b == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("two handles are required"))
	}
	left, err := s.store.ComparePlayerBests(ctx, a)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	right, err := s.store.ComparePlayerBests(ctx, b)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(compareBests(left, right)), nil
}

func compareBests(left, right *store.ComparePlayerRecord) *hubv1.ComparePlayersResponse {
	resp := &hubv1.ComparePlayersResponse{
		Player: &hubv1.ComparePlayer{UserHandle: left.UserHandle, DisplayName: left.DisplayName, AvatarUrl: left.AvatarURL, ScenarioCount: uint32(len(left.Bests)), RunCount: left.RunCount},
		Other:  &hubv1.ComparePlayer{UserHandle: right.UserHandle, DisplayName: right.DisplayName, AvatarUrl: right.AvatarURL, ScenarioCount: uint32(len(right.Bests)), RunCount: right.RunCount},
	}
	for name, mine := range left.Bests {
		theirs, ok := right.Bests[name]
		if !ok {
			continue
		}
		resp.Shared = append(resp.Shared, &hubv1.CompareScenario{
			ScenarioName: name, ScenarioSlug: store.ScenarioSlug(name), ScenarioType: mine.ScenarioType,
			Score: mine.BestScore, OtherScore: theirs.BestScore, RunCount: mine.RunCount, OtherRunCount: theirs.RunCount,
		})
		switch {
		case mine.BestScore > theirs.BestScore:
			resp.Wins++
		case mine.BestScore < theirs.BestScore:
			resp.Losses++
		default:
			resp.Ties++
		}
	}
	sort.Slice(resp.Shared, func(i, j int) bool {
		ri := resp.Shared[i].RunCount + resp.Shared[i].OtherRunCount
		rj := resp.Shared[j].RunCount + resp.Shared[j].OtherRunCount
		if ri != rj {
			return ri > rj
		}
		return resp.Shared[i].ScenarioName < resp.Shared[j].ScenarioName
	})
	return resp
}

// ── KovaaK's players who are not on AimMod ──────────────────────────────────

func (s *HubServer) GetKovaaksPlayer(
	ctx context.Context,
	req *connect.Request[hubv1.GetKovaaksPlayerRequest],
) (*connect.Response[hubv1.GetKovaaksPlayerResponse], error) {
	query := strings.TrimSpace(req.Msg.GetQuery())
	if query == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("query is required"))
	}
	steamID, username := s.resolveKovaaksPlayer(ctx, query)
	if username == "" {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no KovaaK's player matches that"))
	}
	resp := &hubv1.GetKovaaksPlayerResponse{SteamId: steamID, KovaaksUsername: username}
	if profile, err := s.benchmarks.GetUserProfile(ctx, username); err == nil && profile != nil {
		if steamID != "" && profile.SteamID != steamID {
			// The username belongs to someone else; keep the Steam identity.
			profile = nil
		} else {
			resp.SteamId = profile.SteamID
			resp.SteamName = profile.SteamName
			resp.AvatarUrl = profile.AvatarURL
			resp.Country = profile.Country
			resp.ScenariosPlayed = profile.ScenariosPlayed
			resp.CreatedAtIso = isoTime(profile.CreatedAt)
			resp.LastAccessAtIso = isoTime(profile.LastAccessAt)
		}
	}
	if resp.SteamId == "" {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no KovaaK's player matches that"))
	}
	_ = s.store.UpsertKovaaksUsers(ctx, []store.KovaaksUserCacheEntry{{
		SteamID: resp.SteamId, Username: resp.KovaaksUsername, DisplayName: resp.SteamName, AvatarURL: resp.AvatarUrl, Country: resp.Country,
	}})
	if linked, err := s.store.GetBenchmarkIdentityBySteamId(ctx, resp.SteamId); err == nil && linked != nil {
		resp.AimmodHandle = linked.UserHandle
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		items, err := s.benchmarks.ListPlayerBenchmarks(ctx, resp.KovaaksUsername)
		if err != nil {
			return
		}
		for _, item := range items {
			resp.Benchmarks = append(resp.Benchmarks, benchmarkSummary(item))
		}
	}()
	go func() {
		defer wg.Done()
		page, err := s.benchmarks.ListUserScenarios(ctx, resp.KovaaksUsername, 0, 50)
		if err != nil {
			return
		}
		resp.ScenarioTotal = page.Total
		for _, item := range page.Items {
			resp.Scenarios = append(resp.Scenarios, &hubv1.KovaaksPlayerScenario{
				ScenarioName: item.ScenarioName, ScenarioSlug: store.ScenarioSlug(item.ScenarioName), LeaderboardId: item.LeaderboardID,
				Plays: item.Plays, Rank: item.Rank, Score: item.Score, Cm360: item.CM360, PlayedAtIso: isoTime(item.PlayedAt),
			})
		}
	}()
	wg.Wait()
	return connect.NewResponse(resp), nil
}

// resolveKovaaksPlayer turns a Steam id, Steam URL, vanity or KovaaK's
// username into a Steam id and KovaaK's username.
func (s *HubServer) resolveKovaaksPlayer(ctx context.Context, query string) (string, string) {
	if looksLikeSteam64(query) || strings.Contains(query, "steamcommunity.com") {
		resolved, _ := s.benchmarks.ResolveSteamInput(ctx, query)
		steamID := resolved.Steam64
		if steamID == "" {
			return "", ""
		}
		if cached, err := s.store.GetKovaaksUserBySteamId(ctx, steamID); err == nil && cached != nil && cached.Username != "" {
			return steamID, cached.Username
		}
		// The Steam persona name is only a guess at the KovaaK's username;
		// confirm it through KovaaK's search by matching the Steam id.
		for _, candidate := range []string{resolved.KovaaksUsername} {
			if candidate == "" {
				continue
			}
			if users, err := s.benchmarks.SearchUsers(ctx, candidate, 10); err == nil {
				for _, user := range users {
					if user.SteamID == steamID && user.Username != "" {
						return steamID, user.Username
					}
				}
			}
		}
		return steamID, resolved.KovaaksUsername
	}
	if profile, err := s.benchmarks.GetUserProfile(ctx, query); err == nil && profile != nil {
		return profile.SteamID, profile.KovaaksUsername
	}
	return "", ""
}
