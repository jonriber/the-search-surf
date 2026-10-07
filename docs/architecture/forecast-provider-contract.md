# Forecast Provider Contract

## Boundary

`internal/application/forecasting.Provider` is a driven application port. Worker orchestration depends on the port; Open-Meteo HTTP code will implement it under the platform/provider boundary. The domain and use cases do not import provider clients, JSON schemas, HTTP status codes, API keys, or vendor variable names.

```text
worker use case
  └─ forecasting.Provider
       └─ resilience + shared quota policy
            └─ Open-Meteo adapter
                 ├─ model metadata
                 ├─ marine waves and swell
                 ├─ weather wind
                 └─ marine sea level
```

The interface accepts a batch of application-owned forecast points and an inclusive-exclusive hourly UTC window. Point references correlate results without floating-point equality and are never sent upstream. An adapter may split or batch physical requests, but returns one series per accepted reference.

## Canonical output

- Heights are metres, speed is metres per second, periods are seconds, and directions are degrees clockwise from true north in `[0, 360)`.
- Sea level is metres relative to mean sea level; it is not mislabeled as a measured beach tide or a navigation-safe water depth.
- Timestamps are UTC instants. Each series is strictly ordered and contains no duplicate valid time.
- Missing measurements are explicit. A missing wave, wind, or sea-level value is never converted to zero or forward-filled by the adapter.
- Provider-returned sampling coordinates are retained per component because wave, wind, and sea-level models use different grids and may move a request to different nearby cells.
- Invalid ranges, mismatched array lengths, unexpected units, duplicate times, unknown required fields, HTML error bodies, and oversized responses are malformed responses, not partial success.

`forecast.Batch` carries attribution, transformation version, component model references, source issue times, fetch time, and raw-payload SHA-256 digests. Vendor model references are opaque provenance: persistence may store them and diagnostics may display them, but scoring rules cannot branch on them.

## Open-Meteo mapping

| Canonical component | Endpoint | Explicit model | Requested variables |
| --- | --- | --- | --- |
| Waves and swell | Marine API | Météo-France wave | wave and swell height, direction, and period |
| Wind | Forecast API | ECMWF IFS | ten-metre speed, direction, and gusts |
| Sea level | Marine API | Météo-France currents | sea-level height above mean sea level |

Every data request sets `timezone=GMT`; metric length and metres-per-second wind units are explicit. Marine requests prefer sea grid cells. The adapter records each component's sampled coordinates and native temporal resolution. Different model grids are expected and remain visible in provenance. Components are joined only on exact hourly valid times; the adapter never invents spatial or temporal interpolation, and absent component values remain explicit missing measurements.

For each component, read model metadata before and after the forecast payload. Accept the payload only when `last_run_initialisation_time` is unchanged and `last_run_availability_time` is at least ten minutes old. Persistence records initialization as source issue time and availability separately. If a future component genuinely has no issuance concept, represent that fact explicitly; do not substitute fetch time.

## Error and orchestration semantics

| Error | Meaning | Orchestrator action |
| --- | --- | --- |
| `ErrInvalidRequest` | Application supplied an invalid point batch or window | Reject without an upstream call |
| `ErrQuotaExhausted` | Shared local budget denied an attempt | Defer until budget reset; alert before freshness is breached |
| `ErrRateLimited` | Provider returned HTTP 429 | Honor bounded `Retry-After`, then defer |
| `ErrUnavailable` | Retryable failures exhausted, circuit open, or metadata changed mid-fetch | Keep last accepted data and schedule a bounded retry |
| `ErrMalformedResponse` | Contract, unit, range, or schema validation failed | Quarantine payload reference and alert; do not retry blindly |

Cancellation from the caller is preserved as `context.Canceled` or `context.DeadlineExceeded`. Provider adapters wrap stable errors so `errors.Is` works and retain the original cause for internal diagnostics without returning raw provider bodies to clients.

## Testing contract

- Domain tests cover point, coordinate, finite-measurement, and explicit-missingness invariants.
- Port tests cover UTC normalization, hourly windows, stable correlation, duplicate references, and defensive copying.
- The Open-Meteo adapter uses recorded fixtures for canonical units, nulls, array mismatches, sampled-grid changes, metadata rollover, error classification, and attribution.
- Provider contract tests must run against a fake HTTP server; live-provider probes are optional diagnostics and never gate deterministic CI.
- A scheduled, non-blocking canary may detect upstream contract drift without using private coordinates.
