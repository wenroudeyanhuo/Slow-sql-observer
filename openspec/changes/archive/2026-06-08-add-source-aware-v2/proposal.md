## Why

Slow SQL Observer V1 proves that the project can ingest a slow query log and present analysis results, but it still behaves like a local demo. The next version needs a clearer operational model that separates the observed source from the analysis storage, exposes runtime state, and supports long-running use against a real MySQL slow log.

V2 is needed now because the current implementation shape is good enough to evolve incrementally, but its configuration, source identity, and retention model are still too implicit for a production-like setup. This change turns the V2 execution plan into an executable OpenSpec contract before more implementation work begins.

## What Changes

- Add a source-aware ingestion capability that models one observed MySQL source explicitly, separates source configuration from analysis database configuration, and preserves single-source slow-log-file ingestion as the primary acquisition path.
- Add a collector runtime status capability that persists and exposes source metadata, source reachability, last successful ingest, checkpoint position, and latest collector error state.
- Add a raw-record retention capability that allows configurable expiration of `slow_query_records` while keeping fingerprint metadata and aggregate statistics available by default.
- Extend the query API capability so clients can read current source identity and collector runtime status in addition to existing overview, fingerprint list, fingerprint detail, and sample-record data.
- Extend the web UI capability so the dashboard and drill-down pages display source context, collector status, and clearer empty/error-state messaging for a real slow-log deployment.
- Introduce a V2 configuration migration path with new preferred environment variable names for source, analysis, and runtime groups, while accepting V1 names for one compatibility cycle with deprecation warnings.

## Capabilities

### New Capabilities
- `source-aware-ingestion`: Model one observed MySQL source explicitly, separate source and analysis configuration, and enrich source metadata when an optional source DB connection is available.
- `collector-runtime-status`: Persist and expose collector state, source accessibility, checkpoint progress, and latest ingest error information for the active source.
- `raw-record-retention`: Apply configurable retention to raw slow query records during collector operation without removing fingerprint metadata or aggregate stats by default.
- `source-aware-query-api`: Provide HTTP endpoints for current source metadata and collector runtime status while keeping existing analysis endpoints compatible.
- `source-aware-web-ui`: Display current source identity, collector status, and improved empty/error-state messaging throughout the dashboard and fingerprint views.

### Modified Capabilities
- None.

## Impact

- Affected backend areas: configuration loading, collector lifecycle, source metadata handling, storage schema, status persistence, retention cleanup, and API handlers under `cmd/` and `internal/`.
- Affected frontend area: dashboard, fingerprint list, and fingerprint detail pages under `web/`.
- Affected data systems: one analysis MySQL schema that must now store source metadata, collector runtime status, and retention-aware raw records in addition to the existing V1 tables.
- Affected operational setup: environment variable naming, source deployment guidance, real slow-log onboarding, and migration documentation from V1 naming conventions.
