# 0004 — Use PostgreSQL and PostGIS

- Status: Accepted
- Date: 2026-08-23
- Owners: project maintainers

## Context

The domain combines transactional user data, time-series forecast snapshots, and geospatial spot and sampling coordinates.

## Decision

Use PostgreSQL as the system of record and PostGIS for geospatial types and queries.

## Alternatives considered

- SQLite: valuable for small local deployments but weakens the target concurrency and geospatial path.
- A separate time-series database: deferred until measured retention or query requirements justify it.

## Consequences

### Positive

- Mature transactions, constraints, indexing, JSON support, and geospatial capabilities in one system.

### Negative

- Stateful operation, migrations, backup testing, and connection management are required.

## Security implications

Database access will use least-privilege roles, encrypted credentials, and no direct public exposure.

## Operational implications

Backup, restore, migration, retention, and extension-version procedures must be tested.
