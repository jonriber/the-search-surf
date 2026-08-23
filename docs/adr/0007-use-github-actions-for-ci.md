# 0007 — Use GitHub Actions for CI

- Status: Accepted
- Date: 2026-08-23
- Owners: project maintainers

## Context

The source repository is public on GitHub and needs automated validation, security analysis, and artifact production.

## Decision

Use GitHub Actions for continuous integration and trusted release builds. Public pull requests run only on GitHub-hosted runners without secrets or homelab connectivity.

## Alternatives considered

- Woodpecker CI: credible open-source and self-hosted alternative, deferred to avoid operating CI before necessary.
- Tekton: rejected for now because it would make the application cluster part of the build trust boundary.

## Consequences

### Positive

- Native repository integration and no initial CI control-plane operation.

### Negative

- Dependence on GitHub's hosted service and workflow semantics.

## Security implications

Actions will be pinned to immutable revisions, permissions minimized, and untrusted code isolated from persistent self-hosted runners.

## Operational implications

CI validates and publishes; it never applies Kubernetes resources directly.
