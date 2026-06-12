## MODIFIED Requirements

### Requirement: Threshold-aware analysis endpoints
The system SHALL apply the effective analysis threshold consistently to the overview, fingerprint list, fingerprint detail, fingerprint records, dashboard trends, and fingerprint trends views so that users can investigate the same filtered slice of data end to end.

#### Scenario: Fingerprint list only ranks above-threshold statements
- **WHEN** a fingerprint has only records below the effective analysis threshold
- **THEN** that fingerprint SHALL not appear in the fingerprint list response

#### Scenario: Fingerprint records respect the same threshold
- **WHEN** a client opens a fingerprint detail flow with an effective analysis threshold
- **THEN** the fingerprint records response SHALL include only records whose `query_time_sec` is greater than or equal to that threshold

#### Scenario: Dashboard trends respect the same threshold
- **WHEN** a client requests dashboard trends with an effective analysis threshold
- **THEN** the trend buckets SHALL include only records whose `query_time_sec` is greater than or equal to that threshold

#### Scenario: Fingerprint trends respect the same threshold
- **WHEN** a client requests fingerprint trends with an effective analysis threshold
- **THEN** the trend buckets SHALL include only records for that fingerprint whose `query_time_sec` is greater than or equal to that threshold
