# 0005 — Separate scoring from AI explanation

- Status: Accepted
- Date: 2026-08-23
- Owners: project maintainers

## Context

Recommendations must be reproducible, testable, cost-controlled, and available when an AI provider is unavailable.

## Decision

Use a deterministic, versioned engine for condition quality, surfer suitability, confidence, and safety vetoes. AI may explain structured results but does not own the recommendation.

## Alternatives considered

- LLM-generated recommendations: rejected because behavior is probabilistic, difficult to reproduce, and vulnerable to untrusted content.
- Rule-only user text: remains the fallback and may be sufficient for the MVP.

## Consequences

### Positive

- Testable decisions, clear provenance, provider independence, and graceful degradation.

### Negative

- Domain rules and explanation schemas must be explicitly designed and maintained.

## Security implications

AI inputs are structured and minimized; external text cannot grant tool authority or bypass safety rules.

## Operational implications

AI failures affect explanation quality, not core recommendation availability.
