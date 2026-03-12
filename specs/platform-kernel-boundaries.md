# Platform Kernel Boundaries Specification

## 1. Purpose and Scope

This document defines the architectural boundary between the reusable platform kernel, domain packs, and external adapters.

Its goals are to:

- prevent domain-specific logic from leaking into shared platform code
- preserve reuse across multiple business domains such as clinic, ERP, OMS, and POS
- provide a repeatable decision framework for placing new capabilities
- enforce clear dependency and extension rules

This specification applies to backend modules, frontend extension points, data models, workflows, policies, integrations, and operational tooling.

---

## 2. Architectural Layers

The platform is divided into three layers:

### 2.1 Platform Kernel

The platform kernel contains reusable enterprise mechanics that are valid across multiple domains.

The kernel owns:

- identity and access enforcement
- organization and location scope
- document lifecycle mechanics
- workflow and task execution mechanics
- policy evaluation mechanisms
- audit and compliance recording
- eventing, outbox, and background jobs
- search projections and list retrieval mechanics
- template rendering and output infrastructure
- generic integration contracts
- configuration and operational controls

The kernel does not own business meaning.

### 2.2 Domain Packs

Domain packs contain business vocabulary, business documents, business workflows, and domain-specific rules.

Examples:

- clinic domain pack
- ERP domain pack
- OMS domain pack
- POS domain pack

Domain packs implement business meaning by using the contracts provided by the platform kernel.

### 2.3 External Adapters

External adapters contain provider-specific, protocol-specific, or regulation-specific interoperability logic.

Examples:

- payment gateway connector
- tax service connector
- healthcare interoperability connector
- marketplace connector
- messaging provider connector

Adapters translate between internal platform/domain models and external systems.

---

## 3. Boundary Principles

The following principles define what belongs where.

1. **Kernel owns mechanics**  
   The kernel provides general mechanisms such as workflow execution, document versioning, and audit logging.

2. **Domain packs own meaning**  
   A domain pack defines the business concepts, rules, and terminology of a specific business area.

3. **Adapters own interoperability**  
   External communication details belong in adapters, not in the kernel or domain packs.

4. **Policies configure behavior**  
   The kernel should expose policy hooks and evaluation mechanisms, while domain packs contribute policy definitions.

5. **Events connect modules**  
   Cross-module extensibility should happen through explicit events and extension contracts, not implicit coupling.

6. **Audit is non-bypassable**  
   All meaningful actions must be auditable through shared kernel facilities.

7. **Workflow owns transitions**  
   State transitions must always pass through workflow or action execution mechanisms.

8. **If unsure, keep it out of core first**  
   New logic should stay in a domain pack unless there is clear evidence of cross-domain reuse.

---

## 4. Kernel Inclusion Rules

A capability belongs in the platform kernel only if most of the following are true:

- it is useful across multiple business domains
- it can be described using domain-neutral vocabulary
- it provides a reusable mechanism rather than domain meaning
- it is foundational to multiple modules
- it can be exposed through stable contracts
- it can be validated without depending on a specific industry model

A capability must not be placed in the kernel if any of the following are true:

- it requires domain-specific business vocabulary
- it encodes a single-domain business process
- it depends on a named external provider or regulatory scheme
- it is only meaningful to one business family
- it changes the semantics of shared kernel contracts for one domain only

---

## 5. Platform Kernel Module Definitions

The initial kernel is composed of the following modules.

### 5.1 `identity_access`

Responsibilities:

- users
- roles
- permissions
- sessions
- access scopes
- authentication and authorization enforcement

This module must stay domain-neutral and must not include business-specific role semantics.

### 5.2 `organization`

Responsibilities:

- company or organization profile
- branch and location model
- operating unit scope
- deployment-level settings
- organizational policies that are generic in shape

### 5.3 `document_kernel`

Responsibilities:

- generic document header/body model
- versioning
- numbering mechanism
- attachments and links
- lifecycle metadata
- concurrency control

This module defines document mechanics, not domain document types.

### 5.4 `workflow_task_kernel`

Responsibilities:

- state machine execution
- allowed transitions
- action execution contracts
- approval mechanics
- human task queues
- role-based work assignment

This module must not contain domain-specific state names unless contributed by a domain pack.

### 5.5 `policy_engine`

Responsibilities:

- policy registration
- policy evaluation
- guard conditions
- threshold checks
- eligibility and authorization predicates
- numbering or assignment policy hooks

The engine is generic. Concrete policy definitions are contributed by domain packs or deployment configuration.

### 5.6 `audit_compliance`

Responsibilities:

- immutable audit events
- admin action logs
- print/export tracking
- request and job correlation
- accountability metadata

### 5.7 `eventing_jobs`

Responsibilities:

- domain event publication
- transactional outbox
- background jobs
- retries and backoff
- dead-letter handling
- idempotent job execution support

### 5.8 `search_projection`

Responsibilities:

- read models
- summary projections
- list retrieval contracts
- search result shaping
- projection refresh contracts

The kernel owns the mechanism. Domain packs define the contents of their projections.

### 5.9 `reference_masterdata`

Responsibilities:

- reference bundles
- shared metadata distribution
- versioned reference delivery
- reference synchronization contracts

### 5.10 `template_output`

Responsibilities:

- template registration
- print rendering
- export rendering
- notification rendering infrastructure
- official versus draft output mechanics

### 5.11 `integration_kernel`

Responsibilities:

- external identifier registry
- projection contracts for external exchange
- mapping contracts
- connector interfaces
- reconciliation contracts
- integration tracking mechanics

This module must not contain provider-specific payload schemas.

### 5.12 `configuration_featureflags`

Responsibilities:

- deployment settings
- module settings
- feature toggles
- runtime configuration lookup
- configuration validation

