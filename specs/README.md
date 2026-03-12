# Core Platform Bootstrap

This repository now contains the first buildable Go scaffold for the platform kernel described in the specification documents.

## Current Bootstrap

- `cmd/server`
  - minimal HTTP server entrypoint
- `internal/platform/app`
  - application wiring
- `internal/platform/httpx`
  - HTTP routes and JSON helpers
- `internal/platform/shared`
  - shared primitives and common error types
- `internal/platform/organization`
  - organization, location, and scope context bootstrap
- `internal/platform/config`
  - runtime configuration bootstrap service
- `internal/platform/identity`
  - user, role, permission, and authorization bootstrap service
- `internal/platform/document`
  - initial document registry and in-memory document creation service
- `internal/platform/workflow`
  - workflow type definitions
- `internal/platform/eventing`
  - event and outbox type definitions
- `internal/platform/search`
  - projection type definitions
- `internal/platform/integration`
  - integration type definitions

## Run

```bash
go test ./...
go run ./cmd/server
```

## PostgreSQL Mode

To run with PostgreSQL instead of in-memory repositories:

1. create a PostgreSQL database
2. apply `migrations/0001_core_platform.sql`
3. set `DATABASE_URL`
4. start the server

Example:

```bash
psql "$DATABASE_URL" -f migrations/0001_core_platform.sql
DATABASE_URL="postgres://user:pass@localhost:5432/clinic?sslmode=disable" go run ./cmd/server
```

If `DATABASE_URL` is not set, the app falls back to in-memory repositories.

The server exposes:

- `GET /healthz`
- `GET /platform/context`

## Recommended Next Build Slice

1. add update endpoints with optimistic concurrency checks
2. implement workflow execution service and bind submit to workflow definitions
3. add audit and transactional outbox persistence
4. add first projection-backed list endpoint
5. add role binding and permission enforcement backed by PostgreSQL
