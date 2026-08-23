# 0010 — Publish immutable images to GHCR

- Status: Accepted
- Date: 2026-08-23
- Owners: project maintainers

## Context

Deployments must identify exactly which reviewed source and build output is running. The public repository is already hosted on GitHub.

## Decision

Publish OCI images to GitHub Container Registry with commit and release tags. Kubernetes desired state pins the resulting image digest rather than a mutable tag.

## Alternatives considered

- Docker Hub: capable but adds another identity and distribution dependency.
- Private homelab registry: deferred until availability, storage, or sovereignty requirements justify it.
- Mutable `latest` deployment: rejected because it is not reproducible.

## Consequences

### Positive

- Source, workflow, package, provenance, and deployment remain traceable through GitHub and immutable digests.

### Negative

- Registry availability remains an external dependency and retention must be managed.

## Security implications

Release workflows use minimal package permissions and generate SBOM and provenance metadata.

## Operational implications

Rollback changes the pinned digest in Git and lets Argo CD reconcile the previous artifact.
