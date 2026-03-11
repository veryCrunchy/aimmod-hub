package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
	hubv1connect "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1/hubv1connect"
)

type HubServer struct {
	version string
	store   *store.Store
}

func NewHubServer(version string, store *store.Store) *HubServer {
	return &HubServer{version: version, store: store}
}

func (s *HubServer) Store() *store.Store {
	return s.store
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
	if authHeader = strings.TrimSpace(authHeader); authHeader != "" {
		authUser, err := s.store.GetUserByUploadToken(ctx, authHeader)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
		if req.GetUserExternalId() != "" && req.GetUserExternalId() != authUser.UserExternalID {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("upload token does not match user_external_id"))
		}
		req.UserExternalId = authUser.UserExternalID
	}

	run, err := buildIngestedRun(req)
	if err != nil {
		s.recordIngestFailure(ctx, req, err)
		return nil, err
	}

	if err := s.store.SaveIngestedRun(ctx, run); err != nil {
		s.recordIngestFailure(ctx, req, err)
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

	return connect.NewResponse(&hubv1.GetProfileResponse{
		UserExternalId:      profile.UserExternalID,
		UserHandle:          profile.UserHandle,
		UserDisplayName:     profile.UserDisplayName,
		AvatarUrl:           profile.AvatarURL,
		RunCount:            profile.RunCount,
		ScenarioCount:       profile.ScenarioCount,
		PrimaryScenarioType: profile.PrimaryScenarioType,
		AverageScore:        profile.AverageScore,
		AverageAccuracy:     profile.AverageAccuracy,
		TopScenarios:        profile.TopScenarios,
		RecentRuns:          profile.RecentRuns,
		PersonalBests:       profile.PersonalBests,
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

func (s *HubServer) Search(
	ctx context.Context,
	req *connect.Request[hubv1.SearchRequest],
) (*connect.Response[hubv1.SearchResponse], error) {
	results, err := s.store.Search(ctx, strings.TrimSpace(req.Msg.GetQuery()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &hubv1.SearchResponse{
		Query:    results.Query,
		Scenarios: make([]*hubv1.SearchScenarioResult, 0, len(results.Scenarios)),
		Profiles:  make([]*hubv1.SearchProfileResult, 0, len(results.Profiles)),
		Runs:      make([]*hubv1.ReplayPreview, 0, len(results.Runs)),
		Replays:   make([]*hubv1.ReplayPreview, 0, len(results.Replays)),
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
			Available:       false,
			Points:          []*hubv1.MousePathPoint{},
			HitTimestampsMs: []uint64{},
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

	return connect.NewResponse(&hubv1.GetMousePathResponse{
		Available:       true,
		Points:          points,
		HitTimestampsMs: mousePath.HitTimestampsMS,
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
