# Integration

This guide explains how Orbyte connects to other systems, services, and AI runtimes.

## Integration Philosophy

Orbyte is designed to be integrated, not isolated.

The platform provides explicit boundaries for:

- external business systems
- machine operators
- workflow automation
- AI agents and copilots

Integration should happen through governed APIs, contracts, tools, and events rather than private internal shortcuts.

## Integration Model

The core integration concepts are:

- external systems
- endpoints
- adapters
- contracts
- mappings
- submissions
- attempts
- retries
- dead letters

This model supports both synchronous and asynchronous enterprise integration.

## Reference Integration Patterns

Common patterns for product deployments:

- application-to-platform
  - browser, mobile, portal, or backend app calling HTTP APIs
- agent-to-platform
  - external AI runtime using MCP and selected HTTP endpoints
- platform-to-system
  - Orbyte sending submissions or events to ERP, payment, or operational systems
- system-to-platform
  - external system invoking Orbyte APIs or feeding migration/import flows

## Adapter Boundary

Adapters encapsulate how Orbyte speaks to external systems.

Current built-in direction includes:

- proof and test adapters
- HTTP bridge style adapter boundary

Over time, product deployments can add more concrete connectors as reusable packages.

## Contract Model

Orbyte uses explicit contracts for externally meaningful payloads.

Current contract areas include:

- HTTP APIs
- event schemas
- integration submission schemas
- MCP tool and resource contracts

See [API and Contracts](./api-and-contracts.md) and [External Contract Governance](./contracts.md).

## MCP and AI Agents

For external AI platforms, the most relevant surface is MCP.

Orbyte can expose:

- read-only resources
- governed tools
- control-plane data
- workflow and analytics operations
- integration and configuration controls

This lets an external agent use Orbyte as an enterprise system of action and context without embedding the agent directly into the kernel.

## Recommended AI Integration Pattern

For most teams:

1. keep the agent runtime outside Orbyte
2. authenticate through service principals or delegated machine access
3. expose only approved HTTP and MCP capabilities
4. enforce permission checks and policy hooks
5. capture audit trails for machine actions

### Example: Copilot For Purchase Requests

An external copilot can:

1. search draft requests
2. read workflow context
3. suggest missing data
4. call an approved submit tool
5. observe the resulting workflow state

The copilot should not bypass:

- document APIs
- workflow logic
- permission checks
- audit recording

## Service Principals

Use service principals when:

- another service must invoke Orbyte non-interactively
- an AI platform needs machine credentials
- scheduled integration or automation is required

Use delegated or acting contexts when machine access must operate on behalf of a user.

## Event-Driven Integration

Orbyte supports event-driven patterns through:

- domain events
- outbox processing
- optional external event sinks such as NATS
- retry and dead-letter handling

This is useful for:

- downstream analytics
- search projections
- ERP synchronization
- notifications
- workflow side effects

## ERP, POS, and MIS Integration Examples

### ERP Integration

Use Orbyte as:

- operational frontend and workflow layer
- integration boundary to accounting, warehouse, and procurement systems

### POS Integration

Use Orbyte as:

- offline-capable transaction and sync kernel
- policy and configuration layer for branches and terminals

### MIS Integration

Use Orbyte as:

- governed records and workflow system
- reporting and analytics source
- assistant-ready operational backend

## Data Exchange and Migration

The platform also includes DataOps capabilities for:

- backup
- export
- restore
- archive
- migration artifact registration

These are useful for product rollout, tenant onboarding, recovery, and bulk movement workflows.

## Integration Design Recommendations

- make contracts explicit and versioned
- define idempotency behavior for write operations
- separate synchronous command flows from asynchronous event flows
- keep mapping logic visible and testable
- treat dead-letter queues as an operational responsibility, not a rare exception

## Productization Recommendations

If Orbyte is shipped as a product, publish:

- connector setup guides
- service principal onboarding guide
- idempotency and retry guide
- webhook and event examples
- MCP catalog reference for external AI teams
- reference integration blueprints for ERP, POS, and MIS deployments

## Related Guides

- [Architecture](./architecture.md)
- [Features](./features.md)
- [API and Contracts](./api-and-contracts.md)
