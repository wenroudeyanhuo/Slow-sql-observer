## ADDED Requirements

### Requirement: Conservative SQL normalization
The system SHALL normalize common SQL parameter variation into a stable template while preserving structural differences between distinct statements.

#### Scenario: Replace scalar literals
- **WHEN** two SQL statements differ only by numeric, string, datetime, or hexadecimal literal values
- **THEN** the normalized SQL output MUST replace those literal values with placeholders so both statements normalize to the same template

#### Scenario: Preserve structural differences
- **WHEN** two SQL statements differ by column usage, predicate structure, statement type, or table structure rather than only by literal values
- **THEN** the normalized SQL output MUST remain different for those statements

### Requirement: Normalize common list and batch shapes
The system SHALL normalize common collection-shaped parameter differences that would otherwise fragment similar fingerprints.

#### Scenario: Collapse IN-list length differences
- **WHEN** SQL statements differ only by the number or value of items inside an `IN (...)` predicate
- **THEN** the normalized SQL output MUST collapse those variants into the same template

#### Scenario: Collapse batch VALUES count differences
- **WHEN** `INSERT ... VALUES` statements differ only by the number of value groups provided for the same column list
- **THEN** the normalized SQL output MUST collapse those variants into the same template

### Requirement: Stable fingerprint identity
The system SHALL compute a repeatable fingerprint hash from normalized SQL so structurally equivalent statements map to the same fingerprint identity.

#### Scenario: Reuse the same fingerprint for equivalent normalized SQL
- **WHEN** multiple parsed records produce the same normalized SQL text
- **THEN** the system MUST assign the same fingerprint hash and fingerprint record to each of them

#### Scenario: Produce different fingerprints for different templates
- **WHEN** two parsed records produce different normalized SQL templates
- **THEN** the system MUST assign different fingerprint identities to those records

### Requirement: Fingerprint metadata extraction
The system SHALL derive lightweight metadata for each fingerprint that is useful for querying and display.

#### Scenario: Identify common SQL statement type
- **WHEN** the normalized SQL begins with a supported statement family such as `SELECT`, `INSERT`, `UPDATE`, or `DELETE`
- **THEN** the system MUST store that statement family as the fingerprint SQL type

#### Scenario: Allow unknown main table metadata
- **WHEN** the system cannot confidently determine the main table name for a fingerprint using the V1 heuristic rules
- **THEN** it MUST still persist the fingerprint and MAY leave the main table metadata empty

### Requirement: Aggregate fingerprint statistics
The system SHALL maintain aggregate statistics for each fingerprint based on all successfully persisted records associated with that fingerprint.

#### Scenario: Update count and timing aggregates
- **WHEN** a new slow query record is persisted for an existing fingerprint
- **THEN** the system MUST update total count, total query time, average query time, maximum query time, and last-seen time for that fingerprint

#### Scenario: Update row-volume aggregates
- **WHEN** a new slow query record is persisted for an existing fingerprint
- **THEN** the system MUST update total, average, and maximum values for rows sent and rows examined using that record's metrics
