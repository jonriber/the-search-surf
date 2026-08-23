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

## Branch and pull-request workflow

1. Branch from an up-to-date `main`.
2. Keep commits independently buildable and testable.
3. Include behavior and its tests in the same commit.
4. Update architecture or operational documentation when contracts change.
5. Open a pull request and wait for every required quality gate.

Public pull requests must execute only on isolated GitHub-hosted runners. They must not receive deployment secrets or network access to the homelab.

## Local secrets

Local configuration belongs in ignored `.env` files. Committed `.env.example` files contain names, safe defaults, and placeholders only. Never copy working credentials, internal hostnames, private network addresses, or precise private-spot coordinates into examples, tests, or logs.
