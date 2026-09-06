package osu

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

type Config struct {
	PlayerIndex               PlayerIndex
	OfficialBaseURL           string
	CollectorBaseURL          string
	OsuSkinsBaseURL           string
	OsuckBaseURL              string
	OfficialClientID          string
	OfficialClientSecret      string
	UserAgent                 string
	CacheTTL                  time.Duration
	CacheMaxEntries           int
	ProviderRequestsPerSecond float64
	RequestTimeout            time.Duration
	HTTPClient                *http.Client
}

type Server struct {
	official  *officialAdapter
	collector *collectorAdapter
	osuSkins  *osuSkinsAdapter
	osuck     *osuckAdapter
}

func NewServer(cfg Config) (*Server, error) {
	if strings.TrimSpace(cfg.OfficialBaseURL) == "" {
		cfg.OfficialBaseURL = "https://osu.ppy.sh"
	}
	if strings.TrimSpace(cfg.CollectorBaseURL) == "" {
		cfg.CollectorBaseURL = "https://osucollector.com"
	}
	if strings.TrimSpace(cfg.OsuSkinsBaseURL) == "" {
		cfg.OsuSkinsBaseURL = "https://osuskins.net"
	}
	if strings.TrimSpace(cfg.OsuckBaseURL) == "" {
		cfg.OsuckBaseURL = "https://skins.osuck.net"
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		cfg.UserAgent = "AimMod-Hub/dev (https://aimmod.app)"
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 10 * time.Second
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.RequestTimeout}
	}
	cache := newResponseCache(cfg.CacheTTL, cfg.CacheMaxEntries)
	officialLimiter := newIntervalLimiter(cfg.ProviderRequestsPerSecond)
	collectorLimiter := newIntervalLimiter(cfg.ProviderRequestsPerSecond)
	official, err := newOfficialAdapter(cfg, httpClient, cache, officialLimiter)
	if err != nil {
		return nil, err
	}
	collectorClient, err := newUpstreamClient(cfg.CollectorBaseURL, httpClient, cache, collectorLimiter, cfg.UserAgent)
	if err != nil {
		return nil, err
	}
	osuSkinsClient, err := newUpstreamClient(cfg.OsuSkinsBaseURL, httpClient, cache, newIntervalLimiter(cfg.ProviderRequestsPerSecond), cfg.UserAgent)
	if err != nil {
		return nil, err
	}
	osuckClient, err := newUpstreamClient(cfg.OsuckBaseURL, httpClient, cache, newIntervalLimiter(cfg.ProviderRequestsPerSecond), cfg.UserAgent)
	if err != nil {
		return nil, err
	}
	return &Server{
		official:  official,
		collector: newCollectorAdapter(collectorClient),
		osuSkins:  newOsuSkinsAdapter(osuSkinsClient),
		osuck:     newOsuckAdapter(osuckClient),
	}, nil
}

func (s *Server) GetProviderStatus(ctx context.Context, req *connect.Request[osuv1.GetProviderStatusRequest]) (*connect.Response[osuv1.GetProviderStatusResponse], error) {
	providers, err := requestedProviders(req.Msg.GetProviders())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	statuses := make([]*osuv1.ProviderStatus, len(providers))
	var waitGroup sync.WaitGroup
	for index, provider := range providers {
		index, provider := index, provider
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			statuses[index] = s.providerStatus(ctx, provider)
		}()
	}
	waitGroup.Wait()
	return connect.NewResponse(&osuv1.GetProviderStatusResponse{Providers: statuses}), nil
}

