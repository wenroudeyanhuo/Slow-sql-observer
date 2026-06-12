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
	source                *model.Source
	checkpoint            *model.CollectorCheckpoint
	acquisitionCheckpoint *model.AcquisitionCheckpoint
	records               []model.IngestRecordInput
	status                model.CollectorStatus
	acquisitionStatus     model.AcquisitionStatus
	metadata              model.SourceMetadataUpdate
	checkpointUpserts     []model.CollectorCheckpoint
	cleanupCalls          int
	cleanupErr            error
}

func (m *memoryIngestStore) GetSource(context.Context) (*model.Source, error) {
	return m.source, nil
}

func (m *memoryIngestStore) GetCheckpoint(context.Context, int64) (*model.CollectorCheckpoint, error) {
	return m.checkpoint, nil
}

func (m *memoryIngestStore) GetAcquisitionCheckpoint(context.Context, int64) (*model.AcquisitionCheckpoint, error) {
	return m.acquisitionCheckpoint, nil
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

func (m *memoryIngestStore) UpdateAcquisitionStatus(_ context.Context, status model.AcquisitionStatus) error {
	m.acquisitionStatus = status
	return nil
}

func (m *memoryIngestStore) UpsertCheckpoint(_ context.Context, checkpoint model.CollectorCheckpoint) error {
	m.checkpoint = &checkpoint
	m.checkpointUpserts = append(m.checkpointUpserts, checkpoint)
	return nil
}

func (m *memoryIngestStore) UpsertAcquisitionCheckpoint(_ context.Context, checkpoint model.AcquisitionCheckpoint) error {
	m.acquisitionCheckpoint = &checkpoint
	return nil
}

func (m *memoryIngestStore) CleanupExpiredRecords(_ context.Context, _ int64, _ time.Time) (int64, error) {
	m.cleanupCalls++
	if m.cleanupErr != nil {
		return 0, m.cleanupErr
	}
	return 3, nil
}

func (m *memoryIngestStore) GetDiscovery(_ context.Context, _ int64) (*model.SourceDiscovery, error) {
	return nil, nil
}

func (m *memoryIngestStore) UpsertDiscovery(_ context.Context, _ model.SourceDiscovery) error {
	return nil
}

func (m *memoryIngestStore) GetTableIngestionCheckpoint(_ context.Context, _ int64) (*model.TableIngestionCheckpoint, error) {
	return nil, nil
}

func (m *memoryIngestStore) UpsertTableIngestionCheckpoint(_ context.Context, _ model.TableIngestionCheckpoint) error {
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
	if store.acquisitionStatus.AcquisitionState != model.AcquisitionStateHealthy {
		t.Fatalf("expected healthy acquisition status, got %q", store.acquisitionStatus.AcquisitionState)
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

type fakeAcquisitionService struct {
	result model.AcquisitionResult
	err    error
}

func (f fakeAcquisitionService) Acquire(context.Context, config.SourceConfig, *model.AcquisitionCheckpoint) (model.AcquisitionResult, error) {
	return f.result, f.err
}

func (f fakeAcquisitionService) AcquireMySQLFile(_ context.Context, _ config.SourceConfig, _ string, _ *model.AcquisitionCheckpoint) (model.AcquisitionResult, error) {
	return f.result, f.err
}

func TestCollectOnceKeepsParserHealthyWhenAcquisitionFailsButSpoolStillParses(t *testing.T) {
	dir := t.TempDir()
	spoolPath := filepath.Join(dir, "spool.log")
	content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "sample-slow.log"))
	if err != nil {
		t.Fatalf("read sample log: %v", err)
	}
	if err := os.WriteFile(spoolPath, content, 0o644); err != nil {
		t.Fatalf("write spool log: %v", err)
	}

	store := &memoryIngestStore{
		source: &model.Source{
			ID:           1,
			InstanceName: "remote-source",
			SlowLogPath:  "/var/log/mysql/slow.log",
			LogMode:      model.LogModeSSHPull,
		},
		acquisitionCheckpoint: &model.AcquisitionCheckpoint{
			SourceID:           1,
			TransportMode:      model.LogModeSSHPull,
			LastRemoteOffset:   128,
			LastSpoolSizeBytes: int64(len(content)),
		},
	}

	service := NewCollectorService(config.SourceConfig{
		InstanceName:       "remote-source",
		LogMode:            model.LogModeSSHPull,
		RemoteHost:         "db-prod",
		RemoteUser:         "observer",
		RemoteSlowLogPath:  "/var/log/mysql/slow.log",
		SSHPrivateKeyPath:  filepath.Join(dir, "id_rsa"),
		SSHKnownHostsPath:  filepath.Join(dir, "known_hosts"),
		LocalSpoolDir:      dir,
		InitialPosition:    model.InitialPositionEnd,
		LocalSpoolMaxBytes: 1024 * 1024,
	}, config.RuntimeConfig{CollectorPollInterval: time.Second}, store)
	service.acquire = fakeAcquisitionService{
		result: model.AcquisitionResult{
			ParsePath:         spoolPath,
			TransportMode:     model.LogModeSSHPull,
			RemoteAccessState: model.SourceAccessInaccessible,
			SpoolPath:         spoolPath,
			SpoolSizeBytes:    int64(len(content)),
			ShouldParse:       true,
			AcquisitionState:  model.AcquisitionStateError,
		},
		err: os.ErrDeadlineExceeded,
	}

	result, err := service.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("expected parser to continue using existing spool, got %v", err)
	}
	if result.EventsProcessed == 0 {
		t.Fatalf("expected remote spool to be parsed")
	}
	if store.acquisitionStatus.AcquisitionState != model.AcquisitionStateError {
		t.Fatalf("expected acquisition error state, got %q", store.acquisitionStatus.AcquisitionState)
	}
	if store.status.CollectorState != model.CollectorStateHealthy {
		t.Fatalf("expected collector status to stay healthy, got %q", store.status.CollectorState)
	}
}

