## Context

Slow SQL Observer V2 established a clear single-source model, source-aware storage, collector runtime status, and stable parsing/fingerprinting behavior. That version still assumes the slow log file is already accessible on the local filesystem. In the real target environment, the observed MySQL instance and the analysis service are often separate hosts, so the product needs a first-class log acquisition layer instead of relying on pre-mounted files or manual copying.

V3 must preserve the current single-source scope, keep the existing parser and fingerprint pipeline largely unchanged, and add only one bounded remote transport model. The design pressure is to add a real acquisition path without turning the project into a multi-agent or fleet-management system.

## Goals / Non-Goals

**Goals:**

- Add explicit source log modes so a source can be modeled as either `local_file` or `ssh_pull`.
- Support SSH-based remote slow-log acquisition into a local spool directory before parsing.
- Make first-time acquisition behavior explicit so initial onboarding can start from either the file head or the current file end.
- Keep parser checkpoints and acquisition checkpoints separate so remote fetching and local parsing are independently resumable.
- Persist acquisition runtime status, including remote access health, last pull, remote file identity, remote offset, local spool path, and latest acquisition error.
- Keep the current overview/fingerprint analysis API compatible while exposing acquisition metadata and status through dedicated APIs and UI context.
- Prevent unbounded local spool growth by resetting fully consumed spool files when the parser has caught up.

**Non-Goals:**

- Multi-source scheduling, source fleet management, or multiple remote hosts in one deployment.
- Non-SSH remote transports such as SMB, SFTP-only without SSH shell semantics, rsync, cloud object storage, or custom agents.
- Password-based SSH authentication, SSH agent forwarding/agent-only auth, interactive login flows, or automatic source-side MySQL/OS configuration.
- Fetching old rotated log tails from archived remote files after rotation; V3 follows the same pragmatic current-file-only boundary as earlier collector behavior.
- Remote Windows hosts or non-POSIX command environments.
- Replacing the existing parser, fingerprinting rules, or overview/list/detail UI information architecture.

## Decisions

### 1. Introduce explicit source log modes with `local_file` as the default

V3 will add `SSO_SOURCE_LOG_MODE` with two supported values:

- `local_file`
- `ssh_pull`

`local_file` preserves the current behavior and continues to use `SSO_SOURCE_SLOW_LOG_PATH` directly as the parse source.

`ssh_pull` adds these source-specific fields:

- `SSO_SOURCE_REMOTE_HOST`
- `SSO_SOURCE_REMOTE_PORT`
- `SSO_SOURCE_REMOTE_USER`
- `SSO_SOURCE_REMOTE_SLOW_LOG_PATH`
- `SSO_SOURCE_SSH_PRIVATE_KEY_PATH`
- `SSO_SOURCE_SSH_KNOWN_HOSTS_PATH`
- `SSO_SOURCE_LOCAL_SPOOL_DIR`
- `SSO_SOURCE_INITIAL_POSITION`
- `SSO_SOURCE_LOCAL_SPOOL_MAX_BYTES`

Why this decision:

- It keeps V2 deployments working unchanged while making remote acquisition explicit instead of implicit.
- It limits V3 to one production-relevant remote transport instead of prematurely generalizing the transport layer.

Alternatives considered:

- Making remote acquisition replace local-file mode entirely: rejected because local-file deployments remain valid and useful.
- Supporting many transports in V3: rejected because it would grow scope faster than the current product maturity justifies.

### 2. Restrict `ssh_pull` to Linux/OpenSSH sources with key-file authentication

V3 will explicitly support only remote Linux/OpenSSH environments that can execute standard shell commands and expose a readable slow-log file. Authentication is file-based SSH private-key authentication plus known-hosts verification. SSH agent-only authentication and password prompts are out of scope.

Why this decision:

- It narrows the implementation surface to the environment most likely to host MySQL slow-query logs in production.
- It avoids ambiguous "maybe supported" transport behavior across Windows shells or interactive SSH flows.

Alternatives considered:

- Generic SSH target support regardless of OS/shell: rejected because byte-accurate remote reads depend on predictable shell tooling.
- SSH agent support in V3: rejected because it adds another credential path without changing the product value of the first transport release.

### 3. Use SSH key-based pull with host verification as the only remote transport in V3

