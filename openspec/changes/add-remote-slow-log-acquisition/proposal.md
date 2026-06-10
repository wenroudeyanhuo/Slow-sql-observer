## Why

Slow SQL Observer V2 can analyze a slow log that is already readable by the local collector, but it still does not solve the real production requirement: obtaining slow-query logs from a remote MySQL source before analysis. Without a log acquisition layer, the system remains limited to same-host or pre-mounted file setups.

V3 is needed now because the source-aware foundation already exists. The next valuable step is to add a bounded remote acquisition model that pulls slow logs into a local spool, keeps the existing parser/fingerprint pipeline intact, and makes remote-source operation explicit instead of assuming the log file is magically local.

## What Changes

- Add a remote log acquisition capability that introduces explicit source log modes, keeps `local_file` compatibility, and adds an `ssh_pull` mode that fetches slow-log bytes from a remote Linux host into a local spool before parsing.
- Add a local spool management capability that persists acquisition checkpoints separately from parser checkpoints, tracks spool location and size, supports configurable initial positioning for first-time acquisition, and enforces a spool size ceiling so remote ingestion does not grow local storage without bound.
- Add an acquisition runtime status capability that records remote-access health, last successful pull time, last remote offset, remote file identity, spool state, configuration-blocked state, and the latest acquisition error independently from parser collector status.
- Extend the query API capability with source acquisition metadata and acquisition status endpoints while preserving existing overview, fingerprint list, detail, and record routes.
- Extend the web UI capability so operators can see acquisition mode, remote source details, spool state, and the difference between acquisition failures and parse/aggregation failures.
- Add V3 configuration and onboarding guidance for SSH-based remote slow-log acquisition, including required Linux/OpenSSH source assumptions, read-only file access, local spool setup, and the initial-position / spool-limit operating model.

## Capabilities

### New Capabilities
- `remote-log-acquisition`: Support explicit source log modes, first-run positioning, and SSH-based fetching from remote Linux/OpenSSH slow-log sources into a local spool while keeping local-file ingestion compatible.
- `local-spool-management`: Maintain local spool files, enforce a configurable spool size limit, and separate acquisition checkpoints so remote fetching and local parsing can progress independently.
- `acquisition-runtime-status`: Persist and expose acquisition-specific health, blocked configuration state, access state, remote offset, spool size, and latest acquisition error information.
- `acquisition-query-api`: Provide HTTP endpoints and source metadata fields for acquisition mode, remote source metadata, spool metadata, and acquisition status.
- `acquisition-web-ui`: Display acquisition mode, remote-access state, spool state, and acquisition error context throughout the dashboard and fingerprint views.

### Modified Capabilities
- None.

## Impact

- Affected backend areas: configuration loading, collector lifecycle, source metadata handling, acquisition checkpointing, local spool file management, storage schema, and API handlers under `cmd/` and `internal/`.
- Affected frontend areas: dashboard, fingerprint list, and fingerprint detail pages under `web/`, which must now show acquisition state in addition to parser/collector state.
- Affected operational setup: environment variables, SSH key and known-hosts handling, Linux/OpenSSH remote host assumptions, local spool directory management, initial positioning behavior, spool size protection, and documentation for remote slow-log onboarding.
- New dependency impact: the collector will require an SSH transport implementation for remote acquisition, plus migration logic for new source/acquisition tables inside the analysis schema.
