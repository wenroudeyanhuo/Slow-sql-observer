## ADDED Requirements

### Requirement: Persist collector runtime status
The system MUST persist runtime status for the active source independently from checkpoint data.

#### Scenario: Record successful ingest progress
- **WHEN** the collector completes an ingest cycle successfully
- **THEN** it MUST persist the source's last successful ingest time, last checkpoint offset, and last file identity in collector runtime status

### Requirement: Expose collector health and source accessibility
The system MUST track collector state and source accessibility state for the active source.

#### Scenario: Mark a healthy source
- **WHEN** the collector can read the configured slow log file and complete its cycle without source probe errors
- **THEN** collector runtime status MUST reflect a healthy or running state for that source

#### Scenario: Mark a degraded source
- **WHEN** the collector can continue ingesting from the slow log file but encounters an optional source DB probe failure or retention cleanup failure
- **THEN** collector runtime status MUST record a degraded state and the latest related error details

#### Scenario: Mark an inaccessible source file
- **WHEN** the configured slow log file cannot be found or read
- **THEN** collector runtime status MUST record that source accessibility failure and MUST surface an error state

### Requirement: Preserve latest runtime error details
The system MUST persist the latest collector error time and message for the active source.

#### Scenario: Update the latest error after a collector failure
- **WHEN** the collector encounters a file-access, parsing, storage, or retention failure that prevents part of the cycle from completing normally
- **THEN** the runtime status MUST update the latest error timestamp and error message for that source
