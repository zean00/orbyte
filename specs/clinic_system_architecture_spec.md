# Clinic Information System Architecture Specification

## 1. Purpose

This document defines the target architecture for a clinic information system with the following principles:

- web-based application
- low-bandwidth tolerant, not fully offline
- cloud-first with local workbench behavior
- modular and plugin-based, inspired by Odoo principles
- backend in Go
- PostgreSQL as primary database
- SATUSEHAT integration only on server side
- outpatient workflow first
- lightweight enough to operate acceptably on low-cost hardware such as Intel N100 mini PCs

The architecture is intended for a first production version focused on outpatient services, while leaving clear extension points for lab, imaging, pharmacy, HR, assets, and accounting.

---

## 2. Product Goals

### 2.1 Business goals

- support daily outpatient clinic operations end to end
- reduce network dependency in low-bandwidth areas
- maintain reliable cloud-based source of truth
- prepare clean integration path to SATUSEHAT
- allow modular expansion over time without major redesign

### 2.2 Technical goals

- fast UI under 1–5 Mbps connectivity
- local caching of app shell and selected reference data
- local drafting and validation of working documents
- server-authoritative workflow for approval, payment, and integration
- explicit document lifecycle, audit trail, and access control
- modular backend and frontend extension model

### 2.3 Non-goals for v1

- full offline operation without internet for long periods
- inpatient workflow
- full pharmacy inventory/POS
- lab LIS depth
- PACS-grade imaging workflow
- full accounting ledger
- full HR/payroll suite

---

## 3. Architectural Principles

1. **Cloud source of truth**  
   All approved/finalized data lives on the server as authoritative state.

2. **Client as workbench**  
   Browser local storage is for drafting, caching, and bandwidth reduction, not as a second source of truth.

3. **Submit-based synchronization**  
   Working documents may be created and edited locally, then submitted explicitly to server.

4. **Server-authoritative critical actions**  
   Approval, payment posting, final numbering, audit truth, and SATUSEHAT integration are server-only.

5. **Document-oriented core**  
   Business workflows center around documents such as encounter, prescription, and invoice.

6. **Hybrid relational + JSONB model**  
   Indexed operational headers remain relational; extensible bodies use JSONB.

7. **Plugin-first extensibility**  
   Modules extend document schemas, workflows, permissions, UI, and integrations through controlled contracts.

8. **Separate integration boundary**  
   SATUSEHAT-specific logic must remain outside core clinic business logic.

9. **Bandwidth efficiency over raw realtime**  
   Prefer snapshots, bundles, delta refreshes, and submit flows over chatty APIs.

10. **Everything important is auditable**  
   Every meaningful state transition and critical action must be attributable to a user and timestamp.

---

## 4. High-Level System Context

### 4.1 Main components

- **Web frontend (PWA)**
  - app shell
  - form UI
  - local cache
  - local drafts
  - local validation
  - outbox / submit manager

- **Backend API (Go modular monolith)**
  - authentication and access control
  - document engine
  - workflow/approval engine
  - business modules
  - audit service
  - sync/reference-data service
  - print/template service
  - integration projection service

- **Primary database (PostgreSQL)**
  - operational data
  - document headers
  - JSONB bodies
  - audit trails
  - workflow states
  - sync metadata
  - integration records

- **Background jobs / workers**
  - reference bundle generation
  - async document processing
  - print snapshot generation
  - SATUSEHAT outbound queue
  - retry / reconciliation jobs

- **External systems**
  - SATUSEHAT API
  - future BPJS / payer integration
  - future messaging/notification providers

### 4.2 Deployment posture

Recommended first deployment:

- single backend service binary or a few tightly coupled services
- PostgreSQL primary in cloud
- object storage for attachments/doc renders if needed
- CDN optional for app assets
- reverse proxy/load balancer
- separate worker process using same codebase/modules

This should start as a **modular monolith**, not microservices.

---

## 5. Functional Scope

### 5.1 Outpatient MVP modules

