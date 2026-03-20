package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

var liveActivityWebsocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

type liveActivitySocketMessage struct {
	Type    string                     `json:"type"`
	Payload *store.LiveActivityPayload `json:"payload,omitempty"`
}

func (h *authHandler) registerLiveActivity(mux *http.ServeMux) {
	mux.Handle("/activity/live", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleLiveActivity)))
	mux.Handle("/activity/live/list", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleListLiveActivities)))
	mux.Handle("/activity/live/events", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleLiveActivityEvents)))
	mux.Handle("/activity/live/ws", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleLiveActivityWebSocket)))
}

func (h *authHandler) handleLiveActivity(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetLiveActivity(w, r)
	case http.MethodPost:
		h.handleUpsertLiveActivity(w, r)
	case http.MethodDelete:
		h.handleDeleteLiveActivity(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *authHandler) handleGetLiveActivity(w http.ResponseWriter, r *http.Request) {
	handle := strings.TrimSpace(r.URL.Query().Get("handle"))
	if handle == "" {
		http.Error(w, "handle required", http.StatusBadRequest)
		return
	}

	record, err := h.store.GetLiveActivityByHandle(r.Context(), handle)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusOK, store.LiveActivityRecord{Active: false})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func (h *authHandler) handleUpsertLiveActivity(w http.ResponseWriter, r *http.Request) {
	authUser, err := h.store.GetUserByUploadToken(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var payload store.LiveActivityPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	if err := h.store.UpsertLiveActivity(r.Context(), authUser.UserID, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.events != nil {
		if authUser.ProfileHandle != "" {
			h.events.Publish(authUser.ProfileHandle)
		}
		h.events.PublishLiveFeed()
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *authHandler) handleDeleteLiveActivity(w http.ResponseWriter, r *http.Request) {
	authUser, err := h.store.GetUserByUploadToken(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.store.DeleteLiveActivity(r.Context(), authUser.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.events != nil {
		if authUser.ProfileHandle != "" {
			h.events.Publish(authUser.ProfileHandle)
		}
		h.events.PublishLiveFeed()
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *authHandler) handleListLiveActivities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	items, err := h.store.ListLiveActivities(r.Context(), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"count": len(items),
	})
}

func (h *authHandler) handleLiveActivityEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	if _, err := fmt.Fprintf(w, "event: connected\ndata: {}\n\n"); err != nil {
		return
	}
	flusher.Flush()

	if h.events == nil {
		<-r.Context().Done()
		return
	}

	ch, unsub := h.events.SubscribeLiveFeed()
	defer unsub()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ch:
			if _, err := fmt.Fprintf(w, "event: live_activity_updated\ndata: {}\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *authHandler) handleLiveActivityWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	authUser, err := h.store.GetUserByUploadToken(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	conn, err := liveActivityWebsocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(_ string) error {
		return conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	})

	for {
		var message liveActivitySocketMessage
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		switch strings.TrimSpace(strings.ToLower(message.Type)) {
		case "update":
			if message.Payload == nil {
				continue
			}
			if err := h.store.UpsertLiveActivity(r.Context(), authUser.UserID, *message.Payload); err != nil {
				continue
			}
			if h.events != nil {
				if authUser.ProfileHandle != "" {
					h.events.Publish(authUser.ProfileHandle)
				}
				h.events.PublishLiveFeed()
			}
		case "clear":
			if err := h.store.DeleteLiveActivity(r.Context(), authUser.UserID); err != nil {
				continue
			}
			if h.events != nil {
				if authUser.ProfileHandle != "" {
					h.events.Publish(authUser.ProfileHandle)
				}
				h.events.PublishLiveFeed()
			}
		default:
			continue
		}
	}
}
