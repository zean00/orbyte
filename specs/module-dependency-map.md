# Platform Module Dependency Map

## 1. Purpose

This document defines the dependency structure of the domain-agnostic platform kernel.

Its purpose is to make module relationships explicit so that implementation order, interface ownership, testing boundaries, and future change impact remain clear.

This dependency map complements the architectural specifications by describing how the kernel modules depend on each other, which dependencies are allowed, and which modules should be built first.

---

## 2. Goals

- define allowed module dependency directions
- identify foundational modules versus higher-level modules
- support phased implementation planning
- reduce circular dependency risk
- clarify which modules own shared contracts

---

## 3. Dependency Principles

1. **Lower-level modules expose stable contracts**  
   Foundational modules should be depended on by higher-level modules, not the other way around.

2. **No circular dependencies**  
   Modules must communicate through published contracts, events, or application services instead of direct cycles.

3. **Cross-cutting concerns stay reusable**  
   Shared concerns such as identity, workflow, events, and configuration must not become trapped inside business-specific modules.

4. **Write-path authority stays clear**  
   Document, workflow, audit, and event modules must preserve a clear authoritative path for state changes.

5. **Read models depend on authoritative modules**  
   Search and projection logic must derive from authoritative records, not define them.

---

## 4. Kernel Module List

The platform kernel is composed of these primary modules:

- `shared_primitives`
- `organization_scope`
- `identity_access`
- `configuration_featureflags`
- `document_kernel`
- `workflow_task_policy`
- `event_outbox_consistency`
- `integration_kernel`
- `search_projection`

Supporting cross-cutting concerns are represented inside or alongside these modules:

- audit and observability contracts
- validation contracts
- template/output hooks

---

## 5. Dependency Layers

The recommended dependency layering is:

### Layer 0: Value and Contract Foundation

- `shared_primitives`

### Layer 1: Structural and Security Context

- `organization_scope`
- `identity_access`
- `configuration_featureflags`

### Layer 2: Core Transactional Mechanics

- `document_kernel`
- `workflow_task_policy`
- `event_outbox_consistency`

### Layer 3: Derived and Externalized Behavior

- `search_projection`
- `integration_kernel`

Higher layers may depend on lower layers, but lower layers must not depend on higher layers.

---

## 6. Module Dependency Summary

### 6.1 `shared_primitives`

Purpose:

- shared value objects and reusable primitive semantics

May depend on:

- none, except minimal language/runtime utilities

Used by:

- all other modules

Examples of exports:

- `Money`
- `Quantity`
- `Identifier`
- `Address`
- `TimeRange`

### 6.2 `organization_scope`

Purpose:

- organization, location, operating unit, and scope resolution

May depend on:

- `shared_primitives`

Used by:

- `identity_access`
- `configuration_featureflags`
- `document_kernel`
- `workflow_task_policy`
- `search_projection`
- `integration_kernel`

Key exports:

- structural models
- scope resolution contracts
- scope context interfaces

### 6.3 `identity_access`

Purpose:

- users, roles, permissions, sessions, service principals, authorization decisions

May depend on:

- `shared_primitives`
- `organization_scope`
- `configuration_featureflags`

Used by:

- `document_kernel`
- `workflow_task_policy`
- `integration_kernel`
- `search_projection`

Key exports:

- authenticated principal
- authorization decision service
- role and permission contracts

### 6.4 `configuration_featureflags`

Purpose:

- structured runtime configuration and feature toggles

May depend on:

- `shared_primitives`
- `organization_scope`

Used by:

- `identity_access`
- `document_kernel`
- `workflow_task_policy`
- `event_outbox_consistency`
- `integration_kernel`
- `search_projection`

Key exports:

- configuration resolution service
- feature flag resolution service
- validation/activation contracts

### 6.5 `document_kernel`

Purpose:

- governed business documents, versioning, numbering, links, attachments, persistence contracts

May depend on:

