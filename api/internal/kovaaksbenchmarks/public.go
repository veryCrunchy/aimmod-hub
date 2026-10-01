package kovaaksbenchmarks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// PlaceholderSteamID is Steam's base account id. It belongs to no player, so
// benchmark detail requests made with it return only the public definition:
// categories, scenarios, leaderboard ids and rank thresholds.
const PlaceholderSteamID = "76561197960265728"

const (
	leaderboardTTL    = 5 * time.Minute
	scenarioSearchTTL = 30 * time.Minute
	userScenariosTTL  = 10 * time.Minute
	userProfileTTL    = 30 * time.Minute
	definitionTTL     = 6 * time.Hour
	maxProviderPage   = 100
)

// LeaderboardEntry is one row of a KovaaK's scenario leaderboard.
type LeaderboardEntry struct {
	Rank            uint32
	SteamID         string
	KovaaksUsername string
	SteamName       string
	Country         string
	Score           float64
	CM360           float64
	PlayedAt        time.Time
}

type ScenarioLeaderboard struct {
	LeaderboardID uint32
	Total         uint32
	Page          uint32
	PageSize      uint32
	Entries       []LeaderboardEntry
}

type leaderboardPayload struct {
	Total int `json:"total"`
	Page  int `json:"page"`
	Max   int `json:"max"`
	Data  []struct {
		SteamID          string  `json:"steamId"`
		Score            float64 `json:"score"`
		Rank             uint32  `json:"rank"`
		SteamAccountName string  `json:"steamAccountName"`
		WebappUsername   string  `json:"webappUsername"`
		Country          string  `json:"country"`
		Attributes       struct {
			CM360 float64 `json:"cm360"`
			Epoch int64   `json:"epoch"`
		} `json:"attributes"`
	} `json:"data"`
}

func clampPage(page, pageSize uint32) (uint32, uint32) {
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize > maxProviderPage {
		pageSize = maxProviderPage
	}
	return page, pageSize
}

// GetScenarioLeaderboard reads one page of KovaaK's global leaderboard for a
// scenario. Only public leaderboard fields are kept.
func (c *Client) GetScenarioLeaderboard(ctx context.Context, leaderboardID, page, pageSize uint32) (*ScenarioLeaderboard, error) {
	if leaderboardID == 0 {
		return nil, fmt.Errorf("leaderboard id is required")
	}
	page, pageSize = clampPage(page, pageSize)
	var payload leaderboardPayload
	if err := c.getAPIJSON(ctx, "/leaderboard/scores/global", url.Values{
		"leaderboardId": {strconv.FormatUint(uint64(leaderboardID), 10)},
		"page":          {strconv.FormatUint(uint64(page), 10)},
		"max":           {strconv.FormatUint(uint64(pageSize), 10)},
	}, leaderboardTTL, &payload); err != nil {
		return nil, err
	}
	out := &ScenarioLeaderboard{
		LeaderboardID: leaderboardID,
		Total:         uint32(max(payload.Total, 0)),
		Page:          page,
		PageSize:      pageSize,
		Entries:       make([]LeaderboardEntry, 0, len(payload.Data)),
	}
	for _, row := range payload.Data {
		entry := LeaderboardEntry{
			Rank:            row.Rank,
			SteamID:         strings.TrimSpace(row.SteamID),
			KovaaksUsername: strings.TrimSpace(row.WebappUsername),
			SteamName:       strings.TrimSpace(row.SteamAccountName),
			Country:         strings.ToLower(strings.TrimSpace(row.Country)),
			Score:           row.Score,
			CM360:           row.Attributes.CM360,
		}
		if row.Attributes.Epoch > 0 {
			entry.PlayedAt = time.UnixMilli(row.Attributes.Epoch).UTC()
		}
		out.Entries = append(out.Entries, entry)
	}
	return out, nil
}

// ScenarioCatalogItem is a public KovaaK's scenario with popularity counts.
type ScenarioCatalogItem struct {
	LeaderboardID uint32
	ScenarioName  string
	AimType       string
	Authors       []string
	Plays         uint64
	Entries       uint64
	TopScore      float64
}

