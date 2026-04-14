# MCP Target Architecture

This guide defines the target architecture for MCP in Orbyte.

It is the canonical engineering specification for how Orbyte should expose business context and governed actions to external agentic AI tools.

## Purpose and Positioning

Orbyte is intended to support two modes at the same time:

- traditional business-as-usual operation as a governed system of record
- AI-native business operation where external agents can analyze, investigate, recommend, and safely assist through governed machine interfaces

In Orbyte:

- ACP is the runtime bridge to external agent providers and session runtimes
- MCP is the canonical business contract used by those agents
- the agent workspace is the human-facing orchestration surface for ACP-backed sessions and MCP-backed analysis/actions

Orbyte should remain agent-ready, not agent-dependent.

This means:

- Orbyte does not introduce a separate proprietary agent framework beyond ACP, MCP, and the human-facing agent workspace
- business logic, workflow, permissions, audit, and policy remain inside the core platform and business modules
- external agentic tools connect plug-and-play through ACP and MCP

## Architectural Principles

### Business-Semantic, Not UI-Shaped

MCP should expose business meaning, not frontend mechanics.

Tools should be organized around:

- discovery
- investigation
- explanation
- recommendation support
- governed business action

MCP should not devolve into:

- one tool per screen
- one tool per route
- one tool per generic CRUD endpoint with no business context

### Same Governance For Human And Machine Operators

Agents must use the same governed business core as human operators.

An MCP action must not bypass:

- permission checks
- policy hooks
- workflow transitions
- approval requirements
- period-close controls
- audit and event recording

### External Runtime, Internal Business Contract

ACP and MCP have different roles:

- ACP manages provider connectivity, sessions, prompts, approvals, and streamed runtime interaction
- MCP exposes the business contract that external agents use to understand and act on Orbyte

ACP should not become a second business API.
MCP should not become a second agent runtime.

### Draft-First And Explicitly Governed Action

The default machine-write behavior should be:

- inspect
- analyze
- recommend
- create or update draft
- submit only where explicitly allowed

Side-effectful actions should require explicit confirmation and clear runtime metadata.

### Comprehensive Business Legibility

To support useful enterprise agents, MCP must make the business legible across:

- modules and capabilities
- master data
- transactions
- relationships
- workflow state
- configuration
- reporting and KPIs
- exceptions, reconciliation, and variances
- audit and operational history

## Target Capability Model

MCP should expose five capability layers.

### 1. Discovery Layer

This layer lets an agent discover what exists in the platform.

It should expose:

- modules
- module descriptions
- business capabilities
- owned documents
- owned models
- references and datasets
- dependencies and reverse dependents
- available actions
- governance metadata

This is the foundation for business exploration and advisor-style reasoning.

### 2. Operational Read Layer

This layer lets an agent inspect the current state of the business.

It should expose:

- configuration state
- master data
- transactional records
- workflow status
- document and model relationships
- attachments, notes, activities, and timeline where relevant
- current exceptions, pending approvals, and operational backlog

The output should be business-oriented and scoped, not only raw storage payloads.

### 3. Analytical Layer

This layer lets an agent understand the business, not just read records.

It should expose:

- KPIs and datasets
- trend and period comparison views
- aging and backlog views
- reconciliation and settlement views
- inventory and fulfillment health
- production variance and costing views
- treasury and cash visibility
- master-data quality and completeness checks
- anomaly and exception summaries

This is the layer that enables investigation and recommendations.

### 4. Governed Action Layer

This layer lets an agent assist safely.

It should expose:

- draft creation
- draft update
- guided submit actions where allowed
- safe operational helpers
- explicit corrective-draft paths for exceptions

This layer must be metadata-rich and governance-aware.

### 5. Control-Plane And Governance Layer

This layer lets operators and the agent workspace understand MCP itself.

It should expose:

- runtime catalog and inventories
- tool enable/disable state
- action class and risk metadata
- permission requirements
- scope requirements
- audit class
- approval and confirmation requirements

## Business Comprehension Requirements

For Orbyte to function as an AI-native enterprise platform, MCP must let agents see the business comprehensively.

That means the platform should be understandable through MCP in terms of:

- business topology
  - what modules exist
  - what they do
  - what they own
  - how they depend on each other
- business state
  - current configuration
  - master data
  - transactions
  - workflow state
  - responsibilities and ownership
- business history
  - recent changes
  - timeline and activity context
  - audit and approval trail
  - corrections, reversals, and resolution history
- business relationships
  - order to invoice to payment
  - item to stock to procurement to production
  - treasury to settlement to reconciliation
  - asset to depreciation to lifecycle event
  - party to customer/vendor profile to transactions
- business intervention paths
  - what can be drafted
  - what can be submitted
  - what requires approval
  - what is blocked by policy or accounting controls

## Action Model

MCP actions should be explicitly classified.

Recommended action classes:

- `read`
- `analyze`
- `recommend`
- `draft`
- `submit`
- `controlled_mutation`

Expected default behavior:

- `read` and `analyze` are non-mutating
- `recommend` returns reasoning, gaps, or proposed changes
- `draft` creates or updates draft-state artifacts only
- `submit` is only exposed where the user and workflow rules permit it
- `controlled_mutation` is reserved for explicitly approved machine actions and should remain rare

Each side-effectful tool should expose governance metadata such as:

- side-effect class
- requires confirmation
- draft-only
- approval required
- audit action
- required permissions
- required scopes

## MCP Surface Taxonomy

MCP should support three tool categories plus resources and apps.

### Generic Built-In Business Tools

These provide platform-wide discovery, search, read, relationship traversal, analytical summaries, and governed draft actions.

These should be the default path for:

- business discovery
- cross-domain investigation
- generic draft creation and draft updates

### Synthetic Module Wrappers

These are generated from enabled module manifests and pre-bind the generic tools to a module context.

They improve discoverability without introducing bespoke logic for every module.

They should exist for:

- module info
- module record search
- module document search
- module draft create/update where applicable

### Specialized Hand-Authored Tools

These should exist only where the business behavior is not well expressed by generic discovery/read/draft patterns.

Examples:

- domain-specific calculation helpers
- guided reconciliation flows
- packaged business operations with richer semantics

### Resources And Apps

Resources and apps should continue to serve:

- read-only published context
- control-plane views
- UI-linked machine experiences where appropriate

## ACP, MCP, And Agent Surface Roles

### ACP

ACP is responsible for:

- provider configuration
- session creation and runtime lifecycle
- prompt transport
- approvals and streamed interaction

ACP should not be the place where business capabilities are modeled.

### MCP

MCP is responsible for:

- business discovery
- business investigation
- analytical visibility
- governed action contracts

MCP is the stable machine-facing business interface.

### Agent Workspace

The agent workspace is responsible for:

- human review of agent interaction
- session orchestration
- approval UX
- context attachment
- presentation of MCP-backed analysis and action proposals

It should orchestrate MCP-backed work, not replace MCP.

## Advisor Pack Model

Advisor packs are curated business capability bundles built on top of MCP.

They are not separate agent runtimes and should not introduce a second orchestration framework.

Each advisor pack should define:

- what business problems it addresses
- what generic MCP tools it depends on
- what specialized analytical tools it adds, if any
- what governed draft actions it can invoke
- what success criteria apply

Recommended initial advisor packs:

- pricing and promotion advisor
- tax structure advisor
- treasury and reconciliation advisor
- inventory health advisor
- customer and vendor master advisor

Over time, these packs can expand into:

- production cost and margin advisor
- workforce and approval advisor
- retail operations advisor
- asset lifecycle advisor

## Governance And Safety Model

MCP must preserve enterprise governance.

Required properties:

- user-scoped authorization by default
- optional service-principal and delegated contexts where explicitly supported
- permission parity with UI and HTTP surfaces
- explicit scope handling across deployment, organization, location, and operating unit contexts
- auditable action classes and side-effect classes
- confirmation for side-effectful calls
- approval-aware behavior for draft, submit, and controlled mutation paths

The audit model should make it clear:

- who initiated the request
- whether the action was agent-mediated
- what MCP tool was invoked
- what confirmation or approval path was used

## Target Roadmap

### Phase 1: Foundation

Strengthen MCP as the platform-wide business discovery and read layer.

Deliver:

- richer module and capability catalog
- clearer tool metadata
- consistent distinction between generic, synthetic, and specialized MCP tools
- platform-wide discovery of documents, models, references, datasets, and actions

### Phase 2: Business Comprehension

Add cross-domain analytical capability so agents can investigate the business.

Deliver:

- KPI and dataset-oriented MCP tools
- relationship traversal and timeline helpers
- exception, backlog, aging, reconciliation, and variance views
- master-data completeness and configuration quality checks

### Phase 3: Safe Action Governance

Formalize the governed action layer.

Deliver:

- action classes and side-effect classes
- draft-first write behavior
- tool-level confirmation and approval metadata
- runtime enforcement aligned with workflow, finance, and policy controls

### Phase 4: Advisor Packs

Package high-value enterprise reasoning capabilities on top of the generic MCP layer.

Deliver initial packs for:

- pricing and promotion
- tax structure
- treasury and reconciliation
- inventory health
- customer and vendor master quality

### Phase 5: Enterprise Operations

Deepen cross-domain operational intelligence and machine-governed assistance.

Deliver:

- richer trend and anomaly analysis
- better playbook support in the agent workspace
- stronger action governance and audit views
- operational observability for MCP usage and business impact

## Engineering Implications

This target architecture implies that Orbyte should continue to invest in:

- richer module metadata
- generic business-semantic MCP tools
- analytical and relationship-oriented tools, not only search/list/get
- governance metadata in MCP contracts
- ACP and agent workspace features that surface MCP risk and approval semantics clearly

It also implies that new business modules should be designed to be:

- discoverable
- readable in business terms
- analytically visible
- safely draftable

## Related Guides

- [Architecture](./architecture.md)
- [Integration](./integration.md)
- [API and Contracts](./api-and-contracts.md)
- [Security and Governance](./security-and-governance.md)
- [Glossary](./glossary.md)
