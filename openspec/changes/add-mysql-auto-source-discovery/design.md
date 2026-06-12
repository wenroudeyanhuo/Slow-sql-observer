## Context

Slow SQL Observer V3 introduced source-aware remote acquisition, but its remote model is still host-path-first: the operator is expected to know and supply the remote slow-log file path up front. That approach works for controlled environments, but it does not match the real product boundary of "connect to a MySQL source and observe its slow-query output."

V4 must preserve the current single-source scope, reuse the existing parser/fingerprint/storage pipeline where practical, and add a source-discovery stage that determines how slow-query data should be acquired. The design challenge is to introduce `mysql_auto` as the preferred production-oriented mode without breaking local-file usage or destabilizing the existing V3 file-spool architecture.

## Goals / Non-Goals

**Goals:**

- Make the source MySQL connection the authoritative remote-source entrypoint in production-oriented mode.
- Add a discovery stage that inspects slow-query logging state, `log_output`, and the configured slow-log file path when relevant.
- Introduce a deterministic acquisition strategy resolver for `FILE` versus `TABLE` outputs.
- Reuse the existing V3 remote file acquisition path for the `FILE` branch, but feed it with the file path discovered from MySQL rather than a required manual path.
- Add direct `mysql.slow_log` table ingestion for the `TABLE` branch.
- Persist discovery metadata, effective acquisition mode, and stage-specific runtime status so discovery, acquisition, and parser failures are distinguishable.
- Preserve `local_file` mode for local development and mounted-log deployments.

**Non-Goals:**

- Multi-source orchestration or many-source scheduling.
- Automatically enabling or modifying source-side MySQL slow-query settings.
- Adding non-MySQL source types or replacing the parser/fingerprint design.
- Running `FILE` and `TABLE` ingestion in parallel for one source in V4.
- Building an agent-based architecture or introducing source-host daemons.

## Decisions

### 1. Replace the remote-path-first mental model with `mysql_auto`

V4 will introduce `SSO_SOURCE_MODE` with two supported values:

- `local_file`
- `mysql_auto`

`mysql_auto` becomes the preferred production-oriented mode. In that mode, `SSO_SOURCE_DB_DSN` is required and becomes the primary remote source contract.

Why:

- It aligns the product with what operators actually know first: the MySQL source, not necessarily the filesystem path.
- It lets the product inspect the source runtime state before choosing an acquisition transport.

Alternatives considered:

- Keep `ssh_pull` as the primary remote mode and only enrich it with metadata: rejected because it preserves the wrong product entrypoint.
- Remove local-file mode entirely: rejected because local development and mounted-log setups remain valid.

### 2. Introduce a first-class source-discovery stage

Before any remote acquisition begins in `mysql_auto`, the collector will connect to the source MySQL and inspect at least:

- whether slow query logging is enabled
- `log_output`
- `slow_query_log_file`
- source host/version metadata

The system will persist the discovery result and expose it independently from acquisition and parser status.

Why:

- Discovery is now a distinct runtime stage with its own failure modes.
- Operators need to know whether the system failed to discover the source configuration, failed to acquire data, or failed downstream.

Alternatives considered:

- Fold discovery into acquisition without separate persistence: rejected because it makes failure diagnosis too opaque.

### 3. Resolve `FILE` versus `TABLE` into one effective acquisition mode

V4 will introduce a strategy resolver that translates discovery output into one effective acquisition mode per source cycle.

Planned effective modes:

- `local_file`
- `mysql_file`
- `mysql_table`

Resolution rules:

- if source discovery shows slow query logging disabled, the system enters a blocked discovery/acquisition state
- if `log_output` resolves to `FILE`, use `mysql_file`
- if `log_output` resolves to `TABLE`, use `mysql_table`
- if `log_output` resolves to both `FILE` and `TABLE`, default to `mysql_file`

Why:

- A deterministic choice avoids ambiguous double-ingestion or split-brain checkpointing.
- Defaulting to `FILE` preserves more of the current V3 implementation and generally offers more predictable append-only semantics.

Alternatives considered:

- Prefer `TABLE` when both are present: rejected because the current code and operational assumptions already align more naturally with file-based incrementality.
- Run both branches in parallel: rejected because it would complicate deduplication, checkpoints, and product semantics beyond V4 scope.

