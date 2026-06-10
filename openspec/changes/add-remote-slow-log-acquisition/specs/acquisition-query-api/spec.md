## ADDED Requirements

### Requirement: Source metadata exposes acquisition configuration
The source metadata API MUST expose the acquisition-relevant metadata of the active source, including source log mode and remote/spool fields when applicable.

#### Scenario: SSH source metadata is returned
- **WHEN** the active source uses `ssh_pull` mode
- **THEN** `GET /api/source` SHALL return acquisition metadata including log mode, remote host, remote slow-log path, local spool path, initial position, and configured spool limit

#### Scenario: Local-file source metadata remains simple
- **WHEN** the active source uses `local_file` mode
- **THEN** `GET /api/source` SHALL continue to return the local parse path and MAY leave remote-only metadata empty

### Requirement: Acquisition status endpoint
The system MUST provide an acquisition status API for the active source.

#### Scenario: Acquisition status can be queried
- **WHEN** a client requests `GET /api/acquisition/status`
- **THEN** the system SHALL return the active source acquisition state, remote access state, transport mode, last successful pull time, latest remote offset, latest remote file identity, latest spool size, and latest acquisition error fields

#### Scenario: Blocked acquisition state is queryable
- **WHEN** acquisition is blocked by missing or unusable SSH/spool configuration
- **THEN** `GET /api/acquisition/status` SHALL return `blocked` as the acquisition state together with the latest diagnostic error message

### Requirement: Existing analysis endpoints remain compatible
The addition of acquisition metadata and status MUST NOT break existing overview, fingerprint list, fingerprint detail, or fingerprint record endpoints.

#### Scenario: Existing overview route remains valid
- **WHEN** a client requests `GET /api/dashboard/overview`
- **THEN** the route SHALL continue returning overview metrics without requiring acquisition-specific request parameters
