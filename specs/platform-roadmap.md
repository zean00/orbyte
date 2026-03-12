# Platform Roadmap

## 1. Purpose

This document defines the recommended implementation roadmap for the domain-agnostic platform kernel.

Its purpose is to translate the architecture specifications into a practical build sequence that minimizes dependency risk, delivers usable increments early, and preserves clean module boundaries.

This roadmap assumes:

- single-tenant-per-deployment
- modular monolith implementation first
- startup-registered modules rather than runtime hot-pluggable modules
- domain-agnostic kernel first, with domain packs added on top

---

## 2. Roadmap Goals

- implement the platform in dependency-safe phases
- deliver a usable kernel foundation before domain expansion
- reduce rework by building lower-level contracts first
- validate architecture with at least one real domain pack
- create a path from specification to execution backlog

---

## 3. Guiding Delivery Principles

1. **Build bottom-up, validate top-down**  
   Foundational modules should be implemented first, but validated using real domain use cases.

2. **Prefer thin end-to-end slices after the foundation exists**  
   Once the kernel base is stable, prove it with one complete flow instead of isolated module work only.

3. **Keep authoritative write-path core ahead of read and adapter layers**  
   Document, workflow, authorization, and events should be stable before search and external integration expand.

4. **Operational safety before feature breadth**  
   Audit, idempotency, retries, and configuration control should arrive early.

5. **Promote reuse only after proof**  
   If a concept is not yet proven cross-domain, keep it narrow until reuse is demonstrated.

---

## 4. Roadmap Inputs

This roadmap is based on the following specifications:

- `platform-kernel-boundaries.md`
- `canonical-meta-model.md`
- `shared-enterprise-primitives-spec.md`
- `organization-scope-spec.md`
- `identity-access-spec.md`
- `configuration-featureflags-spec.md`
- `document-kernel-spec.md`
- `workflow-task-policy-spec.md`
- `event-outbox-consistency.md`
- `search-projection-spec.md`
- `integration-kernel-spec.md`
- `module-dependency-map.md`

---

## 5. Phase Overview

Recommended build phases:

1. foundation bootstrap
2. structural and control context
3. authoritative document core
4. governed action core
5. event, outbox, and background execution
6. read-model and search layer
7. integration kernel and adapters
8. first domain-pack proof
9. hardening and platform productization

---

## 6. Phase 1: Foundation Bootstrap

### Objectives

- establish repo/module structure
- define shared primitive library
- define canonical core contracts and error model
- establish migration and configuration bootstrap patterns

### Deliverables

- project layout for kernel modules
- shared primitive types and validation helpers
- common error/result model
- config bootstrap loader
- migration framework
- baseline logging and health endpoints

### Modules in focus

- `shared_primitives`

### Exit Criteria

- shared primitives compile and serialize cleanly
- baseline project conventions are documented
- migration path exists for future modules

---

## 7. Phase 2: Structural and Control Context

### Objectives

- implement structural context for scope-aware behavior
- implement configuration and feature flag control plane
- implement authentication, sessions, roles, permissions, and scope-aware authorization

### Deliverables

- organization, location, and operating unit models
- scope resolution service
- configuration and feature flag services
- user, role, permission, session, and service principal models
- authorization decision service
- access audit baseline

### Modules in focus

- `organization_scope`
- `configuration_featureflags`
- `identity_access`

### Exit Criteria

- authenticated user context resolves correctly
- scope-aware RBAC works server-side
- configuration resolution is deterministic and auditable

---

## 8. Phase 3: Authoritative Document Core

### Objectives

- implement the document kernel as the main governed record store
- support headers, bodies, lines, links, attachments, numbering hooks, and concurrency

### Deliverables

- document header and body persistence
- document type registration contract
- version and `etag` handling
- document line model
- document links
- attachment metadata model
- numbering policy binding points

### Modules in focus

- `document_kernel`

### Exit Criteria

- a registered document type can be created, updated, fetched, and version-checked
- stale writes are rejected correctly
- links and attachments work through official APIs

---

## 9. Phase 4: Governed Action Core

### Objectives

- implement workflow, tasks, approvals, and policy-driven protected actions
- bind authorization, state transitions, and policy checks together

### Deliverables

- workflow definition registry
- action execution service
- task model and assignment model
- approval step handling
- policy registration and evaluation interfaces
- audit hooks for actions and transitions

### Modules in focus

- `workflow_task_policy`

### Exit Criteria

- protected actions can move a document through a governed lifecycle
- approvals and tasks work with role and scope constraints
- overrides and denials are auditable

---

## 10. Phase 5: Event, Outbox, and Background Execution

### Objectives

- make authoritative writes safely extensible through events and async jobs
- ensure retries and idempotency are in place before externalized behavior grows

### Deliverables

- event envelope implementation
- transactional outbox tables and dispatch loop
- idempotency store for protected writes
- job runner and job status model
- retry and dead-letter support
- correlation-aware operational logs

### Modules in focus

- `event_outbox_consistency`

### Exit Criteria

- a protected action commits business state, audit, and outbox records together
- async jobs are retry-safe and observable
- failed jobs move through retry and dead-letter flows correctly

---

## 11. Phase 6: Read-Model and Search Layer

### Objectives

- provide efficient list and search retrieval without overloading canonical reads
- define rebuildable projection infrastructure

