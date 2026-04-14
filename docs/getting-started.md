# Getting Started

<p class="page-intro">
This guide covers local installation, the supported runtime modes, the default development commands, and the fastest ways to validate that the current codebase is working.
</p>

## Start Here

<div class="quick-links" markdown>

- [**Install Dependencies**](#installation)
  Set up Go, Node.js, and optional docs tooling.
- [**Pick A Runtime Mode**](#runtime-modes)
  Choose between in-memory speed and PostgreSQL-backed realism.
- [**Verify The App**](#first-run-checklist)
  Run health checks and open the workspace/admin shells.
- [**Load Demo Data**](#demo-and-seed-flows)
  Seed CRM, POS, dashboard, and agent validation scenarios.

</div>

## Prerequisites

Install the following locally:

- Go `1.25`
- Node.js and npm
- Docker and Docker Compose for PostgreSQL-backed development
- `psql` if you want to use the schema reset helpers

## Repository Layout

The main directories you will use are:

- `cmd/`
  - server, migration, contract generation, agentproof tools
- `internal/platform/`
  - core runtime services, app construction, HTTP, MCP, ACP
- `internal/modules/`
  - profile-driven non-kernel module examples
- `frontend/`
  - workspace UI
- `docs/`
  - MkDocs site content

## Installation

### Install Go dependencies

```bash
go mod download
```

### Install frontend dependencies

```bash
cd frontend
npm ci
```

### Optional docs environment

The repository already includes `.venv-docs/` helpers for docs work. To build docs:

```bash
make docs-build
```

## Runtime Modes

> Recommended: use PostgreSQL mode for any serious development, MCP validation, or seeded business-demo work.

### In-memory mode

Use this when you want a fast local session without PostgreSQL.

```bash
make run
```

Current behavior:

- binds to `127.0.0.1:18110` by default
- uses in-memory repositories
- enables dev auth mode through `APP_AUTH_DEV_MODE=true`
- seeds the bootstrap admin path so login works locally

### PostgreSQL mode

Use this when you want persistence, realistic validation, or seeded business demos.

```bash
docker compose up -d postgres
make migrate-up
make run-postgres
```

Or manage it in the background:

```bash
make app-start-postgres
make app-status-postgres
make app-stop-postgres
```

## Default Local Values

The `Makefile` currently defaults to:

- app address: `127.0.0.1:18110`
- base URL: `http://127.0.0.1:18110`
- PostgreSQL host port: `55432`
- database URL: `postgres://orbyte:orbyte@127.0.0.1:55432/orbyte?sslmode=disable`
- bootstrap admin password: `admin123!`
- JWT secret: `dev-secret`

## First Run Checklist

### 1. Run tests

```bash
make test
```

### 2. Start the server

```bash
make run
```

or

```bash
make run-postgres
```

### 3. Verify health

```bash
curl http://127.0.0.1:18110/healthz
curl http://127.0.0.1:18110/readyz
```

### 4. Verify auth/options

```bash
curl http://127.0.0.1:18110/auth/options
```

### 5. Open the app

- workspace shell: `http://127.0.0.1:18110/ui`
- admin shell: `http://127.0.0.1:18110/admin`

## Common Development Commands

```bash
make test
make lint
make coverage
make contracts
make frontend-verify
make docs-build
```

## At A Glance

| Need | Command |
| --- | --- |
| quick local run | `make run` |
| PostgreSQL run | `make run-postgres` |
| start background app | `make app-start-postgres` |
| run migrations | `make migrate-up` |
| rebuild contracts | `make contracts` |
| validate CRM agent flow | `make validate-crm-agent` |

## Demo And Seed Flows

The repository includes demo seed paths for current business examples:

```bash
make seed-pos
make seed-dashboard
make seed-crm-demo
make seed-agent-runtime
make seed-agent-continuity
make seed-showcase-demo
```

These write manifests to `/tmp`, including:

- `/tmp/orbyte-pos-seed.json`
- `/tmp/orbyte-dashboard-seed.json`
- `/tmp/orbyte-crm-seed.json`
- `/tmp/agentproof-runtime.json`

## Validation Flows

For real MCP and agent validation:

```bash
make validate-mcp
make validate-crm-agent
```

These use the `cmd/agentproof` harness and the current ACP provider configuration.

## Contracts

Generate release-facing artifacts with:

```bash
make contracts
```

Current outputs:

- `contracts/openapi/<version>/openapi.json`
- `contracts/mcp/<version>/catalog.json`

## Next Steps

- [Configuration](./configuration.md)
- [Architecture](./architecture.md)
- [Features](./features.md)
- [Modules](./modules.md)
- [Agent Integration](./agent-integration.md)

## Recommended Next Pages

<div class="next-steps" markdown>

- [Configuration](./configuration.md) for MCP, ACP, auth, and search settings
- [Architecture](./architecture.md) for the current service graph and startup model
- [Agent Integration](./agent-integration.md) if you plan to use MCP or ACP

</div>
