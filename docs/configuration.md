# Configuration

This guide explains how runtime configuration works in Orbyte.

## Configuration Model

Orbyte separates configuration into:

- definitions
  - describe the key, category, fields, and allowed scopes
- entries
  - store the configured values
- effective values
  - resolved values after scope inheritance is applied

## Configuration Scopes

Built-in scope levels:

- deployment
- organization
- location

This allows you to keep a global default while overriding behavior for a specific business context.

## How Resolution Works

In general, the runtime resolves values from the most specific applicable scope back to the broader scope.

Typical pattern:

1. location override
2. organization override
3. deployment default
4. built-in default

## Built-In Configuration Areas

Current built-in examples include:

- `platform.http`
  - listener address
- `platform.acp`
  - optional ACP provider configuration
- `platform.db`
  - database instrumentation and read-strategy settings
- `search.typesense`
  - external Typesense connectivity
- `eventing.nats`
  - external NATS publication
- `search.embedding`
  - embedding provider defaults
- `identity.auth`
  - authentication, session, password, rate limit, and Google sign-in policy

## Environment Variables

Important operational environment variables include:

- `APP_ADDRESS`
- `APP_JWT_SECRET`
- `APP_BOOTSTRAP_ADMIN_PASSWORD`
- `APP_AUTH_DEV_MODE`
- `APP_ENV`
- `APP_DOMAIN_PROFILE`
- `DATABASE_URL`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USERNAME`
- `SMTP_PASSWORD`
- `SMTP_FROM`
- `SMTP_TLS`
- `OBJECT_STORE_ENDPOINT`
- `OBJECT_STORE_ACCESS_KEY`
- `OBJECT_STORE_SECRET_KEY`
- `OBJECT_STORE_SSL`

## Common Configuration Examples

### Development HTTP Address

```bash
export APP_ADDRESS=":8080"
```

### PostgreSQL Runtime

```bash
export DATABASE_URL="postgres://orbyte:orbyte@127.0.0.1:55432/orbyte?sslmode=disable"
```

### Production JWT Secret

```bash
export APP_JWT_SECRET="<strong-random-secret>"
```

### Enable External Search

Use a scoped config entry for `search.typesense` with values such as:

```json
{
  "enabled": true,
  "endpoint": "http://typesense:8108",
  "api_key": "replace-me",
  "timeout_seconds": 5
}
```

### Enable External Event Publication

Use a scoped config entry for `eventing.nats` with values such as:

```json
{
  "enabled": true,
  "url": "nats://nats:4222",
  "sink_name": "nats",
  "subject_prefix": "orbyte",
  "timeout_seconds": 5
}
```

## Configuration Administration

Configuration can be managed through:

- admin HTTP routes
- config service APIs
- MCP control-plane tools
- startup environment variables where applicable

## Security Considerations

Treat configuration as production data. In particular:

- do not hard-code secrets in module source
- use secret-aware handling for sensitive fields
- scope configuration as narrowly as operationally reasonable
- audit changes through administrative flows

## Practical Recommendations

- use deployment scope for infrastructure and platform-wide defaults
- use organization scope for enterprise policy differences
- use location scope for branch or site-specific behavior
- keep secrets separate from general runtime settings when possible

## Related Guides

- [Getting Started](./getting-started.md)
- [Deployment](./deployment.md)
- [Operations](./operations.md)
