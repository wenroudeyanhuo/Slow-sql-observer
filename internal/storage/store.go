package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"

	"slow-sql-observer/internal/config"
	"slow-sql-observer/internal/model"
)

type Store struct {
	db           *sql.DB
	schema       string
	activeSource *model.Source
}

func Open(ctx context.Context, cfg config.AnalysisConfig, source *config.SourceConfig) (*Store, error) {
	adminCfg, err := mysql.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	adminCfg.ParseTime = true
	adminCfg.MultiStatements = true

	adminDB, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	defer adminDB.Close()

	if err := adminDB.PingContext(ctx); err != nil {
		return nil, err
	}
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", cfg.Schema)); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}

	appCfg := *adminCfg
	appCfg.DBName = cfg.Schema
	db, err := sql.Open("mysql", appCfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	store := &Store{db: db, schema: cfg.Schema}
	if err := store.EnsureSchema(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if source != nil {
		active, err := store.ensureSource(ctx, *source)
		if err != nil {
			db.Close()
			return nil, err
		}
		if err := store.migrateLegacySourceTables(ctx, active); err != nil {
			db.Close()
			return nil, err
		}
		store.activeSource = active
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS sources (
			id BIGINT NOT NULL AUTO_INCREMENT,
			source_key CHAR(40) NOT NULL,
			source_instance_name VARCHAR(128) NOT NULL,
			source_slow_log_path VARCHAR(1024) NOT NULL,
			source_description TEXT NULL,
			source_db_dsn_configured BOOLEAN NOT NULL,
			source_db_host VARCHAR(255) NULL,
			source_db_version VARCHAR(255) NULL,
			source_log_mode VARCHAR(32) NOT NULL DEFAULT 'local_file',
			source_remote_host VARCHAR(255) NULL,
			source_remote_port INT NULL,
			source_remote_user VARCHAR(255) NULL,
			source_remote_slow_log_path VARCHAR(1024) NULL,
			source_local_spool_path VARCHAR(1024) NULL,
			source_initial_position VARCHAR(16) NOT NULL DEFAULT 'end',
			source_local_spool_max_bytes BIGINT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_source_key (source_key)
		)`,
		`CREATE TABLE IF NOT EXISTS collector_status (
			source_id BIGINT NOT NULL,
			collector_state VARCHAR(32) NOT NULL,
			source_access_state VARCHAR(32) NOT NULL,
			last_successful_ingest_at DATETIME(6) NULL,
			last_checkpoint_offset BIGINT NULL,
			last_file_identity VARCHAR(255) NULL,
			last_error_at DATETIME(6) NULL,
			last_error_message TEXT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (source_id),
			CONSTRAINT fk_status_source FOREIGN KEY (source_id) REFERENCES sources(id)
		)`,
		`CREATE TABLE IF NOT EXISTS collector_checkpoints (
			id BIGINT NOT NULL AUTO_INCREMENT,
			source_id BIGINT NOT NULL,
			log_file_path VARCHAR(1024) NOT NULL,
			log_file_path_hash CHAR(40) NOT NULL,
			file_identity VARCHAR(255) NOT NULL,
			last_offset BIGINT NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_checkpoint_source (source_id),
			CONSTRAINT fk_checkpoint_source FOREIGN KEY (source_id) REFERENCES sources(id)
		)`,
		`CREATE TABLE IF NOT EXISTS acquisition_checkpoints (
			source_id BIGINT NOT NULL,
			transport_mode VARCHAR(32) NOT NULL,
			remote_host VARCHAR(255) NULL,
			remote_path VARCHAR(1024) NULL,
			remote_file_identity VARCHAR(255) NULL,
			last_remote_offset BIGINT NOT NULL,
			local_spool_path VARCHAR(1024) NULL,
			last_spool_size_bytes BIGINT NOT NULL,
			initial_position VARCHAR(16) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (source_id),
			CONSTRAINT fk_acq_checkpoint_source FOREIGN KEY (source_id) REFERENCES sources(id)
		)`,
		`CREATE TABLE IF NOT EXISTS acquisition_status (
			source_id BIGINT NOT NULL,
			acquisition_state VARCHAR(32) NOT NULL,
			remote_access_state VARCHAR(32) NOT NULL,
			transport_mode VARCHAR(32) NOT NULL,
			last_successful_pull_at DATETIME(6) NULL,
			last_remote_offset BIGINT NULL,
			last_remote_file_identity VARCHAR(255) NULL,
			last_spool_size_bytes BIGINT NULL,
			last_error_at DATETIME(6) NULL,
			last_error_message TEXT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (source_id),
			CONSTRAINT fk_acq_status_source FOREIGN KEY (source_id) REFERENCES sources(id)
		)`,
		`CREATE TABLE IF NOT EXISTS fingerprints (
			id BIGINT NOT NULL AUTO_INCREMENT,
			source_id BIGINT NOT NULL,
			fingerprint_hash CHAR(40) NOT NULL,
			normalized_sql LONGTEXT NOT NULL,
			sql_type VARCHAR(32) NOT NULL,
			main_table_name VARCHAR(255) NULL,
			first_seen_at DATETIME(6) NOT NULL,
			last_seen_at DATETIME(6) NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_fingerprint_hash (source_id, fingerprint_hash),
			KEY idx_fingerprints_source_type_table (source_id, sql_type, main_table_name),
			CONSTRAINT fk_fingerprint_source FOREIGN KEY (source_id) REFERENCES sources(id)
		)`,
		`CREATE TABLE IF NOT EXISTS fingerprint_stats (
			fingerprint_id BIGINT NOT NULL,
			total_count BIGINT NOT NULL,
			total_query_time_sec DOUBLE NOT NULL,
			avg_query_time_sec DOUBLE NOT NULL,
			max_query_time_sec DOUBLE NOT NULL,
			total_rows_examined BIGINT NOT NULL,
			avg_rows_examined DOUBLE NOT NULL,
			max_rows_examined BIGINT NOT NULL,
			total_rows_sent BIGINT NOT NULL,
			avg_rows_sent DOUBLE NOT NULL,
			max_rows_sent BIGINT NOT NULL,
			first_seen_at DATETIME(6) NOT NULL,
			last_seen_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (fingerprint_id),
			CONSTRAINT fk_stats_fingerprint FOREIGN KEY (fingerprint_id) REFERENCES fingerprints(id)
		)`,
		`CREATE TABLE IF NOT EXISTS slow_query_records (
			id BIGINT NOT NULL AUTO_INCREMENT,
			source_id BIGINT NOT NULL,
			source_instance_name VARCHAR(128) NOT NULL,
			source_log_file_path VARCHAR(1024) NOT NULL,
			source_log_file_path_hash CHAR(40) NOT NULL,
			source_file_identity VARCHAR(255) NOT NULL,
			source_offset_start BIGINT NOT NULL,
			source_offset_end BIGINT NOT NULL,
			occurred_at DATETIME(6) NOT NULL,
			db_name VARCHAR(255) NULL,
			user_name VARCHAR(255) NULL,
			client_host VARCHAR(255) NULL,
			raw_block LONGTEXT NOT NULL,
			raw_sql LONGTEXT NOT NULL,
			normalized_sql LONGTEXT NOT NULL,
			fingerprint_id BIGINT NOT NULL,
			fingerprint_hash CHAR(40) NOT NULL,
			query_time_sec DOUBLE NOT NULL,
			lock_time_sec DOUBLE NULL,
			rows_sent BIGINT NULL,
			rows_examined BIGINT NULL,
			created_at DATETIME(6) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_record_source (source_id, source_log_file_path_hash, source_file_identity, source_offset_start),
			KEY idx_records_source_fingerprint_time (source_id, fingerprint_id, occurred_at DESC),
			KEY idx_records_source_occurred_at (source_id, occurred_at DESC),
			KEY idx_records_source_query_time (source_id, query_time_sec DESC),
			CONSTRAINT fk_records_source FOREIGN KEY (source_id) REFERENCES sources(id),
			CONSTRAINT fk_records_fingerprint FOREIGN KEY (fingerprint_id) REFERENCES fingerprints(id)
		)`,
	}

	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure schema: %w", err)
		}
	}
	return nil
}

func (s *Store) migrateLegacySourceTables(ctx context.Context, source *model.Source) error {
	if source == nil {
		return nil
	}
	if err := s.ensureColumn(ctx, "sources", "`source_log_mode` VARCHAR(32) NOT NULL DEFAULT 'local_file' AFTER `source_db_version`"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "sources", "`source_remote_host` VARCHAR(255) NULL AFTER `source_log_mode`"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "sources", "`source_remote_port` INT NULL AFTER `source_remote_host`"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "sources", "`source_remote_user` VARCHAR(255) NULL AFTER `source_remote_port`"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "sources", "`source_remote_slow_log_path` VARCHAR(1024) NULL AFTER `source_remote_user`"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "sources", "`source_local_spool_path` VARCHAR(1024) NULL AFTER `source_remote_slow_log_path`"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "sources", "`source_initial_position` VARCHAR(16) NOT NULL DEFAULT 'end' AFTER `source_local_spool_path`"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "sources", "`source_local_spool_max_bytes` BIGINT NULL AFTER `source_initial_position`"); err != nil {
		return err
	}

	if err := s.ensureColumn(ctx, "collector_checkpoints", "`source_id` BIGINT NULL AFTER `id`"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "collector_checkpoints", "`log_file_path_hash` CHAR(40) NOT NULL DEFAULT '' AFTER `log_file_path`"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE collector_checkpoints
		SET source_id = ?,
		    log_file_path_hash = CASE
		    	WHEN log_file_path_hash = '' THEN SHA1(log_file_path)
		    	ELSE log_file_path_hash
		    END
		WHERE source_id IS NULL OR source_id = 0 OR log_file_path_hash = ''`,
		source.ID,
	); err != nil {
		return fmt.Errorf("migrate collector_checkpoints rows: %w", err)
	}
	if err := s.ensureIndex(ctx, "collector_checkpoints", "uk_checkpoint_source", true, []string{"source_id"}, "(`source_id`)"); err != nil {
		return err
	}

	if err := s.ensureColumn(ctx, "fingerprints", "`source_id` BIGINT NULL AFTER `id`"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE fingerprints
		SET source_id = ?
		WHERE source_id IS NULL OR source_id = 0`,
		source.ID,
	); err != nil {
		return fmt.Errorf("migrate fingerprints rows: %w", err)
	}
	if err := s.ensureIndex(ctx, "fingerprints", "uk_fingerprint_hash", true, []string{"source_id", "fingerprint_hash"}, "(`source_id`, `fingerprint_hash`)"); err != nil {
		return err
	}
	if err := s.ensureIndex(ctx, "fingerprints", "idx_fingerprints_source_type_table", false, []string{"source_id", "sql_type", "main_table_name"}, "(`source_id`, `sql_type`, `main_table_name`)"); err != nil {
		return err
	}

	if err := s.ensureColumn(ctx, "slow_query_records", "`source_id` BIGINT NULL AFTER `id`"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "slow_query_records", "`source_log_file_path_hash` CHAR(40) NOT NULL DEFAULT '' AFTER `source_log_file_path`"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE slow_query_records
		SET source_id = ?,
		    source_instance_name = CASE
		    	WHEN source_instance_name = '' THEN ?
		    	ELSE source_instance_name
		    END,
		    source_log_file_path_hash = CASE
		    	WHEN source_log_file_path_hash = '' THEN SHA1(source_log_file_path)
		    	ELSE source_log_file_path_hash
		    END
		WHERE source_id IS NULL OR source_id = 0 OR source_instance_name = '' OR source_log_file_path_hash = ''`,
		source.ID,
		source.InstanceName,
	); err != nil {
		return fmt.Errorf("migrate slow_query_records rows: %w", err)
	}
	if err := s.ensureIndex(ctx, "slow_query_records", "uk_record_source", true, []string{"source_id", "source_log_file_path_hash", "source_file_identity", "source_offset_start"}, "(`source_id`, `source_log_file_path_hash`, `source_file_identity`, `source_offset_start`)"); err != nil {
		return err
	}
	if err := s.ensureIndex(ctx, "slow_query_records", "idx_records_source_fingerprint_time", false, []string{"source_id", "fingerprint_id", "occurred_at"}, "(`source_id`, `fingerprint_id`, `occurred_at` DESC)"); err != nil {
		return err
	}
	if err := s.ensureIndex(ctx, "slow_query_records", "idx_records_source_occurred_at", false, []string{"source_id", "occurred_at"}, "(`source_id`, `occurred_at` DESC)"); err != nil {
		return err
	}
	if err := s.ensureIndex(ctx, "slow_query_records", "idx_records_source_query_time", false, []string{"source_id", "query_time_sec"}, "(`source_id`, `query_time_sec` DESC)"); err != nil {
		return err
	}
	if err := s.rebuildSourceAggregates(ctx, source.ID); err != nil {
		return err
	}

	return nil
}

func (s *Store) rebuildSourceAggregates(ctx context.Context, sourceID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE fingerprints f
		JOIN (
			SELECT fingerprint_id, MIN(occurred_at) AS first_seen_at, MAX(occurred_at) AS last_seen_at
			FROM slow_query_records
			WHERE source_id = ?
			GROUP BY fingerprint_id
		) agg ON agg.fingerprint_id = f.id
		SET f.first_seen_at = agg.first_seen_at,
		    f.last_seen_at = agg.last_seen_at,
		    f.updated_at = ?
		WHERE f.source_id = ?`,
		sourceID,
		now,
		sourceID,
	); err != nil {
		return fmt.Errorf("rebuild fingerprints timestamps: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE fs
		FROM fingerprint_stats fs
		JOIN fingerprints f ON f.id = fs.fingerprint_id
		WHERE f.source_id = ?`,
		sourceID,
	); err != nil {
		return fmt.Errorf("delete fingerprint stats: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO fingerprint_stats (
			fingerprint_id,
			total_count,
			total_query_time_sec,
			avg_query_time_sec,
			max_query_time_sec,
			total_rows_examined,
			avg_rows_examined,
			max_rows_examined,
			total_rows_sent,
			avg_rows_sent,
			max_rows_sent,
			first_seen_at,
			last_seen_at,
			updated_at
		)
		SELECT
			r.fingerprint_id,
			COUNT(*),
			COALESCE(SUM(r.query_time_sec), 0),
			COALESCE(AVG(r.query_time_sec), 0),
			COALESCE(MAX(r.query_time_sec), 0),
			COALESCE(SUM(r.rows_examined), 0),
			COALESCE(AVG(r.rows_examined), 0),
			COALESCE(MAX(r.rows_examined), 0),
			COALESCE(SUM(r.rows_sent), 0),
			COALESCE(AVG(r.rows_sent), 0),
			COALESCE(MAX(r.rows_sent), 0),
			MIN(r.occurred_at),
			MAX(r.occurred_at),
			?
		FROM slow_query_records r
		WHERE r.source_id = ?
		GROUP BY r.fingerprint_id`,
		now,
		sourceID,
	); err != nil {
		return fmt.Errorf("rebuild fingerprint stats: %w", err)
	}

	return tx.Commit()
}

func (s *Store) GetSource(ctx context.Context) (*model.Source, error) {
	if s.activeSource != nil {
		source := *s.activeSource
		return &source, nil
	}
	return nil, fmt.Errorf("active source is not configured")
}

func (s *Store) GetAcquisitionStatus(ctx context.Context) (*model.AcquisitionStatus, error) {
	source, err := s.GetSource(ctx)
	if err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT source_id, acquisition_state, remote_access_state, transport_mode, last_successful_pull_at,
		       last_remote_offset, last_remote_file_identity, last_spool_size_bytes, last_error_at, last_error_message, updated_at
		FROM acquisition_status
		WHERE source_id = ?`,
		source.ID,
	)
	var status model.AcquisitionStatus
	var lastSuccessful sql.NullTime
	var lastRemoteOffset sql.NullInt64
	var lastRemoteIdentity sql.NullString
	var lastSpoolSize sql.NullInt64
	var lastErrorAt sql.NullTime
	var lastErrorMessage sql.NullString
	if err := row.Scan(
		&status.SourceID,
		&status.AcquisitionState,
		&status.RemoteAccessState,
		&status.TransportMode,
		&lastSuccessful,
		&lastRemoteOffset,
		&lastRemoteIdentity,
		&lastSpoolSize,
		&lastErrorAt,
		&lastErrorMessage,
		&status.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			now := time.Now().UTC()
			return &model.AcquisitionStatus{
				SourceID:          source.ID,
				AcquisitionState:  model.AcquisitionStateIdle,
				RemoteAccessState: model.SourceAccessUnknown,
				TransportMode:     source.LogMode,
				UpdatedAt:         now,
			}, nil
		}
		return nil, err
	}
	if lastSuccessful.Valid {
		status.LastSuccessfulPullAt = &lastSuccessful.Time
	}
	if lastRemoteOffset.Valid {
		value := lastRemoteOffset.Int64
		status.LastRemoteOffset = &value
	}
	if lastRemoteIdentity.Valid {
		value := lastRemoteIdentity.String
		status.LastRemoteFileIdentity = &value
	}
	if lastSpoolSize.Valid {
		value := lastSpoolSize.Int64
		status.LastSpoolSizeBytes = &value
	}
	if lastErrorAt.Valid {
		status.LastErrorAt = &lastErrorAt.Time
	}
	if lastErrorMessage.Valid {
		value := lastErrorMessage.String
		status.LastErrorMessage = &value
	}
	return &status, nil
}

