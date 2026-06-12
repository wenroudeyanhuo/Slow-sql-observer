## ADDED Requirements

### Requirement: Use the MySQL-discovered slow-log file path by default
The system MUST use the slow-log file path discovered from the source MySQL as the default remote acquisition path for the `mysql_file` branch.

#### Scenario: Discovery provides the remote slow-log file path
- **WHEN** source discovery resolves the effective acquisition mode to `mysql_file` and returns a slow-log file path
- **THEN** the system SHALL use that discovered file path as the default remote file acquisition target

### Requirement: Treat manual remote path as optional override only
The system MUST NOT require a manually configured remote slow-log file path as the primary source contract in `mysql_auto` mode.

#### Scenario: mysql_auto mode runs without a manual remote path override
- **WHEN** the source is configured with `SSO_SOURCE_MODE=mysql_auto` and `SSO_SOURCE_REMOTE_SLOW_LOG_PATH` is not set
- **THEN** the system SHALL still be able to run the `mysql_file` branch using the file path discovered from MySQL

### Requirement: Preserve V3 spool and checkpoint behavior for the FILE branch
The system MUST preserve local spool and byte-range checkpoint behavior for `mysql_file` acquisition.

#### Scenario: FILE branch acquires new bytes successfully
- **WHEN** the effective acquisition mode is `mysql_file` and remote file acquisition succeeds
- **THEN** the system SHALL append the newly acquired bytes to the local spool file and update the file-acquisition checkpoint independently from parser checkpoints
