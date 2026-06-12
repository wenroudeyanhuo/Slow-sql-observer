# AI Execution Plan for OpenSpec V4

## Document Purpose

This document is written for AI-assisted planning and implementation.
It defines the project scope, boundaries, configuration model, sequencing, deliverables, and acceptance criteria for V4 of Slow SQL Observer.

This document should be treated as the source-of-truth requirements brief for the next version after the current V3 remote acquisition model.

## How AI Should Use This Document

When an AI agent or OpenSpec workflow consumes this file, it should:

1. treat this document as the authoritative V4 requirements source
2. preserve all stated scope boundaries and non-goals
3. derive change proposals, design documents, specs, and tasks from this document instead of re-expanding the scope
4. evolve the existing V3 implementation incrementally instead of redesigning the system from scratch
5. explicitly surface any conflict between this document and the current V3 assumptions

If an implementation choice would keep the old "remote file path is the primary source entrypoint" model, that choice should be treated as conflicting with this V4 brief and must not be adopted silently.

## Version Definition

V4 upgrades Slow SQL Observer from a host-path-oriented remote acquisition model into a MySQL-oriented source acquisition model.

The key shift is:

- V3 asks the operator to tell the system where the remote slow log file is
- V4 asks the operator to connect the system to the remote MySQL source first
- V4 then discovers how that MySQL instance exposes slow-query logs and chooses the correct acquisition path

V4 must preserve the current single-source product boundary while replacing the remote-source mental model with a source-DB-first model.

## Project Definition

Build a single-source MySQL slow SQL analysis system whose primary remote-source entrypoint is the source MySQL connection, not a manually supplied remote slow-log file path.

The system should:

- connect to one observed MySQL source
- inspect the source MySQL slow-log configuration
- detect whether the source exposes slow logs through `FILE`, `TABLE`, or both
- choose the correct acquisition strategy automatically
- continue to parse, fingerprint, aggregate, and persist analysis results in the analysis database
- expose source discovery metadata, acquisition runtime status, and parser runtime status
- preserve local-file mode for development and manual deployments

## Product Goal

Deliver a V4 that proves the product can connect to a real remote MySQL source and automatically determine how to acquire slow-query data without requiring the operator to hardcode the remote log path as the primary source contract.

V4 should make all of the following true:

1. the observed MySQL instance becomes the primary remote source contract
2. the system can inspect MySQL slow-log settings before choosing an acquisition path
3. `FILE`-based and `TABLE`-based slow-log outputs are both modeled explicitly
4. remote file acquisition remains supported, but only as a transport chosen after MySQL inspection
5. source discovery results and chosen acquisition mode are visible through APIs and UI
6. operators can understand whether failure happened during source discovery, file acquisition, table acquisition, parsing, or storage

## Non-Goals for V4

Do not implement the following in V4:

- multi-source orchestration
- automatic source-side MySQL configuration changes
- automatic enabling of slow query logging on the source
- non-MySQL source types
- binlog-based SQL capture
- protocol-level query interception
- `performance_schema` ingestion
- Kubernetes operators, sidecars, or source-host agents
- alerting, paging, or workflow automation
- query optimization recommendation logic

## Scope Boundaries

### In Scope

- one observed MySQL source
- one analysis database
- one collector runtime
- one source discovery flow
- one resolved acquisition mode at a time
- `FILE` output support through discovered-path remote file acquisition
- `TABLE` output support through direct SQL ingestion from `mysql.slow_log`
- source discovery metadata exposure
- acquisition and parser runtime status exposure
- one backend API service
- one frontend web application

### Out of Scope

- multiple source MySQL instances
- mixed concurrent ingestion from many sources
- source-side slow-log configuration automation
- remote archived log replay across historical rotated files
- non-MySQL log backends
- general-purpose database observability outside slow-query logs

## Core Model

V4 must model five distinct concepts:

1. observed source MySQL instance
2. source slow-log configuration discovered from MySQL
3. resolved acquisition strategy chosen from that configuration
4. parser/fingerprint/aggregation pipeline
5. analysis database used by Slow SQL Observer

The conceptual flow is:

```text
source MySQL instance
    -> source discovery
    -> resolved slow-log acquisition mode
        -> FILE path discovered from MySQL, then remote file acquisition
        -> or TABLE rows read directly from mysql.slow_log
    -> parser / mapper
    -> fingerprint
    -> aggregator
    -> analysis database
    -> API
    -> web UI
```

## Configuration Model

V4 must separate three different concerns:

1. source identity and source DB connectivity
2. optional file-acquisition transport credentials
3. analysis database and runtime settings

### Source Configuration

Required for remote MySQL-oriented mode:

- `SSO_SOURCE_MODE`
- `SSO_SOURCE_INSTANCE_NAME`
- `SSO_SOURCE_DB_DSN`

Supported values for `SSO_SOURCE_MODE`:

- `local_file`
- `mysql_auto`

Rules:

- `mysql_auto` becomes the preferred production-oriented mode
- in `mysql_auto`, `SSO_SOURCE_DB_DSN` is required
- the source DB connection is the authoritative entrypoint for remote source discovery
- the system must inspect the source MySQL configuration before choosing the acquisition path

Optional source metadata fields:

- `SSO_SOURCE_TIMEZONE`
- `SSO_SOURCE_DESCRIPTION`

### Local File Compatibility Configuration

Required only in `local_file` mode:

- `SSO_SOURCE_SLOW_LOG_PATH`

Rules:

- `local_file` remains supported for development, demo use, and mounted-log deployments
- `local_file` should behave similarly to current behavior

### Remote File Transport Configuration

These fields are not the primary source definition in V4.
They are transport configuration used only when MySQL discovery resolves to `FILE` output:

- `SSO_SOURCE_REMOTE_HOST`
- `SSO_SOURCE_REMOTE_PORT`
- `SSO_SOURCE_REMOTE_USER`
- `SSO_SOURCE_SSH_PRIVATE_KEY_PATH`
- `SSO_SOURCE_SSH_KNOWN_HOSTS_PATH`
- `SSO_SOURCE_LOCAL_SPOOL_DIR`
- `SSO_SOURCE_INITIAL_POSITION`
- `SSO_SOURCE_LOCAL_SPOOL_MAX_BYTES`

Optional override or fallback fields:

- `SSO_SOURCE_REMOTE_SLOW_LOG_PATH`

Rules:

- `SSO_SOURCE_REMOTE_SLOW_LOG_PATH` must no longer be the primary required input for remote mode
- when MySQL discovery returns a slow-log file path, that discovered path should be preferred
- `SSO_SOURCE_REMOTE_SLOW_LOG_PATH` may exist only as an explicit override or fallback for exceptional environments
- if the source DB host can be parsed from `SSO_SOURCE_DB_DSN`, the system may use that host as the default remote SSH host unless explicitly overridden

### Analysis Configuration

Required:

- `SSO_ANALYSIS_DB_DSN`
- `SSO_ANALYSIS_DB_SCHEMA`

Rules:

- this database stores parsed records, fingerprints, aggregate stats, checkpoints, discovery metadata, and runtime state

### Runtime Configuration

Fields should include at least:

- `SSO_SERVER_ADDR`
- `SSO_WEB_DIR`
- `SSO_COLLECTOR_POLL_INTERVAL`
- `SSO_RAW_RECORD_RETENTION_DAYS`
- `SSO_LOG_LEVEL`

### Backward Compatibility

V4 should support the V3 configuration names for one compatibility cycle where reasonable, but:

- V4 documentation must prefer `SSO_SOURCE_MODE=mysql_auto`
- old remote-path-first assumptions must be marked as deprecated
- any compatibility behavior must emit warnings
- the migration path from V3 remote file assumptions to V4 source-DB-first behavior must be documented

## Source Discovery Requirements

V4 must introduce source discovery as a first-class stage.

At minimum, the system should query the source MySQL for:

- whether slow query logging is enabled
- current `log_output`
- current slow-log file path when available
- source version
- source host identity when available

Suggested discovery fields include:

- `@@global.slow_query_log`
- `@@global.log_output`
- `@@global.slow_query_log_file`
- `@@global.long_query_time`
- `@@hostname`
- `@@port`
- `SELECT VERSION()`

Discovery rules:

- if slow query logging is disabled, the system must enter a blocked or degraded acquisition state with a clear message
- if `log_output` resolves to `FILE`, the system must use file acquisition behavior based on the discovered path
- if `log_output` resolves to `TABLE`, the system must use direct table ingestion behavior
- if `log_output` contains both `FILE` and `TABLE`, the system must choose a deterministic preferred strategy and expose the chosen strategy explicitly

## Required Acquisition Modes

V4 must distinguish:

### 1. Local File Mode

Behavior:

- parse a locally readable slow-log file directly

Use case:

- local development
- already-mounted logs
- manual environments where the file is intentionally local

### 2. MySQL Auto Mode with FILE Output

Behavior:

- connect to source MySQL
- discover that slow logs are emitted to `FILE`
- resolve the slow-log file path from MySQL
- acquire bytes from that discovered file through the remote file transport
- spool locally before parsing

Important rule:

- the file path used for acquisition should come from MySQL discovery, not from a hand-entered required config path

