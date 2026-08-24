# 0014 — Use forward-only SQL migrations and use-case transactions

- Status: Accepted
- Date: 2026-08-24
- Owners: project maintainers

## Context

Database changes must be reproducible across local, CI, and GitOps environments. Automatic schema mutation during API startup couples availability to privileged credentials and makes rollout behavior difficult to reason about. Transaction ownership also needs a stable boundary: repositories cannot independently commit when one use case changes multiple aggregates.

## Decision

- Store ordered, versioned SQL migrations with the application source.
- Execute migrations as an explicit deployment or development operation using a role separate from the API runtime role.
- Prefer forward fixes over destructive down migrations.
- Use expand, migrate, and contract phases for changes that span application releases.
- Make the application use-case layer own transaction begin, principal scoping, commit, and rollback.
- Pass a narrow transaction-capable database interface to concrete repositories; repositories never commit independently.
- Keep domain packages independent from SQL, migration, and transaction libraries.

The concrete migration tool and PostgreSQL driver are implementation choices for issue #18. They must preserve this contract and be pinned and documented when selected.

## Alternatives considered

- Run migrations automatically when the API starts: rejected because every replica would need schema-owner privileges and concurrent startup could interfere with rollout.
- Require a reversible down migration for every change: rejected because data-loss operations are often not safely reversible; a tested forward repair and backup restore are more honest recovery mechanisms.
- Let each repository open transactions: rejected because atomic application behavior may span repositories and nested ownership obscures rollback semantics.
- Introduce a generic repository or unit-of-work framework: rejected because it hides SQL capabilities and creates abstractions before repeated domain behavior exists.

## Consequences

### Positive

- Deployment ordering and privileges are explicit.
- Multi-aggregate invariants can be committed atomically.
- SQL remains reviewable and PostGIS capabilities remain available.

### Negative

- Some releases require compatibility across old and new application versions.
- Rollback may require a new migration instead of reversing history.
- Use cases that touch protected data have transaction ceremony even for simple reads.

## Security implications

The runtime API cannot alter schema. Migration credentials are never available to public pull-request code or long-running application containers. Transactions establish the trusted principal setting before protected queries execute.

## Operational implications

CI applies every migration to an empty PostGIS database and exercises upgrade paths once released schemas exist. GitOps runs migrations as an explicit, observable step before incompatible workloads become ready. Backup restoration is tested separately from ordinary application rollback.
