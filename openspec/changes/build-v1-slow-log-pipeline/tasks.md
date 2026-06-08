## 1. Bootstrap and project wiring

- [x] 1.1 Create the backend and frontend V1 skeleton under `cmd/`, `internal/`, and `web/` with startup entry points for collector and API services.
- [x] 1.2 Add configuration loading for one observed MySQL instance, one slow log path, and one analysis schema.
- [x] 1.3 Add local development wiring for the backend, frontend, and analysis MySQL instance.

## 2. Analysis schema and persistence

- [x] 2.1 Create schema initialization for `collector_checkpoints`, `slow_query_records`, `fingerprints`, and `fingerprint_stats`.
- [x] 2.2 Implement repository methods for checkpoint reads and writes using path, file identity, and completed-event offsets.
- [x] 2.3 Implement repository methods for raw slow query records, fingerprint upserts, and fingerprint stats upserts.
- [x] 2.4 Add transactional ingest persistence that writes record, fingerprint, aggregate stats, and checkpoint state together.

## 3. Slow log ingestion and parsing

- [x] 3.1 Implement incremental slow log reading with buffered framing based on complete `# Time:` event blocks.
- [x] 3.2 Implement incomplete trailing block retention so partially written events are not parsed or checkpointed.
- [x] 3.3 Implement parsing of slow log metadata into structured record fields including time, user, host, query metrics, and SQL body.
- [x] 3.4 Implement file identity handling for normal resume, truncate restart, and rotated-file restart behavior.

## 4. Fingerprinting and aggregation

- [x] 4.1 Implement conservative SQL normalization for comments, whitespace, literals, `IN (...)`, batch `VALUES (...)`, and `LIMIT/OFFSET`.
- [x] 4.2 Implement stable fingerprint hash generation and fingerprint metadata extraction for SQL type and main table heuristics.
- [x] 4.3 Implement aggregate metric updates for count, timing, rows sent, rows examined, first seen, and last seen values.
- [x] 4.4 Add tests that prove equivalent parameterized SQL maps to the same fingerprint while structurally different SQL remains separate.

## 5. Query API

- [x] 5.1 Implement the dashboard overview endpoint with summary metrics and top fingerprints.
- [x] 5.2 Implement the fingerprint list endpoint with pagination, supported sorting, and supported filtering.
- [x] 5.3 Implement the fingerprint detail endpoint for normalized SQL, metadata, and aggregate statistics.
- [x] 5.4 Implement the fingerprint sample-record endpoint with pagination and supported sorting for raw records.

## 6. Web UI

- [x] 6.1 Implement the dashboard overview page and empty-state behavior backed by the overview API.
- [x] 6.2 Implement the fingerprint list page with API-driven paging, sorting, and filtering controls.
- [x] 6.3 Implement the fingerprint detail page with aggregate metrics, metadata, and sample raw record display.

## 7. Verification and packaging

- [x] 7.1 Add representative parser and collector tests covering standard blocks, missing non-core fields, incomplete trailing blocks, and rotated files.
- [x] 7.2 Add end-to-end verification that sample slow logs produce raw records, fingerprints, aggregate stats, and queryable API responses.
- [x] 7.3 Add Docker Compose, sample/demo data, and documented local startup steps for the V1 pipeline.
