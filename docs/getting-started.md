# Getting Started

This guide helps you run Orbyte locally and understand the minimum setup required to explore the platform.

## What You Need

- Go 1.25
- Node.js and npm
- Docker and Docker Compose if you want PostgreSQL in containers

## Local Runtime Modes

Orbyte supports two main local modes:

- in-memory mode
  - fastest way to explore the platform
  - no PostgreSQL required
  - state is process-local and non-persistent
- PostgreSQL mode
  - closer to production
  - uses migrations and persistent repositories
  - recommended for real feature development

## Quick Start

### In-Memory Mode

```bash
make test
make run
```

This starts the platform with:

- `APP_AUTH_DEV_MODE=true`
- an HTTP listener on `:8080`
- an ephemeral development flow suitable for local exploration

### PostgreSQL Mode

```bash
docker compose up -d postgres
./scripts/dev-postgres.sh
```

Or use:

```bash
make migrate-up
make run-postgres
```

## Default Local Addresses

- app: `http://localhost:8080`
- postgres: `localhost:5432`

## Authentication Bootstrap

On startup the platform seeds a bootstrap admin account and core platform data.

Useful environment variables:

- `APP_JWT_SECRET`
- `APP_BOOTSTRAP_ADMIN_PASSWORD`
- `APP_AUTH_DEV_MODE`
- `DATABASE_URL`

For local development:

- `APP_BOOTSTRAP_ADMIN_PASSWORD` defaults to `admin123!` in the provided helper scripts and `Makefile`
- when running without PostgreSQL, `APP_AUTH_DEV_MODE=true` allows development bootstrap behavior

## Basic Commands

```bash
make test
make lint
make coverage
make contracts
go run ./cmd/contractsgen
make migrate-status
make run
make run-postgres
```

## What Gets Bootstrapped

At startup Orbyte seeds:

- built-in kernel module packs
- business modules resolved from the active domain profile
- configuration definitions
- reference data
- security roles and permissions
- module manifests
- baseline workflows and document definitions
- a bootstrap admin user

## First Things To Explore

After startup, inspect:

- `GET /healthz`
- `GET /readyz`
- `GET /platform/context`
- `GET /auth/options`
- `GET /metrics`

In development mode, OpenAPI is also exposed at:

- `GET /dev/openapi.json`
- `GET /dev/swagger`

Generated release artifacts are written to:

- `contracts/openapi/<version>/openapi.json`
- `contracts/mcp/<version>/catalog.json`

These artifacts are generated from the same profile-aware module set used by the default server runtime.

## Next Steps

- Read [Concepts](./concepts.md) to understand what Orbyte is and is not.
- Read [Architecture](./architecture.md) to understand the runtime model.
- Read [Glossary](./glossary.md) if you want a shared vocabulary before going deeper.
- Read [Module System](./module-system.md) if you plan to build business modules.
- Read [Tutorial: Build Your First Module](./tutorial-first-module.md) if you want a guided extension path.
- Read [Integration](./integration.md) if you plan to connect external systems or AI agents.