#### Platform foundation
- authentication
- user management
- access control
- organization and clinic settings
- plugin registry
- audit trail
- document engine
- workflow/approval engine
- reference-data sync
- print/template engine

#### Operational modules
- patient information management
- practitioner / medical staff management
- employee / operational staff management
- schedule and queue management
- registration/check-in
- triage/vitals
- encounter / health record management
- prescription management
- billing
- payment

#### Integration foundation
- terminology mapping
- external identifier registry
- SATUSEHAT projection and submission tracking

### 5.2 Future modules
- lab services
- medical imaging / DICOM layer
- full pharmacy inventory and POS
- HR and payroll
- asset management
- accounting
- insurer/claim integration

---

## 6. User Roles and Access Model

### 6.1 Base role examples
- front desk / registration clerk
- nurse
- doctor
- pharmacy staff
- cashier
- clinic admin
- clinic manager
- auditor
- system administrator

### 6.2 Access control model

The system should use **RBAC + contextual constraints + document-state permissions**.

#### RBAC
Base permission sets by role, for example:
- create patient
- edit patient
- create encounter draft
- approve billing adjustment
- post payment
- print prescription
- manage reference data

#### Contextual constraints
Examples:
- doctor can finalize only their own encounter or those assigned to them
- nurse can draft vitals but cannot finalize diagnosis
- cashier can access billing/payment but not full clinical notes
- location-specific access by clinic branch
- shift or schedule-based access where needed

#### Document-state permissions
Permissions depend on current state, e.g.:
- who can edit local draft
- who can submit
- who can approve
- who can reopen
- who can cancel
- who can print
- who can export
- who can trigger resubmission to integration queue

### 6.3 Access control enforcement layers
- frontend hides or disables unavailable actions for UX
- backend remains the ultimate authority for every protected action
- workflow engine validates role + context + document state together

---

## 7. Document-Oriented Domain Model

### 7.1 Document categories

#### Master documents
Slow-changing records.
- patient profile
- practitioner
- employee
- tariff item
- service item
- location
- medicine reference subset
- disease reference subset

#### Transaction documents
Operational records.
- appointment
- registration
- queue ticket
- triage sheet
- encounter
- prescription
- invoice
- payment receipt

#### Workflow documents
Governance/process records.
- approval request
- correction request
- cancellation request
- reopen request
- sync job
- integration error case

#### Derived/integration documents
System-generated projections.
- FHIR resource projection
- SATUSEHAT submission record
- print snapshot
- export snapshot

### 7.2 Core business documents for outpatient

#### Patient Profile
Contains demographics, identifiers, payer hints, contact info, and status.

#### Registration
Represents patient arrival/check-in for a visit.

#### Triage Sheet
Contains intake data such as complaint, vitals, and screening.

#### Encounter
Represents the clinical session.
Includes SOAP-like notes, diagnoses, procedures, and plan.

#### Prescription
Represents ordered medications and later dispense-related status.

#### Invoice
Represents billable items tied to encounter/services.

#### Payment
Represents server-posted financial settlement.

---

## 8. Document Lifecycle

### 8.1 General lifecycle pattern

Client-side states:
- `local_draft`
- `ready_to_submit`
- `submit_failed`
- `submitted_pending_response`

Server-side states:
- `draft_server`
- `submitted`
- `under_review`
- `approved`
- `rejected`
- `finalized`
- `cancelled`
- `amended`

Not every document uses every state.

### 8.2 Example lifecycle by document type

#### Encounter
- local_draft
- submitted
- approved/finalized
- amended (optional)

#### Prescription
- local_draft
- submitted
- doctor_signed
- dispense_pending
- dispensed
- cancelled

#### Invoice
- local_draft
- submitted
- finalized
- paid
- voided

### 8.3 Lifecycle rules
- only drafts may be edited locally
- after submit, authoritative state is server-side
- every state transition must be explicit
- workflow engine owns transitions
- state transitions must be audited
- finalized documents are immutable except through amend/correction flow

