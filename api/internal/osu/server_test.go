package osu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
	"google.golang.org/protobuf/proto"
)

func TestServerNormalizesProvidersAndCachesSearch(t *testing.T) {
	var officialSearches atomic.Int32
	var collectorSearches atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			if r.Method != http.MethodPost {
				t.Errorf("OAuth method = %s; want POST", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
		case "/api/v2/beatmapsets/search":
			officialSearches.Add(1)
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf("Authorization = %q", got)
			}
			if got := r.URL.Query().Get("m"); got != "0" {
				t.Errorf("mode = %q; want 0", got)
			}
			if got := r.URL.Query().Get("q"); !strings.Contains(got, "stars>=4") || !strings.Contains(got, "stars<=6") {
				t.Errorf("search query = %q; missing star range", got)
			}
			_, _ = w.Write([]byte(`{"cursor_string":"official-next","beatmapsets":[{"id":123,"artist":"Artist","title":"Title","creator":"Mapper","status":"ranked","tags":"aim speed","preview_url":"//preview.example/123.mp3","submitted_date":"2025-01-01T00:00:00Z","last_updated":"2025-01-02T00:00:00Z","play_count":42,"favourite_count":7,"covers":{"card":"https://img.example/card.jpg"},"availability":{"download_disabled":false},"description":{"description":"Official set"},"beatmaps":[{"id":456,"beatmapset_id":123,"version":"Insane","mode":"osu","status":"ranked","difficulty_rating":5.2,"bpm":180,"ar":9,"cs":4,"accuracy":8,"drain":6,"hit_length":90}]}]}`))
		case "/api/collections/search":
			collectorSearches.Add(1)
			_, _ = w.Write([]byte(`{"nextPageCursor":88,"hasMore":true,"collections":[{"id":77,"name":"Aim maps","description":"A collection","uploader":{"id":1,"username":"Collector"},"beatmapCount":10,"favourites":3,"dateUploaded":{"_seconds":1735689600},"dateLastModified":{"_seconds":1735776000},"difficultySpread":{"4":2,"6":3},"bpmSpread":{"160":1,"200":2},"modes":{"osu":10}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	server, err := NewServer(Config{
		OfficialBaseURL:           upstream.URL,
		CollectorBaseURL:          upstream.URL,
		OfficialClientID:          "1234",
		OfficialClientSecret:      "secret",
		CacheTTL:                  time.Minute,
		CacheMaxEntries:           16,
		ProviderRequestsPerSecond: 100,
		HTTPClient:                upstream.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	request := &osuv1.SearchBeatmapItemsRequest{
		Query:     "training",
		Providers: []osuv1.Provider{osuv1.Provider_PROVIDER_OSU_OFFICIAL, osuv1.Provider_PROVIDER_OSU_COLLECTOR},
		Filters: &osuv1.BeatmapSearchFilters{
			Ruleset: osuv1.Ruleset_RULESET_OSU,
			Stars:   &osuv1.NumberRange{Minimum: proto.Float64(4), Maximum: proto.Float64(6)},
		},
	}
	response, err := server.SearchBeatmapItems(context.Background(), connect.NewRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Msg.GetItems()) != 2 {
		t.Fatalf("items = %d; want 2", len(response.Msg.GetItems()))
	}
	official := response.Msg.GetItems()[0]
	if official.GetSourceId() != "123" || official.GetDownloadHandoff().GetUri() != "osu://dl/123" || official.GetPreviewUrl() != "https://preview.example/123.mp3" {
		t.Fatalf("unexpected official normalization: %+v", official)
	}
	collector := response.Msg.GetItems()[1]
	if collector.GetSourceId() != "77" || collector.GetDownloadHandoff().GetAvailable() || collector.GetMinimumStars() != 4 || collector.GetMaximumBpm() != 200 {
		t.Fatalf("unexpected collector normalization: %+v", collector)
	}
	if len(response.Msg.GetNextPageTokens()) != 2 {
		t.Fatalf("next page tokens = %d; want 2", len(response.Msg.GetNextPageTokens()))
	}

	if _, err := server.SearchBeatmapItems(context.Background(), connect.NewRequest(request)); err != nil {
		t.Fatal(err)
	}
	if officialSearches.Load() != 1 || collectorSearches.Load() != 1 {
		t.Fatalf("cached request counts official=%d collector=%d; want 1 each", officialSearches.Load(), collectorSearches.Load())
	}
}

func TestAllProviderSearchKeepsCollectorResultsWithoutOfficialOAuth(t *testing.T) {
	var officialRequests atomic.Int32
	var collectorSearches atomic.Int32
	var collectorDetails atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/collections/search":
			collectorSearches.Add(1)
			_, _ = w.Write([]byte(`{"hasMore":false,"collections":[{"id":77,"name":"Accountless maps","description":"Available without osu OAuth","uploader":{"id":1,"username":"Collector"},"beatmapCount":10,"modes":{"osu":10}}]}`))
		case "/api/collections/77":
			collectorDetails.Add(1)
			_, _ = w.Write([]byte(`{"id":77,"name":"Accountless maps","uploader":{"id":1,"username":"Collector"},"beatmapCount":1,"modes":{"osu":1}}`))
		case "/api/collections/77/beatmapsv2":
			collectorDetails.Add(1)
			_, _ = w.Write([]byte(`{"hasMore":false,"beatmaps":[{"id":456,"beatmapset_id":123,"version":"Hard","mode":"osu","difficulty_rating":4.2,"ar":9,"cs":4,"accuracy":8,"drain":6,"bpm":180,"hit_length":95,"beatmapset":{"id":123,"artist":"Artist","title":"Title","creator":"Mapper","covers":{"card":"https://img.example/card.jpg"}}}]}`))
		default:
			officialRequests.Add(1)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	server, err := NewServer(Config{
		OfficialBaseURL:           upstream.URL,
		CollectorBaseURL:          upstream.URL,
		OsuSkinsBaseURL:           upstream.URL,
		OsuckBaseURL:              upstream.URL,
		ProviderRequestsPerSecond: 100,
		HTTPClient:                upstream.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}

	response, err := server.SearchBeatmapItems(context.Background(), connect.NewRequest(&osuv1.SearchBeatmapItemsRequest{
		Query: "training",
		// An empty provider list is the versioned contract for all providers.
	}))
	if err != nil {
		t.Fatal(err)
	}
	if officialRequests.Load() != 0 {
		t.Fatalf("official requests = %d; want 0 without configured OAuth", officialRequests.Load())
	}
	if collectorSearches.Load() != 1 {
		t.Fatalf("collector searches = %d; want 1", collectorSearches.Load())
	}
	if collectorDetails.Load() != 2 {
		t.Fatalf("collector detail requests = %d; want 2", collectorDetails.Load())
	}
	items := response.Msg.GetItems()
	if len(items) != 1 || items[0].GetProvider() != osuv1.Provider_PROVIDER_OSU_COLLECTOR || items[0].GetSourceId() != "77" {
		t.Fatalf("accountless search items = %+v; want the Collector result", items)
	}
	if len(items[0].GetDifficulties()) != 1 || items[0].GetDifficulties()[0].GetBeatmapId() != "456" {
		t.Fatalf("accountless search difficulties = %+v; want hydrated Collector beatmap 456", items[0].GetDifficulties())
	}
	statuses := response.Msg.GetProviders()
	if len(statuses) != 2 {
		t.Fatalf("provider statuses = %d; want 2", len(statuses))
	}
	official := statuses[0]
	if official.GetProvider() != osuv1.Provider_PROVIDER_OSU_OFFICIAL || official.GetConfigured() || official.GetAvailable() || !strings.Contains(official.GetMessage(), "not configured") {
		t.Fatalf("official status = %+v; want an unconfigured per-provider status", official)
	}
	collector := statuses[1]
	if collector.GetProvider() != osuv1.Provider_PROVIDER_OSU_COLLECTOR || !collector.GetConfigured() || !collector.GetAvailable() {
		t.Fatalf("Collector status = %+v; want available", collector)
	}
}

func TestServerDetailsAndLazerHandoff(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_, _ = w.Write([]byte(`{"access_token":"token","expires_in":3600}`))
		case "/api/v2/beatmapsets/123":
			_, _ = w.Write([]byte(`{"id":123,"title":"Official","availability":{"download_disabled":false},"beatmaps":[]}`))
		case "/api/collections/77":
			_, _ = w.Write([]byte(`{"id":77,"name":"Collection","beatmapCount":1}`))
		case "/api/collections/77/beatmapsv2":
			_, _ = w.Write([]byte(`{"hasMore":false,"beatmaps":[{"id":456,"beatmapset_id":123,"version":"Hard","mode":"osu","difficulty_rating":4.2,"beatmapset":{"id":123,"artist":"Artist","title":"Title","creator":"Mapper","covers":{"card":"https://img.example/card.jpg"}}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	server, err := NewServer(Config{
		OfficialBaseURL:           upstream.URL,
		CollectorBaseURL:          upstream.URL,
		OfficialClientID:          "1234",
		OfficialClientSecret:      "secret",
		ProviderRequestsPerSecond: 100,
		HTTPClient:                upstream.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := server.GetBeatmapItem(context.Background(), connect.NewRequest(&osuv1.GetBeatmapItemRequest{
		Provider: osuv1.Provider_PROVIDER_OSU_COLLECTOR,
		SourceId: "77",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := detail.Msg.GetItem().GetDifficulties(); len(got) != 1 || got[0].GetBeatmapsetId() != "123" || got[0].GetTitle() != "Title" || got[0].GetDownloadHandoff().GetUri() != "osu://dl/123" {
		t.Fatalf("collector difficulties = %+v", got)
	}
	handoff, err := server.GetDownloadHandoff(context.Background(), connect.NewRequest(&osuv1.GetDownloadHandoffRequest{
		Provider:     osuv1.Provider_PROVIDER_OSU_COLLECTOR,
		SourceId:     "77",
		BeatmapsetId: "123",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !handoff.Msg.GetHandoff().GetAvailable() || handoff.Msg.GetHandoff().GetUri() != "osu://dl/123" {
		t.Fatalf("handoff = %+v", handoff.Msg.GetHandoff())
	}
	unavailable, err := server.GetDownloadHandoff(context.Background(), connect.NewRequest(&osuv1.GetDownloadHandoffRequest{
		Provider: osuv1.Provider_PROVIDER_OSU_COLLECTOR,
		SourceId: "77",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if unavailable.Msg.GetHandoff().GetAvailable() {
		t.Fatal("collection-level handoff unexpectedly available")
	}
}

func TestOfficialUserProfileUsesUsernameLookup(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_, _ = w.Write([]byte(`{"access_token":"token","expires_in":3600}`))
		case "/api/v2/users/example-player/osu":
			if got := r.URL.Query().Get("key"); got != "username" {
				t.Errorf("profile key = %q; want username", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer token" {
				t.Errorf("profile authorization = %q", got)
			}
			_, _ = w.Write([]byte(`{"id":42,"username":"example-player","country_code":"ZZ","avatar_url":"https://example.invalid/avatar.png","cover_url":"https://example.invalid/cover.jpg","playmode":"osu","is_active":true,"is_online":false,"is_supporter":true,"join_date":"2021-01-01T00:00:00Z","last_visit":"2026-09-01T00:00:00Z","statistics":{"pp":1234.5,"global_rank":1000,"country_rank":50,"hit_accuracy":98.2,"play_count":2000,"play_time":3000,"total_score":4000,"ranked_score":3500,"maximum_combo":500,"level":{"current":100,"progress":12},"grade_counts":{"ssh":1,"ss":2,"sh":3,"s":4,"a":5}},"team":{"id":10,"name":"AimMod","short_name":"AIM","flag_url":"https://example.invalid/flag.png"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	server, err := NewServer(Config{
		OfficialBaseURL:           upstream.URL,
		CollectorBaseURL:          upstream.URL,
		OsuSkinsBaseURL:           upstream.URL,
		OsuckBaseURL:              upstream.URL,
		OfficialClientID:          "1234",
		OfficialClientSecret:      "secret",
		ProviderRequestsPerSecond: 100,
		HTTPClient:                upstream.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.GetOfficialUserProfile(context.Background(), connect.NewRequest(&osuv1.GetOfficialUserProfileRequest{
		Identifier: "example-player",
		LookupKey:  osuv1.OfficialUserLookupKey_OFFICIAL_USER_LOOKUP_KEY_USERNAME,
		Ruleset:    osuv1.Ruleset_RULESET_OSU,
	}))
	if err != nil {
		t.Fatal(err)
	}
	profile := response.Msg.GetProfile()
	if profile.GetUserId() != 42 || profile.GetUsername() != "example-player" || profile.GetCountryCode() != "ZZ" {
		t.Fatalf("unexpected profile identity: %+v", profile)
	}
	if profile.GetStatistics().GetPp() != 1234.5 || profile.GetTeam().GetName() != "AimMod" || profile.GetAvatarUrl() == "" || profile.GetCoverUrl() == "" {
		t.Fatalf("unexpected profile normalization: %+v", profile)
	}
}

func TestOsuSkinsSearchDetailAndTurnstileBoundary(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			if got := r.URL.Query().Get("q"); got != "" && got != "minimal" {
				t.Errorf("skin query = %q; want minimal", got)
			}
			if r.URL.Query().Get("q") == "minimal" {
				got := r.URL.Query()["mode[]"]
				if len(got) != 1 || got[0] != "1" {
					t.Errorf("skin modes = %v; want [1]", got)
				}
			}
			_, _ = w.Write([]byte(`<html><body><a href="/skin/Abc1234"><img src="https://cdn.example/thumb.webp"><h2>Minimal Skin</h2><span><svg><use href="#arrowDown"></use></svg><span>1.2k</span></span><span><svg><use href="#eyeSolid"></use></svg><span>3.4k</span></span></a><a href="/?p=2">Next</a></body></html>`))
		case "/skin/Abc1234":
			_, _ = w.Write([]byte(`<html><head><script type="application/ld+json">{"@type":"Article","headline":"Minimal Skin","description":"Created by Mapper, supports Standard mode, 105.50 MB file size.","image":"https://cdn.example/header.webp","datePublished":"2025-01-01T00:00:00Z","dateModified":"2025-01-02T00:00:00Z","author":[{"name":"Mapper"}],"interactionStatistic":[{"interactionType":"https://schema.org/ViewAction","userInteractionCount":"3400"},{"interactionType":"https://schema.org/DownloadAction","userInteractionCount":"1200"}]}</script></head><body><a href="/?mode[]=1">Standard</a><div title="Gameplay" data-sskey="gameplay" data-src="https://cdn.example/gameplay.webp" data-lg-size="1920-1080"></div><form action="/skin/Abc1234/download" method="POST"></form></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	server, err := NewServer(Config{
		OfficialBaseURL:           upstream.URL,
		CollectorBaseURL:          upstream.URL,
		OsuSkinsBaseURL:           upstream.URL,
		OsuckBaseURL:              upstream.URL,
		ProviderRequestsPerSecond: 100,
		HTTPClient:                upstream.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	search, err := server.SearchSkins(context.Background(), connect.NewRequest(&osuv1.SearchSkinsRequest{
		Query:     "minimal",
		Providers: []osuv1.SkinProvider{osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS},
		Filters:   &osuv1.SkinSearchFilters{Rulesets: []osuv1.Ruleset{osuv1.Ruleset_RULESET_OSU}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(search.Msg.GetItems()) != 1 || search.Msg.GetItems()[0].GetName() != "Minimal Skin" || search.Msg.GetItems()[0].GetDownloadCount() != 1200 {
		t.Fatalf("unexpected skin search: %+v", search.Msg)
	}
	if got := search.Msg.GetNextPageTokens(); len(got) != 1 || got[0].GetPageToken() != "2" {
		t.Fatalf("unexpected skin cursor: %+v", got)
	}
	detail, err := server.GetSkin(context.Background(), connect.NewRequest(&osuv1.GetSkinRequest{
		Provider: osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS,
		SourceId: "Abc1234",
	}))
	if err != nil {
		t.Fatal(err)
	}
	item := detail.Msg.GetItem()
	if item.GetCreator() != "Mapper" || item.GetFileSizeBytes() != 105500000 || len(item.GetScreenshots()) != 1 || item.GetScreenshots()[0].GetWidth() != 1920 {
		t.Fatalf("unexpected skin detail: %+v", item)
	}
	handoff, err := server.GetSkinDownloadHandoff(context.Background(), connect.NewRequest(&osuv1.GetSkinDownloadHandoffRequest{
		Provider: osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS,
		SourceId: "Abc1234",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !handoff.Msg.GetHandoff().GetAvailable() || handoff.Msg.GetHandoff().GetKind() != osuv1.SkinDownloadHandoffKind_SKIN_DOWNLOAD_HANDOFF_KIND_BROWSER_URL || !handoff.Msg.GetHandoff().GetRequiresInteractiveVerification() || handoff.Msg.GetHandoff().GetUri() != "https://osuskins.net/skin/Abc1234" {
		t.Fatalf("unexpected Turnstile handoff: %+v", handoff.Msg.GetHandoff())
	}
}

func TestOsuSkinsSearchTreatsNotFoundAsEmptyResults(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Fatalf("unexpected upstream request: %s", r.URL.String())
		}
		if r.URL.Query().Get("q") == "missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`<html><body><a href="/skin/Abc1234"><h2>Known skin</h2></a></body></html>`))
	}))
	defer upstream.Close()

	server, err := NewServer(Config{
		OfficialBaseURL:           upstream.URL,
		CollectorBaseURL:          upstream.URL,
		OsuSkinsBaseURL:           upstream.URL,
		OsuckBaseURL:              upstream.URL,
		ProviderRequestsPerSecond: 100,
		HTTPClient:                upstream.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}

	response, err := server.SearchSkins(context.Background(), connect.NewRequest(&osuv1.SearchSkinsRequest{
		Query:     "missing",
		Providers: []osuv1.SkinProvider{osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS},
		Filters:   &osuv1.SkinSearchFilters{},
	}))
	if err != nil {
		t.Fatalf("SearchSkins returned an error for a valid empty search: %v", err)
	}
	if len(response.Msg.GetItems()) != 0 {
		t.Fatalf("expected an empty result set, got %+v", response.Msg.GetItems())
	}
	if len(response.Msg.GetProviders()) != 1 || !response.Msg.GetProviders()[0].GetAvailable() {
		t.Fatalf("expected osuskins.net to remain available, got %+v", response.Msg.GetProviders())
	}
}

func TestOsuSkinsSafeSensitiveDefaultDoesNotSuppressSearch(t *testing.T) {
	includeSensitive := false
	if message := unsupportedOsuSkinsFilterMessage(&osuv1.SkinSearchFilters{IncludeSensitive: &includeSensitive}); message != "" {
		t.Fatalf("safe sensitive-content default suppressed osuskins.net search: %q", message)
	}

	includeSensitive = true
	if message := unsupportedOsuSkinsFilterMessage(&osuv1.SkinSearchFilters{IncludeSensitive: &includeSensitive}); !strings.Contains(message, "sensitive-content") {
		t.Fatalf("explicit sensitive-content request was not rejected: %q", message)
	}
}

func TestOfficialUserProfileRequiresConfiguredOAuth(t *testing.T) {
	server, err := NewServer(Config{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = server.GetOfficialUserProfile(context.Background(), connect.NewRequest(&osuv1.GetOfficialUserProfileRequest{
		Identifier: "example-player",
		LookupKey:  osuv1.OfficialUserLookupKey_OFFICIAL_USER_LOOKUP_KEY_USERNAME,
		Ruleset:    osuv1.Ruleset_RULESET_OSU,
	}))
	if err == nil || connect.CodeOf(err) != connect.CodeUnavailable || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unconfigured profile error = %v", err)
	}
}

func TestOsuckReportsCloudflareBoundaryWithoutFallbackData(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "challenge", http.StatusForbidden)
	}))
	defer upstream.Close()
	server, err := NewServer(Config{
		OfficialBaseURL:           upstream.URL,
		CollectorBaseURL:          upstream.URL,
		OsuSkinsBaseURL:           upstream.URL,
		OsuckBaseURL:              upstream.URL,
		ProviderRequestsPerSecond: 100,
		HTTPClient:                upstream.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	status, err := server.GetSkinProviderStatus(context.Background(), connect.NewRequest(&osuv1.GetSkinProviderStatusRequest{
		Providers: []osuv1.SkinProvider{osuv1.SkinProvider_SKIN_PROVIDER_OSUCK},
	}))
	if err != nil {
		t.Fatal(err)
	}
	provider := status.Msg.GetProviders()[0]
	if provider.GetAvailable() || provider.GetSupportsSearch() || !strings.Contains(provider.GetMessage(), "HTTP 403") {
		t.Fatalf("unexpected osuck status: %+v", provider)
	}
	search, err := server.SearchSkins(context.Background(), connect.NewRequest(&osuv1.SearchSkinsRequest{
		Providers: []osuv1.SkinProvider{osuv1.SkinProvider_SKIN_PROVIDER_OSUCK},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(search.Msg.GetItems()) != 0 || len(search.Msg.GetProviders()) != 1 || search.Msg.GetProviders()[0].GetMessage() == "" {
		t.Fatalf("osuck search returned fallback data: %+v", search.Msg)
	}
}
