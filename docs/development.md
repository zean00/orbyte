# Development Workflow

## Baseline Commands

```bash
make test
make lint
make coverage
make contracts
make migrate-status
make run
make run-postgres
```

## Local Runtime Modes

- In-memory mode:
  - `make run`
  - uses `APP_AUTH_DEV_MODE=true`
  - seeds an ephemeral JWT secret only when `APP_JWT_SECRET` is not provided
- PostgreSQL-backed mode:
  - `docker compose up -d postgres`
  - `./scripts/dev-postgres.sh`

## Bootstrap and Seed Behavior

- The platform kernel seeds built-in modules, config definitions, reference data, policies, and the bootstrap admin user during app startup.
- Set `APP_BOOTSTRAP_ADMIN_PASSWORD` to control the seeded `admin` password.
- Business profile manifests are selected through `APP_DOMAIN_PROFILE`.

## Migration Workflow

```bash
go run ./cmd/migrate status
go run ./cmd/migrate up
```

- Run migrations before starting the app when `DATABASE_URL` is set.
- Keep schema changes backward compatible within a release line; code and migrations for a release should ship together.

## Local Containers

```bash
docker compose up --build
```

- `postgres` exposes `localhost:55432`
- `app` exposes `localhost:8080`

## Delivery Baseline

Any CI system should run the same repo-native checks:

1. `make test`
2. `make lint`
3. `make contracts`
4. `make ui-build`
5. PostgreSQL migration smoke test via `go run ./cmd/migrate up`
