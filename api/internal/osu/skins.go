package osu

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

func (s *Server) GetSkinProviderStatus(ctx context.Context, req *connect.Request[osuv1.GetSkinProviderStatusRequest]) (*connect.Response[osuv1.GetSkinProviderStatusResponse], error) {
	providers, err := requestedSkinProviders(req.Msg.GetProviders())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	statuses := make([]*osuv1.SkinProviderStatus, len(providers))
	var waitGroup sync.WaitGroup
	for index, provider := range providers {
		index, provider := index, provider
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			statuses[index] = s.skinProviderStatus(ctx, provider)
		}()
	}
	waitGroup.Wait()
	return connect.NewResponse(&osuv1.GetSkinProviderStatusResponse{Providers: statuses}), nil
}

func (s *Server) SearchSkins(ctx context.Context, req *connect.Request[osuv1.SearchSkinsRequest]) (*connect.Response[osuv1.SearchSkinsResponse], error) {
	if err := validateSkinSearchRequest(req.Msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	providers, err := requestedSkinProviders(req.Msg.GetProviders())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	requested := make(map[osuv1.SkinProvider]struct{}, len(providers))
	for _, provider := range providers {
		requested[provider] = struct{}{}
	}
	tokens := make(map[osuv1.SkinProvider]string, len(req.Msg.GetPageTokens()))
	for _, token := range req.Msg.GetPageTokens() {
		if token == nil || !supportedSkinProvider(token.GetProvider()) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("every skin page token must name a supported provider"))
		}
		if _, ok := requested[token.GetProvider()]; !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a skin page token may only target a requested provider"))
		}
		if _, duplicate := tokens[token.GetProvider()]; duplicate {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("duplicate skin page token for %s", token.GetProvider()))
		}
		if len(token.GetPageToken()) > 512 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("skin page token is too long"))
		}
		tokens[token.GetProvider()] = token.GetPageToken()
	}
	type providerResult struct {
		provider osuv1.SkinProvider
		items    []*osuv1.SkinItem
		next     string
		status   *osuv1.SkinProviderStatus
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
			case osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS:
				if message := unsupportedOsuSkinsFilterMessage(req.Msg.GetFilters()); message != "" {
					result.status = s.osuSkins.status(ctx)
					result.status.Message = message
					results[index] = result
					return
				}
				result.items, result.next, providerErr = s.osuSkins.search(ctx, req.Msg, tokens[provider])
			case osuv1.SkinProvider_SKIN_PROVIDER_OSUCK:
				result.items, result.next, providerErr = s.osuck.search(ctx, req.Msg, tokens[provider])
			}
			if providerErr != nil {
				result.status = baseSkinProviderStatus(provider)
				result.status.Retryable = true
				if provider == osuv1.SkinProvider_SKIN_PROVIDER_OSUCK {
					result.status.BrowserUrl = "https://skins.osuck.net/search?query=" + url.QueryEscape(req.Msg.GetQuery())
				}
				result.status.Message = providerErr.Error()
			} else {
				result.status = s.skinProviderStatus(ctx, provider)
			}
			results[index] = result
		}()
	}
	waitGroup.Wait()
	response := &osuv1.SearchSkinsResponse{}
	for _, result := range results {
		response.Items = append(response.Items, result.items...)
		response.Providers = append(response.Providers, result.status)
		if result.next != "" {
			response.NextPageTokens = append(response.NextPageTokens, &osuv1.SkinProviderCursor{Provider: result.provider, PageToken: result.next})
		}
	}
	response.Items = deduplicateSkins(response.Items)
	return connect.NewResponse(response), nil
}

func (s *Server) GetSkin(ctx context.Context, req *connect.Request[osuv1.GetSkinRequest]) (*connect.Response[osuv1.GetSkinResponse], error) {
	provider := req.Msg.GetProvider()
	if !supportedSkinProvider(provider) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a supported skin provider is required"))
	}
	sourceID := strings.TrimSpace(req.Msg.GetSourceId())
	if sourceID == "" || len(sourceID) > 128 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_id must contain 1 through 128 characters"))
	}
	var item *osuv1.SkinItem
	var err error
	switch provider {
	case osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS:
		item, err = s.osuSkins.detail(ctx, sourceID)
	case osuv1.SkinProvider_SKIN_PROVIDER_OSUCK:
		item, err = s.osuck.detail(ctx, sourceID)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	normalizeSkinSources(item)
	return connect.NewResponse(&osuv1.GetSkinResponse{Item: item, Provider: s.skinProviderStatus(ctx, provider)}), nil
}

func (s *Server) GetSkinDownloadHandoff(ctx context.Context, req *connect.Request[osuv1.GetSkinDownloadHandoffRequest]) (*connect.Response[osuv1.GetSkinDownloadHandoffResponse], error) {
	provider := req.Msg.GetProvider()
	if !supportedSkinProvider(provider) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a supported skin provider is required"))
	}
	sourceID := strings.TrimSpace(req.Msg.GetSourceId())
	if sourceID == "" || len(sourceID) > 128 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_id must contain 1 through 128 characters"))
	}
	var handoff *osuv1.SkinDownloadHandoff
	var err error
	switch provider {
	case osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS:
		if !osuSkinsIDPattern.MatchString(sourceID) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("osuskins.net source_id must contain seven ASCII letters or digits"))
		}
		item, detailErr := s.osuSkins.detail(ctx, sourceID)
		if detailErr != nil {
			return nil, connect.NewError(connect.CodeUnavailable, detailErr)
		}
		handoff = item.DownloadHandoff
	case osuv1.SkinProvider_SKIN_PROVIDER_OSUCK:
		handoff, err = s.osuck.downloadHandoff(ctx, sourceID)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&osuv1.GetSkinDownloadHandoffResponse{Handoff: handoff, Provider: s.skinProviderStatus(ctx, provider)}), nil
}

