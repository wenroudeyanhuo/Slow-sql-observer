# Slow SQL Observer

A Go-based MySQL slow query log analysis system that reads one slow log source, fingerprints repeated SQL, aggregates metrics, and serves an API and web UI.

For a Chinese version of this guide, see `README.zh-CN.md`.

## V2 runtime model

The current active model is source-aware V2:

- one observed MySQL source
- one slow query log file as the primary ingestion channel
- one analysis MySQL schema used by Slow SQL Observer
- one collector process
- one API and web UI process

The collector must run where the configured slow log file is readable. The recommended setup is the same host as MySQL, or a host with mounted access to the slow log path.

## Configuration

Preferred V2 variables:

- `SSO_SOURCE_INSTANCE_NAME`
- `SSO_SOURCE_SLOW_LOG_PATH`
- `SSO_SOURCE_DB_DSN` (optional metadata/validation probe)
- `SSO_SOURCE_TIMEZONE`
- `SSO_SOURCE_DESCRIPTION`
- `SSO_ANALYSIS_DB_DSN`
- `SSO_ANALYSIS_DB_SCHEMA`
- `SSO_SERVER_ADDR`
- `SSO_WEB_DIR`
- `SSO_COLLECTOR_POLL_INTERVAL`
- `SSO_RAW_RECORD_RETENTION_DAYS`
- `SSO_LOG_LEVEL`

V1 variable names are still accepted for one compatibility cycle:

- `SSO_INSTANCE_NAME`
- `SSO_SLOW_LOG_PATH`
- `SSO_DB_DSN`
- `SSO_DB_SCHEMA`

When legacy names are used, the application logs deprecation warnings. If both V1 and V2 names are present, V2 names win.

## Quick start

1. Copy the environment template:

   ```powershell
   Copy-Item .env.example .env
   ```

2. Point `SSO_ANALYSIS_DB_DSN` at a MySQL account that can create schemas and tables on first startup.

3. Point `SSO_SOURCE_SLOW_LOG_PATH` at a readable MySQL slow query log file.

4. Optionally set `SSO_SOURCE_DB_DSN` if you want source DB connectivity checks and metadata enrichment.

5. Start the API server:

   ```powershell
   go run ./cmd/server
   ```

6. Start the collector in another terminal:

   ```powershell
   go run ./cmd/collector
   ```

7. Open [http://localhost:8080](http://localhost:8080).

## Source-side prerequisites

The observed MySQL source should satisfy:

- slow query logging is enabled
- `log_output=FILE`
- the configured slow log path is correct
- the collector process can read that file

`SSO_SOURCE_DB_DSN` is optional. It is used to probe source connectivity and collect metadata such as host or version. It is not the primary ingestion path.

## Raw-record retention

`SSO_RAW_RECORD_RETENTION_DAYS` controls cleanup of `slow_query_records`:

- `0` or a negative value disables cleanup
- a positive value deletes raw records older than the configured number of days
- fingerprints and aggregate statistics are retained by default

Cleanup runs from the collector loop. A retention failure degrades collector status, but it does not roll back already committed ingest results.

## API endpoints

- `GET /api/source`
- `GET /api/collector/status`
- `GET /api/dashboard/overview`
- `GET /api/slow-sql/fingerprints`
- `GET /api/slow-sql/fingerprints/:id`
- `GET /api/slow-sql/fingerprints/:id/records`

## OpenSpec changes

- V1 baseline: `openspec/changes/build-v1-slow-log-pipeline/`
- Active V2 change: `openspec/changes/add-source-aware-v2/`
