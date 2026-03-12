# Document Kernel Specification

## 1. Purpose

This document defines the domain-agnostic document kernel for the platform.

Its purpose is to provide a reusable model and operational contract for governed business documents across multiple domains such as clinic, ERP, OMS, and POS.

The document kernel is responsible for document identity, lifecycle metadata, versioning, concurrency, numbering, linkage, attachments, and authoritative persistence behavior.

---

## 2. Goals

- define a reusable document model for multiple domains
- separate document mechanics from domain-specific document meaning
- standardize versioning, lifecycle, and concurrency rules
- support explicit numbering and traceability
- support extensible bodies while preserving indexed operational fields
- define how documents relate to workflow, audit, search, and integration

---

## 3. Non-Goals

This document does not define:

- domain-specific document types such as encounter, order, or purchase request
- domain-specific field schemas in detail
- reporting or analytics models
- UI form design
- full content management or arbitrary file repository behavior

---

## 4. Design Principles

1. **Documents represent governed business records**  
   A document is a business record with explicit lifecycle, authorship, and traceability.

2. **Mechanics stay in core, meaning stays in domains**  
   The kernel defines document behavior. Domain packs define document types, schemas, and business semantics.

3. **Headers are indexed, bodies are extensible**  
   Search-critical and operational fields belong in indexed headers. Flexible content may live in structured bodies.

4. **Authoritative state is server-side**  
   The server is the source of truth for accepted document state, version, and numbering.

5. **Lifecycle is explicit**  
   Important document state changes must occur through governed actions and workflow.

6. **Finalized records are protected**  
   Final documents must not be silently overwritten.

7. **Everything important is linkable and auditable**  
   Documents must support explicit relationships and meaningful audit events.

---

## 5. What Is a Document

A `Document` is a governed transactional or controlled business record that:

- has a stable identity
- may have one or more line items
- moves through an explicit lifecycle
- may require approval or workflow steps
- may receive a formal number
- may link to other documents or entities
- must support versioning and auditability

Examples across domains:

- clinic: registration, encounter, prescription, invoice
- OMS: sales order, shipment request, return authorization
- POS: sale, receipt, refund, cash adjustment
- ERP: purchase request, purchase order, vendor invoice, journal request

---

## 6. Canonical Document Model

### 6.1 Core Document Identity

Every document must include:

- `document_id`
- `document_type`
- `status`
- `version`
- `etag`
- `organization_id`
- `location_id` (nullable)
- `number` (nullable until assigned)
- `schema_version`
- `workflow_definition_key` (nullable)
- `created_by`
- `created_at`
- `updated_by`
- `updated_at`
- `submitted_by` (nullable)
- `submitted_at` (nullable)
- `approved_by` (nullable)
- `approved_at` (nullable)
- `finalized_at` (nullable)

### 6.2 Document Header

The header contains indexed operational metadata used for:

- identity
- filtering
- sorting
- lifecycle control
- authorization context
- search summaries
- cross-module references

Recommended header fields beyond the required identity set may include:

- `owner_party_id` (nullable)
- `counterparty_party_id` (nullable)
- `business_date` (nullable)
- `effective_at` (nullable)
- `priority` (nullable)
- `total_amount` (nullable)
- `currency_code` (nullable)
- `external_status` (nullable)

### 6.3 Document Body

The body contains domain-defined structured content.

Recommended body properties:

- represented as structured JSON, typed payload, or equivalent extensible format
- versioned by schema version
- suitable for validation and hashing
- separated from the header for indexing and flexibility reasons

The body should contain:

- domain-specific fields
- nested sections
- optional domain extensions
- non-index-critical content

### 6.4 Document Lines

Documents may contain one or more subordinate `DocumentLine` records.

Use lines when:

- the document contains repeatable commercial or operational items
- line-level amounts or quantities matter
- line-level references are needed

Each line should support:

- `document_line_id`
- `document_id`
- `line_no`
- `line_type`
- `status` (nullable)
- `catalog_ref` (nullable)
- `quantity` (nullable)
- `unit_price` (nullable)
- `line_amount` (nullable)
- `payload`

---

## 7. Document Type Contract

Every domain-defined document type must register a document contract.

### 7.1 Required Contract Elements

- `document_type`
- `display_name`
- `schema_version`
- `header_field_contract`
- `body_schema_ref`
- `line_schema_ref` (nullable)
- `workflow_definition_key` (nullable)
- `numbering_policy_key` (nullable)
- `search_projection_key` (nullable)
- `print_template_key` (nullable)
- `allowed_link_types`
- `lifecycle_semantics`