func (s *Server) GetOfficialUserProfile(ctx context.Context, req *connect.Request[osuv1.GetOfficialUserProfileRequest]) (*connect.Response[osuv1.GetOfficialUserProfileResponse], error) {
	profile, err := s.official.userProfile(ctx, req.Msg)
	if err != nil {
		if strings.Contains(err.Error(), "must") {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&osuv1.GetOfficialUserProfileResponse{Profile: profile, Provider: s.providerSuccessStatus(osuv1.Provider_PROVIDER_OSU_OFFICIAL)}), nil
}

func (s *Server) skinProviderStatus(ctx context.Context, provider osuv1.SkinProvider) *osuv1.SkinProviderStatus {
	switch provider {
	case osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS:
		return s.osuSkins.status(ctx)
	case osuv1.SkinProvider_SKIN_PROVIDER_OSUCK:
		return s.osuck.status(ctx)
	default:
		return baseSkinProviderStatus(provider)
	}
}

func baseSkinProviderStatus(provider osuv1.SkinProvider) *osuv1.SkinProviderStatus {
	status := &osuv1.SkinProviderStatus{Provider: provider, CheckedAtIso: time.Now().UTC().Format(time.RFC3339)}
	if provider == osuv1.SkinProvider_SKIN_PROVIDER_OSUCK {
		status.BrowserUrl = "https://skins.osuck.net/"
	} else if provider == osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS {
		status.BrowserUrl = "https://osuskins.net/"
	}
	return status
}

func requestedSkinProviders(input []osuv1.SkinProvider) ([]osuv1.SkinProvider, error) {
	if len(input) == 0 {
		return []osuv1.SkinProvider{osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS, osuv1.SkinProvider_SKIN_PROVIDER_OSUCK}, nil
	}
	seen := make(map[osuv1.SkinProvider]struct{}, len(input))
	providers := make([]osuv1.SkinProvider, 0, len(input))
	for _, provider := range input {
		if !supportedSkinProvider(provider) {
			return nil, fmt.Errorf("unsupported skin provider %s", provider)
		}
		if _, duplicate := seen[provider]; duplicate {
			continue
		}
		seen[provider] = struct{}{}
		providers = append(providers, provider)
	}
	return providers, nil
}

func supportedSkinProvider(provider osuv1.SkinProvider) bool {
	return provider == osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS || provider == osuv1.SkinProvider_SKIN_PROVIDER_OSUCK
}

func validateSkinSearchRequest(req *osuv1.SearchSkinsRequest) error {
	if len(req.GetQuery()) > 256 {
		return errors.New("skin query must be at most 256 characters")
	}
	filters := req.GetFilters()
	if filters == nil {
		return nil
	}
	for name, value := range map[string]string{
		"aspect_ratio": filters.GetAspectRatio(),
		"creator":      filters.GetCreator(),
		"player":       filters.GetPlayer(),
		"tag":          filters.GetTag(),
	} {
		if len(value) > 128 {
			return fmt.Errorf("%s must be at most 128 characters", name)
		}
	}
	for _, ruleset := range filters.GetRulesets() {
		if _, ok := officialRulesetName(ruleset); !ok {
			return fmt.Errorf("unsupported skin ruleset %s", ruleset)
		}
	}
	return nil
}

func unsupportedOsuSkinsFilterMessage(filters *osuv1.SkinSearchFilters) string {
	if filters == nil {
		return ""
	}
	unsupported := make([]string, 0, 3)
	if strings.TrimSpace(filters.GetAspectRatio()) != "" {
		unsupported = append(unsupported, "aspect ratio")
	}
	if strings.TrimSpace(filters.GetTag()) != "" {
		unsupported = append(unsupported, "tag")
	}
	// false is the normal safe/default browse mode and matches the provider's
	// public catalogue behaviour. Only an explicit request to include sensitive
	// entries requires metadata that osuskins.net does not expose reliably.
	if filters.GetIncludeSensitive() {
		unsupported = append(unsupported, "sensitive-content")
	}
	if len(unsupported) == 0 {
		return ""
	}
	return "osuskins.net does not expose reliable " + strings.Join(unsupported, ", ") + " metadata, so its results were omitted."
}

func unavailableSkinHandoff(interactive bool, message string) *osuv1.SkinDownloadHandoff {
	return &osuv1.SkinDownloadHandoff{
		Kind:                            osuv1.SkinDownloadHandoffKind_SKIN_DOWNLOAD_HANDOFF_KIND_UNAVAILABLE,
		Available:                       false,
		RequiresInteractiveVerification: interactive,
		Message:                         message,
	}
}
