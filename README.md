# Orbyte Platform

Core platform implementation for the Orbyte architecture.

## Run

```bash
go test ./...
go run ./cmd/migrate up
go run ./cmd/server
```

To regenerate coverage locally:

```bash
./scripts/coverage.sh
```

If `DATABASE_URL` is set, PostgreSQL-backed repositories are used. Otherwise the app falls back to in-memory repositories.
Startup now requires `APP_JWT_SECRET` by default.
For local development without `DATABASE_URL`, you can set `APP_AUTH_DEV_MODE=true` to have the server seed an ephemeral per-process JWT secret automatically so `/auth/login` works out of the box.
When `DATABASE_URL` is set, startup requires `APP_JWT_SECRET` and fails fast if PostgreSQL is unavailable.

## Google Auth

Google sign-in is available through either:

- `POST /auth/google` with a Google ID token obtained by the client
- `GET /auth/google/start` and `GET /auth/google/callback` for a server-managed browser OAuth flow
- `GET /auth/options` for frontend capability discovery

Configure it through `identity.auth`:

- `password_enabled`
- `login_title`
- `login_subtitle`
- `google_button_label`
- `google_enabled`
- `google_auto_provision_enabled`
- `google_auto_provision_allowed_domains`
- `google_auto_provision_role_id`
- `google_auto_provision_scope_type`
- `google_auto_provision_scope_id`
- `google_auto_provision_default_location_id`
- `google_client_id`
- `google_client_secret`
- `google_redirect_url`
- `google_auth_url`
- `google_token_url`
- `google_issuer`
- `google_jwks_url`
- `google_hosted_domain`
- `google_timeout_seconds`

By default the platform expects an existing user whose `username` matches the verified Google email on first sign-in. The first successful Google login links that user to `google:<sub>` as its stable authentication subject for later logins.

The built-in `/ui` shell uses `GET /auth/options`, hides the password form when `password_enabled` is false, shows the Google sign-in button only when `google_enabled` is true, and reads the login title/subtitle/button label from the same runtime config.

When `google_auto_provision_enabled` is true, first-time Google sign-in can create a platform user automatically using the configured role, scope, and default location. Restrict this with `google_auto_provision_allowed_domains` so only approved email domains are provisioned.

The admin console now includes a dedicated authentication settings panel with role and location pickers for Google auto-provisioning defaults.

## Migrations

Apply the initial schema before running with PostgreSQL:

```bash
go run ./cmd/migrate up
```

Inspect migration status:

```bash
go run ./cmd/migrate status
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
- `GET /readyz`
- `GET /platform/context`
- `POST /auth/google`
- `GET /auth/options`
- `GET /auth/google/start`
- `GET /auth/google/callback`
- `GET /ops/dashboard`
- `GET /ops/analytics`
- `GET /ops/analytics/report-runs`
- `GET /ops/analytics/report-artifacts`
- `GET /ops/analytics/report-deliveries`
- `POST /ops/jobs/{job_id}/requeue`
- `POST /ops/outbox/{outbox_id}/retry`
- `POST /ops/outbox/deliveries/{delivery_id}/retry`
- `GET /admin/api/config/validate`
- `GET /metrics`
