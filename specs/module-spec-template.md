# Module Specification Template

## 1. Module Identity

- `module_key`:
- `name`:
- `domain_family`:
- `version`:
- `status`:
- `owner`:

---

## 2. Purpose

Describe the business purpose of the module.

Include:

- what business capability this module provides
- why it exists
- which users or actors depend on it

---

## 3. Scope

### In Scope

-

### Out of Scope

-

### Upstream Dependencies

-

### Downstream Dependencies

-

---

## 4. Domain Glossary

List the key business terms used in this module.

| Term | Meaning | Synonyms | Notes |
| --- | --- | --- | --- |
|  |  |  |  |

---

## 5. Business Actors

List the human and system actors involved.

| Actor | Type | Responsibilities | Scope Rules |
| --- | --- | --- | --- |
|  |  |  |  |

---

## 6. Canonical Mapping

Map module concepts to platform kernel concepts.

| Business Concept | Kernel Type | Notes |
| --- | --- | --- |
|  |  |  |

Examples of kernel types:

- `Entity`
- `Document`
- `DocumentLine`
- `Task`
- `WorkflowInstance`
- `Event`
- `Projection`
- `ExternalReference`

---

## 7. Owned Objects

### Entities

| Entity Type | Purpose | Key Fields | Notes |
| --- | --- | --- | --- |
|  |  |  |  |

### Documents

| Document Type | Purpose | Has Lines | Numbered | Workflow |
| --- | --- | --- | --- | --- |
|  |  |  |  |  |

### Supporting Objects

| Object Type | Purpose | Notes |
| --- | --- | --- |
|  |  |  |

---

## 8. Source of Truth Rules

Define what is authoritative and what is derived.

- authoritative records:
- derived/projection records:
- local/client draft behavior:
- external-system-owned data, if any:
- data that must never be silently overwritten:

---

## 9. Business Rules

List the core business rules.

### Validation Rules

-

### Calculation or Derivation Rules

-

### Policy-Sensitive Rules

-

### Forbidden Behaviors

-

---

## 10. Workflow Model

### Workflow Summary

- workflow key:
- target type:
- approval model:

### State Definitions

| State | Meaning | Editable | Terminal |
| --- | --- | --- | --- |
|  |  |  |  |

### Action Catalog

| Action | From State | To State | Actor | Notes |
| --- | --- | --- | --- | --- |
|  |  |  |  |  |

### Transition Rules

-

### Exception Paths

-

### Override Rules

-

---

## 11. Permission and Scope Matrix

| Action | Permission Key | Allowed Roles | Scope Constraints | Notes |
| --- | --- | --- | --- | --- |
|  |  |  |  |  |

Include:

- read/list permissions
- detail view permissions
- create/edit draft permissions
- submit/approve/reject/reopen/cancel permissions
- override or sensitive-access permissions

---

## 12. Data Model Contracts

### Header Fields

| Field | Type | Indexed | Required | Notes |
| --- | --- | --- | --- | --- |
|  |  |  |  |  |

### Body Schema Notes

-

### Line Model

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
|  |  |  |  |

### Links

| Link Type | Target | Purpose |
| --- | --- | --- |
|  |  |  |

### Attachments

- supported attachment types:
- attachment rules:

---

## 13. Numbering and Identity Rules

- numbering required:
- numbering trigger:
- numbering scope:
- identifier rules:
- external identifier behavior, if any:

---

## 14. Search and Projection Requirements

### Summary Projection

| Field | Purpose | Visible To |
| --- | --- | --- |
|  |  |  |

### Worklist Projection

| Projection | Purpose | Filters | Sorts |
| --- | --- | --- | --- |
|  |  |  |  |

### Dashboard or Aggregate Views

-

### Rebuild Rules

-

---

## 15. API and Action Requirements

### Core Endpoints or Action APIs

| Endpoint or Action | Purpose | Notes |
| --- | --- | --- |
|  |  |  |

### Request/Response Notes

-

### Concurrency Rules

-

---

## 16. Events

### Emitted Events

| Event | Trigger | Payload Notes |
| --- | --- | --- |
|  |  |  |

### Consumed Events

| Event | Purpose | Notes |
| --- | --- | --- |
|  |  |  |

### Event Rules

-

---

## 17. Integration Requirements

### External Systems

| System | Direction | Purpose | Adapter |
| --- | --- | --- | --- |
|  |  |  |  |

### Outbound Flow

-

### Inbound Flow

-

### Reconciliation Rules

-

---

## 18. Frontend Requirements

### Routes and Screens

| Route/Screen | Purpose | Notes |
| --- | --- | --- |
|  |  |  |

### Forms and Lists

-

### UX Rules

-

### Local Draft / Offline Notes

-

---

## 19. Templates and Outputs

| Template/Output | Purpose | Trigger | Official or Draft |
| --- | --- | --- | --- |
|  |  |  |  |

---

## 20. Observability and Operations

### Key Metrics

-

### Failure Modes

-

### Admin Diagnostics Needed

-

---

## 21. Testing Requirements

### Required Test Areas

- permission matrix
- workflow transition matrix
- validation rules
- projection refresh
- event emission
- integration behavior
- end-to-end happy path
- key exception paths

### Sample Test Cases

-

---

## 22. AI Implementation Context Pack

Before asking AI to implement this module, provide:

- this completed module spec
- relevant kernel specs
- target package/file paths
- coding conventions
- acceptance criteria
- sample payloads and expected outputs
- constraints on what AI may not change

### Mandatory Examples for AI

- valid example record
- invalid example record
- happy-path workflow example
- exception-path example
- example projection row
- example permission decision case

### Hallucination Risk Checks

Mark as complete before AI implementation:

- [ ] glossary is complete
- [ ] workflow states are defined
- [ ] permissions are explicit
- [ ] source-of-truth rules are explicit
- [ ] examples are provided
- [ ] out-of-scope rules are explicit

---

## 23. Acceptance Criteria

| Area | Acceptance Criteria |
| --- | --- |
| Domain |  |
| Workflow |  |
| Permissions |  |
| API |  |
| Search |  |
| Integration |  |
| Frontend |  |
| Operations |  |

---

## 24. Phased Implementation Plan

### Phase 1

-

### Phase 2

-

### Phase 3

-

---

## 25. Open Questions

-

---

## 26. Change Log

| Date | Version | Change | Author |
| --- | --- | --- | --- |
|  |  |  |  |
