# Development Workflow

## Supported toolchain

The repository records its current development toolchain in `.tool-versions` and `.nvmrc`:

- Go 1.26.6
- Node.js 24.14.1
- npm distributed with the selected Node.js release
- Docker with Compose v2 support

Patch-level updates are expected. A version change that affects builds, language behavior, or runtime compatibility must be reviewed like any other dependency change.

## Repository command contract

The root `Makefile` is the stable developer and CI entry point. Run:

```sh
make help
make verify
```

`make verify` is additive: each implementation boundary extends it with its own format, static-analysis, test, and build checks. CI should invoke the same underlying commands rather than maintain a conflicting validation path.

The current local gates are:

| Boundary | Checks |
| --- | --- |
| API contract | Redocly validation of the OpenAPI document |
| Go API | formatting, `go vet`, pinned `golangci-lint`, `govulncheck`, unit tests, race tests, build |
| PWA | Prettier, ESLint, Vitest coverage thresholds, TypeScript and production build |
| Repository | whitespace and patch hygiene |

`make verify` intentionally excludes dependency audits that require advisory-service availability and the Docker smoke test. Run the complete pre-PR sequence with:

```sh
npm audit --prefix frontend --audit-level=high
make verify
make compose-smoke
```

The first linter or vulnerability run downloads pinned tooling into the repository-local ignored cache. Tool dependencies are not added to the API runtime module.

## Runtime configuration

The API accepts these optional environment variables and rejects invalid explicit values at startup:

| Variable | Default | Purpose |
| --- | --- | --- |
| `HTTP_ADDRESS` | `:8080` | API listen address |
| `HTTP_READ_TIMEOUT` | `5s` | request read timeout |
| `HTTP_WRITE_TIMEOUT` | `10s` | response write timeout |
| `HTTP_IDLE_TIMEOUT` | `60s` | keep-alive idle timeout |
| `HTTP_SHUTDOWN_TIMEOUT` | `10s` | graceful shutdown budget |

The Compose stack binds host ports only to loopback: API on `127.0.0.1:8080` and PWA on `127.0.0.1:8081`. The PWA proxies `/api/*` to the API service. This is a local verification topology, not the future public ingress design.

## Branch and pull-request workflow

1. Branch from an up-to-date `main`.
2. Keep commits independently buildable and testable.
3. Include behavior and its tests in the same commit.
4. Update architecture or operational documentation when contracts change.
5. Open a pull request and wait for every required quality gate.

Public pull requests must execute only on isolated GitHub-hosted runners. They must not receive deployment secrets or network access to the homelab.

See the [CI/CD contract](ci-cd.md) for the enforced checks and the boundary between GitHub Actions and Argo CD.

## Local secrets

Local configuration belongs in ignored `.env` files. Committed `.env.example` files contain names, safe defaults, and placeholders only. Never copy working credentials, internal hostnames, private network addresses, or precise private-spot coordinates into examples, tests, or logs.
