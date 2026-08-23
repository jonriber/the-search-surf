# The Search

The Search is an open-source, self-hostable surf intelligence PWA that combines marine and weather forecasts, local spot knowledge, and surfer preferences to produce explainable, personalized surf recommendations.

## Status

The project is in its foundation phase. The current work establishes its product boundaries, architecture, security model, and delivery conventions before feature development begins.

## Product objective

The Search should answer a practical question:

> Given this surf spot, this surfer, and this time window, is it worth going?

It will turn forecast data into recommendations that are:

- personal to the surfer's experience and preferences;
- specific to a surf spot rather than only a geographic coordinate;
- explainable and reproducible;
- honest about uncertainty and data quality;
- useful on mobile, including degraded connectivity scenarios.

## Architectural direction

- React and TypeScript progressive web application
- Go modular-monolith backend
- PostgreSQL with PostGIS
- Deterministic recommendation engine
- AI used for explanation, not as the decision authority
- OCI containers
- Kubernetes target runtime
- GitHub Actions for continuous integration
- Argo CD for GitOps continuous delivery

The decisions and their trade-offs are recorded in [`docs/adr`](docs/adr/README.md).

## Core principles

1. Safety information is never reduced to a single quality score.
2. Forecasts, transformations, and recommendations retain provenance.
3. Recommendations are reproducible from versioned inputs and rules.
4. Private surf spots and user data are private by default.
5. The application remains useful without an LLM.
6. Operational complexity must solve a concrete problem or a stated learning objective.

## Repository map

```text
docs/
  adr/           Architectural decision records
  architecture/  System boundaries and technical direction
  product/       Vision, scope, and domain language
  security/      Threat model and security guidance
.github/          Contribution and repository governance
```

Application, deployment, and API directories will be added as their foundations are implemented.

## Contributing

The project is not yet accepting feature implementations while the foundation is being established. Design discussion and documented challenges are welcome. See [`CONTRIBUTING.md`](CONTRIBUTING.md) and [`SECURITY.md`](SECURITY.md).

## License

Licensed under the Apache License 2.0. See [`LICENSE`](LICENSE).
