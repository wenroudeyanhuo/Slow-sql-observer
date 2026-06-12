## Why

Slow SQL Observer V3 added remote acquisition, but it still models the remote source primarily as "a known slow-log file on a remote host." That is operationally incomplete for real MySQL environments, because the true source contract is the remote MySQL instance and its configured slow-log output mode, not a manually hardcoded file path.

V4 is needed now because the project already has source-aware storage and remote acquisition building blocks. The next step is to make the source MySQL connection the primary remote entrypoint, discover whether slow logs come from `FILE` or `TABLE`, and choose the acquisition strategy automatically.

## What Changes

- Add a source-discovery stage that connects to the observed MySQL instance and inspects whether slow query logging is enabled, which `log_output` modes are active, and which slow-log file path is configured when `FILE` output is used.
- Add a MySQL-auto source mode that makes `SSO_SOURCE_DB_DSN` the primary remote source contract and deprecates the old remote-path-first assumption for production-oriented usage.
- Keep `local_file` mode for local development and mounted-log setups, but make `mysql_auto` the preferred production-oriented mode.
- Add deterministic acquisition-strategy resolution so the system can choose between `FILE`-based acquisition and `TABLE`-based acquisition after discovery.
- Extend the file-acquisition path so it uses the slow-log file path discovered from MySQL by default instead of requiring a manually supplied remote slow-log path as the primary configuration.
- Add direct `mysql.slow_log` table ingestion for sources that expose slow logs through `TABLE`.
- Add source-discovery metadata and status exposure through storage, API, and UI so operators can distinguish discovery failures from acquisition failures and parser failures.
- Add migration-oriented configuration and onboarding documentation that explains how V3 remote file assumptions map into the new V4 source-DB-first model.

## Capabilities

### New Capabilities
- `source-mysql-discovery`: Inspect the remote MySQL source, persist discovery metadata, and determine whether slow logging is enabled and how it is exposed.
- `mysql-auto-acquisition-routing`: Resolve `mysql_auto` mode into an effective acquisition strategy, validate mode-specific prerequisites, and expose the chosen strategy explicitly.
- `discovered-file-acquisition`: Acquire slow-log data from a remote file path discovered from MySQL, preserving spool and checkpoint behavior for the `FILE` branch.
- `slow-log-table-ingestion`: Acquire slow-log data directly from `mysql.slow_log` when the source MySQL exposes `TABLE` output.
- `source-discovery-status-api`: Expose source discovery metadata, effective acquisition mode, and discovery/acquisition runtime status through backend APIs.
- `source-discovery-web-ui`: Display discovered slow-log mode, effective acquisition mode, and discovery-vs-acquisition-vs-parser health in the web UI.

### Modified Capabilities
- None.

## Impact

- Affected backend areas: configuration loading, source metadata modeling, acquisition orchestration, MySQL probing, table ingestion, checkpoint storage, runtime status storage, API handlers, and collector lifecycle code under `cmd/` and `internal/`.
- Affected frontend areas: dashboard and fingerprint pages under `web/`, which must now explain discovered slow-log mode and effective acquisition mode instead of only remote file acquisition state.
- Affected operational setup: environment variable naming, source DB credentials, SSH transport behavior for `FILE` mode, permissions for `mysql.slow_log` access in `TABLE` mode, onboarding documentation, and migration guidance from the V3 remote-path-first model.
- Dependency and migration impact: V4 reuses the current parser/fingerprint pipeline where possible, but adds source discovery state, table-mode checkpoints, and compatibility logic for V3 remote acquisition configurations.
