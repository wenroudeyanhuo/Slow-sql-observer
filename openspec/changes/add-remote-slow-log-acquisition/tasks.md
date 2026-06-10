## 1. Configuration and Source Metadata

- [x] 1.1 Add `SSO_SOURCE_LOG_MODE` with `local_file` default and validate mode-specific configuration.
- [x] 1.2 Add SSH pull configuration fields for remote host, port, user, remote slow-log path, private key path, known-hosts path, initial position, and local spool directory.
- [x] 1.3 Add configurable local spool size limits and enforce Linux/OpenSSH-only remote-source validation.
- [x] 1.3 Extend persisted source metadata and `/api/source` payloads with acquisition mode, remote source metadata, and local spool path fields.

## 2. Storage and Migration

- [x] 2.1 Add schema initialization for `acquisition_checkpoints` and `acquisition_status`.
- [x] 2.2 Extend source persistence with acquisition metadata columns needed for API and UI display.
- [x] 2.3 Add automatic migration logic for existing V2 schemas so new acquisition tables and source metadata fields are created safely.

## 3. Remote Acquisition Engine

- [x] 3.1 Add an acquisition service abstraction that supports `local_file` no-op mode and `ssh_pull` mode.
- [x] 3.2 Implement SSH key-based remote acquisition with known-hosts verification and byte-range checkpointing.
- [x] 3.3 Detect remote rotation/truncation and restart remote pulling from offset `0` of the new current file.
- [x] 3.4 Apply explicit first-run positioning with `start` / `end` semantics and default to `end`.

## 4. Local Spool Management

- [x] 4.1 Write remotely acquired bytes into a deterministic local spool file for the active source.
- [x] 4.2 Keep acquisition checkpoints separate from parser collector checkpoints.
- [x] 4.3 Truncate the local spool file and reset parser checkpoint offsets when the spool has been fully consumed.

## 5. Runtime Status and Collector Integration

- [x] 5.1 Persist acquisition runtime status separately from parser collector status.
- [x] 5.2 Update the collector loop to acquire remote bytes before parsing the local spool file.
- [x] 5.3 Ensure acquisition failures, parser failures, blocked configuration states, and combined healthy states update the correct status models.

## 6. API and Web UI

- [x] 6.1 Add `GET /api/acquisition/status` for the active source.
- [x] 6.2 Keep existing overview, fingerprint list, detail, and records routes compatible while extending source metadata responses.
- [x] 6.3 Update the dashboard and fingerprint pages to show acquisition mode, remote source context, spool state, and acquisition-vs-parser failure messaging.

## 7. Documentation and Examples

- [x] 7.1 Update `.env.example` and README files with V3 remote acquisition configuration and local-file fallback guidance.
- [x] 7.2 Document Linux/OpenSSH onboarding requirements, known-hosts handling, local spool directory expectations, initial-position defaults, and remote rotation limitations.
- [x] 7.3 Add operator guidance for acquisition status interpretation and spool lifecycle behavior.

## 8. Verification

- [x] 8.1 Add tests for mode selection, SSH configuration validation, and source metadata serialization.
- [x] 8.2 Add tests for acquisition checkpointing, remote rotation handling, and fully consumed spool truncation behavior.
- [x] 8.3 Add tests for acquisition status APIs and UI state handling for acquisition failures versus parser failures.
- [x] 8.4 Run `openspec validate add-remote-slow-log-acquisition` and confirm the change is apply-ready.
