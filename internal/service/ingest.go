package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"

	"slow-sql-observer/internal/collector"
	"slow-sql-observer/internal/config"
	"slow-sql-observer/internal/fingerprint"
	"slow-sql-observer/internal/model"
	"slow-sql-observer/internal/parser"
)

type IngestStore interface {
	GetSource(ctx context.Context) (*model.Source, error)
	GetCheckpoint(ctx context.Context, sourceID int64) (*model.CollectorCheckpoint, error)
	IngestRecord(ctx context.Context, input model.IngestRecordInput) error
	UpdateSourceMetadata(ctx context.Context, sourceID int64, metadata model.SourceMetadataUpdate) error
	UpdateCollectorStatus(ctx context.Context, status model.CollectorStatus) error
	CleanupExpiredRecords(ctx context.Context, sourceID int64, olderThan time.Time) (int64, error)
}

type SourceProbe func(ctx context.Context, dsn string) (model.SourceMetadataUpdate, error)

type CollectorService struct {
	source     config.SourceConfig
	runtime    config.RuntimeConfig
	store      IngestStore
	parser     *parser.Parser
	normalizer *fingerprint.Normalizer
	framer     *collector.Framer
	probe      SourceProbe
}

func NewCollectorService(source config.SourceConfig, runtime config.RuntimeConfig, store IngestStore) *CollectorService {
	return &CollectorService{
		source:     source,
		runtime:    runtime,
		store:      store,
		parser:     parser.New(),
		normalizer: fingerprint.NewNormalizer(),
		framer:     collector.NewFramer(),
		probe:      probeSourceDB,
	}
}

func (s *CollectorService) CollectOnce(ctx context.Context) (model.CollectResult, error) {
	source, err := s.store.GetSource(ctx)
	if err != nil {
		return model.CollectResult{}, err
	}

	probeErr := s.applySourceProbe(ctx, source)
	checkpoint, err := s.store.GetCheckpoint(ctx, source.ID)
	if err != nil {
		s.updateErrorStatus(ctx, source.ID, model.SourceAccessAccessible, checkpoint, "", err)
		return model.CollectResult{}, err
	}

	state, err := collector.StatFile(s.source.SlowLogPath)
	if err != nil {
		s.updateErrorStatus(ctx, source.ID, model.SourceAccessInaccessible, checkpoint, "", err)
		return model.CollectResult{}, err
	}

	startOffset := collector.ResolveStartOffset(checkpoint, state)
	state, blocks, err := s.framer.ReadNewBlocks(ctx, s.source.SlowLogPath, startOffset)
	if err != nil {
		s.updateErrorStatus(ctx, source.ID, model.SourceAccessInaccessible, checkpoint, state.Identity, err)
		return model.CollectResult{}, err
	}

	result := model.CollectResult{
		FileIdentity: state.Identity,
		StartOffset:  startOffset,
		FinalOffset:  startOffset,
	}

	for _, block := range blocks {
		record, err := s.parser.Parse(block.Raw)
		if err != nil {
			s.updateErrorStatus(ctx, source.ID, model.SourceAccessAccessible, checkpoint, state.Identity, fmt.Errorf("parse block at offset %d: %w", block.StartOffset, err))
			return result, fmt.Errorf("parse block at offset %d: %w", block.StartOffset, err)
		}

		processed := s.normalizer.Process(record.RawSQL)
		record.SourceID = source.ID
		record.SourceInstance = source.InstanceName
		record.SourceLogFilePath = source.SlowLogPath
		record.SourceFileID = state.Identity
		record.SourceOffsetStart = block.StartOffset
		record.SourceOffsetEnd = block.EndOffset
		record.NormalizedSQL = processed.NormalizedSQL
		record.FingerprintHash = processed.Hash
		record.CreatedAt = time.Now().UTC()

		if err := s.store.IngestRecord(ctx, model.IngestRecordInput{
			Record:      record,
			Fingerprint: processed,
		}); err != nil {
			s.updateErrorStatus(ctx, source.ID, model.SourceAccessAccessible, checkpoint, state.Identity, fmt.Errorf("ingest block at offset %d: %w", block.StartOffset, err))
			return result, fmt.Errorf("ingest block at offset %d: %w", block.StartOffset, err)
		}

		result.EventsProcessed++
		result.FinalOffset = block.EndOffset
	}

	result.BytesRead = state.Size - startOffset
	lastOffset := startOffset
	if result.EventsProcessed > 0 {
		lastOffset = result.FinalOffset
	} else if checkpoint != nil {
		lastOffset = checkpoint.LastOffset
	}

	retentionErr := s.runRetention(ctx, source.ID)
	statusErr := combineStatusErrors(probeErr, retentionErr)
	if statusErr != nil {
		s.updateStatus(ctx, model.CollectorStatus{
			SourceID:               source.ID,
			CollectorState:         model.CollectorStateDegraded,
			SourceAccessState:      model.SourceAccessAccessible,
			LastSuccessfulIngestAt: timePtr(time.Now().UTC()),
			LastCheckpointOffset:   int64Ptr(lastOffset),
			LastFileIdentity:       stringPtr(state.Identity),
			LastErrorAt:            timePtr(time.Now().UTC()),
			LastErrorMessage:       stringPtr(statusErr.Error()),
		})
		return result, nil
	}

	if err := s.updateStatus(ctx, model.CollectorStatus{
		SourceID:               source.ID,
		CollectorState:         model.CollectorStateHealthy,
		SourceAccessState:      model.SourceAccessAccessible,
		LastSuccessfulIngestAt: timePtr(time.Now().UTC()),
		LastCheckpointOffset:   int64Ptr(lastOffset),
		LastFileIdentity:       stringPtr(state.Identity),
	}); err != nil {
		return result, err
	}
	return result, nil
}

