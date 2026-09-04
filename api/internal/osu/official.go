package osu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

type officialAdapter struct {
	client       *upstreamClient
	http         *http.Client
	oauthURL     string
	clientID     uint64
	clientSecret string
	limiter      *intervalLimiter
	userAgent    string
	tokenMu      sync.Mutex
	token        string
	tokenExpiry  time.Time
	configError  error
}

type officialTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type officialSearchResponse struct {
	Beatmapsets  []officialBeatmapset `json:"beatmapsets"`
	CursorString string               `json:"cursor_string"`
}

type officialBeatmapset struct {
	ID             uint64               `json:"id"`
	Artist         string               `json:"artist"`
	Title          string               `json:"title"`
	Creator        string               `json:"creator"`
	Status         string               `json:"status"`
	Tags           string               `json:"tags"`
	PreviewURL     string               `json:"preview_url"`
	SubmittedDate  string               `json:"submitted_date"`
	LastUpdated    string               `json:"last_updated"`
	PlayCount      uint64               `json:"play_count"`
	FavouriteCount uint64               `json:"favourite_count"`
	Covers         officialCovers       `json:"covers"`
	Availability   officialAvailability `json:"availability"`
	Description    struct {
		Description string `json:"description"`
	} `json:"description"`
	Beatmaps []officialBeatmap `json:"beatmaps"`
}

type officialCovers struct {
	Card  string `json:"card"`
	Cover string `json:"cover"`
	List  string `json:"list"`
	Slim  string `json:"slimcover"`
}

type officialAvailability struct {
	DownloadDisabled bool `json:"download_disabled"`
}

type officialBeatmap struct {
	ID               uint64  `json:"id"`
	BeatmapsetID     uint64  `json:"beatmapset_id"`
	Checksum         string  `json:"checksum"`
	Version          string  `json:"version"`
	Mode             string  `json:"mode"`
	Status           string  `json:"status"`
	DifficultyRating float64 `json:"difficulty_rating"`
	BPM              float64 `json:"bpm"`
	AR               float64 `json:"ar"`
	CS               float64 `json:"cs"`
	Accuracy         float64 `json:"accuracy"`
	Drain            float64 `json:"drain"`
	HitLength        uint32  `json:"hit_length"`
}

type officialUser struct {
	ID          uint64 `json:"id"`
	Username    string `json:"username"`
	CountryCode string `json:"country_code"`
	AvatarURL   string `json:"avatar_url"`
	CoverURL    string `json:"cover_url"`
	Playmode    string `json:"playmode"`
	IsActive    bool   `json:"is_active"`
	IsOnline    bool   `json:"is_online"`
	IsSupporter bool   `json:"is_supporter"`
	JoinDate    string `json:"join_date"`
	LastVisit   string `json:"last_visit"`
	Cover       struct {
		URL       string `json:"url"`
		CustomURL string `json:"custom_url"`
	} `json:"cover"`
	Statistics *struct {
		PP           float64 `json:"pp"`
		GlobalRank   uint32  `json:"global_rank"`
		CountryRank  uint32  `json:"country_rank"`
		HitAccuracy  float64 `json:"hit_accuracy"`
		PlayCount    uint32  `json:"play_count"`
		PlayTime     uint64  `json:"play_time"`
		TotalScore   uint64  `json:"total_score"`
		RankedScore  uint64  `json:"ranked_score"`
		MaximumCombo uint32  `json:"maximum_combo"`
		Level        struct {
			Current  uint32 `json:"current"`
			Progress uint32 `json:"progress"`
		} `json:"level"`
		GradeCounts struct {
			SSH uint32 `json:"ssh"`
			SS  uint32 `json:"ss"`
			SH  uint32 `json:"sh"`
			S   uint32 `json:"s"`
			A   uint32 `json:"a"`
		} `json:"grade_counts"`
	} `json:"statistics"`
	Team *struct {
		ID        uint64 `json:"id"`
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
		FlagURL   string `json:"flag_url"`
	} `json:"team"`
}

