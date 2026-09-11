# 0018 — Use Open-Meteo behind a provider-neutral forecast port

- Status: Accepted
- Date: 2026-09-11
- Owners: project maintainers

## Context

The first useful forecast needs global hourly wave, swell, wind, and sea-level data, with enough provenance to reproduce a recommendation from retained inputs. Private surf-spot coordinates must not become provider identities, and vendor field names, units, model-selection behavior, and commercial terms must not constrain the domain.

The comparison was reviewed against provider documentation on 2026-09-11:

| Criterion | Open-Meteo | Stormglass |
| --- | --- | --- |
| Coverage and variables | Global marine and weather APIs expose wave, swell, wind, and modelled sea level. Explicit upstream models and sampled grid coordinates are supported. | Global API exposes wave, swell, wind, currents, and tide/sea-level data from selectable sources. |
| Source provenance | Publishes upstream data sources and a metadata API with model initialization, availability, resolution, and update interval. The default `best_match` response does not identify the exact contributing run. | Allows callers to select or compare source models, but the public product and terms pages do not establish an exact source-run issue-time contract. |
| Data licence, caching, and redistribution | API data are CC BY 4.0: copying, adaptation, caching, and redistribution are allowed with attribution and disclosure of changes. | Public terms grant service access but do not explicitly grant data caching or redistribution rights. Those rights would need written contractual confirmation before storing or redistributing responses. |
| Attribution | A visible Open-Meteo link is required next to displayed data; upstream acknowledgements must also be preserved when required. | No equivalent public attribution rule was found; source and contractual requirements would still need review. |
| Free limits | Non-commercial only: 600 calls/minute, 5,000/hour, 10,000/day, and 300,000/month; no uptime guarantee. | Non-commercial evaluation: 10 requests/day and no support. |
| Commercial use | Requires a subscription and customer endpoint. API data remain CC BY 4.0; Standard includes 1 million calls/month, while exact single-run access requires Professional or Enterprise. | Medium and larger plans permit commercial use; published limits begin at 5,000 requests/day for Medium. |
| Operational control | No key for non-commercial prototyping, dedicated paid endpoints, published model-update metadata, and an AGPL self-hosting path. | API key required for all calls; paid tiers provide support, but caching and redistribution remain contract questions. |

Primary references:

