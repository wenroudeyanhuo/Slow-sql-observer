## ADDED Requirements

### Requirement: Incremental slow log collection
The system SHALL read one configured MySQL slow query log file incrementally and process only newly available complete events after each collection cycle.

#### Scenario: Resume collection from a saved offset
- **WHEN** a collector starts with an existing checkpoint whose file identity matches the current slow log file
- **THEN** it MUST resume reading from the checkpointed offset instead of reprocessing the file from the beginning

#### Scenario: First run without a checkpoint
- **WHEN** a collector starts and no checkpoint exists for the configured instance and log path
- **THEN** it MUST begin reading from the start of the configured slow log file

### Requirement: Complete event block framing
The collector SHALL frame slow query events as complete raw blocks before handing them to the parser.

#### Scenario: Frame an event using Time headers
- **WHEN** the collector reads slow log content containing multiple `# Time:` headers
- **THEN** it MUST treat each header as the start of a new candidate event block

#### Scenario: Preserve an incomplete trailing block
- **WHEN** the collector reaches the end of currently available file content and the trailing event block does not yet have a complete boundary
- **THEN** it MUST keep that trailing content buffered and MUST NOT emit it for parsing or advance the checkpoint past it

### Requirement: Structured slow log parsing
The parser SHALL convert each complete slow query event block into a structured slow query record.

#### Scenario: Parse a standard slow query block
- **WHEN** the parser receives a complete block containing event time, query metrics, and SQL text
- **THEN** it MUST extract `occurred_at`, `query_time`, `raw_sql`, and the full raw block into the record output

#### Scenario: Allow non-core fields to be absent
- **WHEN** the parser receives a complete block that is missing non-core fields such as `db_name`, `user_name`, `client_host`, `lock_time`, `rows_sent`, or `rows_examined`
- **THEN** it MUST still produce a record as long as event time, query time, and SQL text are present

### Requirement: Durable checkpoint tracking
The system SHALL persist collector progress using checkpoint state that includes instance name, log file path, file identity, last completed offset, and update time.

#### Scenario: Advance checkpoint after a successful event write
- **WHEN** a slow query event is successfully persisted together with its derived fingerprint and aggregate updates
- **THEN** the collector checkpoint MUST advance to the end offset of that fully processed event block

#### Scenario: Do not acknowledge uncommitted progress
- **WHEN** record persistence or aggregate persistence fails for an event
- **THEN** the checkpoint MUST remain at or before the previously committed completed-event offset

### Requirement: Pragmatic file identity handling
The collector SHALL use file identity together with path and offset to decide whether to resume, restart after truncation, or switch to a new file.

#### Scenario: Resume on the same file identity
- **WHEN** the current log file identity matches the checkpoint and the current file size is greater than or equal to the checkpointed offset
- **THEN** the collector MUST resume from the checkpointed offset

#### Scenario: Restart after truncation
- **WHEN** the current log file identity matches the checkpoint but the current file size is smaller than the checkpointed offset
- **THEN** the collector MUST treat the file as truncated and restart reading from offset `0`

#### Scenario: Switch to a new rotated file
- **WHEN** the current log file identity differs from the checkpointed file identity
- **THEN** the collector MUST treat the file as a new source and restart reading from offset `0`
