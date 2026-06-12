## Context

Slow SQL Observer currently exposes snapshot-style analysis views: overview metrics, fingerprint rankings, fingerprint detail, and raw records. Operators can see what is heavy right now, but they cannot see whether slow-SQL volume or total query time is trending up or down across recent days or hours.

This change adds a lightweight time-series analytics layer on top of the existing single-source model. The design needs to preserve the current runtime shape, reuse the existing analysis database, and keep the same analysis-threshold semantics that were already introduced for overview and ranking views.

## Goals / Non-Goals

**Goals:**
- Add dashboard-level time-series analytics for recent slow-SQL activity.
- Add fingerprint-level time-series analytics for one normalized SQL over time.
- Reuse existing filters such as `minQueryTimeSec`, and support optional `dbName` filtering where it makes sense.
- Add UI trend panels that make daily or hourly changes visible without requiring operators to query the API manually.
- Keep the change compatible with the current single-source release and current storage model.

**Non-Goals:**
- Multi-source trend comparison.
- Advanced BI features such as arbitrary dimensions, saved reports, CSV export, or ad hoc query builders.
- Precomputed OLAP tables or background rollup pipelines in this first version.
- Alerting or anomaly-detection logic.

## Decisions

### 1. Add explicit trend endpoints instead of overloading overview responses

The API will add dedicated trend endpoints rather than embedding time-series arrays into the existing overview and fingerprint detail responses.

Planned interfaces:
- `GET /api/dashboard/trends`
- `GET /api/slow-sql/fingerprints/:id/trends`

Rationale:
- Keeps existing clients stable.
- Avoids making overview/detail payloads much heavier by default.
- Lets trend queries evolve independently from snapshot queries.

Alternative considered:
- Extend `/api/dashboard/overview` and `/api/slow-sql/fingerprints/:id` directly.
  Rejected because it couples snapshot and trend concerns and complicates caching and UI load behavior.

### 2. Use grouped query-time aggregation over existing raw records

Trend data will be computed from `slow_query_records` at request time using grouped bucket queries over `occurred_at`.

Rationale:
- No new ingestion pipeline is required.
- Preserves the existing rule that analysis filtering happens at query time, not ingest time.
- Keeps the first version simpler and easier to validate.

Alternative considered:
- Persist daily rollup tables during collection.
  Rejected for now because it introduces more write-path complexity, retention coupling, and migration overhead before trend usage is proven.

### 3. Limit initial bucket model to operator-friendly recent windows

The first version will support a constrained bucket model such as:
- `bucket=day`
- `bucket=hour`

And a bounded lookback control such as:
- `days=7`
- `days=30`

The exact parameter shape may be implemented as `days` plus `bucket`, or a semantically equivalent bounded recent-range contract, but it must remain simple and documented.

Rationale:
- Covers the main operator question: "What changed over the last few days or hours?"
- Avoids a large time-range API surface in the first iteration.

Alternative considered:
- Arbitrary `from` / `to` ranges with many bucket granularities.
  Deferred because it increases validation, timezone, and query-planning complexity.

### 4. Keep threshold semantics consistent with current analysis views

Trend endpoints must honor the same effective analysis threshold resolution model:
- request `minQueryTimeSec` override if provided
- otherwise server default `SSO_ANALYSIS_MIN_QUERY_TIME_SEC`
- `0` means no analysis-layer threshold filtering

Rationale:
- Users should not see one answer in rankings and a contradictory answer in trends for the same filters.

Alternative considered:
- Make trend endpoints always include all stored records.
  Rejected because it would make the UI inconsistent and harder to reason about.

### 5. Return chart-ready series data from the backend

The backend will return arrays of bucket objects with explicit bucket start times and numeric metrics, so the frontend can render charts without additional client-side aggregation.

Dashboard trend buckets should include metrics such as:
- bucket start
- total records
- total fingerprints within the filtered slice
- total query time
- average query time
- max query time

Fingerprint trend buckets should include metrics such as:
- bucket start
- total count
- total query time
- average query time
- max query time

Rationale:
- Keeps chart logic deterministic and testable.
- Reduces frontend complexity.

Alternative considered:
- Return raw records and aggregate in the browser.
  Rejected because it is inefficient and makes the UI responsible for analytics correctness.

### 6. Prefer a lightweight in-repo chart rendering approach

The UI should render simple trend charts using a lightweight approach that fits the existing plain HTML/JS frontend, such as SVG-based line or bar charts.

Rationale:
- Avoids introducing a large chart dependency into an intentionally lightweight frontend.
- Keeps styling and interaction fully under project control.

Alternative considered:
- Add a third-party charting library immediately.
  Deferred unless the in-repo chart approach proves too limiting.

## Risks / Trade-offs

- [Grouped queries over raw records may become slower as history grows] -> Keep the first version bounded to recent windows and indexed by existing occurrence timestamps; revisit rollups later if needed.
- [Timezone handling can make bucket boundaries confusing] -> Use a documented timezone policy and surface bucket timestamps clearly in API responses.
- [Trend metrics can disagree with snapshot views if filters drift] -> Reuse the same threshold-resolution and database-filter semantics across both APIs.
- [A custom lightweight chart may be less feature-rich than a library] -> Keep the first version focused on readability, not deep interactivity.

## Migration Plan

1. Add trend response models and storage query methods.
2. Add new HTTP endpoints for dashboard and fingerprint trends.
3. Extend frontend overview and fingerprint detail pages with trend panels.
4. Update API docs and README usage guidance.
5. Validate that threshold filtering produces consistent results across snapshot and trend endpoints.

Rollback:
- Remove or hide the new trend endpoints and UI panels without affecting the existing collection and snapshot analysis paths.

## Open Questions

- Whether the first UI should default to `7d/day` or `24h/hour`.
- Whether dashboard trend responses should include `totalFingerprints` per bucket from unique qualifying fingerprints, or keep the first version focused on record/time metrics only.
- Whether `dbName` filtering should be supported on both dashboard trends and fingerprint trends, or only dashboard trends in the first iteration.