type ScenarioCatalogPage struct {
	Total uint32
	Items []ScenarioCatalogItem
}

type scenarioPopularPayload struct {
	Total int `json:"total"`
	Data  []struct {
		LeaderboardID json.Number `json:"leaderboardId"`
		ScenarioName  string      `json:"scenarioName"`
		Scenario      struct {
			AimType *string  `json:"aimType"`
			Authors []string `json:"authors"`
		} `json:"scenario"`
		Counts struct {
			Plays   uint64 `json:"plays"`
			Entries uint64 `json:"entries"`
		} `json:"counts"`
		TopScore struct {
			Score float64 `json:"score"`
		} `json:"topScore"`
	} `json:"data"`
}

// SearchScenarios searches KovaaK's public scenario catalog, ordered by
// popularity. An empty query lists the most popular scenarios.
func (c *Client) SearchScenarios(ctx context.Context, query string, page, pageSize uint32) (*ScenarioCatalogPage, error) {
	page, pageSize = clampPage(page, pageSize)
	values := url.Values{
		"page": {strconv.FormatUint(uint64(page), 10)},
		"max":  {strconv.FormatUint(uint64(pageSize), 10)},
	}
	if q := strings.TrimSpace(query); q != "" {
		values.Set("scenarioNameSearch", q)
	}
	var payload scenarioPopularPayload
	if err := c.getAPIJSON(ctx, "/scenario/popular", values, scenarioSearchTTL, &payload); err != nil {
		return nil, err
	}
	out := &ScenarioCatalogPage{Total: uint32(max(payload.Total, 0))}
	for _, row := range payload.Data {
		id, _ := strconv.ParseUint(row.LeaderboardID.String(), 10, 32)
		item := ScenarioCatalogItem{
			LeaderboardID: uint32(id),
			ScenarioName:  strings.TrimSpace(row.ScenarioName),
			Authors:       row.Scenario.Authors,
			Plays:         row.Counts.Plays,
			Entries:       row.Counts.Entries,
			TopScore:      row.TopScore.Score,
		}
		if row.Scenario.AimType != nil {
			item.AimType = strings.TrimSpace(*row.Scenario.AimType)
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// FindScenario returns the catalog entry whose name matches exactly
// (case-insensitive), or nil.
func (c *Client) FindScenario(ctx context.Context, scenarioName string) (*ScenarioCatalogItem, error) {
	name := strings.TrimSpace(scenarioName)
	if name == "" {
		return nil, nil
	}
	page, err := c.SearchScenarios(ctx, name, 0, 20)
	if err != nil {
		return nil, err
	}
	for i := range page.Items {
		if strings.EqualFold(page.Items[i].ScenarioName, name) {
			return &page.Items[i], nil
		}
	}
	return nil, nil
}

// UserScenario is a scenario a KovaaK's player has a leaderboard entry on.
type UserScenario struct {
	LeaderboardID uint32
	ScenarioName  string
	Plays         uint64
	Rank          uint32
	Score         float64
	CM360         float64
	PlayedAt      time.Time
}

type UserScenarioPage struct {
	Total uint32
	Items []UserScenario
}

type userScenarioPayload struct {
	Total int `json:"total"`
	Data  []struct {
		LeaderboardID json.Number `json:"leaderboardId"`
		ScenarioName  string      `json:"scenarioName"`
		Counts        struct {
			Plays uint64 `json:"plays"`
		} `json:"counts"`
		Rank       uint32  `json:"rank"`
		Score      float64 `json:"score"`
		Attributes struct {
			CM360 float64 `json:"cm360"`
			Epoch int64   `json:"epoch"`
		} `json:"attributes"`
	} `json:"data"`
}

// ListUserScenarios lists a KovaaK's player's public scenario entries, most
// played first.
func (c *Client) ListUserScenarios(ctx context.Context, username string, page, pageSize uint32) (*UserScenarioPage, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return &UserScenarioPage{}, nil
	}
	page, pageSize = clampPage(page, pageSize)
	var payload userScenarioPayload
	if err := c.getAPIJSON(ctx, "/user/scenario/total-play", url.Values{
		"username":     {username},
		"page":         {strconv.FormatUint(uint64(page), 10)},
		"max":          {strconv.FormatUint(uint64(pageSize), 10)},
		"sort_param[]": {"count"},
	}, userScenariosTTL, &payload); err != nil {
		return nil, err
	}
	out := &UserScenarioPage{Total: uint32(max(payload.Total, 0))}
	for _, row := range payload.Data {
		id, _ := strconv.ParseUint(row.LeaderboardID.String(), 10, 32)
		item := UserScenario{
			LeaderboardID: uint32(id),
			ScenarioName:  strings.TrimSpace(row.ScenarioName),
			Plays:         row.Counts.Plays,
			Rank:          row.Rank,
			Score:         row.Score,
			CM360:         row.Attributes.CM360,
		}
		if row.Attributes.Epoch > 0 {
			item.PlayedAt = time.UnixMilli(row.Attributes.Epoch).UTC()
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// UserProfile holds the public identity fields of a KovaaK's player. Social
// links, peripherals and other optional profile content are deliberately not
// read.
type UserProfile struct {
	SteamID         string
	KovaaksUsername string
	SteamName       string
	AvatarURL       string
	Country         string
	ScenariosPlayed uint64
	CreatedAt       time.Time
	LastAccessAt    time.Time
}

type userProfilePayload struct {
	SteamID            string `json:"steamId"`
	SteamAccountName   string `json:"steamAccountName"`
	SteamAccountAvatar string `json:"steamAccountAvatar"`
	Country            string `json:"country"`
	Created            string `json:"created"`
	LastAccess         string `json:"lastAccess"`
	ScenariosPlayed    any    `json:"scenariosPlayed"`
	Webapp             struct {
		Username string `json:"username"`
	} `json:"webapp"`
}

// GetUserProfile reads a KovaaK's player's public profile by username.
func (c *Client) GetUserProfile(ctx context.Context, username string) (*UserProfile, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, nil
	}
	var payload userProfilePayload
	if err := c.getAPIJSON(ctx, "/user/profile/by-username", url.Values{"username": {username}}, userProfileTTL, &payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.SteamID) == "" {
		return nil, nil
	}
	profile := &UserProfile{
		SteamID:         strings.TrimSpace(payload.SteamID),
		KovaaksUsername: firstNonEmpty(payload.Webapp.Username, username),
		SteamName:       strings.TrimSpace(payload.SteamAccountName),
		AvatarURL:       strings.TrimSpace(payload.SteamAccountAvatar),
		Country:         strings.ToLower(strings.TrimSpace(payload.Country)),
		ScenariosPlayed: anyToUint(payload.ScenariosPlayed),
	}
	profile.CreatedAt, _ = time.Parse(time.RFC3339, payload.Created)
	profile.LastAccessAt, _ = time.Parse(time.RFC3339, payload.LastAccess)
	c.storeResolvedIdentity(profile.SteamID, "", ResolvedSteamIdentity{Steam64: profile.SteamID, KovaaksUsername: profile.KovaaksUsername})
	return profile, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func anyToUint(value any) uint64 {
	switch v := value.(type) {
	case float64:
		if v > 0 {
			return uint64(v)
		}
	case string:
		parsed, _ := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
		return parsed
	}
	return 0
}

// GetBenchmarkDefinition returns a benchmark's public structure without any
// player's scores.
func (c *Client) GetBenchmarkDefinition(ctx context.Context, benchmarkID uint32) (*BenchmarkDetail, error) {
	if benchmarkID == 0 {
		return nil, nil
	}
	var raw json.RawMessage
	if err := c.fetchJSON(ctx, c.baseURL+"/player-progress-rank-benchmark", url.Values{
		"benchmarkId": {strconv.FormatUint(uint64(benchmarkID), 10)},
		"steamId":     {PlaceholderSteamID},
	}, definitionTTL, &raw); err != nil {
		return nil, err
	}
	return parseBenchmarkDetail(raw)
}
