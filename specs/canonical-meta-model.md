# Canonical Meta-Model Specification

## 1. Purpose

This document defines the domain-agnostic canonical meta-model for the platform kernel.

Its purpose is to provide a stable shared vocabulary for building multiple domain packs such as clinic, ERP, OMS, and POS on top of the same platform.

The meta-model defines platform-level object types, their responsibilities, their relationships, and the rules for how they may be specialized by domain packs.

This specification must remain domain-neutral.

---

## 2. Goals

- provide a common platform vocabulary
- separate reusable mechanics from domain meaning
- standardize shared lifecycle, audit, workflow, and integration concepts
- reduce accidental duplication across domain packs
- support consistent APIs, storage patterns, events, and extension contracts

---

## 3. Non-Goals

This document does not define:

- domain-specific business objects such as patient, purchase order, shipment, or cashier shift
- detailed workflow states for any one domain
- provider-specific integration payloads
- tenant-isolation strategies beyond single-tenant deployment assumptions
- UI layout or frontend framework details

---

## 4. Meta-Model Principles

1. **Domain-neutral naming**  
   Shared model types must use names valid across multiple industries.

2. **Mechanics over meaning**  
   The meta-model defines reusable mechanics, not business-specific semantics.

3. **Stable core, extensible edge**  
   Core objects should remain stable while allowing domain packs to contribute fields, policies, and workflows.

4. **Server-authoritative state**  
   Finalized and approved state lives on the server as the source of truth.

5. **Explicit lifecycle and auditability**  
   Important object transitions and actions must be explicit and traceable.

6. **Read/write separation where needed**  
   Canonical records may differ from search or reporting projections.

7. **Relationships are first-class**  
   The platform must model links between records explicitly.

---

## 5. Core Object Families

The canonical meta-model is organized into the following families:

- organization and identity
- master entities and references
- transactional documents
- workflow and actions
- policy and validation
- events and jobs
- projections and retrieval
- output and attachments
- external integration
- configuration and feature control

---

## 6. Canonical Types

### 6.1 `Organization`

Represents the deployment owner or operating business context.

Examples of use:

- company
- foundation
- clinic operator
- retail operator

Core responsibilities:

- legal or operating identity
- root configuration scope
- ownership boundary for branches and locations
- default policy scope

Typical attributes:

- `organization_id`
- `code`
- `name`
- `status`
- `default_locale`
- `default_timezone`
- `settings_ref`
- `created_at`
- `updated_at`

### 6.2 `Location`

Represents a physical or operational place where work occurs.

Examples of use:

- branch
- clinic site
- warehouse
- store
- service point

Typical attributes:

- `location_id`
- `organization_id`
- `location_type`
- `code`
- `name`
- `parent_location_id`
- `status`
- `address_ref`
- `operating_schedule_ref`

### 6.3 `Party`

Represents a person or organization that participates in business processes.

Examples of use:

- customer
- supplier
- employee
- practitioner
- payer

This is a canonical abstraction. Domain packs may define specialized party roles without changing the core type.

Typical attributes:

- `party_id`
- `party_kind` (`person`, `organization`)
- `display_name`
- `status`
- `primary_identifier_ref`
- `primary_contact_ref`
- `created_at`
- `updated_at`

### 6.4 `User`

Represents an authenticated system actor.

Core responsibilities:

- authentication identity
- session ownership
- action attribution
- role bindings

Typical attributes:

- `user_id`
- `username`
- `status`
- `party_id` (nullable)
- `primary_location_id` (nullable)
- `created_at`
- `updated_at`

### 6.5 `Role`

Represents a named permission grouping.

Typical attributes:

- `role_id`
- `code`
- `name`
- `status`
- `scope_type`
- `created_at`
- `updated_at`

### 6.6 `Permission`

Represents an atomic or grouped authorization capability.

Typical attributes:

- `permission_key`
- `module`
- `action`
- `resource_kind`
- `description`

