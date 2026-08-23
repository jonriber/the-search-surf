# 0009 — Separate application and environment state

- Status: Accepted
- Date: 2026-08-23
- Owners: project maintainers

## Context

Application source should be public, while the homelab repository already documents private operations and must not expose topology or deployment identifiers.

## Decision

Keep generic application source and packaging in this public repository. Store homelab-specific Argo CD applications and values under the private `jonatas-dev-kit/homelab` hierarchy.

## Alternatives considered

- Environment state in the public repository: rejected due to disclosure and coupling.
- A new private GitOps repository: unnecessary until the existing repository's scope or access model becomes a constraint.

## Consequences

### Positive

- Clear visibility boundary and independent application/environment history.

### Negative

- Releases require a controlled update across two repositories.

## Security implications

The private repository is not a secret vault; credentials remain encrypted or external and operational identifiers are still minimized.

## Operational implications

Trusted release automation may propose a digest update, but deployment begins only from a committed environment-state change.
