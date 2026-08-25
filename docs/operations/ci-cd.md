# CI/CD Contract

## Ownership boundary

GitHub Actions owns continuous integration: it decides whether source and candidate artifacts satisfy repository policy. Argo CD will own continuous delivery: it will reconcile approved desired state from a private environment repository into the homelab cluster.

They do not collide because they do not write the same state:

```text
Pull request ──► GitHub Actions ──► merge / release image
                                           │
                                           ▼
                                private environment repository
                                           │
                                           ▼
                                      Argo CD ──► Kubernetes
```

The application repository must never give a public pull-request workflow Kubernetes credentials. Argo CD pulls desired state from Git and is the only deployment reconciler.

## Pull-request admission

The `CI` and `CodeQL` workflows run on pull requests. Their checks are designed to become required branch-protection contexts after GitHub has observed their first successful execution.

| Check | Enforcement |
| --- | --- |
| Backend quality | format, vet, strict lint, known reachable vulnerabilities, unit and race tests, build |
| Frontend quality | format, lint, coverage, type/build validation, high-or-critical npm advisories |
| Database quality | clean PostGIS startup, forward migration, least-privilege roles, spatial schema, constraints, and forced row-level-security isolation under the race detector |
| Contract quality | repository hygiene, OpenAPI validation, Compose model validation |
| Container quality | production build, migration ordering, health smoke test, API/PWA/migrator SBOM generation, and high-or-critical Trivy findings across application and PostGIS images |
| Dependency review | blocks newly introduced dependencies with high-or-critical known vulnerabilities |
| CodeQL Analyze (Go and JavaScript/TypeScript) | independent semantic analysis using each language's supported build mode |

All third-party GitHub Actions are pinned to immutable commit revisions, workflow permissions are read-only by default, jobs have timeouts, and redundant runs are cancelled. Dependabot opens grouped updates for Go, npm, Actions, both Docker build contexts, and the Compose database image.

The scanners answer different questions. `govulncheck` correlates Go advisories with reachable call paths; npm audit evaluates the JavaScript dependency graph; dependency review evaluates the change introduced by a pull request; Trivy evaluates the built runtime images; CodeQL looks for source-level vulnerability patterns. No single scanner substitutes for the others.

## Continuous delivery status

CD is intentionally not active in this foundation slice. The required sequence is:

1. bootstrap and secure Kubernetes in the homelab;
2. install Argo CD with private-only administration and constrained projects;
3. define application manifests in the private environment repository;
4. add a trusted release workflow that publishes signed, immutable images to GHCR;
5. update the environment repository by image digest through a reviewed change;
6. verify Argo CD reconciliation, health reporting, rollback, and drift detection.

Until those controls are exercised, Compose is the only verified deployment path and GitHub Actions performs CI rather than CD.

## Failure policy

Do not weaken a gate merely to unblock a pull request. Fix the finding, upgrade the affected toolchain or dependency, or document a narrowly scoped exception with evidence, expiry, and ownership. A flaky gate is a defect in the delivery system and should be tracked like application code.