### 6.7 `Entity`

Represents a slow-changing master record managed as a stable business object.

Examples of use:

- catalog item
- tariff
- product
- employee profile
- patient profile

`Entity` is a generic modeling concept. A domain pack may implement one or more concrete entity types.

Common attributes:

- `entity_id`
- `entity_type`
- `status`
- `version`
- `created_at`
- `updated_at`

### 6.8 `Document`

Represents a transactional or governed business record that moves through an explicit lifecycle.

Examples of use:

- registration
- order
- invoice
- payment
- prescription
- return authorization

Core responsibilities:

- lifecycle state
- versioning and concurrency
- numbering
- approval/workflow binding
- auditability

Required canonical attributes:

- `document_id`
- `document_type`
- `status`
- `version`
- `etag`
- `number` (nullable until assigned)
- `organization_id`
- `location_id` (nullable)
- `workflow_definition_key`
- `created_by`
- `created_at`
- `updated_by`
- `updated_at`
- `submitted_by` (nullable)
- `submitted_at` (nullable)
- `approved_by` (nullable)
- `approved_at` (nullable)
- `finalized_at` (nullable)

### 6.9 `DocumentLine`

Represents a subordinate line item within a document.

Examples of use:

- order line
- invoice line
- service line
- medication line

Typical attributes:

- `document_line_id`
- `document_id`
- `line_no`
- `line_type`
- `payload`
- `amount_ref` (nullable)
- `quantity_ref` (nullable)

### 6.10 `Attachment`

Represents a file or binary object linked to a canonical record.

Typical attributes:

- `attachment_id`
- `owner_type`
- `owner_id`
- `storage_ref`
- `mime_type`
- `file_name`
- `size_bytes`
- `status`
- `uploaded_by`
- `uploaded_at`

### 6.11 `Link`

Represents an explicit relationship between two canonical records.

Examples:

- order -> invoice
- registration -> encounter
- invoice -> payment

Typical attributes:

- `link_id`
- `source_type`
- `source_id`
- `target_type`
- `target_id`
- `link_type`
- `created_at`

### 6.12 `WorkflowInstance`

Represents the active workflow state of a governed object.

Typical attributes:

- `workflow_instance_id`
- `workflow_definition_key`
- `owner_type`
- `owner_id`
- `current_state`
- `status`
- `started_at`
- `ended_at` (nullable)

### 6.13 `Task`

Represents a unit of human or system work associated with a workflow or action.

Examples:

- approval task
- review task
- fulfillment task
- exception resolution task

Typical attributes:

- `task_id`
- `task_type`
- `owner_type`
- `owner_id`
- `assignee_user_id` (nullable)
- `assignee_role_id` (nullable)
- `status`
- `due_at` (nullable)
- `priority`
- `created_at`
- `completed_at` (nullable)

### 6.14 `Action`

Represents an explicit state-changing operation invoked by a user or system.

Examples:

- submit
- approve
- reject
- reopen
- cancel
- post
- dispatch

Typical attributes:

- `action_key`
- `owner_type`
- `owner_id`
- `actor_id`
- `actor_kind` (`user`, `system`, `job`)
- `requested_at`
- `result_status`
- `reason` (nullable)
- `metadata`

### 6.15 `Policy`

Represents a named rule set or decision logic reference.

Examples:

- numbering policy
- approval threshold policy
- branch access policy
- pricing eligibility policy

Typical attributes:

- `policy_key`
- `policy_type`
- `version`
- `scope_type`
- `scope_id` (nullable)
- `status`
- `definition_ref`
- `effective_from`
- `effective_to` (nullable)

### 6.16 `ValidationRule`

Represents a machine-evaluable validation contract.

Typical attributes:

- `rule_key`
- `rule_type`
- `target_type`
- `version`
- `severity`
- `definition_ref`

### 6.17 `Event`

