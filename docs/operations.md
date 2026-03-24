# Operations

This guide covers day-2 operational topics for Orbyte.

## Runtime Health

Built-in health endpoints:

- `GET /healthz`
- `GET /readyz`

These should be used by:

- load balancers
- orchestrators
- uptime monitors

## Monitoring and Metrics

Orbyte exposes metrics through:

- `GET /metrics`

Use metrics to observe:

- runtime status
- error rates
- integration outcomes
- job behavior
- search and database performance trends

## Logging and Audit

There are two different concerns:

- logs
  - operational debugging and runtime telemetry
- audit trail
  - who did what, to which target, and when

Machine actions and tool invocations should also be auditable when routed through governed surfaces.

## Background Processing

Background work may include:

- integration submission handling
- retry execution
- report generation and delivery
- search or projection maintenance
- data operations

These should be monitored as first-class runtime activity.

## Backup, Restore, and Archive

The DataOps service supports:

- backup planning and execution
- export planning and execution
- restore planning and execution
- archive planning and execution

For product use, define:

- backup schedule
- retention period
- restore verification process
- archive policy

## Operational Runbooks To Maintain

At minimum, maintain runbooks for:

- failed startup
- migration failure
- degraded readiness
- integration dead letters
- failed report delivery
- search backend unavailability
- secret rotation
- backup restore test

## Environment Promotion

When moving between environments:

- apply migrations in a controlled sequence
- validate config definitions and entries
- review feature flag changes
- verify module compatibility
- smoke test core flows and integrations

## Security Operations

- rotate secrets
- review admin access
- review service principal permissions
- review audit trails for high-risk operations
- validate policy changes before activation

## Related Guides

- [Deployment](./deployment.md)
- [Configuration](./configuration.md)
- [Integration](./integration.md)
