package service

import (
	"context"
	"fmt"
	"time"

	"slow-sql-observer/internal/collector"
	"slow-sql-observer/internal/fingerprint"
	"slow-sql-observer/internal/model"
	"slow-sql-observer/internal/parser"
)

type IngestStore interface {
	GetCheckpoint(ctx context.Context, instanceName, logFilePath string) (*model.CollectorCheckpoint, error)
	IngestRecord(ctx context.Context, input model.IngestRecordInput) error
}

type CollectorService struct {
	instanceName string
	logPath      string
	store        IngestStore
	parser       *parser.Parser
	normalizer   *fingerprint.Normalizer
	framer       *collector.Framer
}

func NewCollectorService(instanceName, logPath string, store IngestStore) *CollectorService {
	return &CollectorService{
		instanceName: instanceName,
		logPath:      logPath,
		store:        store,
		parser:       parser.New(),
		normalizer:   fingerprint.NewNormalizer(),
		framer:       collector.NewFramer(),
	}
}

func (s *CollectorService) CollectOnce(ctx context.Context) (model.CollectResult, error) {
	checkpoint, err := s.store.GetCheckpoint(ctx, s.instanceName, s.logPath)
	if err != nil {
		return model.CollectResult{}, err
	}

	state, err := collector.StatFile(s.logPath)
	if err != nil {
		return model.CollectResult{}, err
	}

	startOffset := collector.ResolveStartOffset(checkpoint, state)
	state, blocks, err := s.framer.ReadNewBlocks(ctx, s.logPath, startOffset)
	if err != nil {
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
			return result, fmt.Errorf("parse block at offset %d: %w", block.StartOffset, err)
		}

		processed := s.normalizer.Process(record.RawSQL)
		record.SourceInstance = s.instanceName
		record.SourceLogFilePath = s.logPath
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
			return result, fmt.Errorf("ingest block at offset %d: %w", block.StartOffset, err)
		}

		result.EventsProcessed++
		result.FinalOffset = block.EndOffset
	}

	result.BytesRead = state.Size - startOffset
	return result, nil
}
