# Integration Kernel Specification

## 1. Purpose

This document defines the domain-agnostic integration kernel for the platform.

Its purpose is to provide a reusable integration architecture for connecting domain packs to external systems without leaking provider-specific concerns into the platform kernel or business modules.

This specification applies to external identifiers, mapping, projection, connector execution, submission tracking, reconciliation, and failure handling.

---

## 2. Goals

- define a reusable integration architecture for multiple domains
- separate business state from external interoperability concerns
- standardize outbound and inbound integration flow
- define external identifier ownership and mapping rules
- define submission tracking and reconciliation patterns
- ensure integrations are retry-safe, observable, and auditable

---

## 3. Non-Goals

This document does not define:

- provider-specific API payloads
- transport product choices beyond generic connector assumptions
- domain-specific mapping rules in full detail
- webhook security implementation details for each provider
- batch ETL or analytical data warehouse design

---

## 4. Design Principles

1. **Core domain state remains authoritative**  
   External systems do not directly define platform truth unless an explicit inbound contract says so.

2. **Integration is layered**  
   Mapping, projection, transport, and reconciliation must be separated.

3. **Adapters own provider logic**  
   Provider-specific payloads, authentication, and error semantics stay in adapters.

4. **External side effects happen after commit**  
   Integration work begins only after authoritative business changes commit.

5. **Identifiers are first-class**  
   External references and correlation keys must be explicitly modeled.

6. **All submissions are traceable**  
   Integration attempts, responses, and retries must be auditable and observable.

7. **Reconciliation is explicit**  
   External mismatch handling must be modeled as an explicit operational process.

---

## 5. Integration Layer Model

The integration architecture is divided into four layers.

### 5.1 Projection Layer

Transforms canonical internal records into integration-ready internal representations.

Responsibilities:

- choose source records
- normalize source data for exchange
- assemble outbound projection models
- prepare inbound normalization targets

The projection layer remains platform-facing and provider-neutral where possible.

### 5.2 Mapping Layer

Transforms between internal projection models and external data semantics.

Responsibilities:

- terminology mapping
- field transformation
- code set translation
- identifier resolution
- profile or schema version alignment

### 5.3 Connector Layer

Handles the actual communication with external systems.

Responsibilities:

- authentication
- request/response transport
- rate limiting
- retries
- remote error interpretation
- timeout handling
- payload submission and fetch

### 5.4 Reconciliation Layer

Compares and resolves differences between internal and external states.

Responsibilities:

- detect failed or partial exchanges
- verify external identifiers and statuses
- handle duplicate or missing submissions
- create operational tasks or retry jobs
- support manual intervention when needed

---

## 6. Canonical Integration Concepts

### 6.1 `ExternalSystem`

Represents a named external integration target.

Typical attributes:

- `external_system_key`
- `name`
- `system_type`
- `status`
- `adapter_key`
- `configuration_ref`

### 6.2 `ExternalReference`

Represents the mapping between a canonical internal record and an external identifier.

Typical attributes:

- `external_reference_id`
- `owner_type`
- `owner_id`
- `external_system_key`
- `external_id`
- `reference_type`
- `status`
- `verified_at` (nullable)
- `metadata`

### 6.3 `IntegrationProjection`

Represents a durable internal projection prepared for external exchange.

Typical attributes:

- `integration_projection_id`
- `projection_type`
- `source_type`
- `source_id`
- `projection_version`
- `mapping_profile_key`
- `status`
- `payload_ref`
- `created_at`
- `updated_at`

### 6.4 `SubmissionRecord`

Represents one outbound or inbound exchange attempt.

Typical attributes:

- `submission_record_id`
- `external_system_key`
- `direction` (`outbound`, `inbound`)
- `operation_type`
- `owner_type`
- `owner_id`
- `projection_id` (nullable)
- `status`
- `request_ref`
- `response_ref` (nullable)
- `external_request_id` (nullable)
- `retry_count`
- `submitted_at` (nullable)
- `completed_at` (nullable)
- `last_error` (nullable)

### 6.5 `MappingProfile`

Represents a versioned mapping contract used by a projection or adapter.

Typical attributes:

- `mapping_profile_key`
- `version`
- `external_system_key`
- `source_type`
- `status`
- `definition_ref`

### 6.6 `ReconciliationCase`

Represents an unresolved mismatch or exception in integration state.

Typical attributes:

