# Clinic Platform

Core platform implementation for the clinic system architecture.

## Run

```bash
go test ./...
go run ./cmd/server
```

If `DATABASE_URL` is set, PostgreSQL-backed repositories are used. Otherwise the app falls back to in-memory repositories.
Startup now requires `APP_JWT_SECRET` by default.
For local development without `DATABASE_URL`, you can set `APP_AUTH_DEV_MODE=true` to have the server seed an ephemeral per-process JWT secret automatically so `/auth/login` works out of the box.
When `DATABASE_URL` is set, startup requires `APP_JWT_SECRET` and fails fast if PostgreSQL is unavailable.

## Migrations

Apply the initial schema before running with PostgreSQL:

```bash
psql "$DATABASE_URL" -f migrations/0001_core_platform.sql
```

## Delivery Adapter Configuration

Analytics report delivery supports these channels:

- `download`
- `filesystem`
- `webhook`
- `email`
- `object_store`

### Email Delivery

Email delivery uses SMTP when configured. If SMTP settings are missing, the adapter falls back to writing `.eml` files into a local outbox directory.

Environment variables:

- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USERNAME`
- `SMTP_PASSWORD`
- `SMTP_FROM`
- `SMTP_TLS`

Example:

```bash
export SMTP_HOST="smtp.example.com"
export SMTP_PORT="587"
export SMTP_USERNAME="reports@example.com"
export SMTP_PASSWORD="secret"
export SMTP_FROM="reports@example.com"
export SMTP_TLS="false"
```

Create a scheduled report with email delivery:

```bash
curl -X POST "http://localhost:8080/ops/analytics/reports?name=Daily+Documents&dimension=document_type&format=pdf&schedule=daily&delivery_channel=email&delivery_target=user@example.com"
```

### Object Store Delivery

Object store delivery uses an S3-compatible client when configured. If object store settings are missing, the adapter falls back to writing files into a local object-store-like directory.

Environment variables:

- `OBJECT_STORE_ENDPOINT`
- `OBJECT_STORE_ACCESS_KEY`
- `OBJECT_STORE_SECRET_KEY`
- `OBJECT_STORE_SSL`

Recipient format:

- `bucket/path/to/report.ext`

Example:

```bash
export OBJECT_STORE_ENDPOINT="localhost:9000"
export OBJECT_STORE_ACCESS_KEY="minioadmin"
export OBJECT_STORE_SECRET_KEY="minioadmin"
export OBJECT_STORE_SSL="false"
```

Create a scheduled report with object store delivery:

```bash
curl -X POST "http://localhost:8080/ops/analytics/reports?name=Weekly+Documents&dimension=location&format=xlsx&schedule=weekly&delivery_channel=object_store&delivery_target=reports/weekly/documents.xlsx"
```

### Manual Delivery

Trigger delivery for an existing artifact:

```bash
curl -X POST "http://localhost:8080/ops/analytics/report-artifacts/deliver?artifact_id=<artifact-id>&channel=webhook&recipient=https://example.com/report-hook"
```

Retry failed delivery:

```bash
curl -X POST "http://localhost:8080/ops/analytics/report-deliveries/retry?artifact_id=<artifact-id>&channel=email&recipient=user@example.com"
```

## Useful Endpoints

- `GET /healthz`
- `GET /platform/context`
- `GET /ops/dashboard`
- `GET /ops/analytics`
- `GET /ops/analytics/report-runs`
- `GET /ops/analytics/report-artifacts`
- `GET /ops/analytics/report-deliveries`
- `GET /metrics`
