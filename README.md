# Slow SQL Observer

Slow SQL Observer is a Go-based MySQL slow-query analysis tool. It ingests one observed slow-log source, fingerprints repeated SQL, aggregates performance metrics, and serves both an API and a lightweight web UI.

For a Chinese version of this guide, see `README.zh-CN.md`.

## V3 runtime model

The current runtime model is still single-source, but V3 adds a first-class acquisition layer:

- one observed MySQL source
- one analysis MySQL schema owned by Slow SQL Observer
- one collector process
- one API/web process
- one source log mode:
  - `local_file`: parse a locally readable slow log directly
  - `ssh_pull`: pull a remote Linux/OpenSSH slow log into a local spool file, then parse the spool

The analysis database and the observed source database are intentionally separate concerns. `SSO_ANALYSIS_DB_DSN` stores analysis data. `SSO_SOURCE_DB_DSN` is optional and is only used for metadata probing and connectivity validation.

## Configuration

### Core settings

- `SSO_SERVER_ADDR`
- `SSO_WEB_DIR`
- `SSO_SOURCE_INSTANCE_NAME`
- `SSO_SOURCE_LOG_MODE`
- `SSO_SOURCE_DB_DSN` (optional)
- `SSO_SOURCE_TIMEZONE`
- `SSO_SOURCE_DESCRIPTION`
- `SSO_ANALYSIS_DB_DSN`
- `SSO_ANALYSIS_DB_SCHEMA`
- `SSO_COLLECTOR_POLL_INTERVAL`
- `SSO_RAW_RECORD_RETENTION_DAYS`
- `SSO_LOG_LEVEL`

### Local-file mode

- `SSO_SOURCE_SLOW_LOG_PATH`

### SSH-pull mode

- `SSO_SOURCE_REMOTE_HOST`
- `SSO_SOURCE_REMOTE_PORT`
- `SSO_SOURCE_REMOTE_USER`
- `SSO_SOURCE_REMOTE_SLOW_LOG_PATH`
- `SSO_SOURCE_SSH_PRIVATE_KEY_PATH`
- `SSO_SOURCE_SSH_KNOWN_HOSTS_PATH`
- `SSO_SOURCE_LOCAL_SPOOL_DIR`
- `SSO_SOURCE_INITIAL_POSITION`
- `SSO_SOURCE_LOCAL_SPOOL_MAX_BYTES`

### V1 compatibility

The following V1 names are still accepted for one transition cycle:

- `SSO_INSTANCE_NAME`
- `SSO_SLOW_LOG_PATH`
- `SSO_DB_DSN`
- `SSO_DB_SCHEMA`

When both V1 and V2/V3 names are present, the newer names win and deprecation warnings are emitted.

## Quick start

1. Copy the template:

   ```powershell
   Copy-Item .env.example .env
   ```

2. Point `SSO_ANALYSIS_DB_DSN` at a MySQL account that can create schemas and tables on first startup.

3. Choose one source mode:

   `local_file`
   Set `SSO_SOURCE_LOG_MODE=local_file` and point `SSO_SOURCE_SLOW_LOG_PATH` at a readable slow log.

   `ssh_pull`
   Set `SSO_SOURCE_LOG_MODE=ssh_pull` and fill the SSH-related `SSO_SOURCE_REMOTE_*`, key, known-hosts, and spool settings.

4. Optionally set `SSO_SOURCE_DB_DSN` if you want source metadata such as host/version to be collected.

5. Start the API server:

   ```powershell
   go run ./cmd/server
   ```

6. Start the collector in another terminal:

   ```powershell
   go run ./cmd/collector
   ```

7. Open [http://localhost:8080](http://localhost:8080).

## Source prerequisites

### Common

- MySQL slow-query logging is enabled
- `log_output=FILE`
- the configured slow-log path is correct

### For `local_file`

- the collector host can read the local slow-log path directly

### For `ssh_pull`

- the remote host is Linux with OpenSSH shell access
- the configured SSH user can read the MySQL slow log
- `SSO_SOURCE_SSH_KNOWN_HOSTS_PATH` contains the remote host key
- `SSO_SOURCE_SSH_PRIVATE_KEY_PATH` points to the private key used for authentication
- the local collector host can write to `SSO_SOURCE_LOCAL_SPOOL_DIR`

V3 only supports key-file SSH auth with known-hosts verification. Password auth, agent-only auth, and remote Windows targets are out of scope.

## Acquisition and spool behavior

`ssh_pull` mode follows this pipeline:

`remote slow log -> SSH pull -> local spool file -> parser -> fingerprint -> analysis storage`

Important operating rules:

- `SSO_SOURCE_INITIAL_POSITION=end` is the default and is recommended for first production onboarding
- `SSO_SOURCE_INITIAL_POSITION=start` intentionally backfills from the current remote file head
- remote acquisition tracks its own checkpoint separately from parser progress
- if the local spool reaches `SSO_SOURCE_LOCAL_SPOOL_MAX_BYTES`, acquisition is blocked for that cycle and the condition is exposed through acquisition status
- when the parser fully consumes the spool, the spool file is truncated and the parser checkpoint is reset to `0`
- V3 only follows the current remote log file; it does not backfill archived rotated files

## Retention

`SSO_RAW_RECORD_RETENTION_DAYS` controls cleanup of `slow_query_records`:

- `0` or a negative value disables cleanup
- a positive value deletes raw records older than the configured number of days
- fingerprint aggregates are retained

Retention runs inside the collector loop. A retention failure degrades parser status, but already committed ingest data is kept.

## API

Current API routes:

- `GET /api/source`
- `GET /api/acquisition/status`
- `GET /api/collector/status`
- `GET /api/dashboard/overview`
- `GET /api/slow-sql/fingerprints`
- `GET /api/slow-sql/fingerprints/:id`
- `GET /api/slow-sql/fingerprints/:id/records`

The web UI now shows source metadata, acquisition runtime status, parser status, remote context, and spool state separately.

## OpenSpec workflow

- V1 archived change: `openspec/changes/archive/2026-06-09-build-v1-slow-log-pipeline/`
- V2 archived change: `openspec/changes/archive/2026-06-09-add-source-aware-v2/`
- Active V3 change: `openspec/changes/add-remote-slow-log-acquisition/`