### 3. MySQL Auto Mode with TABLE Output

Behavior:

- connect to source MySQL
- discover that slow logs are emitted to `TABLE`
- read rows from `mysql.slow_log`
- map each row into the existing record model or a compatible ingestion record

Important rule:

- `TABLE` output must not require SSH or a remote file path

## Deployment Assumptions

V4 must explicitly assume:

- the analysis service can connect to the source MySQL over the network
- source discovery is always done through the source MySQL connection in `mysql_auto` mode
- remote file transport is needed only when MySQL discovery resolves to `FILE`
- `TABLE` ingestion requires a source DB user that can read slow-log rows

V4 documentation must explicitly state the source-side prerequisites:

- slow query logging is enabled
- the source DB account can read the required metadata variables
- if `FILE` output is used, an SSH user can read the discovered slow-log file
- if `TABLE` output is used, the DB account can read `mysql.slow_log`

## Required Modules

### 1. Source Discovery

Responsibilities:

- connect to the source MySQL
- inspect slow-log runtime configuration
- determine whether `FILE`, `TABLE`, or both are configured
- persist discovery metadata and the resolved effective acquisition mode

Expected outputs:

- source discovery records
- chosen acquisition strategy
- discovered file path or table mode metadata

### 2. Acquisition Strategy Resolver

Responsibilities:

- translate source discovery results into the active acquisition mode
- validate that the required credentials for the resolved mode are present
- expose blocked configuration states clearly

Expected outputs:

- `effectiveAcquisitionMode`
- validation results
- operator-facing diagnostic messages

### 3. Remote File Acquisition

Responsibilities:

- handle the `FILE` branch after discovery
- fetch slow-log bytes from the discovered remote file
- preserve spool and checkpoint behavior from V3 where still applicable

Expected outputs:

- local spool file updates
- acquisition checkpoints
- acquisition runtime status

### 4. Table Acquisition

Responsibilities:

- handle the `TABLE` branch after discovery
- read new rows from `mysql.slow_log`
- map rows into the existing ingestion model
- persist a resumable table-ingestion checkpoint

Expected outputs:

- structured raw records derived from table rows
- table-mode checkpoints
- acquisition runtime status

### 5. Parser / Mapper

Responsibilities:

- continue parsing file blocks for file-based inputs
- introduce row-to-record mapping for table-based inputs
- preserve the downstream normalized record contract

Expected outputs:

- `SlowQueryRecord` domain objects from both acquisition branches

### 6. Fingerprint

Responsibilities:

- preserve existing normalization and hashing behavior
- remain agnostic to whether the source record came from file or table mode

Expected outputs:

- `Fingerprint` objects
- normalized SQL text

### 7. Aggregator

Responsibilities:

- preserve aggregate update behavior
- remain agnostic to acquisition origin

Expected outputs:

- `FingerprintStats`

### 8. Storage

Responsibilities:

- persist raw records
- persist fingerprints and aggregate stats
- persist source discovery metadata
- persist acquisition checkpoints for file and table branches
- persist parser checkpoints where still applicable
- persist discovery, acquisition, and parser runtime state

Expected outputs:

- queryable read models
- safe migrations from V3

### 9. Runtime Status

Responsibilities:

- distinguish source discovery health
- distinguish acquisition health
- distinguish parser/ingest health
- expose enough detail for operators to know which stage is failing

Expected outputs:

- source discovery status
- acquisition status
- parser collector status

### 10. API

Responsibilities:

- serve overview statistics
- serve fingerprint list and detail
- serve raw sample records
- serve source metadata
- serve source discovery metadata
- serve acquisition runtime status
- serve parser runtime status

Expected outputs:

- stable HTTP APIs

### 11. Web UI

Responsibilities:

- display source MySQL identity
- display discovered slow-log mode
- display resolved effective acquisition mode
- display discovery status, acquisition status, and parser status separately
- explain clearly whether the source is using `FILE` or `TABLE`

Expected outputs:

- dashboard source/acquisition panels
- list/detail source context
- clear state messaging for discovery failures vs acquisition failures vs parser failures

## Suggested Domain Models

### Source

Fields should include at least:

- source id
- source instance name
- source mode
- source DB DSN presence flag
- source host or version when available
- description
- created at
- updated at

### SourceDiscovery

Fields should include at least:

- source id
- slow query log enabled flag
- discovered log output
- discovered slow-log file path when available
- discovered source host
- discovered source version
- discovery state
- last discovered at
- last error at
- last error message

### AcquisitionStatus

Fields should include at least:

- source id
- configured source mode
- effective acquisition mode
- acquisition state
- remote access state where relevant
- table access state where relevant
- last successful acquisition at
- last acquisition checkpoint
- last spool size when relevant
- last error at
- last error message

