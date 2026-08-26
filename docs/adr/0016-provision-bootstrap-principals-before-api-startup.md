# 0016 — Provision bootstrap principals before API startup

- Status: Accepted
- Date: 2026-08-26
- Owners: project maintainers

## Context

ADR 0011 requires one stable, environment-owned principal in bootstrap mode. The runtime database role intentionally has no access to the `principals` table, while user-owned rows require an existing principal through foreign keys. The system therefore needs an explicit provisioning boundary without giving the long-running API schema-owner or identity-administration privileges.

Provisioning is environment state rather than a schema migration: the principal identifier differs between deployments and must survive restore. It is also not request behavior and must not race across API replicas during startup.

## Decision

- Add a dedicated, idempotent bootstrap-principal command to the application release artifacts.
- Require an explicit principal UUID and a separate bootstrap database URL; do not generate an environment identity implicitly.
- Execute the command after forward migrations and before API startup as an observable one-shot deployment step.
- Use the short-lived migration role for the initial implementation because it owns the `principals` table; never expose that credential to the API container.
- Treat an existing enabled principal as success, an existing disabled principal as a conflict, and invalid or missing configuration as a startup failure.
- Configure the API with the same stable principal UUID. The trusted HTTP resolver injects it into request context; clients never submit an owner identifier.
- Keep the command interface independent from Compose so Kubernetes can later run the same artifact as a Job controlled by the private environment repository.

## Alternatives considered

- Let the API insert its principal during startup: rejected because every replica would receive identity-administration privileges and readiness would mutate persistent state.
- Encode the principal UUID in a migration: rejected because migrations are shared release state while the identifier belongs to each environment.
- Generate a random principal whenever it is missing: rejected because an unnoticed configuration loss would orphan existing ownership and break restore continuity.
- Use the PostgreSQL administrator role: rejected because principal provisioning does not require superuser privileges.
- Introduce a fourth database role immediately: deferred until identity administration needs privileges distinct from schema ownership in a multi-user deployment.

## Consequences

### Positive

- The API remains least-privileged and horizontally safe.
- Identity provisioning is repeatable, observable, and testable without coupling it to HTTP traffic.
- Compose and future GitOps deployment use the same release command and ordering contract.

### Negative

- Deployment gains another one-shot step and must provide the stable UUID twice through separate process configurations.
- The migration role temporarily owns both schema changes and bootstrap identity creation.
- Restore procedures must verify that environment configuration and the restored principal row still agree.

## Security implications

Bootstrap credentials and migration credentials must never enter the API environment. Command output and logs must not print database URLs. The bootstrap UUID is an identity handle, not a credential, but it must not be accepted from an untrusted request as authorization evidence.

## Operational implications

Compose and Kubernetes ordering is database health, migration completion, principal provisioning completion, then API startup. A restore drill verifies the configured principal exists and is enabled before serving user-data requests. Introducing real authentication replaces the fixed resolver and triggers a separate identity-provisioning decision.
