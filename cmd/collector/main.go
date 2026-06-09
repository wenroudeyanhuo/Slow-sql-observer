package main

import (
	"context"
	"log"
	"time"

	"slow-sql-observer/internal/config"
	"slow-sql-observer/internal/service"
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

	collector := service.NewCollectorService(cfg.Source, cfg.Runtime, store)
	for {
		result, err := collector.CollectOnce(ctx)
		if err != nil {
			log.Printf("collect once failed: %v", err)
		} else {
			log.Printf("processed %d events from %s (offset %d -> %d)", result.EventsProcessed, cfg.Source.SlowLogPath, result.StartOffset, result.FinalOffset)
		}
		time.Sleep(cfg.Runtime.CollectorPollInterval)
	}
}
