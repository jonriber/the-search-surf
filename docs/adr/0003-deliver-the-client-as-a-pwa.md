# 0003 — Deliver the client as a PWA

- Status: Accepted
- Date: 2026-08-23
- Owners: project maintainers

## Context

The primary experience is mobile, but the project should retain one web delivery path and avoid native-store overhead during the MVP.

## Decision

Build a mobile-first React and TypeScript progressive web application with an installable manifest, service worker, and deliberate offline behavior.

## Alternatives considered

- Native iOS and Android applications: deferred because they create multiple release surfaces.
- Cross-platform native wrapper: retained as a future option when device APIs or store distribution justify it.

## Consequences

### Positive

- One deployable client, link-based distribution, and a native-like installed experience.

### Negative

- Browser-engine restrictions on device APIs, notifications, storage, and background execution.

## Security implications

Cached private data requires deliberate storage, expiration, logout, and content-security policies.

## Operational implications

Service-worker updates and backward API compatibility become part of release planning.