---

## 9. Versioning, Concurrency, and Audit

### 9.1 Required versioning fields
Every document should have:
- `document_id`
- `document_type`
- `status`
- `version`
- `etag`
- `content_hash`
- `created_by`
- `created_at`
- `updated_by`
- `updated_at`
- `submitted_by`
- `submitted_at`
- `approved_by`
- `approved_at`

### 9.2 Semantics
- **version**: increments on each accepted server-side change
- **etag**: optimistic concurrency token for writes
- **content_hash**: integrity/comparison aid, optional for conflict/debugging

### 9.3 Audit trail
Two layers:

#### Header metadata
Fast summary fields on document header.

#### Full audit event log
Stores meaningful events only, such as:
- create draft
- edit draft
- submit
- approve
- reject
- reopen
- cancel
- amend
- print
- export
- external sync

Each event should include:
- `event_id`
- `document_id`
- `document_type`
- `action`
- `actor_id`
- `actor_role`
- `timestamp`
- `from_state`
- `to_state`
- `reason`
- `diff_summary`
- `source` (web/client/server/job)
- request metadata where appropriate

### 9.4 Correction policy
Finalized clinical/billing documents should not be directly overwritten.
Use:
- addendum
- amend
- cancel and recreate
- reopen with reason and authorization

---

## 10. Low-Bandwidth Local Workbench Architecture

### 10.1 Intent
The client should remain usable on poor internet by minimizing repeated downloads and API chatter.

### 10.2 Local storage responsibilities
Use browser storage such as IndexedDB for:
- app metadata
- current user/session context
- local drafts
- reference-data subsets
- validation rule snapshots
- recent operational cache
- outbox state
- print/template cache

### 10.3 Data classes stored locally

#### Reference cache
- disease classifications
- medicine catalog subset
- dosage/frequency/unit references
- practitioner list for current clinic
- service/tariff summary
- location/unit/poli list
- schedule windows
- validation rules
- schema/form metadata

#### Draft store
- registration draft
- triage draft
- encounter draft
- prescription draft
- invoice draft

#### Recent operational cache
- current-day queue
- current-day appointments
- recent patients opened by current user
- recent encounter summaries
- recent unpaid invoices

#### Outbox
- pending submissions
- retry counters
- client timestamps
- last error details

### 10.4 What should not be kept as authoritative local data
- full historical EMR
- final approval records
- payment truth
- SATUSEHAT sync truth
- full audit history
- large attachments as primary truth
- complete pharmacy/inventory ledgers

---

## 11. Reference Data Packaging and Sync

### 11.1 Why packaged sync is needed
Reference data such as disease and medicine lists should not be fetched ad hoc on every form use.
Use versioned bundles to reduce network requests.

### 11.2 Bundle examples
- disease-classification bundle
- medicine-reference bundle
- dosage/form/frequency bundle
- practitioner summary bundle
- clinic location bundle
- tariff summary bundle
- validation rules bundle
- form/schema metadata bundle

### 11.3 Bundle metadata
Each bundle should have:
- bundle name
- bundle version
- generated_at
- checksum/hash
- optional delta base version
- expiry / TTL

### 11.4 Sync strategy
- client sends versions it currently has
- server returns only outdated bundles or deltas
- client updates local cache atomically per bundle
- stale bundles are tolerated for read usage but revalidated on submit

### 11.5 Refresh triggers
- login
- app start
- explicit refresh
- before opening certain forms if bundle expired
- after server reject due to outdated reference

---

## 12. Local Validation and Server Revalidation

### 12.1 Local validation purpose
Provide immediate feedback and reduce invalid submissions.

### 12.2 Validation types
- required field rules
- data type rules
- schema rules
- cross-field rules
- document-state rules
- role-based action rules
- simple business rules

Examples:
- prescription requires patient and prescriber
- encounter cannot close without diagnosis if policy requires it
- invoice cannot finalize without at least one line item

