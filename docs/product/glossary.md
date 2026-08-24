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

## Spot transformation

The versioned rules that translate offshore or coarse forecast conditions into an estimate for a particular surf spot.

## Surfer profile

Experience, comfort thresholds, preferences, and equipment that influence suitability without changing objective forecast conditions.

The profile belongs to a principal but is not an authentication identity.

## Condition quality

An assessment of wave-forming conditions independent of a specific surfer.

## Surfer suitability

An assessment of whether the expected conditions fit a surfer's profile and preferences.

## Recommendation

A versioned result combining normalized forecasts, spot transformation, condition quality, surfer suitability, and confidence.

## Confidence

An explicit estimate of recommendation uncertainty based on forecast horizon, provider agreement, missing inputs, and known spot-model limitations.

## Session feedback

A surfer's report about observed conditions and recommendation usefulness. Feedback is evidence for calibration, not automatically trusted ground truth.