### 7.2 Contract Rules

- document type names must be stable
- contract changes must be versioned
- required header fields must remain queryable and well-defined
- every document type must declare whether it is governed by workflow

---

## 8. Lifecycle Model

The kernel does not force domain-specific state names, but it defines common lifecycle semantics.

### 8.1 Common Lifecycle Semantic Classes

- `draft`
- `submitted`
- `under_review`
- `approved`
- `rejected`
- `finalized`
- `cancelled`
- `amended`
- `closed`

### 8.2 Lifecycle Rules

- only allowed actions may change document state
- workflow governs protected transitions where enabled
- finalized documents are immutable except through approved amend, reopen, or cancel mechanisms
- lifecycle transitions must update version and audit history as appropriate

### 8.3 Draft Semantics

- draft-like documents may be edited before finalization or protected submission
- local client drafts are not authoritative documents until accepted by server
- server drafts become authoritative once created or accepted by server according to domain flow

---

## 9. Versioning and Concurrency

### 9.1 Version Rules

- `version` increments on each accepted authoritative change
- `etag` changes whenever the authoritative content or protected metadata changes
- version increments must be monotonic per document

### 9.2 Concurrency Rules

- protected writes must use `etag` or expected version checks
- stale updates must be rejected explicitly
- clients must receive enough information to retry safely

### 9.3 Content Integrity

The kernel may maintain a `content_hash` or equivalent integrity value for:

- conflict diagnostics
- snapshot traceability
- comparison and deduplication support

---

## 10. Numbering Model

Numbering is a kernel mechanism governed by policy.

### 10.1 Numbering Rules

- official numbers are assigned only by the server
- numbering may occur on creation, approval, finalization, or another policy-bound event
- once assigned, official numbers are immutable except through controlled administrative correction

### 10.2 Numbering Inputs

Numbering policy may depend on:

- document type
- organization or location scope
- business date
- sequence family
- legal or operational policy reference

### 10.3 Numbering Traceability

The platform should retain:

- assigned number
- numbering policy used
- assignment timestamp
- assignment actor or system context

---

## 11. Document Links and Relationships

Documents frequently relate to other canonical objects.

### 11.1 Link Model

Links should be explicit and queryable.

Each link should include:

- `link_id`
- `source_type`
- `source_id`
- `target_type`
- `target_id`
- `link_type`
- `created_at`
- `created_by` (nullable)

### 11.2 Example Link Types

- `derived_from`
- `references`
- `settles`
- `replaces`
- `cancels`
- `belongs_to`

### 11.3 Link Rules

- allowed link types must be declared by document contracts
- link creation may require policy or workflow checks
- link changes must be auditable when they affect business meaning

---

## 12. Attachment Model

Documents may have zero or more attachments.

### 12.1 Attachment Rules

- attachments are separate from the canonical document body
- attachment metadata must remain queryable
- large binary content should use external or object storage references where appropriate
- attachment authorization must follow platform security rules

### 12.2 Attachment Metadata

Recommended metadata:

- `attachment_id`
- `document_id`
- `attachment_type`
- `storage_ref`
- `mime_type`
- `file_name`
- `size_bytes`
- `status`
- `uploaded_by`
- `uploaded_at`

---

## 13. Storage Model

The preferred default storage model is hybrid relational plus structured payload.

### 13.1 Recommended Table Families

- `document_headers`
- `document_bodies`
- `document_lines`
- `document_versions`
- `document_links`
- `document_tags` (optional)
- `document_attachments`

### 13.2 Header Storage Rules

- store indexed lifecycle and query fields in header tables
- normalize important references used by authorization or search
- avoid placing search-critical fields only inside arbitrary payload bodies

### 13.3 Body Storage Rules

- store extensible content separately from indexed header metadata
- preserve schema version with the body
- support snapshot or version storage for important changes

### 13.4 Version Storage Rules

Version history may use:

- full snapshots
- structural diffs
- hybrid snapshot-plus-diff approach

The chosen approach must preserve auditability and replay/debug value.

---

## 14. Validation Model

Document validation works at multiple levels.

### 14.1 Validation Layers

- schema validation
- header contract validation
- line validation
- cross-field validation
- policy validation
- workflow transition validation

### 14.2 Validation Rules

