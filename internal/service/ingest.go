package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"

	"slow-sql-observer/internal/acquisition"
	"slow-sql-observer/internal/collector"
	"slow-sql-observer/internal/config"
	"slow-sql-observer/internal/fingerprint"
	"slow-sql-observer/internal/model"
	"slow-sql-observer/internal/parser"
)

type IngestStore interface {
	GetSource(ctx context.Context) (*model.Source, error)
	GetCheckpoint(ctx context.Context, sourceID int64) (*model.CollectorCheckpoint, error)
	GetAcquisitionCheckpoint(ctx context.Context, sourceID int64) (*model.AcquisitionCheckpoint, error)
	IngestRecord(ctx context.Context, input model.IngestRecordInput) error
	UpdateSourceMetadata(ctx context.Context, sourceID int64, metadata model.SourceMetadataUpdate) error
	UpdateCollectorStatus(ctx context.Context, status model.CollectorStatus) error
	UpdateAcquisitionStatus(ctx context.Context, status model.AcquisitionStatus) error
	UpsertCheckpoint(ctx context.Context, checkpoint model.CollectorCheckpoint) error
	UpsertAcquisitionCheckpoint(ctx context.Context, checkpoint model.AcquisitionCheckpoint) error
	CleanupExpiredRecords(ctx context.Context, sourceID int64, olderThan time.Time) (int64, error)
}

type SourceProbe func(ctx context.Context, dsn string) (model.SourceMetadataUpdate, error)

type AcquisitionService interface {
	Acquire(ctx context.Context, source config.SourceConfig, checkpoint *model.AcquisitionCheckpoint) (model.AcquisitionResult, error)
}

type CollectorService struct {
	source     config.SourceConfig
	runtime    config.RuntimeConfig
	store      IngestStore
	parser     *parser.Parser
	normalizer *fingerprint.Normalizer
	framer     *collector.Framer
	probe      SourceProbe
	acquire    AcquisitionService
}

func NewCollectorService(source config.SourceConfig, runtime config.RuntimeConfig, store IngestStore) *CollectorService {
	if strings.TrimSpace(source.LogMode) == "" {
		source.LogMode = model.LogModeLocalFile
	}
	if strings.TrimSpace(source.InitialPosition) == "" {
		source.InitialPosition = model.InitialPositionEnd
	}
	return &CollectorService{
		source:     source,
		runtime:    runtime,
		store:      store,
		parser:     parser.New(),
		normalizer: fingerprint.NewNormalizer(),
		framer:     collector.NewFramer(),
		probe:      probeSourceDB,
		acquire:    acquisition.NewService(nil),
	}
}

