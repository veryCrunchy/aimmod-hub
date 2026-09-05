package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/media"
	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

const (
	maxOsuSyncBytes   = 4 << 20
	maxOsuReplayBytes = 64 << 20
)

type osuSyncStore interface {
	GetUserByUploadToken(context.Context, string) (store.AuthUser, error)
	SaveOsuSync(context.Context, int64, store.OsuSyncInput) (store.OsuSyncResult, error)
	GetOsuReplayUploadTarget(context.Context, int64, string) (store.OsuReplayUploadTarget, error)
	CompleteOsuReplayUpload(context.Context, int64, string, int64) error
	GetOsuPublicReplay(context.Context, string) (store.OsuPublicReplay, error)
	ListOsuCommunity(context.Context, int) ([]store.OsuPublicReplay, error)
	GetOsuPublicProfile(context.Context, string, int) (store.OsuPublicProfile, error)
	GetOsuReplayFile(context.Context, string) (string, string, int64, error)
}

type osuSyncHandler struct {
	store    osuSyncStore
	media    media.Storage
	official replayScoreProvider
}

type replayScoreProvider interface {
	GetPublicScore(context.Context, int64) (osuservice.OfficialScoreDetail, error)
}

func newOsuSyncHandler(dataStore osuSyncStore, mediaStorage media.Storage, official ...replayScoreProvider) *osuSyncHandler {
	h := &osuSyncHandler{store: dataStore, media: mediaStorage}
	if len(official) > 0 {
		h.official = official[0]
	}
	return h
}

func (h *osuSyncHandler) register(mux *http.ServeMux, origin string) {
	mux.Handle("/api/osu/v1/sync", withCORS(origin, http.HandlerFunc(h.handleSync)))
	mux.Handle("/api/osu/v1/community", withCORS(origin, http.HandlerFunc(h.handleCommunity)))
	mux.Handle("/api/osu/v1/profiles/", withCORS(origin, http.HandlerFunc(h.handleProfile)))
	mux.Handle("/api/osu/v1/replays/", withCORS(origin, http.HandlerFunc(h.handleReplay)))
	mux.Handle("/media/osu-replays/", withCORS(origin, http.HandlerFunc(h.handleReplayFile)))
}

