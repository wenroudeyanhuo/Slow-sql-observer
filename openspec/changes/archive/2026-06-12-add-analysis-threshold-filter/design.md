## Context

Slow SQL Observer currently treats all ingested slow-log events as eligible for the main analysis views. That works for basic collection, but it becomes noisy when operators intentionally set a low MySQL `long_query_time` to preserve more raw samples. In those environments, the system needs a second threshold at the analysis layer so the main UI and ranking APIs can focus on materially slow queries without throwing away lower-latency raw records.

This change touches configuration, storage query behavior, HTTP APIs, and the web UI. The design therefore needs a clear contract for where the threshold applies and where it does not.

## Goals / Non-Goals

**Goals:**

- Introduce a configurable default analysis threshold expressed in seconds.
- Keep ingestion behavior unchanged so all collected raw records can still be stored.
- Apply the threshold consistently to overview, fingerprint list, fingerprint detail, and fingerprint records queries.
- Allow request-level overrides for investigation without changing the system default.
- Make the difference between collection threshold and analysis threshold explicit in docs and UX.

**Non-Goals:**

- Changing MySQL `slow_query_log` or `long_query_time` automatically.
- Recomputing stored fingerprint identities based on the threshold.
- Adding alerting, anomaly detection, or multi-threshold rule engines in this change.
- Introducing parser-based fingerprinting as part of this work.

## Decisions

### 1. Add a dedicated analysis-layer config with a default of 1 second

Decision:

- Add a new config such as `SSO_ANALYSIS_MIN_QUERY_TIME_SEC`.
- Default it to `1.0`.
- Treat `0` as disabled.

Rationale:

- The user request is explicitly about defining slow SQL in the product layer, for example “only include queries over 1 second in the list”.
- A dedicated config avoids overloading MySQL collection settings.
- Defaulting to `1.0` keeps the main experience opinionated and useful out of the box while still remaining adjustable.

Alternatives considered:

- Reuse MySQL `long_query_time`: rejected because collection and analysis serve different purposes.
- Hardcode `1s` with no config: rejected because some teams will want `0.5s`, `2s`, or disabled filtering.

### 2. Apply the threshold at query time, not ingestion time

Decision:

- Persist all ingested raw records as today.
- Apply the threshold only when building overview and list/detail/records responses.

Rationale:

- Query-time filtering is reversible and supports ad hoc debugging.
- Ingestion-time filtering would discard evidence that may still matter during investigations.
- This approach is safer for existing installations because it does not require replaying raw logs.

Alternatives considered:

- Filter during ingest and skip below-threshold rows: rejected because it permanently loses data.

### 3. Support request-level API overrides with `minQueryTimeSec`

Decision:

- Add an optional `minQueryTimeSec` query parameter to analysis endpoints.
- If omitted, the server uses `SSO_ANALYSIS_MIN_QUERY_TIME_SEC`.
- If provided, the request value overrides the default for that call.

Rationale:

- Operators need a stable default for daily use, but also need quick temporary overrides.
- Query parameters are easy for both UI and external API consumers to use.

Alternatives considered:

- UI-only client-side filtering: rejected because the backend aggregates would still be noisy and inconsistent.
- A global mutable runtime setting via API: rejected as too heavy for this scope.

### 4. Keep fingerprint identity stable; filter counts and rankings, not identity

Decision:

- Existing fingerprint IDs and normalized SQL entries remain unchanged.
- Threshold filtering affects which records and aggregate slices participate in overview and ranking results.

Rationale:

- The threshold is an analysis lens, not a data model boundary.
- Rebuilding fingerprint identity around thresholds would add unnecessary migration complexity.

Alternatives considered:

- Maintain separate “slow-enough fingerprint tables” per threshold: rejected as over-engineered.

### 5. Show the effective threshold in the UI and documentation

Decision:

- The web UI should expose the active threshold control.
- Documentation should explain the distinction between:
  - MySQL collection threshold
  - Slow SQL Observer analysis threshold

Rationale:

- Otherwise users will assume the product is changing what MySQL collects.
- Visibility reduces confusion when records exist in storage but do not show in the default list.

## Risks / Trade-offs

- [Risk] Overview and list queries become more complex because they must filter by query time across aggregates. → Mitigation: keep the threshold logic centralized in storage query builders and reuse the same parameter handling across endpoints.
- [Risk] Users may be confused when a raw record exists but does not appear in the default ranking views. → Mitigation: document the threshold clearly and expose it in the UI/API.
- [Risk] Fingerprint detail behavior may feel inconsistent if some views are filtered and others are not. → Mitigation: apply the same threshold semantics to overview, list, detail, and records endpoints whenever a threshold is active.
- [Risk] Existing dashboards may look “emptier” after the default filter is introduced. → Mitigation: use a request-level override and document how to lower or disable the default threshold.

## Migration Plan

1. Add the new config to `.env.example`, config loading, and runtime wiring.
2. Extend list/detail/overview/records query parameter parsing with `minQueryTimeSec`.
3. Update storage queries to apply the effective threshold.
4. Update the web UI to send and display the active threshold.
5. Update README and API docs.
6. Validate with both:
   - default `1.0` threshold
   - explicit override such as `0.2`

Rollback strategy:

- Set `SSO_ANALYSIS_MIN_QUERY_TIME_SEC=0` to disable the feature behavior without removing data.
- Revert the code if needed; no persisted data migration is required because raw records remain unchanged.

## Open Questions

- Should the fingerprint detail response surface both filtered and unfiltered counts in the same payload, or keep only the filtered view for simplicity?
- Should the UI remember the user’s last threshold locally, or only use the server default plus current-page overrides?
