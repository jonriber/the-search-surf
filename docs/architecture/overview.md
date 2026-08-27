# Architecture Overview

## Context

```text
Forecast providers ──► The Search ──► Surfer
                           │
                           ├──► Notification provider
                           └──► Optional AI explanation provider
```

The Search consumes untrusted external forecast data, applies versioned domain rules, stores recommendations and provenance, and presents results through an installable PWA.

## Implemented foundation slice

```text
Browser / installed PWA ──► unprivileged nginx ──/api/*──► Go API
          │                         │                            │
          └── cached app shell      └── security headers        ├── liveness
                                                              ├── readiness
                                                              └── release identity

PostGIS ──healthy──► forward-only migrator ──completed──► Go API startup
   │                         │
   └── forced RLS            └── one-shot principal bootstrap

PWA ──same origin──► HTTP adapter ──trusted principal──► use cases
                                               │
                                               └── transaction-scoped pgx repositories
```

The browser uses only same-origin `/api` URLs. nginx owns routing at the deployment edge, so the PWA does not embed a homelab address and the API does not need permissive cross-origin rules. The client caches the application shell and the last successfully validated release identity; it does not cache arbitrary API responses.

The API exposes health and release contracts plus ownership-scoped profile, private-spot, and favorite operations. Its OpenAPI description under `api/openapi` is the transport contract. HTTP translation lives under `internal/transport`; server lifecycle and middleware remain under `internal/platform`. Domain and application packages do not import either transport or PostgreSQL adapters.

Bootstrap mode resolves every user-data request to one stable server-configured principal. Request bodies and responses intentionally omit owner identifiers. The API connects with the least-privilege application role, and every use case opens a transaction that sets the principal through transaction-local PostgreSQL state before accessing repositories. Explicit owner predicates express authorization intent; forced RLS is the database backstop.

The [initial domain and data model](domain-model.md) defines explicit principal ownership, private surf spots, favorites, and the separate forecast-ingestion seam. Its first PostGIS migration implements these user-owned tables, spatial indexing, least-privilege runtime grants, and forced row-level security. Database enforcement is a backstop; application use cases remain responsible for authorization intent and transaction scope.

## Target containers

```text
PWA ──HTTP──► Go API ──────► PostgreSQL/PostGIS
                │                    ▲
                ▼                    │
             Worker ─────────────────┘
                │
                ├──► Forecast providers
                └──► Optional AI provider
```

The API and worker are separate runtime processes built from the same modular-monolith codebase. Domain and application logic must not depend on their transport, database, or provider adapters.

## Initial module boundaries

- identity and access;
- surfer profiles;
- surf spots;
- forecast ingestion and normalization;
- recommendation scoring;
- session feedback;
- notifications;
- explanation generation.

Module boundaries are not service boundaries. Extraction requires evidence such as independent scaling, deployment, ownership, or failure-isolation needs.

## Deployment direction

- GitHub Actions currently validates source and local OCI images.
- A future trusted release workflow will publish immutable OCI images by digest.
- A private environment repository records desired homelab state.
- Argo CD reconciles that state into Kubernetes.
- Public CI does not possess Kubernetes credentials.
- Administrative surfaces remain reachable only through the private network.

The current verified runtime is Docker Compose with loopback-only host ports, read-only filesystems, dropped Linux capabilities, and non-root processes. Kubernetes and Argo CD are target-state capabilities and must not be documented as operational until verified.
