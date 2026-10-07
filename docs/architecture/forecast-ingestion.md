# Forecast Ingestion

## Runtime flow

Forecast ingestion is a one-shot worker designed to be invoked by an external scheduler. The process keeps orchestration in the application layer and vendor/database details at driven adapters:

```text
external scheduler
  └─ ingest-forecast command
       └─ forecasting.Service
            ├─ forecasting.Provider ──► Open-Meteo adapter
            └─ forecasting.Repository ──► PostgreSQL adapter
```

The repository returns active provider sampling points. The provider adapter fetches waves, wind, and sea level using explicit models, converts them into canonical SI measurements, and brackets every component payload with model-metadata reads. The service validates exact point correlation, complete hourly sequencing, provenance, attribution, checksums, UTC timestamps, missing values, and plausible physical bounds before asking persistence to commit anything.

The worker is deliberately separate from the API. Provider latency, retries, and failures therefore cannot consume API request capacity, and the worker connects with `the_search_ingester` rather than inheriting access to private profiles or surf spots.

## Canonical persistence graph

- `forecast_points` stores provider sampling coordinates and the version of the selection algorithm. A forecast point is not a private surf spot.
- `forecast_batches` stores the requested validity window, fetch time, transformation version, attribution, licence, and a deterministic 32-byte idempotency key.
- `forecast_sources` stores the exact component model, known/unknown issue-time semantics, availability time, sampled grid coordinate, and native temporal resolution.
- `forecast_observations` stores nullable hourly SI measurements. Database constraints repeat the application plausibility checks as a corruption backstop.
- `forecast_payloads` stores a SHA-256 digest per component response. Raw bodies are not stored in PostgreSQL by default.
- `forecast_rejections` stores checksum-only evidence and a curated reason for malformed input. It is not readable by the API role.
- `provider_quota_usage` atomically coordinates daily and monthly physical-request budgets across worker replicas.

All accepted rows for a batch are inserted in one transaction. A replay computes the same key from provider, window, points, transformation version, component checksums, and source-run metadata; `ON CONFLICT DO NOTHING` then reports a successful replay without duplicating observations. Fetch time is intentionally excluded because a retry of identical upstream data occurs at a different instant.

## Tide terminology

The provider field is persisted as `sea_level_height_metres_above_msl`. It includes modelled tidal effects but is not presented as a measured beach tide, chart-datum height, water depth, or navigation-safe value. Future recommendation and UI work must preserve that distinction.

## Failure containment

- A logical batch has a 15-second deadline; each physical call has a 5-second timeout.
- Retryable calls get at most two retries after the first attempt with capped full-jitter exponential backoff.
- Every physical attempt reserves shared daily and monthly quota before making a network call.
- Endpoint circuits open after five retryable terminal failures for one minute.
- HTTP errors, model-run rollover, too-recent availability, malformed JSON, unit drift, array mismatch, duplicate timestamps, oversized bodies, non-finite values, and implausible ranges never produce a partial committed batch.
- Caller cancellation remains distinguishable from provider unavailability.

The current in-process circuit state is intentionally replica-local; quota is the horizontally coordinated safety boundary. A distributed circuit should be introduced only if independent replicas create a demonstrated provider-pressure problem.

## Local operation

The migration does not invent a default location. An operator or future spot-to-grid mapping use case must first provision a public provider sampling point with the migrator role. For example:

```sql
INSERT INTO forecast_points (id, provider_id, position, selection_algorithm_version)
VALUES (
  '11111111-1111-4111-8111-111111111111',
  'open-meteo',
  ST_SetSRID(ST_Point(-9.417, 38.963), 4326)::geography,
  'nearest-sea-grid-v1'
);
```

Then run one cycle:

```sh
docker compose --profile forecast run --rm forecast-ingest
```

The default 72-hour horizon starts at the current UTC hour. `THE_SEARCH_FORECAST_HORIZON`, `THE_SEARCH_OPEN_METEO_ACCOUNT_SCOPE`, `THE_SEARCH_OPEN_METEO_DAILY_LIMIT`, and `THE_SEARCH_OPEN_METEO_MONTHLY_LIMIT` configure Compose. The default budgets are 80% of the documented non-commercial allowance; commercial or self-hosted deployments must configure their contracted endpoints and usable budgets explicitly.

The free Open-Meteo endpoint is for non-commercial development only. Attribution and licence metadata travel with every stored batch and must be rendered by later forecast/recommendation UI work.
