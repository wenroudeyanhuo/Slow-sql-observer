## ADDED Requirements

### Requirement: Overview metrics endpoint
The system SHALL provide an overview endpoint that summarizes the current V1 slow SQL analysis state.

#### Scenario: Return dashboard overview data
- **WHEN** a client requests the overview endpoint
- **THEN** the API MUST return summary metrics including total records, total fingerprints, aggregate query-time information, the last ingested time, and a small set of top fingerprints

### Requirement: Fingerprint list endpoint
The system SHALL provide a fingerprint list endpoint that supports browsing ranked fingerprint results.

#### Scenario: Return paginated fingerprint results
- **WHEN** a client requests the fingerprint list endpoint with pagination parameters
- **THEN** the API MUST return a paginated list of fingerprint results containing fingerprint identity, normalized SQL, lightweight metadata, and aggregate statistics

#### Scenario: Apply sort and filter options
- **WHEN** a client requests the fingerprint list endpoint with supported sort or filter parameters
- **THEN** the API MUST apply those options to the fingerprint result set before returning the response

### Requirement: Fingerprint detail endpoint
The system SHALL provide a fingerprint detail endpoint for inspecting one fingerprint and its aggregate state.

#### Scenario: Return fingerprint detail
- **WHEN** a client requests the detail endpoint for an existing fingerprint identifier
- **THEN** the API MUST return the fingerprint hash, normalized SQL, SQL type, main table metadata when available, first-seen time, last-seen time, and aggregate statistics

#### Scenario: Reject an unknown fingerprint identifier
- **WHEN** a client requests the detail endpoint for a fingerprint identifier that does not exist
- **THEN** the API MUST return a not-found response

### Requirement: Sample record endpoint
The system SHALL provide a sample-record endpoint for retrieving raw slow query examples associated with a fingerprint.

#### Scenario: Return paginated sample records
- **WHEN** a client requests sample records for an existing fingerprint identifier
- **THEN** the API MUST return a paginated list of matching raw records including occurrence time, database name, user, client host, SQL text, and performance metrics

#### Scenario: Sort sample records by supported fields
- **WHEN** a client requests sample records using a supported sort field such as occurrence time or query time
- **THEN** the API MUST return the records ordered by that field