- `shared_primitives`
- `organization_scope`
- `identity_access`
- `configuration_featureflags`

Used by:

- `workflow_task_policy`
- `event_outbox_consistency`
- `search_projection`
- `integration_kernel`

Key exports:

- document contracts
- document repositories/services
- document type registration contracts

### 6.6 `workflow_task_policy`

Purpose:

- state machines, tasks, approvals, action execution, policy evaluation contracts

May depend on:

- `shared_primitives`
- `organization_scope`
- `identity_access`
- `configuration_featureflags`
- `document_kernel`

Used by:

- `event_outbox_consistency`
- `search_projection`
- `integration_kernel`

Key exports:

- workflow definitions
- action execution contracts
- task contracts
- policy evaluation interfaces

### 6.7 `event_outbox_consistency`

Purpose:

- event model, transactional outbox, jobs, idempotency, retry semantics

May depend on:

- `shared_primitives`
- `configuration_featureflags`
- `document_kernel`
- `workflow_task_policy`

Used by:

- `search_projection`
- `integration_kernel`

Key exports:

- event envelope
- outbox contracts
- job contracts
- idempotency contracts

### 6.8 `search_projection`

Purpose:

- summary read models, search, queues, dashboards, projection refresh

May depend on:

- `shared_primitives`
- `organization_scope`
- `identity_access`
- `configuration_featureflags`
- `document_kernel`
- `workflow_task_policy`
- `event_outbox_consistency`

Used by:

- domain read APIs and admin dashboards

Key exports:

- projection definitions
- search contracts
- projection status interfaces

### 6.9 `integration_kernel`

Purpose:

- external references, mapping/projection contracts, submissions, reconciliation

May depend on:

- `shared_primitives`
- `organization_scope`
- `identity_access`
- `configuration_featureflags`
- `document_kernel`
- `workflow_task_policy`
- `event_outbox_consistency`

Used by:

- external adapters
- domain integration services

Key exports:

- integration projection contracts
- submission tracking contracts
- reconciliation contracts

---

## 7. Dependency Matrix

Legend:

- `D` = direct dependency allowed
- `-` = no direct dependency expected

| Module | shared_primitives | organization_scope | identity_access | configuration_featureflags | document_kernel | workflow_task_policy | event_outbox_consistency | search_projection | integration_kernel |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `shared_primitives` | - | - | - | - | - | - | - | - | - |
| `organization_scope` | D | - | - | - | - | - | - | - | - |
| `identity_access` | D | D | - | D | - | - | - | - | - |
| `configuration_featureflags` | D | D | - | - | - | - | - | - | - |
| `document_kernel` | D | D | D | D | - | - | - | - | - |
| `workflow_task_policy` | D | D | D | D | D | - | - | - | - |
| `event_outbox_consistency` | D | - | - | D | D | D | - | - | - |
| `search_projection` | D | D | D | D | D | D | D | - | - |
| `integration_kernel` | D | D | D | D | D | D | D | - | - |

---

## 8. Dependency Graph Narrative

The dependency graph centers on four ideas:

- `shared_primitives` is the universal foundation
- `organization_scope`, `identity_access`, and `configuration_featureflags` provide structural and control context
- `document_kernel`, `workflow_task_policy`, and `event_outbox_consistency` form the authoritative write-path core
- `search_projection` and `integration_kernel` sit on top as derived and external-facing layers

This means implementation should flow from foundation and control context into write-path modules, then into read and external layers.

---

## 9. Forbidden Dependency Rules

The following direct dependencies are forbidden:

- `shared_primitives` depending on any higher-level module
- `organization_scope` depending on `identity_access`, `document_kernel`, or higher layers
- `document_kernel` depending on `workflow_task_policy` or `event_outbox_consistency`
- `workflow_task_policy` depending on `search_projection` or `integration_kernel`
- `event_outbox_consistency` depending on `search_projection` or `integration_kernel`
- `search_projection` and `integration_kernel` depending on each other directly unless a narrow shared contract is extracted downward