For `ssh_pull`, the collector will connect to the remote host with key-based SSH authentication and host verification driven by `SSO_SOURCE_SSH_KNOWN_HOSTS_PATH`. V3 does not support password authentication.

Why this decision:

- It is secure enough for a real operational model and avoids normalizing password-in-config workflows.
- It keeps the transport implementation simple and predictable for one release.

Alternatives considered:

- Password-based auth: rejected because it encourages insecure configuration and complicates operational handling.
- Auto-accept or insecure host verification by default: rejected because it weakens the security posture of a remote acquisition feature.

### 4. Make first-time acquisition position explicit and default it to file end

V3 will add `SSO_SOURCE_INITIAL_POSITION` with supported values:

- `start`
- `end`

Default: `end`

`start` is intended for controlled backfill runs. `end` is intended for production onboarding where only newly written remote slow-log content should be acquired after setup.

Why this decision:

- It prevents accidental multi-gigabyte first syncs when connecting to an existing production slow log.
- It still leaves a deliberate backfill path for operators who want historical replay.

Alternatives considered:

- Always start from file head: rejected because it is too risky for first-time production onboarding.
- Always start from file end: rejected because it removes an intentional backfill mode.

### 5. Acquire remote bytes into a local spool file and keep parser ingestion local

`ssh_pull` mode will not change the parser's input model. Instead, remote log acquisition writes fetched bytes into a local spool file such as:

`<SSO_SOURCE_LOCAL_SPOOL_DIR>/<source_key>.slow.log`

The existing framer/parser/fingerprint pipeline continues to read from the local spool file as if it were a local slow log. This keeps the main analysis pipeline stable while introducing acquisition as a distinct stage:

`remote source -> SSH pull -> local spool -> framer -> parser -> fingerprint -> storage`

Why this decision:

- It minimizes parser and collector risk by keeping their input shape file-based.
- It creates a clean boundary between remote transport problems and SQL analysis problems.

Alternatives considered:

- Streaming remote lines directly into the parser: rejected because it entangles transport state with parser completeness logic.
- Rewriting the parser to understand remote chunk semantics: rejected because V3 should build on the V2 file pipeline.

### 6. Separate acquisition checkpoints from parser checkpoints

V3 will add a dedicated `acquisition_checkpoints` table keyed by `source_id` that tracks at least:

- remote host/path identity
- remote file identity
- last remote offset copied
- local spool path
- last local spool size
- initial position mode
- updated timestamp

`collector_checkpoints` remains the parser-side completed-event checkpoint for whichever local file is currently being parsed.

Why this decision:

- Remote pulling and local parsing fail for different reasons and resume from different coordinates.
- It prevents parser checkpoint resets from corrupting remote acquisition state.

Alternatives considered:

- Reusing `collector_checkpoints` for remote offsets: rejected because acquisition and parsing are distinct state machines.

### 7. Enforce a local spool size ceiling

V3 will add `SSO_SOURCE_LOCAL_SPOOL_MAX_BYTES`. If the local spool file exceeds this ceiling, the acquisition stage will stop pulling new remote bytes for that cycle and mark acquisition status as degraded or blocked until the parser catches up enough for the spool to shrink or reset.

Why this decision:

- It provides a bounded safety valve when acquisition outpaces parsing.
- It turns a disk-growth problem into an explicit operational state instead of silent local exhaustion.

Alternatives considered:

- No spool limit at all: rejected because remote acquisition can outpace parsing in real traffic spikes.
- Automatic aggressive compaction of partially consumed spool data: rejected because it adds fragile offset translation logic.

### 8. Keep spool management simple by truncating only when fully consumed

V3 will not implement partial spool compaction. Instead, after a collector cycle:

- if parser checkpoint has caught up to the end of the local spool file
- and the acquisition stage has no unread bytes left in spool
- then the collector truncates the spool file to empty and resets the parser checkpoint offset to `0`

If the parser is not fully caught up, the spool file remains unchanged.

Why this decision:

- It prevents unbounded spool growth without introducing fragile mid-file compaction and checkpoint rewrite logic.
- In the normal case, the parser immediately consumes newly pulled data, so full resets are sufficient.

Alternatives considered:

- Never compact the spool: rejected because remote acquisition would cause unbounded local disk growth.
- Arbitrary prefix compaction of partially consumed spool content: rejected because it adds a second offset-translation problem.

