# Deployment

This guide explains how to deploy Orbyte in local, containerized, and production-oriented environments.

## Deployment Modes

### Local Development

Use:

- `make run` for in-memory mode
- `make run-postgres` for PostgreSQL mode

### Containerized Development

Use:

```bash
docker compose up --build
```

This starts:

- PostgreSQL
- the Orbyte app container

### Production-Oriented Deployment

Typical production deployment should use:

- PostgreSQL
- a managed secret source
- durable object store if using report/object delivery
- SMTP if using email delivery
- optional Typesense for externalized search
- optional NATS for external event publication

## Deployment Blueprints

### Blueprint 1: Single-Node Local Or Pilot

```text
+-------------------------------+
| Host or VM                    |
|-------------------------------|
| reverse proxy optional        |
| orbyte app                    |
| postgres                      |
| optional object store / smtp  |
+-------------------------------+
```

Use this when:

- developing locally
- running a small pilot
- proving one module or workflow domain

### Blueprint 2: Production Reference Topology

```text
        +-------------------+
        | Load Balancer/TLS |
        +---------+---------+
                  |
        +---------+---------+
        |   Orbyte App Pods |
        +----+---------+----+
             |         |
     +-------+--+   +--+--------+
     | Postgres |   | Background |
     | primary  |   | jobs/runtime|
     +----------+   +-----------+
             |         |     |
             |         |     +--> SMTP / Object Store
             |         +--------> NATS
             +------------------> Typesense
```

Use this when:

- operating the platform as a shared enterprise service
- enabling analytics, integrations, and external agent access
- requiring durability and operational separation

## Build Artifacts

The repository provides a multi-stage Docker build that produces:

- `orbyte-server`
- `orbyte-migrate`

## Startup Sequence

A normal deployment sequence is:

1. provide environment variables and secrets
2. ensure PostgreSQL is reachable
3. run migrations
4. start the server
5. verify `/healthz` and `/readyz`

## Release Sequence

A practical release flow:

1. build the release artifact
2. back up the database
3. apply migrations
4. deploy the new app version
5. verify health and readiness
6. smoke test login, context, and one business flow
7. monitor integrations, jobs, and logs

## Required Runtime Inputs

At minimum for durable deployment:

- `DATABASE_URL`
- `APP_JWT_SECRET`
- `APP_BOOTSTRAP_ADMIN_PASSWORD`

## Example Local PostgreSQL Deployment

```bash
docker compose up -d postgres
go run ./cmd/migrate up
go run ./cmd/server
```

## Operational Recommendations

- never rely on in-memory mode in production
- manage `APP_JWT_SECRET` as a real secret
- run migrations as part of release rollout
- externalize infrastructure credentials
- monitor health, readiness, logs, and metrics
- test restore and recovery procedures before go-live

## Production Concerns

Before positioning Orbyte as a product deployment target, document and standardize:

- reverse proxy and TLS termination
- database backup policy
- secret management policy
- environment promotion flow
- config promotion flow
- scaling strategy
- audit and retention policy
- observability baseline

## Production Readiness Checklist

- PostgreSQL backup and restore tested
- secrets externalized
- migration rollback plan documented
- health checks wired to orchestration
- metrics collection enabled
- log retention defined
- admin access reviewed
- service principal permissions reviewed
- integration retry and dead-letter runbooks documented

## Documentation Assets

If you want to render or version architecture diagrams separately, see:

- `docs/assets/architecture-overview.mmd`
- `docs/assets/request-flow.mmd`

## Related Guides

- [Getting Started](./getting-started.md)
- [Configuration](./configuration.md)
- [Operations](./operations.md)