func (h *osuSyncHandler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authUser, err := h.store.GetUserByUploadToken(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxOsuSyncBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input store.OsuSyncInput
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid osu sync payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := ensureJSONEOF(decoder); err != nil {
		http.Error(w, "invalid osu sync payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	normalizeOsuSyncInput(&input)
	if err := validateOsuSyncInput(input, r.Header.Get("Idempotency-Key")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.store.SaveOsuSync(r.Context(), authUser.UserID, input)
	if err != nil {
		if strings.Contains(err.Error(), "different content") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "could not store osu score", http.StatusInternalServerError)
		return
	}
	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (h *osuSyncHandler) handleCommunity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := parseOsuListLimit(r.URL.Query().Get("limit"))
	items, err := h.store.ListOsuCommunity(r.Context(), limit)
	if err != nil {
		http.Error(w, "could not load osu community", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *osuSyncHandler) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	handle := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/osu/v1/profiles/"))
	if handle == "" || strings.Contains(handle, "/") {
		http.Error(w, "profile handle is required", http.StatusBadRequest)
		return
	}
	profile, err := h.store.GetOsuPublicProfile(r.Context(), handle, parseOsuListLimit(r.URL.Query().Get("limit")))
	if err != nil {
		http.Error(w, "osu profile not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, profile)
}

func (h *osuSyncHandler) handleReplay(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/osu/v1/replays/"), "/")
	if strings.HasSuffix(path, "/file") {
		shareID := strings.TrimSuffix(path, "/file")
		h.handleReplayUpload(w, r, shareID)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !validOsuShareID(path) {
		http.Error(w, "replay not found", http.StatusNotFound)
		return
	}
	replay, err := h.store.GetOsuPublicReplay(r.Context(), path)
	if err != nil {
		http.Error(w, "replay not found", http.StatusNotFound)
		return
	}
	if replay.Visibility == store.OsuVisibilityUnlisted {
		w.Header().Set("Cache-Control", "private, no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=30")
	}
	if replay.PerformancePoints == nil && replay.OnlineScoreID > 0 && h.official != nil {
		detail, err := h.official.GetPublicScore(r.Context(), replay.OnlineScoreID)
		if err == nil && detail.Status == "available" && detail.Item != nil {
			score := detail.Item
			if score.OnlineScoreID == replay.OnlineScoreID && score.OsuUserID == replay.OsuUserID && score.BeatmapID == replay.BeatmapID && score.Ruleset == replay.Ruleset && score.PerformancePoints != nil && !math.IsNaN(*score.PerformancePoints) && !math.IsInf(*score.PerformancePoints, 0) && *score.PerformancePoints >= 0 {
				pp := *score.PerformancePoints
				replay.PerformancePoints = &pp
			}
		}
	}
	writeJSON(w, http.StatusOK, replay)
}

func (h *osuSyncHandler) handleReplayUpload(w http.ResponseWriter, r *http.Request, shareID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !validOsuShareID(shareID) {
		http.Error(w, "replay target not found", http.StatusNotFound)
		return
	}
	authUser, err := h.store.GetUserByUploadToken(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	target, err := h.store.GetOsuReplayUploadTarget(r.Context(), authUser.UserID, shareID)
	if err != nil {
		http.Error(w, "replay target not found for this account", http.StatusNotFound)
		return
	}
	declaredHash := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Content-SHA256")))
	if !validSHA256(declaredHash) || !strings.EqualFold(declaredHash, target.ReplaySHA256) {
		http.Error(w, "replay hash does not match metadata", http.StatusBadRequest)
		return
	}
	contentType := strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])
	if contentType != "application/x-osu-replay" && contentType != "application/octet-stream" {
		http.Error(w, "unsupported replay content type", http.StatusUnsupportedMediaType)
		return
	}
	if r.ContentLength > maxOsuReplayBytes {
		http.Error(w, "replay exceeds 64 MiB", http.StatusRequestEntityTooLarge)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxOsuReplayBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read replay", http.StatusBadRequest)
		return
	}
	if len(payload) == 0 {
		http.Error(w, "replay is empty", http.StatusBadRequest)
		return
	}
	digest := sha256.Sum256(payload)
	actualHash := hex.EncodeToString(digest[:])
	if !strings.EqualFold(actualHash, declaredHash) {
		http.Error(w, "replay content hash mismatch", http.StatusBadRequest)
		return
	}
	if target.StorageKey != "" && strings.EqualFold(target.ReplaySHA256, actualHash) && target.ByteSize == int64(len(payload)) {
		writeJSON(w, http.StatusOK, map[string]any{"shareId": shareID, "uploaded": false, "deduplicated": true, "byteSize": len(payload)})
		return
	}

	storageKey := fmt.Sprintf("osu-replays/%s/%s.osr", shareID, actualHash)
	if err := h.media.Put(r.Context(), storageKey, "application/x-osu-replay", bytes.NewReader(payload), int64(len(payload))); err != nil {
		http.Error(w, "could not store replay", http.StatusInternalServerError)
		return
	}
	if err := h.store.CompleteOsuReplayUpload(r.Context(), target.ScoreID, storageKey, int64(len(payload))); err != nil {
		_ = h.media.Delete(r.Context(), storageKey)
		http.Error(w, "could not finalize replay", http.StatusInternalServerError)
		return
	}
	if target.StorageKey != "" && target.StorageKey != storageKey {
		_ = h.media.Delete(r.Context(), target.StorageKey)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"shareId": shareID, "uploaded": true, "deduplicated": false, "byteSize": len(payload)})
}

func (h *osuSyncHandler) handleReplayFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/media/osu-replays/"), "/")
	shareID := strings.TrimSuffix(name, ".osr")
	if !strings.HasSuffix(name, ".osr") || !validOsuShareID(shareID) {
		http.NotFound(w, r)
		return
	}
	storageKey, contentType, byteSize, err := h.store.GetOsuReplayFile(r.Context(), shareID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body, _, err := h.media.Get(r.Context(), storageKey)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(byteSize, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.osr"`, shareID))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func normalizeOsuSyncInput(input *store.OsuSyncInput) {
	input.ClientUploadID = strings.TrimSpace(input.ClientUploadID)
	input.ContentHash = strings.ToLower(strings.TrimSpace(input.ContentHash))
	input.Visibility = strings.ToLower(strings.TrimSpace(input.Visibility))
	if input.Visibility == "" {
		input.Visibility = store.OsuVisibilityPrivate
	}
	input.Profile.Username = strings.TrimSpace(input.Profile.Username)
	input.Profile.CountryCode = strings.ToUpper(strings.TrimSpace(input.Profile.CountryCode))
	input.Profile.AvatarURL = strings.TrimSpace(input.Profile.AvatarURL)
	input.BeatmapSet.SetKey = strings.TrimSpace(input.BeatmapSet.SetKey)
	input.BeatmapSet.Title = strings.TrimSpace(input.BeatmapSet.Title)
	input.BeatmapSet.Artist = strings.TrimSpace(input.BeatmapSet.Artist)
	input.BeatmapSet.Creator = strings.TrimSpace(input.BeatmapSet.Creator)
	input.BeatmapSet.Source = strings.TrimSpace(input.BeatmapSet.Source)
	input.BeatmapSet.CoverURL = strings.TrimSpace(input.BeatmapSet.CoverURL)
	input.Difficulty.DifficultyKey = strings.TrimSpace(input.Difficulty.DifficultyKey)
	input.Difficulty.SetKey = strings.TrimSpace(input.Difficulty.SetKey)
	input.Difficulty.Checksum = strings.ToLower(strings.TrimSpace(input.Difficulty.Checksum))
	input.Difficulty.Version = strings.TrimSpace(input.Difficulty.Version)
	input.Difficulty.Ruleset = strings.ToLower(strings.TrimSpace(input.Difficulty.Ruleset))
	input.Score.ClientScoreID = strings.TrimSpace(input.Score.ClientScoreID)
	for index := range input.Score.Mods {
		input.Score.Mods[index] = strings.ToUpper(strings.TrimSpace(input.Score.Mods[index]))
	}
	if input.Replay != nil {
		input.Replay.SHA256 = strings.ToLower(strings.TrimSpace(input.Replay.SHA256))
		input.Replay.ClientFilename = strings.TrimSpace(input.Replay.ClientFilename)
	}
}

func validateOsuSyncInput(input store.OsuSyncInput, idempotencyKey string) error {
	if input.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schemaVersion")
	}
	if input.ClientUploadID == "" || len(input.ClientUploadID) > 128 {
		return fmt.Errorf("clientUploadId is required and must be at most 128 characters")
	}
	if strings.TrimSpace(idempotencyKey) != input.ClientUploadID {
		return fmt.Errorf("Idempotency-Key must match clientUploadId")
	}
	if !validSHA256(input.ContentHash) {
		return fmt.Errorf("contentHash must be a SHA-256 hex digest")
	}
	if input.Visibility != store.OsuVisibilityPrivate && input.Visibility != store.OsuVisibilityUnlisted && input.Visibility != store.OsuVisibilityPublic {
		return fmt.Errorf("visibility must be private, unlisted, or public")
	}
	if input.Profile.OsuUserID <= 0 || input.Profile.Username == "" || len(input.Profile.Username) > 64 {
		return fmt.Errorf("profile requires a positive osuUserId and username")
	}
	if len(input.Profile.CountryCode) > 2 || input.Profile.PlayCount < 0 || input.Profile.PlayTimeSeconds < 0 {
		return fmt.Errorf("profile statistics are invalid")
	}
	if input.Profile.PerformancePoints != nil && !finiteBetween(*input.Profile.PerformancePoints, 0, 100_000) {
		return fmt.Errorf("profile performancePoints is invalid")
	}
	if input.Profile.GlobalRank != nil && *input.Profile.GlobalRank <= 0 {
		return fmt.Errorf("profile globalRank is invalid")
	}
	if len(input.Profile.AvatarURL) > 2_048 {
		return fmt.Errorf("profile avatarUrl is too long")
	}
	if !validOptionalPublicURL(input.Profile.AvatarURL) {
		return fmt.Errorf("profile avatarUrl must be HTTP(S)")
	}
	if input.BeatmapSet.SetKey == "" || input.BeatmapSet.Title == "" || len(input.BeatmapSet.SetKey) > 160 || len(input.BeatmapSet.Title) > 512 {
		return fmt.Errorf("beatmapSet requires a valid setKey and title")
	}
	if input.BeatmapSet.OnlineID < 0 || (input.BeatmapSet.OnlineID > 0 && input.BeatmapSet.SetKey != fmt.Sprintf("online:%d", input.BeatmapSet.OnlineID)) {
		return fmt.Errorf("beatmapSet online identity is inconsistent")
	}
	if len(input.BeatmapSet.CoverURL) > 2_048 || !validOptionalPublicURL(input.BeatmapSet.CoverURL) {
		return fmt.Errorf("beatmapSet coverUrl must be HTTP(S)")
	}
	if input.Difficulty.DifficultyKey == "" || input.Difficulty.SetKey != input.BeatmapSet.SetKey || input.Difficulty.Version == "" {
		return fmt.Errorf("difficulty requires a matching setKey, difficultyKey, and version")
	}
	if input.Difficulty.OnlineID < 0 || (input.Difficulty.OnlineID > 0 && input.Difficulty.DifficultyKey != fmt.Sprintf("online:%d", input.Difficulty.OnlineID)) {
		return fmt.Errorf("difficulty online identity is inconsistent")
	}
	if input.Difficulty.Ruleset != "osu" {
		return fmt.Errorf("only osu!standard scores are supported")
	}
	if !finiteBetween(input.Difficulty.StarRating, 0, 20) || !finiteBetween(input.Difficulty.BPM, 0, 1000) || input.Difficulty.LengthMS < 0 {
		return fmt.Errorf("difficulty statistics are invalid")
	}
	if !finiteBetween(input.Difficulty.CircleSize, 0, 20) || !finiteBetween(input.Difficulty.ApproachRate, 0, 20) || !finiteBetween(input.Difficulty.OverallDifficulty, 0, 20) || !finiteBetween(input.Difficulty.DrainRate, 0, 20) || input.Difficulty.MaxCombo < 0 {
		return fmt.Errorf("difficulty attributes are invalid")
	}
	if input.Score.ClientScoreID == "" || len(input.Score.ClientScoreID) > 128 || input.Score.PlayedAt.IsZero() || input.Score.PlayedAt.After(time.Now().UTC().Add(10*time.Minute)) {
		return fmt.Errorf("score identity or playedAt is invalid")
	}
	if !finiteBetween(input.Score.Accuracy, 0, 1) || input.Score.TotalScore < 0 || input.Score.MaxCombo < 0 || input.Score.Count300 < 0 || input.Score.Count100 < 0 || input.Score.Count50 < 0 || input.Score.CountMiss < 0 {
		return fmt.Errorf("score statistics are invalid")
	}
	if input.Score.PerformancePoints != nil && !finiteBetween(*input.Score.PerformancePoints, 0, 100_000) {
		return fmt.Errorf("score performancePoints is invalid")
	}
	if len(input.Score.Mods) > 32 {
		return fmt.Errorf("too many mods")
	}
	for _, mod := range input.Score.Mods {
		if mod == "" || len(mod) > 16 {
			return fmt.Errorf("invalid mod acronym")
		}
	}
	if input.Replay != nil && !validSHA256(input.Replay.SHA256) {
		return fmt.Errorf("replay.sha256 must be a SHA-256 hex digest")
	}
	if input.Replay != nil && input.Replay.UploadFile && input.Replay.ClientFilename == "" {
		return fmt.Errorf("replay.clientFilename is required when uploading a file")
	}
	if input.Replay != nil && len(input.Replay.ClientFilename) > 255 {
		return fmt.Errorf("replay.clientFilename is too long")
	}
	if input.Analysis != nil {
		if input.Analysis.SchemaVersion <= 0 || input.Analysis.EngineVersion == "" || len(input.Analysis.EngineVersion) > 128 || len(input.Analysis.Payload) == 0 || !json.Valid(input.Analysis.Payload) {
			return fmt.Errorf("analysis contract is invalid")
		}
		if len(input.Analysis.Payload) > 2<<20 {
			return fmt.Errorf("analysis payload exceeds 2 MiB")
		}
	}
	return nil
}

func finiteBetween(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validOsuShareID(value string) bool {
	if !strings.HasPrefix(value, "osu_") || len(value) != 36 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "osu_"))
	return err == nil
}

func validOptionalPublicURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

func parseOsuListLimit(value string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || limit <= 0 {
		return 24
	}
	if limit > 100 {
		return 100
	}
	return limit
}
