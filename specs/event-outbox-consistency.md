# Event, Outbox, and Consistency Specification

## 1. Purpose

This document defines the domain-agnostic event model, transactional outbox pattern, and consistency rules for the platform kernel.

Its purpose is to ensure that:

- authoritative writes remain safe and auditable
- asynchronous processing is reliable and observable
- cross-module communication is explicit
- retries do not create duplicate business effects
- projections and integrations are updated in a controlled way

This specification applies to kernel modules, domain packs, worker processes, projections, and external adapters.

---

## 2. Goals

- define a standard event model for the platform
- define when work is synchronous versus asynchronous
- standardize transactional outbox publishing
- define idempotency and retry behavior
- define projection refresh and integration trigger semantics
- provide a clear failure-handling model

---

## 3. Non-Goals

This document does not define:

- message broker product selection
- cross-region distributed consistency
- event sourcing as the primary persistence model
- domain-specific event taxonomies beyond examples
- real-time websocket delivery contracts

---

## 4. Design Principles

1. **Authoritative writes are transactional**  
   A business write must commit authoritative state and its durable event intent together.

2. **Events do not replace authoritative storage**  
   Canonical state remains in authoritative tables. Events describe meaningful change.

3. **Async processing must be retry-safe**  
   Workers, connectors, and projection updaters must tolerate duplicates and restarts.

4. **External side effects happen after commit**  
   No external connector call should occur inside the main write transaction.

5. **Read models may be eventually consistent**  
   Search projections, dashboards, exports, and external submissions may lag behind authoritative writes.

6. **Failures must be visible**  
   Every failed async action must be traceable through logs, job records, and operational status.

7. **Event contracts must be stable**  
   Event names and envelope fields must be versioned and backward compatible.

---

## 5. Consistency Model

The platform uses a mixed consistency model.

### 5.1 Strongly Consistent Operations

The following must complete in one authoritative server transaction where applicable:

- canonical record create/update accepted by server
- document version increment
- workflow transition state change
- audit event persistence
- outbox event persistence
- idempotency record update for the triggering request

These actions either commit together or fail together.

### 5.2 Eventually Consistent Operations

The following may occur asynchronously after the authoritative transaction commits:

- search projection refresh
- dashboard counters and summaries
- print snapshot generation
- external adapter submission
- notification delivery
- long-running policy evaluation or enrichment
- reconciliation and cleanup jobs

### 5.3 Rule of Separation

If a side effect changes the truth of the business transaction, it must be inside the authoritative transaction.

If a side effect derives, distributes, transmits, or renders information based on committed truth, it should be asynchronous.

---

## 6. Canonical Event Model

An event is a durable record that a meaningful state change or system occurrence has taken place.

### 6.1 Event Categories

- `domain events`
  - emitted when canonical business state changes
- `workflow events`
  - emitted when states or approvals change
- `system events`
  - emitted for infrastructure or platform activity
- `integration events`
  - emitted when external exchange status changes

### 6.2 Event Naming

Event names should follow a stable dotted format:

- `document.created`
- `document.submitted`
- `document.finalized`
- `workflow.transitioned`
- `task.assigned`
- `task.completed`
- `projection.refresh_requested`
- `integration.submission_failed`

Domain packs may define their own namespaced events, for example:

- `clinic.encounter.finalized`
- `oms.order.released`
- `pos.sale.completed`

### 6.3 Event Envelope

Every event should contain at least:

- `event_id`
- `event_type`
- `event_version`
- `category`
- `aggregate_type`
- `aggregate_id`
- `aggregate_version` (nullable where not applicable)
- `occurred_at`
- `published_at` (nullable until dispatched)
- `actor_id` (nullable)
- `actor_kind`
- `correlation_id` (nullable)
- `causation_id` (nullable)
- `source`
- `payload`
- `headers` or `metadata`

### 6.4 Event Payload Rules

- payloads should contain enough context for subscribers to act safely
- payloads must avoid embedding unnecessary sensitive data
- payloads should prefer identifiers and summaries over full authoritative documents
- payload schema changes must be versioned

