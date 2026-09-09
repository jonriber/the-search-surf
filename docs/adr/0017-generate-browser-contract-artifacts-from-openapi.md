# 0017 — Generate browser contract artifacts from OpenAPI

- Status: Accepted
- Date: 2026-08-27
- Owners: project maintainers

## Context

The browser client now consumes profile, private-spot, and favorite APIs. Handwritten request and response types can silently drift from the canonical OpenAPI document, while TypeScript types alone cannot protect the client from an incompatible response at runtime. A fully generated SDK would reduce request boilerplate but would also make timeout behavior, public error semantics, and privacy-sensitive request handling less explicit.

OpenAPI `int64` fields introduce an additional boundary concern. JSON has no integer-width type, the generated TypeScript model uses `number`, and the generated Zod schema conservatively produces `bigint`. Optimistic-lock versions currently cross the API as JSON numbers and must remain serializable in browser state and request bodies.

## Decision

- Treat `api/openapi/the-search.yaml` as the browser contract source of truth.
- Generate TypeScript DTOs and Zod runtime schemas with a pinned `@hey-api/openapi-ts` version; commit the generated artifacts for reviewable API changes.
- Keep a small handwritten fetch adapter that owns same-origin routing, timeouts, stable public error categories, response validation, and `204` handling.
- Validate generated artifacts for drift in the standard frontend verification command.
- Reject unsafe JavaScript integer versions before generated validation, then normalize validated `int64` versions from `bigint` to `number` at the adapter boundary. Requests only accept positive safe-integer versions.
- Pin a transitive `js-yaml` override until the generator no longer resolves to the vulnerable release; dependency audit remains the enforcement mechanism.

## Alternatives considered

- Handwrite DTOs and validators: rejected because the browser contract could drift independently from the API specification.
- Generate a complete HTTP SDK: rejected for this slice because transport failure semantics and privacy controls are important product behavior and should remain explicit and directly tested.
- Expose `bigint` versions throughout the application: rejected because JSON serialization would require special handling and the API currently emits JSON numbers.
- Force-install `openapi-typescript`: rejected because its current peer dependency excludes the repository's TypeScript 6 toolchain.
- Downgrade the generator to remove the dependency override: rejected because the compatible older generator also resolves to a vulnerable `js-yaml` release.

## Consequences

### Positive

- Contract changes become visible in review and stale generated files fail CI.
- Runtime payload validation prevents incompatible server data from entering UI state.
- Transport behavior remains small, explicit, and independently testable.

### Negative

- Generated artifacts add repository churn when the OpenAPI document changes.
- The adapter contains a documented normalization step for `int64` versions.
- The temporary dependency override must be reviewed and removed after the upstream dependency is fixed.

## Security implications

The adapter exposes stable client messages rather than proxy bodies or unvalidated server payloads. It does not persist profile, coordinate, favorite, or mutation data. Dependency audit protects the generator toolchain, even though generation only consumes the repository-owned specification.

## Operational implications

Developers run `npm run api:generate` after contract changes. `npm run verify` regenerates into an isolated temporary directory and fails when committed output differs. Updating the generator requires reviewing generated diffs, the `int64` normalization assumption, and whether the `js-yaml` override is still necessary.
