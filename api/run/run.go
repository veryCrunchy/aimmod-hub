package run

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"

	httpserver "github.com/veryCrunchy/aimmod-hub/api/internal/http"
	"github.com/veryCrunchy/aimmod-hub/api/internal/service"
)

// Main boots the API and blocks until the server exits.
func Main() error {
	loadOptionalDotEnv(".env.local")
	loadOptionalDotEnv(".env")
	cfg := httpserver.DefaultConfig()
	ctx := context.Background()

	store, err := httpserver.OpenStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	hub := service.NewHubServer(cfg.Version, store)
	return httpserver.ListenAndServe(cfg, hub)
}

func loadOptionalDotEnv(path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	if err := godotenv.Load(path); err != nil {
		log.Printf("could not load %s: %v", path, err)
	}
}
