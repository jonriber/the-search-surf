# The Search

The Search is an open-source, self-hostable surf intelligence PWA that combines marine and weather forecasts, local spot knowledge, and surfer preferences to produce explainable, personalized surf recommendations.

## Status

The project is in its foundation phase. A production-shaped vertical slice now proves the installable PWA, same-origin API boundary, health and release contracts, hardened containers, forward-only PostGIS migrations, user-data isolation, and pull-request quality gates. Surf-domain features are the next milestone.

## Product objective

The Search should answer a practical question:

> Given this surf spot, this surfer, and this time window, is it worth going?

It will turn forecast data into recommendations that are:

- personal to the surfer's experience and preferences;
- specific to a surf spot rather than only a geographic coordinate;
- explainable and reproducible;
- honest about uncertainty and data quality;
- useful on mobile, including degraded connectivity scenarios.

## Architectural direction

- React and TypeScript progressive web application
- Go modular-monolith backend
- PostgreSQL with PostGIS
- Deterministic recommendation engine
- AI used for explanation, not as the decision authority
- OCI containers
- Kubernetes target runtime
- GitHub Actions for continuous integration
- Argo CD for GitOps continuous delivery

The decisions and their trade-offs are recorded in [`docs/adr`](docs/adr/README.md).

## Core principles

1. Safety information is never reduced to a single quality score.
2. Forecasts, transformations, and recommendations retain provenance.
3. Recommendations are reproducible from versioned inputs and rules.
4. Private surf spots and user data are private by default.
5. The application remains useful without an LLM.
6. Operational complexity must solve a concrete problem or a stated learning objective.

## Repository map

```text
api/openapi/     Versioned HTTP contract
backend/         Go API and production container
database/        PostgreSQL role bootstrap and database initialization
frontend/        React and TypeScript PWA and production container
docs/
  adr/           Architectural decision records
  architecture/  System boundaries and technical direction
  operations/    Development, CI/CD, and runtime guidance
  product/       Vision, scope, and domain language
  security/      Threat model and security guidance
.github/          Repository governance and automation
scripts/          Repeatable operational checks
```

## Run the foundation slice

Install the pinned Go and Node.js versions, then run the local quality contract:

```sh
npm ci --prefix frontend
make verify
make database-test
```

To exercise the production containers and the PWA-to-API proxy:

```sh
make compose-smoke
```

For an interactive local stack, run `make compose-up` and open <http://127.0.0.1:8081>. The stack waits for PostgreSQL and applies pending migrations before starting the API. Stop it with `make compose-down`. See [database operations](docs/operations/database.md) before resetting data or changing migrations.

## Contributing

Every change starts from an issue, preserves the documented boundaries, and passes the same verification locally and in CI. See [`CONTRIBUTING.md`](CONTRIBUTING.md), the [development workflow](docs/operations/development.md), the [CI/CD contract](docs/operations/ci-cd.md), and [`SECURITY.md`](SECURITY.md).

## License

Licensed under the Apache License 2.0. See [`LICENSE`](LICENSE).
