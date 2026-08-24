# 0012 — Separate private surf spots from forecast points

- Status: Accepted
- Date: 2026-08-24
- Owners: project maintainers

## Context

A named surf break and the coordinate used to query a forecast provider are not the same thing. A provider may use a coarse grid point offshore, different providers may select different cells, and spot-specific transformation rules may combine more than one input. Treating one coordinate as both the user's private location and the provider query key would leak provider concerns into the domain and make provenance ambiguous.

The first product also does not need community discovery or a shared public spot catalog. Adding visibility modes prematurely would complicate ownership and disclosure rules for precise coordinates.

## Decision

Model these concepts separately:

- a `SurfSpot` is a user-owned aggregate describing the break the surfer recognizes;
- a `ForecastPoint` is a provider-specific sampling location used by forecast ingestion;
- a `Favorite` is an owner-scoped relationship between a principal and a surf spot, not a boolean property on the spot;
- spot display coordinates use a PostGIS geography point with SRID 4326;
- forecast points retain provider, model, grid, and selection provenance independently from display coordinates;
- all user-created spots are private in the first release.

Shared catalog spots, publication, and collaboration are absent rather than represented by an unused visibility flag. If introduced later, catalog entities will be modeled separately from private user spots and require a new authorization and privacy decision.

## Alternatives considered

- One coordinate on `SurfSpot` for display and ingestion: rejected because provider grid selection and spot meaning evolve independently.
- A shared spot row plus per-user favorites: deferred because it creates moderation, deduplication, attribution, and secret-spot disclosure concerns before they deliver MVP value.
- A visibility enum on every spot now: rejected because unsupported states invite accidental exposure and untested authorization branches.
- A favorite boolean on the spot: rejected because favorites carry user-specific ordering and preferences and may later reference catalog spots.

## Consequences

### Positive

- Provider adapters can change sampling logic without mutating the user's spot identity.
- Recommendations can preserve the exact forecast input location and selection version.
- Private-by-default behavior has no public state to misconfigure.

### Negative

- Ingestion needs an explicit mapping from spot to provider forecast points.
- Display and forecast coordinates can diverge and need clear UI language and debugging tools.
- Shared spot discovery will require a separate model and migration later.

## Security implications

Exact spot coordinates are sensitive personal data. They are returned only to the authorized owner, excluded from routine logs and telemetry, and protected by encrypted disks and backups. Application-level field encryption is not used initially because it would prevent ordinary PostGIS indexing and spatial queries; revisiting the threat model may change that trade-off.

## Operational implications

Backups contain sensitive coordinates and inherit the same access controls as the live database. Provider sampling changes must be versioned so stored forecast and recommendation provenance remains reproducible.
