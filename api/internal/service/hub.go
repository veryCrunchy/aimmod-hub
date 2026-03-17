package service

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/veryCrunchy/aimmod-hub/api/internal/kovaaksbenchmarks"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
	hubv1connect "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1/hubv1connect"
)

type HubServer struct {
	version    string
	store      *store.Store
	benchmarks *kovaaksbenchmarks.Client
}

const optionalBenchmarkTimeout = 1500 * time.Millisecond

func NewHubServer(version string, store *store.Store) *HubServer {
	return &HubServer{
		version:    version,
		store:      store,
		benchmarks: kovaaksbenchmarks.NewClient(),
	}
}

func (s *HubServer) Store() *store.Store {
	return s.store
}

func (s *HubServer) Benchmarks() *kovaaksbenchmarks.Client {
	return s.benchmarks
}

func (s *HubServer) GetHealth(
	context.Context,
	*connect.Request[hubv1.HealthRequest],
) (*connect.Response[hubv1.HealthResponse], error) {
	return connect.NewResponse(&hubv1.HealthResponse{
		Service: "aimmod-hub",
		Version: s.version,
		Status:  "ok",
		NowIso:  time.Now().UTC().Format(time.RFC3339),
	}), nil
}

func (s *HubServer) IngestAuthorized(
	ctx context.Context,
	authHeader string,
	req *hubv1.IngestSessionRequest,
) (*hubv1.IngestSessionResponse, error) {
	authHeader = strings.TrimSpace(authHeader)
	var authUser *store.AuthUser
	if authHeader != "" {
		resolvedUser, err := s.store.GetUserByUploadToken(ctx, authHeader)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
		req.UserExternalId = resolvedUser.UserExternalID
		authUser = &resolvedUser
	}

	run, err := buildIngestedRun(req)
	if err != nil {
		s.recordIngestFailure(ctx, req, err)
		return nil, err
	}

	if err := s.store.SaveIngestedRun(ctx, run, authUser); err != nil {
		s.recordIngestFailure(ctx, req, err)
		if strings.Contains(err.Error(), "already verified") ||
			strings.Contains(err.Error(), "identity conflict") ||
			strings.Contains(err.Error(), "belongs to a verified profile") {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &hubv1.IngestSessionResponse{
		Accepted:  true,
		SessionId: run.SessionID,
		Message:   "Ingest accepted and stored.",
	}, nil
}

func (s *HubServer) IngestSession(
	ctx context.Context,
	req *connect.Request[hubv1.IngestSessionRequest],
) (*connect.Response[hubv1.IngestSessionResponse], error) {
	resp, err := s.IngestAuthorized(ctx, req.Header().Get("Authorization"), req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *HubServer) LinkDiscordAccount(
	ctx context.Context,
	req *connect.Request[hubv1.LinkDiscordAccountRequest],
) (*connect.Response[hubv1.LinkDiscordAccountResponse], error) {
	link, err := buildDiscordLink(req.Msg)
	if err != nil {
		return nil, err
	}

	if err := s.store.LinkDiscordAccount(ctx, link); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&hubv1.LinkDiscordAccountResponse{
		Linked:         true,
		UserExternalId: link.UserExternalID,
		DiscordUserId:  link.DiscordUserID,
		Message:        "Discord account linked.",
	}), nil
}

func (s *HubServer) recordIngestFailure(ctx context.Context, req *hubv1.IngestSessionRequest, err error) {
	if req == nil || err == nil {
		return
	}
	_ = s.store.RecordIngestFailure(ctx, store.IngestFailureRecord{
		UserExternalID: req.GetUserExternalId(),
		SessionID:      req.GetSessionId(),
		ScenarioName:   req.GetScenarioName(),
		ErrorMessage:   err.Error(),
	})
}

func (s *HubServer) GetOverview(
	ctx context.Context,
	_ *connect.Request[hubv1.GetOverviewRequest],
) (*connect.Response[hubv1.GetOverviewResponse], error) {
	overview, err := s.store.GetOverview(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&hubv1.GetOverviewResponse{
		TotalRuns:      overview.TotalRuns,
		TotalScenarios: overview.TotalScenarios,
		TotalPlayers:   overview.TotalPlayers,
		RecentRuns:     overview.RecentRuns,
		TopScenarios:   overview.TopScenarios,
		ActiveProfiles: overview.ActiveProfiles,
	}), nil
}

func (s *HubServer) GetRun(
	ctx context.Context,
	req *connect.Request[hubv1.GetRunRequest],
) (*connect.Response[hubv1.GetRunResponse], error) {
	runID := strings.TrimSpace(req.Msg.GetRunId())
	if runID == "" {
		runID = strings.TrimSpace(req.Msg.GetSessionId())
	}
	if runID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("run_id is required"))
	}

	run, err := s.store.GetRun(ctx, runID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	benchmarkCtx, cancel := context.WithTimeout(ctx, optionalBenchmarkTimeout)
	defer cancel()
	benchmarkRanks, err := s.fetchScenarioBenchmarkRanks(benchmarkCtx, run.UserHandle, run.ScenarioName, nil)
	if err != nil {
		benchmarkRanks = nil
	}

	return connect.NewResponse(&hubv1.GetRunResponse{
		SessionId:       run.SessionID,
		ScenarioName:    run.ScenarioName,
		ScenarioType:    run.ScenarioType,
		PlayedAtIso:     run.PlayedAt.UTC().Format(time.RFC3339),
		Score:           run.Score,
		Accuracy:        run.Accuracy,
		DurationMs:      run.DurationMS,
		UserHandle:      run.UserHandle,
		UserDisplayName: run.UserDisplayName,
		Summary:         run.Summary,
		FeatureSet:      run.FeatureSet,
		TimelineSeconds: run.Timeline,
		ContextWindows:  run.ContextWindows,
		RunId:           run.PublicRunID,
		ScenarioRuns:    run.ScenarioRuns,
		BenchmarkRanks:  benchmarkRanks,
	}), nil
}

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

	return connect.NewResponse(&hubv1.GetScenarioPageResponse{
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
	}), nil
}

