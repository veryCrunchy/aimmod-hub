package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/encoding/protojson"

	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/service"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
	hubv1connect "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1/hubv1connect"
	osuv1connect "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1/osuv1connect"
)

type ingestBatchRequest struct {
	Sessions []json.RawMessage `json:"sessions"`
}

type ingestBatchFailure struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

type ingestBatchResponse struct {
	UploadedSessionIDs []string             `json:"uploadedSessionIds"`
	Failures           []ingestBatchFailure `json:"failures"`
	UploadedCount      int                  `json:"uploadedCount"`
	FailedCount        int                  `json:"failedCount"`
}

type Config struct {
	Addr                 string
	Version              string
	AllowedWebOrigin     string
	WebAppOrigin         string
	ProfileSubdomainHost string
	// StaticDir, when set, makes the API server also serve the built frontend
	// with server-side meta tag injection. Set AIMMOD_HUB_STATIC_DIR to enable.
	StaticDir                              string
	DatabaseURL                            string
	DiscordClientID                        string
	DiscordClientSecret                    string
	DiscordRedirectURI                     string
	AdminDiscordUserID                     string
	SessionCookieSecure                    bool
	MediaDir                               string
	LLMDir                                 string
	LLMManifestVersion                     string
	LLMRuntimeWindowsX64URL                string
	LLMRuntimeWindowsX64SHA256             string
	LLMRuntimeWindowsX64ArchiveType        string
	LLMRuntimeWindowsX64ExtraURL           string
	LLMRuntimeWindowsX64ExtraSHA256        string
	LLMRuntimeWindowsX64ExtraArchiveType   string
	LLMRuntimeWindowsArm64URL              string
	LLMRuntimeWindowsArm64SHA256           string
	LLMRuntimeWindowsArm64ArchiveType      string
	LLMRuntimeWindowsArm64ExtraURL         string
	LLMRuntimeWindowsArm64ExtraSHA256      string
	LLMRuntimeWindowsArm64ExtraArchiveType string
	LLMModelURL                            string
	LLMModelSHA256                         string
	LLMModelFilename                       string
	MediaBackend                           string
	S3Bucket                               string
	S3Region                               string
	S3Endpoint                             string
	S3AccessKeyID                          string
	S3SecretAccessKey                      string
	S3ForcePathStyle                       bool
	OsuClientID                            string
	OsuClientSecret                        string
	OsuCacheTTL                            time.Duration
	OsuCacheMaxEntries                     int
	OsuProviderRequestsPerSecond           float64
	OsuRequestTimeout                      time.Duration
}