### 5.13 `observability_ops`

Responsibilities:

- structured logging contracts
- metrics and health reporting
- diagnostics
- admin operational controls
- worker and queue visibility

---

## 6. Domain Pack Rules

Domain packs own:

- business vocabulary
- business documents and entities
- business workflow definitions
- domain-specific validation rules
- domain-specific policies
- domain-specific search projections
- domain-specific UI and reports

Domain packs may:

- register document types
- contribute workflow definitions
- contribute action definitions
- contribute policy definitions
- contribute search projection definitions
- contribute templates
- subscribe to allowed events

Domain packs may not:

- bypass kernel workflow execution
- bypass authorization checks
- bypass audit recording
- redefine shared kernel semantics
- embed provider-specific connector logic as part of the kernel

Examples of domain pack content:

- clinic: patient, encounter, prescription
- ERP: purchase order, invoice posting, asset lifecycle
- OMS: order, fulfillment, return authorization
- POS: sale, receipt, cashier shift

---

## 7. External Adapter Rules

Adapters own:

- external provider communication
- protocol-specific authentication
- payload transformation
- outbound submission and inbound reconciliation
- provider-specific retry/error interpretation

Adapters may:

- consume published domain events or approved projections
- use integration kernel contracts
- persist integration tracking data through approved interfaces

Adapters may not:

- directly redefine business workflows
- directly mutate authoritative business state outside approved application services
- inject provider-specific types into shared kernel contracts
- replace domain validation with provider rules

Examples of adapter content:

- payment gateway connector
- healthcare interoperability connector
- e-commerce marketplace connector
- tax authority connector
- messaging provider connector

---

## 8. Dependency Rules

Allowed dependency directions:

- kernel modules may depend only on other kernel modules
- domain packs may depend on kernel modules
- adapters may depend on kernel modules and published domain contracts

Forbidden dependency directions:

- kernel -> domain pack
- kernel -> adapter
- domain pack -> provider-specific adapter internals
- adapter -> private domain internals that are not published contracts

Dependency intent:

- the kernel remains reusable and stable
- domains remain replaceable and independent
- adapters remain isolated from core semantics

---

## 9. Extension Model

Extension into the platform must happen only through explicit contracts.

Approved extension surfaces include:

- document type registration
- workflow registration
- action registration
- policy registration
- event subscription
- search projection registration
- template registration
- configuration registration

All extensions must be:

- versioned
- validated at startup
- deterministic in registration order
- visible in operational/admin diagnostics

The default deployment model is startup-registered modules, not live runtime hot-plugging.

---

## 10. Promotion Rules

A capability may be promoted from a domain pack into the kernel only when:

- it has been implemented in more than one domain
- its naming can be made domain-neutral without distortion
- its rules can be expressed as a reusable mechanism
- its dependencies do not introduce domain-specific coupling
- promotion reduces duplication without weakening boundaries

Promotion must include:

- boundary review
- contract definition
- migration and compatibility analysis
- examples from at least two domains

---

## 11. Decision Framework

For any new capability, evaluate the following questions:

1. Is it useful across at least two or three different business domains?
2. Can it be named without industry-specific vocabulary?
3. Is it a reusable mechanism rather than business meaning?
4. Must it be trusted centrally by many modules?
5. Can it be exposed through a stable contract?
6. Does it avoid coupling to a named provider or regulation?

Decision rule:

- mostly yes -> platform kernel
- business meaning or industry workflow -> domain pack
- external provider/protocol/regulatory interoperability -> adapter

If uncertainty remains, default to domain pack first.

---

## 12. Example Classification Table

| Capability | Layer | Reason |
| --- | --- | --- |
| document numbering engine | kernel | reusable mechanism across many domains |
| role-based approval execution | kernel | generic workflow mechanic |
| audit log | kernel | compliance and traceability are shared concerns |
| patient profile | domain pack | clinic-specific business concept |
| purchase order | domain pack | ERP-specific business document |
| cashier shift | domain pack | POS-specific operating concept |
| pricing threshold rule | kernel + domain policy | generic rule engine, domain-owned rule definition |
| payment gateway connector | adapter | external provider interoperability |
| healthcare FHIR mapper | adapter | industry-specific external mapping |
| marketplace order importer | adapter | provider/protocol-specific integration |

---

## 13. Shared Primitive Candidates

Some primitives may become part of the kernel or a shared platform primitive layer if they prove cross-domain reuse.

Examples:

- money and currency
- unit of measure
- tax calculation primitives
- address and contact structures
- party and identity references
- item or catalog references
- calendar and operating schedule primitives

These should not be added to the kernel prematurely. They should be promoted only after reuse is demonstrated.

---

## 14. Governance Rules

- adding a new kernel capability requires explicit cross-domain justification
- new kernel contracts must use domain-neutral naming
- domain-specific shortcuts are forbidden in shared packages
- all kernel contract changes require compatibility review
- all kernel extension points must be documented and testable
- if a domain needs special behavior, prefer policy/configuration or domain extension before changing core semantics

---

## 15. Change Control for Kernel Additions

Any proposal to add or promote a capability into the kernel must include:

- problem statement
- why domain-local implementation is insufficient
- expected reuse across multiple domains
- proposed API or contract
- dependency impact
- data model impact
- migration and backward-compatibility impact
- operational and observability impact
- at least two domain examples

---

## 16. Final Boundary Summary

The platform architecture must preserve three clear responsibilities:

- **platform kernel** provides reusable enterprise mechanics
- **domain packs** provide business meaning and business process
- **external adapters** provide interoperability with outside systems

The platform remains reusable only if the kernel stays domain-neutral, the domain packs stay responsible for business meaning, and adapters stay isolated from core semantics.