func (s *HubServer) GetProfile(
	ctx context.Context,
	req *connect.Request[hubv1.GetProfileRequest],
) (*connect.Response[hubv1.GetProfileResponse], error) {
	handle := strings.TrimSpace(req.Msg.GetHandle())
	if handle == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("handle is required"))
	}

	profile, err := s.store.GetProfile(ctx, handle)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	benchmarkCtx, cancel := context.WithTimeout(ctx, optionalBenchmarkTimeout)
	defer cancel()
	benchmarks, _, err := s.fetchProfileBenchmarks(benchmarkCtx, handle)
	if err != nil {
		benchmarks = nil
	}

	return connect.NewResponse(&hubv1.GetProfileResponse{
		UserExternalId:      profile.UserExternalID,
		UserHandle:          profile.UserHandle,
		UserDisplayName:     profile.UserDisplayName,
		AvatarUrl:           profile.AvatarURL,
		IsVerified:          profile.IsVerified,
		RunCount:            profile.RunCount,
		ScenarioCount:       profile.ScenarioCount,
		PrimaryScenarioType: profile.PrimaryScenarioType,
		AverageScore:        profile.AverageScore,
		AverageAccuracy:     profile.AverageAccuracy,
		TopScenarios:        profile.TopScenarios,
		RecentRuns:          profile.RecentRuns,
		PersonalBests:       profile.PersonalBests,
		Benchmarks:          benchmarks,
	}), nil
}

func searchScenarioResult(record store.SearchScenarioRecord) *hubv1.SearchScenarioResult {
	return &hubv1.SearchScenarioResult{
		ScenarioName: record.ScenarioName,
		ScenarioSlug: record.ScenarioSlug,
		ScenarioType: record.ScenarioType,
		RunCount:     record.RunCount,
	}
}