func TestCollectOnceTruncatesFullyConsumedRemoteSpool(t *testing.T) {
	dir := t.TempDir()
	spoolPath := filepath.Join(dir, "spool.log")
	content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "sample-slow.log"))
	if err != nil {
		t.Fatalf("read sample log: %v", err)
	}
	if err := os.WriteFile(spoolPath, content, 0o644); err != nil {
		t.Fatalf("write spool log: %v", err)
	}

	store := &memoryIngestStore{
		source: &model.Source{
			ID:           1,
			InstanceName: "remote-source",
			SlowLogPath:  "/var/log/mysql/slow.log",
			LogMode:      model.LogModeSSHPull,
		},
	}
	service := NewCollectorService(config.SourceConfig{
		InstanceName:       "remote-source",
		LogMode:            model.LogModeSSHPull,
		RemoteHost:         "db-prod",
		RemoteUser:         "observer",
		RemoteSlowLogPath:  "/var/log/mysql/slow.log",
		SSHPrivateKeyPath:  filepath.Join(dir, "id_rsa"),
		SSHKnownHostsPath:  filepath.Join(dir, "known_hosts"),
		LocalSpoolDir:      dir,
		InitialPosition:    model.InitialPositionStart,
		LocalSpoolMaxBytes: 1024 * 1024,
	}, config.RuntimeConfig{CollectorPollInterval: time.Second}, store)
	service.acquire = fakeAcquisitionService{
		result: model.AcquisitionResult{
			ParsePath:          spoolPath,
			TransportMode:      model.LogModeSSHPull,
			RemoteAccessState:  model.SourceAccessAccessible,
			RemoteFileIdentity: "dev:inode",
			RemoteOffsetEnd:    int64(len(content)),
			SpoolPath:          spoolPath,
			SpoolSizeBytes:     int64(len(content)),
			ShouldParse:        true,
			AcquisitionState:   model.AcquisitionStateHealthy,
		},
	}

	if _, err := service.CollectOnce(context.Background()); err != nil {
		t.Fatalf("collect once returned error: %v", err)
	}
	info, err := os.Stat(spoolPath)
	if err != nil {
		t.Fatalf("stat spool: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected consumed spool to be truncated, got %d bytes", info.Size())
	}
	if store.checkpoint == nil || store.checkpoint.LastOffset != 0 {
		t.Fatalf("expected parser checkpoint to reset to 0 after truncation")
	}
	if store.acquisitionCheckpoint == nil || store.acquisitionCheckpoint.LastSpoolSizeBytes != 0 {
		t.Fatalf("expected acquisition checkpoint spool size to reset to 0")
	}
}
