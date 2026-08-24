# Architectural Decision Records

ADRs record decisions that materially constrain implementation or operation.

## Statuses

- **Proposed**: under active evaluation;
- **Accepted**: current direction;
- **Deprecated**: retained for history but discouraged;
- **Superseded**: replaced by a later ADR.

Accepted records are historical evidence. When a decision changes, add a new ADR and link both records rather than rewriting the original context.

## Index

- [0001 — Use Go for the backend](0001-use-go-for-the-backend.md)
- [0002 — Build a modular monolith](0002-build-a-modular-monolith.md)
- [0003 — Deliver the client as a PWA](0003-deliver-the-client-as-a-pwa.md)
- [0004 — Use PostgreSQL and PostGIS](0004-use-postgresql-and-postgis.md)
- [0005 — Separate scoring from AI explanation](0005-separate-scoring-from-ai-explanation.md)
- [0006 — Target Kubernetes for production](0006-target-kubernetes-for-production.md)
- [0007 — Use GitHub Actions for CI](0007-use-github-actions-for-ci.md)
- [0008 — Use Argo CD for GitOps delivery](0008-use-argocd-for-gitops-delivery.md)
- [0009 — Separate application and environment state](0009-separate-application-and-environment-state.md)
- [0010 — Publish immutable images to GHCR](0010-publish-immutable-images-to-ghcr.md)
- [0011 — Use a bootstrap principal with explicit ownership](0011-use-a-bootstrap-principal-with-explicit-ownership.md)
- [0012 — Separate private surf spots from forecast points](0012-separate-private-surf-spots-from-forecast-points.md)
- [0013 — Enforce user ownership with PostgreSQL row-level security](0013-enforce-user-ownership-with-postgresql-row-level-security.md)
- [0014 — Use forward-only SQL migrations and use-case transactions](0014-use-forward-only-sql-migrations-and-use-case-transactions.md)

Use [`template.md`](template.md) for new records.
