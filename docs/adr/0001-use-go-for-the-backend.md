# 0001 — Use Go for the backend

- Status: Accepted
- Date: 2026-08-23
- Owners: project maintainers

## Context

The backend will expose HTTP APIs, ingest forecasts concurrently, execute scheduled work, apply deterministic domain rules, and run in resource-conscious containers. The project also aims to deepen production engineering and operational maturity.

## Decision

Use Go for the production backend. Keep Python available for offline data exploration or model training when a concrete analytical use case appears.

## Alternatives considered

- Python: faster scientific and ML experimentation, but a larger runtime and weaker default compile-time guarantees.
- A mixed Go/Python production architecture: rejected until independent runtime requirements justify a second stack.

## Consequences

### Positive

- Strong compile-time contracts, efficient concurrency, small runtime images, and predictable resource use.

### Negative

- More explicit code and a weaker scientific-computing ecosystem than Python.

## Security implications

Smaller runtime images reduce attack surface, but memory safety and input validation remain application responsibilities.

## Operational implications

API and worker processes will be built from the same Go modules and released as immutable OCI images.
