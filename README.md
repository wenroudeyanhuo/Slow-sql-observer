# Slow SQL Observer

A Go-based MySQL slow query log analysis system for collecting, normalizing, aggregating, and visualizing slow SQL events.

For a Chinese version of this guide, see `README.zh-CN.md`.

## What is implemented

The current V1 codebase includes:

- a `collector` command that incrementally reads one slow query log file
- event framing based on MySQL `# Time:` block boundaries
- parsing into structured slow query records
- conservative SQL fingerprint normalization and stable hashing
- MySQL persistence for raw records, fingerprints, aggregate stats, and checkpoints
- HTTP APIs for overview, fingerprint list, fingerprint detail, and sample records
- a lightweight static web UI for overview, ranking, and detail pages

## Quick start with Docker

This is the recommended path for first-time users because it provides a predictable MySQL environment with the fewest manual steps.

1. Start MySQL:

   ```powershell
   docker compose up -d
   ```

2. Copy environment defaults:

   ```powershell
   Copy-Item .env.example .env
   ```

3. Adjust `.env` if you want to change the MySQL DSN, schema, slow log path, or listen address.

4. Start the API server:

   ```powershell
   go run ./cmd/server
   ```

5. Start the collector in another terminal:

   ```powershell
   go run ./cmd/collector
   ```

6. Open [http://localhost:8080](http://localhost:8080).

The sample slow log at `scripts/sample-slow.log` is configured by default.

## Use an existing local MySQL instance

If you already have MySQL running locally or on a reachable host, you can skip Docker.

1. Copy environment defaults:

   ```powershell
   Copy-Item .env.example .env
   ```

2. Update `.env` to point at your MySQL instance:

   - set `SSO_DB_DSN` to a MySQL DSN with permission to create schemas and tables
   - adjust `SSO_DB_SCHEMA` if you want a schema name other than `slow_sql_observer`
   - optionally change `SSO_SLOW_LOG_PATH` to your own slow query log file

   You do not need to create the analysis schema or tables manually. The application creates them automatically on startup as long as the configured MySQL user has sufficient privileges.

3. Start the API server:

   ```powershell
   go run ./cmd/server
   ```

4. Start the collector in another terminal:

   ```powershell
   go run ./cmd/collector
   ```

5. Open [http://localhost:8080](http://localhost:8080).

The application reads `.env` automatically on startup. Explicit environment variables still take precedence if you set them in your shell or deployment environment.

## Core flow

```text
MySQL slow query log
  -> collector
  -> parser
  -> fingerprint
  -> storage
  -> API
  -> web UI
```

## API endpoints

- `GET /api/dashboard/overview`
- `GET /api/slow-sql/fingerprints`
- `GET /api/slow-sql/fingerprints/:id`
- `GET /api/slow-sql/fingerprints/:id/records`

## OpenSpec change

The active V1 implementation plan is tracked in:

- `openspec/changes/build-v1-slow-log-pipeline/`
