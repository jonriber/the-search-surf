# Security Policy

## Supported versions

The Search is pre-release software. Security fixes currently apply to the default branch only.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability.

Use GitHub's private vulnerability reporting feature for this repository. Include:

- affected component and version or commit;
- reproduction steps or proof of concept;
- expected impact;
- suggested mitigation, when known.

Reports will be acknowledged as soon as practical. A remediation timeline will depend on severity, exploitability, and whether downstream users require coordination.

## Security boundaries

- Repository examples must never contain working credentials or private infrastructure identifiers.
- Kubernetes deployment automation must not require cluster credentials in public CI jobs.
- Pull requests from forks must not execute on persistent runners connected to the homelab.
- Forecast recommendations are informational and must not be represented as navigation or safety guarantees.

See [`docs/security/threat-model.md`](docs/security/threat-model.md) for the initial threat model.
