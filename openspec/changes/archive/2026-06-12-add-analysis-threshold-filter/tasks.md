## 1. Configuration and request model

- [x] 1.1 Add `SSO_ANALYSIS_MIN_QUERY_TIME_SEC` to config loading with a default of `1.0` and `0` meaning disabled
- [x] 1.2 Extend API request parsing to accept `minQueryTimeSec` on overview, fingerprint list, fingerprint detail, and fingerprint records endpoints
- [x] 1.3 Define a shared effective-threshold resolution path that prefers request overrides over the configured default

## 2. Storage and query filtering

- [x] 2.1 Update overview queries to apply the effective minimum query-time threshold
- [x] 2.2 Update fingerprint list queries to exclude fingerprints whose qualifying records are all below the effective threshold
- [x] 2.3 Update fingerprint detail and records queries to apply the same effective threshold semantics

## 3. Web UI behavior

- [x] 3.1 Add a visible minimum query-time filter control to the dashboard and fingerprint views
- [x] 3.2 Make the UI send `minQueryTimeSec` with overview, list, detail, and records requests
- [x] 3.3 Show the currently active threshold in the UI so users understand the filtered slice they are viewing

## 4. Documentation and operator guidance

- [x] 4.1 Document `SSO_ANALYSIS_MIN_QUERY_TIME_SEC` in `.env.example`
- [x] 4.2 Update README guidance to explain the difference between MySQL collection threshold and Slow SQL Observer analysis threshold
- [x] 4.3 Update API reference examples to include `minQueryTimeSec`

## 5. Verification

- [x] 5.1 Add or update tests for config parsing of the default and disabled threshold cases
- [x] 5.2 Add or update API and storage tests for default threshold filtering and request overrides
- [x] 5.3 Validate locally with a low MySQL `long_query_time` and confirm that below-threshold records are stored but hidden from default rankings