- `reconciliation_case_id`
- `external_system_key`
- `owner_type`
- `owner_id`
- `case_type`
- `status`
- `severity`
- `opened_at`
- `resolved_at` (nullable)
- `resolution_note` (nullable)

---

## 7. Integration Flow Types

### 7.1 Outbound Push

Used when an internal business change triggers data submission to an external system.

Flow:

1. authoritative state change commits
2. outbox event is persisted
3. integration worker selects eligible event
4. projection is built or reused
5. mapping profile is applied
6. connector sends request
7. submission result is recorded
8. external reference and status are updated where appropriate

### 7.2 Inbound Pull

Used when the platform fetches updates from an external system.

Flow:

1. scheduled or manual sync job starts
2. connector fetches external data
3. payload is normalized
4. mapping and validation are applied
5. inbound contract determines whether to create, update, ignore, or open reconciliation case
6. results are recorded in submission and audit logs

### 7.3 Inbound Push

Used when an external system pushes events or callbacks into the platform.

Flow:

1. inbound endpoint authenticates and validates source
2. payload is stored or referenced durably
3. normalization and mapping occur
4. idempotency and duplicate checks run
5. permitted business or tracking updates occur through application services
6. audit and submission records are updated

---

## 8. Integration Boundary Rules

### 8.1 What Belongs in the Kernel

- external identifier registry
- integration projection mechanics
- mapping profile contracts
- connector interfaces
- submission tracking model
- reconciliation case model
- retry and observability contracts

### 8.2 What Belongs in Domain Packs

- domain-specific projection definitions
- domain-specific field preparation rules
- business meaning of inbound changes
- business-side approval for externally sourced updates

### 8.3 What Belongs in Adapters

- provider authentication
- remote payload schemas
- transport logic
- provider-specific error handling
- rate limits and protocol quirks
- remote pagination or webhook semantics

---

## 9. Identifier Management Rules

External identifiers must be explicitly modeled and never hidden inside arbitrary payload blobs only.

### 9.1 Identifier Rules

- a canonical object may have zero or more external references
- external references must be scoped by external system
- the same external id must not ambiguously map to multiple internal records unless explicitly allowed by reference type
- identifier verification state should be tracked where relevant

### 9.2 Correlation Rules

Every integration attempt should preserve:

- internal object id
- external system key
- operation type
- correlation id
- external request or transaction id where available

---

## 10. Mapping Rules

Mapping logic must be versioned and explicit.

### 10.1 Mapping Responsibilities

- transform internal codes to external codes
- transform internal shape to external payload shape
- transform external payload back into canonical inbound form
- validate required external fields
- record profile/version used for each exchange

### 10.2 Mapping Constraints

- mapping rules must not directly commit business state
- mapping failures must be visible and diagnosable
- mapping changes must be versioned to preserve traceability of historical submissions

---

## 11. Projection Rules

Integration projections are durable derived models prepared for external exchange.

### 11.1 Projection Purpose

- decouple canonical business records from external payload schemas
- allow snapshot traceability
- support replay and resubmission

### 11.2 Projection Rules

- a projection should record source object and source version
- projection generation should be idempotent
- projection content should be reproducible or durably stored
- projections may be regenerated when source changes, subject to policy
- historical submissions should retain reference to the exact projection or payload used

---

## 12. Connector Contract

The connector layer should be implemented behind stable adapter interfaces.

### 12.1 Connector Responsibilities

- obtain credentials or tokens
- send requests
- receive responses
- classify errors
- expose retry guidance
- provide request/response correlation metadata

### 12.2 Connector Result Model

Connector results should be normalized into at least:

- `accepted`
- `rejected_permanent`
- `failed_retryable`
- `rate_limited`
- `duplicate`
- `unauthorized`
- `invalid_payload`

### 12.3 Connector Rules

- connectors must be stateless where practical
- secrets must remain outside business/domain modules
- connectors must not directly mutate authoritative business tables
- connectors must produce structured logs for requests and responses

---

## 13. Submission Tracking Rules

Every exchange attempt should produce or update a `SubmissionRecord`.

### 13.1 Submission Statuses

Recommended statuses:

- `pending`
- `processing`
- `submitted`
- `acknowledged`
- `completed`
- `failed_retryable`
- `failed_terminal`
- `cancelled`
- `reconciled`

### 13.2 Submission Tracking Requirements

- preserve request snapshot or reference
- preserve response snapshot or reference when useful
- preserve retry count and last error
- preserve timestamps for each major step
- link submission to canonical object and projection used

---