func (s *Server) SearchBeatmapItems(ctx context.Context, req *connect.Request[osuv1.SearchBeatmapItemsRequest]) (*connect.Response[osuv1.SearchBeatmapItemsResponse], error) {
	if err := validateSearchRequest(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	providers, err := requestedProviders(req.Msg.GetProviders())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	requested := make(map[osuv1.Provider]struct{}, len(providers))
	for _, provider := range providers {
		requested[provider] = struct{}{}
	}
	tokens := make(map[osuv1.Provider]string, len(req.Msg.GetPageTokens()))
	for _, token := range req.Msg.GetPageTokens() {
		if token == nil || !supportedProvider(token.GetProvider()) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("every page token must name a provider"))
		}
		if _, ok := requested[token.GetProvider()]; !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a page token may only target a requested provider"))
		}
		if _, duplicate := tokens[token.GetProvider()]; duplicate {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("duplicate page token for %s", token.GetProvider()))
		}
		if len(token.GetPageToken()) > 512 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("page token is too long"))
		}
		tokens[token.GetProvider()] = token.GetPageToken()
	}

	type providerResult struct {
		provider osuv1.Provider
		items    []*osuv1.BeatmapItem
		next     string
		status   *osuv1.ProviderStatus
	}
	results := make([]providerResult, len(providers))
	var waitGroup sync.WaitGroup
	for index, provider := range providers {
		index, provider := index, provider
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			result := providerResult{provider: provider}
			var providerErr error
			switch provider {
			case osuv1.Provider_PROVIDER_OSU_OFFICIAL:
				result.items, result.next, providerErr = s.official.search(ctx, req.Msg, tokens[provider])
			case osuv1.Provider_PROVIDER_OSU_COLLECTOR:
				result.items, result.next, providerErr = s.collector.search(ctx, req.Msg, tokens[provider])
			}
			if providerErr != nil {
				result.status = s.providerErrorStatus(provider, providerErr)
			} else {
				result.status = s.providerSuccessStatus(provider)
				if provider == osuv1.Provider_PROVIDER_OSU_COLLECTOR {
					if message := collectorFilterMessage(req.Msg.GetFilters()); message != "" {
						result.status.Message = message
					}
				}
			}
			results[index] = result
		}()
	}
	waitGroup.Wait()

	response := &osuv1.SearchBeatmapItemsResponse{}
	for _, result := range results {
		response.Items = append(response.Items, result.items...)
		response.Providers = append(response.Providers, result.status)
		if result.next != "" {
			response.NextPageTokens = append(response.NextPageTokens, &osuv1.ProviderCursor{
				Provider:  result.provider,
				PageToken: result.next,
			})
		}
	}
	return connect.NewResponse(response), nil
}

func (s *Server) GetBeatmapItem(ctx context.Context, req *connect.Request[osuv1.GetBeatmapItemRequest]) (*connect.Response[osuv1.GetBeatmapItemResponse], error) {
	provider := req.Msg.GetProvider()
	if !supportedProvider(provider) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a supported provider is required"))
	}
	sourceID := strings.TrimSpace(req.Msg.GetSourceId())
	if _, err := parsePositiveID(sourceID, "source_id"); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var item *osuv1.BeatmapItem
	var next string
	var err error
	switch provider {
	case osuv1.Provider_PROVIDER_OSU_OFFICIAL:
		if req.Msg.GetPageToken() != "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the official beatmapset detail does not use page tokens"))
		}
		item, err = s.official.detail(ctx, sourceID)
	case osuv1.Provider_PROVIDER_OSU_COLLECTOR:
		item, next, err = s.collector.detail(ctx, sourceID, req.Msg.GetPageToken())
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&osuv1.GetBeatmapItemResponse{
		Item:          item,
		NextPageToken: next,
		Provider:      s.providerSuccessStatus(provider),
	}), nil
}

func (s *Server) GetDownloadHandoff(_ context.Context, req *connect.Request[osuv1.GetDownloadHandoffRequest]) (*connect.Response[osuv1.GetDownloadHandoffResponse], error) {
	provider := req.Msg.GetProvider()
	if !supportedProvider(provider) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a supported provider is required"))
	}
	beatmapsetID := strings.TrimSpace(req.Msg.GetBeatmapsetId())
	if beatmapsetID == "" && provider == osuv1.Provider_PROVIDER_OSU_OFFICIAL {
		beatmapsetID = strings.TrimSpace(req.Msg.GetSourceId())
	}
	if beatmapsetID == "" {
		return connect.NewResponse(&osuv1.GetDownloadHandoffResponse{
			Handoff: unavailableHandoff("A beatmapset_id is required for a lazer handoff from a collection."),
		}), nil
	}
	if _, err := parsePositiveID(beatmapsetID, "beatmapset_id"); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&osuv1.GetDownloadHandoffResponse{Handoff: lazerHandoff(beatmapsetID, false)}), nil
}

