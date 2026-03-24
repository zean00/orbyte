# Components

This guide describes the main components of the Orbyte platform.

## Application Entry Points

- `cmd/server`
  - starts the HTTP server and background runtime
- `cmd/migrate`
  - applies and inspects database migrations
- `cmd/modulegen`
  - scaffolds new modules for this repository

## Kernel Services

### Config Service

Stores configuration definitions and scoped values. Used to control runtime behavior without code changes.

### Identity Service

Manages users, roles, permissions, sessions, service principals, and delegated execution patterns.

### Module Service

Registers module manifests and exposes module-contributed capabilities to the rest of the platform.

### Model Service

Provides metadata-driven records, relations, defaulting, compute rules, and constraints.

### Document Service

Handles transactional records with lifecycle, versioning, and workflow integration.

### Workflow Service

Stores workflow definitions, versions, drafts, routing, and execution state.

### Policy Service

Evaluates configurable governance logic through rule configuration or Rego modules.

### Integration Service

Manages external systems, adapters, contracts, mappings, submissions, retries, and dead letters.

### Search Service

Manages search indexes and query execution. Can attach to external search infrastructure such as Typesense.

### Reporting and Analytics

Supports datasets, metrics, dashboards, saved queries, reports, and delivery channels.

### Eventing and Audit

Captures domain events, audit records, and outbox-driven follow-up behavior.

### Jobs

Executes asynchronous platform work such as integration processing, retries, and data operations.

### DataOps

Handles backup, export, restore, archive, and migration-input registration workflows.

### Template Output

Supports template design, draft management, and rendered output generation.

## Delivery Surfaces

### HTTP APIs

Used by browsers, integrations, services, and custom applications.

### UI Contract Endpoints

Provide headless route, menu, and view metadata so a generic shell can render platform functionality.

### Admin APIs

Support runtime administration, module management, security management, and governance operations.

### MCP Server

Exposes governed tools and resources for external agent clients and machine operators.

## Persistence Components

- PostgreSQL schema and migrations
- in-memory repositories for development and tests
- optional external sinks such as NATS, SMTP, object storage, and Typesense

## Cross-Cutting Components

- observability
- runtime health
- localization
- field-level security
- idempotency
- secret store

## Component Boundaries

A useful way to understand the product is:

- modules define capability
- services execute capability
- HTTP and MCP expose capability
- policy and identity govern capability
- audit and eventing record capability
