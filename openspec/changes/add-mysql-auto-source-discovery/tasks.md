## 1. Configuration and Source Mode

- [x] 1.1 Introduce `SSO_SOURCE_MODE` with `local_file` and `mysql_auto`, making `mysql_auto` the preferred remote-oriented mode.
- [x] 1.2 Require `SSO_SOURCE_DB_DSN` in `mysql_auto` mode and keep `SSO_SOURCE_SLOW_LOG_PATH` required only for `local_file` mode.
- [x] 1.3 Reclassify `SSO_SOURCE_REMOTE_SLOW_LOG_PATH` as an optional override/fallback instead of the primary required remote input.
- [x] 1.4 Add compatibility warnings and migration behavior for V3 remote-path-first configuration assumptions.

## 2. Source Discovery

- [x] 2.1 Add a source discovery service that connects to the source MySQL and inspects slow-query logging state and `log_output`.
- [x] 2.2 Persist discovery metadata including enabled state, discovered `log_output`, discovered slow-log file path, source version, and host identity.
- [x] 2.3 Persist and expose a discovery-stage runtime state with clear diagnostics for disabled slow logging or failed metadata access.

## 3. Acquisition Strategy Resolution

- [x] 3.1 Add an acquisition strategy resolver that maps discovery output into `local_file`, `mysql_file`, or `mysql_table`.
- [x] 3.2 Default mixed `FILE,TABLE` discovery results to `mysql_file` and expose the chosen effective mode explicitly.
- [x] 3.3 Validate mode-specific prerequisites after discovery and surface blocked states for missing SSH transport or inaccessible `mysql.slow_log`.

## 4. FILE Branch Evolution

- [x] 4.1 Refactor the current V3 remote file acquisition path so it uses the MySQL-discovered slow-log file path by default.
- [x] 4.2 Preserve local spool behavior, file acquisition checkpoints, and parser checkpoint separation for `mysql_file`.
- [x] 4.3 Keep `SSO_SOURCE_REMOTE_SLOW_LOG_PATH` available only as an explicit override or fallback path.

## 5. TABLE Branch Implementation

- [x] 5.1 Add direct ingestion from `mysql.slow_log` for the `mysql_table` branch.
- [x] 5.2 Define and persist a deterministic table-ingestion checkpoint that supports restart-safe forward progress.
- [x] 5.3 Map `mysql.slow_log` rows into the existing downstream record, fingerprint, and aggregation pipeline.

## 6. Storage and Runtime Status

- [x] 6.1 Extend the schema for source discovery metadata, effective acquisition mode, and discovery runtime state.
- [x] 6.2 Extend acquisition status storage so it can represent file-mode and table-mode health distinctly.
- [x] 6.3 Add compatibility and migration logic for existing V3 source metadata and acquisition tables.

## 7. API and Web UI

- [x] 7.1 Extend source/status APIs to expose discovered slow-log mode, discovered file path when relevant, and the effective acquisition mode.
- [x] 7.2 Keep overview, fingerprint list, detail, and records APIs compatible while adding discovery and acquisition context.
- [x] 7.3 Update the dashboard and fingerprint pages to show discovery health, effective acquisition mode, and clear FILE-vs-TABLE operator messaging.

## 8. Documentation and Verification

- [x] 8.1 Update `.env.example` and README files for `mysql_auto`, FILE-mode transport requirements, and TABLE-mode onboarding.
- [x] 8.2 Document the migration path from V3 remote-path-first configuration to V4 source-DB-first configuration.
- [x] 8.3 Add tests for source discovery, mixed FILE/TABLE resolution, discovered file path usage, TABLE-mode checkpoints, and stage-specific API/UI status behavior.
- [x] 8.4 Run `openspec validate add-mysql-auto-source-discovery` and confirm the change is ready for apply.