## 14. Inbound Update Rules

Inbound updates require strong control because not all external changes should directly mutate business truth.

### 14.1 Inbound Modes

- `tracking_only`
  - update only integration tracking metadata
- `authoritative_import`
  - external data is accepted as source for a defined object type
- `proposed_change`
  - external data creates a review task or approval action before business state changes

### 14.2 Inbound Safety Rules

- inbound changes must use explicit application services
- inbound payloads must pass validation and idempotency checks
- sensitive inbound changes should be routed through policy or approval gates
- ambiguous matches should open reconciliation cases instead of applying silently

---

## 15. Retry and Idempotency Rules

Integrations must follow the platform retry and idempotency rules from `event-outbox-consistency.md`.

Additional integration-specific rules:

- outbound submissions must define a deduplication key
- repeated connector calls must not create duplicate external business effects where preventable
- retryable and terminal failures must be classified explicitly
- resubmission after mapping or data correction must preserve lineage to prior failed attempts

Examples of deduplication keys:

- projection id + operation type
- source object id + source version + external system key
- externally assigned request id

---

## 16. Reconciliation Model

Reconciliation handles mismatches between internal and external state.

### 16.1 Reconciliation Triggers

- repeated submission failures
- missing external acknowledgment
- external duplicate detection
- external status conflicting with internal tracking
- inbound payload not matching any internal record confidently
- manual operator review

### 16.2 Reconciliation Outcomes

- retry submission
- update tracking only
- create missing external reference
- link to existing internal object
- open review task
- close as expected exception
- mark as terminal failure

### 16.3 Reconciliation Rules

- reconciliation actions must be auditable
- manual resolution requires actor attribution
- final business changes caused by reconciliation must still go through approved application services

---

## 17. Audit and Observability Requirements

The integration kernel must provide end-to-end traceability.

Required audit and observability areas:

- projection generation
- connector attempts
- submission statuses
- retry scheduling
- reconciliation actions
- external identifier creation or correction
- inbound payload acceptance or rejection

Recommended metrics:

- pending submissions by external system
- success and failure rate by operation type
- retry counts and dead-letter counts
- oldest unresolved reconciliation case age
- projection generation latency
- connector latency and rate-limit incidents

---

## 18. Storage Guidance

Recommended table families:

- `external_systems`
- `external_references`
- `mapping_profiles`
- `integration_projections`
- `submission_records`
- `submission_attempts`
- `reconciliation_cases`
- `inbound_payload_logs`

Suggested indexing priorities:

- external references by `external_system_key`, `external_id`
- submission records by `external_system_key`, `status`, `operation_type`
- reconciliation cases by `status`, `severity`, `external_system_key`
- projections by `source_type`, `source_id`, `projection_type`

---

## 19. API Guidance

The integration kernel should expose admin and operational APIs such as:

- list submission records
- inspect submission attempt history
- list reconciliation cases
- trigger replay or resubmission
- verify external references
- inspect effective mapping profile version

Direct business write APIs should not expose adapter-specific payloads as their primary contract.

---

## 20. Example Domain Mappings

### 20.1 Clinic

- finalized encounter -> integration projection
- healthcare adapter -> external submission
- external clinical resource id -> external reference
- failed submission -> reconciliation case

### 20.2 OMS

- order shipment notice -> integration projection
- marketplace connector -> outbound adapter
- marketplace order id -> external reference
- fulfillment mismatch -> reconciliation case

### 20.3 POS

- posted sale -> accounting export projection
- payment terminal reference -> external reference
- failed receipt export -> retryable submission record

### 20.4 ERP

- approved vendor invoice -> tax/reporting submission
- supplier registry id -> external reference
- rejected filing -> reconciliation case

---

## 21. Governance Rules

- every adapter must declare its external system key and supported operation types
- every outbound integration path must define projection, mapping, connector, and reconciliation ownership
- every inbound integration path must define validation and idempotency strategy
- provider-specific schemas must not become core kernel contracts
- integration contract changes must be versioned and operationally visible

---

## 22. Final Summary

The integration kernel provides a reusable boundary between internal platform/domain logic and external systems.

Its core structure is:

- projection prepares internal exchange-ready models
- mapping translates semantics
- connectors handle transport and provider behavior
- submission tracking records every exchange
- reconciliation resolves mismatches safely

This architecture keeps the platform domain-agnostic while still supporting industry-specific adapters such as healthcare interoperability, tax filing, payment gateways, and marketplaces.
