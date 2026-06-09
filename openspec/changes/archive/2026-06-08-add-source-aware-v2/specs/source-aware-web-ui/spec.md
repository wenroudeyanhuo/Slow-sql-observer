## ADDED Requirements

### Requirement: Display source identity and collector status
The web UI MUST display the active source identity and collector runtime status on the dashboard and fingerprint pages.

#### Scenario: Show source context on the dashboard
- **WHEN** the dashboard page loads successfully
- **THEN** it MUST display the active source instance name, slow log path, and collector status summary

#### Scenario: Show source context on fingerprint drill-down pages
- **WHEN** a user views the fingerprint list or fingerprint detail page
- **THEN** those pages MUST also display the active source identity and collector status summary

### Requirement: Distinguish empty data from source or collector failures
The web UI MUST present different states for "no data yet" and for degraded or failed collection.

#### Scenario: Show an empty-state message for a healthy but empty source
- **WHEN** the source is reachable and collector status is healthy but no slow query records exist yet
- **THEN** the UI MUST show a no-data-yet message instead of a source failure message

#### Scenario: Show an error-state message for source or collector failure
- **WHEN** the collector status reports degraded or failed collection
- **THEN** the UI MUST display an operational warning or error summary that distinguishes it from a normal empty state
