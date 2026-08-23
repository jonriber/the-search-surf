# 0002 — Build a modular monolith

- Status: Accepted
- Date: 2026-08-23
- Owners: project maintainers

## Context

The domain contains distinct capabilities, but initial scale, ownership, and availability requirements do not justify distributed services.

## Decision

Build one versioned Go codebase with explicit domain modules and separate API and worker entry points. Preserve module boundaries without introducing network boundaries.

## Alternatives considered

- Microservices: rejected because deployment, consistency, observability, and failure costs exceed current benefits.
- Unstructured monolith: rejected because it would obscure ownership and hinder later extraction.

## Consequences

### Positive

- Simple transactions, local development, testing, and refactoring with explicit internal boundaries.

### Negative

- Modules share a release cadence and cannot scale independently until extracted.

## Security implications

Authorization must remain consistent across modules; internal calls are not automatically trusted.

## Operational implications

The API and worker may scale separately while sharing release artifacts and database infrastructure.
