package httpserver

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

type publicScoreStore interface {
	GetOsuPublicProfile(context.Context, string, int) (store.OsuPublicProfile, error)
	GetOsuPublicProfileByOsuUserID(context.Context, int64, int) (store.OsuPublicProfile, error)
}
type publicScoreProvider interface {
	GetPublicUserScores(context.Context, int64, string) (osuservice.OfficialScoresResult, error)
	GetPublicScore(context.Context, int64) (osuservice.OfficialScoreDetail, error)
	GetPublicScoreProfile(context.Context, int64, string) (store.OsuPublicProfile, error)
}
type osuProfileScoresHandler struct {
	store    publicScoreStore
	official publicScoreProvider
}

func newOsuProfileScoresHandler(st publicScoreStore, official publicScoreProvider) *osuProfileScoresHandler {
	return &osuProfileScoresHandler{store: st, official: official}
}
func (h *osuProfileScoresHandler) register(mux *http.ServeMux, origin string) {
	mux.Handle("/api/osu/v1/profile-scores/", withCORS(origin, http.HandlerFunc(h.profileScores)))
	mux.Handle("/api/osu/v1/official-scores/", withCORS(origin, http.HandlerFunc(h.scoreDetail)))
}

type profileScoresResponse struct {
	Profile  store.OsuPublicProfile           `json:"profile"`
	Items    []osuservice.PublicScoreItem     `json:"items"`
	Coverage osuservice.OfficialScoreCoverage `json:"coverage"`
	Local    localScoreCoverage               `json:"local"`
	HasMore  bool                             `json:"hasMore"`
}
type localScoreCoverage struct {
	Returned int  `json:"returned"`
	HasMore  bool `json:"hasMore"`
}

func (h *osuProfileScoresHandler) profileScores(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	handle := strings.TrimPrefix(r.URL.Path, "/api/osu/v1/profile-scores/")
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "osu"
	}
	if handle == "" || len(handle) > 128 || strings.Contains(handle, "/") || (mode != "osu" && mode != "taiko" && mode != "fruits" && mode != "mania") {
		http.Error(w, "invalid profile or mode", http.StatusBadRequest)
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			http.Error(w, "limit must be 1 through 100", http.StatusBadRequest)
			return
		}
		limit = value
	}
	var profile store.OsuPublicProfile
	var err error
	id, parseErr := strconv.ParseInt(handle, 10, 64)
	if parseErr == nil && id > 0 {
		profile, err = h.store.GetOsuPublicProfileByOsuUserID(r.Context(), id, 100)
	} else {
		profile, err = h.store.GetOsuPublicProfile(r.Context(), handle, 100)
	}
	if err != nil {
		if h.official == nil {
			http.Error(w, "official API unavailable", 503)
			return
		}
		if resolver, ok := h.official.(interface {
			ResolvePublicPlayer(context.Context, string, string) (store.OsuPublicProfile, error)
		}); ok {
			profile, err = resolver.ResolvePublicPlayer(r.Context(), handle, mode)
		} else if parseErr == nil && id > 0 {
			profile, err = h.official.GetPublicScoreProfile(r.Context(), id, mode)
		}
		if err != nil {
			http.Error(w, "official profile unavailable", 502)
			return
		}
	}
	local := make([]store.OsuPublicReplay, 0, len(profile.RecentReplays))
	for _, replay := range profile.RecentReplays {
		if replay.Ruleset == mode && replay.Visibility == store.OsuVisibilityPublic {
			local = append(local, replay)
		}
	}
	result := osuservice.OfficialScoresResult{Coverage: osuservice.OfficialScoreCoverage{Best: osuservice.ScoreCoverage{Status: "not_configured"}, Recent: osuservice.ScoreCoverage{Status: "not_configured"}}}
	if h.official != nil {
		result, err = h.official.GetPublicUserScores(r.Context(), profile.OsuUserID, mode)
		if err != nil {
			if r.Context().Err() != nil {
				return
			}
			http.Error(w, "official scores unavailable", http.StatusBadGateway)
			return
		}
	}
	if index, ok := h.official.(interface {
		IndexPublicScores(context.Context, []osuservice.OfficialPublicScore) error
	}); ok {
		_ = index.IndexPublicScores(r.Context(), result.Scores)
	}
	items := osuservice.MergePublicScores(local, result.Scores)
	for i := range items {
		if items[i].Source == "official" {
			items[i].HubHandle = profile.HubHandle
			items[i].OsuUsername = profile.OsuUsername
			items[i].AvatarURL = profile.AvatarURL
			items[i].CountryCode = profile.CountryCode
		}
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	enrichSharedScoreItems(r.Context(), h.official, items)
	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, profileScoresResponse{Profile: profile, Items: items, Coverage: result.Coverage,
		Local: localScoreCoverage{Returned: len(local), HasMore: profile.SharedReplayCount > len(profile.RecentReplays)}, HasMore: hasMore})
}

func (h *osuProfileScoresHandler) scoreDetail(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/replay") {
		h.scoreReplay(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/osu/v1/official-scores/"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid score ID", http.StatusBadRequest)
		return
	}
	if h.official == nil {
		writeJSON(w, http.StatusServiceUnavailable, osuservice.OfficialScoreDetail{Status: "not_configured"})
		return
	}
	result, err := h.official.GetPublicScore(r.Context(), id)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		http.Error(w, "official score unavailable", http.StatusBadGateway)
		return
	}
	status := http.StatusOK
	if result.Status == "not_found" {
		status = http.StatusNotFound
	} else if result.Status != "available" {
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, status, result)
}

func (h *osuProfileScoresHandler) scoreReplay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/osu/v1/official-scores/"), "/replay")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid score ID", http.StatusBadRequest)
		return
	}
	provider, ok := h.official.(interface {
		DownloadPublicReplay(context.Context, int64) (osuservice.OfficialReplayDownload, error)
	})
	if !ok {
		writeJSON(w, 503, map[string]string{"status": "not_configured"})
		return
	}
	result, err := provider.DownloadPublicReplay(r.Context(), id)
	if result.Body != nil {
		defer result.Body.Close()
	}
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		writeJSON(w, 502, map[string]string{"status": "unavailable"})
		return
	}
	if result.Status != "available" || result.Body == nil {
		status := map[string]int{"not_configured": 503, "authentication_failed": 401, "permission_denied": 403, "not_found": 404, "rate_limited": 429, "too_large": 413}[result.Status]
		if status == 0 {
			status = 502
		}
		writeJSON(w, status, map[string]string{"status": result.Status})
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `attachment; filename="osu-`+strconv.FormatInt(id, 10)+`.osr"`)
	w.Header().Set("Content-Length", strconv.FormatInt(result.Size, 10))
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Now().Add(45 * time.Second))
	defer controller.SetWriteDeadline(time.Time{})
	_, _ = io.Copy(w, result.Body)
}
