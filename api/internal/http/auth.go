package httpserver

import (
	"bytes"
	"encoding/csv"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/media"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

const (
	sessionCookieName   = "aimmod_hub_session"
	discordAuthBaseURL  = "https://discord.com/oauth2/authorize"
	discordTokenURL     = "https://discord.com/api/v10/oauth2/token"
	discordUserInfoURL  = "https://discord.com/api/v10/users/@me"
	defaultReturnToPath = "/account"
)

type authHandler struct {
	cfg    Config
	store  *store.Store
	media  media.Storage
	client *http.Client
}

type discordTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type discordUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Avatar     string `json:"avatar"`
}

type sessionResponse struct {
	Authenticated bool                      `json:"authenticated"`
	IsAdmin       bool                      `json:"isAdmin"`
	AdminConfigured bool                    `json:"adminConfigured"`
	AdminReason     string                  `json:"adminReason,omitempty"`
	User          *store.AuthUser           `json:"user,omitempty"`
	Tokens        []store.UploadTokenRecord `json:"tokens,omitempty"`
}

type createTokenRequest struct {
	Label string `json:"label"`
}

type createTokenResponse struct {
	Token  string                  `json:"token"`
	Record store.UploadTokenRecord `json:"record"`
}

type revokeTokenRequest struct {
	ID int64 `json:"id"`
}

type deviceStartRequest struct {
	Label string `json:"label"`
}

type deviceStartResponse struct {
	DeviceCode               string `json:"deviceCode"`
	UserCode                 string `json:"userCode"`
	VerificationURI          string `json:"verificationUri"`
	VerificationURIComplete  string `json:"verificationUriComplete"`
	ExpiresIn                int64  `json:"expiresIn"`
	Interval                 int64  `json:"interval"`
}

type devicePollRequest struct {
	DeviceCode string `json:"deviceCode"`
}

type devicePollResponse struct {
	Status      string          `json:"status"`
	ExpiresAt   time.Time       `json:"expiresAt"`
	User        *store.AuthUser `json:"user,omitempty"`
	UploadToken string          `json:"uploadToken,omitempty"`
}

type deviceApproveRequest struct {
	UserCode string `json:"userCode"`
}

func requestBaseURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

func replayMediaURL(r *http.Request, publicRunID string, quality string) string {
	baseURL := strings.TrimRight(requestBaseURL(r), "/")
	if baseURL == "" {
		return "/media/replays/" + publicRunID + ".mp4?quality=" + url.QueryEscape(quality)
	}
	return fmt.Sprintf("%s/media/replays/%s.mp4?quality=%s", baseURL, publicRunID, url.QueryEscape(quality))
}

func newAuthHandler(cfg Config, store *store.Store) *authHandler {
	mediaStorage, err := media.New(media.Config{
		Backend:        cfg.MediaBackend,
		LocalDir:       cfg.MediaDir,
		S3Bucket:       cfg.S3Bucket,
		S3Region:       cfg.S3Region,
		S3Endpoint:     cfg.S3Endpoint,
		S3AccessKeyID:  cfg.S3AccessKeyID,
		S3SecretAccess: cfg.S3SecretAccessKey,
		S3ForcePath:    cfg.S3ForcePathStyle,
	})
	if err != nil {
		panic(err)
	}
	return &authHandler{
		cfg:   cfg,
		store: store,
		media: mediaStorage,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (h *authHandler) register(mux *http.ServeMux) {
	mux.Handle("/auth/discord/start", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleDiscordStart)))
	mux.Handle("/auth/discord/callback", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleDiscordCallback)))
	mux.Handle("/auth/session", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleSession)))
	mux.Handle("/auth/logout", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleLogout)))
	mux.Handle("/auth/upload-tokens", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleUploadTokens)))
	mux.Handle("/auth/upload-tokens/revoke", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleRevokeUploadToken)))
	mux.Handle("/auth/device/start", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleDeviceStart)))
	mux.Handle("/auth/device/poll", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleDevicePoll)))
	mux.Handle("/auth/device/approve", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleDeviceApprove)))
	mux.Handle("/admin/overview", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleAdminOverview)))
	mux.Handle("/admin/user", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleAdminUserDetail)))
	mux.Handle("/admin/actions/reclassify", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleAdminReclassify)))
	mux.Handle("/admin/actions/repair-metrics", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleAdminRepairMetrics)))
	mux.Handle("/admin/actions/clear-failures", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleAdminClearFailures)))
	mux.Handle("/admin/actions/reclassify-user", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleAdminReclassifyUser)))
	mux.Handle("/admin/actions/repair-user-metrics", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleAdminRepairUserMetrics)))
	mux.Handle("/admin/actions/clear-user-failures", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleAdminClearUserFailures)))
	mux.Handle("/admin/failures/export", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleAdminFailuresExport)))
	mux.Handle("/admin/user/export", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleAdminUserMetricsExport)))
	mux.Handle("/media/replays/upload", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleReplayMediaUpload)))
	mux.Handle("/media/replays/delete", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleReplayMediaDelete)))
	mux.Handle("/media/replays/meta", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleReplayMediaMeta)))
	mux.Handle("/media/replays/", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleReplayMediaStream)))
	mux.Handle("/replays/mouse-path/upload", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleMousePathUpload)))
	mux.Handle("/replays/mouse-path", withAuthCORS(h.cfg.AllowedWebOrigin, http.HandlerFunc(h.handleMousePathMeta)))
}

