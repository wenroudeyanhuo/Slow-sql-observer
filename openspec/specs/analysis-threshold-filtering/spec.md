# analysis-threshold-filtering Specification

## Purpose
TBD - created by archiving change add-analysis-threshold-filter. Update Purpose after archive.
## Requirements
### Requirement: Configurable default analysis threshold
The system SHALL support a configurable minimum query-time threshold for analysis that is independent from MySQL slow-log collection settings. The system SHALL use this threshold as the default filter for overview and slow-SQL analysis endpoints. A threshold value of `0` SHALL disable analysis-layer filtering.

#### Scenario: Default threshold filters analysis views
- **WHEN** `SSO_ANALYSIS_MIN_QUERY_TIME_SEC` is set to `1.0`
- **THEN** overview and slow-SQL analysis endpoints SHALL exclude records whose `query_time_sec` is less than `1.0`

#### Scenario: Disabled threshold includes all collected records
- **WHEN** `SSO_ANALYSIS_MIN_QUERY_TIME_SEC` is set to `0`
- **THEN** overview and slow-SQL analysis endpoints SHALL include all collected records regardless of `query_time_sec`

### Requirement: Collection and storage remain unaffected by analysis threshold
The system SHALL continue ingesting and storing all collected slow-log records even when they are below the active analysis threshold. Analysis-layer filtering SHALL be applied at query time rather than ingest time.

#### Scenario: Below-threshold records are still preserved
- **WHEN** a collected record has `query_time_sec` below the active analysis threshold
- **THEN** the record SHALL still be persisted in storage
- **AND** the record SHALL become visible again if the analysis threshold is lowered or disabled later

### Requirement: Request-level threshold override
The system SHALL allow API consumers to override the default analysis threshold per request by supplying `minQueryTimeSec`. When provided, the request value SHALL take precedence over the configured default for that response only.

#### Scenario: Request override narrows the result set
- **WHEN** the configured default threshold is `1.0`
- **AND** a client requests `minQueryTimeSec=3`
- **THEN** the response SHALL include only records and aggregates whose `query_time_sec` is greater than or equal to `3`

#### Scenario: Request override widens the result set
- **WHEN** the configured default threshold is `1.0`
- **AND** a client requests `minQueryTimeSec=0.2`
- **THEN** the response SHALL include records and aggregates whose `query_time_sec` is greater than or equal to `0.2`

### Requirement: Threshold-aware analysis endpoints
The system SHALL apply the effective analysis threshold consistently to the overview, fingerprint list, fingerprint detail, fingerprint records, dashboard trends, and fingerprint trends views so that users can investigate the same filtered slice of data end to end.

#### Scenario: Fingerprint list only ranks above-threshold statements
- **WHEN** a fingerprint has only records below the effective analysis threshold
- **THEN** that fingerprint SHALL not appear in the fingerprint list response

#### Scenario: Fingerprint records respect the same threshold
- **WHEN** a client opens a fingerprint detail flow with an effective analysis threshold
- **THEN** the fingerprint records response SHALL include only records whose `query_time_sec` is greater than or equal to that threshold

#### Scenario: Dashboard trends respect the same threshold
- **WHEN** a client requests dashboard trends with an effective analysis threshold
- **THEN** the trend buckets SHALL include only records whose `query_time_sec` is greater than or equal to that threshold

#### Scenario: Fingerprint trends respect the same threshold
- **WHEN** a client requests fingerprint trends with an effective analysis threshold
- **THEN** the trend buckets SHALL include only records for that fingerprint whose `query_time_sec` is greater than or equal to that threshold

### Requirement: The effective threshold is documented and visible
The system SHALL document the difference between MySQL collection threshold and Slow SQL Observer analysis threshold. The web UI SHALL expose the active threshold so users can understand why some collected records are not shown in default rankings.

#### Scenario: UI communicates active threshold
- **WHEN** a user visits the slow-SQL dashboard
- **THEN** the UI SHALL show the active minimum query-time threshold being applied to the current analysis view

