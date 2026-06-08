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

	ctx := context.Background()
	store, err := storage.Open(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	collector := service.NewCollectorService(cfg.Collector.InstanceName, cfg.Collector.SlowLogPath, store)
	for {
		result, err := collector.CollectOnce(ctx)
		if err != nil {
			log.Printf("collect once failed: %v", err)
		} else {
			log.Printf("processed %d events from %s (offset %d -> %d)", result.EventsProcessed, cfg.Collector.SlowLogPath, result.StartOffset, result.FinalOffset)
		}
		time.Sleep(cfg.Collector.PollInterval)
	}
}