Represents a meaningful business or system occurrence.

Examples:

- document submitted
- payment posted
- workflow rejected
- bundle refreshed
- connector failed

Typical attributes:

- `event_id`
- `event_type`
- `event_version`
- `aggregate_type`
- `aggregate_id`
- `occurred_at`
- `actor_id` (nullable)
- `correlation_id` (nullable)
- `causation_id` (nullable)
- `payload`

### 6.18 `Job`

Represents an asynchronous processing unit.

Typical attributes:

- `job_id`
- `job_type`
- `status`
- `payload`
- `retry_count`
- `run_after`
- `last_error` (nullable)
- `correlation_id` (nullable)
- `created_at`
- `updated_at`

### 6.19 `Projection`

Represents a read-optimized derived model used for search, lists, dashboards, or exports.

Typical attributes:

- `projection_key`
- `projection_type`
- `source_type`
- `source_id`
- `projection_version`
- `payload`
- `refreshed_at`

### 6.20 `ExternalReference`

Represents the mapping between an internal canonical record and an external identifier.

Examples:

- payment provider reference
- government registry identifier
- marketplace order id
- interoperability resource id

Typical attributes:

- `external_reference_id`
- `owner_type`
- `owner_id`
- `external_system_key`
- `external_id`
- `reference_type`
- `status`
- `last_verified_at` (nullable)

### 6.21 `Template`

Represents a versioned output template used for print, export, or notifications.

Typical attributes:

- `template_id`
- `template_type`
- `template_key`
- `version`
- `status`
- `channel`
- `definition_ref`

### 6.22 `Configuration`

Represents structured deployment or module configuration.

Typical attributes:

- `configuration_key`
- `scope_type`
- `scope_id` (nullable)
- `version`
- `status`
- `value_ref`
- `updated_at`

### 6.23 `FeatureFlag`

Represents a runtime toggle controlling optional behavior.

Typical attributes:

- `flag_key`
- `scope_type`
- `scope_id` (nullable)
- `enabled`
- `rollout_metadata`
- `updated_at`

---

## 7. Canonical Relationships

The following relationships are part of the meta-model:

- `Organization` has many `Location`
- `User` may map to one `Party`
- `Role` grants many `Permission`
- `Entity` and `Document` may belong to an `Organization`
- `Document` has many `DocumentLine`
- `Document` may have many `Attachment`
- `Document` and `Entity` may have many `Link`
- `Document` may have one active `WorkflowInstance`
- `WorkflowInstance` may produce many `Task`
- `Action` operates on an `Entity`, `Document`, `Task`, or `WorkflowInstance`
- `Policy` applies to one or more canonical types
- `ValidationRule` targets a canonical type or action
- `Event` references an aggregate such as `Document`, `Task`, or `Entity`
- `Job` may be caused by an `Event`
- `Projection` derives from one or more canonical sources
- `ExternalReference` may attach to `Party`, `Entity`, `Document`, or `Location`
- `Template` may render from a `Document`, `Projection`, or `Event` context

---

## 8. Canonical Lifecycle Concepts

The meta-model standardizes lifecycle concepts without fixing domain-specific state names.

### 8.1 Draft and Final State

- mutable draft-like states are allowed
- finalized states are immutable except through controlled amend, reopen, or cancel mechanisms
- state names are domain-defined but lifecycle semantics are platform-defined

### 8.2 Versioning

- canonical records must support explicit versioning where governed mutation matters
- server-accepted changes increment version
- concurrency tokens such as `etag` protect against stale writes

### 8.3 Auditability

- meaningful actions must produce auditable events
- workflow transitions must be attributable to actor and time
- generated outputs and sensitive exports must be traceable

---

## 9. Extensibility Rules

Domain packs may extend the meta-model by:

- defining specialized document types
- defining specialized entity types
- attaching additional JSON or structured fields through approved schema extension points
- registering domain policies
- registering validation rules
- registering projection definitions

