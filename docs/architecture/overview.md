# Architecture Overview

## Context

```text
Forecast providers ──► The Search ──► Surfer
                           │
                           ├──► Notification provider
                           └──► Optional AI explanation provider
```

The Search consumes untrusted external forecast data, applies versioned domain rules, stores recommendations and provenance, and presents results through an installable PWA.

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

- GitHub Actions validates source and publishes immutable OCI images.
- A private environment repository records desired homelab state.
- Argo CD reconciles that state into Kubernetes.
- Public CI does not possess Kubernetes credentials.
- Administrative surfaces remain reachable only through the private network.

The current homelab is Docker-based. Kubernetes and Argo CD are target-state capabilities and must not be documented as operational until verified.
