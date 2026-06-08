package model

import "time"

type SlowQueryRecord struct {
	ID                int64     `json:"id"`
	SourceInstance    string    `json:"sourceInstanceName"`
	SourceLogFilePath string    `json:"sourceLogFilePath"`
	SourceFileID      string    `json:"sourceFileIdentity"`
	SourceOffsetStart int64     `json:"sourceOffsetStart"`
	SourceOffsetEnd   int64     `json:"sourceOffsetEnd"`
	OccurredAt        time.Time `json:"occurredAt"`
	DBName            *string   `json:"dbName"`
	UserName          *string   `json:"userName"`
	ClientHost        *string   `json:"clientHost"`
	RawBlock          string    `json:"rawBlock"`
	RawSQL            string    `json:"rawSql"`
	NormalizedSQL     string    `json:"normalizedSql"`
	FingerprintID     int64     `json:"fingerprintId"`
	FingerprintHash   string    `json:"fingerprintHash"`
	QueryTimeSec      float64   `json:"queryTimeSec"`
	LockTimeSec       *float64  `json:"lockTimeSec"`
	RowsSent          *int64    `json:"rowsSent"`
	RowsExamined      *int64    `json:"rowsExamined"`
	CreatedAt         time.Time `json:"createdAt"`
}

type Fingerprint struct {
	ID            int64     `json:"id"`
	Hash          string    `json:"fingerprintHash"`
	NormalizedSQL string    `json:"normalizedSql"`
	SQLType       string    `json:"sqlType"`
	MainTableName *string   `json:"mainTableName"`
	FirstSeenAt   time.Time `json:"firstSeenAt"`
	LastSeenAt    time.Time `json:"lastSeenAt"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type FingerprintStats struct {
	FingerprintID     int64     `json:"fingerprintId"`
	TotalCount        int64     `json:"totalCount"`
	TotalQueryTimeSec float64   `json:"totalQueryTimeSec"`
	AvgQueryTimeSec   float64   `json:"avgQueryTimeSec"`
	MaxQueryTimeSec   float64   `json:"maxQueryTimeSec"`
	TotalRowsExamined int64     `json:"totalRowsExamined"`
	AvgRowsExamined   float64   `json:"avgRowsExamined"`
	MaxRowsExamined   int64     `json:"maxRowsExamined"`
	TotalRowsSent     int64     `json:"totalRowsSent"`
	AvgRowsSent       float64   `json:"avgRowsSent"`
	MaxRowsSent       int64     `json:"maxRowsSent"`
	FirstSeenAt       time.Time `json:"firstSeenAt"`
	LastSeenAt        time.Time `json:"lastSeenAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type CollectorCheckpoint struct {
	InstanceName string    `json:"instanceName"`
	LogFilePath  string    `json:"logFilePath"`
	LogFileHash  string    `json:"logFileHash"`
	FileIdentity string    `json:"fileIdentity"`
	LastOffset   int64     `json:"lastOffset"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type FingerprintRecordView struct {
	Fingerprint
	FingerprintStats
}

type Overview struct {
	TotalRecords      int64                   `json:"totalRecords"`
	TotalFingerprints int64                   `json:"totalFingerprints"`
	TotalQueryTimeSec float64                 `json:"totalQueryTimeSec"`
	AvgQueryTimeSec   float64                 `json:"avgQueryTimeSec"`
	MaxQueryTimeSec   float64                 `json:"maxQueryTimeSec"`
	LastIngestedAt    *time.Time              `json:"lastIngestedAt"`
	TopFingerprints   []FingerprintRecordView `json:"topFingerprints"`
}

type ListFingerprintsParams struct {
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
	DBName    string
	SQLType   string
	Keyword   string
}

type ListFingerprintRecordsParams struct {
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

type PaginatedFingerprints struct {
	Items    []FingerprintRecordView `json:"items"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"pageSize"`
}

type PaginatedRecords struct {
	Items    []SlowQueryRecord `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

type ProcessedFingerprint struct {
	Hash          string
	NormalizedSQL string
	SQLType       string
	MainTableName *string
}

type IngestRecordInput struct {
	Record      SlowQueryRecord
	Fingerprint ProcessedFingerprint
}

type CollectResult struct {
	FileIdentity    string `json:"fileIdentity"`
	StartOffset     int64  `json:"startOffset"`
	FinalOffset     int64  `json:"finalOffset"`
	EventsProcessed int    `json:"eventsProcessed"`
	BytesRead       int64  `json:"bytesRead"`
}
