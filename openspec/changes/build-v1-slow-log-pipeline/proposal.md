## Why

Slow SQL Observer currently has a project-level execution plan, but it does not yet have an executable OpenSpec change that turns the V1 scope into concrete capabilities, design decisions, and implementation tasks. We need that layer now so the team can build a focused end-to-end pipeline without drifting into multi-instance collection, advanced observability features, or premature abstractions.

This change establishes the V1 contract for a runnable single-instance slow query analysis system: ingest MySQL slow logs incrementally, parse them into structured records, normalize similar SQL into stable fingerprints, aggregate metrics, persist both raw and derived data, and expose the results through APIs and a web UI.

## What Changes

- Add a V1 slow-log ingestion capability that reads one MySQL slow query log incrementally, frames complete event blocks, parses structured fields, and tracks collector checkpoints using file path, file identity, and offset.
- Add a fingerprinting and aggregation capability that normalizes common SQL parameter differences into stable templates, generates repeatable fingerprint hashes, and maintains per-fingerprint aggregate statistics.
- Add a query API capability that serves overview, fingerprint list, fingerprint detail, and sample-record endpoints for the V1 analysis data model.
- Add a web dashboard capability that presents overview metrics, ranked fingerprint results, and fingerprint-level detail views backed by the V1 APIs.
- Constrain V1 to one observed MySQL instance, one collector flow, one analysis schema, and a pragmatic log-rotation strategy that switches to a new file without tail-following the rotated file.

## Capabilities

### New Capabilities
- `slow-log-ingestion`: Incrementally collect and parse one MySQL slow query log into structured slow query records with durable checkpoints.
- `sql-fingerprinting`: Normalize SQL into stable templates, generate fingerprint identities, and maintain aggregate metrics for repeated statements.
- `slow-sql-query-api`: Provide HTTP endpoints for overview metrics, fingerprint browsing, fingerprint detail, and sample raw records.
- `slow-sql-web-ui`: Display overview, fingerprint list, and fingerprint detail pages for V1 analysis results.

### Modified Capabilities
- None.

## Impact

- Affected backend areas: collector, parser, fingerprint, aggregator, storage, config, and API modules under `cmd/` and `internal/`.
- Affected frontend area: dashboard, fingerprint list, and detail pages under `web/`.
- Affected data systems: one MySQL instance hosting both application data and a separate analysis schema for Slow SQL Observer.
- Affected operational setup: local startup flow, Docker Compose packaging, schema initialization, and sample/demo data setup.
