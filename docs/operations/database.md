# Database Operations

## Runtime and ownership model

The local database is PostgreSQL 18 with PostGIS 3.6. A minimal project image derives from the digest-pinned official Alpine variant, removes its unused privilege-transition helper, and runs directly as the existing PostgreSQL UID/GID 70 with all Linux capabilities dropped. It runs as `linux/amd64`; Apple Silicon development therefore uses Docker's amd64 emulation. This gives every environment the same tested image at the cost of slower startup and migration tests on arm64 workstations.

Three database roles keep privileges separated:

| Role | Responsibility | Privileges |
| --- | --- | --- |
| `postgres` | local bootstrap and recovery administration | superuser; never used by an application process |
| `the_search_migrator` | apply versioned schema migrations | non-superuser; can create objects only in the `the_search` database |
| `the_search_app` | serve application use cases | non-superuser; limited table and sequence access governed by row-level security |

The role initializer runs only when PostgreSQL creates an empty data directory. Changing a password in `.env` does not rotate an existing role. Production credentials must come from the private environment repository's secret-management path, not from this public repository or Kubernetes manifests in plaintext.

## Local lifecycle

Copy `.env.example` to an ignored `.env` only when overriding local Compose defaults. Start the complete stack with:

```sh
make compose-up
```

Compose waits for PostgreSQL, runs all pending migrations once, and starts the API only after migration success. Useful commands are:

```sh
make database-status
make database-migrate
make compose-down
```

The PostgreSQL 18 data volume is mounted at `/var/lib/postgresql`. The database port is loopback-only and can be moved from `5432` with `THE_SEARCH_DATABASE_PORT`.

To irreversibly reset only this project's local Compose data, first stop the stack and explicitly remove its named volumes:

```sh
docker compose down --volumes --remove-orphans
docker compose up --build --detach --wait
```

This deletes the local database. Do not use the reset sequence against an environment that contains data you need.

## Migration policy

Migrations are embedded into the dedicated Go migration binary and use the `the_search_schema_migrations` metadata table. The public command contract intentionally exposes only:

- `up`: atomically apply pending forward migrations;
- `status`: report applied and pending migrations;
- `version`: report the current schema version.

There is no `down` command. Once shared, a migration is immutable; correcting it requires a new forward migration. This avoids pretending that destructive schema changes have a universally safe inverse. Restore from a verified backup when a forward fix cannot preserve data or availability.

Each application request that touches user-owned data begins a transaction, sets `app.principal_id` with transaction-local scope, executes the use case, and commits or rolls back. Forced row-level security fails closed when that context is missing. The API uses only `the_search_app`; the short-lived bootstrap command uses the migrator role to idempotently provision the configured principal before API startup.

## Tests and TDD contract

Run the real database contract with:

```sh
make database-test
```

The harness creates a project-scoped PostGIS container and volume, initializes least-privilege roles, applies migrations from an empty database, runs integration tests with the Go race detector, and removes the container, network, and volume on exit. It uses loopback port `55432` to avoid the interactive stack.

Testing is organized by boundary:

- unit tests live beside their Go package and exercise configuration, command behavior, and adapter assembly through narrow ports;
- migration integration tests live in the external `migrations_test` package, carry the `integration` build tag, and observe only exported behavior plus database contracts;
- `scripts/database-integration.sh` owns infrastructure setup, test-only credentials, DSNs, and cleanup;
- integration tests require explicit DSNs and fail when the harness is absent; they must never silently skip.

Every behavior change follows red-green-refactor. A schema or authorization change is incomplete until a test first demonstrates the missing behavior against a clean database, the smallest migration makes it pass, and both unit and integration suites remain green. Prefer assertions on externally observable invariants—privileges, policies, isolation, constraints, and indexes—over assertions coupled to migration implementation details.

## Backup and restore

Before a risky migration, create a custom-format logical backup from a healthy database and record the image digest and schema version used:

```sh
docker compose exec -T database pg_dump --username postgres --dbname the_search --format=custom > the-search.dump
docker compose run --rm migrate version
```

Validate recovery in an isolated database before relying on the backup. A local restore drill can target a freshly initialized disposable stack:

```sh
docker compose exec -T database pg_restore --username postgres --dbname the_search --clean --if-exists --no-owner --exit-on-error < the-search.dump
docker compose run --rm migrate up
docker compose run --rm migrate status
```

Migration status proves schema compatibility, not data integrity. A recovery drill must also validate representative row counts, ownership isolation, spatial queries, and application readiness against the restored target. `make database-test` independently revalidates the schema and authorization contract against a clean disposable database.

The database dump does not replace separate backup of production role definitions, encrypted secrets, or cluster configuration. Production backup automation, retention, encryption, off-host storage, and scheduled restore drills belong to the private environment repository and are not yet operational in this foundation slice.
