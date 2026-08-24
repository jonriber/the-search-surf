# 0013 — Enforce user ownership with PostgreSQL row-level security

- Status: Accepted
- Date: 2026-08-24
- Owners: project maintainers

## Context

Application-layer owner filters are necessary but easy to omit from one query. Precise private-spot coordinates make a cross-user read materially harmful. The schema therefore needs a defense-in-depth authorization boundary that remains useful when bootstrap mode is replaced by real authentication.

## Decision

Use PostgreSQL row-level security (RLS) on user-owned tables in addition to application authorization:

- the runtime database role does not own protected tables and does not have `BYPASSRLS`;
- every user-owned row contains a non-null `owner_id`;
- each application use-case transaction sets a transaction-local `app.principal_id` from trusted server context;
- policies compare `owner_id` with the transaction-local principal and deny access when the setting is absent or invalid;
- composite keys and foreign keys preserve same-owner relationships between aggregates;
- migration and narrowly scoped operational roles are separate from the runtime role.

RLS is a backstop, not the primary domain authorization model. Application use cases still authorize intent and repositories still express owner-scoped operations.

## Alternatives considered

- Application filters only: rejected because a missed predicate can expose another user's complete row.
- Schema per user: rejected because migrations, pooling, and cross-user administration become operationally expensive.
- Database per user: rejected because it does not fit a future hosted deployment and complicates provider ingestion and aggregate operations.
- Add RLS only when the second user exists: rejected because retrofitting transaction scoping after repositories are established is costly and error-prone.

## Consequences

### Positive

- Accidental unscoped SQL fails closed for protected tables.
- Ownership rules are integration-testable at the database boundary.
- The database model remains viable when authentication changes.

### Negative

- User-data operations must run inside a correctly initialized transaction.
- Connection-pool and policy mistakes can cause surprising empty results or denied writes.
- Administrative and migration workflows require explicitly different roles.

## Security implications

The principal setting must be transaction-local so pooled connections cannot leak identity between requests. The runtime role must not be able to disable or bypass RLS. Integration tests must prove absent-principal denial and cross-principal isolation for reads, inserts, updates, deletes, and joins.

## Operational implications

Database health checks use non-sensitive operations that do not require a principal. Support procedures needing user data use audited, time-bounded administrative access rather than granting the application role broader permissions.
