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
```

The browser uses only same-origin `/api` URLs. nginx owns routing at the deployment edge, so the PWA does not embed a homelab address and the API does not need permissive cross-origin rules. The client caches the application shell and the last successfully validated release identity; it does not cache arbitrary API responses.

The API currently exposes `/health/live`, `/health/ready`, and `/version`. Its OpenAPI description under `api/openapi` is the transport contract. HTTP concerns live under `internal/platform`; surf-domain packages will be introduced only with the first domain behavior.

The [initial domain and data model](domain-model.md) defines explicit principal ownership, private surf spots, favorites, and the separate forecast-ingestion seam. PostgreSQL row-level security is a database backstop; application use cases remain responsible for authorization intent and transaction scope.

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