### 9. Persist acquisition status separately from parser collector status

V3 will add an `acquisition_status` table keyed by `source_id`. It stores at least:

- `acquisition_state`
- `remote_access_state`
- `transport_mode`
- `last_successful_pull_at`
- `last_remote_offset`
- `last_remote_file_identity`
- `last_spool_size_bytes`
- `last_error_at`
- `last_error_message`
- `updated_at`

`acquisition_state` supports at least:

- `idle`
- `healthy`
- `degraded`
- `error`
- `blocked`

The existing `collector_status` remains focused on local parsing/ingestion status. This means the UI and API can distinguish:

- remote fetch is failing, but old parsed data is still queryable
- remote fetch is healthy, but parser/ingest is failing
- both are healthy
- acquisition cannot even start because config or credential prerequisites are missing

Why this decision:

- Acquisition and parsing are now two different runtime concerns and must not overwrite each other's health view.

Alternatives considered:

- Extend `collector_status` with acquisition fields only: rejected because it blurs two separate operational layers.

### 10. Extend source metadata and APIs with acquisition fields

The `sources` table and `/api/source` response will be extended with acquisition-facing metadata such as:

- `logMode`
- `remoteHost`
- `remoteSlowLogPath`
- `localSpoolPath`
- `initialPosition`
- `localSpoolMaxBytes`

V3 will add `GET /api/acquisition/status` while preserving:

- `GET /api/source`
- `GET /api/collector/status`
- existing overview and fingerprint routes

Why this decision:

- It keeps acquisition concerns explicit and lets the UI fetch operational context without changing every analysis payload.

Alternatives considered:

- Hide acquisition fields behind the collector status route only: rejected because source configuration and acquisition runtime are different categories of data.

## Risks / Trade-offs

- [SSH configuration is stricter than local-file mode] -> Provide clear docs and examples for private-key and known-hosts setup.
- [Remote Linux/OpenSSH assumptions may surprise some users] -> Document explicitly that V3 remote acquisition supports Linux/OpenSSH sources only.
- [Remote rotation may still drop unread archived tail content] -> Preserve the pragmatic current-file-only boundary and document it explicitly.
- [Default-first-sync behavior may be unexpected] -> Make `SSO_SOURCE_INITIAL_POSITION=end` explicit in docs and surface it in source metadata.
- [Spool truncation logic can reset parser offsets incorrectly if implemented carelessly] -> Only truncate when parser offset equals spool EOF, and update the parser checkpoint atomically with truncation.
- [Spool growth can outrun parsing under heavy remote write load] -> Enforce `SSO_SOURCE_LOCAL_SPOOL_MAX_BYTES` and surface blocked/degraded acquisition state clearly.
- [Acquisition and parser status can diverge and confuse operators] -> Expose both statuses explicitly in API/UI rather than merging them into one ambiguous health signal.
- [New acquisition tables and source metadata fields add migration work] -> Keep automatic schema migration in the analysis database and reuse the V2 source-aware pattern.

## Migration Plan

1. Extend configuration loading with source log mode, SSH fields, initial-position, and local spool limit fields while keeping `local_file` as the default.
2. Extend `sources` schema with acquisition metadata fields and add `acquisition_checkpoints` plus `acquisition_status`.
3. Add an acquisition service that pulls remote bytes into a local spool file in `ssh_pull` mode and no-ops in `local_file` mode.
4. Update the collector loop order to: resolve source -> acquire into spool/local file -> parse local file -> update parser and acquisition statuses -> compact fully consumed spool when applicable.
5. Add acquisition APIs and update the UI to display acquisition context separately from parser collector health.
6. Update docs, examples, and onboarding steps for remote Linux/OpenSSH slow-log access, first-sync behavior, and local spool handling.

Rollback strategy:

- Switch `SSO_SOURCE_LOG_MODE` back to `local_file`.
- Stop using remote acquisition fields and spool directories.
- Ignore the new acquisition tables if rolling back to V2 behavior.

## Open Questions

- No blocking product questions remain. Implementation may still choose the exact SSH execution strategy (`tail`/`dd`/`cat` with offsets) as long as it preserves byte-accurate checkpointing and the V3 transport contract.
