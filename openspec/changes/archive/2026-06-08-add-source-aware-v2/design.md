## Context

Slow SQL Observer already has a runnable V1 pipeline: one collector reads one MySQL slow query log file, parses records, derives SQL fingerprints, aggregates metrics, persists results in a MySQL analysis schema, and exposes the data through a small API and web UI. That baseline is useful, but its current operational model is still demo-oriented because source identity, storage role separation, runtime status, and raw-record lifecycle are mostly implicit.

V2 keeps the existing end-to-end flow intact and evolves it incrementally. The design must preserve the current single-source scope, keep slow-log-file ingestion as the primary acquisition path, and avoid turning this iteration into a broader observability platform. The major design pressure is to make the system understandable and operable in a real slow-log deployment without introducing multi-instance orchestration or a second ingestion model.

## Goals / Non-Goals

**Goals:**

- Separate configuration into source, analysis, and runtime groups while preserving one compatibility cycle for V1 names.
- Introduce an explicit source identity model and persist source metadata independently from raw slow-query records.
- Persist collector runtime state, source accessibility, last success state, and latest error state in a queryable form.
- Keep slow-log-file ingestion as the primary pipeline while optionally enriching source metadata through `SSO_SOURCE_DB_DSN`.
- Add configurable raw-record retention that runs automatically in the collector loop and leaves fingerprint metadata and aggregate stats intact.
- Extend the API and web UI so operators can see which source they are inspecting and whether collection is healthy.

**Non-Goals:**

- Multi-source scheduling, source fleet management, or any UI for switching between many active sources.
- `performance_schema`, protocol interception, or any new primary ingestion channel beyond slow-log-file reading.
- Replacing the current parser or fingerprint strategy with a new semantic SQL analysis engine.
- Cross-host log shipping, remote agent coordination, or source-side automation of MySQL slow-log settings.

## Decisions

### 1. Split configuration into source, analysis, and runtime groups

V2 will introduce these preferred configuration groups:

- Source: `SSO_SOURCE_INSTANCE_NAME`, `SSO_SOURCE_SLOW_LOG_PATH`, `SSO_SOURCE_DB_DSN`, `SSO_SOURCE_TIMEZONE`, `SSO_SOURCE_DESCRIPTION`
- Analysis: `SSO_ANALYSIS_DB_DSN`, `SSO_ANALYSIS_DB_SCHEMA`
- Runtime: `SSO_SERVER_ADDR`, `SSO_WEB_DIR`, `SSO_COLLECTOR_POLL_INTERVAL`, `SSO_RAW_RECORD_RETENTION_DAYS`, `SSO_LOG_LEVEL`

V1 names remain valid for one compatibility cycle only as fallback mappings:

- `SSO_INSTANCE_NAME` -> `SSO_SOURCE_INSTANCE_NAME`
- `SSO_SLOW_LOG_PATH` -> `SSO_SOURCE_SLOW_LOG_PATH`
- `SSO_DB_DSN` -> `SSO_ANALYSIS_DB_DSN`
- `SSO_DB_SCHEMA` -> `SSO_ANALYSIS_DB_SCHEMA`

If both new and old names are present, the new names win and the process emits deprecation warnings for the old names.

Why this decision:

- It makes source and storage roles explicit without breaking all existing local setups immediately.
- It keeps V2 migration simple for current users while giving new documentation a cleaner model.

Alternatives considered:

- Hard-cut to new names only: rejected because it creates needless upgrade friction for the first V2 release.
- Keep V1 naming permanently: rejected because it leaves the source/storage model ambiguous.

### 2. Treat source identity as a first-class persisted model

The system will add a `sources` table with a surrogate `id` and a unique `source_key`. `source_key` is derived from the normalized pair `(source_instance_name, source_slow_log_path)`. A change to either field creates a new source identity.

The `sources` record stores:

- `source_key`
- `source_instance_name`
- `source_slow_log_path`
- `source_description`
- `source_db_dsn_configured`
- optional probed metadata such as source host and source version
- timestamps

All new runtime state, checkpoints, and ingested records will reference `source_id` instead of relying only on repeated instance/path strings.

Why this decision:

- It makes "which source does this data belong to?" explicit across storage, API, and UI.
- It avoids accidental mixing of state when a user repoints the collector to another slow log path.

Alternatives considered:

- Keep only strings on every record with no persisted source model: rejected because it keeps source identity implicit and makes V2 status APIs awkward.
- Always overwrite a single source row: rejected because configuration identity changes should start a new source lineage.

### 3. Keep slow-log-file ingestion primary and make source DB probing optional

`SSO_SOURCE_DB_DSN` is optional. When absent, the collector still performs normal slow-log-file ingestion. When present, the collector attempts a lightweight source probe to:

- confirm source DB connectivity
- collect source metadata such as version and host when available
- surface probe failures in runtime status

Source DB probe failure does not disable the primary ingestion path if the slow log file remains accessible. Instead, the collector marks source metadata/status as degraded while continuing to ingest from the file.

Why this decision:

- It preserves the agreed product boundary that slow-log-file reading is the primary acquisition mechanism.
- It gives operators richer source context without turning V2 into a database-query collector.