func searchProfileResult(record store.SearchProfileRecord) *hubv1.SearchProfileResult {
	return &hubv1.SearchProfileResult{
		UserHandle:          record.UserHandle,
		UserDisplayName:     record.UserDisplayName,
		AvatarUrl:           record.AvatarURL,
		IsVerified:          record.IsVerified,
		RunCount:            record.RunCount,
		ScenarioCount:       record.ScenarioCount,
		PrimaryScenarioType: record.PrimaryScenarioType,
	}
}

func replayPreview(record store.SearchRunRecord) *hubv1.ReplayPreview {
	return &hubv1.ReplayPreview{
		PublicRunId:     record.PublicRunID,
		SessionId:       record.SessionID,
		ScenarioSlug:    record.ScenarioSlug,
		ScenarioName:    record.ScenarioName,
		ScenarioType:    record.ScenarioType,
		PlayedAtIso:     record.PlayedAt.UTC().Format(time.RFC3339),
		Score:           record.Score,
		Accuracy:        record.Accuracy,
		DurationMs:      record.DurationMS,
		UserHandle:      record.UserHandle,
		UserDisplayName: record.UserDisplayName,
		HasVideo:        record.HasVideo,
		HasMousePath:    record.HasMousePath,
		ReplayQuality:   record.ReplayQuality,
	}
}

func slugifyScenarioName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			out.WriteRune(r)
			lastDash = false
		default:
			if out.Len() > 0 && !lastDash {
				out.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(out.String(), "-")
}

func benchmarkRankVisual(rank kovaaksbenchmarks.BenchmarkRankVisual) *hubv1.BenchmarkRankVisual {
	return &hubv1.BenchmarkRankVisual{
		RankIndex: rank.RankIndex,
		RankName:  rank.RankName,
		IconUrl:   rank.IconURL,
		Color:     rank.Color,
		FrameUrl:  rank.FrameURL,
	}
}

func benchmarkSummary(summary kovaaksbenchmarks.ProfileBenchmarkSummary) *hubv1.BenchmarkSummary {
	return &hubv1.BenchmarkSummary{
		BenchmarkId:      summary.BenchmarkID,
		BenchmarkName:    summary.BenchmarkName,
		BenchmarkIconUrl: summary.BenchmarkIconURL,
		BenchmarkAuthor:  summary.BenchmarkAuthor,
		BenchmarkType:    summary.BenchmarkType,
		OverallRank: &hubv1.BenchmarkRankVisual{
			RankName: summary.OverallRankName,
			IconUrl:  summary.OverallRankIcon,
			Color:    summary.OverallRankColor,
		},
	}
}

func scenarioBenchmarkRank(rank kovaaksbenchmarks.ScenarioBenchmarkRank) *hubv1.ScenarioBenchmarkRank {
	return &hubv1.ScenarioBenchmarkRank{
		BenchmarkId:      rank.BenchmarkID,
		BenchmarkName:    rank.BenchmarkName,
		BenchmarkIconUrl: rank.BenchmarkIconURL,
		CategoryName:     rank.CategoryName,
		ScenarioScore:    rank.ScenarioScore,
		LeaderboardRank:  rank.LeaderboardRank,
		LeaderboardId:    rank.LeaderboardID,
		ScenarioRank:     benchmarkRankVisual(rank.ScenarioRank),
	}
}

func benchmarkThreshold(threshold kovaaksbenchmarks.BenchmarkThreshold) *hubv1.BenchmarkThreshold {
	return &hubv1.BenchmarkThreshold{
		RankIndex: threshold.RankIndex,
		RankName:  threshold.RankName,
		IconUrl:   threshold.IconURL,
		Color:     threshold.Color,
		Score:     threshold.Score,
	}
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
	}
}

func (s *HubServer) fetchProfileBenchmarks(ctx context.Context, handle string) ([]*hubv1.BenchmarkSummary, []kovaaksbenchmarks.ProfileBenchmarkSummary, error) {
	identity, err := s.store.GetBenchmarkIdentityByHandle(ctx, handle)
	if err != nil {
		return nil, nil, err
	}
	items, err := s.benchmarks.ListPlayerBenchmarks(ctx, identity.KovaaksUsername)
	if err != nil {
		return nil, nil, err
	}
	out := make([]*hubv1.BenchmarkSummary, 0, len(items))
	for _, item := range items {
		out = append(out, benchmarkSummary(item))
	}
	return out, items, nil
}

