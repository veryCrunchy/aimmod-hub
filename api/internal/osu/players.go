package osu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

type PlayerIndex interface {
	SaveIndexedOsuScores(context.Context, []store.IndexedOsuScore) error
	ListIndexedOsuScores(context.Context, int, bool) ([]json.RawMessage, error)
	ClaimOsuScoreIndexPlayer(context.Context) (int64, string, error)
	FinishOsuScoreIndexPlayer(context.Context, int64, string) error

	SaveIndexedOsuPlayers(context.Context, string, []store.OsuPublicProfile) error
	ListIndexedOsuPlayers(context.Context, string, string, int) ([]store.OsuPublicProfile, error)
	ClaimOsuPlayerIndexPage(context.Context) (string, int, error)
	FinishOsuPlayerIndexPage(context.Context, string, int, int) error
}

type PublicPlayersPage struct {
	Items    []store.OsuPublicProfile `json:"items"`
	NextPage int                      `json:"nextPage"`
	Cached   bool                     `json:"cached"`
}

func (s *Server) ListPublicPlayers(ctx context.Context, mode, query string, page int) (PublicPlayersPage, error) {
	if s == nil || s.official == nil {
		return PublicPlayersPage{}, fmt.Errorf("official API unavailable")
	}
	if !validPlayerMode(mode) || page < 1 || page > 200 || len(query) > 64 {
		return PublicPlayersPage{}, fmt.Errorf("invalid player search")
	}
	if id, parseErr := strconv.ParseInt(strings.TrimSpace(query), 10, 64); parseErr == nil && id > 0 {
		profile, err := s.ResolvePublicPlayer(ctx, strings.TrimSpace(query), mode)
		if err != nil {
			return PublicPlayersPage{}, err
		}
		return PublicPlayersPage{Items: []store.OsuPublicProfile{profile}}, nil
	}
	result, err := s.official.publicPlayers(ctx, mode, strings.TrimSpace(query), page)
	if err == nil {
		s.official.retainPlayers(ctx, mode, result.Items)
		return result, nil
	}
	if s.official.playerIndex != nil {
		items, cacheErr := s.official.playerIndex.ListIndexedOsuPlayers(ctx, mode, query, page)
		if cacheErr == nil && len(items) > 0 {
			next := 0
			if len(items) > 50 {
				items = items[:50]
				next = page + 1
			}
			return PublicPlayersPage{Items: items, NextPage: next, Cached: true}, nil
		}
	}
	return PublicPlayersPage{}, err
}

func validPlayerMode(mode string) bool {
	return mode == "osu" || mode == "taiko" || mode == "fruits" || mode == "mania"
}

func (a *officialAdapter) publicPlayers(ctx context.Context, mode, query string, page int) (PublicPlayersPage, error) {
	token, err := a.accessToken(ctx)
	if err != nil {
		return PublicPlayersPage{}, err
	}
	path := "/api/v2/rankings/" + mode + "/performance"
	params := url.Values{"cursor[page]": {strconv.Itoa(page)}}
	if query != "" {
		path = "/api/v2/search"
		params = url.Values{"mode": {"user"}, "query": {query}, "page": {strconv.Itoa(page)}}
	}
	body, err := a.client.get(ctx, path, params, "Bearer "+token)
	if err != nil {
		return PublicPlayersPage{}, err
	}
	result := PublicPlayersPage{Items: []store.OsuPublicProfile{}}
	if query != "" {
		var response struct {
			User *struct {
				Data  []officialUser `json:"data"`
				Total int            `json:"total"`
			} `json:"user"`
		}
		if err = json.Unmarshal(body, &response); err != nil {
			return result, err
		}
		if response.User == nil || response.User.Data == nil {
			return result, fmt.Errorf("invalid player search response")
		}
		for _, u := range response.User.Data {
			if u.ID > 0 && u.Username != "" {
				result.Items = append(result.Items, store.OsuPublicProfile{OsuUserID: int64(u.ID), OsuUsername: u.Username, AvatarURL: absolutePreviewURL(u.AvatarURL), CountryCode: u.CountryCode, RecentReplays: []store.OsuPublicReplay{}})
			}
		}
		// osu! search returns 20 users per page.
		if page*20 < response.User.Total && len(response.User.Data) > 0 {
			result.NextPage = page + 1
		}
	} else {
		var response struct {
			Ranking []struct {
				User       officialUser `json:"user"`
				PP         *float64     `json:"pp"`
				GlobalRank *int64       `json:"global_rank"`
				PlayCount  int64        `json:"play_count"`
				PlayTime   int64        `json:"play_time"`
			} `json:"ranking"`
			Cursor *struct {
				Page int `json:"page"`
			} `json:"cursor"`
		}
		if err = json.Unmarshal(body, &response); err != nil {
			return result, err
		}
		if response.Ranking == nil {
			return result, fmt.Errorf("invalid ranking response")
		}
		for _, r := range response.Ranking {
			u := r.User
			if u.ID > 0 && u.Username != "" {
				result.Items = append(result.Items, store.OsuPublicProfile{OsuUserID: int64(u.ID), OsuUsername: u.Username, AvatarURL: absolutePreviewURL(u.AvatarURL), CountryCode: u.CountryCode, GlobalRank: r.GlobalRank, PerformancePoints: r.PP, PlayCount: r.PlayCount, PlayTimeSeconds: r.PlayTime, RecentReplays: []store.OsuPublicReplay{}})
			}
		}
		if response.Cursor != nil && response.Cursor.Page > page && response.Cursor.Page <= 200 {
			result.NextPage = response.Cursor.Page
		}
	}
	return result, nil
}

