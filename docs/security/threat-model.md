# Initial Threat Model

## Scope

This document covers the public source repository, build pipeline, PWA, backend, database, forecast providers, AI providers, and homelab deployment boundary.

## Assets

- user identity and session material;
- precise coordinates for private spots;
- surfer preferences and session history;
- provider credentials and quotas;
- recommendation integrity and provenance;
- container and release integrity;
- homelab network and Kubernetes credentials.

## Trust boundaries

1. Browser to public application endpoint
2. Application to database
3. Worker to third-party providers
4. Public GitHub workflows to artifact registry
5. Private GitOps repository to Argo CD
6. Kubernetes workloads to homelab infrastructure

## Initial threats and controls

| Threat | Initial control |
| --- | --- |
| Malformed or adversarial provider response | Boundary validation, limits, timeouts, canonical normalization |
| Prompt injection through external content | Structured AI inputs, no provider content with tool authority, deterministic decision engine |
| Exposure of private spot coordinates | Private-by-default authorization, data minimization, log redaction |
| Compromised public pull request | GitHub-hosted runners, read-only tokens, no secrets, no homelab connectivity |
| Mutable or substituted container image | Immutable digests, provenance, SBOM, registry and deployment verification |
| CI-to-cluster credential theft | Pull-based GitOps; no cluster credentials in public CI |
| Secret committed to Git | Secret scanning, review policy, external or encrypted secret management |
| Recommendation interpreted as safety guarantee | Separate safety warnings, confidence display, explicit disclaimer |
| Provider outage or stale data | Timestamped data, freshness policy, graceful degradation |
| Cross-user data access | Server-side authorization at every resource boundary and tenant-aware schema |

## Explicit non-guarantee

The Search must not be used for coastal navigation, rescue decisions, or as a substitute for local safety guidance and direct observation.

## Review triggers

Review this model when authentication, public exposure, a new data provider, AI retrieval, community content, native-device integration, or multi-tenancy is introduced.
