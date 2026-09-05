package osu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

// Coverage describes only the bounded best/recent endpoints, never all play history.
type ScoreCoverage struct {
	Status  string `json:"status"`
	Fetched int    `json:"fetched"`
	Pages   int    `json:"pages"`
	HasMore bool   `json:"hasMore"`
}

type OfficialScoreCoverage struct {
	Best            ScoreCoverage `json:"best"`
	Recent          ScoreCoverage `json:"recent"`
	CompleteHistory bool          `json:"completeHistory"`
}

type OfficialPublicScore struct {
	Replay           store.OsuPublicReplay
	FallbackEligible bool
}

type OfficialScoresResult struct {
	Scores   []OfficialPublicScore
	Coverage OfficialScoreCoverage
}

type officialScore struct {
	ID            int64          `json:"id"`
	UserID        int64          `json:"user_id"`
	RulesetID     *int           `json:"ruleset_id"`
	BeatmapID     int64          `json:"beatmap_id"`
	EndedAt       time.Time      `json:"ended_at"`
	TotalScore    int64          `json:"total_score"`
	LegacyScoreID *int64         `json:"legacy_score_id"`
	PP            *float64       `json:"pp"`
	Accuracy      float64        `json:"accuracy"`
	MaxCombo      int            `json:"max_combo"`
	Passed        bool           `json:"passed"`
	HasReplay     bool           `json:"has_replay"`
	Statistics    map[string]int `json:"statistics"`
	Mods          []struct {
		Acronym  string         `json:"acronym"`
		Settings map[string]any `json:"settings"`
	} `json:"mods"`
	Beatmap    officialBeatmap    `json:"beatmap"`
	Beatmapset officialBeatmapset `json:"beatmapset"`
}

