# AI Execution Plan for OpenSpec

## Document Purpose

This document is written for AI-assisted planning and implementation.
It defines the project scope, boundaries, sequencing, deliverables, and acceptance criteria for V1 of Slow SQL Observer.

## Project Definition

Build a runnable, reasonably complete, single-instance MySQL slow query log analysis system.

The system should:

- collect slow query events from MySQL slow query logs
- parse them into structured records
- normalize SQL into reusable templates
- aggregate metrics by fingerprint
- persist raw and aggregated data
- provide APIs for querying results
- visualize results in a web UI

## Product Goal

Deliver a V1 that proves the full analysis pipeline works in practice:

1. a slow query log file can be ingested incrementally
2. slow SQL can be parsed into structured events
3. similar SQL statements can be normalized into the same fingerprint
4. fingerprint-level statistics can be aggregated and stored
5. the frontend can query and display useful results

## Non-Goals for V1

Do not implement the following in V1:

- multi-instance collection
- `performance_schema` ingestion
- endpoint tracing
- trace to SQL correlation
- alerting
- automatic SQL optimization engine
- ClickHouse migration
- OpenTelemetry integration

## Scope Boundaries

### In Scope

- one MySQL instance
- one slow query log source
- one collector service
- one backend API service
- one frontend web application
- MySQL as the analysis database

### Out of Scope

- agent deployment across multiple hosts
- cloud-native distributed collection
- full APM capabilities
- cross-service observability

## Required Modules

### 1. Collector

Responsibilities:

- read slow query log files incrementally
- track file offset or checkpoint state
- emit raw slow log entries for parsing

Expected outputs:

- parsed raw event blocks
- updated checkpoints

### 2. Parser

Responsibilities:

- parse MySQL slow log blocks into structured records
- extract timestamp, database, user, host, query time, lock time, rows sent, rows examined, and SQL text

Expected outputs:

- `SlowQueryRecord` domain objects

### 3. Fingerprint

Responsibilities:

- normalize SQL parameters
- collapse structurally equivalent SQL into a stable template
- compute a repeatable fingerprint hash
- identify SQL type and main table when possible

Expected outputs:

- `Fingerprint` objects
- normalized SQL text

### 4. Aggregator

Responsibilities:

- update aggregate metrics by fingerprint
- maintain count, avg, max, total timing metrics
- maintain rows-sent and rows-examined metrics
- maintain first-seen and last-seen timestamps

Expected outputs:

- `FingerprintStats`

### 5. Storage

Responsibilities:

- persist raw records
- persist fingerprints
- persist aggregated stats
- persist collector checkpoints

Expected outputs:

- successful writes and queryable read models

### 6. API

Responsibilities:

- serve overview statistics
- serve fingerprint list and detail
- serve raw sample records
- support basic filter, sort, and pagination behavior

Expected outputs:

- stable HTTP APIs

### 7. Web UI

Responsibilities:

- display summary metrics
- display ranked fingerprint list
- display fingerprint detail and sample SQL

Expected outputs:

- dashboard page
- fingerprint list page
- fingerprint detail page

## Suggested Domain Models

### SlowQueryRecord

Fields should include at least:

- record id
- occurred at
- db name
- user name
- client host or ip
- raw sql
- normalized sql
- fingerprint hash
- query time
- lock time
- rows sent
- rows examined

### Fingerprint

Fields should include at least:

- fingerprint id
- fingerprint hash
- normalized sql
- sql type
- main table name
- first seen at
- last seen at

### FingerprintStats

Fields should include at least:

- fingerprint id
- total count
- total query time
- avg query time
- max query time
- total rows examined
- avg rows examined
- max rows examined
- total rows sent
- avg rows sent
- max rows sent
- last seen at

### CollectorCheckpoint

Fields should include at least:

- instance name
- log file path
- last offset
- updated at

## Implementation Order

Follow this order strictly unless there is a strong reason to change it:

1. bootstrap repository structure
2. define config and domain models
3. implement collector
4. implement parser
5. implement fingerprint
6. implement aggregator
7. implement storage layer
8. connect collector to parser to storage
9. implement API layer
10. implement frontend pages
11. add Docker Compose and demo setup
12. refine README and docs

## Deliverables by Phase

### Phase 1: Bootstrap

- backend and frontend skeleton
- config files
- startup entry points

### Phase 2: Ingestion Core

- collector
- parser
- fingerprint module
- aggregator

### Phase 3: Persistence

- schema initialization
- repositories
- checkpoint management

### Phase 4: Application Layer

- query services
- REST API

### Phase 5: UI Layer

- dashboard
- list
- detail

### Phase 6: Packaging

- Docker Compose
- sample data
- final documentation

## Acceptance Criteria for V1

V1 is complete when all of the following are true:

1. a sample MySQL slow query log can be ingested without manual editing
2. parsed records are stored in MySQL analysis tables
3. normalized fingerprints are generated consistently
4. aggregate statistics are visible for each fingerprint
5. overview, list, and detail APIs are usable
6. the frontend can display overview, list, and detail pages
7. the system can be started locally with documented steps

## Design Rules for AI Implementation

- do not add multi-instance abstractions in V1
- do not add distributed job scheduling in V1
- do not introduce ClickHouse in V1
- do not build generic plugin systems in V1
- keep interfaces clear but avoid speculative abstractions
- prefer a working pipeline over architectural ornament

## Future Extensions

These may be proposed after V1, but should not be merged into the V1 scope:

- endpoint latency observation
- trace to SQL association
- `performance_schema` collector
- ClickHouse analytics backend
- alerting and notification
- OpenTelemetry support
- multi-instance collection

