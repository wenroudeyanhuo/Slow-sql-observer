## ADDED Requirements

### Requirement: Support direct ingestion from mysql.slow_log
The system MUST support a `mysql_table` acquisition branch that reads slow-query data directly from `mysql.slow_log`.

#### Scenario: TABLE output is selected
- **WHEN** source discovery resolves the effective acquisition mode to `mysql_table`
- **THEN** the system SHALL read slow-query records directly from `mysql.slow_log` instead of attempting SSH-based file acquisition

### Requirement: TABLE mode must not require SSH transport
The system MUST allow `mysql_table` mode to operate without SSH credentials or remote file path configuration.

#### Scenario: TABLE mode runs without SSH settings
- **WHEN** the effective acquisition mode is `mysql_table`
- **THEN** the system SHALL not require SSH host, user, key, known-hosts, or remote file path configuration to ingest slow-query data

### Requirement: Persist a deterministic table-ingestion checkpoint
The system MUST persist a dedicated checkpoint for table ingestion that supports deterministic forward progress with minimal duplicate re-ingestion.

#### Scenario: TABLE ingestion resumes after restart
- **WHEN** the collector restarts after previously ingesting rows from `mysql.slow_log`
- **THEN** the system SHALL resume table ingestion from the persisted table-ingestion checkpoint rather than restarting full-table ingestion from the beginning

### Requirement: Map TABLE rows into the existing downstream record contract
The system MUST map `mysql.slow_log` rows into the normalized downstream record contract used by fingerprinting, aggregation, and APIs.

#### Scenario: TABLE row becomes a normalized analysis record
- **WHEN** a new row is ingested from `mysql.slow_log`
- **THEN** the system SHALL transform that row into a record compatible with the downstream fingerprint and aggregation pipeline
