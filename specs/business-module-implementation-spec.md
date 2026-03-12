# Business Module Implementation Specification

## 1. Purpose

This document defines how business modules should be designed and implemented on top of the platform kernel.

Its purpose is to provide a consistent implementation model for domain packs such as clinic, ERP, OMS, and POS while preserving the kernel boundaries defined in `platform-kernel-boundaries.md`.

This specification applies to backend modules, frontend modules, document types, workflows, policies, search projections, integrations, and operational ownership inside a business domain.

---

## 2. Goals

- define a standard structure for business modules
- clarify what a business module may and may not own
- show how business modules plug into kernel services
- standardize module deliverables, contracts, and dependencies
- reduce ad hoc domain implementations that bypass the platform

---

## 3. Non-Goals

This document does not define:

- the detailed schema of a specific business module
- full UI design standards
- provider-specific adapter implementations
- project management backlog details

---

## 4. Design Principles

1. **Business modules own meaning**  
   Business modules define domain vocabulary, business objects, workflows, and rules.

2. **Kernel owns mechanics**  
   Business modules must use kernel facilities for identity, documents, workflow, events, search, and integration.

3. **No bypassing core controls**  
   A business module must not bypass authorization, workflow enforcement, audit, or outbox handling.

4. **Module contracts are explicit**  
   Every module should publish its document types, policies, actions, permissions, projections, and integration points.

5. **Domain logic stays cohesive**  
   A module should own a coherent business capability, not a random collection of unrelated features.

6. **UI and backend module boundaries should align**  
   Frontend routes, actions, and forms should map to the same domain boundaries as backend services.

---

## 5. What Is a Business Module

A business module is a domain-owned capability package that implements a specific business area using the platform kernel.

Examples:

- clinic: `registration`, `triage`, `encounter`, `prescription`, `billing`
- OMS: `order_management`, `fulfillment`, `returns`
- POS: `sales`, `refunds`, `register_control`
- ERP: `procurement`, `payables`, `asset_management`

A business module may contain:

- domain entities
- domain document types
- workflows and actions
- policies and validation rules
- search projections
- UI routes and forms
- reports and templates
- integration mappings

---

## 6. Module Boundary Rules

### 6.1 A Business Module May Own

- domain vocabulary
- domain entities and document types
- domain-specific validation rules
- domain workflows and approval paths
- domain permissions
- domain projections and dashboards
- domain templates and outputs
- domain mapping logic for integrations

### 6.2 A Business Module Must Use From the Kernel

- `identity_access`
- `organization_scope`
- `configuration_featureflags`
- `document_kernel`
- `workflow_task_policy`
- `event_outbox_consistency`
- `search_projection`
- `integration_kernel` where needed

### 6.3 A Business Module Must Not Own

- alternate authentication or authorization engines
- alternate workflow engine for governed actions
- direct provider-specific adapter logic in core business code
- hidden background write paths outside event/outbox rules
- ad hoc search behavior that ignores the projection model

---

## 7. Business Module Standard Structure

Each business module should define at least the following sections or subpackages.

### 7.1 `domain`

Contains:

- domain concepts
- aggregate rules
- document types
- entities
- value object usage
- invariants

### 7.2 `application`

Contains:

- use cases
- action orchestration
- service coordination across kernel contracts
- transaction boundaries

### 7.3 `interfaces`

Contains:

- API handlers
- DTOs
- request/response mapping
- command and query adapters

### 7.4 `projections`

Contains:

- search projection definitions
- dashboard summaries
- queue/worklist models

### 7.5 `policies`

Contains:

- module policy definitions
- approval routing rules
- business guard conditions
- override conditions

### 7.6 `integrations`

Contains:

- module-owned projection definitions for external exchange
- mapping profiles
- reconciliation interpretation rules

### 7.7 `frontend` (if applicable)

Contains:

- module routes
- forms and screens
- action affordances
- permission-aware navigation
- projection-backed list views

---

## 8. Required Module Contracts

Every business module should publish a clear contract set.

### 8.1 Module Manifest

Each module should define:

- `module_key`
- `name`
- `version`
- `domain_family`
- `dependencies`
- `owned_document_types`
- `owned_entity_types`
- `owned_workflow_keys`
- `owned_permission_keys`
- `owned_projection_keys`
- `owned_template_keys`
- `feature_flags`

### 8.2 Document Contracts

For each governed document type, the module must define:

- document type key
- schema version
- header field contract
- line model if any
- lifecycle semantics
- workflow binding
- numbering policy binding
- projection binding
- template binding

### 8.3 Workflow Contracts

For each workflow, the module must define:

- workflow key
- states
- actions
- transitions
- policy bindings
- task creation rules
- approval model

