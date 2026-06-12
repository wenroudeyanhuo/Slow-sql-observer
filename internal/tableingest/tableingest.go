package tableingest

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"

	"slow-sql-observer/internal/model"
)

// TableIngester reads slow-query rows from mysql.slow_log.
type TableIngester struct{}

func New() *TableIngester {
	return &TableIngester{}
}

// slowLogRow represents a single row from mysql.slow_log.
type slowLogRow struct {
	StartTime    time.Time
	ThreadID     int64
	ServerID     int64
	UserHost     string
	QueryTime    string
	LockTime     string
	RowsSent     int64
	RowsExamined int64
	DB           sql.NullString
	SQLText      string
	IdentityHash string
}

// FetchAndMap reads new rows from mysql.slow_log since the given checkpoint,
// and maps them into IngestRecordInput slices ready for the downstream pipeline.
func (t *TableIngester) FetchAndMap(ctx context.Context, dsn string, checkpoint *model.TableIngestionCheckpoint, sourceID int64, instanceName string) ([]model.IngestRecordInput, model.TableIngestionCheckpoint, error) {
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, model.TableIngestionCheckpoint{}, fmt.Errorf("parse source DSN: %w", err)
	}
	parsed.ParseTime = true

	db, err := sql.Open("mysql", parsed.FormatDSN())
	if err != nil {
		return nil, model.TableIngestionCheckpoint{}, fmt.Errorf("open source DB: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return nil, model.TableIngestionCheckpoint{}, fmt.Errorf("ping source DB: %w", err)
	}

	// Validate mysql.slow_log access
	if err := validateSlowLogAccess(ctx, db); err != nil {
		return nil, model.TableIngestionCheckpoint{}, err
	}

	rows, err := fetchRows(ctx, db, checkpoint)
	if err != nil {
		return nil, model.TableIngestionCheckpoint{}, err
	}
	defer rows.Close()

	var records []model.IngestRecordInput
	var lastStartTime time.Time
	var lastThreadID int64
	var lastServerID int64
	var lastIdentityHash string
	var rowCount int64

	for rows.Next() {
		var row slowLogRow
		if err := rows.Scan(
			&row.StartTime,
			&row.ThreadID,
			&row.ServerID,
			&row.UserHost,
			&row.QueryTime,
			&row.LockTime,
			&row.RowsSent,
			&row.RowsExamined,
			&row.DB,
			&row.SQLText,
			&row.IdentityHash,
		); err != nil {
			return nil, model.TableIngestionCheckpoint{}, fmt.Errorf("scan slow_log row: %w", err)
		}

		rawSQL := strings.TrimSpace(row.SQLText)
		if rawSQL == "" {
			continue
		}

		userName, clientHost := parseUserHost(row.UserHost)
		queryTime := parseTimeToSeconds(row.QueryTime)
		lockTime := parseTimeToSeconds(row.LockTime)

		var dbName *string
		if row.DB.Valid && strings.TrimSpace(row.DB.String) != "" {
			v := strings.TrimSpace(row.DB.String)
			dbName = &v
		}

		rawBlock := buildRawBlock(row)

		record := model.SlowQueryRecord{
			SourceID:          sourceID,
			SourceInstance:    instanceName,
			SourceLogFilePath: "mysql.slow_log",
			SourceFileID:      "mysql.slow_log:" + row.IdentityHash,
			SourceOffsetStart: row.StartTime.UnixMicro(),
			SourceOffsetEnd:   row.StartTime.UnixMicro(),
			OccurredAt:        row.StartTime,
			DBName:            dbName,
			UserName:          userName,
			ClientHost:        clientHost,
			RawBlock:          rawBlock,
			RawSQL:            rawSQL,
			QueryTimeSec:      queryTime,
			LockTimeSec:       &lockTime,
			RowsSent:          &row.RowsSent,
			RowsExamined:      &row.RowsExamined,
		}

		records = append(records, model.IngestRecordInput{
			Record: record,
		})

		lastStartTime = row.StartTime
		lastThreadID = row.ThreadID
		lastServerID = row.ServerID
		lastIdentityHash = row.IdentityHash
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return nil, model.TableIngestionCheckpoint{}, fmt.Errorf("iterate slow_log rows: %w", err)
	}

	newCheckpoint := model.TableIngestionCheckpoint{
		SourceID:            sourceID,
		LastStartTime:       lastStartTime,
		LastThreadID:        lastThreadID,
		LastServerID:        lastServerID,
		LastRowIdentityHash: lastIdentityHash,
		RowsIngested:        rowCount,
		UpdatedAt:           time.Now().UTC(),
	}
	if checkpoint != nil && !checkpointAdvanced(newCheckpoint, checkpoint) {
		newCheckpoint.LastStartTime = checkpoint.LastStartTime
		newCheckpoint.LastThreadID = checkpoint.LastThreadID
		newCheckpoint.LastServerID = checkpoint.LastServerID
		newCheckpoint.LastRowIdentityHash = checkpoint.LastRowIdentityHash
		newCheckpoint.RowsIngested = 0
	}

	return records, newCheckpoint, nil
}