### FileAcquisitionCheckpoint

Fields should include at least:

- source id
- discovered remote file path
- remote file identity
- last remote offset
- local spool path
- last local spool size
- updated at

### TableAcquisitionCheckpoint

Fields should include at least:

- source id
- checkpoint tuple that can resume table ingestion deterministically
- updated at

The exact tuple may vary by implementation, but it must be documented and must minimize duplicate re-ingestion.

### CollectorStatus

Fields should include at least:

- source id
- parser collector state
- last successful ingest at
- last downstream checkpoint
- last error at
- last error message
- updated at

## Data-Path Requirements

### FILE Branch

When discovery resolves to `FILE`:

- the system must use the path discovered from MySQL as the default acquisition path
- remote SSH acquisition should only begin after discovery succeeds
- spool and parser behavior may reuse the V3 model

### TABLE Branch

When discovery resolves to `TABLE`:

- the system must ingest directly from `mysql.slow_log`
- the system must not require SSH credentials
- the system must define a deterministic table checkpoint model
- the system must map table rows into the existing normalized downstream model

### Mixed FILE and TABLE Branch

When discovery resolves to both `FILE` and `TABLE`:

- the system must choose one effective acquisition strategy deterministically
- the chosen strategy must be visible through API and UI
- the unchosen strategy must not run silently in parallel in V4

## Implementation Order

Follow this order strictly unless there is a strong reason to change it:

1. redefine remote source configuration around `mysql_auto`
2. introduce source discovery models and status
3. implement a strategy resolver for discovered `FILE` vs `TABLE`
4. adapt the existing V3 file acquisition path so it consumes the discovered slow-log file path instead of a required manual path
5. introduce table-mode ingestion and checkpointing
6. update storage schema and compatibility logic
7. update APIs for discovery metadata and resolved acquisition mode
8. update frontend pages to display discovery mode and effective acquisition mode
9. revise documentation for real remote MySQL onboarding and V3 migration

## Deliverables by Phase

### Phase 1: Source-DB-First Configuration

- `mysql_auto` mode definition
- source DB requiredness in remote mode
- deprecation plan for remote-path-first assumptions

### Phase 2: Discovery and Strategy Resolution

- source discovery service
- persisted discovery metadata
- chosen effective acquisition mode

### Phase 3: Acquisition Branches

- discovered-path file acquisition branch
- table acquisition branch
- checkpoints and runtime status

### Phase 4: Persistence and Compatibility

- schema updates
- V3 migration logic
- compatibility read models

### Phase 5: Application Layer

- discovery metadata API
- acquisition status API updates
- existing analysis API compatibility

### Phase 6: UI Layer

- discovery status display
- `FILE` vs `TABLE` explanation in UI
- clearer failure-state messaging

### Phase 7: Packaging and Documentation

- V4 configuration docs
- remote MySQL onboarding guide
- FILE vs TABLE operating guide
- migration guide from V3

## Acceptance Criteria for V4

V4 is complete when all of the following are true:

1. remote production-oriented mode is defined around the source MySQL connection, not a required remote file path
2. `mysql_auto` mode can inspect source MySQL slow-log settings before acquisition begins
3. the system supports both discovered `FILE` output and direct `TABLE` output
4. `SSO_SOURCE_REMOTE_SLOW_LOG_PATH` is no longer the primary required configuration for remote mode
5. source discovery metadata and resolved effective acquisition mode are exposed through APIs
6. the web UI displays source discovery state, effective acquisition mode, and parser status clearly
7. a source with slow logging disabled surfaces a clear blocked/degraded state
8. `TABLE` mode works without SSH configuration
9. `FILE` mode still supports spool, checkpointing, and parser compatibility
10. V3 users have a documented migration path

## Design Rules for AI Implementation

- preserve single-source scope in V4
- treat the source MySQL connection as the authoritative remote-source entrypoint
- do not keep remote file path as the primary required remote contract
- preserve `local_file` compatibility
- preserve the current parser/fingerprint/aggregation behavior as much as possible
- separate source discovery, acquisition, and parser runtime states
- keep `FILE` and `TABLE` branches explicit in code, API, and UI
- do not silently expand V4 into source management, agent deployment, or automatic MySQL configuration

## Future Extensions

These may be proposed after V4, but should not be merged into the V4 scope:

- auto-enabling slow query logs on the source
- runtime switching policies between `FILE` and `TABLE`
- multi-source remote MySQL management
- source-host agents
- archived rotated-log recovery
- `performance_schema` ingestion
- deeper masking and compliance policies
- alerting and notification workflows
