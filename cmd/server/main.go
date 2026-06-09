package main

import (
	"context"
	"log"
	"net/http"

	"slow-sql-observer/internal/api"
	"slow-sql-observer/internal/config"
	"slow-sql-observer/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	for _, warning := range cfg.Warnings {
		log.Printf("config warning: %s", warning)
	}

	ctx := context.Background()
	store, err := storage.Open(ctx, cfg.Analysis, &cfg.Source)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	server := api.NewServer(store, cfg.Server.WebDir)
	log.Printf("server listening on %s", cfg.Server.Addr)
	if err := http.ListenAndServe(cfg.Server.Addr, server.Handler()); err != nil {
		log.Fatalf("listen and serve: %v", err)
	}
}