### 12.3 Authoritative validation
Every submission must be revalidated on server, regardless of local success.

### 12.4 Validation rule distribution
Validation rules can be distributed to clients as versioned metadata for local execution.
Server maintains canonical rule set.

---

## 13. Submit, Outbox, and Conflict Handling

### 13.1 Submit model
1. user edits draft locally
2. local validation runs
3. user submits
4. payload enters local outbox
5. client sends to server
6. server validates and commits or rejects
7. client updates local state based on response

### 13.2 Outbox entry fields
- local_document_id
- document_type
- intended action
- payload snapshot
- client version/etag if available
- created_at
- retry_count
- last_attempt_at
- last_error
- status

### 13.3 Conflict scenarios
Examples:
- stale practitioner schedule
- outdated tariff
- deactivated medicine
- patient data changed on server
- another user already modified same server draft

### 13.4 Conflict policy
- detect via etag/version mismatch or business rule rejection
- preserve local draft
- show actionable error to user
- allow refresh and revalidate
- allow manual correction and retry

### 13.5 Server should never silently coerce critical changes
Especially for:
- changed prices
- changed doctor assignment
- inactive medicines
- approval state mismatches

---

## 14. Frontend Architecture

### 14.1 Recommended shape
A PWA web application with:
- cached app shell
- route-based code splitting
- IndexedDB local store
- service worker for assets and app shell
- form engine driven by schema/plugin metadata where appropriate
- local validation runtime
- outbox/sync manager

### 14.2 Frontend concerns
- authentication UX
- module navigation
- local draft autosave
- local form validation
- permission-aware action rendering
- queue/dashboard pages
- print rendering
- sync/error indicators
- degraded network behavior

### 14.3 Performance guidance
To run well on Intel N100-class devices:
- avoid overly heavy frontend frameworks or large component libraries
- lazy load modules
- minimize bundle size
- prefer server-driven summaries for lists/search results
- avoid loading large historical data into client memory

---

## 15. Backend Architecture

### 15.1 Recommended style
**Modular monolith in Go**.

Reasons:
- simpler deployment and ops
- strong internal boundaries still possible
- easier transaction consistency
- easier plugin lifecycle control in early stages

### 15.2 Core backend subsystems
- API gateway/router
- auth/session/token handling
- access control engine
- plugin/module registry
- document engine
- workflow/approval engine
- validation engine
- audit engine
- reference-data service
- search/index service
- print/template service
- business modules
- integration projection layer
- external connector layer
- job worker framework

### 15.3 Internal module boundaries

#### Core platform modules
- `core`
- `identity_access`
- `organization`
- `document_engine`
- `workflow_engine`
- `audit`
- `reference_sync`
- `search`
- `printing`

#### Business modules
- `patient`
- `practitioner`
- `employee`
- `schedule_queue`
- `registration`
- `triage`
- `encounter`
- `prescription`
- `billing`
- `payment`

#### Integration modules
- `terminology_mapping`
- `external_identifier_registry`
- `satusehat_projection`
- `satusehat_connector`
- `integration_reconciliation`

### 15.4 Internal communication
Use in-process interfaces/events first.
Avoid remote service calls between modules unless later truly needed.

---

## 16. Database Architecture

### 16.1 Primary database
PostgreSQL is the authoritative operational database.

### 16.2 Modeling approach
Use a hybrid model:
- relational tables for searchable/indexed operational structures
- JSONB for extensible document bodies and plugin extensions

### 16.3 Recommended major table families

#### Identity and access
- users
- roles
- permissions
- role_bindings
- user_location_scope
- sessions

#### Organization/master
- organizations
- clinics
- locations
- practitioners
- employees
- patients
- service_items
- tariff_items
- reference_bundle_versions

#### Document core
- document_headers
- document_bodies
- document_versions
- document_links
- document_tags

#### Workflow and approval
- workflow_definitions
- workflow_instances
- approval_steps
- approval_actions

