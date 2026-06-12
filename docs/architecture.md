# Architecture

This document captures the runtime architecture and the collector flow of Slow SQL Observer in a GitHub-friendly Mermaid format.

## System architecture

```mermaid
flowchart LR
    User["User / Browser"]
    UI["Web UI<br/>static files"]
    API["API Server<br/>cmd/server"]
    Collector["Collector<br/>cmd/collector"]
    Analysis["Analysis MySQL<br/>slow_sql_observer schema"]
    SourceDB["Observed MySQL<br/>single source"]
    LocalLog["Local slow log file"]
    RemoteLog["Remote slow log file"]
    Spool["Local spool file"]

    User -->|"open dashboard / call API"| API
    API --> UI
    API -->|"read overview, fingerprints, records, status"| Analysis

    Collector -->|"write raw records, fingerprints, status, checkpoints"| Analysis
    Collector -->|"optional source probe / mysql_auto discovery"| SourceDB

    SourceDB -->|"log_output=TABLE"| Collector
    LocalLog -->|"local_file"| Collector
    RemoteLog -->|"ssh_pull / mysql_file"| Spool
    Spool --> Collector
```

## Collector decision flow

```mermaid
flowchart TD
    Start["Collector tick starts"] --> Load["Load source config and checkpoints"]
    Load --> Mode{"Source mode?"}

    Mode -->|"local_file"| Local["Read local slow log file"]
    Mode -->|"ssh_pull"| SSH["Pull remote slow log through SSH"]
    Mode -->|"mysql_auto"| Discover["Connect to MySQL and discover slow log mode"]

    Discover --> DiscoveryState{"Discovery healthy?"}
    DiscoveryState -->|"no"| Blocked["Update blocked / error status"]
    DiscoveryState -->|"yes"| Effective{"Effective mode?"}

    Effective -->|"mysql_table"| Table["Query mysql.slow_log"]
    Effective -->|"mysql_file"| MySQLFile["Pull remote slow log through SSH"]

    SSH --> Parse
    Local --> Parse
    MySQLFile --> Parse
    Table --> Normalize

    Parse["Parse slow log blocks"] --> Normalize["Normalize SQL and build fingerprint"]
    Normalize --> Persist["Persist raw records, fingerprint stats, status, checkpoints"]
    Persist --> Retention["Run raw-record retention if enabled"]
    Retention --> Done["Collector tick completes"]

    Blocked --> Done
```

## Data model flow

```mermaid
flowchart LR
    SlowEvent["Slow SQL event"]
    Parsed["Parsed record"]
    Fingerprint["Fingerprint / normalized SQL"]
    RawTable["slow_query_records"]
    FingerprintTable["fingerprints"]
    StatsTable["fingerprint_stats"]
    StatusTable["collector_status / acquisition_status / discovery"]
    CheckpointTable["collector_checkpoints / acquisition_checkpoints / table_ingestion_checkpoints"]

    SlowEvent --> Parsed
    Parsed --> Fingerprint
    Parsed --> RawTable
    Fingerprint --> FingerprintTable
    Parsed --> StatsTable
    Parsed --> CheckpointTable
    Parsed --> StatusTable
```

## Scope notes

- The current release is single-source by design.
- The collector watches one MySQL instance, not one individual business schema.
- A new business database created on the same observed MySQL instance is included automatically as long as its slow SQL enters the slow-log source.
- Hosted MySQL with `log_output=FILE` but without SSH or a provider log API is outside the supported scope of this release.
