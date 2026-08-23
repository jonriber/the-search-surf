# 0008 — Use Argo CD for GitOps delivery

- Status: Accepted
- Date: 2026-08-23
- Owners: project maintainers

## Context

The Kubernetes environment needs auditable reconciliation, drift visibility, and rollback without exposing cluster credentials to public CI.

## Decision

Use standard non-HA Argo CD inside the homelab cluster. Git records desired state; Argo CD alone reconciles application resources into the cluster.

## Alternatives considered

- Flux: a strong, lighter GitOps alternative, but Argo CD's UI better supports the project's operational learning goal.
- GitHub Actions deployment: rejected because it pushes from CI and requires cluster credentials outside the cluster.

## Consequences

### Positive

- Pull-based delivery, drift detection, visible reconciliation, and Git-audited rollback.

### Negative

- Another privileged controller must be installed, secured, monitored, and upgraded.

## Security implications

The UI remains private; AppProjects restrict sources, destinations, and resource kinds; repository credentials are least privilege.

## Operational implications

Argo CD must not compete with CI or another controller as a writer of the same desired state.