### Deliverables

- projection definition registry
- projection status tracking
- event-driven projection refresh worker
- first summary projection tables
- search/list API contract
- rebuild and repair operations

### Modules in focus

- `search_projection`

### Exit Criteria

- list pages work from projection-backed summaries
- projection lag and failure are observable
- projections can be rebuilt without altering authoritative state

---

## 12. Phase 7: Integration Kernel and Adapter Base

### Objectives

- establish generic external integration handling after the core platform is stable
- provide submission tracking, reconciliation, and adapter-safe boundaries

### Deliverables

- external system registry
- external reference registry
- mapping profile contract
- integration projection model
- submission record model
- reconciliation case model
- adapter base interfaces

### Modules in focus

- `integration_kernel`

### Exit Criteria

- an outbound submission can be tracked end-to-end
- retries and reconciliation work through the kernel model
- provider-specific adapters can be added without leaking into core modules

---

## 13. Phase 8: First Domain-Pack Proof

### Objectives

- validate the kernel with one realistic domain pack
- prove that the platform is truly reusable, not only theoretically clean

### Recommended first proof

- clinic outpatient-lite domain pack, or
- OMS order-lite domain pack

### Minimum scope for proof

- 2-3 document types
- 1-2 workflow lifecycles
- scope-aware access control
- event-driven projection refresh
- one simple external adapter or mock adapter

### Exit Criteria

- one domain pack runs end-to-end without changing kernel semantics
- at least one protected business flow completes through document, workflow, audit, outbox, projection, and integration layers

---

## 14. Phase 9: Hardening and Productization

### Objectives

- turn the platform from architecture proof into reliable product foundation

### Deliverables

- migration compatibility rules
- performance optimization for low-end devices and modest infrastructure
- admin and operational dashboards
- test suites by module boundary
- disaster recovery and backup procedures
- deployment packaging and environment conventions

### Exit Criteria

- core platform is observable, testable, and supportable
- module boundaries remain intact under real usage
- implementation is ready for broader domain expansion

---

## 15. Recommended MVP Cut

The recommended kernel MVP should include:

- `shared_primitives`
- `organization_scope`
- `identity_access`
- `configuration_featureflags`
- `document_kernel`
- `workflow_task_policy`
- `event_outbox_consistency`
- minimal `search_projection`

The following can be limited in MVP:

- advanced reconciliation tooling
- rich dashboard projections
- multiple adapter families
- advanced override and step-up auth flows

The following should not be skipped even in MVP:

- audit for protected actions
- version/etag handling
- idempotency for important writes
- scope-aware authorization
- explicit document contracts

---

## 16. Recommended First End-to-End Slice

After Phases 1-5 are sufficiently stable, implement one thin end-to-end slice:

1. authenticate user
2. resolve scope
3. create document draft
4. submit document action
5. run workflow transition
6. write audit + outbox
7. refresh projection
8. show list/search result

This slice proves that the core platform is operational before broad feature expansion.

---

## 17. Risks and Mitigations

### Risk 1: Over-generalizing too early

Mitigation:

- keep domain packs narrow until reuse is proven
- promote concepts into core only after repeated need

### Risk 2: Workflow becoming too abstract to use

Mitigation:

- validate with one real domain flow early
- keep action semantics explicit and testable

### Risk 3: Search/read models drifting from truth

Mitigation:

- keep projections rebuildable
- enforce authoritative re-checks for protected actions

### Risk 4: Integration concerns leaking into core

Mitigation:

- keep provider-specific behavior inside adapters
- standardize submission and reconciliation contracts

### Risk 5: Authorization complexity becoming untestable

Mitigation:

- centralize authorization decisions
- define deterministic scope and permission rules
- test by decision matrix, not ad hoc UI behavior

---

## 18. Suggested Team Workstreams

If multiple workstreams are available, the safest parallelization pattern is:

- workstream A: primitives, organization, configuration
- workstream B: identity and authorization
- workstream C: document kernel
- workstream D: workflow and event/outbox after document contracts stabilize
- workstream E: search and integration after core write-path stabilizes

Parallel work should follow the dependency map and avoid inventing duplicate contracts.

---

## 19. Suggested Deliverables by Milestone

### Milestone A: Kernel Foundation

- modules bootstrapped
- primitives finalized
- scope/config/auth baseline implemented

### Milestone B: Governed Write Core

- document lifecycle works
- workflow and actions work
- audit and outbox work

### Milestone C: Usable Read Layer

- search and projections available
- first list screens or APIs practical to use

### Milestone D: Externalization Ready

- integration kernel live
- one adapter path proven

### Milestone E: Domain-Proven Platform

- first domain pack runs end-to-end
- kernel remains stable under real use

---

## 20. Next Planning Outputs

After this roadmap, the most useful execution documents are:

1. `implementation-backlog.md`
2. `milestone-plan.md`
3. `acceptance-criteria-matrix.md`
4. `domain-pack-proof-plan.md`

---

## 21. Final Summary

The platform should be built in a dependency-aware sequence:

- foundation first
- structural and security context second
- authoritative document and workflow core third
- event/outbox safety next
- search and integration after the core stabilizes
- real domain validation before broad expansion

This roadmap gives a practical path from architecture specifications to a working, reusable platform kernel.