#### Audit
- audit_events
- print_events
- export_events

#### Operations
- appointments
- queue_items
- invoices
- payments

#### Sync/integration
- outbox_jobs
- inbound_sync_logs
- integration_projections
- satusehat_submission_records
- dead_letter_jobs

### 16.4 Document storage pattern

#### document_headers
Stores indexed fields such as:
- document_id
- document_type
- patient_id (nullable depending on type)
- clinic_id
- status
- version
- etag
- created_by
- created_at
- updated_by
- updated_at
- submitted_by
- submitted_at
- approved_by
- approved_at

#### document_bodies
Stores JSONB body:
- document_id
- schema_version
- payload_jsonb
- content_hash

#### document_versions
Historical snapshots or diffs depending on storage choice.

### 16.5 Indexing strategy
Index headers and summary/search fields, not arbitrary full payload by default.
Examples:
- patient identifiers
- patient name normalized columns
- document type/status/date
- encounter date
- prescriber id
- invoice number
- payment status
- queue date/location

---

## 17. Plugin Architecture Specification

### 17.1 Goal
Allow modules to extend system behavior without uncontrolled coupling.

### 17.2 Plugin principles
- explicit dependencies
- explicit extension points
- versioned contracts
- no bypass of core workflow or permission enforcement
- migrations required for schema changes
- plugin install/upgrade must be deterministic

### 17.3 Plugin manifest
Each plugin should define:
- plugin name
- version
- description
- dependencies
- optional dependencies
- backend migrations
- document types contributed
- routes/endpoints contributed
- permission definitions
- workflow definitions
- reference bundles contributed
- frontend routes/views/components contributed
- feature flags

### 17.4 Plugin extension surfaces

#### Data model extension
- add document type
- add schema fragment to existing document type
- add indexed header fields if approved via migration

#### UI extension
- add menu items
- add pages/routes
- extend forms with sections/widgets
- add actions/buttons conditioned by permissions/workflow

#### Workflow extension
- define or extend state machine
- define approval path
- define state transition rules

#### Permission extension
- contribute permissions
- bind default permissions to roles

#### Integration extension
- subscribe to domain events
- produce projections
- enqueue outbound jobs

### 17.5 Suggested initial plugins
- `core`
- `identity_access`
- `patient`
- `practitioner_employee`
- `schedule_queue`
- `registration`
- `triage`
- `encounter`
- `prescription`
- `billing_payment`
- `approval_workflow`
- `satusehat_connector`
- `reporting_print`

---

## 18. Workflow and Approval Engine

### 18.1 Responsibility
Centralize all state transition and approval logic.

### 18.2 Workflow definition model
For each document type, define:
- states
- allowed transitions
- triggering actions
- role requirements
- contextual requirements
- whether transition is auto or manual
- whether transition requires signature/approval
- side effects

### 18.3 Example approval use cases

#### Patient registration
- draft by front desk
- submit
- auto-approved or clerk-approved based on policy

#### Encounter
- nurse drafts intake
- doctor finalizes encounter

#### Prescription
- doctor signs
- pharmacy acknowledges and dispenses

#### Billing adjustment/refund
- cashier proposes
- supervisor approves

### 18.4 Hard rules
- workflow engine is server-authoritative
- plugins cannot bypass transition validation
- all workflow actions are audited
- approval actions must record actor and timestamp

---

## 19. Search and Retrieval Architecture

### 19.1 Why separate search strategy is needed
Low-bandwidth systems should not fetch full documents to populate list/search screens.

### 19.2 Search design
Use indexed summaries for:
- patient search
- encounter search
- prescription search
- invoice search
- queue search
- practitioner search

### 19.3 Search payload style
Return lightweight result rows with:
- primary identifiers
- display summary
- key status fields
- timestamps
- only minimal preview fields

### 19.4 Local cache role in search
Client may cache recent results, but authoritative search remains server-side.

---

## 20. Printing and Document Output

