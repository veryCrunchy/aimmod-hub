package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/veryCrunchy/aimmod-hub/api/internal/kovaaksbenchmarks"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament"
	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament/tournamentapi"
	"github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/tournament/v1/tournamentv1connect"
)

type tournamentUsers interface {
	GetUserBySession(context.Context, string) (store.AuthUser, error)
	GetUserByUploadToken(context.Context, string) (store.AuthUser, error)
}

// tournamentAuthenticator accepts the in-game client's bearer upload token or
// the website's session cookie. Cookie requests from another origin are
// treated as anonymous, so a third-party page can't act for a signed-in user.
func tournamentAuthenticator(users tournamentUsers, allowedOrigin string, isAdmin func(store.AuthUser) bool) tournamentapi.Authenticator {
	return func(ctx context.Context, h http.Header) (tournament.Actor, error) {
		var user store.AuthUser
		var err error
		if auth := strings.TrimSpace(h.Get("Authorization")); auth != "" {
			user, err = users.GetUserByUploadToken(ctx, auth)
		} else {
			r := &http.Request{Header: h}
			cookie, cerr := r.Cookie(sessionCookieName)
			if cerr != nil || cookie.Value == "" {
				return tournament.Actor{}, nil
			}
			if origin := h.Get("Origin"); origin != "" && origin != allowedOrigin && h.Get("Sec-Fetch-Site") != "same-origin" {
				return tournament.Actor{}, nil
			}
			user, err = users.GetUserBySession(ctx, cookie.Value)
		}
		if err != nil {
			return tournament.Actor{}, err
		}
		display := user.ProfileDisplayName
		if display == "" {
			display = user.DisplayName
		}
		return tournament.Actor{
			UserRef: tournament.UserRef{UserID: user.UserID, ExternalID: user.UserExternalID, Handle: user.ProfileHandle,
				DisplayName: display, AvatarURL: user.AvatarURL, SteamID: user.SteamID},
			HubAdmin: isAdmin(user),
			Verified: user.ProfileVerified,
		}, nil
	}
}

// benchmarkRanks rates players by their overall rank on a KovaaK's benchmark,
// with the sum of scenario ranks as the tie-break.
type benchmarkRanks struct{ client *kovaaksbenchmarks.Client }

func (b benchmarkRanks) Rank(ctx context.Context, id uint32, steamID string) (float64, string, bool, error) {
	if b.client == nil {
		return 0, "", false, errors.New("benchmarks unavailable")
	}
	detail, err := b.client.GetBenchmarkDetail(ctx, id, steamID)
	if err != nil || detail == nil {
		return 0, "", false, err
	}
	sum := 0.0
	for _, c := range detail.Categories {
		for _, s := range c.Scenarios {
			sum += float64(s.ScenarioRank)
		}
	}
	label := ""
	if int(detail.OverallRank) < len(detail.Ranks) {
		label = detail.Ranks[detail.OverallRank].RankName
	}
	return float64(detail.OverallRank)*1e6 + sum, label, detail.OverallRank > 0 || sum > 0, nil
}

func registerTournaments(mux *http.ServeMux, cfg Config, st *store.Store, auth *authHandler, benchmarks *kovaaksbenchmarks.Client) {
	if st == nil {
		return
	}
	server := tournamentapi.New(tournamentapi.Config{
		Store:      st,
		Auth:       tournamentAuthenticator(st, cfg.AllowedWebOrigin, auth.isAdminUser),
		Benchmarks: benchmarkRanks{client: benchmarks},
		Creators:   tournamentapi.CreatorPolicy(cfg.TournamentCreators),
		IsVerified: func(a tournament.Actor) bool { return a.Verified },
	})
	path, handler := tournamentv1connect.NewTournamentServiceHandler(server)
	mux.Handle(path, withCORS(cfg.AllowedWebOrigin, handler))
	var media tournamentapi.Media
	if auth.media != nil {
		media = auth.media
	}
	replays := withCORS(cfg.AllowedWebOrigin, server.ReplayHandler(media))
	mux.Handle(tournamentapi.ReplayPath, replays)
	mux.Handle(tournamentapi.ReplayPath+"/", replays)
}
