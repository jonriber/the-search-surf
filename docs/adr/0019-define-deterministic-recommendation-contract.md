# 0019 — Define the deterministic recommendation contract

- Status: Accepted
- Date: 2026-10-10
- Owners: project maintainers

## Context

The forecast-ingestion slice retains attributable, normalized hourly conditions, but it does not yet define how those conditions become a recommendation. A weighted score alone cannot carry the product meaning: good wave-forming conditions may be unsuitable for a surfer, a known caution must not be cancelled by positive conditions, and absent data must not be presented as bad surf or false certainty.

The first recommendation contract must also remain reproducible after mutable profile, board, spot, mapping, and preference records change. Issue #13 needs to compare favorite spots and hours, so the contract must define both evaluation of one candidate and deterministic ordering of many candidates. It must do so without claiming that forecast data can certify coastal safety.

## Decision

### Candidate and evaluation boundary

A recommendation candidate is one owned favorite spot at one UTC forecast hour for one explicit surfer-profile, selected-board, preference, spot-interpretation, spot-to-forecast-mapping, canonical-forecast, ruleset, and evaluation-time snapshot.

The domain exposes a pure single-candidate evaluator. Application orchestration constructs candidates for the requested UTC window and a separate pure ranker orders them. Spot-local timestamps and UTC offsets are presentation values derived from the snapshotted IANA time zone; they never identify or order an instant.

Every result has an eligibility status. The initial vocabulary is:

- `ranked`: every required input is present, fresh, and compatible;
- `unrankable_missing_required_input`;
- `unrankable_stale_forecast`;
- `unrankable_incompatible_version`.

Machine-readable reasons refine the status, including missing mapping, spot interpretation, preferences, selected board, or canonical forecast. New statuses and reason codes require an input/output schema version change.

### Separate assessments

An evaluated candidate contains five separate assessments:

1. `conditionQuality`: wave-forming conditions for the configured spot, independent of a surfer;
2. `surferSuitability`: fit with explicit comfort limits, preferences, experience, and the selected board;
3. `riskSignals`: forecast-derived known cautions and suitability limits;
4. `dataConfidence`: completeness, freshness, forecast horizon, spot-model maturity, and source-agreement availability;
5. `eligibility`: whether the candidate may participate in ranking.

Quality and suitability use integer basis points in `[0, 10000]` when available. They are indices for comparison within one ruleset, not probabilities. User-facing ordinal bands may be derived by versioned thresholds.

Risk signals use `unknown`, `info`, `caution`, or `avoid` severity and carry evidence plus the applied threshold. They are never converted to a numeric score. An `avoid` signal makes a candidate ineligible for the ranked recommendation set; it remains visible with its evidence. A `caution` remains visible without changing quality or suitability.

The product describes these as known forecast-derived cautions, not as a complete safety assessment. It never emits `safe=true`. Experience and equipment influence suitability but are not evidence of safety.

### Missingness and confidence

Each ruleset declares its required inputs. Missing, stale, or incompatible required input makes the candidate unrankable instead of lowering a score. Optional inputs do not contribute to a numeric aggregate; when absent, they produce an unavailable factor and reduce the relevant confidence or coverage dimension.

Confidence is an ordinal, reasoned assessment rather than a calibrated probability. The first ruleset reports completeness, freshness, forecast horizon, and spot-model maturity independently. Source agreement is `not_assessed` while the application has only one selected source/model per component. A future calibrated confidence percentage requires empirical validation and a new ruleset.

### Scoring and ranking determinism

Score-relevant measurements are quantized into versioned integer units before evaluation. Weights are product-owned integers that sum to 10,000 for each aggregate. Intermediate arithmetic uses integers; division uses round-half-away-from-zero. Threshold inclusivity is declared by each rule and exact-boundary tests are mandatory.

Directions are degrees clockwise from true north in `[0, 360)` and mean the direction **from which** wind or waves arrive. This matches the Open-Meteo marine contract for waves and the meteorological convention for wind. A circular sector includes both endpoints and wraps through north when its start is greater than its end; for example, `330°–30°` contains `0°`. Equal endpoints describe one direction, not a full circle. A full-circle rule must be represented explicitly.

For ranked candidates the stable ordering key is:

1. surfer suitability descending;
2. condition quality descending;
3. confidence band descending;
4. valid UTC hour ascending;
5. spot UUID in canonical byte order ascending.

Unrankable and `avoid` candidates are returned after ranked candidates in deterministic status, hour, and spot-ID order so the client can explain exclusions. Rules never read the system clock; `asOf` is an explicit input.

### Modeled sea-level semantics

`sea_level_height_metres_above_msl` remains modeled sea-level height above global mean sea level. It may include modeled tidal effects, but it is not measured beach tide, chart-datum height, local water depth, or a navigation or safety signal. Coastal accuracy may be limited.

