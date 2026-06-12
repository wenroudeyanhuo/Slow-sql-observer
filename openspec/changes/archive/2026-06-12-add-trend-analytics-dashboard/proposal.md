## Why

The current product can show static aggregates and fingerprint rankings, but it cannot answer whether slow-SQL pressure is rising, falling, or concentrated in a specific time window. We need a time-series view so operators can understand trend direction, compare time buckets, and verify whether a spike is getting better or worse.

## What Changes

- Add dashboard trend analytics APIs that aggregate slow-SQL data by time bucket.
- Add fingerprint-specific trend APIs so one normalized SQL can be inspected over time.
- Add web UI trend sections with charts or timeline visuals on the overview and fingerprint detail pages.
- Allow trend queries to reuse existing analysis filters such as `minQueryTimeSec` and optional database scoping where applicable.
- Document the new trend endpoints, query parameters, and operator expectations for time-bucketed analytics.

## Capabilities

### New Capabilities
- `trend-analytics`: Time-series aggregation and visualization for dashboard-level and fingerprint-level slow-SQL analysis.

### Modified Capabilities
- `analysis-threshold-filtering`: Extend analysis-threshold semantics so trend endpoints honor the same effective `minQueryTimeSec` behavior as overview, list, detail, and records views.

## Impact

- Affected code: API handlers, storage queries, domain models, frontend pages, and documentation.
- New public interfaces: trend-oriented HTTP endpoints and related UI controls.
- Dependencies: likely a lightweight frontend charting approach or a small in-repo visualization implementation.
- Systems: analysis database query patterns will expand from point-in-time aggregates to grouped time-bucket aggregations.
