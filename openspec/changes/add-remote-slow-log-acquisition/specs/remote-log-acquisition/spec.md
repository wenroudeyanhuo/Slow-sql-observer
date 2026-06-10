## ADDED Requirements

### Requirement: Source log mode selection
The system MUST support explicit source log modes for slow-log acquisition. It MUST support `local_file` mode and `ssh_pull` mode. `local_file` mode SHALL continue to read a directly accessible local slow-log file. `ssh_pull` mode SHALL acquire remote slow-log content over SSH before parsing.

#### Scenario: Local-file mode remains valid
- **WHEN** `SSO_SOURCE_LOG_MODE=local_file` and `SSO_SOURCE_SLOW_LOG_PATH` points to a readable local slow-log file
- **THEN** the collector SHALL continue using the local file as the parse source without requiring any SSH configuration

#### Scenario: SSH-pull mode requires remote acquisition inputs
- **WHEN** `SSO_SOURCE_LOG_MODE=ssh_pull`
- **THEN** the system SHALL require remote host, remote user, remote slow-log path, SSH private key path, known-hosts path, and local spool directory configuration before acquisition starts

### Requirement: Linux/OpenSSH remote source boundary
The `ssh_pull` mode MUST support only remote Linux/OpenSSH environments that provide a readable slow-log file and standard shell command execution semantics.

#### Scenario: Unsupported remote environment is rejected
- **WHEN** the configured remote source cannot satisfy the required Linux/OpenSSH command assumptions
- **THEN** the acquisition stage SHALL fail fast and report that the remote source is unsupported for V3 remote acquisition

### Requirement: First-time acquisition position
The system MUST support explicit first-time acquisition positioning with `start` and `end` modes. If no value is provided, it MUST default to `end`.

#### Scenario: Default first acquisition starts at file end
- **WHEN** `ssh_pull` mode is used for a source with no prior acquisition checkpoint and no explicit initial-position override
- **THEN** the acquisition stage SHALL begin at the current remote file end and only pull subsequently appended bytes

#### Scenario: Backfill acquisition starts at file head
- **WHEN** `SSO_SOURCE_INITIAL_POSITION=start` is configured for a source with no prior acquisition checkpoint
- **THEN** the acquisition stage SHALL begin pulling from remote offset `0`

### Requirement: Remote acquisition into local spool
In `ssh_pull` mode, the system MUST pull newly appended bytes from the configured remote slow-log file into a local spool file before running the existing framing and parsing pipeline.

#### Scenario: Remote bytes are appended to spool
- **WHEN** the remote slow-log file has grown beyond the last recorded acquisition offset
- **THEN** the acquisition stage SHALL copy only the new byte range into the local spool file and update the remote acquisition checkpoint

#### Scenario: No new remote bytes leaves spool unchanged
- **WHEN** the remote slow-log file has not grown since the last successful pull
- **THEN** the acquisition stage SHALL not duplicate previously pulled bytes in the local spool file

### Requirement: Pragmatic remote rotation handling
The system MUST detect remote file recreation, replacement, or truncation and treat the remote source as a new current file. It MUST resume acquisition from offset `0` of the new current file and SHALL NOT require archived-tail recovery from older rotated files.

#### Scenario: Remote file identity changes
- **WHEN** the remote slow-log file identity differs from the recorded acquisition checkpoint
- **THEN** the acquisition stage SHALL reset the remote acquisition offset to `0` and continue pulling from the new current file

#### Scenario: Remote file size shrinks
- **WHEN** the remote slow-log file size is smaller than the last recorded remote offset
- **THEN** the acquisition stage SHALL treat the file as truncated and restart remote pulling from offset `0`
