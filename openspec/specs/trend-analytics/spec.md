# trend-analytics Specification

## Purpose
TBD - created by archiving change add-trend-analytics-dashboard. Update Purpose after archive.
## Requirements
### Requirement: Dashboard trend analytics API
The system SHALL provide a dashboard trend analytics endpoint that aggregates collected slow-SQL data into time buckets over a bounded recent window. The endpoint SHALL return chart-ready bucket metrics for the currently observed source.

#### Scenario: Dashboard trends can be queried by day
- **WHEN** a client requests dashboard trends with a daily bucket over a supported recent window
- **THEN** the system SHALL return an ordered series of daily buckets
- **AND** each bucket SHALL include at least bucket time, total record count, total query time, average query time, and maximum query time

#### Scenario: Dashboard trends can be filtered by business database
- **WHEN** a client requests dashboard trends with a `dbName` filter
- **THEN** the system SHALL aggregate only qualifying records from that business database

### Requirement: Fingerprint trend analytics API
The system SHALL provide a fingerprint trend analytics endpoint that aggregates one fingerprint's qualifying records into time buckets over a bounded recent window.

#### Scenario: Fingerprint trends show one SQL over time
- **WHEN** a client requests trends for a fingerprint id over a supported recent window
- **THEN** the system SHALL return an ordered series of buckets for that fingerprint
- **AND** each bucket SHALL include at least bucket time, total count, total query time, average query time, and maximum query time

#### Scenario: Missing fingerprint trend target returns not found
- **WHEN** a client requests trends for a fingerprint id that does not exist for the observed source
- **THEN** the system SHALL return a not-found response

### Requirement: Trend query contract is bounded and explicit
The system SHALL expose a documented and validated trend query contract so operators can request recent windows without creating unbounded analytics queries.

#### Scenario: Unsupported bucket is rejected
- **WHEN** a client requests a bucket granularity that the system does not support
- **THEN** the system SHALL reject the request with a client error response

#### Scenario: Oversized recent window is rejected or normalized
- **WHEN** a client requests a recent window outside the supported range
- **THEN** the system SHALL either reject the request with a client error response or normalize it to a documented supported bound

### Requirement: Overview UI shows trend charts
The web UI SHALL display a trend visualization on the overview page so operators can see recent slow-SQL change over time without leaving the dashboard.

#### Scenario: Overview page renders dashboard trend panel
- **WHEN** a user opens the overview page and trend data is available
- **THEN** the page SHALL render at least one trend chart or time-series visualization based on dashboard trend data

#### Scenario: Overview page explains empty trend state
- **WHEN** no qualifying trend data exists for the selected filters
- **THEN** the page SHALL show an empty-state message instead of a broken chart

### Requirement: Fingerprint detail UI shows fingerprint trends
The web UI SHALL display a trend visualization on the fingerprint detail page so operators can inspect how one normalized SQL changes over time.

#### Scenario: Fingerprint detail renders time-series view
- **WHEN** a user opens a fingerprint detail page and trend data is available
- **THEN** the page SHALL render a chart or timeline visualization for that fingerprint

#### Scenario: Fingerprint detail keeps trend filters understandable
- **WHEN** a user changes the active trend window or threshold-related filters
- **THEN** the page SHALL reload the fingerprint trend view using the active filters