- [Open-Meteo Marine API](https://open-meteo.com/en/docs/marine-weather-api)
- [Open-Meteo model-update metadata](https://open-meteo.com/en/docs/model-updates)
- [Open-Meteo licence](https://open-meteo.com/en/license)
- [Open-Meteo terms](https://open-meteo.com/en/terms)
- [Open-Meteo pricing](https://open-meteo.com/en/pricing)
- [Stormglass marine offering](https://stormglass.io/marine-weather/)
- [Stormglass pricing](https://stormglass.io/pricing/)
- [Stormglass terms](https://stormglass.io/terms-and-conditions/)

## Decision

Use Open-Meteo as the initial forecast provider behind the `forecasting.Provider` driven port.

The initial adapter will request UTC timestamps and metric units explicitly and combine three canonical components:

- waves and swell from the Marine API with the explicit Météo-France wave model;
- ten-metre wind and gusts from the Forecast API with the explicit ECMWF IFS model;
- sea-level height above mean sea level from the Marine API with the explicit Météo-France currents model.

Before and after each component fetch, the adapter will read that model's metadata. A component is accepted only when both metadata reads identify the same initialization and the run has been available for at least ten minutes. This prevents a payload fetched during rollout from being assigned the wrong issue time. `best_match` is not used for persisted ingestion because its exact contributing model is not part of the response contract.

The provider result contains canonical SI measurements and opaque provenance only: provider identifier, component, model reference, source issue and availability times, native temporal resolution, fetch time, requested-point correlation, per-component sampled coordinates, transformation version, attribution, and SHA-256 digests of retained raw payloads. Domain and scoring code must never branch on vendor model references.

The free endpoint is allowed only for non-commercial development. Before any commercial use, configuration must move to the contracted customer endpoint and the selected plan must cover the required APIs and volume. Attribution remains mandatory in every mode.

## Resilience and quota policy

- Apply a five-second timeout to each physical metadata or forecast request and a fifteen-second deadline to one logical batch.
- Retry idempotent requests at most twice after the initial attempt, using full-jitter exponential backoff starting at 250 milliseconds and capped at two seconds.
- Retry only connection failures, timeouts, HTTP 408, HTTP 429 when `Retry-After` fits the remaining deadline, and HTTP 5xx. Do not retry authentication, authorization, request-validation, or schema failures.
- Open an endpoint-local circuit after five consecutive retryable terminal failures. Keep it open for sixty seconds, permit one half-open probe, and close it after a successful probe.
- Reserve provider quota before every physical attempt, including metadata calls and retries. Use a persistent, provider-account-scoped UTC budget so multiple worker replicas cannot overspend independently.
- Configure the usable daily and monthly budget to at most 80 percent of the contractual limit. The remaining 20 percent is reserved for retry variance, operator diagnosis, and provider accounting differences.
- Batch coordinates where the provider contract permits it, but never include user, spot, or principal identifiers in provider requests.
- Return the last accepted stored forecast as stale data when policy permits; quota exhaustion and open circuits must not trigger unbounded retry loops.

Stable application errors distinguish invalid requests, exhausted local quota, upstream rate limiting, terminal unavailability, and malformed responses. The worker owns scheduling and deferral; provider adapters own HTTP translation and response validation.

## Alternatives considered

- Stormglass: technically capable and commercially supported, but the free allowance is too small for scheduled multi-spot ingestion and its public terms do not clearly authorize retained caching or redistribution. Reconsider if a commercial agreement explicitly grants those rights and its data quality materially exceeds Open-Meteo.
- Direct national-model ingestion: maximizes source control and run provenance but adds GRIB decoding, grid selection, multi-provider licensing, storage, and model-update operations before the MVP validates product value.
- Open-Meteo `best_match`: useful for interactive exploration, but rejected for persisted ingestion because the response does not establish the exact contributing model run.

## Consequences

### Positive

- The application and future scoring model consume stable SI values rather than vendor payloads.
- Explicit models and metadata bracketing make source issue times testable and prevent mixed-run provenance.
- CC BY 4.0 permits retaining normalized and raw inputs required for reproducibility.
- A second provider can implement the same port without changing domain or use-case APIs.

### Negative

- One logical forecast batch requires multiple physical endpoints and model metadata reads.
- Explicit models trade automatic best-match improvements for predictable provenance and must be reviewed as provider coverage evolves.
- The project must render attribution and indicate transformations wherever forecast-derived data are displayed.
- A persistent quota ledger becomes part of ingestion before horizontal worker scaling.

## Security implications

Provider calls are server-side only. API keys never enter the PWA, logs, query telemetry, or stored raw payloads. Requests contain sampling coordinates but no principal, user, or private-spot identifiers. URLs and raw bodies are treated as sensitive because coordinates can reveal private surf locations. Retained payloads inherit the access controls and backup protections of private spot data.

Open-Meteo documents that its free service may retain technical logs containing coordinates for up to 90 days. This disclosure is accepted for the private non-commercial prototype and must be reassessed before broader release.

## Operational implications

Observe logical fetches, physical attempts, estimated quota cost, circuit state, response age, source issue time, missing-field counts, and transformation version without logging coordinates or credentials. Alert on authentication failures, sustained open circuits, quota exhaustion, metadata instability, attribution configuration errors, and forecasts older than the product freshness threshold.

Provider terms, prices, limits, attribution text, selected models, and coverage must be reviewed at least before commercial launch and on every adapter upgrade; this ADR records the 2026-09-11 decision, not an immutable vendor promise.
