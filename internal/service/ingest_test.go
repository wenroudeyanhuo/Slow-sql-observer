package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"slow-sql-observer/internal/model"
)

type memoryIngestStore struct {
	checkpoint *model.CollectorCheckpoint
	records    []model.IngestRecordInput
}

func (m *memoryIngestStore) GetCheckpoint(context.Context, string, string) (*model.CollectorCheckpoint, error) {
	return m.checkpoint, nil
}

func (m *memoryIngestStore) IngestRecord(_ context.Context, input model.IngestRecordInput) error {
	m.records = append(m.records, input)
	m.checkpoint = &model.CollectorCheckpoint{
		InstanceName: input.Record.SourceInstance,
		LogFilePath:  input.Record.SourceLogFilePath,
		FileIdentity: input.Record.SourceFileID,
		LastOffset:   input.Record.SourceOffsetEnd,
	}
	return nil
}

func TestCollectOnceProcessesSampleLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sample-slow.log")
	content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "sample-slow.log"))
	if err != nil {
		t.Fatalf("read sample log: %v", err)
	}
	if err := os.WriteFile(logPath, content, 0o644); err != nil {
		t.Fatalf("write sample log: %v", err)
	}

	store := &memoryIngestStore{}
	service := NewCollectorService("test-instance", logPath, store)
	result, err := service.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("CollectOnce returned error: %v", err)
	}
	if result.EventsProcessed == 0 {
		t.Fatalf("expected collector to process sample events")
	}
	if len(store.records) != result.EventsProcessed {
		t.Fatalf("expected ingested records to match processed events")
	}
	if store.records[0].Fingerprint.Hash == "" {
		t.Fatalf("expected fingerprint hash to be populated")
	}
}