---

## 7. Transactional Outbox Pattern

The platform uses a database-backed transactional outbox as the default event publication mechanism.

### 7.1 Purpose

The outbox ensures that authoritative writes and event publication intent are committed atomically.

### 7.2 Write Path

For a state-changing request:

1. validate request, permissions, policies, and workflow rules
2. update authoritative state
3. write audit records
4. write one or more outbox events in the same database transaction
5. commit transaction
6. a dispatcher or worker publishes/processes the outbox entries after commit

### 7.3 Outbox Record Fields

Each outbox record should include:

- `outbox_id`
- `event_id`
- `event_type`
- `event_version`
- `aggregate_type`
- `aggregate_id`
- `aggregate_version` (nullable)
- `status` (`pending`, `dispatched`, `failed`, `dead_letter`)
- `available_at`
- `attempt_count`
- `last_attempt_at` (nullable)
- `last_error` (nullable)
- `correlation_id` (nullable)
- `payload`
- `headers`
- `created_at`
- `dispatched_at` (nullable)

### 7.4 Dispatch Model

The default dispatch model is polling worker-based dispatch from the database outbox.

The dispatcher must:

- claim eligible pending records safely
- publish or hand off work idempotently
- update attempt metadata
- mark terminal failure when retry policy is exhausted
- emit operational metrics

---

## 8. Idempotency Model

Idempotency is mandatory for all operations that may be retried by client, worker, or connector.

### 8.1 Request Idempotency

State-changing APIs should support idempotency keys for operations such as:

- submit
- approve
- reject
- cancel
- post payment
- dispatch order

An idempotency record should track:

- `idempotency_key`
- `operation_type`
- `actor_id`
- `request_hash`
- `result_ref`
- `status`
- `created_at`
- `expires_at` (nullable)

Rules:

- the same key with the same effective request must return the original result
- the same key with a materially different request must be rejected
- idempotency records must be written transactionally with business effect where possible

### 8.2 Event Consumer Idempotency

Event consumers must protect against duplicate delivery.

Recommended mechanisms:

- processed event log keyed by `event_id`
- aggregate-version checks where appropriate
- unique business effect keys for external submissions

### 8.3 Job Idempotency

Jobs must define an idempotency strategy before implementation.

Examples:

- projection refresh can overwrite by source version
- print snapshot generation can deduplicate by document id + template version + status
- external submission can deduplicate by integration operation key

---

## 9. Ordering Rules

The platform does not require global event ordering.

Ordering guarantees should be scoped as follows:

- per aggregate, event order should follow committed aggregate version where versioning exists
- subscribers must not assume total ordering across different aggregates
- workers must tolerate reordering between unrelated records

If a consumer requires strict ordering for a specific aggregate, it must enforce it using aggregate version or sequence checks.

---

## 10. Projection Consistency Rules

Projections are read-optimized derived views and are not authoritative state.

### 10.1 Projection Sources

Projections may be built from:

- canonical tables directly
- outbox or event streams
- approved derived transformation jobs

### 10.2 Projection Update Triggers

Projection updates may be triggered by:

- domain event dispatch
- scheduled refresh
- reconciliation job
- manual admin repair action

### 10.3 Projection Safety Rules

- projections must be rebuildable from authoritative data where feasible
- projection refresh must be idempotent
- stale projections must be detectable
- user interfaces must not treat projections as final truth for protected actions

### 10.4 Projection Metadata

Every maintained projection should track:

- source object id
- source version or watermark
- projection version
- refreshed_at
- refresh_status
- last_error (nullable)

---

## 11. External Integration Consistency Rules

External integrations always operate after authoritative commit.

### 11.1 Integration Flow

1. authoritative business change commits
2. outbox event is persisted
3. integration worker or adapter consumes event
4. outbound payload is built or fetched
5. external call is made
6. submission result is recorded
7. failures go through retry or dead-letter handling

### 11.2 Integration Safety Rules

