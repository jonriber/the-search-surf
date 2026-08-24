# 0011 — Use a bootstrap principal with explicit ownership

- Status: Accepted
- Date: 2026-08-24
- Owners: project maintainers

## Context

The first deployment serves one person in a private homelab, but the product direction permits multiple users and hosted operation. Omitting ownership because the first runtime is single-user would spread an implicit global-user assumption through domain objects, SQL, caches, and APIs. Implementing complete OIDC or passkey authentication now would delay validation of the surf domain.

Identity and surfer characteristics are also different concepts. A principal establishes who is acting; a surfer profile records experience, equipment, and preferences used by recommendations.

## Decision

Run the first release in bootstrap-principal mode:

- the server resolves every request to one configured, stable principal identifier;
- the principal identifier is established at the trusted HTTP boundary and carried in application context;
- clients never select or submit an owner identifier as authorization evidence;
- every user-owned aggregate and persistence row has a non-null owner identifier;
- repository operations require the acting principal and scope reads and writes to that owner;
- a surfer profile is owned by a principal but does not replace the identity concept.

The bootstrap identifier is not a credential. Network access to the homelab remains the initial authentication perimeter. Before the application is exposed to multiple people or a public network, an authentication adapter must replace the bootstrap resolver and map a verified issuer/subject pair to the internal principal.

## Alternatives considered

- No identity or owner fields until multi-user support: rejected because later remediation would touch every boundary and existing rows would have ambiguous ownership.
- Implement full OIDC immediately: deferred because provider selection, account recovery, session management, and public exposure are not needed to test the first product loop.
- Accept a principal identifier from the PWA: rejected because a caller-controlled identifier is data, not authentication.
- Treat the surfer profile as the authenticated user: rejected because one identity may eventually manage multiple profiles and identity lifecycle must not be coupled to recommendation attributes.

## Consequences

### Positive

- The MVP stays operationally simple without creating ownerless data.
- Authentication can be added at an adapter boundary rather than by rewriting the domain.
- Authorization tests can exercise multiple principals before multi-user login exists.

### Negative

- Anyone who can reach the bootstrap deployment acts as the same principal.
- A stable principal identifier must be generated and preserved across restores.
- Application APIs and tests carry identity context even in single-user mode.

## Security implications

Bootstrap mode is not suitable for an endpoint shared by mutually untrusted users. The API must fail closed when it cannot resolve a principal, redact principal-associated private coordinates from logs, and reject owner identifiers supplied as a shortcut by clients.

## Operational implications

Private environment state stores the stable bootstrap principal identifier. Restores must preserve both that identifier and its database row. Introducing real authentication triggers a threat-model review, an identity-mapping migration, session-management decisions, and explicit bootstrap-mode deprecation criteria.