func (s *Store) UpdateAcquisitionStatus(ctx context.Context, status model.AcquisitionStatus) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO acquisition_status (
			source_id, acquisition_state, remote_access_state, transport_mode, last_successful_pull_at,
			last_remote_offset, last_remote_file_identity, last_spool_size_bytes, last_error_at, last_error_message, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			acquisition_state = VALUES(acquisition_state),
			remote_access_state = VALUES(remote_access_state),
			transport_mode = VALUES(transport_mode),
			last_successful_pull_at = VALUES(last_successful_pull_at),
			last_remote_offset = VALUES(last_remote_offset),
			last_remote_file_identity = VALUES(last_remote_file_identity),
			last_spool_size_bytes = VALUES(last_spool_size_bytes),
			last_error_at = VALUES(last_error_at),
			last_error_message = VALUES(last_error_message),
			updated_at = VALUES(updated_at)`,
		status.SourceID,
		status.AcquisitionState,
		status.RemoteAccessState,
		status.TransportMode,
		status.LastSuccessfulPullAt,
		status.LastRemoteOffset,
		status.LastRemoteFileIdentity,
		status.LastSpoolSizeBytes,
		status.LastErrorAt,
		status.LastErrorMessage,
		time.Now().UTC(),
	)
	return err
}

func (s *Store) UpdateSourceMetadata(ctx context.Context, sourceID int64, metadata model.SourceMetadataUpdate) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sources
		SET source_db_host = COALESCE(?, source_db_host),
			source_db_version = COALESCE(?, source_db_version),
			updated_at = ?
		WHERE id = ?`,
		metadata.DatabaseHost,
		metadata.DatabaseVersion,
		time.Now().UTC(),
		sourceID,
	)
	if err != nil {
		return err
	}
	if s.activeSource != nil && s.activeSource.ID == sourceID {
		if metadata.DatabaseHost != nil {
			s.activeSource.DatabaseHost = metadata.DatabaseHost
		}
		if metadata.DatabaseVersion != nil {
			s.activeSource.DatabaseVersion = metadata.DatabaseVersion
		}
		s.activeSource.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (s *Store) GetCollectorStatus(ctx context.Context) (*model.CollectorStatus, error) {
	source, err := s.GetSource(ctx)
	if err != nil {
		return nil, err
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT source_id, collector_state, source_access_state, last_successful_ingest_at, last_checkpoint_offset,
		       last_file_identity, last_error_at, last_error_message, updated_at
		FROM collector_status
		WHERE source_id = ?`,
		source.ID,
	)
	var status model.CollectorStatus
	var lastSuccessful sql.NullTime
	var lastCheckpoint sql.NullInt64
	var lastFileIdentity sql.NullString
	var lastErrorAt sql.NullTime
	var lastErrorMessage sql.NullString
	if err := row.Scan(
		&status.SourceID,
		&status.CollectorState,
		&status.SourceAccessState,
		&lastSuccessful,
		&lastCheckpoint,
		&lastFileIdentity,
		&lastErrorAt,
		&lastErrorMessage,
		&status.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			now := time.Now().UTC()
			return &model.CollectorStatus{
				SourceID:          source.ID,
				CollectorState:    model.CollectorStateIdle,
				SourceAccessState: model.SourceAccessUnknown,
				UpdatedAt:         now,
			}, nil
		}
		return nil, err
	}
	if lastSuccessful.Valid {
		status.LastSuccessfulIngestAt = &lastSuccessful.Time
	}
	if lastCheckpoint.Valid {
		value := lastCheckpoint.Int64
		status.LastCheckpointOffset = &value
	}
	if lastFileIdentity.Valid {
		value := lastFileIdentity.String
		status.LastFileIdentity = &value
	}
	if lastErrorAt.Valid {
		status.LastErrorAt = &lastErrorAt.Time
	}
	if lastErrorMessage.Valid {
		value := lastErrorMessage.String
		status.LastErrorMessage = &value
	}
	return &status, nil
}

func (s *Store) UpdateCollectorStatus(ctx context.Context, status model.CollectorStatus) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO collector_status (
			source_id, collector_state, source_access_state, last_successful_ingest_at, last_checkpoint_offset,
			last_file_identity, last_error_at, last_error_message, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			collector_state = VALUES(collector_state),
			source_access_state = VALUES(source_access_state),
			last_successful_ingest_at = VALUES(last_successful_ingest_at),
			last_checkpoint_offset = VALUES(last_checkpoint_offset),
			last_file_identity = VALUES(last_file_identity),
			last_error_at = VALUES(last_error_at),
			last_error_message = VALUES(last_error_message),
			updated_at = VALUES(updated_at)`,
		status.SourceID,
		status.CollectorState,
		status.SourceAccessState,
		status.LastSuccessfulIngestAt,
		status.LastCheckpointOffset,
		status.LastFileIdentity,
		status.LastErrorAt,
		status.LastErrorMessage,
		time.Now().UTC(),
	)
	return err
}

func (s *Store) GetCheckpoint(ctx context.Context, sourceID int64) (*model.CollectorCheckpoint, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT source_id, log_file_path, log_file_path_hash, file_identity, last_offset, updated_at
		FROM collector_checkpoints
		WHERE source_id = ?`,
		sourceID,
	)
	var checkpoint model.CollectorCheckpoint
	if err := row.Scan(
		&checkpoint.SourceID,
		&checkpoint.LogFilePath,
		&checkpoint.LogFileHash,
		&checkpoint.FileIdentity,
		&checkpoint.LastOffset,
		&checkpoint.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &checkpoint, nil
}

func (s *Store) UpsertCheckpoint(ctx context.Context, checkpoint model.CollectorCheckpoint) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO collector_checkpoints (source_id, log_file_path, log_file_path_hash, file_identity, last_offset, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			log_file_path = VALUES(log_file_path),
			log_file_path_hash = VALUES(log_file_path_hash),
			file_identity = VALUES(file_identity),
			last_offset = VALUES(last_offset),
			updated_at = VALUES(updated_at)`,
		checkpoint.SourceID,
		checkpoint.LogFilePath,
		pathHash(checkpoint.LogFilePath),
		checkpoint.FileIdentity,
		checkpoint.LastOffset,
		time.Now().UTC(),
	)
	return err
}

