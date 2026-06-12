## Why

The current product treats every event that has already entered the MySQL slow-log source as equal during analysis and ranking. In real usage, teams often lower MySQL slow-log thresholds for sampling, which causes the main overview and fingerprint list to be flooded with marginally slow statements and makes it harder to focus on truly expensive SQL.

## What Changes

- Add a configurable analysis threshold for slow-query duration that is independent from the MySQL slow-log collection threshold.
- Apply the threshold to overview and fingerprint-list style analysis so the main UI defaults to showing only statements above a meaningful latency floor such as 1 second.
- Allow API consumers and the web UI to override the default threshold per request for ad hoc investigation.
- Preserve raw record ingestion and storage below the analysis threshold so operators can still inspect lower-latency samples when needed.
- Expose the effective threshold behavior clearly in configuration and user-facing documentation.

## Capabilities

### New Capabilities
- `analysis-threshold-filtering`: Configure and apply an analysis-layer minimum query-time threshold for overview, fingerprint ranking, and record exploration.

### Modified Capabilities
- None.

## Impact

- Affected code: configuration loading, query/list APIs, storage query filters, and web UI controls.
- Affected interfaces: overview and slow-SQL list/detail/records endpoints gain threshold-aware behavior and request parameters.
- Affected docs: `.env.example`, README, API reference, and operator guidance around the difference between collection threshold and analysis threshold.
