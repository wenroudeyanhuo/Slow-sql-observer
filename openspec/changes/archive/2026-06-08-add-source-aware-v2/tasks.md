## 1. Configuration and Source Modeling

- [x] 1.1 Add V2 source, analysis, and runtime config fields with fallback mapping from V1 environment variable names.
- [x] 1.2 Emit deprecation warnings when V1-only config names are used.
- [x] 1.3 Introduce a persisted source model keyed by source instance name and slow log path.

## 2. Storage and Runtime Status

- [x] 2.1 Add schema initialization for `sources` and `collector_status`.
- [x] 2.2 Update checkpoints and raw-record persistence to reference `source_id`.
- [x] 2.3 Add storage operations for source upsert, source metadata updates, and collector status writes.

## 3. Collector and Source Integration

- [x] 3.1 Resolve the active source before collector ingest cycles begin.
- [x] 3.2 Add optional `SSO_SOURCE_DB_DSN` probe logic for source validation and metadata enrichment.
- [x] 3.3 Persist collector status for healthy, degraded, and error outcomes during collector execution.

## 4. Raw-Record Retention

- [x] 4.1 Add `SSO_RAW_RECORD_RETENTION_DAYS` handling to runtime configuration.
- [x] 4.2 Implement collector-driven cleanup for expired `slow_query_records`.
- [x] 4.3 Ensure retention cleanup failures update collector status without breaking committed ingest results.

## 5. API Expansion

- [x] 5.1 Add a source metadata endpoint for the active source.
- [x] 5.2 Add a collector runtime status endpoint for the active source.
- [x] 5.3 Preserve compatibility of existing overview, fingerprint list, detail, and records endpoints.

## 6. Web UI Updates

- [x] 6.1 Add shared source identity and collector status display to dashboard and fingerprint pages.
- [x] 6.2 Distinguish empty-data states from degraded or failed collection states in the UI.

## 7. Documentation and Migration

- [x] 7.1 Update `.env.example` and README files to document the V2 config model and migration path from V1 names.
- [x] 7.2 Document real slow-log onboarding requirements, including file access and optional source DB probing.
- [x] 7.3 Document raw-record retention behavior and the meaning of source identity changes.

## 8. Verification

- [x] 8.1 Add tests for V2 config precedence, V1 fallback compatibility, and deprecation behavior.
- [x] 8.2 Add tests for source identity lifecycle, source DB probe behavior, and collector runtime status updates.
- [x] 8.3 Add tests for raw-record retention behavior and API/UI source-status responses.
- [x] 8.4 Run `openspec validate add-source-aware-v2` and confirm the change is apply-ready.
