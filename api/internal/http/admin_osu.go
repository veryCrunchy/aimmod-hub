package httpserver

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

type adminOsuStore interface {
	GetUserBySession(context.Context, string) (store.AuthUser, error)
	GetAdminOsuOverview(context.Context, store.AdminOsuFilter) (store.AdminOsuOverview, error)
	GetAdminOsuPlayers(context.Context, store.AdminOsuRecordFilter) (store.AdminOsuPlayers, error)
	GetAdminOsuBeatmaps(context.Context, store.AdminOsuRecordFilter) (store.AdminOsuBeatmaps, error)
}

type adminOsuProviders interface {
	GetProviderStatus(context.Context, *connect.Request[osuv1.GetProviderStatusRequest]) (*connect.Response[osuv1.GetProviderStatusResponse], error)
	GetSkinProviderStatus(context.Context, *connect.Request[osuv1.GetSkinProviderStatusRequest]) (*connect.Response[osuv1.GetSkinProviderStatusResponse], error)
}

type adminOsuHandler struct {
	store     adminOsuStore
	isAdmin   func(store.AuthUser) bool
	providers adminOsuProviders
}

type adminOsuProvider struct {
	Name        string `json:"name"`
	Configured  bool   `json:"configured"`
	Available   bool   `json:"available"`
	CheckedAt   string `json:"checkedAt"`
	BrowserOnly bool   `json:"browserOnly"`
}

func (h *adminOsuHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.store.GetUserBySession(r.Context(), cookie.Value)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.isAdmin(user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if r.URL.Path == "/admin/osu/providers" {
		if h.providers == nil {
			http.Error(w, "provider service unavailable", http.StatusServiceUnavailable)
			return
		}
		maps, err := h.providers.GetProviderStatus(ctx, connect.NewRequest(&osuv1.GetProviderStatusRequest{}))
		if err != nil {
			http.Error(w, "could not check beatmap providers", http.StatusBadGateway)
			return
		}
		skins, err := h.providers.GetSkinProviderStatus(ctx, connect.NewRequest(&osuv1.GetSkinProviderStatusRequest{}))
		if err != nil {
			http.Error(w, "could not check skin providers", http.StatusBadGateway)
			return
		}
		items := make([]adminOsuProvider, 0, 4)
		for _, p := range maps.Msg.Providers {
			name := "osu!Collector"
			if p.Provider == osuv1.Provider_PROVIDER_OSU_OFFICIAL {
				name = "osu! official"
			}
			items = append(items, adminOsuProvider{Name: name, Configured: p.Configured, Available: p.Available, CheckedAt: p.CheckedAtIso})
		}
		for _, p := range skins.Msg.Providers {
			name := "skins.osuck.net"
			if p.Provider == osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS {
				name = "osuskins.net"
			}
			browserOnly := p.Provider == osuv1.SkinProvider_SKIN_PROVIDER_OSUCK
			items = append(items, adminOsuProvider{Name: name, Configured: !browserOnly, Available: p.Available && !browserOnly, CheckedAt: p.CheckedAtIso, BrowserOnly: browserOnly})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	if r.URL.Path == "/admin/osu/players" || r.URL.Path == "/admin/osu/beatmaps" {
		h.handleRecords(w, r.WithContext(ctx))
		return
	}
	if r.URL.Path != "/admin/osu/overview" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	filter := store.AdminOsuFilter{Search: strings.TrimSpace(q.Get("q")), Visibility: q.Get("visibility"), Status: q.Get("status"), Limit: 25}
	filter.DifficultyKey = q.Get("difficultyKey")
	if len(filter.DifficultyKey) > 256 {
		http.Error(w, "invalid difficulty key", http.StatusBadRequest)
		return
	}
	if q.Get("userId") != "" {
		filter.UserID, err = strconv.ParseInt(q.Get("userId"), 10, 64)
		if err != nil || filter.UserID <= 0 {
			http.Error(w, "invalid user id", http.StatusBadRequest)
			return
		}
	}
	if len(filter.Search) > 200 || !adminOsuChoice(filter.Visibility, "", "public", "unlisted", "private") || !adminOsuChoice(filter.Status, "", "uploaded", "pending", "none") {
		http.Error(w, "invalid filter", http.StatusBadRequest)
		return
	}
	if raw := q.Get("offset"); raw != "" {
		filter.Offset, err = strconv.Atoi(raw)
		if err != nil || filter.Offset < 0 || filter.Offset > 1000000 {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		}
	}
	result, err := h.store.GetAdminOsuOverview(ctx, filter)
	if err != nil {
		http.Error(w, "could not load osu administration", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *adminOsuHandler) handleRecords(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.AdminOsuRecordFilter{Search: strings.TrimSpace(q.Get("q")), Kind: q.Get("kind"), Limit: 25}
	allowed := []string{"", "synced", "unsynced"}
	if r.URL.Path == "/admin/osu/beatmaps" {
		allowed = []string{"", "online", "local"}
	}
	if len(f.Search) > 200 || !adminOsuChoice(f.Kind, allowed...) {
		http.Error(w, "invalid filter", http.StatusBadRequest)
		return
	}
	var err error
	if q.Get("offset") != "" {
		f.Offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil || f.Offset < 0 || f.Offset > 1000000 {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		}
	}
	var result any
	if r.URL.Path == "/admin/osu/players" {
		result, err = h.store.GetAdminOsuPlayers(r.Context(), f)
	} else {
		result, err = h.store.GetAdminOsuBeatmaps(r.Context(), f)
	}
	if err != nil {
		http.Error(w, "could not load admin records", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func adminOsuChoice(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