// Public user scores require the application's public OAuth scope, not a user's token.
// Each category is capped at two pages of 100. The cache key includes mode, type,
// offset and the pinned modern score format (legacy IDs must not alias modern IDs).
func (s *Server) GetPublicUserScores(ctx context.Context, userID int64, mode string) (OfficialScoresResult, error) {
	result := OfficialScoresResult{Scores: []OfficialPublicScore{}}
	if userID <= 0 || !validScoreMode(mode) {
		return result, fmt.Errorf("invalid score user or mode")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if s == nil || s.official == nil || !s.official.configured() {
		result.Coverage.Best.Status = "not_configured"
		result.Coverage.Recent.Status = "not_configured"
		return result, nil
	}
	token, err := s.official.accessToken(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.Coverage.Best.Status = "authentication_failed"
		result.Coverage.Recent.Status = "authentication_failed"
		return result, nil
	}
	seen := map[int64]bool{}
	for _, category := range []string{"best", "recent"} {
		coverage := &result.Coverage.Best
		if category == "recent" {
			coverage = &result.Coverage.Recent
		}
		coverage.Status = "available"
		for page := 0; page < 2; page++ {
			query := url.Values{"mode": {mode}, "legacy_only": {"0"}, "limit": {"100"}, "offset": {strconv.Itoa(page * 100)}}
			if category == "recent" {
				query.Set("include_fails", "1")
			}
			body, err := s.official.getScorePage(ctx, fmt.Sprintf("/api/v2/users/%d/scores/%s", userID, category), query, token)
			if err != nil {
				if ctx.Err() != nil {
					return result, ctx.Err()
				}
				coverage.Status = scoreErrorStatus(err)
				coverage.HasMore = true
				if coverage.Status == "rate_limited" || coverage.Status == "authentication_failed" {
					if category == "best" {
						result.Coverage.Recent = ScoreCoverage{Status: coverage.Status, HasMore: true}
					}
					return result, nil
				}
				break
			}
			var scores []officialScore
			if json.Unmarshal(body, &scores) != nil || scores == nil {
				coverage.Status = "invalid_response"
				coverage.HasMore = true
				break
			}
			coverage.Pages++
			coverage.Fetched += len(scores)
			coverage.HasMore = len(scores) >= 100
			newIDs := 0
			for _, score := range scores {
				if score.UserID != userID || score.ID <= 0 || score.RulesetID == nil || scoreMode(*score.RulesetID) != mode {
					coverage.Status = "invalid_response"
					continue
				}
				if seen[score.ID] {
					continue
				}
				seen[score.ID] = true
				newIDs++
				result.Scores = append(result.Scores, normalizePublicScore(score, mode))
			}
			if !coverage.HasMore {
				break
			}
			if page > 0 && newIDs == 0 {
				coverage.Status = "repeated_page"
				break
			}
			if page == 1 {
				coverage.Status = "page_limit"
			}
		}
	}
	return result, nil
}

func validScoreMode(mode string) bool {
	return mode == "osu" || mode == "taiko" || mode == "fruits" || mode == "mania"
}
func scoreMode(id int) string {
	return map[int]string{0: "osu", 1: "taiko", 2: "fruits", 3: "mania"}[id]
}
func scoreErrorStatus(err error) string {
	switch {
	case isUpstreamHTTPStatus(err, 429):
		return "rate_limited"
	case isUpstreamHTTPStatus(err, 401), isUpstreamHTTPStatus(err, 403):
		return "authentication_failed"
	case isUpstreamHTTPStatus(err, 404):
		return "not_found"
	default:
		return "unavailable"
	}
}

func (a *officialAdapter) getScorePage(ctx context.Context, path string, query url.Values, token string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	requestURL := a.client.resolve(path, query)
	key := "GET score-v20220705 " + requestURL
	if body, ok := a.client.cache.get(key); ok {
		return body, nil
	}
	if err := a.limiter.wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("x-api-version", "20220705")
	response, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &upstreamHTTPError{StatusCode: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxUpstreamResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxUpstreamResponseBytes {
		return nil, fmt.Errorf("score page too large")
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("invalid score JSON")
	}
	a.client.cache.set(key, body)
	return body, nil
}

func normalizePublicScore(score officialScore, mode string) OfficialPublicScore {
	mods := make([]string, 0, len(score.Mods))
	for _, mod := range score.Mods {
		mods = append(mods, mod.Acronym)
	}
	id := score.BeatmapID
	if id == 0 {
		id = int64(score.Beatmap.ID)
	}
	replay := store.OsuPublicReplay{
		OnlineScoreID: score.ID, OsuUserID: score.UserID, Visibility: store.OsuVisibilityPublic,
		BeatmapID: id, BeatmapSetID: int64(score.Beatmap.BeatmapsetID), BeatmapChecksum: score.Beatmap.Checksum,
		Title: score.Beatmapset.Title, Artist: score.Beatmapset.Artist, Creator: score.Beatmapset.Creator,
		CoverURL: score.Beatmapset.Covers.Card, Difficulty: score.Beatmap.Version, Ruleset: mode,
		StarRating: score.Beatmap.DifficultyRating, BPM: score.Beatmap.BPM, LengthMS: int64(score.Beatmap.TotalLength) * 1000,
		PlayedAt: score.EndedAt, TotalScore: score.TotalScore, PerformancePoints: score.PP, Accuracy: score.Accuracy,
		MaxCombo: score.MaxCombo, Count300: score.Statistics["great"], Count100: score.Statistics["ok"],
		Count50: score.Statistics["meh"], CountMiss: score.Statistics["miss"], Passed: score.Passed, Mods: mods,
	}
	// Uploads lack mod settings and non-standard judgements. Those cannot prove a
	// content identity, nor can legacy and standardised score totals be equated.
	eligible := mode == "osu" && len(mods) == 0 && score.LegacyScoreID == nil && score.Statistics != nil
	return OfficialPublicScore{Replay: replay, FallbackEligible: eligible}
}