func (s *Store) GetAcquisitionCheckpoint(ctx context.Context, sourceID int64) (*model.AcquisitionCheckpoint, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT source_id, transport_mode, remote_host, remote_path, remote_file_identity, last_remote_offset,
		       local_spool_path, last_spool_size_bytes, initial_position, updated_at
		FROM acquisition_checkpoints
		WHERE source_id = ?`,
		sourceID,
	)
	var checkpoint model.AcquisitionCheckpoint
	var remoteHost sql.NullString
	var remotePath sql.NullString
	var remoteIdentity sql.NullString
	var spoolPath sql.NullString
	if err := row.Scan(
		&checkpoint.SourceID,
		&checkpoint.TransportMode,
		&remoteHost,
		&remotePath,
		&remoteIdentity,
		&checkpoint.LastRemoteOffset,
		&spoolPath,
		&checkpoint.LastSpoolSizeBytes,
		&checkpoint.InitialPosition,
		&checkpoint.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if remoteHost.Valid {
		value := remoteHost.String
		checkpoint.RemoteHost = &value
	}
	if remotePath.Valid {
		value := remotePath.String
		checkpoint.RemotePath = &value
	}
	if remoteIdentity.Valid {
		value := remoteIdentity.String
		checkpoint.RemoteFileIdentity = &value
	}
	if spoolPath.Valid {
		value := spoolPath.String
		checkpoint.LocalSpoolPath = &value
	}
	return &checkpoint, nil
}

func (s *Store) UpsertAcquisitionCheckpoint(ctx context.Context, checkpoint model.AcquisitionCheckpoint) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO acquisition_checkpoints (
			source_id, transport_mode, remote_host, remote_path, remote_file_identity, last_remote_offset,
			local_spool_path, last_spool_size_bytes, initial_position, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			transport_mode = VALUES(transport_mode),
			remote_host = VALUES(remote_host),
			remote_path = VALUES(remote_path),
			remote_file_identity = VALUES(remote_file_identity),
			last_remote_offset = VALUES(last_remote_offset),
			local_spool_path = VALUES(local_spool_path),
			last_spool_size_bytes = VALUES(last_spool_size_bytes),
			initial_position = VALUES(initial_position),
			updated_at = VALUES(updated_at)`,
		checkpoint.SourceID,
		checkpoint.TransportMode,
		checkpoint.RemoteHost,
		checkpoint.RemotePath,
		checkpoint.RemoteFileIdentity,
		checkpoint.LastRemoteOffset,
		checkpoint.LocalSpoolPath,
		checkpoint.LastSpoolSizeBytes,
		checkpoint.InitialPosition,
		time.Now().UTC(),
	)
	return err
}

