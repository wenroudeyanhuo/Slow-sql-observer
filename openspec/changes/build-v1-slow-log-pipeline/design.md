## Context

Slow SQL Observer has a clear V1 execution plan, but the implementation still needs a concrete technical design that turns the plan into buildable decisions. The repository is currently at an early bootstrap stage, so this design must optimize for a working end-to-end pipeline rather than for extensibility-first architecture.

The V1 system is intentionally narrow:

- one observed MySQL instance
- one slow query log source
- one collector flow
- one backend API service
- one web UI
- one MySQL instance hosting an isolated analysis schema

The most important design pressure is reliability of the ingestion path. If collector framing, parser behavior, checkpoint semantics, and aggregation writes are vague, the system will either duplicate records, miss partially written events, or produce unstable fingerprint statistics.

## Goals / Non-Goals

**Goals:**

- Build a runnable V1 pipeline from MySQL slow query log ingestion to web presentation.
- Persist both raw slow query records and fingerprint-level aggregate read models.
- Keep fingerprinting conservative and deterministic so common parameterized SQL collapses into stable templates without aggressive semantic merging.
- Make collector progress durable and restart-safe using file path, file identity, and completed-event offsets.
- Expose a minimal but useful API and UI surface: overview, fingerprint list, fingerprint detail, and sample records.

**Non-Goals:**

- Multi-instance collection, distributed coordination, or host-level deployment orchestration.
- `performance_schema`, endpoint tracing, trace correlation, or OpenTelemetry ingestion.
- Industrial-strength SQL semantic parsing or equivalence rewriting.
- Full log rotation tail-draining across old and new files.
- Advanced analytics storage, including ClickHouse or time-series optimizations.

## Decisions

### 1. Keep V1 single-instance and single-schema-targeted

The system will observe one MySQL instance and ingest one slow query log source. Analysis data will be stored in a dedicated schema on the same MySQL instance rather than introducing a separate database deployment for V1.

Why this decision:

- It matches the execution plan and avoids infrastructure overhead during validation.
- It keeps operational setup simple for local runs and Docker Compose demos.
- It still provides clean separation from business tables by using a dedicated analysis schema.

Alternatives considered:

- Separate MySQL deployment for analysis: rejected for V1 because it adds operational complexity without improving the core product proof.
- Same business schema with prefixed tables: rejected because schema-level separation is cleaner for migration, permissions, and cleanup.

### 2. Split collector and parser responsibilities clearly

Collector and parser will be separate modules with distinct responsibilities:

- Collector reads the slow log incrementally, maintains a rolling buffer, frames complete event blocks, and advances checkpoints only after successful persistence.
- Parser receives a complete event block and extracts structured fields into a `SlowQueryRecord`.

Why this decision:

- It isolates stream-handling concerns from format-parsing concerns.
- It keeps partially written trailing data out of the parser.
- It makes restart behavior and parser testing easier.

Alternatives considered:

- Parse directly from the stream in one module: rejected because it couples file-offset logic and parsing semantics too tightly.

### 3. Use completed-event checkpoint semantics

Checkpoint state will include:

- `instance_name`
- `log_file_path`
- `file_identity`
- `last_offset`
- `updated_at`

`last_offset` represents the end of the last fully processed event block, not simply the farthest byte ever read.

Why this decision:

- It prevents partially written trailing blocks from being acknowledged too early.
- It supports safe restart behavior after process failure.
- It aligns checkpoint advancement with successful transaction completion.

Alternatives considered:

- Store only `path + offset`: rejected because it is too fragile for truncate and rotate scenarios.
- Store read-to-end offsets regardless of parsing/persistence status: rejected because it can skip data after failures.

### 4. Handle file rotation pragmatically in V1

Collector will compare checkpointed `file_identity` with the current file's identity:

- same identity and file size at or beyond `last_offset`: resume from `last_offset`
- same identity and smaller size than `last_offset`: treat as truncate and restart from `0`
- different identity: treat as a new file and start from `0`

V1 will not attempt to drain the tail of the rotated-out file before switching to the new file.

Why this decision:

- It handles the most common operational cases with low complexity.
- It keeps V1 focused on stable end-to-end behavior instead of complex rotation state machines.

Alternatives considered:

- Tail-drain old file after rotation: rejected for V1 because it adds lifecycle complexity disproportionate to the product goal.

### 5. Use a conservative rule-based fingerprinting strategy

Fingerprint generation will normalize common parameter variation while avoiding aggressive semantic rewrites. V1 normalization will:

- remove comments
- compress whitespace
- ignore trailing semicolon differences
- normalize numeric, string, datetime, and hexadecimal literals to placeholders
- collapse `IN (...)` parameter count differences
- collapse batch `VALUES (...)` count differences
- normalize numeric `LIMIT` and `OFFSET` variants

V1 will not reorder predicates, infer semantic equivalence, or rewrite structurally different SQL into the same fingerprint.

Why this decision:

- It is sufficient to prove the aggregation pipeline works.
- It minimizes false merges that would corrupt analysis accuracy.
- It remains explainable and easy to test using representative SQL examples.

Alternatives considered:

- AST-level SQL parser from day one: rejected for V1 due to complexity and limited incremental value.
- More aggressive equivalence rules: rejected because incorrect merging is more harmful than conservative splitting.

### 6. Persist raw facts and aggregate read models separately

The analysis schema will include four core tables:

- `collector_checkpoints`
- `slow_query_records`
- `fingerprints`
- `fingerprint_stats`

`slow_query_records` stores source-location metadata, raw block content, parsed SQL text, fingerprint reference, and performance metrics. `fingerprints` stores identity and template metadata. `fingerprint_stats` stores aggregate counters and summary metrics.

Why this decision:

- It preserves raw evidence for debugging and record-level inspection.
- It supports efficient list/detail APIs without recomputing aggregates on each request.
- It enables idempotency protections on record ingestion using source file identity and block offsets.

Alternatives considered:

- Compute all aggregates at query time: rejected because it complicates V1 performance and API implementation.
- Store only raw events and infer fingerprints lazily: rejected because it weakens deterministic ingestion semantics.

### 7. Use a transactional ingest write path

For each successfully parsed event block, the system will complete the following work in one database transaction:

1. insert the raw slow query record
2. upsert the fingerprint metadata
3. upsert fingerprint aggregate stats
4. update the collector checkpoint

Why this decision:

- It keeps raw facts, aggregate read models, and checkpoint state consistent.
- It prevents checkpoint advancement without corresponding data persistence.

Alternatives considered:

- Separate asynchronous aggregation: rejected for V1 because it introduces failure windows and orchestration complexity.

### 8. Keep the API/UI surface minimal and list-driven

V1 will expose:

- `GET /api/dashboard/overview`
- `GET /api/slow-sql/fingerprints`
- `GET /api/slow-sql/fingerprints/:id`
- `GET /api/slow-sql/fingerprints/:id/records`

The web UI will provide:

- dashboard overview page
- fingerprint ranking/list page
- fingerprint detail page with sample records

Why this decision:

- These views cover the core user value of identifying slow SQL patterns and drilling into examples.
- They avoid premature investment in trends, alerting, or advanced analytics visualizations.

Alternatives considered:

- Broader dashboard scope with trends and comparisons: rejected for V1 because it expands storage and query complexity.

## Risks / Trade-offs

- [Conservative fingerprinting may under-merge similar SQL] -> Start with representative normalization cases and prefer false splits over false merges.
- [Pragmatic rotation handling may miss a small rotated-file tail] -> Document the limitation explicitly in V1 and keep the collector simple.
- [Same-instance analysis storage can compete with business workload] -> Use a separate analysis schema and keep V1 ingestion volume limited to local/demo scope.
- [Slow log format variations can cause partial parsing gaps] -> Treat only time, query time, and SQL body as required; allow non-core fields to be nullable.
- [Synchronous transactional aggregation can limit peak ingest throughput] -> Accept this trade-off in V1 to preserve consistency and implementation simplicity.

## Migration Plan

1. Initialize the analysis schema and core tables in the target MySQL instance.
2. Configure the collector with one instance name and one slow log file path.
3. Run the collector against a sample slow log file and verify records, fingerprints, stats, and checkpoints are created together.
4. Start the API service and validate overview, list, detail, and sample-record endpoints.
5. Connect the web UI to the V1 APIs and verify the three core pages render useful data.
6. Package the local demo flow with Docker Compose, sample data, and documented startup steps.

Rollback strategy:

- Stop the collector and API services.
- Drop or truncate the analysis schema objects created for the feature.
- Restore the prior local/demo setup if needed.

## Open Questions

- Which Go library, if any, should be used for SQL normalization helpers before introducing heavier parsing dependencies?
- Should dashboard overview metrics be computed purely from `fingerprint_stats`, or should some values also reference raw records for validation?
- What volume of sample record retention is acceptable for local V1 demos before storage cleanup or retention policies are needed?