func (s *HubServer) fetchScenarioBenchmarkRanks(
	ctx context.Context,
	handle string,
	scenarioName string,
	preloaded []kovaaksbenchmarks.ProfileBenchmarkSummary,
) ([]*hubv1.ScenarioBenchmarkRank, error) {
	identity, err := s.store.GetBenchmarkIdentityByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	items := preloaded
	if len(items) == 0 {
		items, err = s.benchmarks.ListPlayerBenchmarks(ctx, identity.KovaaksUsername)
		if err != nil {
			return nil, err
		}
	}
	ranks, err := s.benchmarks.ListScenarioRanks(ctx, identity.SteamID, scenarioName, items)
	if err != nil {
		return nil, err
	}
	out := make([]*hubv1.ScenarioBenchmarkRank, 0, len(ranks))
	for _, rank := range ranks {
		out = append(out, scenarioBenchmarkRank(rank))
	}
	return out, nil
}

func (s *HubServer) GetBenchmarkPage(
	ctx context.Context,
	req *connect.Request[hubv1.GetBenchmarkPageRequest],
) (*connect.Response[hubv1.GetBenchmarkPageResponse], error) {
	handle := strings.TrimSpace(req.Msg.GetHandle())
	if handle == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("handle is required"))
	}
	benchmarkID := req.Msg.GetBenchmarkId()
	if benchmarkID == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("benchmark_id is required"))
	}

	profile, err := s.store.GetProfile(ctx, handle)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	identity, err := s.store.GetBenchmarkIdentityByHandle(ctx, handle)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	_, preloaded, err := s.fetchProfileBenchmarks(ctx, handle)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var summary *kovaaksbenchmarks.ProfileBenchmarkSummary
	for i := range preloaded {
		if preloaded[i].BenchmarkID == benchmarkID {
			summary = &preloaded[i]
			break
		}
	}
	if summary == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("benchmark not found for this player"))
	}

	detail, categories, err := s.benchmarks.BuildFullBenchmarkPage(ctx, *summary, identity.SteamID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if detail == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("benchmark detail not found"))
	}

	outCategories := make([]*hubv1.BenchmarkCategoryPage, 0, len(categories))
	for _, category := range categories {
		scenarios := make([]*hubv1.BenchmarkScenarioEntry, 0, len(category.Scenarios))
		for _, scenario := range category.Scenarios {
			scenarios = append(scenarios, benchmarkScenarioEntry(scenario))
		}
		outCategories = append(outCategories, &hubv1.BenchmarkCategoryPage{
			CategoryName: category.CategoryName,
			CategoryRank: category.CategoryRank,
			Scenarios:    scenarios,
		})
	}

	return connect.NewResponse(&hubv1.GetBenchmarkPageResponse{
		UserHandle:       profile.UserHandle,
		UserDisplayName:  profile.UserDisplayName,
		BenchmarkId:      summary.BenchmarkID,
		BenchmarkName:    summary.BenchmarkName,
		BenchmarkIconUrl: summary.BenchmarkIconURL,
		BenchmarkAuthor:  summary.BenchmarkAuthor,
		BenchmarkType:    summary.BenchmarkType,
		OverallRank: benchmarkRankVisual(kovaaksbenchmarks.BenchmarkRankVisual{
			RankName: summary.OverallRankName,
			IconURL:  summary.OverallRankIcon,
			Color:    summary.OverallRankColor,
		}),
		Categories: outCategories,
	}), nil
}