func validateSlowLogAccess(ctx context.Context, db *sql.DB) error {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mysql.slow_log LIMIT 1`).Scan(&count)
	if err != nil {
		return fmt.Errorf("cannot access mysql.slow_log (check SELECT privilege): %w", err)
	}
	return nil
}

func fetchRows(ctx context.Context, db *sql.DB, checkpoint *model.TableIngestionCheckpoint) (*sql.Rows, error) {
	if checkpoint != nil && !checkpoint.LastStartTime.IsZero() {
		return db.QueryContext(ctx, `
			SELECT
				start_time,
				COALESCE(thread_id, 0) AS thread_id_value,
				COALESCE(server_id, 0) AS server_id_value,
				user_host,
				query_time,
				lock_time,
				rows_sent,
				rows_examined,
				db,
				sql_text,
				SHA1(CONCAT_WS('|',
					DATE_FORMAT(start_time, '%Y-%m-%d %H:%i:%s.%f'),
					COALESCE(user_host, ''),
					COALESCE(query_time, ''),
					COALESCE(lock_time, ''),
					COALESCE(rows_sent, 0),
					COALESCE(rows_examined, 0),
					COALESCE(db, ''),
					COALESCE(sql_text, ''),
					COALESCE(thread_id, 0),
					COALESCE(server_id, 0)
				)) AS row_identity_hash
			FROM mysql.slow_log
			WHERE
				start_time > ?
				OR (start_time = ? AND COALESCE(thread_id, 0) > ?)
				OR (start_time = ? AND COALESCE(thread_id, 0) = ? AND COALESCE(server_id, 0) > ?)
				OR (
					start_time = ?
					AND COALESCE(thread_id, 0) = ?
					AND COALESCE(server_id, 0) = ?
					AND SHA1(CONCAT_WS('|',
						DATE_FORMAT(start_time, '%Y-%m-%d %H:%i:%s.%f'),
						COALESCE(user_host, ''),
						COALESCE(query_time, ''),
						COALESCE(lock_time, ''),
						COALESCE(rows_sent, 0),
						COALESCE(rows_examined, 0),
						COALESCE(db, ''),
						COALESCE(sql_text, ''),
						COALESCE(thread_id, 0),
						COALESCE(server_id, 0)
					)) > ?
				)
			ORDER BY start_time ASC, thread_id_value ASC, server_id_value ASC, row_identity_hash ASC
			LIMIT 10000`,
			checkpoint.LastStartTime,
			checkpoint.LastStartTime,
			checkpoint.LastThreadID,
			checkpoint.LastStartTime,
			checkpoint.LastThreadID,
			checkpoint.LastServerID,
			checkpoint.LastStartTime,
			checkpoint.LastThreadID,
			checkpoint.LastServerID,
			checkpoint.LastRowIdentityHash,
		)
	}
	return db.QueryContext(ctx, `
		SELECT
			start_time,
			COALESCE(thread_id, 0) AS thread_id_value,
			COALESCE(server_id, 0) AS server_id_value,
			user_host,
			query_time,
			lock_time,
			rows_sent,
			rows_examined,
			db,
			sql_text,
			SHA1(CONCAT_WS('|',
				DATE_FORMAT(start_time, '%Y-%m-%d %H:%i:%s.%f'),
				COALESCE(user_host, ''),
				COALESCE(query_time, ''),
				COALESCE(lock_time, ''),
				COALESCE(rows_sent, 0),
				COALESCE(rows_examined, 0),
				COALESCE(db, ''),
				COALESCE(sql_text, ''),
				COALESCE(thread_id, 0),
				COALESCE(server_id, 0)
			)) AS row_identity_hash
		FROM mysql.slow_log
		ORDER BY start_time ASC, thread_id_value ASC, server_id_value ASC, row_identity_hash ASC
		LIMIT 10000`)
}

func parseUserHost(raw string) (*string, *string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	// Typical format: "user[user] @ hostname [ip]" or "user[user] @ [ip]"
	parts := strings.SplitN(raw, "@", 2)
	var userName, clientHost *string
	if len(parts) >= 1 {
		u := strings.TrimSpace(parts[0])
		u = strings.SplitN(u, "[", 2)[0]
		u = strings.TrimSpace(u)
		if u != "" {
			userName = &u
		}
	}
	if len(parts) >= 2 {
		h := strings.TrimSpace(parts[1])
		h = strings.Trim(h, "[] ")
		h = strings.SplitN(h, "[", 2)[0]
		h = strings.TrimSpace(h)
		if h != "" {
			clientHost = &h
		}
	}
	return userName, clientHost
}

func parseTimeToSeconds(timeStr string) float64 {
	timeStr = strings.TrimSpace(timeStr)
	parts := strings.Split(timeStr, ":")
	if len(parts) != 3 {
		return 0
	}
	var hours, minutes int
	var seconds float64
	fmt.Sscanf(parts[0], "%d", &hours)
	fmt.Sscanf(parts[1], "%d", &minutes)
	fmt.Sscanf(parts[2], "%f", &seconds)
	return float64(hours)*3600 + float64(minutes)*60 + seconds
}

func buildRawBlock(row slowLogRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Time: %s\n", row.StartTime.Format("2006-01-02T15:04:05.000000Z"))
	if row.UserHost != "" {
		fmt.Fprintf(&b, "# User@Host: %s\n", row.UserHost)
	}
	if row.DB.Valid && row.DB.String != "" {
		fmt.Fprintf(&b, "# Schema: %s\n", row.DB.String)
	}
	fmt.Fprintf(&b, "# Query_time: %s  Lock_time: %s  Rows_sent: %d  Rows_examined: %d\n",
		row.QueryTime, row.LockTime, row.RowsSent, row.RowsExamined)
	fmt.Fprintf(&b, "SET timestamp=%d;\n", row.StartTime.Unix())
	b.WriteString(row.SQLText)
	if !strings.HasSuffix(strings.TrimSpace(row.SQLText), ";") {
		b.WriteString(";")
	}
	return b.String()
}

func checkpointAdvanced(current model.TableIngestionCheckpoint, previous *model.TableIngestionCheckpoint) bool {
	if previous == nil {
		return current.RowsIngested > 0
	}
	if current.LastStartTime.After(previous.LastStartTime) {
		return true
	}
	if current.LastStartTime.Before(previous.LastStartTime) {
		return false
	}
	if current.LastThreadID != previous.LastThreadID {
		return current.LastThreadID > previous.LastThreadID
	}
	if current.LastServerID != previous.LastServerID {
		return current.LastServerID > previous.LastServerID
	}
	return current.LastRowIdentityHash > previous.LastRowIdentityHash
}