Domain packs may not:

- alter the semantics of canonical identifiers
- bypass versioning or audit rules for governed objects
- redefine shared lifecycle mechanics
- introduce domain-specific mandatory fields into all canonical objects

---

## 10. Storage Guidance

The canonical meta-model does not require a single storage pattern, but the preferred default is:

- relational headers for indexed operational fields
- structured or JSONB bodies for extensible payloads
- separate tables for versions, links, audit events, tasks, jobs, projections, and external references

Recommended storage behavior:

- keep search-critical fields indexed outside arbitrary payload blobs
- store relationship records explicitly rather than inferring them from document bodies only
- separate authoritative records from read-optimized projections

---

## 11. API Guidance

API boundaries should reflect canonical type families.

Examples:

- identity/session APIs
- entity APIs
- document submit and action APIs
- workflow/task APIs
- policy/configuration APIs
- reference bundle APIs
- projection/search APIs
- template/output APIs
- integration and external reference APIs

The preferred write model is explicit action-based APIs for governed state changes.

---

## 12. Event Guidance

Every important canonical type may emit events.

Examples:

- `document.created`
- `document.submitted`
- `document.finalized`
- `workflow.transitioned`
- `task.assigned`
- `task.completed`
- `job.failed`
- `projection.refreshed`
- `external_reference.verified`

Event naming should remain generic and stable. Domain packs may define specialized event families under their own namespaces.

---

## 13. Domain Mapping Examples

### 13.1 Clinic Example

- `patient profile` -> specialized `Entity` or `Party`
- `encounter` -> `Document`
- `prescription` -> `Document`
- `doctor approval` -> `Action` + `WorkflowInstance`
- `SATUSEHAT identifier` -> `ExternalReference`

### 13.2 OMS Example

- `customer` -> `Party`
- `sales order` -> `Document`
- `order line` -> `DocumentLine`
- `fulfillment review` -> `Task`
- `marketplace order id` -> `ExternalReference`

### 13.3 POS Example

- `cash sale` -> `Document`
- `receipt line` -> `DocumentLine`
- `store` -> `Location`
- `cashier` -> specialized `Party` + `User`
- `receipt print template` -> `Template`

### 13.4 ERP Example

- `supplier` -> `Party`
- `purchase request` -> `Document`
- `approval threshold` -> `Policy`
- `journal export file` -> `Template` or `Projection`
- `vendor external code` -> `ExternalReference`

---

## 14. Shared Primitive Candidates

The following reusable primitives may be defined as supporting shared types once reuse is validated:

- `Money`
- `Currency`
- `Quantity`
- `UnitOfMeasure`
- `Address`
- `ContactMethod`
- `Identifier`
- `TimeRange`
- `ScheduleWindow`
- `TaxComponent`

These should be introduced as shared value objects, not as domain modules.

---

## 15. Governance Rules

- canonical type definitions must remain domain-neutral
- any new canonical type must show cross-domain value
- domain-specific aliases are allowed only outside the kernel
- removal or semantic change of a canonical type requires compatibility review
- event and API naming must align with canonical type families

---

## 16. Final Summary

The canonical meta-model defines the shared platform vocabulary needed to build multiple domain packs on the same foundation.

Its core purpose is to keep the platform kernel stable, domain-neutral, and extensible while allowing domain packs to express real business meaning through specialization.

The most important canonical types are:

- `Organization`
- `Location`
- `Party`
- `User`
- `Role`
- `Entity`
- `Document`
- `DocumentLine`
- `Attachment`
- `Link`
- `WorkflowInstance`
- `Task`
- `Action`
- `Policy`
- `ValidationRule`
- `Event`
- `Job`
- `Projection`
- `ExternalReference`
- `Template`
- `Configuration`
- `FeatureFlag`

These types form the stable modeling layer beneath domain packs such as clinic, ERP, OMS, and POS.
