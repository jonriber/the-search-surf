# Contributing

## Current phase

The Search is establishing its foundation. Before proposing implementation work, check the product scope and architectural decision records.

## Change process

1. Open or identify an issue that states the problem and acceptance criteria.
2. For an architectural change, add or supersede an ADR before implementation.
3. Keep changes focused and include tests proportional to their risk.
4. Update user, architecture, operational, or API documentation when behavior changes.
5. Submit a pull request using the repository template.

## Engineering expectations

- Prefer explicit domain language over generic abstractions.
- Keep domain logic independent from HTTP, persistence, forecast providers, and AI vendors.
- Validate untrusted data at system boundaries.
- Preserve forecast and recommendation provenance.
- Do not log credentials, session tokens, precise private-spot coordinates, or unnecessary personal data.
- Explain why non-obvious code exists; do not use comments to narrate syntax.
- Avoid adding infrastructure or dependencies without documenting their operational cost.

## Architectural decisions

ADRs live in [`docs/adr`](docs/adr/README.md). Accepted ADRs describe the current direction. A meaningful reversal should supersede the original record rather than silently rewriting its history.

## Commits

Use concise, imperative commit subjects. The initial convention is:

```text
system: summarize the main change, affected areas, and problem solved
```

Issue-related changes may use `feature/<issue-number>:` or `bugfix/<issue-number>:` prefixes.