### 20.1 Print requirements
Support low-dependency local printing for:
- queue tickets
- patient label/card
- prescription
- invoice
- receipt
- visit summary
- referral/certificate later

### 20.2 Architecture
- templates managed server-side and versioned
- printable templates cached locally for resilience
- rendering may happen client-side for immediate printing or server-side for official finalized snapshots depending on document type

### 20.3 Official vs draft printing
- draft prints should be clearly marked where needed
- official legal/financial prints should derive from server-authoritative finalized data

---

## 21. Attachment Strategy

### 21.1 Expected attachment types
- scanned IDs
- consent forms
- referral letters
- photo attachments
- PDFs

### 21.2 Rules
- attachments should be separate from core document payload
- uploads should be async where possible
- previews may be cached locally
- official attachment truth remains server-side/object storage

### 21.3 Low-bandwidth handling
- compress where allowed
- deferred upload supported for non-blocking cases
- document may carry attachment placeholder status if policy allows

---

## 22. Numbering and Legal Record Policy

### 22.1 Server-generated numbering
Official numbers should be generated only on server for:
- registration
- encounter
- prescription
- invoice
- receipt
- future referral/certificate documents

### 22.2 Numbering properties
- unique per policy scope
- auditable
- branch/clinic aware if needed
- immutable once assigned except special administrative correction process

---

## 23. SATUSEHAT Integration Architecture

### 23.1 Boundary rule
SATUSEHAT integration must happen only on server, never directly from client.

### 23.2 Layer separation

#### Core clinic domain layer
Owns clinic workflows and internal documents.

#### Projection/mapping layer
Transforms internal documents into interoperable representations.
Handles:
- terminology mapping
- identifier mapping
- resource assembly
- validation profile application

#### External connector layer
Handles:
- authentication/token handling
- API calls
- retries
- rate limiting
- response logging
- failure handling
- reconciliation

### 23.3 Async integration pattern
Recommended flow:
1. encounter/prescription/etc finalized on server
2. domain event emitted
3. projection job builds outbound payload(s)
4. outbound record queued
5. connector submits to SATUSEHAT
6. result stored in submission records
7. failures go to retry or dead-letter workflow

### 23.4 Data that should be stored for integration traceability
- internal document id
- projection type/version
- outbound payload snapshot or reference
- external identifier
- submission status
- submitted_at
- response summary
- retry count
- last_error

### 23.5 Change tolerance
Because SATUSEHAT may evolve, isolate change impact to:
- projection schemas
- mapping rules
- connector implementation
- integration validation profiles

---

## 24. Background Jobs and Async Processing

### 24.1 Why async processing is needed
To keep UI responsive and isolate unreliable external dependencies.

### 24.2 Job categories
- reference-data bundle generation
- sync cleanup
- print snapshot generation
- integration projection build
- SATUSEHAT submission
- retry jobs
- reconciliation jobs
- notification jobs later

### 24.3 Job reliability requirements
- persistent job records in DB
- idempotent processing where possible
- retry policy with backoff
- dead-letter handling
- job observability dashboard/logs

---

## 25. Security and Privacy Requirements

### 25.1 Minimum security controls
- HTTPS everywhere
- secure session/token management
- password hashing / identity best practices
- RBAC and contextual authorization
- audit logging
- encryption at rest where applicable
- attachment access control
- least-privilege service access

### 25.2 Clinical privacy controls
- restricted access to sensitive notes where needed
- branch/location scoping
- break-glass access model if required in later phase
- print/export tracking

### 25.3 Operational security
- backup and restore procedures
- migration controls
- secrets management
- admin activity auditing

---

## 26. Observability and Operations

### 26.1 Required observability areas
- API health and latency
- submission failure rates
- outbox/retry queue health
- workflow transition failures
- SATUSEHAT integration status
- DB health and slow queries

### 26.2 Admin dashboards
Recommended dashboards:
- sync/reference bundle status
- document submission failures
- approval queue
- payment posting errors
- integration retry/dead-letter dashboard

