## ADDED Requirements

### Requirement: Resolve `mysql_auto` into one effective acquisition mode
The system MUST translate `mysql_auto` source discovery results into exactly one effective acquisition mode for each source cycle.

#### Scenario: FILE output resolves to mysql_file mode
- **WHEN** source discovery finds that the source MySQL exposes slow query logs through `FILE`
- **THEN** the system SHALL resolve the effective acquisition mode to `mysql_file`

#### Scenario: TABLE output resolves to mysql_table mode
- **WHEN** source discovery finds that the source MySQL exposes slow query logs through `TABLE`
- **THEN** the system SHALL resolve the effective acquisition mode to `mysql_table`

### Requirement: Prefer FILE when both FILE and TABLE are active
The system MUST choose a deterministic preferred mode when the source MySQL exposes both `FILE` and `TABLE`.

#### Scenario: FILE and TABLE are both configured
- **WHEN** source discovery finds that `log_output` includes both `FILE` and `TABLE`
- **THEN** the system SHALL resolve the effective acquisition mode to `mysql_file` and expose that choice through persisted metadata and runtime status

### Requirement: Validate prerequisites for the resolved mode
The system MUST validate mode-specific prerequisites after discovery and before acquisition.

#### Scenario: FILE mode is selected but SSH transport settings are incomplete
- **WHEN** the effective acquisition mode resolves to `mysql_file` and required SSH transport settings are missing or unusable
- **THEN** the system SHALL surface a blocked acquisition state with a diagnostic message instead of attempting remote file acquisition

#### Scenario: TABLE mode is selected but table access is unavailable
- **WHEN** the effective acquisition mode resolves to `mysql_table` and the source DB account cannot read `mysql.slow_log`
- **THEN** the system SHALL surface a blocked or degraded acquisition state with a diagnostic message instead of silently falling back to file acquisition
