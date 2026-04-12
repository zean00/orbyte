# Orbyte Platform

Orbyte is a modular business application platform for operational systems, backoffice workflows, analytics, and governed agent connectivity. It combines a metadata-driven kernel, profile-driven business modules, generic UI surfaces, and machine-facing HTTP/MCP contracts in one runtime.

This repository contains:

- the Go application server
- the module and kernel-pack manifests
- the React workspace frontend
- MCP and ACP integration
- contract generation and validation tooling
- local demo/seed flows for POS, dashboards, CRM, and agent scenarios

## Architecture At A Glance

```mermaid
flowchart TB
    Browser[Browser / Workspace UI]
    Agent[External Agent Runtime via ACP]
    Systems[External Systems / Services]

    Browser --> HTTP[HTTP + UI Surface Layer]
    Agent --> ACP[ACP Session Runtime]
    Agent --> MCP[MCP JSON-RPC]
    Systems --> HTTP
    Systems --> MCP

    HTTP --> Kernel[Platform Kernel Services]
    ACP --> Kernel
    MCP --> Kernel

    Kernel --> Modules[Manifest-Driven Modules]
    Kernel --> Data[(PostgreSQL or In-Memory Stores)]
    Kernel --> Search[Search / Embeddings]
    Kernel --> Events[Jobs, Eventing, Audit, Integrations]
```

## Getting Started

### Fastest local run

```bash
make test
make run
```

Default local values from the `Makefile`:

- app URL: `http://127.0.0.1:18110`
- bootstrap admin password: `admin123!`
- JWT secret: `dev-secret`

### PostgreSQL-backed run

```bash
docker compose up -d postgres
make migrate-up
make run-postgres
```

For a background app lifecycle:

```bash
make app-start-postgres
make app-status-postgres
make app-stop-postgres
```

### Useful local commands

```bash
make test
make lint
make coverage
make contracts
make frontend-verify
make docs-build
```

### Demo and validation flows

```bash
make seed-pos
make seed-dashboard
make seed-crm-demo
make seed-agent-runtime
make validate-mcp
make validate-crm-agent
```

## Installation

### Prerequisites

- Go `1.25`
- Node.js and npm
- Docker and Docker Compose for local PostgreSQL
- `psql` if you use the reset helpers

### Application entry points

- server: `go run ./cmd/server`
- migrations: `go run ./cmd/migrate`
- contracts: `go run ./cmd/contractsgen`
- agent validation: `go run ./cmd/agentproof ...`

### Frontend build

```bash
cd frontend
npm ci
npm run typecheck
npm run build
```

## Runtime Modes

Orbyte supports two main runtime modes:

- in-memory mode
  - fastest local development path
  - no PostgreSQL required
  - process-local state only
- PostgreSQL mode
  - persistent repositories
  - migrations required
  - recommended for realistic development, MCP validation, and demo seeds

## Configuration

The main built-in config areas are:

- `platform.http`
- `platform.acp`
- `platform.mcp`
- `platform.db`
- `search.typesense`
- `search.embedding`
- `eventing.nats`
- `identity.auth`

Important environment variables:

- `APP_ADDRESS`
- `APP_ENV`
- `APP_JWT_SECRET`
- `APP_BOOTSTRAP_ADMIN_PASSWORD`
- `APP_AUTH_DEV_MODE`
- `APP_DOMAIN_PROFILE`
- `DATABASE_URL`

The most important current MCP runtime settings are under `platform.mcp`:

- `enabled`
- `exposure_mode`
  - `full`
  - `compact`
  - `minimal`
- `discovery_mode`
  - `keyword`
  - `vector`
  - `hybrid`
- `tool_discovery_mode`
- `playbook_discovery_mode`
- `discovery_indexing_enabled`
- `governance_enabled`
- `default_action_mode`
- `playbooks_json`

See [docs/configuration.md](docs/configuration.md) for the full runtime/config model.

## Current Platform Shape

### Core platform areas

- identity, roles, sessions, service principals, delegated execution
- configuration definitions and scoped runtime entries
- generic models and records
- documents and workflows
- analytics, dashboards, datasets, report delivery
- search and indexing
- audit, jobs, eventing, integrations, idempotency
- ACP session runtime and MCP server

### Current business modules

The repo currently includes kernel packs for:

- commercial and promotions
- CRM
- procurement, inventory, fulfillment, delivery, returns, supplier returns
- planning, production, production costing, traceability, recall
- POS
- finance reporting, manual journals, collections, treasury, fixed assets, retail finance, inventory finance
- workforce, attendance, leave, payroll, payroll remittance, employee spend
- masterdata, reference masterdata, organization structure
- analytics, documents, workflow approval policy, monitoring/integration

There is also a profile-driven example business module under `internal/modules/clinic.go`.

See [docs/modules.md](docs/modules.md) for the current module inventory and capabilities.

## Surfaces

Declared UI/runtime surfaces in the current codebase:

- `backoffice`
- `admin`
- `worklist`
- `self_service`
- `agent`
- `pos`
- `dashboard`
- `mobile`

Operationally relevant routes include:

- `/ui`
- `/admin`
- `/mcp`
- `/mcp/analytics`
- `/agent/api/...`
- `/auth/...`
- `/healthz`
- `/readyz`

See [docs/surfaces.md](docs/surfaces.md) for the current surface model.

## Agent Integration

Orbyte currently supports:

- ACP provider-backed agent sessions
- MCP JSON-RPC endpoints
- switchable MCP exposure modes
- minimal MCP discovery using:
  - `skills.find`
  - `skills.describe`
  - `tools.find`
  - `tools.describe`
  - `tools.call`
- dashboard artifact promotion into ACP sessions
- service-principal and delegated-user agent access

See [docs/agent-integration.md](docs/agent-integration.md) for the current agent integration model.

## Module Development

Modules are defined through manifests and kernel packs, not ad hoc route-only extensions. The current codebase supports:

- manifest registration
- permissions and role templates
- models, documents, datasets, workflows
- UI menus, actions, views, and dashboard widgets
- MCP tools and resources
- profile-based bootstrapping

See [docs/module-development.md](docs/module-development.md) for current development guidance.

## Documentation

- [Documentation Home](docs/index.md)
- [Getting Started](docs/getting-started.md)
- [Configuration](docs/configuration.md)
- [Architecture](docs/architecture.md)
- [Features](docs/features.md)
- [Modules](docs/modules.md)
- [Agent Integration](docs/agent-integration.md)
- [Module Development](docs/module-development.md)
- [Surfaces](docs/surfaces.md)

## Contracts

Published contract artifacts are generated into:

- `contracts/openapi/<version>/openapi.json`
- `contracts/mcp/<version>/catalog.json`

Generate them with:

```bash
make contracts
```

## Notes

- Startup requires `APP_JWT_SECRET`.
- In non-PostgreSQL local development, `APP_AUTH_DEV_MODE=true` allows a seeded ephemeral dev auth path.
- The default docs site is built with MkDocs Material from `docs/`.
