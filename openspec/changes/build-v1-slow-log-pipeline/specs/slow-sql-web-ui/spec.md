## ADDED Requirements

### Requirement: Overview dashboard page
The web UI SHALL provide a dashboard page that presents high-level V1 slow SQL analysis metrics.

#### Scenario: Display summary metrics
- **WHEN** the dashboard page loads successfully
- **THEN** it MUST display overview metrics returned by the overview API together with a small set of top fingerprint results

#### Scenario: Handle an empty dataset
- **WHEN** the dashboard page loads before any slow query data has been ingested
- **THEN** it MUST render a clear empty state instead of failing or showing misleading metrics

### Requirement: Fingerprint ranking page
The web UI SHALL provide a page for browsing fingerprint results from the query API.

#### Scenario: Display ranked fingerprint results
- **WHEN** the fingerprint list page loads successfully
- **THEN** it MUST display paginated fingerprint results with normalized SQL and key aggregate metrics suitable for ranking slow SQL patterns

#### Scenario: Preserve API-driven browsing controls
- **WHEN** a user changes supported list controls such as paging, sorting, or filtering
- **THEN** the page MUST request updated results from the list API and render the returned state

### Requirement: Fingerprint detail page
The web UI SHALL provide a page for inspecting one fingerprint in detail.

#### Scenario: Display fingerprint metadata and aggregate statistics
- **WHEN** a user opens the detail page for an existing fingerprint
- **THEN** the page MUST display the fingerprint's normalized SQL, metadata, aggregate metrics, and last-seen information

#### Scenario: Display sample slow query records
- **WHEN** the detail page requests sample records for the selected fingerprint
- **THEN** it MUST render the returned record list with raw SQL and performance metrics so the user can inspect concrete examples

#### Scenario: Handle missing fingerprint results
- **WHEN** a user navigates to a fingerprint detail route whose identifier is not found by the API
- **THEN** the page MUST display a not-found state instead of crashing