func (s *HubServer) buildBenchmarkList(ctx context.Context) ([]*hubv1.BenchmarkListItem, error) {
	users, err := s.store.ListUsersWithBenchmarkIdentity(ctx)
	if err != nil {
		return nil, err
	}

	// Phase 1: collect the set of benchmarks each user participates in via the
	// cheap list endpoint (one call per user, paginated).
	type listResult struct {
		steamID string
		items   []kovaaksbenchmarks.ProfileBenchmarkSummary
	}
	listCh := make(chan listResult, len(users))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, u := range users {
		wg.Add(1)
		go func(u store.BenchmarkUserIdentity) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			items, _ := s.benchmarks.ListPlayerBenchmarks(ctx, u.KovaaksUsername)
			listCh <- listResult{steamID: u.SteamID, items: items}
		}(u)
	}
	go func() { wg.Wait(); close(listCh) }()

	type aggEntry struct {
		summary  kovaaksbenchmarks.ProfileBenchmarkSummary
		steamIDs []string
	}
	byID := map[uint32]*aggEntry{}
	for r := range listCh {
		for _, b := range r.items {
			if b.BenchmarkID == 0 {
				continue
			}
			if e, ok := byID[b.BenchmarkID]; ok {
				e.steamIDs = append(e.steamIDs, r.steamID)
			} else {
				byID[b.BenchmarkID] = &aggEntry{summary: b, steamIDs: []string{r.steamID}}
			}
		}
	}

	// Phase 2: verify each (benchmarkID, steamID) pair via GetBenchmarkDetail so
	// that the displayed count matches exactly what the leaderboard will show.
	type verifyWork struct {
		benchmarkID uint32
		steamID     string
	}
	type verifyResult struct {
		benchmarkID uint32
		counted     bool
	}
	var pairs []verifyWork
	for id, e := range byID {
		for _, sid := range e.steamIDs {
			pairs = append(pairs, verifyWork{benchmarkID: id, steamID: sid})
		}
	}
	verifyCh := make(chan verifyResult, len(pairs))
	var wg2 sync.WaitGroup
	for _, p := range pairs {
		wg2.Add(1)
		go func(p verifyWork) {
			defer wg2.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			detail, err := s.benchmarks.GetBenchmarkDetail(ctx, p.benchmarkID, p.steamID)
			if err != nil || detail == nil || detail.OverallRank == 0 {
				verifyCh <- verifyResult{benchmarkID: p.benchmarkID, counted: false}
				return
			}
			rv := kovaaksbenchmarks.RankVisualFromDetail(detail, detail.OverallRank)
			rankName := strings.TrimSpace(rv.RankName)
			verifyCh <- verifyResult{
				benchmarkID: p.benchmarkID,
				counted:     rankName != "" && !strings.EqualFold(rankName, "No Rank"),
			}
		}(p)
	}
	go func() { wg2.Wait(); close(verifyCh) }()

	counts := map[uint32]uint32{}
	for r := range verifyCh {
		if r.counted {
			counts[r.benchmarkID]++
		}
	}

	out := make([]*hubv1.BenchmarkListItem, 0, len(byID))
	for id, e := range byID {
		c := counts[id]
		if c == 0 {
			continue // skip benchmarks with no verified ranked players
		}
		out = append(out, &hubv1.BenchmarkListItem{
			BenchmarkId:      e.summary.BenchmarkID,
			BenchmarkName:    e.summary.BenchmarkName,
			BenchmarkIconUrl: e.summary.BenchmarkIconURL,
			BenchmarkAuthor:  e.summary.BenchmarkAuthor,
			BenchmarkType:    e.summary.BenchmarkType,
			PlayerCount:      c,
		})
	}
	// Sort by player count desc, then name asc.
	sort.Slice(out, func(i, j int) bool {
		if out[i].PlayerCount != out[j].PlayerCount {
			return out[i].PlayerCount > out[j].PlayerCount
		}
		return out[i].BenchmarkName < out[j].BenchmarkName
	})
	return out, nil
}