func (s *Store) IngestRecord(ctx context.Context, input model.IngestRecordInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	record := input.Record
	record.CreatedAt = now

	if err := upsertFingerprintTx(ctx, tx, record.SourceID, input.Fingerprint, record.OccurredAt, now); err != nil {
		return err
	}
	fingerprintID, err := lookupFingerprintIDTx(ctx, tx, record.SourceID, input.Fingerprint.Hash)
	if err != nil {
		return err
	}
	record.FingerprintID = fingerprintID

	inserted, err := insertRecordTx(ctx, tx, record)
	if err != nil {
		return err
	}
	if inserted {
		if err := upsertFingerprintStatsTx(ctx, tx, record, now); err != nil {
			return err
		}
	}
	if err := upsertCheckpointTx(ctx, tx, record, now); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) CleanupExpiredRecords(ctx context.Context, sourceID int64, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM slow_query_records
		WHERE source_id = ? AND occurred_at < ?`,
		sourceID,
		olderThan,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) GetOverview(ctx context.Context) (model.Overview, error) {
	sourceID, err := s.activeSourceID()
	if err != nil {
		return model.Overview{}, err
	}

	overview := model.Overview{}
	var totalCount int64
	var lastSeen sql.NullTime
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE((SELECT COUNT(*) FROM slow_query_records WHERE source_id = ?), 0),
			COALESCE((SELECT COUNT(*) FROM fingerprints WHERE source_id = ?), 0),
			COALESCE(SUM(fs.total_query_time_sec), 0),
			COALESCE(SUM(fs.total_count), 0),
			COALESCE(MAX(fs.max_query_time_sec), 0),
			MAX(fs.last_seen_at)
		FROM fingerprint_stats fs
		JOIN fingerprints f ON f.id = fs.fingerprint_id
		WHERE f.source_id = ?`,
		sourceID,
		sourceID,
		sourceID,
	).Scan(
		&overview.TotalRecords,
		&overview.TotalFingerprints,
		&overview.TotalQueryTimeSec,
		&totalCount,
		&overview.MaxQueryTimeSec,
		&lastSeen,
	); err != nil {
		return overview, err
	}
	if totalCount > 0 {
		overview.AvgQueryTimeSec = overview.TotalQueryTimeSec / float64(totalCount)
	}
	if lastSeen.Valid {
		overview.LastIngestedAt = &lastSeen.Time
	}
	items, err := s.ListFingerprints(ctx, model.ListFingerprintsParams{
		Page:      1,
		PageSize:  5,
		SortBy:    "totalQueryTimeSec",
		SortOrder: "desc",
	})
	if err != nil {
		return overview, err
	}
	overview.TopFingerprints = items.Items
	return overview, nil
}

