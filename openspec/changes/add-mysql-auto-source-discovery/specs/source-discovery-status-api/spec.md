## ADDED Requirements

### Requirement: Expose source discovery metadata through API
The system MUST expose source discovery metadata and the resolved effective acquisition mode through backend APIs for the active source.

#### Scenario: Client requests current source metadata
- **WHEN** a client requests source metadata for the active source
- **THEN** the response SHALL include the configured source mode, discovered slow-log output, discovered slow-log file path when applicable, and the resolved effective acquisition mode

### Requirement: Expose discovery and acquisition status separately from parser status
The system MUST expose discovery status and acquisition status without collapsing them into the existing parser collector status.

#### Scenario: Discovery fails before acquisition begins
- **WHEN** source discovery fails for the active source
- **THEN** the API SHALL expose a discovery error state and message even if the parser collector status still reflects the last known downstream health

#### Scenario: Acquisition fails after discovery succeeds
- **WHEN** source discovery succeeds but the chosen acquisition branch fails
- **THEN** the API SHALL expose acquisition failure details separately from parser collector status

### Requirement: Preserve compatibility for existing analysis APIs
The system MUST keep overview, fingerprint list, fingerprint detail, and sample record APIs compatible while extending source-status responses.

#### Scenario: Existing analysis routes are queried in V4
- **WHEN** a client calls existing overview or fingerprint analysis APIs
- **THEN** the system SHALL preserve their functional compatibility while allowing clients to retrieve discovery and acquisition context through dedicated status metadata
