package httpserver

import (
	"context"
	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"net/http"
	"strconv"
	"strings"
)

type osuPlayersProvider interface {
	ListPublicPlayers(context.Context, string, string, int) (osuservice.PublicPlayersPage, error)
}
type osuPlayersHandler struct{ provider osuPlayersProvider }

func newOsuPlayersHandler(p osuPlayersProvider) *osuPlayersHandler { return &osuPlayersHandler{p} }
func (h *osuPlayersHandler) register(mux *http.ServeMux, origin string) {
	mux.Handle("/api/osu/v1/players", withCORS(origin, http.HandlerFunc(h.list)))
}
func (h *osuPlayersHandler) list(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "osu"
	}
	page := 1
	var err error
	if raw := r.URL.Query().Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if err != nil || page < 1 || page > 200 || len(query) > 64 || (mode != "osu" && mode != "taiko" && mode != "fruits" && mode != "mania") {
		http.Error(w, "invalid player search", 400)
		return
	}
	if h.provider == nil {
		http.Error(w, "players unavailable", 503)
		return
	}
	result, err := h.provider.ListPublicPlayers(r.Context(), mode, query, page)
	if err != nil {
		http.Error(w, "players temporarily unavailable", 502)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, 200, result)
}