func (s *CollectorService) CollectOnce(ctx context.Context) (model.CollectResult, error) {
	source, err := s.store.GetSource(ctx)
	if err != nil {
		return model.CollectResult{}, err
	}

	probeErr := s.applySourceProbe(ctx, source)
	parserCheckpoint, err := s.store.GetCheckpoint(ctx, source.ID)
	if err != nil {
		s.updateErrorStatus(ctx, source.ID, model.SourceAccessAccessible, nil, "", err)
		return model.CollectResult{}, err
	}

	acquisitionCheckpoint, err := s.store.GetAcquisitionCheckpoint(ctx, source.ID)
	if err != nil {
		s.updateErrorStatus(ctx, source.ID, model.SourceAccessUnknown, parserCheckpoint, "", err)
		return model.CollectResult{}, err
	}

	acquisitionResult, acquisitionErr := s.acquire.Acquire(ctx, s.source, acquisitionCheckpoint)
	if err := s.persistAcquisitionState(ctx, source, acquisitionResult, acquisitionErr); err != nil {
		return model.CollectResult{}, err
	}
	if acquisitionErr != nil && !acquisitionResult.ShouldParse {
		return model.CollectResult{}, acquisitionErr
	}

	parsePath := acquisitionResult.ParsePath
	if strings.TrimSpace(parsePath) == "" {
		parsePath = s.source.EffectiveParsePath()
	}

	state, err := collector.StatFile(parsePath)
	if err != nil {
		s.updateErrorStatus(ctx, source.ID, normalizeAccessState(acquisitionResult.RemoteAccessState), parserCheckpoint, "", err)
		return model.CollectResult{}, err
	}

	startOffset := collector.ResolveStartOffset(parserCheckpoint, state)
	state, blocks, err := s.framer.ReadNewBlocks(ctx, parsePath, startOffset)
	if err != nil {
		s.updateErrorStatus(ctx, source.ID, normalizeAccessState(acquisitionResult.RemoteAccessState), parserCheckpoint, state.Identity, err)
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
			s.updateErrorStatus(ctx, source.ID, normalizeAccessState(acquisitionResult.RemoteAccessState), parserCheckpoint, state.Identity, fmt.Errorf("parse block at offset %d: %w", block.StartOffset, err))
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
			s.updateErrorStatus(ctx, source.ID, normalizeAccessState(acquisitionResult.RemoteAccessState), parserCheckpoint, state.Identity, fmt.Errorf("ingest block at offset %d: %w", block.StartOffset, err))
			return result, fmt.Errorf("ingest block at offset %d: %w", block.StartOffset, err)
		}

		result.EventsProcessed++
		result.FinalOffset = block.EndOffset
	}

	result.BytesRead = state.Size - startOffset
	lastOffset := startOffset
	if result.EventsProcessed > 0 {
		lastOffset = result.FinalOffset
	}

	resetCheckpoint, truncated, truncateErr := s.maybeResetSpool(ctx, source, acquisitionResult, state, lastOffset)
	if truncated {
		lastOffset = 0
		result.FinalOffset = 0
		if resetCheckpoint != nil {
			result.FileIdentity = resetCheckpoint.FileIdentity
		}
		acquisitionResult.ShouldTruncate = true
		acquisitionResult.SpoolSizeBytes = 0
		if err := s.persistAcquisitionState(ctx, source, acquisitionResult, acquisitionErr); err != nil {
			return result, err
		}
	}

	retentionErr := s.runRetention(ctx, source.ID)
	statusErr := combineStatusErrors(probeErr, retentionErr, truncateErr)
	if statusErr != nil {
		s.updateStatus(ctx, model.CollectorStatus{
			SourceID:               source.ID,
			CollectorState:         model.CollectorStateDegraded,
			SourceAccessState:      normalizeAccessState(acquisitionResult.RemoteAccessState),
			LastSuccessfulIngestAt: timePtr(time.Now().UTC()),
			LastCheckpointOffset:   int64Ptr(lastOffset),
			LastFileIdentity:       stringPtr(result.FileIdentity),
			LastErrorAt:            timePtr(time.Now().UTC()),
			LastErrorMessage:       stringPtr(statusErr.Error()),
		})
		return result, nil
	}

	if err := s.updateStatus(ctx, model.CollectorStatus{
		SourceID:               source.ID,
		CollectorState:         model.CollectorStateHealthy,
		SourceAccessState:      normalizeAccessState(acquisitionResult.RemoteAccessState),
		LastSuccessfulIngestAt: timePtr(time.Now().UTC()),
		LastCheckpointOffset:   int64Ptr(lastOffset),
		LastFileIdentity:       stringPtr(result.FileIdentity),
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
		SourceAccessState: normalizeAccessState(accessState),
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

func (s *CollectorService) persistAcquisitionState(ctx context.Context, source *model.Source, result model.AcquisitionResult, cycleErr error) error {
	status := model.AcquisitionStatus{
		SourceID:          source.ID,
		AcquisitionState:  normalizeAcquisitionState(result.AcquisitionState, cycleErr),
		RemoteAccessState: normalizeAccessState(result.RemoteAccessState),
		TransportMode:     firstNonEmpty(result.TransportMode, source.LogMode, model.LogModeLocalFile),
	}
	if cycleErr == nil {
		status.LastSuccessfulPullAt = timePtr(time.Now().UTC())
	} else {
		status.LastErrorAt = timePtr(time.Now().UTC())
		status.LastErrorMessage = stringPtr(cycleErr.Error())
	}
	if shouldPersistRemoteOffset(result) {
		status.LastRemoteOffset = int64Ptr(result.RemoteOffsetEnd)
	}
	if strings.TrimSpace(result.RemoteFileIdentity) != "" {
		status.LastRemoteFileIdentity = stringPtr(result.RemoteFileIdentity)
	}
	if strings.TrimSpace(result.SpoolPath) != "" {
		status.LastSpoolSizeBytes = int64Ptr(result.SpoolSizeBytes)
	}
	if err := s.store.UpdateAcquisitionStatus(ctx, status); err != nil {
		return fmt.Errorf("update acquisition status: %w", err)
	}

	if source.LogMode != model.LogModeSSHPull {
		return nil
	}

	checkpoint := model.AcquisitionCheckpoint{
		SourceID:           source.ID,
		TransportMode:      status.TransportMode,
		RemoteHost:         normalizeNullableString(source.RemoteHost),
		RemotePath:         nullableTrimmedPtr(source.SlowLogPath),
		RemoteFileIdentity: nullableTrimmedPtr(result.RemoteFileIdentity),
		LastRemoteOffset:   result.RemoteOffsetEnd,
		LocalSpoolPath:     nullableTrimmedPtr(result.SpoolPath),
		LastSpoolSizeBytes: result.SpoolSizeBytes,
		InitialPosition:    firstNonEmpty(source.InitialPosition, model.InitialPositionEnd),
	}
	if err := s.store.UpsertAcquisitionCheckpoint(ctx, checkpoint); err != nil {
		return fmt.Errorf("update acquisition checkpoint: %w", err)
	}
	return nil
}

func (s *CollectorService) maybeResetSpool(ctx context.Context, source *model.Source, acquisitionResult model.AcquisitionResult, state collector.FileState, lastOffset int64) (*model.CollectorCheckpoint, bool, error) {
	if source.LogMode != model.LogModeSSHPull {
		return nil, false, nil
	}
	if strings.TrimSpace(acquisitionResult.SpoolPath) == "" || state.Size == 0 {
		return nil, false, nil
	}
	if s.framer.HasPending() || lastOffset != state.Size {
		return nil, false, nil
	}
	if err := os.Truncate(acquisitionResult.SpoolPath, 0); err != nil {
		return nil, false, fmt.Errorf("truncate spool file: %w", err)
	}
	s.framer.Reset()

	resetState, err := collector.StatFile(acquisitionResult.SpoolPath)
	if err != nil {
		return nil, true, fmt.Errorf("stat truncated spool file: %w", err)
	}
	checkpoint := model.CollectorCheckpoint{
		SourceID:     source.ID,
		LogFilePath:  acquisitionResult.SpoolPath,
		FileIdentity: resetState.Identity,
		LastOffset:   0,
	}
	if err := s.store.UpsertCheckpoint(ctx, checkpoint); err != nil {
		return nil, true, fmt.Errorf("reset parser checkpoint after spool truncate: %w", err)
	}
	return &checkpoint, true, nil
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

func normalizeAccessState(value string) string {
	switch strings.TrimSpace(value) {
	case model.SourceAccessAccessible, model.SourceAccessInaccessible:
		return value
	default:
		return model.SourceAccessUnknown
	}
}

func normalizeAcquisitionState(value string, cycleErr error) string {
	switch strings.TrimSpace(value) {
	case model.AcquisitionStateHealthy, model.AcquisitionStateDegraded, model.AcquisitionStateError, model.AcquisitionStateBlocked:
		return value
	case model.AcquisitionStateIdle:
		if cycleErr == nil {
			return model.AcquisitionStateHealthy
		}
		return model.AcquisitionStateError
	default:
		if cycleErr == nil {
			return model.AcquisitionStateHealthy
		}
		return model.AcquisitionStateError
	}
}

func shouldPersistRemoteOffset(result model.AcquisitionResult) bool {
	if result.TransportMode == model.LogModeLocalFile {
		return true
	}
	return result.RemoteOffsetEnd > 0 || strings.TrimSpace(result.RemoteFileIdentity) != ""
}

func nullableTrimmedPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeNullableString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