func (s *HubServer) ListBenchmarks(
	ctx context.Context,
	_ *connect.Request[hubv1.ListBenchmarksRequest],
) (*connect.Response[hubv1.ListBenchmarksResponse], error) {
	out, err := s.buildBenchmarkList(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&hubv1.ListBenchmarksResponse{Benchmarks: out}), nil
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
		entry  *hubv1.BenchmarkLeaderboardEntry
		rankID uint32
	}
	results := make(chan fetchResult, len(users))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for _, u := range users {
		wg.Add(1)
		go func(u store.BenchmarkUserIdentity) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			detail, err := s.benchmarks.GetBenchmarkDetail(ctx, benchmarkID, u.SteamID)
			if err != nil || detail == nil || detail.OverallRank == 0 {
				results <- fetchResult{}
				return
			}
			rv := kovaaksbenchmarks.RankVisualFromDetail(detail, detail.OverallRank)
			results <- fetchResult{
				entry: &hubv1.BenchmarkLeaderboardEntry{
					UserHandle:         u.UserHandle,
					DisplayName:        u.DisplayName,
					AvatarUrl:          u.AvatarURL,
					OverallRankName:    rv.RankName,
					OverallRankIconUrl: rv.IconURL,
					OverallRankIndex:   detail.OverallRank,
				},
				rankID: detail.OverallRank,
			}
		}(u)
	}
	go func() { wg.Wait(); close(results) }()

	var entries []*hubv1.BenchmarkLeaderboardEntry
	var rankIDs []uint32
	for r := range results {
		if r.entry != nil {
			entries = append(entries, r.entry)
			rankIDs = append(rankIDs, r.rankID)
		}
	}

	// Sort by rank index descending (higher = better).
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0; j-- {
			if rankIDs[j] > rankIDs[j-1] {
				entries[j], entries[j-1] = entries[j-1], entries[j]
				rankIDs[j], rankIDs[j-1] = rankIDs[j-1], rankIDs[j]
			} else {
				break
			}
		}
	}

	// Look up benchmark name from any user's benchmark list.
	var benchmarkName, benchmarkIconURL string
	for _, u := range users {
		items, err := s.benchmarks.ListPlayerBenchmarks(ctx, u.KovaaksUsername)
		if err != nil {
			continue
		}
		for _, item := range items {
			if item.BenchmarkID == benchmarkID {
				benchmarkName = item.BenchmarkName
				benchmarkIconURL = item.BenchmarkIconURL
				break
			}
		}
		if benchmarkName != "" {
			break
		}
	}

	return connect.NewResponse(&hubv1.GetBenchmarkLeaderboardResponse{
		BenchmarkId:      benchmarkID,
		BenchmarkName:    benchmarkName,
		BenchmarkIconUrl: benchmarkIconURL,
		Entries:          entries,
	}), nil
}

