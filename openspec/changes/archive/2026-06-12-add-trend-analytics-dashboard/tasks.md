## 1. Trend query contract and domain models

- [x] 1.1 Define request parameters and response models for dashboard trend and fingerprint trend endpoints
- [x] 1.2 Add validated bucket and recent-window parsing so trend queries stay within documented bounds
- [x] 1.3 Extend shared analysis-threshold resolution so trend endpoints use the same effective `minQueryTimeSec` behavior

## 2. Storage and backend APIs

- [x] 2.1 Implement dashboard trend aggregation queries over `slow_query_records`
- [x] 2.2 Implement fingerprint trend aggregation queries over `slow_query_records`
- [x] 2.3 Add HTTP endpoints for `/api/dashboard/trends` and `/api/slow-sql/fingerprints/:id/trends`
- [x] 2.4 Return client-error responses for unsupported bucket or recent-window requests

## 3. Web UI trends

- [x] 3.1 Add overview-page controls and a trend panel for dashboard analytics
- [x] 3.2 Add fingerprint-detail controls and a trend panel for fingerprint analytics
- [x] 3.3 Render lightweight chart or timeline visuals from chart-ready API responses
- [x] 3.4 Show clear empty and error states for missing or filtered-out trend data

## 4. Documentation and tests

- [x] 4.1 Update API documentation with trend endpoints, query parameters, and example responses
- [x] 4.2 Update README guidance to explain trend capabilities, supported windows, and threshold interaction
- [x] 4.3 Add backend tests for bucket validation and threshold-aware trend aggregation
- [x] 4.4 Add frontend tests or validation coverage for trend rendering and filter propagation
