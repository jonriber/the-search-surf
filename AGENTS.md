# Repository agent instructions

## Zero-failure CI/CD rule

- Do not consider work complete, recommend a merge, or hand off a pull request while any CI/CD check is failing, cancelled, or still pending.
- After every push, monitor the complete check matrix to a terminal state and investigate every non-passing result, including failures inherited from `main`.
- Fix root causes. Never disable, skip, suppress, or weaken tests, security scans, quality gates, severity thresholds, or branch protections merely to obtain a green result.
- When a failure is unrelated to the feature, isolate the repair in a focused issue and pull request, merge it first, then update the blocked feature branch and confirm its full matrix passes.
- Local verification is necessary but does not replace hosted-only checks such as dependency review, CodeQL, SBOM generation, image scanning, and deployment validation.
- If an external outage or unavailable upstream fix makes success impossible, keep the work explicitly blocked and provide evidence; do not report completion or readiness.