func (s *HubServer) Search(
	ctx context.Context,
	req *connect.Request[hubv1.SearchRequest],
) (*connect.Response[hubv1.SearchResponse], error) {
	query := strings.TrimSpace(req.Msg.GetQuery())
	results, err := s.store.Search(ctx, query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &hubv1.SearchResponse{
		Query:      results.Query,
		Scenarios:  make([]*hubv1.SearchScenarioResult, 0, len(results.Scenarios)),
		Profiles:   make([]*hubv1.SearchProfileResult, 0, len(results.Profiles)),
		Runs:       make([]*hubv1.ReplayPreview, 0, len(results.Runs)),
		Replays:    make([]*hubv1.ReplayPreview, 0, len(results.Replays)),
		Benchmarks: make([]*hubv1.BenchmarkListItem, 0),
	}
	for _, record := range results.Scenarios {
		resp.Scenarios = append(resp.Scenarios, searchScenarioResult(record))
	}
	for _, record := range results.Profiles {
		resp.Profiles = append(resp.Profiles, searchProfileResult(record))
	}
	for _, record := range results.Runs {
		resp.Runs = append(resp.Runs, replayPreview(record))
	}
	for _, record := range results.Replays {
		resp.Replays = append(resp.Replays, replayPreview(record))
	}

	if query != "" {
		lower := strings.ToLower(query)
		allBenchmarks, _ := s.buildBenchmarkList(ctx)
		for _, b := range allBenchmarks {
			if strings.Contains(strings.ToLower(b.BenchmarkName), lower) ||
				strings.Contains(strings.ToLower(b.BenchmarkAuthor), lower) ||
				strings.Contains(strings.ToLower(b.BenchmarkType), lower) {
				resp.Benchmarks = append(resp.Benchmarks, b)
				if len(resp.Benchmarks) >= 6 {
					break
				}
			}
		}
	}

	return connect.NewResponse(resp), nil
}

func (s *HubServer) ListReplays(
	ctx context.Context,
	req *connect.Request[hubv1.ListReplaysRequest],
) (*connect.Response[hubv1.ListReplaysResponse], error) {
	limit := int(req.Msg.GetLimit())
	if limit <= 0 {
		limit = 50
	}
	results, err := s.store.ListReplays(
		ctx,
		strings.TrimSpace(req.Msg.GetQuery()),
		strings.TrimSpace(req.Msg.GetScenarioName()),
		strings.TrimSpace(req.Msg.GetHandle()),
		limit,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &hubv1.ListReplaysResponse{
		Query:        results.Query,
		ScenarioName: results.ScenarioName,
		UserHandle:   results.UserHandle,
		Items:        make([]*hubv1.ReplayPreview, 0, len(results.Items)),
	}
	for _, record := range results.Items {
		resp.Items = append(resp.Items, replayPreview(record))
	}
	return connect.NewResponse(resp), nil
}

func (s *HubServer) GetReplayMedia(
	ctx context.Context,
	req *connect.Request[hubv1.GetReplayMediaRequest],
) (*connect.Response[hubv1.GetReplayMediaResponse], error) {
	runID := strings.TrimSpace(req.Msg.GetRunId())
	if runID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("run_id is required"))
	}

	meta, err := s.store.GetReplayMediaMeta(ctx, runID)
	if err != nil {
		return connect.NewResponse(&hubv1.GetReplayMediaResponse{
			Available: false,
		}), nil
	}

	return connect.NewResponse(&hubv1.GetReplayMediaResponse{
		Available:   true,
		RunId:       meta.PublicRunID,
		Quality:     meta.Quality,
		ContentType: meta.ContentType,
		ByteSize:    uint64(meta.ByteSize),
		MediaUrl:    replayMediaPath(meta.PublicRunID, meta.Quality),
	}), nil
}

func (s *HubServer) GetMousePath(
	ctx context.Context,
	req *connect.Request[hubv1.GetMousePathRequest],
) (*connect.Response[hubv1.GetMousePathResponse], error) {
	runID := strings.TrimSpace(req.Msg.GetRunId())
	if runID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("run_id is required"))
	}

	mousePath, err := s.store.GetMousePath(ctx, runID)
	if err != nil {
		return connect.NewResponse(&hubv1.GetMousePathResponse{
			Available:        false,
			Points:           []*hubv1.MousePathPoint{},
			HitTimestampsMs:  []uint64{},
			PlaybackOffsetMs: 0,
		}), nil
	}

	points := make([]*hubv1.MousePathPoint, 0, len(mousePath.Points))
	for _, point := range mousePath.Points {
		points = append(points, &hubv1.MousePathPoint{
			X:           point.X,
			Y:           point.Y,
			TimestampMs: point.Timestamp,
			IsClick:     point.IsClick,
		})
	}

	firstPointMS := uint64(0)
	lastPointMS := uint64(0)
	if len(mousePath.Points) > 0 {
		firstPointMS = mousePath.Points[0].Timestamp
		lastPointMS = mousePath.Points[len(mousePath.Points)-1].Timestamp
	}
	firstHitMS := uint64(0)
	lastHitMS := uint64(0)
	if len(mousePath.HitTimestampsMS) > 0 {
		firstHitMS = mousePath.HitTimestampsMS[0]
		lastHitMS = mousePath.HitTimestampsMS[len(mousePath.HitTimestampsMS)-1]
	}
	log.Printf(
		"service_get_mouse_path: run=%q available=%t points=%d hits=%d offset_ms=%d video_offset_ms=%d first_point_ms=%d last_point_ms=%d first_hit_ms=%d last_hit_ms=%d",
		runID,
		true,
		len(mousePath.Points),
		len(mousePath.HitTimestampsMS),
		mousePath.PlaybackOffsetMS,
		mousePath.VideoOffsetMS,
		firstPointMS,
		lastPointMS,
		firstHitMS,
		lastHitMS,
	)

	return connect.NewResponse(&hubv1.GetMousePathResponse{
		Available:        true,
		Points:           points,
		HitTimestampsMs:  mousePath.HitTimestampsMS,
		PlaybackOffsetMs: mousePath.PlaybackOffsetMS,
		VideoOffsetMs:    mousePath.VideoOffsetMS,
	}), nil
}