If a forbidden dependency is needed, the shared abstraction should be moved into a lower module.

---

## 10. Cross-Cutting Concern Placement

### 10.1 Audit

Audit is a cross-cutting concern and should be implemented as a shared service contract used by:

- `identity_access`
- `document_kernel`
- `workflow_task_policy`
- `event_outbox_consistency`
- `integration_kernel`

Audit must not introduce reverse dependencies into foundational modules.

### 10.2 Validation

Validation contracts should be split by concern:

- primitive validation in `shared_primitives`
- structural/scope validation in `organization_scope`
- identity and credential validation in `identity_access`
- document contract validation in `document_kernel`
- workflow/policy validation in `workflow_task_policy`

### 10.3 Templates and Output

Template/output concerns should depend on:

- `document_kernel`
- `configuration_featureflags`
- optionally `search_projection`

They should not define authoritative workflow or document state.

---

## 11. Domain Pack Dependency Rules

Domain packs may depend on kernel modules, but should do so in a disciplined way.

### 11.1 Recommended Domain Dependencies

Most domain packs will depend on:

- `shared_primitives`
- `organization_scope`
- `identity_access`
- `configuration_featureflags`
- `document_kernel`
- `workflow_task_policy`
- `event_outbox_consistency`
- optionally `search_projection`
- optionally `integration_kernel`

### 11.2 Domain Rules

- domains must not create alternate core identity or workflow engines
- domains may add document types, permissions, policies, projections, and integration mappings
- domains must not bypass kernel write-path or authorization rules

---

## 12. Adapter Dependency Rules

External adapters should depend only on the contracts they need.

### 12.1 Recommended Adapter Dependencies

Adapters typically depend on:

- `shared_primitives`
- `configuration_featureflags`
- `identity_access` for service principal context where needed
- `event_outbox_consistency`
- `integration_kernel`

Adapters may also consume published domain contracts where necessary, but should avoid direct dependence on broad domain internals.

### 12.2 Adapter Rules

- adapters must not depend directly on `search_projection` for authoritative decisions
- adapters must not mutate authoritative document state outside approved application services
- provider-specific behavior must stay isolated from kernel modules

---

## 13. Suggested Build Order

Recommended implementation sequence:

1. `shared_primitives`
2. `organization_scope`
3. `configuration_featureflags`
4. `identity_access`
5. `document_kernel`
6. `workflow_task_policy`
7. `event_outbox_consistency`
8. `search_projection`
9. `integration_kernel`

This order minimizes rework and keeps dependency arrows pointing downward.

---

## 14. Testing Strategy by Dependency Layer

### 14.1 Foundation Layer

Test:

- value semantics
- validation rules
- serialization stability

### 14.2 Structural and Security Layer

Test:

- scope resolution
- role binding behavior
- permission evaluation
- configuration resolution

### 14.3 Write-Path Core

Test:

- document versioning
- workflow transitions
- idempotent actions
- event and outbox commit behavior

### 14.4 Derived and External Layers

Test:

- projection rebuilds
- search visibility
- integration retry and reconciliation behavior

---

## 15. Refactoring Rules

If a module starts needing a higher-layer concern frequently, use one of these responses:

- extract a lower-level shared contract
- move generic logic downward
- convert direct coupling into event-driven integration
- split the module if it mixes authoritative and derived concerns

Do not solve dependency tension by allowing reverse imports.

---

## 16. Final Summary

The platform dependency map establishes a clear implementation shape:

- `shared_primitives` is the foundation
- `organization_scope`, `configuration_featureflags`, and `identity_access` provide structural and control context
- `document_kernel`, `workflow_task_policy`, and `event_outbox_consistency` form the authoritative core
- `search_projection` and `integration_kernel` are derived layers built on top of the core

This dependency structure reduces circular coupling, supports phased delivery, and keeps the platform reusable across multiple business domains.
