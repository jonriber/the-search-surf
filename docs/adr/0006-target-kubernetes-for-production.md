# 0006 — Target Kubernetes for production

- Status: Accepted
- Date: 2026-08-23
- Owners: project maintainers

## Context

The homelab currently runs Docker services, while this project intentionally aims to exercise container orchestration and create a portable deployment target.

## Decision

Use Docker Compose for local development and Kubernetes, likely k3s, as the homelab production target.

## Alternatives considered

- Docker Compose in production: operationally simpler but does not satisfy the orchestration learning objective.
- Managed Kubernetes: unnecessary cost and external dependency for the initial personal deployment.

## Consequences

### Positive

- Declarative scheduling, health management, resource controls, and a transferable operational model.

### Negative

- Controllers, networking, storage, upgrades, and recovery add significant operational complexity.

## Security implications

Workloads require restricted service accounts, security contexts, network policies, and private administrative access.

## Operational implications

Kubernetes is target state, not current state. Cluster installation and recovery require their own homelab plan.
