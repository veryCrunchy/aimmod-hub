package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"

	httpserver "github.com/veryCrunchy/aimmod-hub/api/internal/http"
	"github.com/veryCrunchy/aimmod-hub/api/internal/service"
)

func main() {
	loadOptionalDotEnv(".env.local")
	loadOptionalDotEnv(".env")
	cfg := httpserver.DefaultConfig()
	ctx := context.Background()

	store, err := httpserver.OpenStore(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	hub := service.NewHubServer(cfg.Version, store)
	if err := httpserver.ListenAndServe(cfg, hub); err != nil {
		log.Fatal(err)
	}
}

func loadOptionalDotEnv(path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	if err := godotenv.Load(path); err != nil {
		log.Printf("could not load %s: %v", path, err)
	}
}