func NewMux(cfg Config, hub *service.HubServer) http.Handler {
	mux := http.NewServeMux()
	auth := newAuthHandler(cfg, hub.Store(), hub.Events())
	path, handler := hubv1connect.NewHubServiceHandler(hub)
	mux.Handle(path, withCORS(cfg.AllowedWebOrigin, handler))
	osuServer, err := osuservice.NewServer(osuservice.Config{
		OfficialClientID:          cfg.OsuClientID,
		PlayerIndex:               hub.Store(),
		OfficialClientSecret:      cfg.OsuClientSecret,
		UserAgent:                 "AimMod-Hub/" + cfg.Version + " (https://aimmod.app)",
		CacheTTL:                  cfg.OsuCacheTTL,
		CacheMaxEntries:           cfg.OsuCacheMaxEntries,
		ProviderRequestsPerSecond: cfg.OsuProviderRequestsPerSecond,
		RequestTimeout:            cfg.OsuRequestTimeout,
	})
	if err != nil {
		log.Printf("osu provider service disabled: %v", err)
	} else {
		osuPath, osuHandler := osuv1connect.NewOsuServiceHandler(osuServer)
		mux.Handle(osuPath, withCORS(cfg.AllowedWebOrigin, osuHandler))
	}
	auth.register(mux)
	adminOsu := &adminOsuHandler{store: hub.Store(), isAdmin: auth.isAdminUser}
	if osuServer != nil {
		adminOsu.providers = osuServer
	}
	mux.Handle("/admin/osu/overview", withAuthCORS(cfg.AllowedWebOrigin, adminOsu))
	mux.Handle("/admin/osu/players", withAuthCORS(cfg.AllowedWebOrigin, adminOsu))
	mux.Handle("/admin/osu/beatmaps", withAuthCORS(cfg.AllowedWebOrigin, adminOsu))
	mux.Handle("/admin/osu/providers", withAuthCORS(cfg.AllowedWebOrigin, adminOsu))
	newOsuSyncHandler(hub.Store(), auth.media, osuServer).register(mux, cfg.AllowedWebOrigin)
	newOsuProfileScoresHandler(hub.Store(), osuServer).register(mux, cfg.AllowedWebOrigin)
	newOsuPlayersHandler(osuServer).register(mux, cfg.AllowedWebOrigin)
	if osuServer != nil && hub.Store() != nil && cfg.OsuClientID != "" && cfg.OsuClientSecret != "" {
		go osuServer.RunPlayerIndexer(context.Background())
	}
	var playbackMetadata osuPlaybackMetadataProvider
	if osuServer != nil {
		playbackMetadata = osuServer
	}
	newOsuPlaybackHandler(playbackMetadata).register(mux, cfg.AllowedWebOrigin)
	mux.Handle("/api/osu/v1/pp/calculate", withCORS(cfg.AllowedWebOrigin, newOsuPpHandler()))
	mux.Handle(osuSkinPrefix, withCORS(cfg.AllowedWebOrigin, newOsuSkinHandler()))
	mux.Handle("/api/osu/v1/releases/", withCORS(cfg.AllowedWebOrigin, newOsuReleaseChannelHandler()))
	mux.Handle("/ingest/batch", withCORS(cfg.AllowedWebOrigin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req ingestBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		result := ingestBatchResponse{
			UploadedSessionIDs: []string{},
			Failures:           []ingestBatchFailure{},
		}
		for _, rawSession := range req.Sessions {
			if len(rawSession) == 0 {
				continue
			}
			session := &hubv1.IngestSessionRequest{}
			if err := protojson.Unmarshal(rawSession, session); err != nil {
				result.Failures = append(result.Failures, ingestBatchFailure{
					Message: "invalid JSON payload: " + err.Error(),
				})
				continue
			}
			resp, err := hub.IngestAuthorized(r.Context(), r.Header.Get("Authorization"), session)
			if err != nil {
				result.Failures = append(result.Failures, ingestBatchFailure{
					SessionID: session.GetSessionId(),
					Message:   err.Error(),
				})
				continue
			}
			result.UploadedSessionIDs = append(result.UploadedSessionIDs, resp.GetSessionId())
		}
		result.UploadedCount = len(result.UploadedSessionIDs)
		result.FailedCount = len(result.Failures)

		w.Header().Set("content-type", "application/json")
		if result.UploadedCount == 0 && result.FailedCount > 0 {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(result)
	})))
	mux.Handle("/api/events", withCORS(cfg.AllowedWebOrigin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handle := strings.TrimSpace(r.URL.Query().Get("handle"))
		if handle == "" {
			http.Error(w, "handle required", http.StatusBadRequest)
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
		ch, unsub := hub.Events().Subscribe(handle)
		defer unsub()
		fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
		flusher.Flush()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ch:
				fmt.Fprintf(w, "event: scores_updated\ndata: {}\n\n")
				flusher.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})))
	externalHandler := newExternalHandler(hub)
	coachingHandler := newCoachingHandler(hub.Store())
	mux.Handle("/api/lookup", withCORS(cfg.AllowedWebOrigin, externalHandler))
	mux.Handle("/api/lookup/", withCORS(cfg.AllowedWebOrigin, externalHandler))
	mux.Handle("/api/coaching/", withCORS(cfg.AllowedWebOrigin, coachingHandler))
	mux.Handle("/robots.txt", newRobotsHandler(cfg.WebAppOrigin))
	mux.Handle("/join", newDiscordJoinHandler("https://discord.com/api/guilds/1477238446706917389/widget.json"))
	sitemaps := newSitemapHandler(cfg.WebAppOrigin, hub.Store())
	mux.Handle("/sitemap.xml", sitemaps)
	mux.Handle("/sitemaps/", sitemaps)
	mux.Handle("/social-preview.png", newSocialPreviewHandler(hub.Store()))
	if hasLLMManifest(cfg) {
		mux.Handle("/llm/manifest.json", newLLMManifestHandler(cfg))
	}
	if strings.TrimSpace(cfg.LLMDir) != "" {
		mux.Handle("/llm/", newLLMAssetHandler(cfg.LLMDir))
	}
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"service":"aimmod-hub"}`))
	})
	if cfg.StaticDir != "" {
		mux.Handle("/", NewSPAHandler(cfg.StaticDir, hub.Store(), cfg.WebAppOrigin, osuServer))
	}
	rootHandler := http.Handler(mux)
	rootHandler = newProfileSubdomainRedirectHandler(cfg, hub.Store(), rootHandler)
	return h2c.NewHandler(rootHandler, &http2.Server{})
}

func ListenAndServe(cfg Config, hub *service.HubServer) error {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	log.Printf("aimmod-hub api listening on %s", cfg.Addr)
	return http.ListenAndServe(cfg.Addr, NewMux(cfg, hub))
}

func OpenStore(ctx context.Context, cfg Config) (*store.Store, error) {
	start := time.Now()
	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	log.Printf("aimmod-hub database ready in %s", time.Since(start).Round(time.Millisecond))
	return db, nil
}

func DefaultConfig() Config {
	addr := os.Getenv("AIMMOD_HUB_ADDR")
	if addr == "" {
		port := strings.TrimSpace(os.Getenv("PORT"))
		if port != "" {
			if strings.HasPrefix(port, ":") {
				addr = port
			} else {
				addr = ":" + port
			}
		}
	}
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		Addr:                                   addr,
		Version:                                envOrDefault("AIMMOD_HUB_VERSION", "dev"),
		AllowedWebOrigin:                       envOrDefault("AIMMOD_HUB_WEB_ORIGIN", "http://localhost:5173"),
		WebAppOrigin:                           envOrDefault("AIMMOD_HUB_WEB_ORIGIN", "http://localhost:5173"),
		ProfileSubdomainHost:                   strings.TrimSpace(os.Getenv("AIMMOD_HUB_PROFILE_SUBDOMAIN_HOST")),
		DatabaseURL:                            envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/aimmod_hub?sslmode=disable"),
		DiscordClientID:                        os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret:                    os.Getenv("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURI:                     os.Getenv("DISCORD_REDIRECT_URI"),
		AdminDiscordUserID:                     strings.TrimSpace(os.Getenv("AIMMOD_HUB_ADMIN_DISCORD_USER_ID")),
		SessionCookieSecure:                    envOrDefault("SESSION_COOKIE_SECURE", "false") == "true",
		StaticDir:                              strings.TrimSpace(os.Getenv("AIMMOD_HUB_STATIC_DIR")),
		MediaDir:                               envOrDefault("AIMMOD_HUB_MEDIA_DIR", "./var/media"),
		LLMDir:                                 strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_DIR")),
		LLMManifestVersion:                     strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_MANIFEST_VERSION")),
		LLMRuntimeWindowsX64URL:                strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_URL")),
		LLMRuntimeWindowsX64SHA256:             strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_SHA256")),
		LLMRuntimeWindowsX64ArchiveType:        envOrDefault("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_ARCHIVE_TYPE", "zip"),
		LLMRuntimeWindowsX64ExtraURL:           strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_EXTRA_URL")),
		LLMRuntimeWindowsX64ExtraSHA256:        strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_EXTRA_SHA256")),
		LLMRuntimeWindowsX64ExtraArchiveType:   envOrDefault("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_EXTRA_ARCHIVE_TYPE", "zip"),
		LLMRuntimeWindowsArm64URL:              strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_ARM64_URL")),
		LLMRuntimeWindowsArm64SHA256:           strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_ARM64_SHA256")),
		LLMRuntimeWindowsArm64ArchiveType:      envOrDefault("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_ARM64_ARCHIVE_TYPE", "zip"),
		LLMRuntimeWindowsArm64ExtraURL:         strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_ARM64_EXTRA_URL")),
		LLMRuntimeWindowsArm64ExtraSHA256:      strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_ARM64_EXTRA_SHA256")),
		LLMRuntimeWindowsArm64ExtraArchiveType: envOrDefault("AIMMOD_HUB_LLM_RUNTIME_WINDOWS_ARM64_EXTRA_ARCHIVE_TYPE", "zip"),
		LLMModelURL:                            strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_MODEL_URL")),
		LLMModelSHA256:                         strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_MODEL_SHA256")),
		LLMModelFilename:                       strings.TrimSpace(os.Getenv("AIMMOD_HUB_LLM_MODEL_FILENAME")),
		MediaBackend:                           envOrDefault("AIMMOD_HUB_MEDIA_BACKEND", "local"),
		S3Bucket:                               strings.TrimSpace(os.Getenv("AIMMOD_HUB_S3_BUCKET")),
		S3Region:                               envOrDefault("AIMMOD_HUB_S3_REGION", "auto"),
		S3Endpoint:                             strings.TrimSpace(os.Getenv("AIMMOD_HUB_S3_ENDPOINT")),
		S3AccessKeyID:                          strings.TrimSpace(os.Getenv("AIMMOD_HUB_S3_ACCESS_KEY_ID")),
		S3SecretAccessKey:                      strings.TrimSpace(os.Getenv("AIMMOD_HUB_S3_SECRET_ACCESS_KEY")),
		S3ForcePathStyle:                       parseEnvBool("AIMMOD_HUB_S3_FORCE_PATH_STYLE", false),
		OsuClientID:                            strings.TrimSpace(os.Getenv("AIMMOD_OSU_CLIENT_ID")),
		OsuClientSecret:                        strings.TrimSpace(os.Getenv("AIMMOD_OSU_CLIENT_SECRET")),
		OsuCacheTTL:                            parseEnvDuration("AIMMOD_OSU_CACHE_TTL", 5*time.Minute),
		OsuCacheMaxEntries:                     parseEnvInt("AIMMOD_OSU_CACHE_MAX_ENTRIES", 256, 1, 4096),
		OsuProviderRequestsPerSecond:           parseEnvFloat("AIMMOD_OSU_PROVIDER_RPS", 4, 0.1, 100),
		OsuRequestTimeout:                      parseEnvDuration("AIMMOD_OSU_REQUEST_TIMEOUT", 10*time.Second),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseEnvInt(key string, fallback, minimum, maximum int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}

func parseEnvFloat(key string, fallback, minimum, maximum float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}

func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Connect-Protocol-Version,Connect-Timeout-Ms,Authorization,Idempotency-Key,X-Content-SHA256")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "Grpc-Status,Grpc-Message,Grpc-Status-Details-Bin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