func newOfficialAdapter(cfg Config, httpClient *http.Client, cache *responseCache, limiter *intervalLimiter) (*officialAdapter, error) {
	client, err := newUpstreamClient(cfg.OfficialBaseURL, httpClient, cache, limiter, cfg.UserAgent)
	if err != nil {
		return nil, err
	}
	var clientID uint64
	if strings.TrimSpace(cfg.OfficialClientID) != "" {
		clientID, err = strconv.ParseUint(strings.TrimSpace(cfg.OfficialClientID), 10, 64)
		if err != nil {
			clientID = 0
		}
	}
	adapter := &officialAdapter{
		client:       client,
		http:         httpClient,
		oauthURL:     strings.TrimRight(cfg.OfficialBaseURL, "/") + "/oauth/token",
		clientID:     clientID,
		clientSecret: strings.TrimSpace(cfg.OfficialClientSecret),
		limiter:      limiter,
		userAgent:    cfg.UserAgent,
	}
	if err != nil {
		adapter.configError = fmt.Errorf("invalid osu client ID: %w", err)
	}
	return adapter, nil
}

func (a *officialAdapter) configured() bool {
	return a.clientID != 0 && a.clientSecret != ""
}

func (a *officialAdapter) accessToken(ctx context.Context) (string, error) {
	if a.configError != nil {
		return "", a.configError
	}
	if !a.configured() {
		return "", fmt.Errorf("osu OAuth client credentials are not configured")
	}
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	if a.token != "" && time.Now().Add(30*time.Second).Before(a.tokenExpiry) {
		return a.token, nil
	}
	if err := a.limiter.wait(ctx); err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{
		"client_id":     a.clientID,
		"client_secret": a.clientSecret,
		"grant_type":    "client_credentials",
		"scope":         "public",
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.oauthURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", a.userAgent)
	resp, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("osu OAuth returned HTTP %d", resp.StatusCode)
	}
	var token officialTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return "", err
	}
	if token.AccessToken == "" || token.ExpiresIn <= 0 {
		return "", fmt.Errorf("osu OAuth response did not contain a usable access token")
	}
	a.token = token.AccessToken
	a.tokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return a.token, nil
}

func (a *officialAdapter) status(ctx context.Context) *osuv1.ProviderStatus {
	status := baseProviderStatus(osuv1.Provider_PROVIDER_OSU_OFFICIAL)
	status.Configured = a.configured()
	status.Authentication = "OAuth 2.0 client credentials, public scope"
	status.ContractIsDocumented = true
	if a.configError != nil {
		status.Message = "Official osu! API configuration is invalid: " + a.configError.Error()
		return status
	}
	if !status.Configured {
		status.Message = "Set AIMMOD_OSU_CLIENT_ID and AIMMOD_OSU_CLIENT_SECRET to enable the official osu! API."
		return status
	}
	if _, err := a.accessToken(ctx); err != nil {
		status.Message = "Official osu! API authentication failed: " + err.Error()
		return status
	}
	status.Available = true
	status.SupportsSearch = true
	status.SupportsDetail = true
	status.SupportsDownloadHandoff = true
	status.Message = "Official osu! API is available."
	return status
}