func (h *authHandler) handleDiscordStart(w http.ResponseWriter, r *http.Request) {
	if h.cfg.DiscordClientID == "" || h.cfg.DiscordClientSecret == "" || h.cfg.DiscordRedirectURI == "" {
		http.Error(w, "Discord auth is not configured", http.StatusServiceUnavailable)
		return
	}

	returnTo := r.URL.Query().Get("return_to")
	if returnTo == "" {
		returnTo = defaultReturnToPath
	}
	state, err := h.store.CreateAuthState(r.Context(), returnTo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	values := url.Values{}
	values.Set("client_id", h.cfg.DiscordClientID)
	values.Set("response_type", "code")
	values.Set("redirect_uri", h.cfg.DiscordRedirectURI)
	values.Set("scope", "identify")
	values.Set("prompt", "consent")
	values.Set("state", state)
	http.Redirect(w, r, discordAuthBaseURL+"?"+values.Encode(), http.StatusFound)
}

func (h *authHandler) handleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	returnTo, err := h.store.ConsumeAuthState(r.Context(), state)
	if err != nil {
		http.Error(w, "invalid auth state", http.StatusBadRequest)
		return
	}

	token, err := h.exchangeDiscordCode(r.Context(), code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	userInfo, err := h.fetchDiscordUser(r.Context(), token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	avatarURL := ""
	if userInfo.Avatar != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png?size=256", userInfo.ID, userInfo.Avatar)
	}
	authUser, err := h.store.UpsertDiscordUser(r.Context(), userInfo.ID, userInfo.Username, userInfo.GlobalName, avatarURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessionID, err := h.store.CreateSession(r.Context(), authUser.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.SessionCookieSecure,
		MaxAge:   60 * 60 * 24 * 30,
	})

	redirectTo := strings.TrimSpace(returnTo)
	if redirectTo == "" {
		redirectTo = defaultReturnToPath
	}
	if strings.HasPrefix(redirectTo, "/") {
		redirectTo = strings.TrimRight(h.cfg.WebAppOrigin, "/") + redirectTo
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (h *authHandler) handleSession(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireSessionUser(w, r)
	if !ok {
		writeJSON(w, http.StatusOK, sessionResponse{
			Authenticated: false,
			IsAdmin:       false,
		})
		return
	}

	tokens, err := h.store.ListUploadTokens(r.Context(), user.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isAdmin, adminReason := h.adminStatus(user)
	user.IsAdmin = isAdmin
	writeJSON(w, http.StatusOK, sessionResponse{
		Authenticated: true,
		IsAdmin:       isAdmin,
		User:          &user,
		Tokens:        tokens,
		AdminConfigured: func() bool {
			if !isAdmin {
				return false
			}
			return strings.TrimSpace(h.cfg.AdminDiscordUserID) != ""
		}(),
		AdminReason: func() string {
			if !isAdmin {
				return ""
			}
			return adminReason
		}(),
	})
}

func (h *authHandler) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	overview, err := h.store.GetAdminOverview(r.Context(), parseAdminDays(r.URL.Query().Get("days")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, overview)
}

func (h *authHandler) handleAdminReclassify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	updated, err := h.store.RepairScenarioTypes(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"updated": updated,
	})
}

func (h *authHandler) handleAdminRepairMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	updated, err := h.store.RepairRunMetrics(r.Context(), "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"updated": updated,
	})
}

