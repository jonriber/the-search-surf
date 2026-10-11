# Domain Glossary

## Principal

The internal identity handle that owns private application data. A principal is established by the server's trusted identity boundary and is not selected by the client.

## Bootstrap principal

The single stable principal used by the first private homelab deployment before a multi-user authentication adapter exists.

## Forecast sample

A provider-originated prediction for a coordinate, time, model, and model run. It retains the provider's units and provenance.

## Normalized forecast

A validated representation of forecast data converted into the application's canonical units and vocabulary.

## Spot

A private, user-owned surf location representing the break the surfer recognizes. Its display position and local attributes are distinct from provider sampling coordinates.

## Favorite

An owner-scoped selection of a surf spot with user-specific ordering. It is a relationship rather than a property of the spot.

## Forecast point

A provider-specific coordinate or grid location used to obtain forecast samples. It records selection provenance and is not the identity or display position of a surf spot.

## Spot interpretation

Versioned local knowledge describing how a surf spot responds to incoming swell, wind, modeled sea level, and known model limitations. It is an owned configuration input, not provider data and not a claim about live conditions.

## Spot transformation

The deterministic application of a spot interpretation to offshore or coarse forecast conditions for a particular surf spot.

It is independent of provider normalization. A spot transformation must state its limitations and must not manufacture an unvalidated at-break wave height.

## Surfer profile

Experience, comfort thresholds, preferences, and equipment that influence suitability without changing objective forecast conditions.

The profile belongs to a principal but is not an authentication identity.

## Condition quality

An assessment of wave-forming conditions independent of a specific surfer.

It is a versioned comparison index, not a probability and not a safety assessment.

## Surfer suitability

An assessment of whether the expected conditions fit a surfer's profile and preferences.

It includes the explicitly selected board and stated comfort limits. Experience and equipment influence suitability but do not prove safety.

## Risk signal

A forecast-derived known caution or suitability limit with a structured severity, evidence, and rule. Risk signals are not numeric penalties and cannot be cancelled by good condition quality or suitability.

The absence of a known signal does not mean that a session is safe; forecasts do not cover every coastal hazard or live condition.

## Recommendation

A versioned evaluation of one favorite spot and one UTC forecast hour from immutable surfer, selected-board, spot-interpretation, mapping, forecast, and ruleset inputs.

A recommendation keeps condition quality, surfer suitability, risk signals, data confidence, and eligibility separate. Only eligible candidates participate in the primary ranking.

## Recommendation eligibility

Whether a candidate has every required, fresh, compatible input and no `avoid` signal. An unrankable result remains visible with machine-readable reasons rather than becoming a low score.

## Data confidence

An ordinal, reasoned assessment of input completeness, freshness, forecast horizon, spot-model maturity, and source-agreement availability.

It is not a probability. Source agreement is `not_assessed` while the application has only one selected source/model for a component.

## Explanation factor

A deterministic structured fact that connects an observed canonical input to the rule, threshold, outcome or contribution, confidence state, and fallback message used by a recommendation. Optional AI may paraphrase factors but cannot change them.

## Ruleset

An immutable, product-owned collection of quantization, thresholds, weights, ordering, and factor rules. Its identifier, engine-protocol version, and content hash travel with every recommendation.

## Session feedback

A surfer's report about observed conditions and recommendation usefulness. Feedback is evidence for calibration, not automatically trusted ground truth.