The first numeric ruleset does not use modeled sea level in quality, suitability, or risk calculations. It may be displayed as an attributed optional fact. Any future local sea-level fit or rising/falling interpretation must be opt-in per spot, include the required surrounding samples in the immutable input snapshot, document its datum and limitations, and introduce a new ruleset version.

### Reproducibility and versioning

A persisted recommendation retains canonical input and output envelopes. The input contains:

- input-schema and canonical-encoding versions;
- engine-protocol version, ruleset identifier, and ruleset content hash;
- explicit `asOf` and candidate UTC hour;
- immutable profile, preference, selected-board, spot, spot-interpretation, and mapping snapshots or versions;
- canonical forecast observations and their batch, source, issue, fetch, normalization, attribution, and payload provenance;
- every default and assumption used by evaluation.

All score-relevant values are canonical integers, arrays have specified ordering, and unordered maps are not part of the signed envelope. The stored envelope has a SHA-256 digest. The output envelope uses the same discipline and has its own digest.

Reproduction means that the registered evaluator for the stored engine-protocol and ruleset versions produces bit-identical canonical output bytes from the stored canonical input bytes. Persisting the previous result or a human-readable version string is not sufficient. Released evaluators and immutable rule parameters remain available through a version registry; changing behavior creates a new ruleset or engine-protocol version. Replay fixtures protect previous versions.

Provider normalization and spot interpretation are different stages. Provider-facing `TransformationVersion` is treated as a normalization version at the recommendation boundary and will be renamed without changing historical database meaning when that model is next migrated. Spot interpretation has its own independent version.

### Explanation contract

Every factor has a stable code, assessment category, observed canonical value and unit when available, applied rule or threshold, outcome or signed contribution, confidence/availability state, and deterministic fallback message key plus parameters. Factors are emitted in ruleset-defined order and reconcile to their aggregate.

Deterministic fallback text is always available from these facts. An optional AI renderer may later paraphrase a minimized structured subset, but it cannot change, suppress, or create assessments, factors, ranking, eligibility, or risk signals. External AI integration is outside the first recommendation milestone.

## Alternatives considered

- One combined 0–100 recommendation score: rejected because it hides whether a result reflects objective conditions, personal fit, missing data, or known cautions.
- Subtract missing-data and risk penalties from quality: rejected because unknown conditions are not bad conditions and positive factors must not cancel cautions.
- Let users configure scoring weights: rejected for the first version because arbitrary algorithms are hard to explain, compare, test, and support. Users configure preferences and limits instead.
- Rank only a selected spot: rejected because the product outcome and issue #13 require choosing where and when across favorites.
- Use current mutable rows plus a scoring-version string for replay: rejected because it cannot reconstruct historical inputs or prove recomputation.
- Use modeled sea level as ordinary tide: rejected because the datum, coastal accuracy, and local break relationship do not support that claim.
- Let an LLM produce the recommendation: rejected consistently with ADR 0005 because probabilistic output cannot be the decision authority or replay contract.

## Consequences

### Positive

- Product meaning is explicit before persistence and scoring code make it expensive to change.
- Unknown, unsuitable, and poor conditions remain distinguishable.
- Issue #13 receives deterministic ranking and degraded-state semantics rather than inventing them in the client.
- Historical recommendations can be audited and replayed.
- AI, provider, database, and HTTP adapters remain outside the decision authority.

### Negative

- More domain types and provenance must be persisted than a simple score would require.
- Released rules and canonical encodings become compatibility commitments.
- Numeric scores are comparable only within compatible ruleset versions unless an explicit migration or calibration proves otherwise.
- The first release requires deliberate local configuration and cannot honestly recommend an arbitrary coordinate.

## Security implications

Recommendation inputs contain sensitive coordinates, preferences, equipment, and behavior. Canonical snapshots remain owner-scoped, protected by forced row-level security, excluded from routine logs, and minimized in client/offline responses. Digests support integrity checks but do not anonymize low-entropy or sensitive input and therefore are not public identifiers.

Structured AI inputs, if introduced later, exclude coordinates and unnecessary profile data. Provider or user text never gains tool authority. Risk evidence and coverage limitations cannot be removed by an explanation adapter.

## Operational implications

Deployments must retain evaluator compatibility for stored engine-protocol and ruleset versions or explicitly migrate evidence with an auditable process. Observability records status/reason counts, ruleset versions, replay failures, freshness, and confidence dimensions without logging private inputs.

Rule changes require golden replay fixtures, boundary tests, a new immutable version/hash, and release notes describing expected ranking changes. Provider-direction and sea-level semantics must be revalidated when an adapter changes.

Primary semantic references reviewed for this decision:

- [Open-Meteo Marine Weather API](https://open-meteo.com/en/docs/marine-weather-api)
- [WMO OSCAR near-surface wind direction](https://space.oscar.wmo.int/variables/view/wind_direction_near_surface)
