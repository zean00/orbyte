# Features

<p class="page-intro">
This guide summarizes the current feature set reflected in the repository today.
</p>

## Read This Page By Area

<div class="quick-links" markdown>

- [**Platform Features**](#platform-features)
  Kernel, security, data, workflow, search, reporting, and operations.
- [**MCP Features**](#current-mcp-features)
  Full vs minimal mode, skill-based discovery, and machine-access behavior.
- [**Business Areas**](#business-features-by-area)
  CRM, commercial, inventory, finance, workforce, and POS.
- [**Current Constraints**](#current-product-constraints)
  Understand where the runtime is still maturing.

</div>

## Platform Features

### Kernel And Runtime

- modular service graph construction
- in-memory and PostgreSQL-backed repository modes
- runtime health, observability, and audit instrumentation
- scoped runtime configuration with typed definitions
- profile-driven business manifest loading

### Identity And Security

- users, roles, permissions, sessions
- service principals
- delegated acting-context support
- configurable auth policy
- Google sign-in and auto-provisioning support
- TOTP policy fields in the auth configuration model

### Data, Documents, And Workflow

- generic model registry and CRUD support
- document definitions and document lifecycle support
- workflow definitions, approvals, and related flow metadata
- module-owned datasets and search definitions
- application-level actions for document/model mutation flows

### Search, Analytics, And Reporting

- analytics widgets and dashboard surfaces
- datasets defined by modules
- reporting and template output services
- report delivery adapters:
  - download
  - filesystem
  - webhook
  - email
  - object store
- search service with attachable model/document sources

### Integration And Operations

- external eventing configuration via NATS
- search integration via Typesense
- integration service and submission-style boundaries
- jobs and asynchronous follow-up paths
- idempotency service
- admin and operational endpoints

### Agent And Machine Access

- ACP provider-backed sessions
- MCP JSON-RPC endpoints
- switchable MCP exposure modes
- minimal MCP skill/tool discovery flow
- service-principal and delegated-user execution
- generated MCP contract artifacts

<div class="orbyte-note">
The current platform is agent-ready, not agent-dependent. Agent access goes through ACP sessions and governed MCP tools.
</div>

## Current MCP Features

### Full MCP mode

- exposes the broader direct business tool surface
- intended for admin/debug and direct-tool use cases

### Minimal MCP mode

Current primary minimal tools:

- `skills.find`
- `skills.describe`
- `tools.find`
- `tools.describe`
- `tools.call`

Current minimal design goals:

- reduce prompt/catalog size
- encourage workflow-first discovery
- narrow business tool selection before execution

### Skill-based discovery

Current skills/playbooks can express:

- `use_when`
- ordered `workflow_steps`
- `tool_inventory`
- `required_final_facts`
- `required_artifacts`
- `required_draft_outputs`
- `guardrails`
- `success_checks`
- `pitfalls`

## UI Features

The current workspace model supports:

- surface-aware navigation
- generic list/detail/form rendering from module metadata
- dashboard widgets
- worklist and self-service routes
- agent workspace shell
- admin shell

## Business Features By Area

### CRM

- service ticketing
- queues, SLA policies, assignment rules
- ticket comments and ticket activities
- customer 360 payloads
- lead and opportunity tracking
- sales/activity summaries and dashboard widgets
- MCP tools for ticket, customer, lead, and opportunity flows

### Commercial

- catalog and commercial master views
- document and record search through business tools
- pricing/promotion related module composition
- templates and workflow-linked commercial document shapes

### Inventory And Operations

- procurement
- inventory
- fulfillment
- delivery
- returns and supplier returns
- planning
- production and production costing
- traceability and recall
- POS

### Finance

- reporting core
- manual journals
- collections
- treasury
- fixed assets
- retail finance
- inventory finance

### Workforce

- employee workforce
- workforce attendance
- leave policy
- payroll
- payroll remittance
- employee spend

## Current Product Strengths

- broad kernel-pack coverage for operational business domains
- serious MCP and ACP integration work
- strong manifest-driven modeling
- runtime-configurable machine exposure and governance
- seeded demo and validation scenarios for real workflows

## Current Product Constraints

The current codebase still has these practical limits:

- some agent/provider runs can stall mid-turn
- some domains are deeper than others
- profile modules under `internal/modules` are still sparse compared to built-in kernel packs
- some advanced enterprise workflows remain metadata-first foundations rather than full productized end-user flows

## Related Guides

- [Architecture](./architecture.md)
- [Modules](./modules.md)
- [Agent Integration](./agent-integration.md)
- [Surfaces](./surfaces.md)

## Recommended Next Pages

<div class="next-steps" markdown>

- [Modules](./modules.md) for the concrete current module inventory
- [Agent Integration](./agent-integration.md) for ACP/MCP behavior and minimal mode

</div>
