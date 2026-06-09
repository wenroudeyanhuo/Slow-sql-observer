package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"slow-sql-observer/internal/config"
	"slow-sql-observer/internal/model"
)

type memoryIngestStore struct {
	source       *model.Source
	checkpoint   *model.CollectorCheckpoint
	records      []model.IngestRecordInput
	status       model.CollectorStatus
	metadata     model.SourceMetadataUpdate
	cleanupCalls int
	cleanupErr   error
}

func (m *memoryIngestStore) GetSource(context.Context) (*model.Source, error) {
	return m.source, nil
}

func (m *memoryIngestStore) GetCheckpoint(context.Context, int64) (*model.CollectorCheckpoint, error) {
	return m.checkpoint, nil
}

func (m *memoryIngestStore) IngestRecord(_ context.Context, input model.IngestRecordInput) error {
	m.records = append(m.records, input)
	m.checkpoint = &model.CollectorCheckpoint{
		SourceID:     input.Record.SourceID,
		LogFilePath:  input.Record.SourceLogFilePath,
		FileIdentity: input.Record.SourceFileID,
		LastOffset:   input.Record.SourceOffsetEnd,
	}
	return nil
}

func (m *memoryIngestStore) UpdateSourceMetadata(_ context.Context, _ int64, metadata model.SourceMetadataUpdate) error {
	m.metadata = metadata
	return nil
}

func (m *memoryIngestStore) UpdateCollectorStatus(_ context.Context, status model.CollectorStatus) error {
	m.status = status
	return nil
}

func (m *memoryIngestStore) CleanupExpiredRecords(_ context.Context, _ int64, _ time.Time) (int64, error) {
	m.cleanupCalls++
	if m.cleanupErr != nil {
		return 0, m.cleanupErr
	}
	return 3, nil
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

	store := &memoryIngestStore{
		source: &model.Source{
			ID:           1,
			InstanceName: "test-instance",
			SlowLogPath:  logPath,
		},
	}
	service := NewCollectorService(config.SourceConfig{
		InstanceName: "test-instance",
		SlowLogPath:  logPath,
	}, config.RuntimeConfig{CollectorPollInterval: time.Second}, store)
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
	if store.status.CollectorState != model.CollectorStateHealthy {
		t.Fatalf("expected healthy collector status, got %q", store.status.CollectorState)
	}
}

func TestCollectOnceContinuesWhenSourceProbeFails(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sample-slow.log")
	content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "sample-slow.log"))
	if err != nil {
		t.Fatalf("read sample log: %v", err)
	}
	if err := os.WriteFile(logPath, content, 0o644); err != nil {
		t.Fatalf("write sample log: %v", err)
	}

	store := &memoryIngestStore{
		source: &model.Source{
			ID:                    1,
			InstanceName:          "test-instance",
			SlowLogPath:           logPath,
			DatabaseDSNConfigured: true,
		},
	}
	service := NewCollectorService(config.SourceConfig{
		InstanceName: "test-instance",
		SlowLogPath:  logPath,
		DatabaseDSN:  "user:pass@tcp(localhost:3306)/",
	}, config.RuntimeConfig{CollectorPollInterval: time.Second}, store)
	service.probe = func(context.Context, string) (model.SourceMetadataUpdate, error) {
		return model.SourceMetadataUpdate{}, os.ErrPermission
	}

	if _, err := service.CollectOnce(context.Background()); err != nil {
		t.Fatalf("expected ingestion to continue despite probe failure, got %v", err)
	}
	if store.status.CollectorState != model.CollectorStateDegraded {
		t.Fatalf("expected degraded status, got %q", store.status.CollectorState)
	}
	if store.status.LastErrorMessage == nil || !strings.Contains(*store.status.LastErrorMessage, "probe source db") {
		t.Fatalf("expected probe failure to be reflected in collector status")
	}
}

func TestCollectOnceRetentionFailureDoesNotBreakCommittedIngest(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sample-slow.log")
	content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "sample-slow.log"))
	if err != nil {
		t.Fatalf("read sample log: %v", err)
	}
	if err := os.WriteFile(logPath, content, 0o644); err != nil {
		t.Fatalf("write sample log: %v", err)
	}

	store := &memoryIngestStore{
		source: &model.Source{
			ID:           1,
			InstanceName: "test-instance",
			SlowLogPath:  logPath,
		},
		cleanupErr: os.ErrInvalid,
	}
	service := NewCollectorService(config.SourceConfig{
		InstanceName: "test-instance",
		SlowLogPath:  logPath,
	}, config.RuntimeConfig{
		CollectorPollInterval:  time.Second,
		RawRecordRetentionDays: 7,
	}, store)

	result, err := service.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("expected retention failure to avoid breaking ingest, got %v", err)
	}
	if result.EventsProcessed == 0 {
		t.Fatalf("expected records to be ingested before retention cleanup")
	}
	if store.cleanupCalls == 0 {
		t.Fatalf("expected retention cleanup to be attempted")
	}
	if store.status.CollectorState != model.CollectorStateDegraded {
		t.Fatalf("expected degraded status after retention failure, got %q", store.status.CollectorState)
	}
}
