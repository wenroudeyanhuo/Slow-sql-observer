package model

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"time"
)

const (
	CollectorStateIdle     = "idle"
	CollectorStateHealthy  = "healthy"
	CollectorStateDegraded = "degraded"
	CollectorStateError    = "error"

	SourceAccessUnknown      = "unknown"
	SourceAccessAccessible   = "accessible"
	SourceAccessInaccessible = "inaccessible"

	AcquisitionStateIdle     = "idle"
	AcquisitionStateHealthy  = "healthy"
	AcquisitionStateDegraded = "degraded"
	AcquisitionStateError    = "error"
	AcquisitionStateBlocked  = "blocked"

	LogModeLocalFile = "local_file"
	LogModeSSHPull   = "ssh_pull"
	LogModeMySQLAuto = "mysql_auto"

	EffectiveModeMySQLFile  = "mysql_file"
	EffectiveModeMySQLTable = "mysql_table"

	DiscoveryStateUnknown = "unknown"
	DiscoveryStateHealthy = "healthy"
	DiscoveryStateBlocked = "blocked"
	DiscoveryStateError   = "error"

	InitialPositionStart = "start"
	InitialPositionEnd   = "end"
)

type Source struct {
	ID                    int64     `json:"id"`
	Key                   string    `json:"key"`
	InstanceName          string    `json:"instanceName"`
	SlowLogPath           string    `json:"slowLogPath"`
	Description           *string   `json:"description"`
	DatabaseDSNConfigured bool      `json:"databaseDsnConfigured"`
	DatabaseHost          *string   `json:"databaseHost"`
	DatabaseVersion       *string   `json:"databaseVersion"`
	LogMode               string    `json:"logMode"`
	RemoteHost            *string   `json:"remoteHost"`
	RemotePort            *int      `json:"remotePort"`
	RemoteUser            *string   `json:"remoteUser"`
	RemoteSlowLogPath     *string   `json:"remoteSlowLogPath"`
	LocalSpoolPath        *string   `json:"localSpoolPath"`
	InitialPosition       string    `json:"initialPosition"`
	LocalSpoolMaxBytes    *int64    `json:"localSpoolMaxBytes"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type SourceDiscovery struct {
	SourceID            int64     `json:"sourceId"`
	DiscoveryState      string    `json:"discoveryState"`
	SlowLogEnabled      *bool     `json:"slowLogEnabled"`
	DiscoveredLogOutput *string   `json:"discoveredLogOutput"`
	DiscoveredFilePath  *string   `json:"discoveredFilePath"`
	SourceVersion       *string   `json:"sourceVersion"`
	SourceHost          *string   `json:"sourceHost"`
	EffectiveAcqMode    *string   `json:"effectiveAcquisitionMode"`
	DiagnosticMessage   *string   `json:"diagnosticMessage"`
	DiscoveredAt        time.Time `json:"discoveredAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type TableIngestionCheckpoint struct {
	SourceID            int64     `json:"sourceId"`
	LastStartTime       time.Time `json:"lastStartTime"`
	LastThreadID        int64     `json:"lastThreadId"`
	LastServerID        int64     `json:"lastServerId"`
	LastRowIdentityHash string    `json:"lastRowIdentityHash"`
	RowsIngested        int64     `json:"rowsIngested"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type SourceMetadataUpdate struct {
	DatabaseHost    *string
	DatabaseVersion *string
}

type CollectorStatus struct {
	SourceID               int64      `json:"sourceId"`
	CollectorState         string     `json:"collectorState"`
	SourceAccessState      string     `json:"sourceAccessState"`
	LastSuccessfulIngestAt *time.Time `json:"lastSuccessfulIngestAt"`
	LastCheckpointOffset   *int64     `json:"lastCheckpointOffset"`
	LastFileIdentity       *string    `json:"lastFileIdentity"`
	LastErrorAt            *time.Time `json:"lastErrorAt"`
	LastErrorMessage       *string    `json:"lastErrorMessage"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type AcquisitionCheckpoint struct {
	SourceID           int64     `json:"sourceId"`
	TransportMode      string    `json:"transportMode"`
	RemoteHost         *string   `json:"remoteHost"`
	RemotePath         *string   `json:"remotePath"`
	RemoteFileIdentity *string   `json:"remoteFileIdentity"`
	LastRemoteOffset   int64     `json:"lastRemoteOffset"`
	LocalSpoolPath     *string   `json:"localSpoolPath"`
	LastSpoolSizeBytes int64     `json:"lastSpoolSizeBytes"`
	InitialPosition    string    `json:"initialPosition"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type AcquisitionStatus struct {
	SourceID               int64      `json:"sourceId"`
	AcquisitionState       string     `json:"acquisitionState"`
	RemoteAccessState      string     `json:"remoteAccessState"`
	TransportMode          string     `json:"transportMode"`
	LastSuccessfulPullAt   *time.Time `json:"lastSuccessfulPullAt"`
	LastRemoteOffset       *int64     `json:"lastRemoteOffset"`
	LastRemoteFileIdentity *string    `json:"lastRemoteFileIdentity"`
	LastSpoolSizeBytes     *int64     `json:"lastSpoolSizeBytes"`
	LastErrorAt            *time.Time `json:"lastErrorAt"`
	LastErrorMessage       *string    `json:"lastErrorMessage"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type SlowQueryRecord struct {
	ID                int64     `json:"id"`
	SourceID          int64     `json:"sourceId"`
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
	SourceID      int64     `json:"sourceId"`
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
	SourceID     int64     `json:"sourceId"`
	LogFilePath  string    `json:"logFilePath"`
	LogFileHash  string    `json:"logFileHash"`
	FileIdentity string    `json:"fileIdentity"`
	LastOffset   int64     `json:"lastOffset"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type FingerprintRecordView struct {
	ActiveMinQueryTimeSec float64 `json:"activeMinQueryTimeSec"`
	Fingerprint
	FingerprintStats
}

type Overview struct {
	ActiveMinQueryTimeSec float64                 `json:"activeMinQueryTimeSec"`
	TotalRecords          int64                   `json:"totalRecords"`
	TotalFingerprints     int64                   `json:"totalFingerprints"`
	TotalQueryTimeSec     float64                 `json:"totalQueryTimeSec"`
	AvgQueryTimeSec       float64                 `json:"avgQueryTimeSec"`
	MaxQueryTimeSec       float64                 `json:"maxQueryTimeSec"`
	LastIngestedAt        *time.Time              `json:"lastIngestedAt"`
	TopFingerprints       []FingerprintRecordView `json:"topFingerprints"`
}

type OverviewParams struct {
	MinQueryTimeSec float64
}

const (
	TrendBucketDay  = "day"
	TrendBucketHour = "hour"
)

type TrendParams struct {
	Bucket          string
	Days            int
	DBName          string
	MinQueryTimeSec float64
}

type DashboardTrendBucket struct {
	BucketStart       time.Time `json:"bucketStart"`
	TotalRecords      int64     `json:"totalRecords"`
	TotalFingerprints int64     `json:"totalFingerprints"`
	TotalQueryTimeSec float64   `json:"totalQueryTimeSec"`
	AvgQueryTimeSec   float64   `json:"avgQueryTimeSec"`
	MaxQueryTimeSec   float64   `json:"maxQueryTimeSec"`
}

type DashboardTrends struct {
	ActiveMinQueryTimeSec float64                `json:"activeMinQueryTimeSec"`
	Bucket                string                 `json:"bucket"`
	Days                  int                    `json:"days"`
	DBName                string                 `json:"dbName,omitempty"`
	WindowStart           time.Time              `json:"windowStart"`
	WindowEnd             time.Time              `json:"windowEnd"`
	Series                []DashboardTrendBucket `json:"series"`
}

type FingerprintTrendBucket struct {
	BucketStart       time.Time `json:"bucketStart"`
	TotalCount        int64     `json:"totalCount"`
	TotalQueryTimeSec float64   `json:"totalQueryTimeSec"`
	AvgQueryTimeSec   float64   `json:"avgQueryTimeSec"`
	MaxQueryTimeSec   float64   `json:"maxQueryTimeSec"`
}

type FingerprintTrends struct {
	ActiveMinQueryTimeSec float64                  `json:"activeMinQueryTimeSec"`
	FingerprintID         int64                    `json:"fingerprintId"`
	Bucket                string                   `json:"bucket"`
	Days                  int                      `json:"days"`
	WindowStart           time.Time                `json:"windowStart"`
	WindowEnd             time.Time                `json:"windowEnd"`
	Series                []FingerprintTrendBucket `json:"series"`
}

type ListFingerprintsParams struct {
	Page            int
	PageSize        int
	SortBy          string
	SortOrder       string
	DBName          string
	SQLType         string
	Keyword         string
	MinQueryTimeSec float64
}

type GetFingerprintParams struct {
	MinQueryTimeSec float64
}

type ListFingerprintRecordsParams struct {
	Page            int
	PageSize        int
	SortBy          string
	SortOrder       string
	MinQueryTimeSec float64
}

type PaginatedFingerprints struct {
	ActiveMinQueryTimeSec float64                 `json:"activeMinQueryTimeSec"`
	Items                 []FingerprintRecordView `json:"items"`
	Total                 int64                   `json:"total"`
	Page                  int                     `json:"page"`
	PageSize              int                     `json:"pageSize"`
}

type PaginatedRecords struct {
	ActiveMinQueryTimeSec float64           `json:"activeMinQueryTimeSec"`
	Items                 []SlowQueryRecord `json:"items"`
	Total                 int64             `json:"total"`
	Page                  int               `json:"page"`
	PageSize              int               `json:"pageSize"`
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

type AcquisitionResult struct {
	SourceLogPath        string
	ParsePath            string
	TransportMode        string
	RemoteAccessState    string
	RemoteFileIdentity   string
	RemoteOffsetStart    int64
	RemoteOffsetEnd      int64
	SpoolPath            string
	SpoolSizeBytes       int64
	ShouldParse          bool
	ShouldTruncate       bool
	AcquisitionState     string
	AcquisitionError     error
	BlockedConfiguration bool
}

func SourceKey(instanceName, slowLogPath string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(instanceName) + "|" + strings.TrimSpace(slowLogPath)))
	return hex.EncodeToString(sum[:])
}