func (s *CollectorService) applySourceProbe(ctx context.Context, source *model.Source) error {
	if strings.TrimSpace(s.source.DatabaseDSN) == "" || s.probe == nil {
		return nil
	}
	metadata, err := s.probe(ctx, s.source.DatabaseDSN)
	if err != nil {
		return fmt.Errorf("probe source db: %w", err)
	}
	if err := s.store.UpdateSourceMetadata(ctx, source.ID, metadata); err != nil {
		return fmt.Errorf("update source metadata: %w", err)
	}
	return nil
}

func (s *CollectorService) runRetention(ctx context.Context, sourceID int64) error {
	if s.runtime.RawRecordRetentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -s.runtime.RawRecordRetentionDays)
	_, err := s.store.CleanupExpiredRecords(ctx, sourceID, cutoff)
	if err != nil {
		return fmt.Errorf("cleanup expired records: %w", err)
	}
	return nil
}

func (s *CollectorService) updateErrorStatus(ctx context.Context, sourceID int64, accessState string, checkpoint *model.CollectorCheckpoint, fileIdentity string, err error) {
	status := model.CollectorStatus{
		SourceID:          sourceID,
		CollectorState:    model.CollectorStateError,
		SourceAccessState: accessState,
		LastErrorAt:       timePtr(time.Now().UTC()),
		LastErrorMessage:  stringPtr(err.Error()),
	}
	if checkpoint != nil {
		status.LastCheckpointOffset = int64Ptr(checkpoint.LastOffset)
	}
	if strings.TrimSpace(fileIdentity) != "" {
		status.LastFileIdentity = stringPtr(fileIdentity)
	}
	_ = s.updateStatus(ctx, status)
}

func (s *CollectorService) updateStatus(ctx context.Context, status model.CollectorStatus) error {
	return s.store.UpdateCollectorStatus(ctx, status)
}

func probeSourceDB(ctx context.Context, dsn string) (model.SourceMetadataUpdate, error) {
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		return model.SourceMetadataUpdate{}, err
	}
	parsed.ParseTime = true

	db, err := sql.Open("mysql", parsed.FormatDSN())
	if err != nil {
		return model.SourceMetadataUpdate{}, err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return model.SourceMetadataUpdate{}, err
	}

	var version string
	if err := db.QueryRowContext(ctx, `SELECT VERSION()`).Scan(&version); err != nil {
		return model.SourceMetadataUpdate{}, err
	}
	host := parsed.Addr
	return model.SourceMetadataUpdate{
		DatabaseHost:    nullableTrimmed(host),
		DatabaseVersion: nullableTrimmed(version),
	}, nil
}

func combineStatusErrors(values ...error) error {
	var messages []string
	for _, err := range values {
		if err != nil {
			messages = append(messages, err.Error())
		}
	}
	if len(messages) == 0 {
		return nil
	}
	return errors.New(strings.Join(messages, "; "))
}

func nullableTrimmed(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}