func (a *officialAdapter) search(ctx context.Context, req *osuv1.SearchBeatmapItemsRequest, pageToken string) ([]*osuv1.BeatmapItem, string, error) {
	token, err := a.accessToken(ctx)
	if err != nil {
		return nil, "", err
	}
	query := url.Values{}
	if searchQuery := buildOfficialSearchQuery(req.GetQuery(), req.GetFilters()); searchQuery != "" {
		query.Set("q", searchQuery)
	}
	if filters := req.GetFilters(); filters != nil {
		if mode, ok := officialMode(filters.GetRuleset()); ok {
			query.Set("m", mode)
		}
		if status := strings.TrimSpace(filters.GetStatus()); status != "" {
			query.Set("s", status)
		}
	}
	if sortValue := officialSort(req.GetSort()); sortValue != "" {
		query.Set("sort", sortValue)
	}
	if pageToken != "" {
		query.Set("cursor_string", pageToken)
	}
	body, err := a.client.get(ctx, "/api/v2/beatmapsets/search", query, "Bearer "+token)
	if err != nil {
		return nil, "", err
	}
	var response officialSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, "", err
	}
	items := make([]*osuv1.BeatmapItem, 0, len(response.Beatmapsets))
	for i := range response.Beatmapsets {
		items = append(items, normalizeOfficialBeatmapset(&response.Beatmapsets[i]))
	}
	return items, response.CursorString, nil
}

func (a *officialAdapter) detail(ctx context.Context, sourceID string) (*osuv1.BeatmapItem, error) {
	if _, err := parsePositiveID(sourceID, "source_id"); err != nil {
		return nil, err
	}
	token, err := a.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	body, err := a.client.get(ctx, "/api/v2/beatmapsets/"+sourceID, nil, "Bearer "+token)
	if err != nil {
		return nil, err
	}
	var beatmapset officialBeatmapset
	if err := json.Unmarshal(body, &beatmapset); err != nil {
		return nil, err
	}
	return normalizeOfficialBeatmapset(&beatmapset), nil
}

func (a *officialAdapter) userProfile(ctx context.Context, req *osuv1.GetOfficialUserProfileRequest) (*osuv1.OfficialUserProfile, error) {
	identifier := strings.TrimSpace(req.GetIdentifier())
	lookupKey := ""
	switch req.GetLookupKey() {
	case osuv1.OfficialUserLookupKey_OFFICIAL_USER_LOOKUP_KEY_ID:
		if _, err := parsePositiveID(identifier, "identifier"); err != nil {
			return nil, err
		}
		lookupKey = "id"
	case osuv1.OfficialUserLookupKey_OFFICIAL_USER_LOOKUP_KEY_USERNAME:
		if identifier == "" || len(identifier) > 32 || strings.ContainsAny(identifier, "/?#") {
			return nil, fmt.Errorf("username identifier must contain 1 through 32 characters and no URL path separators")
		}
		lookupKey = "username"
	default:
		return nil, fmt.Errorf("lookup_key must be ID or username")
	}
	mode, ok := officialRulesetName(req.GetRuleset())
	if !ok {
		return nil, fmt.Errorf("a supported ruleset is required")
	}
	token, err := a.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	query := url.Values{"key": []string{lookupKey}}
	body, err := a.client.get(ctx, "/api/v2/users/"+identifier+"/"+mode, query, "Bearer "+token)
	if err != nil {
		return nil, err
	}
	var user officialUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}
	if user.ID == 0 || strings.TrimSpace(user.Username) == "" {
		return nil, fmt.Errorf("official osu! API returned an incomplete user profile")
	}
	profile := &osuv1.OfficialUserProfile{
		UserId:         user.ID,
		Username:       user.Username,
		CountryCode:    user.CountryCode,
		AvatarUrl:      absolutePreviewURL(user.AvatarURL),
		CoverUrl:       absolutePreviewURL(firstNonEmpty(user.CoverURL, user.Cover.URL, user.Cover.CustomURL)),
		DefaultRuleset: rulesetFromName(user.Playmode),
		IsActive:       user.IsActive,
		IsOnline:       user.IsOnline,
		IsSupporter:    user.IsSupporter,
		JoinDateIso:    normalizeTimestamp(user.JoinDate),
		LastVisitIso:   normalizeTimestamp(user.LastVisit),
	}
	if user.Statistics != nil {
		statistics := user.Statistics
		profile.Statistics = &osuv1.OfficialUserStatistics{
			Pp:              statistics.PP,
			GlobalRank:      statistics.GlobalRank,
			CountryRank:     statistics.CountryRank,
			HitAccuracy:     statistics.HitAccuracy,
			PlayCount:       statistics.PlayCount,
			PlayTimeSeconds: statistics.PlayTime,
			TotalScore:      statistics.TotalScore,
			RankedScore:     statistics.RankedScore,
			MaximumCombo:    statistics.MaximumCombo,
			LevelCurrent:    statistics.Level.Current,
			LevelProgress:   statistics.Level.Progress,
			GradeCounts: &osuv1.OfficialUserGradeCounts{
				Ssh: statistics.GradeCounts.SSH,
				Ss:  statistics.GradeCounts.SS,
				Sh:  statistics.GradeCounts.SH,
				S:   statistics.GradeCounts.S,
				A:   statistics.GradeCounts.A,
			},
		}
	}
	if user.Team != nil {
		profile.Team = &osuv1.OfficialUserTeam{
			Id:        user.Team.ID,
			Name:      user.Team.Name,
			ShortName: user.Team.ShortName,
			FlagUrl:   absolutePreviewURL(user.Team.FlagURL),
		}
	}
	return profile, nil
}

