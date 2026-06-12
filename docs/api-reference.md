# Slow SQL Observer API Reference

Base URL:

```text
http://localhost:<port>
```

Use the port configured by `SSO_SERVER_ADDR`.

Examples:

- if `SSO_SERVER_ADDR=:8080`, use `http://localhost:8080`
- if `SSO_SERVER_ADDR=:8191`, use `http://localhost:8191`

All endpoints return `application/json`.

## Common concepts

### Analysis threshold

Several analysis endpoints support the optional query parameter:

- `minQueryTimeSec`

This is an analysis-layer filter, not a MySQL collection setting.

Examples:

- `minQueryTimeSec=1`
  Only include records whose `query_time_sec >= 1`.
- `minQueryTimeSec=0`
  Disable analysis-layer filtering and include all collected records.

If the parameter is omitted, the server uses `SSO_ANALYSIS_MIN_QUERY_TIME_SEC`.

### Error response

```json
{
  "error": "error message"
}
```

## 1. Source metadata

```http
GET /api/source
```

Returns the currently observed source configuration and stored metadata.

## 2. Collector status

```http
GET /api/collector/status
```

Returns parser-side runtime health and checkpoint information.

## 3. Acquisition status

```http
GET /api/acquisition/status
```

Returns acquisition-side runtime health, remote access status, and spool information.

## 4. Discovery status

```http
GET /api/discovery/status
```

Returns `mysql_auto` discovery information when available.

If the current source is not using `mysql_auto`, the response may be:

```json
{
  "discoveryState": "unknown",
  "message": "no discovery data available; source may not be in mysql_auto mode"
}
```

## 5. Dashboard overview

```http
GET /api/dashboard/overview
GET /api/dashboard/overview?minQueryTimeSec=1
```

Response fields:

- `activeMinQueryTimeSec`
- `totalRecords`
- `totalFingerprints`
- `totalQueryTimeSec`
- `avgQueryTimeSec`
- `maxQueryTimeSec`
- `lastIngestedAt`
- `topFingerprints`

## 6. Fingerprint list

```http
GET /api/slow-sql/fingerprints
GET /api/slow-sql/fingerprints?page=1&pageSize=20&sortBy=totalQueryTimeSec&sortOrder=desc&dbName=sso_demo_app&sqlType=SELECT&keyword=orders&minQueryTimeSec=1
```

Query parameters:

- `page`
- `pageSize`
- `sortBy`
  - `totalQueryTimeSec`
  - `avgQueryTimeSec`
  - `maxQueryTimeSec`
  - `totalCount`
  - `lastSeenAt`
- `sortOrder`
  - `asc`
  - `desc`
- `dbName`
- `sqlType`
- `keyword`
- `minQueryTimeSec`

Response fields:

- `activeMinQueryTimeSec`
- `items`
- `total`
- `page`
- `pageSize`

## 7. Fingerprint detail

```http
GET /api/slow-sql/fingerprints/:id
GET /api/slow-sql/fingerprints/:id?minQueryTimeSec=1
```

Returns one fingerprint view with threshold-aware aggregate fields such as:

- `totalCount`
- `totalQueryTimeSec`
- `avgQueryTimeSec`
- `maxQueryTimeSec`
- `firstSeenAt`
- `lastSeenAt`
- `activeMinQueryTimeSec`

If the fingerprint has no qualifying records under the active threshold, the endpoint returns `404`.

## 8. Fingerprint records

```http
GET /api/slow-sql/fingerprints/:id/records
GET /api/slow-sql/fingerprints/:id/records?page=1&pageSize=20&sortBy=occurredAt&sortOrder=desc&minQueryTimeSec=1
```

Query parameters:

- `page`
- `pageSize`
- `sortBy`
  - `occurredAt`
  - `queryTimeSec`
- `sortOrder`
  - `asc`
  - `desc`
- `minQueryTimeSec`

Response fields:

- `activeMinQueryTimeSec`
- `items`
- `total`
- `page`
- `pageSize`

## Notes for UI consumers

- `activeMinQueryTimeSec` is returned by overview, list, detail, and records responses so the frontend can show the effective threshold to the user.
- `minQueryTimeSec` filters analysis output only. It does not change what the collector stores in `slow_query_records`.
