## ADDED Requirements

### Requirement: Show discovered slow-log mode and effective acquisition mode
The web UI MUST show both the discovered source slow-log output mode and the effective acquisition mode chosen by the system.

#### Scenario: Dashboard displays FILE-based source
- **WHEN** the active source is discovered as `FILE` output and resolves to `mysql_file`
- **THEN** the dashboard SHALL show that the source MySQL exposes `FILE` output and that the system is currently using `mysql_file` acquisition

#### Scenario: Dashboard displays TABLE-based source
- **WHEN** the active source is discovered as `TABLE` output and resolves to `mysql_table`
- **THEN** the dashboard SHALL show that the source MySQL exposes `TABLE` output and that the system is currently using `mysql_table` acquisition

### Requirement: Distinguish discovery, acquisition, and parser failures in UI
The web UI MUST distinguish discovery failures from acquisition failures and parser failures.

#### Scenario: Discovery fails before data acquisition
- **WHEN** the system cannot inspect the source MySQL slow-log configuration
- **THEN** the UI SHALL show a discovery-stage failure message instead of presenting it as a generic parser or collector error

#### Scenario: Acquisition fails after discovery succeeds
- **WHEN** discovery succeeds but the chosen acquisition branch fails
- **THEN** the UI SHALL show acquisition-stage failure context while keeping parser health messaging distinct

### Requirement: Explain why SSH is or is not required
The web UI MUST help operators understand whether the active source currently depends on SSH file acquisition or direct table ingestion.

#### Scenario: TABLE mode does not require SSH
- **WHEN** the effective acquisition mode is `mysql_table`
- **THEN** the UI SHALL indicate that the source is being ingested directly from `mysql.slow_log` and is not currently using SSH-based file transport
