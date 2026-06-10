## ADDED Requirements

### Requirement: UI displays acquisition context
The web UI MUST display acquisition mode and acquisition runtime status for the active source on the dashboard and fingerprint drill-down pages.

#### Scenario: Dashboard shows SSH acquisition context
- **WHEN** the active source uses `ssh_pull` mode
- **THEN** the dashboard SHALL show the acquisition mode, remote host, remote slow-log path, local spool path, initial position, spool limit, and acquisition status summary

#### Scenario: Local-file mode remains understandable
- **WHEN** the active source uses `local_file` mode
- **THEN** the UI SHALL show that no remote acquisition transport is active and continue presenting the local parse source context

### Requirement: UI distinguishes acquisition failures from parser failures
The web UI MUST distinguish failures that occur during remote acquisition from failures that occur later in parsing or persistence.

#### Scenario: Acquisition failure is called out explicitly
- **WHEN** acquisition status is degraded or error while parsed historical data still exists
- **THEN** the UI SHALL continue to show available analysis data and separately highlight the acquisition error summary

#### Scenario: Blocked acquisition is called out explicitly
- **WHEN** acquisition status is `blocked`
- **THEN** the UI SHALL prioritize showing the configuration or credential prerequisite problem before analysis freshness messaging

#### Scenario: Parser failure remains a separate signal
- **WHEN** parser collector status is degraded or error after a successful acquisition cycle
- **THEN** the UI SHALL identify that downstream parsing or storage failed instead of reporting the issue as a remote-access problem