func (a *officialAdapter) retainPlayers(ctx context.Context, mode string, items []store.OsuPublicProfile) {
	if a.playerIndex == nil {
		return
	}
	saveCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := a.playerIndex.SaveIndexedOsuPlayers(saveCtx, mode, items); err != nil {
		log.Print("osu public player index update failed")
	}
}

// Crawling uses provider cursors and the shared rate limiter; restarts resume
// durable progress. It never requests every player's score history in bulk.
func (s *Server) RunPlayerIndexer(ctx context.Context) {
	if s == nil || s.official == nil || !s.official.configured() || s.official.playerIndex == nil {
		return
	}
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		jobCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		mode, page, err := s.official.playerIndex.ClaimOsuPlayerIndexPage(jobCtx)
		if err == nil {
			result, fetchErr := s.official.publicPlayers(jobCtx, mode, "", page)
			if fetchErr == nil {
				if saveErr := s.official.playerIndex.SaveIndexedOsuPlayers(jobCtx, mode, result.Items); saveErr == nil {
					_ = s.official.playerIndex.FinishOsuPlayerIndexPage(jobCtx, mode, page, result.NextPage)
				}
			}
		}
		cancel()
		scoreCtx, scoreCancel := context.WithTimeout(ctx, 20*time.Second)
		id, scoreMode, claimErr := s.official.playerIndex.ClaimOsuScoreIndexPlayer(scoreCtx)
		if claimErr == nil {
			result, scoreErr := s.GetPublicUserScores(scoreCtx, id, scoreMode)
			if scoreErr == nil && (result.Coverage.Best.Status == "available" || result.Coverage.Best.Status == "page_limit") && (result.Coverage.Recent.Status == "available" || result.Coverage.Recent.Status == "page_limit") {
				if s.retainPublicScores(scoreCtx, result.Scores) == nil {
					_ = s.official.playerIndex.FinishOsuScoreIndexPlayer(scoreCtx, id, scoreMode)
				}
			}
		}
		scoreCancel()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (s *Server) ResolvePublicPlayer(ctx context.Context, identifier, mode string) (store.OsuPublicProfile, error) {
	if s == nil || s.official == nil || !validPlayerMode(mode) {
		return store.OsuPublicProfile{}, fmt.Errorf("official profile unavailable")
	}
	key := osuv1.OfficialUserLookupKey_OFFICIAL_USER_LOOKUP_KEY_USERNAME
	if id, err := strconv.ParseUint(identifier, 10, 64); err == nil && id > 0 {
		key = osuv1.OfficialUserLookupKey_OFFICIAL_USER_LOOKUP_KEY_ID
	}
	ruleset := map[string]osuv1.Ruleset{"osu": 1, "taiko": 2, "fruits": 3, "mania": 4}[mode]
	user, err := s.official.userProfile(ctx, &osuv1.GetOfficialUserProfileRequest{Identifier: identifier, LookupKey: key, Ruleset: ruleset})
	if err != nil {
		return store.OsuPublicProfile{}, err
	}
	return publicProfileFromOfficial(user), nil
}

func publicProfileFromOfficial(user *osuv1.OfficialUserProfile) store.OsuPublicProfile {
	p := store.OsuPublicProfile{OsuUserID: int64(user.UserId), OsuUsername: user.Username, CountryCode: user.CountryCode, AvatarURL: user.AvatarUrl, RecentReplays: []store.OsuPublicReplay{}}
	if stats := user.Statistics; stats != nil {
		p.PerformancePoints = &stats.Pp
		if stats.GlobalRank > 0 {
			rank := int64(stats.GlobalRank)
			p.GlobalRank = &rank
		}
		p.PlayCount = int64(stats.PlayCount)
		p.PlayTimeSeconds = int64(stats.PlayTimeSeconds)
	}
	return p
}
