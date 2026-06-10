## ADDED Requirements

### Requirement: Separate acquisition and parser checkpoints
The system MUST persist acquisition checkpoints separately from parser checkpoints. Acquisition checkpoints SHALL track remote pull progress and spool state. Parser checkpoints SHALL continue to track completed-event progress against the local parse file.

#### Scenario: Remote pull and local parse resume independently
- **WHEN** the collector restarts after a partial remote pull or a completed parse cycle
- **THEN** acquisition SHALL resume from the last remote acquisition checkpoint while parsing SHALL resume from the parser checkpoint of the local file

### Requirement: Fully consumed spool reset
The system MUST prevent unbounded spool growth by resetting the local spool file when the parser has consumed all bytes currently present in the spool.

#### Scenario: Fully consumed spool is truncated
- **WHEN** the parser checkpoint offset equals the local spool file end offset after a successful cycle
- **THEN** the system SHALL truncate the spool file to empty and reset the parser checkpoint offset to `0`

#### Scenario: Partially consumed spool remains intact
- **WHEN** unread bytes still remain in the local spool file after a collector cycle
- **THEN** the system SHALL keep the spool file contents unchanged and retain the parser checkpoint offset

### Requirement: Spool metadata persistence
The system MUST persist spool metadata for the active source, including the local spool path and latest spool size in bytes.

#### Scenario: Spool metadata is queryable
- **WHEN** the acquisition stage completes a cycle
- **THEN** the persisted acquisition state SHALL reflect the active local spool path and the latest spool size in bytes

### Requirement: Spool size ceiling
The system MUST support a configurable maximum local spool size for remote acquisition.

#### Scenario: Spool ceiling prevents further pulling
- **WHEN** the local spool size exceeds `SSO_SOURCE_LOCAL_SPOOL_MAX_BYTES`
- **THEN** the acquisition stage SHALL stop pulling additional remote bytes for that cycle and surface the spool-capacity condition through acquisition status