func (s *Server) providerStatus(ctx context.Context, provider osuv1.Provider) *osuv1.ProviderStatus {
	switch provider {
	case osuv1.Provider_PROVIDER_OSU_OFFICIAL:
		return s.official.status(ctx)
	case osuv1.Provider_PROVIDER_OSU_COLLECTOR:
		return s.collector.status(ctx)
	default:
		return baseProviderStatus(provider)
	}
}

func (s *Server) providerSuccessStatus(provider osuv1.Provider) *osuv1.ProviderStatus {
	status := baseProviderStatus(provider)
	status.Available = true
	status.SupportsSearch = true
	status.SupportsDetail = true
	status.SupportsDownloadHandoff = true
	switch provider {
	case osuv1.Provider_PROVIDER_OSU_OFFICIAL:
		status.Configured = s.official.configured()
		status.Authentication = "OAuth 2.0 client credentials, public scope"
		status.ContractIsDocumented = true
		status.Message = "Official osu! API request succeeded."
	case osuv1.Provider_PROVIDER_OSU_COLLECTOR:
		status.Configured = true
		status.Authentication = "No credentials; public read-only site API"
		status.ContractIsDocumented = false
		status.Message = "osu!Collector request succeeded. Its site API is not formally documented and may change."
	}
	return status
}

func (s *Server) providerErrorStatus(provider osuv1.Provider, err error) *osuv1.ProviderStatus {
	status := baseProviderStatus(provider)
	status.Configured = provider == osuv1.Provider_PROVIDER_OSU_COLLECTOR || s.official.configured()
	status.ContractIsDocumented = provider == osuv1.Provider_PROVIDER_OSU_OFFICIAL
	status.Message = err.Error()
	return status
}

func baseProviderStatus(provider osuv1.Provider) *osuv1.ProviderStatus {
	return &osuv1.ProviderStatus{
		Provider:     provider,
		CheckedAtIso: time.Now().UTC().Format(time.RFC3339),
	}
}

func requestedProviders(input []osuv1.Provider) ([]osuv1.Provider, error) {
	if len(input) == 0 {
		return []osuv1.Provider{
			osuv1.Provider_PROVIDER_OSU_OFFICIAL,
			osuv1.Provider_PROVIDER_OSU_COLLECTOR,
		}, nil
	}
	seen := make(map[osuv1.Provider]struct{}, len(input))
	providers := make([]osuv1.Provider, 0, len(input))
	for _, provider := range input {
		if !supportedProvider(provider) {
			return nil, fmt.Errorf("unsupported provider %s", provider)
		}
		if _, duplicate := seen[provider]; duplicate {
			continue
		}
		seen[provider] = struct{}{}
		providers = append(providers, provider)
	}
	return providers, nil
}

func supportedProvider(provider osuv1.Provider) bool {
	return provider == osuv1.Provider_PROVIDER_OSU_OFFICIAL || provider == osuv1.Provider_PROVIDER_OSU_COLLECTOR
}

func validateSearchRequest(req *osuv1.SearchBeatmapItemsRequest) error {
	if len(req.GetQuery()) > 256 {
		return errors.New("query must be at most 256 characters")
	}
	if sortValue := strings.TrimSpace(req.GetSort()); sortValue != "" && officialSort(sortValue) == "" {
		return errors.New("sort is not supported")
	}
	if req.GetFilters() == nil {
		return nil
	}
	for name, value := range map[string]*osuv1.NumberRange{
		"stars":              req.GetFilters().GetStars(),
		"bpm":                req.GetFilters().GetBpm(),
		"length_seconds":     req.GetFilters().GetLengthSeconds(),
		"approach_rate":      req.GetFilters().GetApproachRate(),
		"circle_size":        req.GetFilters().GetCircleSize(),
		"overall_difficulty": req.GetFilters().GetOverallDifficulty(),
	} {
		if err := validateRange(name, value); err != nil {
			return err
		}
	}
	return nil
}