### 4. Keep V3 spool behavior for the discovered `FILE` branch

For `mysql_file`, V4 will continue to use the V3 pattern:

`discovered remote file -> SSH pull -> local spool -> parser`

The important change is that the default remote slow-log path now comes from MySQL discovery instead of being a required primary configuration input. `SSO_SOURCE_REMOTE_SLOW_LOG_PATH` becomes an optional override or fallback only.

Why:

- It preserves the proven V3 parser input model.
- It minimizes implementation risk while correcting the remote-source contract.

Alternatives considered:

- Replace spooling with direct streaming: rejected because the V3 spool/checkpoint boundary is already in place and remains valuable.

### 5. Add a separate table-ingestion path with its own checkpoint model

For `mysql_table`, V4 will ingest directly from `mysql.slow_log` and map table rows into the existing downstream record model. This branch will use a dedicated table checkpoint, not the file checkpoint or parser spool checkpoint.

The exact checkpoint tuple may depend on implementation details, but it must support deterministic forward progress with minimal duplication and must be documented in the design/specs.

Why:

- Table ingestion has fundamentally different resume coordinates than file-based acquisition.
- Separate checkpoint state prevents file-mode assumptions from leaking into table mode.

Alternatives considered:

- Reuse file checkpoints for table ingestion: rejected because file offsets and table rows are different state models.
- Force table-mode users to export rows into a file first: rejected because it defeats the purpose of supporting `TABLE` output directly.

### 6. Split runtime health into discovery, acquisition, and parser layers

V4 will expose three distinct operational layers:

- source discovery health
- acquisition health
- parser/ingest health

The existing collector status remains parser/ingest-oriented. A new discovery status model and the evolved acquisition status model will show where failure occurred.

Why:

- V4 introduces one more meaningful stage before acquisition, so a two-state model is no longer enough.
- Operators need precise fault isolation.

Alternatives considered:

- Merge all status into one expanded collector_status payload: rejected because it produces ambiguous operator signals.

### 7. Keep V3 compatibility for one transition cycle

V4 will accept V3-era remote configuration where practical, but it will:

- warn when the configuration still relies on remote-path-first assumptions
- prefer `mysql_auto` in documentation and examples
- treat `SSO_SOURCE_REMOTE_SLOW_LOG_PATH` as an optional override, not the primary required remote input

Why:

- Existing V3 users need a migration path without abrupt breakage.

Alternatives considered:

- Hard-break V3 remote configurations immediately: rejected because it would create unnecessary migration pain.

## Risks / Trade-offs

- [Discovery succeeds but file transport cannot reach the discovered path] -> Surface discovery success and acquisition failure separately, with explicit operator messaging.
- [TABLE mode resume semantics may be tricky] -> Require a documented deterministic checkpoint tuple and cover it with tests before implementation is considered complete.
- [Some environments expose both FILE and TABLE and operators may expect both] -> Choose one effective mode deterministically and expose that choice in API/UI.
- [Source DB credentials may not have permission to read `mysql.slow_log`] -> Validate permissions explicitly and surface a blocked or degraded table-mode state with actionable errors.
- [V3 users may continue to think the remote path is mandatory] -> Rewrite docs and config examples around `mysql_auto` and mark the old assumption as deprecated.
- [More runtime stages increase operator complexity] -> Show the stages clearly in API/UI instead of collapsing them into one health banner.

## Migration Plan

1. Introduce `SSO_SOURCE_MODE` and `mysql_auto` while preserving `local_file`.
2. Add source discovery models, storage tables or columns, and runtime status support.
3. Refactor the V3 file-acquisition path so it consumes a discovered remote file path by default.
4. Add the table-ingestion branch and its checkpoint model.
5. Extend APIs and UI for discovery metadata and effective acquisition mode.
6. Update configuration templates and documentation to make source-DB-first onboarding the default guidance.

Rollback strategy:

- switch production setups back to `local_file` or the V3-compatible remote-path-first configuration if needed
- ignore discovery-specific tables or fields when rolling back to V3 behavior
- keep the parser/fingerprint/storage pipeline stable so rollback scope stays focused on source acquisition

## Open Questions

- The exact table-ingestion checkpoint tuple remains an implementation detail to finalize during coding, but the chosen tuple must support deterministic resume and be documented explicitly.
