# 0015 — Use pgx, goose, and disposable PostGIS integration tests

- Status: Accepted
- Date: 2026-08-25
- Owners: project maintainers

## Context

ADR 0014 defines forward-only SQL migrations and application-owned transactions but deliberately leaves implementation tooling open. The implementation needs native PostgreSQL features, embedded migrations for immutable artifacts, and tests against real PostGIS and row-level-security behavior. Mocking SQL cannot prove role privileges, spatial constraints, migration ordering, or database policy isolation.

The official PostGIS project currently recommends PostgreSQL 18 with PostGIS 3.6 on its stable Debian image. That image is amd64-only, while local development may occur on Apple silicon.

## Decision

- Use `github.com/jackc/pgx/v5` v5.10.0 as the PostgreSQL driver.
- Use pgx native connections and pools for future application persistence adapters.
- Use pgx's `database/sql` bridge only at the migration boundary required by goose.
- Use `github.com/pressly/goose/v3` v3.27.3 as a library inside a small project-owned migration command.
- Embed sequential, forward-only SQL migration files into the migration binary.
- Expose safe migration operations such as validation, status, version, and upgrade; do not expose destructive rollback from the production entry point.
- Use `postgis/postgis:18-3.6` pinned to immutable digest `sha256:8d67cc8fe5f45808d54fe95cc210b05ce6b3ea3682e9a97c36362f3e1b8ff939` for development and CI.
- Run integration tests against a disposable real database orchestrated outside the Go test process.
- Mark Docker-dependent Go tests with the `integration` build tag and require explicit administrative, migration, and runtime test DSNs. An integration run with missing configuration fails rather than silently skipping.

## Test structure

- Unit tests remain beside their package and use fakes only for boundaries the package owns.
- Migration contract tests live beside the migration package and verify embedding, command validation, and error handling without a database where possible.
- Database integration tests live beside the PostgreSQL adapter or migration boundary and use an external test package when black-box behavior is sufficient.
- One disposable database instance may be shared within a test process, but every test creates isolated data and unique principal identifiers.
- CI always starts from an empty database and applies every migration before exercising schema, PostGIS, privileges, row-level security, and idempotency.
- Integration helpers register cleanup immediately and never rely on test ordering.

## Alternatives considered

- `database/sql` for all application access: rejected because pgx exposes PostgreSQL types, batching, notifications, copy, and pool behavior without a generic compatibility layer.
- The goose CLI installed separately on every runtime: rejected because an embedded project binary keeps migration version, SQL files, driver, and release artifact aligned.
- `golang-migrate`: credible, but goose's optional down sections and embedded-library workflow fit the forward-only decision with less wrapper behavior.
- Atlas: deferred because schema-diff and hosted workflow capabilities add operational surface not needed for the first explicit SQL model.
- Testcontainers for Go: deferred because Compose already defines the production-shaped dependency topology, while test-process container orchestration would add a large dependency graph and hide lifecycle behavior from non-Go tooling.
- A custom multi-architecture PostGIS image: deferred because pinning operating-system packages and maintaining a database image is a higher risk than amd64 emulation during current Apple-silicon development.
- Database mocks as the primary persistence tests: rejected because they test expectations about SQL rather than PostgreSQL enforcement.

## Consequences

### Positive

- Production migrations are immutable and travel with the application release.
- Integration tests exercise the actual database engine and extension versions.
- Future adapters retain access to PostgreSQL-native behavior without coupling domain packages to pgx.
- Tool versions and the database image are reproducible.

### Negative

- The migration binary includes both goose and the pgx standard-library bridge.
- Docker is required for database integration tests.
- Apple-silicon developers pay an emulation cost until the official image becomes multi-architecture or the project accepts custom-image ownership.
- The external test orchestrator must reliably clean containers and volumes after failure.

## Security implications

Administrative, migration, and runtime DSNs represent distinct privilege levels and must not be interchangeable in deployment configuration. Test credentials are local-only and cannot be reused outside the disposable topology. The database port binds only to loopback when host access is required.

## Operational implications

PostgreSQL 18 changed the image volume root to `/var/lib/postgresql`; Compose and backup procedures must use that path. Dependabot tracks the Docker tag while digest updates remain reviewed changes. Upgrading PostgreSQL, PostGIS, pgx, or goose requires migration and restore verification, not only compilation.