func normalizeOfficialBeatmapset(set *officialBeatmapset) *osuv1.BeatmapItem {
	item := &osuv1.BeatmapItem{
		Provider:       osuv1.Provider_PROVIDER_OSU_OFFICIAL,
		Kind:           osuv1.ItemKind_ITEM_KIND_BEATMAPSET,
		SourceId:       strconv.FormatUint(set.ID, 10),
		Title:          set.Title,
		Artist:         set.Artist,
		Creator:        set.Creator,
		Description:    set.Description.Description,
		Status:         set.Status,
		CoverUrl:       firstNonEmpty(set.Covers.Card, set.Covers.Cover, set.Covers.List, set.Covers.Slim),
		PreviewUrl:     absolutePreviewURL(set.PreviewURL),
		BeatmapCount:   uint32(len(set.Beatmaps)),
		FavouriteCount: clampUint32(set.FavouriteCount),
		PlayCount:      clampUint32(set.PlayCount),
		SubmittedAtIso: normalizeTimestamp(set.SubmittedDate),
		UpdatedAtIso:   normalizeTimestamp(set.LastUpdated),
		Tags:           strings.Fields(set.Tags),
	}
	rulesets := make(map[osuv1.Ruleset]uint32)
	for i := range set.Beatmaps {
		difficulty := normalizeOfficialDifficulty(&set.Beatmaps[i], set)
		item.Difficulties = append(item.Difficulties, difficulty)
		rulesets[difficulty.Ruleset]++
		includeRanges(item, difficulty.Stars, difficulty.Bpm)
	}
	item.RulesetCounts = sortedRulesetCounts(rulesets)
	item.DownloadHandoff = lazerHandoff(strconv.FormatUint(set.ID, 10), set.Availability.DownloadDisabled)
	return item
}

func normalizeOfficialDifficulty(beatmap *officialBeatmap, set *officialBeatmapset) *osuv1.BeatmapDifficulty {
	return &osuv1.BeatmapDifficulty{
		BeatmapId:         strconv.FormatUint(beatmap.ID, 10),
		BeatmapsetId:      strconv.FormatUint(beatmap.BeatmapsetID, 10),
		Checksum:          beatmap.Checksum,
		Name:              beatmap.Version,
		Ruleset:           rulesetFromName(beatmap.Mode),
		Status:            beatmap.Status,
		Stars:             beatmap.DifficultyRating,
		Bpm:               beatmap.BPM,
		ApproachRate:      beatmap.AR,
		CircleSize:        beatmap.CS,
		OverallDifficulty: beatmap.Accuracy,
		DrainRate:         beatmap.Drain,
		LengthSeconds:     beatmap.HitLength,
		Title:             set.Title,
		Artist:            set.Artist,
		Creator:           set.Creator,
		CoverUrl:          firstNonEmpty(set.Covers.Card, set.Covers.Cover, set.Covers.List, set.Covers.Slim),
		DownloadHandoff:   lazerHandoff(strconv.FormatUint(set.ID, 10), set.Availability.DownloadDisabled),
	}
}

