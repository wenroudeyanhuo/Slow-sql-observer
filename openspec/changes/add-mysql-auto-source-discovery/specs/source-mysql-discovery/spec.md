## ADDED Requirements

### Requirement: Discover source MySQL slow-log configuration
The system MUST introduce a source-discovery stage for `mysql_auto` mode that connects to the observed MySQL instance and inspects its slow-query logging configuration before remote acquisition begins.

#### Scenario: Discovery reads slow-log configuration successfully
- **WHEN** the collector starts a cycle for a source configured with `SSO_SOURCE_MODE=mysql_auto`
- **THEN** the system SHALL connect to the source MySQL and discover whether slow query logging is enabled, which `log_output` values are active, and the configured slow-log file path when available

### Requirement: Persist discovery metadata for the active source
The system MUST persist source discovery metadata independently from parser collector state.

#### Scenario: Discovery metadata is stored after a successful inspection
- **WHEN** source discovery succeeds
- **THEN** the system SHALL persist the active source id, discovery state, discovered `log_output`, discovered slow-log file path when applicable, discovered source version, discovered source host identity when available, and the discovery timestamp

### Requirement: Surface disabled slow logging clearly
The system MUST surface a blocked or degraded state when the observed MySQL source has slow query logging disabled.

#### Scenario: Slow query logging is disabled on the source
- **WHEN** source discovery finds that slow query logging is disabled
- **THEN** the system SHALL persist a non-healthy discovery state and an operator-facing diagnostic message explaining that the source MySQL slow query log is not enabled
