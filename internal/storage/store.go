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
	db     *sql.DB
	schema string
}

func Open(ctx context.Context, cfg config.DatabaseConfig) (*Store, error) {
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
		`CREATE TABLE IF NOT EXISTS collector_checkpoints (
			id BIGINT NOT NULL AUTO_INCREMENT,
			instance_name VARCHAR(128) NOT NULL,
			log_file_path VARCHAR(1024) NOT NULL,
			log_file_path_hash CHAR(40) NOT NULL,
			file_identity VARCHAR(255) NOT NULL,
			last_offset BIGINT NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_checkpoint_source (instance_name, log_file_path_hash)
		)`,
		`CREATE TABLE IF NOT EXISTS fingerprints (
			id BIGINT NOT NULL AUTO_INCREMENT,
			fingerprint_hash CHAR(40) NOT NULL,
			normalized_sql LONGTEXT NOT NULL,
			sql_type VARCHAR(32) NOT NULL,
			main_table_name VARCHAR(255) NULL,
			first_seen_at DATETIME(6) NOT NULL,
			last_seen_at DATETIME(6) NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_fingerprint_hash (fingerprint_hash),
			KEY idx_fingerprints_type_table (sql_type, main_table_name)
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
			UNIQUE KEY uk_record_source (source_instance_name, source_log_file_path_hash, source_file_identity, source_offset_start),
			KEY idx_records_fingerprint_time (fingerprint_id, occurred_at DESC),
			KEY idx_records_occurred_at (occurred_at DESC),
			KEY idx_records_query_time (query_time_sec DESC),
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

func (s *Store) GetCheckpoint(ctx context.Context, instanceName, logFilePath string) (*model.CollectorCheckpoint, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT instance_name, log_file_path, log_file_path_hash, file_identity, last_offset, updated_at
		FROM collector_checkpoints
		WHERE instance_name = ? AND log_file_path_hash = ?`,
		instanceName, pathHash(logFilePath),
	)
	var checkpoint model.CollectorCheckpoint
	if err := row.Scan(
		&checkpoint.InstanceName,
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

func (s *Store) IngestRecord(ctx context.Context, input model.IngestRecordInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	record := input.Record
	record.CreatedAt = now

	if err := upsertFingerprintTx(ctx, tx, input.Fingerprint, record.OccurredAt, now); err != nil {
		return err
	}
	fingerprintID, err := lookupFingerprintIDTx(ctx, tx, input.Fingerprint.Hash)
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

func upsertFingerprintTx(ctx context.Context, tx *sql.Tx, fingerprint model.ProcessedFingerprint, occurredAt, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO fingerprints (
			fingerprint_hash, normalized_sql, sql_type, main_table_name, first_seen_at, last_seen_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			last_seen_at = GREATEST(last_seen_at, VALUES(last_seen_at)),
			first_seen_at = LEAST(first_seen_at, VALUES(first_seen_at)),
			sql_type = VALUES(sql_type),
			main_table_name = COALESCE(VALUES(main_table_name), main_table_name),
			updated_at = VALUES(updated_at)
	`,
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

func lookupFingerprintIDTx(ctx context.Context, tx *sql.Tx, hash string) (int64, error) {
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM fingerprints WHERE fingerprint_hash = ?`, hash).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func insertRecordTx(ctx context.Context, tx *sql.Tx, record model.SlowQueryRecord) (bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO slow_query_records (
			source_instance_name, source_log_file_path, source_log_file_path_hash, source_file_identity, source_offset_start, source_offset_end,
			occurred_at, db_name, user_name, client_host, raw_block, raw_sql, normalized_sql,
			fingerprint_id, fingerprint_hash, query_time_sec, lock_time_sec, rows_sent, rows_examined, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
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
		INSERT INTO collector_checkpoints (instance_name, log_file_path, log_file_path_hash, file_identity, last_offset, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			log_file_path = VALUES(log_file_path),
			file_identity = VALUES(file_identity),
			last_offset = VALUES(last_offset),
			updated_at = VALUES(updated_at)
	`,
		record.SourceInstance,
		record.SourceLogFilePath,
		pathHash(record.SourceLogFilePath),
		record.SourceFileID,
		record.SourceOffsetEnd,
		now,
	)
	return err
}

func (s *Store) GetOverview(ctx context.Context) (model.Overview, error) {
	overview := model.Overview{}
	var totalCount int64
	var lastSeen sql.NullTime
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE((SELECT COUNT(*) FROM slow_query_records), 0),
			COALESCE((SELECT COUNT(*) FROM fingerprints), 0),
			COALESCE(SUM(total_query_time_sec), 0),
			COALESCE(SUM(total_count), 0),
			COALESCE(MAX(max_query_time_sec), 0),
			MAX(last_seen_at)
		FROM fingerprint_stats`).Scan(
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
	page := normalizePage(params.Page)
	pageSize := normalizePageSize(params.PageSize)
	sortBy := normalizeFingerprintSort(params.SortBy)
	sortOrder := normalizeSortOrder(params.SortOrder)

	clauses := []string{"1=1"}
	args := []any{}
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
			WHERE r.fingerprint_id = f.id AND r.db_name = ?
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
			f.id, f.fingerprint_hash, f.normalized_sql, f.sql_type, f.main_table_name,
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
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			f.id, f.fingerprint_hash, f.normalized_sql, f.sql_type, f.main_table_name,
			f.first_seen_at, f.last_seen_at, f.created_at, f.updated_at,
			fs.total_count, fs.total_query_time_sec, fs.avg_query_time_sec, fs.max_query_time_sec,
			fs.total_rows_examined, fs.avg_rows_examined, fs.max_rows_examined,
			fs.total_rows_sent, fs.avg_rows_sent, fs.max_rows_sent,
			fs.first_seen_at, fs.last_seen_at, fs.updated_at
		FROM fingerprints f
		JOIN fingerprint_stats fs ON fs.fingerprint_id = f.id
		WHERE f.id = ?`, id)
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
	page := normalizePage(params.Page)
	pageSize := normalizePageSize(params.PageSize)
	sortBy := normalizeRecordSort(params.SortBy)
	sortOrder := normalizeSortOrder(params.SortOrder)

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM slow_query_records WHERE fingerprint_id = ?`, fingerprintID).Scan(&total); err != nil {
		return model.PaginatedRecords{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, source_instance_name, source_log_file_path, source_file_identity, source_offset_start, source_offset_end,
			occurred_at, db_name, user_name, client_host, raw_block, raw_sql, normalized_sql, fingerprint_id,
			fingerprint_hash, query_time_sec, lock_time_sec, rows_sent, rows_examined, created_at
		FROM slow_query_records
		WHERE fingerprint_id = ?
		ORDER BY `+sortBy+` `+sortOrder+`
		LIMIT ? OFFSET ?`,
		fingerprintID, pageSize, (page-1)*pageSize,
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