func buildOfficialSearchQuery(query string, filters *osuv1.BeatmapSearchFilters) string {
	parts := make([]string, 0, 12)
	if query = strings.TrimSpace(query); query != "" {
		parts = append(parts, query)
	}
	if filters == nil {
		return strings.Join(parts, " ")
	}
	parts = appendRangeQuery(parts, "stars", filters.Stars)
	parts = appendRangeQuery(parts, "bpm", filters.Bpm)
	parts = appendRangeQuery(parts, "length", filters.LengthSeconds)
	parts = appendRangeQuery(parts, "ar", filters.ApproachRate)
	parts = appendRangeQuery(parts, "cs", filters.CircleSize)
	parts = appendRangeQuery(parts, "od", filters.OverallDifficulty)
	return strings.Join(parts, " ")
}

func appendRangeQuery(parts []string, key string, value *osuv1.NumberRange) []string {
	if value == nil {
		return parts
	}
	if value.Minimum != nil {
		parts = append(parts, key+">="+formatNumber(value.GetMinimum()))
	}
	if value.Maximum != nil {
		parts = append(parts, key+"<="+formatNumber(value.GetMaximum()))
	}
	return parts
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func officialMode(ruleset osuv1.Ruleset) (string, bool) {
	switch ruleset {
	case osuv1.Ruleset_RULESET_OSU:
		return "0", true
	case osuv1.Ruleset_RULESET_TAIKO:
		return "1", true
	case osuv1.Ruleset_RULESET_CATCH:
		return "2", true
	case osuv1.Ruleset_RULESET_MANIA:
		return "3", true
	default:
		return "", false
	}
}

func officialRulesetName(ruleset osuv1.Ruleset) (string, bool) {
	switch ruleset {
	case osuv1.Ruleset_RULESET_OSU:
		return "osu", true
	case osuv1.Ruleset_RULESET_TAIKO:
		return "taiko", true
	case osuv1.Ruleset_RULESET_CATCH:
		return "fruits", true
	case osuv1.Ruleset_RULESET_MANIA:
		return "mania", true
	default:
		return "", false
	}
}

func officialSort(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	allowed := map[string]string{
		"artist_asc": "artist_asc", "artist_desc": "artist_desc",
		"creator_asc": "creator_asc", "creator_desc": "creator_desc",
		"difficulty_asc": "difficulty_asc", "difficulty_desc": "difficulty_desc",
		"favourites_asc": "favourites_asc", "favourites_desc": "favourites_desc",
		"plays_asc": "plays_asc", "plays_desc": "plays_desc",
		"ranked_asc": "ranked_asc", "ranked_desc": "ranked_desc",
		"relevance_asc": "relevance_asc", "relevance_desc": "relevance_desc",
		"title_asc": "title_asc", "title_desc": "title_desc",
		"updated_asc": "updated_asc", "updated_desc": "updated_desc",
	}
	return allowed[value]
}

func sortedRulesetCounts(counts map[osuv1.Ruleset]uint32) []*osuv1.RulesetCount {
	keys := make([]int, 0, len(counts))
	for ruleset, count := range counts {
		if ruleset != osuv1.Ruleset_RULESET_UNSPECIFIED && count > 0 {
			keys = append(keys, int(ruleset))
		}
	}
	sort.Ints(keys)
	result := make([]*osuv1.RulesetCount, 0, len(keys))
	for _, key := range keys {
		ruleset := osuv1.Ruleset(key)
		result = append(result, &osuv1.RulesetCount{Ruleset: ruleset, Count: counts[ruleset]})
	}
	return result
}
