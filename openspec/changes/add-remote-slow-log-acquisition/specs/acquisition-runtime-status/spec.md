## ADDED Requirements

### Requirement: Persist acquisition runtime status
The system MUST persist acquisition runtime status independently from parser collector status for the active source.

#### Scenario: Successful remote pull updates acquisition status
- **WHEN** a remote acquisition cycle succeeds
- **THEN** the system SHALL persist the active source id, transport mode, acquisition state, remote access state, last successful pull time, latest remote offset, remote file identity, spool size, and updated time

#### Scenario: Acquisition failure is visible without erasing prior parsed data
- **WHEN** remote acquisition fails but existing parsed analysis data still exists
- **THEN** the system SHALL record the acquisition error state and latest error message without deleting previously parsed analysis data

#### Scenario: Configuration-blocked acquisition is visible
- **WHEN** acquisition cannot start because required SSH or spool configuration is missing or unusable
- **THEN** the system SHALL persist acquisition state as `blocked` together with a diagnostic error message

### Requirement: Distinguish acquisition and parser health
The system MUST keep acquisition status separate from parser/ingest collector status so operators can tell whether failures occurred before parsing or during parsing/storage.

#### Scenario: Acquisition degrades while parser status remains unchanged
- **WHEN** remote access fails before new bytes are pulled
- **THEN** the acquisition status SHALL enter a degraded or error state even if the parser collector status still reflects the last known parser health

#### Scenario: Parser failure does not overwrite acquisition state
- **WHEN** remote acquisition succeeds but parsing or persistence later fails
- **THEN** the acquisition status SHALL continue to report the successful pull state while parser collector status reflects the downstream failure