### 8.4 Permission Contracts

The module must define explicit permission keys for:

- read/list access
- create/edit draft access
- protected action execution
- override or privileged actions
- reporting/export access where relevant

### 8.5 Projection Contracts

The module must define:

- search summary projections
- worklist projections
- dashboard projections if needed
- rebuild strategy for each projection

---

## 9. Implementation Model

### 9.1 Domain Model First

A module implementation should begin with:

- business concepts
- canonical document/entity mapping
- lifecycle and workflow
- permissions and scope rules
- key list/search views

### 9.2 Kernel Mapping Second

After domain modeling, map each business concern to kernel capabilities.

Examples:

- business record -> `Document` or `Entity`
- approval flow -> `workflow_task_policy`
- list page -> `search_projection`
- external exchange -> `integration_kernel`

### 9.3 Delivery Slice Third

Then implement an end-to-end slice:

- create or edit draft
- submit or protected action
- workflow transition
- audit + outbox event
- projection refresh
- UI refresh or downstream integration hook

---

## 10. Domain Modeling Rules

### 10.1 Entity vs Document

Use `Entity` when the object is a relatively stable master record.

Use `Document` when the object:

- has lifecycle transitions
- represents a transaction or governed record
- needs approval, numbering, or explicit versioning

### 10.2 Module Cohesion

Keep modules cohesive.

Good examples:

- `encounter` owns clinical encounter lifecycle
- `billing` owns billable compilation and invoice generation

Weak examples:

- one giant `clinic_operations` module containing unrelated concerns with no internal structure

### 10.3 Cross-Module References

- cross-module references should use explicit links and published contracts
- avoid hidden database coupling between modules
- emit events when downstream modules need to react

---

## 11. Workflow and Policy Requirements

Every governed business module should define:

- action catalog
- state model
- approval requirements
- task creation rules
- guard policies
- override rules

Business modules must not implement protected state changes as unrestricted CRUD updates.

---

## 12. Authorization Requirements

Each business module must define:

- who can read summaries
- who can open details
- who can create drafts
- who can edit drafts
- who can submit
- who can approve, reject, reopen, cancel, or override
- which actions depend on location, assignment, ownership, or sensitive access rules

Authorization must be enforced through `identity_access` and `workflow_task_policy`, not only the UI.

---

## 13. Search and Projection Requirements

Each business module should define at least:

- one summary projection for list/search
- one worklist projection if workflow or tasks exist
- key filter and sort fields
- rebuild strategy

Rules:

- do not use full document payloads as default list APIs
- search results must respect module permission and scope rules
- projections must be derived from authoritative records

---

## 14. Integration Requirements

If a business module integrates externally, it should define:

- outbound trigger events
- integration projection shape
- mapping profile ownership
- external reference rules
- reconciliation behavior

Business modules own business meaning of exchange, but provider transport belongs in adapters.

---

## 15. Frontend Module Requirements

Frontend module design should align with backend domain boundaries.

Each module frontend should define:

- route set
- navigation entry
- permission-aware actions
- list/search screens backed by projections
- detail or form views backed by canonical records
- offline/local-draft behavior if relevant

Rules:

- frontend must not invent hidden business rules that differ from backend contracts
- protected actions should call explicit action endpoints
- disabled/hidden controls in UI are convenience only, not security

---

## 16. Module Events

Every business module should define important domain events.

Examples:

- `registration.submitted`
- `encounter.finalized`
- `order.released`
- `sale.completed`
- `purchase_request.approved`

Rules:

- events should align with kernel event envelope rules
- emitted events should represent meaningful domain occurrences
- downstream dependencies should prefer events over hidden direct coupling

---

## 17. Testing Requirements

Each business module should have tests for:

- permission matrix
- workflow transition matrix
- policy decisions
- projection refresh correctness
- integration trigger behavior
- end-to-end happy path
- key rejection/error paths

Recommended test layers:

- domain tests
- application service tests
- API contract tests
- projection refresh tests
- integration mapping tests

---

## 18. Operational Requirements

Each business module should define:

- operational owner
- key metrics
- failure modes
- retry or reconciliation expectations
- admin diagnostics needed

Examples of module-level metrics:

- pending approvals
- submission failure count
- projection lag
- external submission failure count
- task backlog age

---

## 19. Recommended Module Specification Template

Each module spec should follow this structure:

1. module purpose
2. business scope
3. owned entities and document types
4. workflows and actions
5. policies and permissions
6. projections and search
7. integrations
8. frontend surfaces
9. events
10. reports and templates
11. operational metrics and failure handling
12. phased implementation plan

---

## 20. AI-Assisted Development Context Requirements

If business module development will be AI-assisted, the module specification must provide enough context to reduce ambiguity and hallucination.