func (h *HubServer) GetLeaderboard(
	ctx context.Context,
	req *connect.Request[hubv1.GetLeaderboardRequest],
) (*connect.Response[hubv1.GetLeaderboardResponse], error) {
	board, err := h.store.GetLeaderboard(ctx, req.Msg.GetScenarioType())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&hubv1.GetLeaderboardResponse{
		Records:   board.Records,
		TopScores: board.TopScores,
	}), nil
}

func (h *HubServer) GetPlayerScenarioHistory(
	ctx context.Context,
	req *connect.Request[hubv1.GetPlayerScenarioHistoryRequest],
) (*connect.Response[hubv1.GetPlayerScenarioHistoryResponse], error) {
	history, err := h.store.GetPlayerScenarioHistory(ctx, req.Msg.GetHandle(), req.Msg.GetScenarioSlug())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	benchmarkCtx, cancel := context.WithTimeout(ctx, optionalBenchmarkTimeout)
	defer cancel()
	benchmarkRanks, err := h.fetchScenarioBenchmarkRanks(benchmarkCtx, req.Msg.GetHandle(), history.ScenarioName, nil)
	if err != nil {
		benchmarkRanks = nil
	}
	return connect.NewResponse(&hubv1.GetPlayerScenarioHistoryResponse{
		ScenarioName:    history.ScenarioName,
		ScenarioSlug:    history.ScenarioSlug,
		ScenarioType:    history.ScenarioType,
		Runs:            history.Runs,
		BestScore:       history.BestScore,
		AverageScore:    history.AverageScore,
		BestAccuracy:    history.BestAccuracy,
		AverageAccuracy: history.AverageAccuracy,
		RunCount:        history.RunCount,
		BenchmarkRanks:  benchmarkRanks,
	}), nil
}

func (h *HubServer) GetAimProfile(
	ctx context.Context,
	req *connect.Request[hubv1.GetAimProfileRequest],
) (*connect.Response[hubv1.GetAimProfileResponse], error) {
	profile, err := h.store.GetAimProfile(ctx, req.Msg.GetHandle())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&hubv1.GetAimProfileResponse{
		UserHandle:                profile.UserHandle,
		UserDisplayName:           profile.UserDisplayName,
		TypeBands:                 profile.TypeBands,
		OverallAccuracy:           profile.OverallAccuracy,
		OverallAccuracyPercentile: profile.OverallPercentile,
		TotalRunCount:             profile.TotalRunCount,
		StrongestType:             profile.StrongestType,
		MostPracticedType:         profile.MostPracticedType,
	}), nil
}

func (h *HubServer) GetAimFingerprint(
	ctx context.Context,
	req *connect.Request[hubv1.GetAimFingerprintRequest],
) (*connect.Response[hubv1.GetAimFingerprintResponse], error) {
	fp, err := h.store.GetAimFingerprint(ctx, req.Msg.GetHandle())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&hubv1.GetAimFingerprintResponse{Overall: fp}), nil
}

func replayMediaPath(publicRunID, quality string) string {
	trimmedRunID := strings.TrimSpace(publicRunID)
	if trimmedRunID == "" {
		return ""
	}
	trimmedQuality := strings.TrimSpace(quality)
	if trimmedQuality == "" {
		trimmedQuality = "standard"
	}
	return "/media/replays/" + trimmedRunID + ".mp4?quality=" + trimmedQuality
}

var _ hubv1connect.HubServiceHandler = (*HubServer)(nil)