func (s *Store) ListFingerprints(ctx context.Context, params model.ListFingerprintsParams) (model.PaginatedFingerprints, error) {
	sourceID, err := s.activeSourceID()
	if err != nil {
		return model.PaginatedFingerprints{}, err
	}

	page := normalizePage(params.Page)
	pageSize := normalizePageSize(params.PageSize)
	sortBy := normalizeFingerprintSort(params.SortBy)
	sortOrder := normalizeSortOrder(params.SortOrder)

	clauses := []string{"f.source_id = ?"}
	args := []any{sourceID}
	if params.SQLType != "" {
		clauses = append(clauses, "f.sql_type = ?")
		args = append(args, strings.ToUpper(params.SQLType))
	}
	if params.Keyword != "" {
		clauses = append(clauses, "f.normalized_sql LIKE ?")
		args = append(args, "%"+params.Keyword+"%")
	}
	if params.DBName != "" {
		clauses = append(clauses, `EXISTS (
			SELECT 1 FROM slow_query_records r
			WHERE r.source_id = f.source_id AND r.fingerprint_id = f.id AND r.db_name = ?
		)`)
		args = append(args, params.DBName)
	}

	where := strings.Join(clauses, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM fingerprints f
		JOIN fingerprint_stats fs ON fs.fingerprint_id = f.id
		WHERE `+where, args...).Scan(&total); err != nil {
		return model.PaginatedFingerprints{}, err
	}

	query := `
		SELECT
			f.id, f.source_id, f.fingerprint_hash, f.normalized_sql, f.sql_type, f.main_table_name,
			f.first_seen_at, f.last_seen_at, f.created_at, f.updated_at,
			fs.total_count, fs.total_query_time_sec, fs.avg_query_time_sec, fs.max_query_time_sec,
			fs.total_rows_examined, fs.avg_rows_examined, fs.max_rows_examined,
			fs.total_rows_sent, fs.avg_rows_sent, fs.max_rows_sent,
			fs.first_seen_at, fs.last_seen_at, fs.updated_at
		FROM fingerprints f
		JOIN fingerprint_stats fs ON fs.fingerprint_id = f.id
		WHERE ` + where + `
		ORDER BY ` + sortBy + ` ` + sortOrder + `
		LIMIT ? OFFSET ?`

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PaginatedFingerprints{}, err
	}
	defer rows.Close()

	var items []model.FingerprintRecordView
	for rows.Next() {
		view, err := scanFingerprintRecordView(rows)
		if err != nil {
			return model.PaginatedFingerprints{}, err
		}
		items = append(items, view)
	}
	return model.PaginatedFingerprints{Items: items, Total: total, Page: page, PageSize: pageSize}, rows.Err()
}

func (s *Store) GetFingerprint(ctx context.Context, id int64) (*model.FingerprintRecordView, error) {
	sourceID, err := s.activeSourceID()
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			f.id, f.source_id, f.fingerprint_hash, f.normalized_sql, f.sql_type, f.main_table_name,
			f.first_seen_at, f.last_seen_at, f.created_at, f.updated_at,
			fs.total_count, fs.total_query_time_sec, fs.avg_query_time_sec, fs.max_query_time_sec,
			fs.total_rows_examined, fs.avg_rows_examined, fs.max_rows_examined,
			fs.total_rows_sent, fs.avg_rows_sent, fs.max_rows_sent,
			fs.first_seen_at, fs.last_seen_at, fs.updated_at
		FROM fingerprints f
		JOIN fingerprint_stats fs ON fs.fingerprint_id = f.id
		WHERE f.source_id = ? AND f.id = ?`, sourceID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	view, err := scanFingerprintRecordView(rows)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (s *Store) ListFingerprintRecords(ctx context.Context, fingerprintID int64, params model.ListFingerprintRecordsParams) (model.PaginatedRecords, error) {
	sourceID, err := s.activeSourceID()
	if err != nil {
		return model.PaginatedRecords{}, err
	}

	page := normalizePage(params.Page)
	pageSize := normalizePageSize(params.PageSize)
	sortBy := normalizeRecordSort(params.SortBy)
	sortOrder := normalizeSortOrder(params.SortOrder)

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM slow_query_records WHERE source_id = ? AND fingerprint_id = ?`, sourceID, fingerprintID).Scan(&total); err != nil {
		return model.PaginatedRecords{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, source_id, source_instance_name, source_log_file_path, source_file_identity, source_offset_start, source_offset_end,
			occurred_at, db_name, user_name, client_host, raw_block, raw_sql, normalized_sql, fingerprint_id,
			fingerprint_hash, query_time_sec, lock_time_sec, rows_sent, rows_examined, created_at
		FROM slow_query_records
		WHERE source_id = ? AND fingerprint_id = ?
		ORDER BY `+sortBy+` `+sortOrder+`
		LIMIT ? OFFSET ?`,
		sourceID, fingerprintID, pageSize, (page-1)*pageSize,
	)
	if err != nil {
		return model.PaginatedRecords{}, err
	}
	defer rows.Close()

	var items []model.SlowQueryRecord
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return model.PaginatedRecords{}, err
		}
		items = append(items, record)
	}
	return model.PaginatedRecords{Items: items, Total: total, Page: page, PageSize: pageSize}, rows.Err()
}