### 20.1 Mandatory Context

The following context should be considered mandatory before asking AI to design or implement a business module:

- module purpose and business boundary
- glossary of domain terms and synonyms
- in-scope and out-of-scope behaviors
- canonical mapping of business objects to `Entity`, `Document`, `DocumentLine`, `Task`, and `Event`
- state machine and action list
- permission matrix and scope rules
- key policies and validation rules
- source-of-truth rules and ownership boundaries
- upstream and downstream module dependencies
- required projections, reports, and templates
- integration triggers and external systems involved
- example scenarios, including happy path and exception cases
- explicit non-goals and forbidden shortcuts

### 20.2 Minimum Module Context Pack

For accurate AI-assisted implementation, each business module should have a compact but complete context pack containing at least:

1. module summary
2. glossary
3. business rules
4. document and entity contracts
5. workflow diagram or transition table
6. permission matrix
7. API/action expectations
8. search/projection requirements
9. integration notes
10. example records or sample payloads

### 20.3 Mandatory Examples

AI performs better when the module spec includes concrete examples such as:

- example document headers and bodies
- example transitions with allowed actors
- example validation failures
- example search result rows
- example integration payload mapping notes
- example edge cases and exception handling

### 20.4 Mandatory Existing-Code Context

If AI is modifying an existing implementation, provide or reference:

- current module file paths
- existing interfaces and contracts
- related kernel services used by the module
- existing naming conventions
- relevant migrations or table names
- current tests and fixtures

### 20.5 Mandatory Decision Records

Where the domain has tricky rules, include explicit decisions for:

- what is authoritative
- what requires approval
- what can be edited and when
- when numbering is assigned
- what causes events to be emitted
- what must be auditable
- what must never be automated silently

### 20.6 Red Flags That Increase Hallucination Risk

AI output is more likely to be inaccurate when any of these are missing:

- undefined domain terminology
- unclear module boundary
- missing workflow states
- missing permission rules
- unspecified source-of-truth rules
- no examples of valid and invalid cases
- vague references like "same as current process" without written detail

### 20.7 Recommended AI Prompt Input Checklist

Before asking AI to implement a business module, provide:

- the relevant module spec
- the relevant kernel specs
- the target files or package structure
- the exact task to perform
- constraints on what may not be changed
- acceptance criteria
- example inputs and expected outputs

---

## 21. Recommended Implementation Sequence for a Business Module

1. define module purpose and boundaries
2. define domain vocabulary and canonical mappings
3. define entities and document contracts
4. define workflow and action model
5. define permission and scope rules
6. define projection and list/search needs
7. define integration requirements if any
8. define frontend routes and actions
9. implement one end-to-end slice
10. add reports, outputs, and operational tooling

---

## 22. Example Module Breakdown

### 21.1 Clinic `registration`

- documents: registration
- workflows: draft -> submitted -> approved/finalized
- permissions: create, submit, approve, reopen
- projections: current-day registrations, pending approvals
- integrations: optional identifier verification or future payer checks

### 21.2 OMS `order_management`

- documents: sales order, order amendment
- workflows: draft -> submitted -> released -> fulfilled/cancelled
- permissions: create, release, hold, cancel
- projections: open orders, held orders, recent order list
- integrations: marketplace sync, payment status correlation

### 21.3 POS `sales`

- documents: sale, refund
- workflows: draft/open -> completed -> voided/refunded
- permissions: create sale, refund, void, supervisor override
- projections: recent sales, exception queue
- integrations: payment terminal, receipt export, accounting handoff

---

## 23. Governance Rules

- every business module must have a named owner and published contract set
- every governed module must use kernel workflow, audit, and event rules
- business modules must not create hidden direct dependencies on other modules' internal tables or handlers
- module boundaries should be reviewed before implementation begins
- if a module repeatedly reimplements the same pattern as another module, evaluate whether the logic belongs in the kernel or a shared domain library

---

## 24. Relationship to Other Documents

This specification should be used together with:

- `platform-kernel-boundaries.md`
- `canonical-meta-model.md`
- `document-kernel-spec.md`
- `workflow-task-policy-spec.md`
- `identity-access-spec.md`
- `search-projection-spec.md`
- `integration-kernel-spec.md`
- `module-dependency-map.md`
- `platform-roadmap.md`

---

## 25. Final Summary

Business modules are the layer where real business meaning is implemented.

They should:

- own domain vocabulary and business behavior
- use kernel services for mechanics and enforcement
- publish explicit contracts for documents, workflows, permissions, projections, and integrations
- be built as cohesive, testable, and operationally visible units

This gives the platform a repeatable way to implement clinic, ERP, OMS, POS, and future domains without weakening the kernel architecture.