func (h *authHandler) handleAdminUserDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	handle := strings.TrimSpace(r.URL.Query().Get("handle"))
	if handle == "" {
		http.Error(w, "handle is required", http.StatusBadRequest)
		return
	}

	detail, err := h.store.GetAdminUserDetail(r.Context(), handle, parseAdminDays(r.URL.Query().Get("days")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *authHandler) handleAdminClearFailures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	count, err := h.store.ClearIngestFailures(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"cleared": count,
	})
}

func (h *authHandler) handleReplayMediaUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authUser, err := h.store.GetUserByUploadToken(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("sessionId"))
	if sessionID == "" {
		http.Error(w, "sessionId is required", http.StatusBadRequest)
		return
	}
	quality := normalizeMediaQuality(r.URL.Query().Get("quality"))
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "video/mp4"
	}
	if contentType != "video/mp4" {
		http.Error(w, "only video/mp4 is supported right now", http.StatusBadRequest)
		return
	}

	target, err := h.store.GetReplayMediaUploadTarget(r.Context(), authUser.UserID, sessionID)
	if err != nil {
		http.Error(w, "run not found for this account", http.StatusNotFound)
		return
	}

	storageKey := strings.TrimPrefix(fmt.Sprintf("replays/%s/%s.mp4", target.PublicRunID, quality), "/")
	buffer, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	written := int64(len(buffer))
	if written <= 0 {
		http.Error(w, "empty replay media upload", http.StatusBadRequest)
		return
	}
	if err := h.media.Put(r.Context(), storageKey, contentType, bytes.NewReader(buffer), written); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.store.UpsertReplayMediaAsset(r.Context(), target.StoredSessionID, quality, storageKey, contentType, written); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":       true,
		"runId":    target.PublicRunID,
		"quality":  quality,
		"mediaUrl": replayMediaURL(r, target.PublicRunID, quality),
		"byteSize": written,
	})
}

func (h *authHandler) handleReplayMediaDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authUser, ok := h.requireSessionUser(w, r)
	if !ok {
		return
	}
	runID := strings.TrimSpace(r.URL.Query().Get("runId"))
	if runID == "" {
		http.Error(w, "runId is required", http.StatusBadRequest)
		return
	}
	storageKey, err := h.store.DeleteReplayMediaAssetForUser(r.Context(), authUser.UserID, runID)
	if err != nil {
		http.Error(w, "replay media not found for this account", http.StatusNotFound)
		return
	}
	if storageKey != "" {
		if err := h.media.Delete(r.Context(), storageKey); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *authHandler) handleReplayMediaMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	runID := strings.TrimSpace(r.URL.Query().Get("runId"))
	if runID == "" {
		http.Error(w, "runId is required", http.StatusBadRequest)
		return
	}
	meta, err := h.store.GetReplayMediaMeta(r.Context(), runID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"available":   true,
		"runId":       meta.PublicRunID,
		"quality":     meta.Quality,
		"contentType": meta.ContentType,
		"byteSize":    meta.ByteSize,
		"mediaUrl":    replayMediaURL(r, meta.PublicRunID, meta.Quality),
	})
}