func (s *Store) ensureSource(ctx context.Context, cfg config.SourceConfig) (*model.Source, error) {
	now := time.Now().UTC()
	key := model.SourceKey(cfg.InstanceName, cfg.IdentityPath())
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sources (
			source_key, source_instance_name, source_slow_log_path, source_description,
			source_db_dsn_configured, source_log_mode, source_remote_host, source_remote_port,
			source_remote_user, source_remote_slow_log_path, source_local_spool_path,
			source_initial_position, source_local_spool_max_bytes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			source_instance_name = VALUES(source_instance_name),
			source_slow_log_path = VALUES(source_slow_log_path),
			source_description = VALUES(source_description),
			source_db_dsn_configured = VALUES(source_db_dsn_configured),
			source_log_mode = VALUES(source_log_mode),
			source_remote_host = VALUES(source_remote_host),
			source_remote_port = VALUES(source_remote_port),
			source_remote_user = VALUES(source_remote_user),
			source_remote_slow_log_path = VALUES(source_remote_slow_log_path),
			source_local_spool_path = VALUES(source_local_spool_path),
			source_initial_position = VALUES(source_initial_position),
			source_local_spool_max_bytes = VALUES(source_local_spool_max_bytes),
			updated_at = VALUES(updated_at)`,
		key,
		cfg.InstanceName,
		cfg.IdentityPath(),
		nullableString(cfg.Description),
		strings.TrimSpace(cfg.DatabaseDSN) != "",
		cfg.LogMode,
		nullableString(cfg.RemoteHost),
		nullableInt(cfg.RemotePort),
		nullableString(cfg.RemoteUser),
		nullableString(cfg.RemoteSlowLogPath),
		nullableString(cfg.EffectiveParsePath()),
		cfg.InitialPosition,
		nullableInt64(cfg.LocalSpoolMaxBytes),
		now,
		now,
	)
	if err != nil {
		return nil, err
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT id, source_key, source_instance_name, source_slow_log_path, source_description,
		       source_db_dsn_configured, source_db_host, source_db_version, source_log_mode,
		       source_remote_host, source_remote_port, source_remote_user, source_remote_slow_log_path,
		       source_local_spool_path, source_initial_position, source_local_spool_max_bytes,
		       created_at, updated_at
		FROM sources
		WHERE source_key = ?`,
		key,
	)
	return scanSource(row)
}

func (s *Store) activeSourceID() (int64, error) {
	if s.activeSource == nil {
		return 0, fmt.Errorf("active source is not configured")
	}
	return s.activeSource.ID, nil
}

