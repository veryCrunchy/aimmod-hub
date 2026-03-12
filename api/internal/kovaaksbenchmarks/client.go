package kovaaksbenchmarks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	listPageSize = 20
	cacheTTL     = 15 * time.Minute
)

type Client struct {
	baseURL string
	http    *http.Client

	mu          sync.RWMutex
	listCache   map[string]cachedProfileBenchmarks
	detailCache map[string]cachedBenchmarkDetail
	rankCache   map[string]cachedScenarioRanks
}

type cachedProfileBenchmarks struct {
	expiresAt time.Time
	items     []ProfileBenchmarkSummary
}

type cachedBenchmarkDetail struct {
	expiresAt time.Time
	detail    *BenchmarkDetail
}

type cachedScenarioRanks struct {
	expiresAt time.Time
	items     []ScenarioBenchmarkRank
}

type ProfileBenchmarkSummary struct {
	BenchmarkID      uint32
	BenchmarkName    string
	BenchmarkIconURL string
	BenchmarkAuthor  string
	BenchmarkType    string
	OverallRankName  string
	OverallRankIcon  string
	OverallRankColor string
}

type BenchmarkRankVisual struct {
	RankIndex uint32
	RankName  string
	IconURL   string
	Color     string
	FrameURL  string
}

type ScenarioBenchmarkRank struct {
	BenchmarkID      uint32
	BenchmarkName    string
	BenchmarkIconURL string
	CategoryName     string
	ScenarioScore    float64
	LeaderboardRank  uint32
	LeaderboardID    uint32
	ScenarioRank     BenchmarkRankVisual
}

type BenchmarkThreshold struct {
	RankIndex uint32
	RankName  string
	IconURL   string
	Color     string
	Score     float64
}

type BenchmarkScenarioPage struct {
	ScenarioName    string
	CategoryName    string
	Score           float64
	LeaderboardRank uint32
	LeaderboardID   uint32
	ScenarioRank    BenchmarkRankVisual
	Thresholds      []BenchmarkThreshold
}

type BenchmarkDetail struct {
	OverallRank uint32
	Categories  map[string]BenchmarkCategory
	Ranks       []BenchmarkRankVisual
}

type BenchmarkCategory struct {
	CategoryRank uint32
	Scenarios    map[string]BenchmarkScenario
}

type BenchmarkScenario struct {
	Score           float64
	LeaderboardRank uint32
	ScenarioRank    uint32
	LeaderboardID   uint32
	RankMaxes       []float64
}

type benchmarkListEnvelope struct {
	Page  int                    `json:"page"`
	Max   int                    `json:"max"`
	Total int                    `json:"total"`
	Data  []benchmarkListSummary `json:"data"`
}

type benchmarkListSummary struct {
	BenchmarkName string `json:"benchmarkName"`
	BenchmarkID   uint32 `json:"benchmarkId"`
	BenchmarkIcon string `json:"benchmarkIcon"`
	BenchmarkAuth string `json:"benchmarkAuthor"`
	Type          string `json:"type"`
	RankName      string `json:"rankName"`
	RankIcon      string `json:"rankIcon"`
	RankColor     string `json:"rankColor"`
}

type benchmarkDetailPayload struct {
	OverallRank uint32                             `json:"overall_rank"`
	Categories  map[string]benchmarkCategoryRecord `json:"categories"`
	Ranks       []benchmarkRankRecord              `json:"ranks"`
}

type benchmarkCategoryRecord struct {
	CategoryRank uint32                             `json:"category_rank"`
	Scenarios    map[string]benchmarkScenarioRecord `json:"scenarios"`
}

type benchmarkScenarioRecord struct {
	Score           float64   `json:"score"`
	LeaderboardRank *uint32   `json:"leaderboard_rank"`
	ScenarioRank    uint32    `json:"scenario_rank"`
	RankMaxes       []float64 `json:"rank_maxes"`
	LeaderboardID   uint32    `json:"leaderboard_id"`
}

