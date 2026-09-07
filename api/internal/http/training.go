package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

type trainingStore interface {
	GetUserByUploadToken(context.Context, string) (store.AuthUser, error)
	GetUserBySession(context.Context, string) (store.AuthUser, error)
	SaveTraining(context.Context, int64, []store.TrainingSession) error
	ListTraining(context.Context, string, int64, int, string) ([]store.TrainingSession, bool, error)
	SetTrainingVisibility(context.Context, int64, string, string) (bool, error)
}
type trainingHandler struct {
	store         trainingStore
	allowedOrigin string
}

func (h trainingHandler) register(mux *http.ServeMux, origin string) {
	h.allowedOrigin = origin
	mux.Handle("/api/osu/v1/training/sessions", withCORS(origin, http.HandlerFunc(h.upload)))
	mux.Handle("/api/osu/v1/training/profiles/", withCORS(origin, http.HandlerFunc(h.profile)))
	mux.Handle("/api/osu/v1/training/me", withAuthCORS(origin, http.HandlerFunc(h.mine)))
	mux.Handle("/api/osu/v1/training/visibility", withAuthCORS(origin, http.HandlerFunc(h.visibility)))
}
func (h trainingHandler) owner(r *http.Request) (store.AuthUser, error) {
	if r.Header.Get("Authorization") != "" {
		return h.store.GetUserByUploadToken(r.Context(), r.Header.Get("Authorization"))
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return store.AuthUser{}, err
	}
	return h.store.GetUserBySession(r.Context(), cookie.Value)
}
func (h trainingHandler) upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	user, err := h.store.GetUserByUploadToken(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "unauthenticated", 401)
		return
	}
	var input struct {
		SchemaVersion int                     `json:"schemaVersion"`
		Sessions      []store.TrainingSession `json:"sessions"`
	}
	if !decodeTraining(w, r, &input) {
		return
	}
	if input.SchemaVersion != 1 || len(input.Sessions) == 0 || len(input.Sessions) > 20 {
		http.Error(w, "Expected 1 to 20 sessions using schema version 1", 400)
		return
	}
	seen := map[string]bool{}
	for _, session := range input.Sessions {
		if session.Validate(time.Now()) != nil || seen[session.ID] {
			http.Error(w, "invalid or duplicate training session", 400)
			return
		}
		seen[session.ID] = true
	}
	if err := h.store.SaveTraining(r.Context(), user.UserID, input.Sessions); err != nil {
		if errors.Is(err, store.ErrTrainingConflict) {
			http.Error(w, err.Error(), 409)
		} else {
			http.Error(w, "Could not save training", 500)
		}
		return
	}
	writeJSON(w, 200, map[string]int{"accepted": len(input.Sessions)})
}
func decodeTraining(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		http.Error(w, "invalid training payload", 400)
		return false
	}
	if ensureJSONEOF(decoder) != nil {
		http.Error(w, "invalid training payload", 400)
		return false
	}
	return true
}
func (h trainingHandler) profile(w http.ResponseWriter, r *http.Request) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/osu/v1/training/profiles/")
	if handle == "" || len(handle) > 100 || strings.Contains(handle, "/") {
		http.NotFound(w, r)
		return
	}
	h.list(w, r, handle, 0)
}
func (h trainingHandler) mine(w http.ResponseWriter, r *http.Request) {
	user, err := h.owner(r)
	if err != nil {
		http.Error(w, "unauthenticated", 401)
		return
	}
	h.list(w, r, "", user.UserID)
}
func (h trainingHandler) list(w http.ResponseWriter, r *http.Request, handle string, ownerID int64) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 365 {
			http.Error(w, "days must be between 1 and 365", 400)
			return
		}
		days = parsed
	}
	mode := r.URL.Query().Get("mode")
	if mode != "" && !store.ValidTrainingMode(mode) {
		http.Error(w, "unknown training mode", 400)
		return
	}
	sessions, truncated, err := h.store.ListTraining(r.Context(), handle, ownerID, days, mode)
	if err != nil {
		http.Error(w, "Could not load training", 500)
		return
	}
	writeJSON(w, 200, store.SummarizeTraining(sessions, days, truncated))
}
func (h trainingHandler) visibility(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	// Cookie requests must be same-origin; bearer clients are not susceptible to cookie CSRF.
	if r.Header.Get("Authorization") == "" && r.Header.Get("Sec-Fetch-Site") != "same-origin" && (h.allowedOrigin == "" || r.Header.Get("Origin") != h.allowedOrigin) {
		http.Error(w, "same-origin request required", 403)
		return
	}
	user, err := h.owner(r)
	if err != nil {
		http.Error(w, "unauthenticated", 401)
		return
	}
	var input struct {
		ID         string `json:"id"`
		Visibility string `json:"visibility"`
	}
	if !decodeTraining(w, r, &input) {
		return
	}
	if input.Visibility != "private" && input.Visibility != "public" || !store.ValidTrainingID(input.ID) {
		http.Error(w, "invalid visibility", 400)
		return
	}
	found, err := h.store.SetTrainingVisibility(r.Context(), user.UserID, input.ID, input.Visibility)
	if err != nil {
		http.Error(w, "Could not update training", 500)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, 200, map[string]bool{"updated": true})
}
