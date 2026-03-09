package httpserver

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/veryCrunchy/aimmod-hub/api/internal/service"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
	hubv1connect "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1/hubv1connect"
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
	Addr                string
	Version             string
	AllowedWebOrigin    string
	WebAppOrigin        string
	DatabaseURL         string
	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURI  string
	SessionCookieSecure bool
}

func NewMux(cfg Config, hub *service.HubServer) http.Handler {
	mux := http.NewServeMux()
	auth := newAuthHandler(cfg, hub.Store())
	path, handler := hubv1connect.NewHubServiceHandler(hub)
	mux.Handle(path, withCORS(cfg.AllowedWebOrigin, handler))
	auth.register(mux)
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
	mux.Handle("/search", withCORS(cfg.AllowedWebOrigin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		results, err := hub.Store().Search(r.Context(), query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(results)
	})))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"service":"aimmod-hub"}`))
	})
	return h2c.NewHandler(mux, &http2.Server{})
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
	return Config{
		Addr:                envOrDefault("AIMMOD_HUB_ADDR", ":8080"),
		Version:             envOrDefault("AIMMOD_HUB_VERSION", "dev"),
		AllowedWebOrigin:    envOrDefault("AIMMOD_HUB_WEB_ORIGIN", "http://localhost:5173"),
		WebAppOrigin:        envOrDefault("AIMMOD_HUB_WEB_ORIGIN", "http://localhost:5173"),
		DatabaseURL:         envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/aimmod_hub?sslmode=disable"),
		DiscordClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURI:  os.Getenv("DISCORD_REDIRECT_URI"),
		SessionCookieSecure: envOrDefault("SESSION_COOKIE_SECURE", "false") == "true",
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Connect-Protocol-Version,Connect-Timeout-Ms,Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "Grpc-Status,Grpc-Message,Grpc-Status-Details-Bin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