type benchmarkRankRecord struct {
	Icon  string `json:"icon"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Frame string `json:"frame"`
}

func NewClient() *Client {
	return &Client{
		baseURL: "https://kovaaks.com/webapp-backend/benchmarks",
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
		listCache:   map[string]cachedProfileBenchmarks{},
		detailCache: map[string]cachedBenchmarkDetail{},
		rankCache:   map[string]cachedScenarioRanks{},
	}
}

func (c *Client) ListPlayerBenchmarks(ctx context.Context, username string) ([]ProfileBenchmarkSummary, error) {
	normalized := strings.TrimSpace(username)
	if normalized == "" {
		return nil, nil
	}

	c.mu.RLock()
	if cached, ok := c.listCache[strings.ToLower(normalized)]; ok && time.Now().Before(cached.expiresAt) {
		c.mu.RUnlock()
		return append([]ProfileBenchmarkSummary(nil), cached.items...), nil
	}
	c.mu.RUnlock()

	items := make([]ProfileBenchmarkSummary, 0, listPageSize)
	for page := 0; page < 10; page++ {
		var payload benchmarkListEnvelope
		if err := c.getJSON(ctx, "/player-progress-rank", url.Values{
			"max":      {strconv.Itoa(listPageSize)},
			"page":     {strconv.Itoa(page)},
			"username": {normalized},
		}, &payload); err != nil {
			return nil, err
		}
		for _, item := range payload.Data {
			rankName := strings.TrimSpace(item.RankName)
			if rankName == "" || strings.EqualFold(rankName, "No Rank") {
				continue
			}
			items = append(items, ProfileBenchmarkSummary{
				BenchmarkID:      item.BenchmarkID,
				BenchmarkName:    strings.TrimSpace(item.BenchmarkName),
				BenchmarkIconURL: strings.TrimSpace(item.BenchmarkIcon),
				BenchmarkAuthor:  strings.TrimSpace(item.BenchmarkAuth),
				BenchmarkType:    strings.TrimSpace(item.Type),
				OverallRankName:  rankName,
				OverallRankIcon:  strings.TrimSpace(item.RankIcon),
				OverallRankColor: strings.TrimSpace(item.RankColor),
			})
		}
		if len(payload.Data) < listPageSize {
			break
		}
	}

	c.mu.Lock()
	c.listCache[strings.ToLower(normalized)] = cachedProfileBenchmarks{
		expiresAt: time.Now().Add(cacheTTL),
		items:     append([]ProfileBenchmarkSummary(nil), items...),
	}
	c.mu.Unlock()

	return items, nil
}

func (c *Client) ListScenarioRanks(
	ctx context.Context,
	steamID string,
	scenarioName string,
	benchmarks []ProfileBenchmarkSummary,
) ([]ScenarioBenchmarkRank, error) {
	if strings.TrimSpace(steamID) == "" || strings.TrimSpace(scenarioName) == "" || len(benchmarks) == 0 {
		return nil, nil
	}

	targetSlug := benchmarkScenarioSlug(scenarioName)
	cacheKey := fmt.Sprintf("%s:%s", strings.TrimSpace(steamID), targetSlug)

	c.mu.RLock()
	if cached, ok := c.rankCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		c.mu.RUnlock()
		return append([]ScenarioBenchmarkRank(nil), cached.items...), nil
	}
	c.mu.RUnlock()

	type result struct {
		ranks []ScenarioBenchmarkRank
	}

	sem := make(chan struct{}, 6)
	results := make(chan result, len(benchmarks))
	var wg sync.WaitGroup

	for _, benchmark := range benchmarks {
		benchmark := benchmark
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			detail, err := c.GetBenchmarkDetail(ctx, benchmark.BenchmarkID, steamID)
			if err != nil || detail == nil {
				return
			}

			out := make([]ScenarioBenchmarkRank, 0, 2)
			for categoryName, category := range detail.Categories {
				for benchmarkScenarioName, scenario := range category.Scenarios {
					if benchmarkScenarioSlug(benchmarkScenarioName) != targetSlug {
						continue
					}
					if scenario.ScenarioRank == 0 {
						continue
					}
					out = append(out, ScenarioBenchmarkRank{
						BenchmarkID:      benchmark.BenchmarkID,
						BenchmarkName:    benchmark.BenchmarkName,
						BenchmarkIconURL: benchmark.BenchmarkIconURL,
						CategoryName:     categoryName,
						ScenarioScore:    scenario.Score,
						LeaderboardRank:  scenario.LeaderboardRank,
						LeaderboardID:    scenario.LeaderboardID,
						ScenarioRank:     rankVisual(detail.Ranks, scenario.ScenarioRank),
					})
				}
			}
			if len(out) > 0 {
				results <- result{ranks: out}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	matches := make([]ScenarioBenchmarkRank, 0, 4)
	for result := range results {
		matches = append(matches, result.ranks...)
	}

	c.mu.Lock()
	c.rankCache[cacheKey] = cachedScenarioRanks{
		expiresAt: time.Now().Add(cacheTTL),
		items:     append([]ScenarioBenchmarkRank(nil), matches...),
	}
	c.mu.Unlock()

	return matches, nil
}

func (c *Client) BuildBenchmarkPage(
	ctx context.Context,
	benchmark ProfileBenchmarkSummary,
	steamID string,
) (*BenchmarkDetail, []BenchmarkCategoryPageRecord, error) {
	detail, err := c.GetBenchmarkDetail(ctx, benchmark.BenchmarkID, steamID)
	if err != nil || detail == nil {
		return nil, nil, err
	}
	categories := make([]BenchmarkCategoryPageRecord, 0, len(detail.Categories))
	for categoryName, category := range detail.Categories {
		scenarios := make([]BenchmarkScenarioPage, 0, len(category.Scenarios))
		for scenarioName, scenario := range category.Scenarios {
			if scenario.ScenarioRank == 0 {
				continue
			}
			thresholds := make([]BenchmarkThreshold, 0, len(scenario.RankMaxes))
			// rank_maxes[n] is the minimum score required to enter rank n+1.
			// The API returns player scores multiplied by 100, so divide by 100 to
			// get the real score that can be compared directly against rank_maxes.
			for rankIndex, threshold := range scenario.RankMaxes {
				nextRankIndex := rankIndex + 1
				if nextRankIndex >= len(detail.Ranks) {
					continue
				}
				rank := detail.Ranks[nextRankIndex]
				if rank.RankName == "" || strings.EqualFold(rank.RankName, "No Rank") {
					continue
				}
				thresholds = append(thresholds, BenchmarkThreshold{
					RankIndex: uint32(nextRankIndex),
					RankName:  rank.RankName,
					IconURL:   rank.IconURL,
					Color:     rank.Color,
					Score:     threshold,
				})
			}
			scenarios = append(scenarios, BenchmarkScenarioPage{
				ScenarioName:    scenarioName,
				CategoryName:    categoryName,
				Score:           scenario.Score / 100.0,
				LeaderboardRank: scenario.LeaderboardRank,
				LeaderboardID:   scenario.LeaderboardID,
				ScenarioRank:    rankVisual(detail.Ranks, scenario.ScenarioRank),
				Thresholds:      thresholds,
			})
		}
		if len(scenarios) == 0 {
			continue
		}
		categories = append(categories, BenchmarkCategoryPageRecord{
			CategoryName: categoryName,
			CategoryRank: category.CategoryRank,
			Scenarios:    scenarios,
		})
	}
	return detail, categories, nil
}

type BenchmarkCategoryPageRecord struct {
	CategoryName string
	CategoryRank uint32
	Scenarios    []BenchmarkScenarioPage
}

func (c *Client) GetBenchmarkDetail(ctx context.Context, benchmarkID uint32, steamID string) (*BenchmarkDetail, error) {
	if benchmarkID == 0 || strings.TrimSpace(steamID) == "" {
		return nil, nil
	}
	cacheKey := fmt.Sprintf("%d:%s", benchmarkID, strings.TrimSpace(steamID))

	c.mu.RLock()
	if cached, ok := c.detailCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		c.mu.RUnlock()
		return cached.detail, nil
	}
	c.mu.RUnlock()

	var payload benchmarkDetailPayload
	if err := c.getJSON(ctx, "/player-progress-rank-benchmark", url.Values{
		"benchmarkId": {strconv.FormatUint(uint64(benchmarkID), 10)},
		"steamId":     {strings.TrimSpace(steamID)},
	}, &payload); err != nil {
		return nil, err
	}

	detail := &BenchmarkDetail{
		OverallRank: payload.OverallRank,
		Categories:  make(map[string]BenchmarkCategory, len(payload.Categories)),
		Ranks:       make([]BenchmarkRankVisual, 0, len(payload.Ranks)),
	}
	for idx, rank := range payload.Ranks {
		detail.Ranks = append(detail.Ranks, BenchmarkRankVisual{
			RankIndex: uint32(idx),
			RankName:  strings.TrimSpace(rank.Name),
			IconURL:   strings.TrimSpace(rank.Icon),
			Color:     strings.TrimSpace(rank.Color),
			FrameURL:  strings.TrimSpace(rank.Frame),
		})
	}
	for categoryName, category := range payload.Categories {
		nextCategory := BenchmarkCategory{
			CategoryRank: category.CategoryRank,
			Scenarios:    make(map[string]BenchmarkScenario, len(category.Scenarios)),
		}
		for scenarioName, scenario := range category.Scenarios {
			leaderboardRank := uint32(0)
			if scenario.LeaderboardRank != nil {
				leaderboardRank = *scenario.LeaderboardRank
			}
			nextCategory.Scenarios[scenarioName] = BenchmarkScenario{
				Score:           scenario.Score,
				LeaderboardRank: leaderboardRank,
				ScenarioRank:    scenario.ScenarioRank,
				RankMaxes:       append([]float64(nil), scenario.RankMaxes...),
				LeaderboardID:   scenario.LeaderboardID,
			}
		}
		detail.Categories[categoryName] = nextCategory
	}

	c.mu.Lock()
	c.detailCache[cacheKey] = cachedBenchmarkDetail{
		expiresAt: time.Now().Add(cacheTTL),
		detail:    detail,
	}
	c.mu.Unlock()

	return detail, nil
}

// RankVisualFromDetail returns the rank visual for a given rank index in a BenchmarkDetail.
func RankVisualFromDetail(detail *BenchmarkDetail, rankIndex uint32) BenchmarkRankVisual {
	if detail == nil {
		return BenchmarkRankVisual{RankIndex: rankIndex}
	}
	return rankVisual(detail.Ranks, rankIndex)
}

func rankVisual(ranks []BenchmarkRankVisual, rankIndex uint32) BenchmarkRankVisual {
	if int(rankIndex) >= 0 && int(rankIndex) < len(ranks) {
		return ranks[rankIndex]
	}
	if len(ranks) > 0 {
		return ranks[0]
	}
	return BenchmarkRankVisual{RankIndex: rankIndex}
}

func benchmarkScenarioSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
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

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build benchmark request: %w", err)
	}
	req.Header.Set("accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request benchmarks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("benchmark request failed: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode benchmark response: %w", err)
	}
	return nil
}