### 26.3 Logging
Use structured logs with request IDs and job IDs.

---

## 27. Performance Targets and Constraints

### 27.1 Low-end device target
The application should remain usable on Intel N100-class devices.

### 27.2 Design implications
- keep frontend bundles small
- keep forms responsive with local validation
- avoid large list payloads
- paginate/search summaries server-side
- cache assets and reference bundles aggressively
- avoid expensive client-side data transforms on large datasets

### 27.3 Bandwidth optimization tactics
- compressed responses
- versioned bundle sync
- delta refresh where useful
- submit-based writes instead of frequent autosync
- local assets and templates cached

---

## 28. Suggested API Boundary Style

### 28.1 API categories
- authentication/session APIs
- reference bundle APIs
- search/list summary APIs
- document draft submit APIs
- workflow action APIs
- payment APIs
- print/template APIs
- admin/configuration APIs

### 28.2 General API rules
- avoid chatty request patterns
- return summaries for list endpoints
- full documents fetched only when needed
- all critical writes validated server-side
- workflow transitions use explicit action endpoints

Examples:
- submit encounter draft
- approve billing adjustment
- post payment
- reopen document with reason
- regenerate reference bundle

---

## 29. Initial Module Roadmap

### Phase 1: Platform foundation
- auth and access control
- plugin registry
- document engine
- workflow engine
- audit engine
- PWA shell
- local draft store
- reference bundle sync

### Phase 2: Outpatient operations
- patient management
- practitioner/employee basics
- queue/schedule
- registration
- triage
- encounter
- prescription
- billing
- payment

### Phase 3: Integration and compliance
- terminology mapping
- external identifier registry
- SATUSEHAT projection layer
- connector/retry/reconciliation
- official print snapshots

### Phase 4: Future expansion
- lab
- imaging
- pharmacy full
- HR/payroll
- assets
- accounting

---

## 30. Hard Architectural Rules

1. Only drafts may be created/edited locally.
2. Final approval, payment, and external integration states are server-only truths.
3. Every document must have header, body, version, and audit metadata.
4. Every server state transition must go through workflow engine.
5. Plugins may extend schemas and UI, but may not bypass permission/workflow enforcement.
6. SATUSEHAT integration must stay outside core clinic domain modules.
7. Reference/master data must be distributed as versioned bundles or efficient summaries.
8. Every submit must be revalidated on server.
9. Final numbering is generated only on server.
10. Every meaningful action must be auditable.

---

## 31. Recommended Next Technical Deliverables

After this architecture document, the next specs to produce should be:

1. **Domain model spec**
   - detailed document schemas
   - indexed header fields
   - JSONB payload boundaries

2. **Plugin contract spec**
   - manifest format
   - backend/frontend extension points
   - migration rules

3. **Workflow spec**
   - state machines per document type
   - approval paths
   - action permissions

4. **Reference sync spec**
   - bundle format
   - versioning rules
   - client refresh behavior

5. **API spec**
   - endpoint groups
   - request/response models
   - concurrency/error semantics

6. **SATUSEHAT integration spec**
   - internal projection model
   - identifier/mapping design
   - submission tracking/retry model

7. **Frontend local storage spec**
   - IndexedDB stores
   - cache eviction
   - outbox design

---

## 32. Final Summary

The target system should be implemented as a **cloud-first, low-bandwidth tolerant, modular clinic platform**.

Its defining characteristics are:
- PWA web frontend with local workbench behavior
- Go modular backend
- PostgreSQL as source of truth
- document-oriented workflow model
- server-authoritative approval/payment/integration
- explicit versioning and audit trail
- Odoo-inspired plugin architecture with stricter contracts
- separate SATUSEHAT adapter layer
- designed first for outpatient flow, but extensible for future clinical and administrative modules

This architecture provides a practical balance between:
- weak connectivity tolerance
- operational safety
- extensibility
- performance on cheap hardware
- long-term interoperability readiness

