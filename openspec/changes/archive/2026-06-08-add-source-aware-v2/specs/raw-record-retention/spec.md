## ADDED Requirements

### Requirement: Configurable raw-record retention
The system MUST support a configurable retention window for raw slow query records.

#### Scenario: Enable retention with a positive number of days
- **WHEN** `SSO_RAW_RECORD_RETENTION_DAYS` is configured with a positive value
- **THEN** the collector MUST treat raw records older than that retention window as eligible for cleanup

#### Scenario: Disable retention cleanup
- **WHEN** `SSO_RAW_RECORD_RETENTION_DAYS` is absent, zero, or negative
- **THEN** the collector MUST leave raw slow query records untouched by retention cleanup

### Requirement: Collector-executed cleanup
The system MUST execute retention cleanup from the collector runtime rather than requiring a separate scheduler.

#### Scenario: Run retention cleanup during collector operation
- **WHEN** a collector cycle completes and retention is enabled
- **THEN** the collector MUST execute raw-record cleanup as part of its runtime loop

### Requirement: Preserve fingerprint and aggregate read models
The system MUST apply retention to raw slow query records only by default.

#### Scenario: Keep fingerprints and aggregate stats after raw-record cleanup
- **WHEN** expired raw records are deleted by retention cleanup
- **THEN** fingerprint metadata and aggregate statistics MUST remain available unless another future feature explicitly removes them