func (h *authHandler) handleMousePathUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authUser, err := h.store.GetUserByUploadToken(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	sessionID := strings.TrimSpace(r.URL.Query().Get("sessionId"))
	if sessionID == "" {
		http.Error(w, "sessionId is required", http.StatusBadRequest)
		return
	}
	target, err := h.store.GetReplayMediaUploadTarget(r.Context(), authUser.UserID, sessionID)
	if err != nil {
		http.Error(w, "run not found for this account", http.StatusNotFound)
		return
	}
	var payload struct {
		Points          []store.MousePathPoint `json:"points"`
		HitTimestampsMS []uint64               `json:"hitTimestampsMs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(payload.Points) == 0 {
		http.Error(w, "empty mouse path", http.StatusBadRequest)
		return
	}
	if err := h.store.UpsertMousePath(r.Context(), target.StoredSessionID, payload.Points, payload.HitTimestampsMS); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":              true,
		"runId":           target.PublicRunID,
		"pointCount":      len(payload.Points),
		"hitMarkerCount":  len(payload.HitTimestampsMS),
	})
}

func (h *authHandler) handleMousePathMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	runID := strings.TrimSpace(r.URL.Query().Get("runId"))
	if runID == "" {
		http.Error(w, "runId is required", http.StatusBadRequest)
		return
	}
	mousePath, err := h.store.GetMousePath(r.Context(), runID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"available": false,
			"points": []store.MousePathPoint{},
			"hitTimestampsMs": []uint64{},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"available": true,
		"points": mousePath.Points,
		"hitTimestampsMs": mousePath.HitTimestampsMS,
	})
}

func (h *authHandler) handleReplayMediaStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	runID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/media/replays/"), ".mp4")
	runID = strings.TrimSpace(runID)
	if runID == "" {
		http.NotFound(w, r)
		return
	}
	meta, err := h.store.GetReplayMediaMeta(r.Context(), runID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body, contentType, err := h.media.Get(r.Context(), meta.StorageKey)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", contentType)
	_, _ = io.Copy(w, body)
}

func normalizeMediaQuality(value string) string {
	switch strings.TrimSpace(value) {
	case "high", "ultra":
		return strings.TrimSpace(value)
	default:
		return "standard"
	}
}

func (h *authHandler) handleAdminReclassifyUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	handle := strings.TrimSpace(r.URL.Query().Get("handle"))
	if handle == "" {
		http.Error(w, "handle is required", http.StatusBadRequest)
		return
	}

	updated, err := h.store.RepairScenarioTypesForUser(r.Context(), handle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"updated": updated,
	})
}

func (h *authHandler) handleAdminRepairUserMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	handle := strings.TrimSpace(r.URL.Query().Get("handle"))
	if handle == "" {
		http.Error(w, "handle is required", http.StatusBadRequest)
		return
	}

	updated, err := h.store.RepairRunMetrics(r.Context(), handle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"updated": updated,
	})
}

func (h *authHandler) handleAdminClearUserFailures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	handle := strings.TrimSpace(r.URL.Query().Get("handle"))
	if handle == "" {
		http.Error(w, "handle is required", http.StatusBadRequest)
		return
	}

	count, err := h.store.ClearIngestFailuresForUser(r.Context(), handle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"cleared": count,
	})
}

func (h *authHandler) handleAdminFailuresExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	handle := strings.TrimSpace(r.URL.Query().Get("handle"))
	format := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("format")))
	if format == "" {
		format = "csv"
	}

	failures, err := h.store.GetAdminFailures(r.Context(), handle, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if format == "json" {
		filename := "aimmod-hub-failures.json"
		if handle != "" {
			filename = "aimmod-hub-failures-" + handle + ".json"
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		_ = json.NewEncoder(w).Encode(failures)
		return
	}

	filename := "aimmod-hub-failures.csv"
	if handle != "" {
		filename = "aimmod-hub-failures-" + handle + ".csv"
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"id", "user_external_id", "session_id", "scenario_name", "error_message", "created_at"})
	for _, failure := range failures {
		_ = writer.Write([]string{
			fmt.Sprintf("%d", failure.ID),
			failure.UserExternalID,
			failure.SessionID,
			failure.ScenarioName,
			failure.ErrorMessage,
			failure.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writer.Flush()
}

func (h *authHandler) handleAdminUserMetricsExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	handle := strings.TrimSpace(r.URL.Query().Get("handle"))
	if handle == "" {
		http.Error(w, "handle is required", http.StatusBadRequest)
		return
	}

	export, err := h.store.GetAdminUserMetricsExport(r.Context(), handle, parseAdminDays(r.URL.Query().Get("days")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("aimmod-user-metrics-%s.json", strings.ToLower(strings.ReplaceAll(handle, " ", "-")))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(export)
}

func (h *authHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		_ = h.store.DeleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.SessionCookieSecure,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *authHandler) handleUploadTokens(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireSessionUser(w, r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		tokens, err := h.store.ListUploadTokens(r.Context(), user.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, tokens)
	case http.MethodPost:
		var body createTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		label := strings.TrimSpace(body.Label)
		if label == "" {
			label = "Desktop app"
		}
		record, rawToken, err := h.store.CreateUploadToken(r.Context(), user.UserID, label)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, createTokenResponse{
			Token:  rawToken,
			Record: record,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *authHandler) handleRevokeUploadToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, ok := h.requireSessionUser(w, r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body revokeTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if body.ID <= 0 {
		http.Error(w, "invalid token id", http.StatusBadRequest)
		return
	}

	if err := h.store.RevokeUploadToken(r.Context(), user.UserID, body.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *authHandler) handleDeviceStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body deviceStartRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	link, err := h.store.CreateDeviceLinkRequest(r.Context(), body.Label)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	webOrigin := strings.TrimRight(h.cfg.WebAppOrigin, "/")
	verificationURI := webOrigin + "/link-device"
	writeJSON(w, http.StatusCreated, deviceStartResponse{
		DeviceCode:              link.DeviceCode,
		UserCode:                link.UserCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURI + "?user_code=" + url.QueryEscape(link.UserCode),
		ExpiresIn:               int64(time.Until(link.ExpiresAt).Seconds()),
		Interval:                3,
	})
}

func (h *authHandler) handleDevicePoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body devicePollRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	result, err := h.store.PollDeviceLink(r.Context(), body.DeviceCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, devicePollResponse{
		Status:      result.Status,
		ExpiresAt:   result.ExpiresAt,
		User:        result.User,
		UploadToken: result.UploadToken,
	})
}

func (h *authHandler) handleDeviceApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, ok := h.requireSessionUser(w, r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body deviceApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	result, err := h.store.ApproveDeviceLink(r.Context(), user.UserID, body.UserCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, devicePollResponse{
		Status:      result.Status,
		ExpiresAt:   result.ExpiresAt,
		User:        result.User,
		UploadToken: result.UploadToken,
	})
}

func (h *authHandler) exchangeDiscordCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", h.cfg.DiscordClientID)
	form.Set("client_secret", h.cfg.DiscordClientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", h.cfg.DiscordRedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, discordTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("discord token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("discord token exchange returned %s", resp.Status)
	}

	var payload discordTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode discord token response: %w", err)
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("discord token response missing access_token")
	}
	return payload.AccessToken, nil
}

func (h *authHandler) fetchDiscordUser(ctx context.Context, accessToken string) (discordUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discordUserInfoURL, nil)
	if err != nil {
		return discordUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := h.client.Do(req)
	if err != nil {
		return discordUser{}, fmt.Errorf("discord user lookup failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return discordUser{}, fmt.Errorf("discord user lookup returned %s", resp.Status)
	}

	var user discordUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return discordUser{}, fmt.Errorf("decode discord user: %w", err)
	}
	return user, nil
}

func (h *authHandler) requireSessionUser(w http.ResponseWriter, r *http.Request) (store.AuthUser, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return store.AuthUser{}, false
	}
	user, err := h.store.GetUserBySession(r.Context(), cookie.Value)
	if err != nil {
		return store.AuthUser{}, false
	}
	return user, true
}

func (h *authHandler) requireAdminUser(w http.ResponseWriter, r *http.Request) (store.AuthUser, bool) {
	user, ok := h.requireSessionUser(w, r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return store.AuthUser{}, false
	}
	if !h.isAdminUser(user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return store.AuthUser{}, false
	}
	return user, true
}

func (h *authHandler) isAdminUser(user store.AuthUser) bool {
	isAdmin, _ := h.adminStatus(user)
	return isAdmin
}

func (h *authHandler) adminStatus(user store.AuthUser) (bool, string) {
	adminDiscordID := strings.TrimSpace(h.cfg.AdminDiscordUserID)
	if adminDiscordID == "" {
		return false, "admin_not_configured"
	}
	if strings.TrimSpace(user.DiscordUserID) != adminDiscordID {
		return false, "discord_id_mismatch"
	}
	return true, ""
}

func parseAdminDays(raw string) int {
	trimmed := strings.TrimSpace(raw)
	switch trimmed {
	case "", "0", "all":
		return 0
	case "7":
		return 7
	case "30":
		return 30
	case "90":
		return 90
	case "365":
		return 365
	default:
		return 0
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func withAuthCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerTokenFromRequest(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}