- external side effects must never occur before authoritative commit
- adapter retries must be idempotent or deduplicated
- external status must not silently rewrite authoritative business state
- reconciliation must be explicit and auditable

---

## 12. Failure Handling Model

Failures are expected and must be classified clearly.

### 12.1 Failure Types

- `transaction failure`
  - authoritative write did not commit
- `dispatch failure`
  - event not yet processed after commit
- `consumer failure`
  - subscriber failed while handling a published event
- `projection failure`
  - read model refresh failed
- `integration failure`
  - external adapter call or reconciliation failed
- `poison message or terminal failure`
  - repeated retries are unlikely to succeed without intervention

### 12.2 Retry Rules

- retries should use capped backoff
- retry policy must be explicit per job or consumer type
- non-retryable failures should move directly to terminal state
- retry history must remain observable

### 12.3 Dead-Letter Rules

Dead-letter handling should exist for:

- repeated dispatch failure
- repeated projection failure
- repeated integration failure
- malformed or incompatible payloads

Dead-letter records should include:

- failure category
- payload snapshot or reference
- last error
- retry count
- first failure time
- last failure time
- recovery hints where available

---

## 13. Operational Observability

The event and outbox system must expose operational visibility.

Required metrics and views:

- pending outbox count
- dispatch rate
- dispatch failure rate
- job retry counts
- dead-letter counts
- projection lag
- integration lag and failure counts
- oldest pending item age

Required logs and tracing:

- correlation id across request, event, job, and connector
- causation chain where practical
- event dispatch attempts
- worker claim and completion logs

---

## 14. Recommended Storage Model

Recommended table families:

- `outbox_events`
- `idempotency_records`
- `job_records`
- `job_attempts`
- `dead_letter_records`
- `projection_status`
- `integration_submission_records`
- `consumer_offsets` or `processed_events`

Suggested indexing priorities:

- outbox by `status`, `available_at`
- jobs by `status`, `run_after`
- idempotency by `operation_type`, `idempotency_key`
- projection status by `projection_type`, `refresh_status`
- submission records by `external_system_key`, `status`

---

## 15. API and Worker Guidance

### 15.1 API Guidance

- client-facing writes should return authoritative result status after commit
- async side effects should be represented as accepted follow-up work, not implied as already complete
- APIs should return stable identifiers that allow later polling or lookup

### 15.2 Worker Guidance

- workers must be restart-safe
- workers must not assume exclusive long-term ownership without leases or claim semantics
- workers must update status often enough for observability
- long-running work should be broken into bounded steps where possible

---

## 16. Example End-to-End Flows

### 16.1 Document Submit

1. client submits document action with idempotency key
2. server validates request
3. server updates document state and version
4. server writes audit event and outbox event in same transaction
5. transaction commits
6. response returns authoritative document state
7. projection worker refreshes search view asynchronously
8. notification or external adapter work happens asynchronously if configured

### 16.2 Payment Posting

1. user posts payment
2. payment state commits transactionally with audit and outbox
3. receipt projection and official print snapshot are queued
4. accounting/export adapter work is triggered asynchronously

### 16.3 External Connector Failure

1. integration worker picks pending submission event
2. adapter call fails due to remote timeout
3. attempt metadata is updated
4. retry is scheduled with backoff
5. after policy exhaustion, item moves to dead-letter state
6. admin dashboard shows unresolved failure

---

## 17. Governance Rules

- every new state-changing module must define emitted events
- every new async job type must define retry and idempotency strategy
- every new projection must define rebuild and staleness detection rules
- every external adapter must define submission deduplication rules
- event contract changes must be versioned and reviewed for compatibility

---

## 18. Final Summary

The platform uses an authoritative transactional write model combined with asynchronous event-driven processing.

The core rules are:

- authoritative business state, audit, and outbox intent commit together
- external side effects happen only after commit
- projections and integrations are eventually consistent
- retries must be idempotent
- failures must be observable and recoverable

This model allows the platform to stay operationally safe while supporting extensibility, background processing, low-bandwidth clients, and external integrations.