- every accepted document write must pass server validation
- local validation may improve UX but is never authoritative
- document type contracts must define required validation references

---

## 15. Document Actions

Documents should be modified through explicit actions where behavior is governed.

### 15.1 Common Actions

- `create`
- `update`
- `submit`
- `approve`
- `reject`
- `finalize`
- `cancel`
- `reopen`
- `amend`
- `link`
- `attach`

### 15.2 Action Rules

- action availability depends on status, permissions, context, and workflow
- protected actions must be auditable
- actions that materially change document state must emit events through the outbox model

---

## 16. Audit Requirements

The document kernel must ensure traceability of meaningful changes.

### 16.1 Auditable Events

- document created
- document updated
- document submitted
- document approved or rejected
- document finalized
- document cancelled or reopened
- number assigned
- attachment added or removed where relevant
- link created or changed where relevant

### 16.2 Recommended Audit Fields

- `event_id`
- `document_id`
- `document_type`
- `action`
- `actor_id`
- `timestamp`
- `from_state` (nullable)
- `to_state` (nullable)
- `version`
- `reason` (nullable)
- `diff_summary` (nullable)

---

## 17. Event Integration Rules

Document changes are a major source of domain events.

### 17.1 Recommended Event Families

- `document.created`
- `document.updated`
- `document.submitted`
- `document.approved`
- `document.rejected`
- `document.finalized`
- `document.cancelled`
- `document.amended`
- `document.number_assigned`

### 17.2 Event Rules

- authoritative write, audit, and outbox records must commit together
- external side effects must occur after commit
- projections and integrations may subscribe to document events

---

## 18. Search and Projection Guidance

Documents are often searched through summary projections rather than full payload reads.

Rules:

- document contracts should identify summary fields needed for search and lists
- list screens should use projections or indexed headers
- full bodies should be fetched only when needed for detailed views or processing

Common summary fields may include:

- document number
- document type
- status
- owner or counterparty display
- key dates
- total amount
- location

---

## 19. Print and Output Guidance

Documents are frequent sources of official and draft output.

Rules:

- document contracts may bind to template keys
- official output should derive from authoritative document state
- draft output must be clearly distinguishable where policy requires it
- output generation should be auditable when it affects business or compliance significance

---

## 20. Integration Guidance

Documents often drive external exchanges.

Rules:

- external payloads should be based on projections or mapped snapshots, not raw uncontrolled document bodies
- external identifiers should be tracked through the integration kernel
- resubmission must preserve linkage to source document and source version

---

## 21. API Guidance

### 21.1 Document APIs

Recommended API categories:

- create or submit document draft
- fetch document header or full document
- execute document action
- list linked documents
- manage attachments
- fetch version history

### 21.2 API Rules

- protected changes should prefer action-style endpoints
- full replacement writes should be used carefully and with concurrency checks
- partial updates must not bypass validation or workflow rules

---

## 22. Example Domain Mapping

### 22.1 Clinic

- `encounter` -> document
- `prescription line` -> document line
- `invoice number` -> numbering policy output
- `encounter -> invoice` -> document link

### 22.2 OMS

- `sales order` -> document
- `order item` -> document line
- `order -> shipment request` -> document link

### 22.3 POS

- `sale receipt` -> document
- `sale line` -> document line
- `refund -> original sale` -> document link

### 22.4 ERP

- `purchase request` -> document
- `request line` -> document line
- `invoice -> payment settlement` -> document link

---

## 23. Governance Rules

- every document type must publish a stable contract
- every governed document must define lifecycle semantics
- required indexed fields must be explicit and justified
- document changes must remain compatible with workflow, audit, and event rules
- domains must not bypass the document kernel with ad hoc tables for governed transactional records unless there is a strong exception case

---

## 24. Recommended Initial Implementation Sequence

1. define base document header and body tables
2. define document type contract registration
3. implement create/load/update with version and etag handling
4. implement document line support
5. implement document links and attachments
6. integrate workflow-driven actions
7. integrate audit and outbox events
8. add numbering policy binding
9. add version history and projection hooks

---

## 25. Final Summary

The document kernel provides the central governed record model for the platform.

Its core responsibilities are:

- document identity
- indexed headers and extensible bodies
- line items
- lifecycle and versioning
- numbering
- links and attachments
- auditability and event emission

This makes it possible for different domain packs to implement their own business documents on a shared, consistent, and domain-agnostic foundation.