Alternatives considered:

- Require `SSO_SOURCE_DB_DSN` for all V2 runs: rejected because file-only deployments must remain supported.
- Ignore `SSO_SOURCE_DB_DSN` entirely in V2: rejected because the user explicitly wants source DB alignment and metadata support.

### 4. Persist runtime state separately from checkpoints

V2 will add a `collector_status` table keyed by `source_id`. It is separate from `collector_checkpoints`.

`collector_status` stores at least:

- `source_id`
- `collector_state`
- `source_access_state`
- `last_successful_ingest_at`
- `last_checkpoint_offset`
- `last_file_identity`
- `last_error_at`
- `last_error_message`
- `updated_at`

`collector_checkpoints` remains the durable completed-event resume mechanism, while `collector_status` becomes the operational read model for APIs and UI.

Why this decision:

- Checkpoints and operator-facing health state serve different purposes and change at different cadences.
- It avoids overloading checkpoint rows with transient health semantics.

Alternatives considered:

- Add status fields into `collector_checkpoints`: rejected because checkpoint semantics should remain focused on resume state.

### 5. Run retention cleanup inside the collector loop

The collector loop will perform raw-record retention cleanup after normal ingest work in the same runtime process. `SSO_RAW_RECORD_RETENTION_DAYS <= 0` disables cleanup. Positive values delete expired rows from `slow_query_records` based on record time while leaving fingerprints and aggregate stats untouched.

Retention cleanup is best-effort and independent from the per-event ingest transaction. Cleanup failures update collector status error fields but do not roll back already-ingested records from the current cycle.

Why this decision:

- The collector already owns ingest cadence and source-specific lifecycle work.
- It avoids adding a second background scheduler for a single-source V2.

Alternatives considered:

- Run cleanup in the API server: rejected because retention is collector-owned operational work.
- Manual cleanup only: rejected because V2 must support long-running use by default.

### 6. Add source/status APIs without changing existing analysis routes

V2 will keep the current analysis routes stable:

- `GET /api/dashboard/overview`
- `GET /api/slow-sql/fingerprints`
- `GET /api/slow-sql/fingerprints/:id`
- `GET /api/slow-sql/fingerprints/:id/records`

V2 adds:

- `GET /api/source`
- `GET /api/collector/status`

These routes return the active source metadata and collector runtime state for the single source deployment. Existing analysis responses stay compatible; source/status context is retrieved via the new endpoints.

Why this decision:

- It avoids a breaking API migration for the V1 analysis pages and tests.
- It keeps source/status concerns explicit instead of overloading existing payloads.

Alternatives considered:

- Inject source/status into every existing endpoint payload: rejected because it creates unnecessary payload churn and complicates compatibility.

### 7. Surface source context and health on every page

The UI will add a shared source-status summary that appears on the dashboard and fingerprint pages. It will display:

- source instance name
- slow log path
- source DB metadata when available
- collector state
- source accessibility state
- last successful ingest time
- latest error summary when degraded or failed

The UI must distinguish among:

- no data yet
- collector healthy but no records
- source or collector degraded/error

Why this decision:

- The current UI can show empty lists without explaining whether collection is healthy, misconfigured, or simply not yet producing slow queries.

Alternatives considered:

- Show source status only on the dashboard: rejected because list/detail pages also need operational context in a single-source product.

## Risks / Trade-offs

- [Optional source DB probing can create status noise when credentials are wrong] -> Treat probe failure as degraded status, not as a hard stop for slow-log ingestion.
- [New source identity records can fragment history after config changes] -> Document clearly that changing instance name or slow-log path creates a new source lineage by design.
- [Retention cleanup can temporarily increase collector cycle duration] -> Keep cleanup scoped to raw records only and make it configurable/disable-able.
- [Maintaining V1 compatibility names increases config-loader complexity] -> Keep the mapping explicit and limit it to one compatibility cycle with warnings.
- [Separate source/status tables add schema migration work] -> Reuse the current single analysis schema and keep the new tables narrowly focused on V2 operational needs.

## Migration Plan

1. Extend configuration loading to support new source and analysis names, plus compatibility fallback from V1 names.
2. Add schema initialization for `sources` and `collector_status`, and adapt checkpoint/raw-record writes to resolve `source_id`.
3. Update the collector startup flow to upsert the active source, optionally probe `SSO_SOURCE_DB_DSN`, and persist collector status on each cycle.
4. Add retention cleanup to the collector loop and expose the resulting status through the storage/API layers.
5. Add `/api/source` and `/api/collector/status` while keeping the existing analysis endpoints intact.
6. Update the web UI and docs to use the new source-aware runtime model and migration guidance.

Rollback strategy:

- Stop the V2 collector and API processes.
- Revert to V1 configuration names and V1 binary behavior if needed.
- Ignore or drop the new `sources` and `collector_status` tables if rolling back fully to V1.

## Open Questions

- No open product decisions remain for the proposal scope. Implementation may still decide the exact SQL used to probe MySQL metadata, but that choice does not change the V2 contract.
