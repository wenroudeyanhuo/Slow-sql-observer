## ADDED Requirements

### Requirement: Source metadata endpoint
The system MUST provide an endpoint that returns metadata for the active source.

#### Scenario: Return current source metadata
- **WHEN** a client requests the source metadata endpoint
- **THEN** the API MUST return the active source identity, slow log path, description when configured, and source DB metadata when available

### Requirement: Collector runtime status endpoint
The system MUST provide an endpoint that returns runtime status for the active source.

#### Scenario: Return current collector runtime state
- **WHEN** a client requests the collector status endpoint
- **THEN** the API MUST return collector state, source accessibility state, last successful ingest time, last checkpoint offset, last file identity, and latest error details for the active source

### Requirement: Preserve existing analysis endpoint compatibility
The system MUST keep the existing overview, fingerprint list, fingerprint detail, and sample-record endpoints compatible while V2 source/status APIs are added.

#### Scenario: Continue serving overview metrics
- **WHEN** a client requests the overview endpoint in V2
- **THEN** the API MUST still return the existing overview analysis data without requiring source identifiers in the route

#### Scenario: Continue serving fingerprint drill-down routes
- **WHEN** a client requests fingerprint list, detail, or sample-record endpoints in V2
- **THEN** the API MUST continue to serve those routes for the active source without changing their basic route shape
