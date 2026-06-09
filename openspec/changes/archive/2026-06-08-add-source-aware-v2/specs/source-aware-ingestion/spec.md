## ADDED Requirements

### Requirement: Source and analysis configuration separation
The system MUST load source configuration independently from analysis database configuration and prefer V2 source/analysis environment variable names over legacy V1 names.

#### Scenario: Start with V2 source and analysis configuration
- **WHEN** the application starts with `SSO_SOURCE_INSTANCE_NAME`, `SSO_SOURCE_SLOW_LOG_PATH`, `SSO_ANALYSIS_DB_DSN`, and `SSO_ANALYSIS_DB_SCHEMA`
- **THEN** it MUST use those values as the active source and analysis configuration

#### Scenario: Fall back to V1 names for one compatibility cycle
- **WHEN** the application starts without V2 source or analysis names but with `SSO_INSTANCE_NAME`, `SSO_SLOW_LOG_PATH`, `SSO_DB_DSN`, and `SSO_DB_SCHEMA`
- **THEN** it MUST accept those legacy names as fallback values and MUST emit deprecation warnings

### Requirement: First-class source identity
The system MUST persist one active observed source as a first-class model derived from source instance name and slow log path.

#### Scenario: Reuse the same source identity
- **WHEN** the collector starts with the same source instance name and slow log path as a previously recorded source
- **THEN** it MUST reuse that persisted source identity instead of creating a duplicate source record

#### Scenario: Create a new source identity after a source configuration change
- **WHEN** the collector starts with a changed source instance name or changed source slow log path
- **THEN** it MUST create a new source identity and MUST NOT merge its checkpoint or runtime state into the previous source

### Requirement: Slow log file remains the primary ingestion channel
The system MUST continue to collect slow query events from the configured slow log file even when no source DB connection is configured.

#### Scenario: Ingest using only file access
- **WHEN** `SSO_SOURCE_DB_DSN` is absent and the configured slow log file is readable
- **THEN** the collector MUST still ingest slow query events from the file

### Requirement: Optional source DB metadata enrichment
The system MUST treat `SSO_SOURCE_DB_DSN` as an optional source validation and metadata probe connection rather than as the primary ingestion path.

#### Scenario: Enrich source metadata when source DB is available
- **WHEN** `SSO_SOURCE_DB_DSN` is configured and the source database is reachable
- **THEN** the collector MUST record source DB connectivity success and SHOULD persist available metadata such as version or host information

#### Scenario: Continue file ingestion when source DB probe fails
- **WHEN** `SSO_SOURCE_DB_DSN` is configured but the source database probe fails while the slow log file remains accessible
- **THEN** the collector MUST continue slow-log-file ingestion and MUST record the probe failure in source or collector status

### Requirement: Source-scoped checkpoints and raw records
The system MUST associate checkpoints and raw records with the persisted source identity.

#### Scenario: Persist checkpoint for the active source
- **WHEN** an event is successfully ingested for the active source
- **THEN** the completed-event checkpoint MUST be stored against that source identity

#### Scenario: Isolate source-specific raw records
- **WHEN** the collector ingests raw slow query records for one source
- **THEN** those records MUST remain attributable to that source identity for later API and UI queries