func validateRange(name string, value *osuv1.NumberRange) error {
	if value == nil {
		return nil
	}
	if value.Minimum != nil && (math.IsNaN(value.GetMinimum()) || math.IsInf(value.GetMinimum(), 0) || value.GetMinimum() < 0) {
		return fmt.Errorf("%s minimum must be a finite non-negative number", name)
	}
	if value.Maximum != nil && (math.IsNaN(value.GetMaximum()) || math.IsInf(value.GetMaximum(), 0) || value.GetMaximum() < 0) {
		return fmt.Errorf("%s maximum must be a finite non-negative number", name)
	}
	if value.Minimum != nil && value.Maximum != nil && value.GetMinimum() > value.GetMaximum() {
		return fmt.Errorf("%s minimum cannot exceed maximum", name)
	}
	return nil
}

func parsePositiveID(value, field string) (uint64, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("%s must be a positive decimal identifier", field)
	}
	return parsed, nil
}

func lazerHandoff(beatmapsetID string, disabled bool) *osuv1.DownloadHandoff {
	if disabled {
		return unavailableHandoff("The provider marks this beatmapset as unavailable for download.")
	}
	return &osuv1.DownloadHandoff{
		Kind:                     osuv1.DownloadHandoffKind_DOWNLOAD_HANDOFF_KIND_LAZER_URI,
		Available:                true,
		Uri:                      "osu://dl/" + beatmapsetID,
		BeatmapsetId:             beatmapsetID,
		RequiresOsuLazer:         true,
		RequiresUserConfirmation: true,
		Message:                  "Open this URI with osu!lazer to review and download the beatmapset in game.",
	}
}

func unavailableHandoff(message string) *osuv1.DownloadHandoff {
	return &osuv1.DownloadHandoff{
		Kind:      osuv1.DownloadHandoffKind_DOWNLOAD_HANDOFF_KIND_UNAVAILABLE,
		Available: false,
		Message:   message,
	}
}

func rulesetFromName(value string) osuv1.Ruleset {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "osu":
		return osuv1.Ruleset_RULESET_OSU
	case "taiko":
		return osuv1.Ruleset_RULESET_TAIKO
	case "fruits", "catch":
		return osuv1.Ruleset_RULESET_CATCH
	case "mania":
		return osuv1.Ruleset_RULESET_MANIA
	default:
		return osuv1.Ruleset_RULESET_UNSPECIFIED
	}
}

func includeRanges(item *osuv1.BeatmapItem, stars, bpm float64) {
	if stars > 0 {
		if item.MinimumStars == 0 || stars < item.MinimumStars {
			item.MinimumStars = stars
		}
		if stars > item.MaximumStars {
			item.MaximumStars = stars
		}
	}
	if bpm > 0 {
		if item.MinimumBpm == 0 || bpm < item.MinimumBpm {
			item.MinimumBpm = bpm
		}
		if bpm > item.MaximumBpm {
			item.MaximumBpm = bpm
		}
	}
}

func clampUint32(value uint64) uint32 {
	const maxUint32 = ^uint32(0)
	if value > uint64(maxUint32) {
		return maxUint32
	}
	return uint32(value)
}

func absolutePreviewURL(value string) string {
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	return value
}

func normalizeTimestamp(value string) string {
	if value == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return parsed.UTC().Format(time.RFC3339)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func sortItemsForStableProviderOrder(items []*osuv1.BeatmapItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Provider != items[j].Provider {
			return items[i].Provider < items[j].Provider
		}
		return items[i].SourceId < items[j].SourceId
	})
}