func upsertFingerprintTx(ctx context.Context, tx *sql.Tx, sourceID int64, fingerprint model.ProcessedFingerprint, occurredAt, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO fingerprints (
			source_id, fingerprint_hash, normalized_sql, sql_type, main_table_name, first_seen_at, last_seen_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			last_seen_at = GREATEST(last_seen_at, VALUES(last_seen_at)),
			first_seen_at = LEAST(first_seen_at, VALUES(first_seen_at)),
			sql_type = VALUES(sql_type),
			main_table_name = COALESCE(VALUES(main_table_name), main_table_name),
			updated_at = VALUES(updated_at)
	`,
		sourceID,
		fingerprint.Hash,
		fingerprint.NormalizedSQL,
		fingerprint.SQLType,
		fingerprint.MainTableName,
		occurredAt,
		occurredAt,
		now,
		now,
	)
	return err
}

func lookupFingerprintIDTx(ctx context.Context, tx *sql.Tx, sourceID int64, hash string) (int64, error) {
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM fingerprints WHERE source_id = ? AND fingerprint_hash = ?`, sourceID, hash).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func insertRecordTx(ctx context.Context, tx *sql.Tx, record model.SlowQueryRecord) (bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO slow_query_records (
			source_id, source_instance_name, source_log_file_path, source_log_file_path_hash, source_file_identity, source_offset_start, source_offset_end,
			occurred_at, db_name, user_name, client_host, raw_block, raw_sql, normalized_sql,
			fingerprint_id, fingerprint_hash, query_time_sec, lock_time_sec, rows_sent, rows_examined, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.SourceID,
		record.SourceInstance,
		record.SourceLogFilePath,
		pathHash(record.SourceLogFilePath),
		record.SourceFileID,
		record.SourceOffsetStart,
		record.SourceOffsetEnd,
		record.OccurredAt,
		record.DBName,
		record.UserName,
		record.ClientHost,
		record.RawBlock,
		record.RawSQL,
		record.NormalizedSQL,
		record.FingerprintID,
		record.FingerprintHash,
		record.QueryTimeSec,
		record.LockTimeSec,
		record.RowsSent,
		record.RowsExamined,
		record.CreatedAt,
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

func upsertFingerprintStatsTx(ctx context.Context, tx *sql.Tx, record model.SlowQueryRecord, now time.Time) error {
	rowsExamined := valueOrZero(record.RowsExamined)
	rowsSent := valueOrZero(record.RowsSent)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO fingerprint_stats (
			fingerprint_id, total_count, total_query_time_sec, avg_query_time_sec, max_query_time_sec,
			total_rows_examined, avg_rows_examined, max_rows_examined,
			total_rows_sent, avg_rows_sent, max_rows_sent,
			first_seen_at, last_seen_at, updated_at
		) VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			avg_query_time_sec = (total_query_time_sec + VALUES(total_query_time_sec)) / (total_count + 1),
			avg_rows_examined = (total_rows_examined + VALUES(total_rows_examined)) / (total_count + 1),
			avg_rows_sent = (total_rows_sent + VALUES(total_rows_sent)) / (total_count + 1),
			total_count = total_count + 1,
			total_query_time_sec = total_query_time_sec + VALUES(total_query_time_sec),
			max_query_time_sec = GREATEST(max_query_time_sec, VALUES(max_query_time_sec)),
			total_rows_examined = total_rows_examined + VALUES(total_rows_examined),
			max_rows_examined = GREATEST(max_rows_examined, VALUES(max_rows_examined)),
			total_rows_sent = total_rows_sent + VALUES(total_rows_sent),
			max_rows_sent = GREATEST(max_rows_sent, VALUES(max_rows_sent)),
			first_seen_at = LEAST(first_seen_at, VALUES(first_seen_at)),
			last_seen_at = GREATEST(last_seen_at, VALUES(last_seen_at)),
			updated_at = VALUES(updated_at)
	`,
		record.FingerprintID,
		record.QueryTimeSec,
		record.QueryTimeSec,
		record.QueryTimeSec,
		rowsExamined,
		float64(rowsExamined),
		rowsExamined,
		rowsSent,
		float64(rowsSent),
		rowsSent,
		record.OccurredAt,
		record.OccurredAt,
		now,
	)
	return err
}

func upsertCheckpointTx(ctx context.Context, tx *sql.Tx, record model.SlowQueryRecord, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO collector_checkpoints (source_id, log_file_path, log_file_path_hash, file_identity, last_offset, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			log_file_path = VALUES(log_file_path),
			log_file_path_hash = VALUES(log_file_path_hash),
			file_identity = VALUES(file_identity),
			last_offset = VALUES(last_offset),
			updated_at = VALUES(updated_at)
	`,
		record.SourceID,
		record.SourceLogFilePath,
		pathHash(record.SourceLogFilePath),
		record.SourceFileID,
		record.SourceOffsetEnd,
		now,
	)
	return err
}

func scanSource(scanner interface {
	Scan(dest ...any) error
}) (*model.Source, error) {
	var source model.Source
	var description sql.NullString
	var databaseHost sql.NullString
	var databaseVersion sql.NullString
	var remoteHost sql.NullString
	var remotePort sql.NullInt64
	var remoteUser sql.NullString
	var remoteSlowLogPath sql.NullString
	var localSpoolPath sql.NullString
	var localSpoolMaxBytes sql.NullInt64
	if err := scanner.Scan(
		&source.ID,
		&source.Key,
		&source.InstanceName,
		&source.SlowLogPath,
		&description,
		&source.DatabaseDSNConfigured,
		&databaseHost,
		&databaseVersion,
		&source.LogMode,
		&remoteHost,
		&remotePort,
		&remoteUser,
		&remoteSlowLogPath,
		&localSpoolPath,
		&source.InitialPosition,
		&localSpoolMaxBytes,
		&source.CreatedAt,
		&source.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if description.Valid {
		value := description.String
		source.Description = &value
	}
	if databaseHost.Valid {
		value := databaseHost.String
		source.DatabaseHost = &value
	}
	if databaseVersion.Valid {
		value := databaseVersion.String
		source.DatabaseVersion = &value
	}
	if remoteHost.Valid {
		value := remoteHost.String
		source.RemoteHost = &value
	}
	if remotePort.Valid {
		value := int(remotePort.Int64)
		source.RemotePort = &value
	}
	if remoteUser.Valid {
		value := remoteUser.String
		source.RemoteUser = &value
	}
	if remoteSlowLogPath.Valid {
		value := remoteSlowLogPath.String
		source.RemoteSlowLogPath = &value
	}
	if localSpoolPath.Valid {
		value := localSpoolPath.String
		source.LocalSpoolPath = &value
	}
	if localSpoolMaxBytes.Valid {
		value := localSpoolMaxBytes.Int64
		source.LocalSpoolMaxBytes = &value
	}
	return &source, nil
}

func scanFingerprintRecordView(scanner interface {
	Scan(dest ...any) error
}) (model.FingerprintRecordView, error) {
	var view model.FingerprintRecordView
	var mainTable sql.NullString
	var fingerprintFirstSeen time.Time
	var fingerprintLastSeen time.Time
	var fingerprintCreatedAt time.Time
	var fingerprintUpdatedAt time.Time
	var statsFirstSeen time.Time
	var statsLastSeen time.Time
	var statsUpdatedAt time.Time
	err := scanner.Scan(
		&view.ID,
		&view.SourceID,
		&view.Hash,
		&view.NormalizedSQL,
		&view.SQLType,
		&mainTable,
		&fingerprintFirstSeen,
		&fingerprintLastSeen,
		&fingerprintCreatedAt,
		&fingerprintUpdatedAt,
		&view.TotalCount,
		&view.TotalQueryTimeSec,
		&view.AvgQueryTimeSec,
		&view.MaxQueryTimeSec,
		&view.TotalRowsExamined,
		&view.AvgRowsExamined,
		&view.MaxRowsExamined,
		&view.TotalRowsSent,
		&view.AvgRowsSent,
		&view.MaxRowsSent,
		&statsFirstSeen,
		&statsLastSeen,
		&statsUpdatedAt,
	)
	if err != nil {
		return view, err
	}
	view.Fingerprint.FirstSeenAt = fingerprintFirstSeen
	view.Fingerprint.LastSeenAt = fingerprintLastSeen
	view.Fingerprint.CreatedAt = fingerprintCreatedAt
	view.Fingerprint.UpdatedAt = fingerprintUpdatedAt
	view.FingerprintStats.FirstSeenAt = statsFirstSeen
	view.FingerprintStats.LastSeenAt = statsLastSeen
	view.FingerprintStats.UpdatedAt = statsUpdatedAt
	if mainTable.Valid {
		view.MainTableName = &mainTable.String
	}
	view.FingerprintID = view.ID
	return view, nil
}

func scanRecord(scanner interface {
	Scan(dest ...any) error
}) (model.SlowQueryRecord, error) {
	var record model.SlowQueryRecord
	err := scanner.Scan(
		&record.ID,
		&record.SourceID,
		&record.SourceInstance,
		&record.SourceLogFilePath,
		&record.SourceFileID,
		&record.SourceOffsetStart,
		&record.SourceOffsetEnd,
		&record.OccurredAt,
		&record.DBName,
		&record.UserName,
		&record.ClientHost,
		&record.RawBlock,
		&record.RawSQL,
		&record.NormalizedSQL,
		&record.FingerprintID,
		&record.FingerprintHash,
		&record.QueryTimeSec,
		&record.LockTimeSec,
		&record.RowsSent,
		&record.RowsExamined,
		&record.CreatedAt,
	)
	return record, err
}

func normalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizePageSize(size int) int {
	switch {
	case size <= 0:
		return 20
	case size > 100:
		return 100
	default:
		return size
	}
}

func normalizeSortOrder(order string) string {
	if strings.EqualFold(order, "asc") {
		return "ASC"
	}
	return "DESC"
}

func normalizeFingerprintSort(sortBy string) string {
	switch sortBy {
	case "avgQueryTimeSec":
		return "fs.avg_query_time_sec"
	case "maxQueryTimeSec":
		return "fs.max_query_time_sec"
	case "totalCount":
		return "fs.total_count"
	case "lastSeenAt":
		return "fs.last_seen_at"
	case "avgRowsExamined":
		return "fs.avg_rows_examined"
	default:
		return "fs.total_query_time_sec"
	}
}

func normalizeRecordSort(sortBy string) string {
	switch sortBy {
	case "queryTimeSec":
		return "query_time_sec"
	default:
		return "occurred_at"
	}
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func nullableString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullableInt(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func nullableInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func (s *Store) ensureColumn(ctx context.Context, table, definition string) error {
	columnName := columnNameFromDefinition(definition)
	exists, err := s.columnExists(ctx, table, columnName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s", table, definition)); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, columnName, err)
	}
	return nil
}

func (s *Store) columnExists(ctx context.Context, table, column string) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
		s.schema,
		table,
		column,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) ensureIndex(ctx context.Context, table, indexName string, unique bool, expectedColumns []string, definition string) error {
	matches, exists, err := s.indexMatches(ctx, table, indexName, unique, expectedColumns)
	if err != nil {
		return err
	}
	if matches {
		return nil
	}
	if exists {
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`", table, indexName)); err != nil {
			return fmt.Errorf("drop index %s.%s: %w", table, indexName, err)
		}
	}

	kind := "KEY"
	if unique {
		kind = "UNIQUE KEY"
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE `%s` ADD %s `%s` %s", table, kind, indexName, definition)); err != nil {
		return fmt.Errorf("add index %s.%s: %w", table, indexName, err)
	}
	return nil
}

func (s *Store) indexMatches(ctx context.Context, table, indexName string, unique bool, expectedColumns []string) (bool, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT NON_UNIQUE, COLUMN_NAME
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND INDEX_NAME = ?
		ORDER BY SEQ_IN_INDEX`,
		s.schema,
		table,
		indexName,
	)
	if err != nil {
		return false, false, err
	}
	defer rows.Close()

	var columns []string
	var nonUnique int
	exists := false
	for rows.Next() {
		exists = true
		var column string
		if err := rows.Scan(&nonUnique, &column); err != nil {
			return false, false, err
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return false, false, err
	}
	if !exists {
		return false, false, nil
	}
	if (nonUnique == 0) != unique {
		return false, true, nil
	}
	if len(columns) != len(expectedColumns) {
		return false, true, nil
	}
	for i := range columns {
		if !strings.EqualFold(columns[i], expectedColumns[i]) {
			return false, true, nil
		}
	}
	return true, true, nil
}

func columnNameFromDefinition(definition string) string {
	definition = strings.TrimSpace(definition)
	if strings.HasPrefix(definition, "`") {
		if end := strings.Index(definition[1:], "`"); end >= 0 {
			return definition[1 : end+1]
		}
	}
	parts := strings.Fields(definition)
	if len(parts) == 0 {
		return ""
	}
	return strings.Trim(parts[0], "`")
}
