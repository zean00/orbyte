# Features

This guide summarizes the major capabilities already present in the Orbyte platform.

## Core Platform Features

- modular kernel bootstrapping
- PostgreSQL and in-memory runtime modes
- manifest-driven business extensions
- metadata-driven model definitions and records
- transactional document lifecycle management
- configurable workflow and approval routing
- scoped configuration and feature flags
- role-based access control
- service principal support
- audit events and domain events
- job execution and asynchronous processing
- idempotency support
- runtime health and monitoring

## Enterprise Application Features

### Identity and Security

- users, roles, permissions, sessions
- service principals for system access
- delegated execution and acting-context support
- authentication policy configuration
- optional Google sign-in

### Data and Transactions

- generic model registry and CRUD
- document records with status, version, links, lines, and attachments
- reference and master data support
- scoped configuration values
- search index definitions

### Workflow and Governance

- workflow definitions, drafts, versions, and publish flow
- runtime workflow tasks and approvals
- reporting-line hierarchy support
- Rego-backed or code-backed policy hooks

### Integration

- external systems and endpoints
- adapter registry
- contracts and mappings
- submission tracking
- retries, dead letters, and replay flow
- HTTP integration adapter boundary

### Analytics and Reporting

- metrics and dashboards
- saved analytics queries
- report definitions and delivery
- report channels including download, filesystem, webhook, email, and object store

### Templates and Output

- template definitions
- draft and publish flow
- preview and render support
- printable or export-ready output generation

### Offline and Field Support

- offline bootstrap and package endpoints
- offline sync batches
- conflict tracking
- projection and reference packaging for disconnected clients

## AI Integration Readiness Features

These are especially important if Orbyte is used as a backend for external AI agents:

- stable generic business APIs
- MCP server and tool catalog
- permission-aware tool exposure
- audit trail for machine-invoked actions
- service-principal access model
- config and policy control plane
- structured search interfaces including keyword, vector, and hybrid query modes

## Product Strengths

The strongest current platform qualities are:

- a broad kernel surface
- a serious module model
- explicit integration concepts
- governance-aware runtime behavior
- good automated test coverage across packages

## Product Gaps To Close Over Time

As the product matures, typical areas to deepen include:

- more first-class enterprise business modules
- more public API documentation and examples
- richer production deployment guidance
- stronger tenancy and compliance documentation
- more ready-made connectors for external systems
