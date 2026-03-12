# Search and Projection Specification

## 1. Purpose

This document defines the domain-agnostic search and projection architecture for the platform kernel.

Its purpose is to provide a reusable read-model strategy for list views, search, dashboards, queues, exports, and low-bandwidth retrieval without treating full canonical documents as the default query model.

This specification applies to search summaries, read projections, query APIs, projection refresh behavior, and operational rebuild/recovery rules.

---

## 2. Goals

- define a reusable read-model architecture across domains
- separate authoritative write models from query-optimized projections
- support low-bandwidth and low-latency retrieval patterns
- standardize projection ownership, refresh, and rebuild rules
- define search API boundaries and result contracts
- ensure projections are observable, recoverable, and safe to rely on

---

## 3. Non-Goals

This document does not define:

- full analytics or BI warehouse design
- full-text search product selection
- domain-specific ranking formulas
- frontend UI layouts for search screens
- arbitrary reporting DSL design

---

## 4. Design Principles

1. **Canonical write model and read model are distinct**  
   The authoritative source of truth remains in canonical records. Projections exist for retrieval efficiency.

2. **Search returns summaries first**  
   Most list and search queries should return compact summary rows, not full record payloads.

3. **Projections are derived and rebuildable**  
   Projections should be reproducible from authoritative data or deterministic event streams.

4. **Eventually consistent by default**  
   Search results may lag slightly behind authoritative writes unless a specific use case requires synchronous refresh.

5. **Projection contracts are explicit**  
   Each projection must define its source, shape, refresh triggers, and ownership.

6. **Low-bandwidth efficiency matters**  
   APIs should minimize payload size, redundant fetches, and unnecessary joins at request time.

7. **Protected actions never trust projection state alone**  
   Authorization and state-changing actions must re-check authoritative records.

---

## 5. Core Concepts

### 5.1 `Projection`

A `Projection` is a derived, read-optimized representation of one or more canonical records.

Projection purposes include:

- search results
- queue views
- worklists
- dashboard counters
- export staging
- printable summary context

### 5.2 `Projection Definition`

A projection definition is the contract that describes:

- projection key
- projection type
- source objects
- shape of output
- refresh triggers
- refresh strategy
- ownership and rebuild rules

### 5.3 `Search Index` or `Search Store`

The search store is the persistence layer used to serve search and list retrieval.

It may be implemented using:

- relational summary tables
- materialized views
- dedicated index stores
- hybrid approaches

The product choice is secondary to the projection contract and refresh guarantees.

---

## 6. Query Model Types

The platform should support multiple read-model types.

### 6.1 Search Summary Projections

Used for:

- list pages
- search results
- quick lookups

Characteristics:

- compact rows
- indexed fields
- minimal preview content

### 6.2 Queue and Worklist Projections

Used for:

- pending approvals
- fulfillment queues
- operational inboxes
- task assignments

Characteristics:

- state and priority focused
- optimized for filtering and sorting

### 6.3 Dashboard Projections

Used for:

- counts
- recent activity summaries
- operational status widgets

Characteristics:

- aggregated values
- not suitable as authoritative detailed records

### 6.4 Export Projections

Used for:

- CSV or report extracts
- integration snapshots
- offline review packages

Characteristics:

- stable shape
- often batch-oriented

---

## 7. Projection Definition Model

Each projection definition should include:

- `projection_key`
- `projection_type`
- `owner_module`
- `source_types`
- `shape_version`
- `primary_identity_fields`
- `indexed_fields`
- `filter_fields`
- `sort_fields`
- `display_fields`
- `refresh_strategy`
- `refresh_triggers`
- `staleness_policy`
- `rebuild_strategy`
- `retention_policy` (nullable)
- `status`

Rules:

- projection keys must be stable
- shape changes must be versioned
- ownership must be explicit
- every projection must define how staleness is detected

---

## 8. Projection Sources

Projections may derive from one or more of the following sources:

- canonical tables
- document headers and lines
- workflow/task state
- events and outbox-driven refresh pipelines
- integration status records
- reference or configuration lookups

Rules:

- authoritative source must always be identifiable
- projection derivation logic must be deterministic
- the same projection should not silently depend on hidden data sources

---

## 9. Refresh Strategies

The platform should support multiple refresh strategies.

### 9.1 Synchronous Refresh

Use only when immediate read-after-write visibility is required and cost is acceptable.

Typical use cases:

- small local counters tightly coupled to transaction
- very small critical summary tables

### 9.2 Event-Driven Asynchronous Refresh

Preferred default for most projections.

Flow:

1. authoritative write commits
2. outbox event is emitted
3. projection worker consumes event
4. projection row is upserted or recalculated

### 9.3 Scheduled Refresh

Used for:

- aggregated dashboards
- expensive derived summaries
- low-priority operational reports

### 9.4 Manual Rebuild or Repair

Used for:

- failed projections
- historical backfill
- schema changes
- operational recovery

---

## 10. Staleness and Freshness Model

Projections are eventually consistent and must expose freshness state.

### 10.1 Staleness Metadata

Each maintained projection should track:

- `source_id`
- `source_type`
- `source_version` or watermark
- `projection_version`
- `refreshed_at`
- `refresh_status`
- `last_error` (nullable)

### 10.2 Staleness Rules

- stale projection state must be detectable operationally
- stale projections may still be usable for read scenarios depending on policy
- protected actions must re-read authoritative state when correctness matters

---

## 11. Search Contract Model

Search endpoints should return stable, compact result contracts.

### 11.1 Search Request Shape

Recommended request fields:

- `query` (nullable)
- `filters`
- `sort`
- `page`
- `page_size`
- `cursor` (optional alternative)
- `fields` (optional for sparse selection)
- `scope_context`

### 11.2 Search Result Shape

Recommended response fields:

- `items`
- `total_count` or `next_cursor`
- `applied_filters`
- `sort`
- `projection_key`
- `projection_version`
- `generated_at`

### 11.3 Search Row Rules

Each row should include:

- stable identifier
- display summary
- status summary
- key timestamps or dates
- key scope information if relevant
- only minimal preview fields needed for the list context

---

## 12. Filtering and Sorting Rules

Projection definitions must declare allowed filters and sorts explicitly.

### 12.1 Filter Rules

- filterable fields must be indexed or efficiently retrievable
- filter semantics must be stable and documented
- cross-domain free-form filters should be limited unless intentionally supported

### 12.2 Sort Rules

- sortable fields must be declared in the projection contract
- default sort must be deterministic
- null handling should be consistent and documented

---

## 13. Search Safety Rules

Search is a retrieval mechanism, not an authority.

Rules:

- search results must not be used as the sole input for protected updates
- stale summaries must not override authoritative state
- sensitive fields should be excluded or masked based on authorization context
- result visibility must still respect role and scope rules

---

## 14. Authorization Model for Search

Search and list retrieval must respect platform authorization.

### 14.1 Visibility Rules

Visibility may depend on:

- role
- location or organization scope
- workflow state
- ownership or assignment
- field sensitivity

### 14.2 Enforcement Rules

- access filtering should happen server-side
- UI-only hiding is not sufficient
- if row-level filtering is expensive, projection design should account for security slicing up front

---

## 15. Low-Bandwidth Retrieval Patterns

The platform should optimize for constrained environments.

Recommended patterns:

- compact summary rows
- server-side pagination
- cursor or keyset pagination where helpful
- sparse field selection for larger lists
- recent-cache support on client for non-authoritative convenience
- delta refresh where practical for queue-like screens

Avoid:

- returning full documents for search pages
- large nested payloads in summary APIs
- client-side filtering of very large raw result sets

---

## 16. Projection Rebuild and Recovery

Every projection must define a rebuild strategy.

### 16.1 Rebuild Triggers

- schema version changes
- projection code changes
- corruption or drift detection
- operational recovery
- bulk import or migration backfill

### 16.2 Rebuild Rules

- rebuild must be idempotent
- rebuild must not alter authoritative records
- rebuild progress should be visible operationally
- partial rebuild scope should be supported where feasible

---

## 17. Projection Ownership Rules

Each projection must have a clear owner.

Ownership responsibilities include:

- defining projection contract
- maintaining projection logic
- defining rebuild strategy
- defining security behavior
- defining staleness tolerance

Recommended ownership pattern:

- kernel owns projection mechanism and contracts
- domain packs own domain-specific projection definitions and fields
- adapters may own integration-facing projections only where needed

---

## 18. Storage Guidance

Recommended storage approaches include:

- relational projection tables for common searches
- materialized summary tables for dashboards
- append-friendly event-driven refresh tables for work queues
- optional specialized search index for full-text scenarios

Recommended table families:

- `projection_definitions`
- `projection_status`
- domain-specific projection tables
- `projection_rebuild_jobs`
- `projection_refresh_failures`

Suggested indexing priorities:

- status + date filters
- location or scope filters
- owner or assignee filters
- normalized display lookup fields
- number or code lookups

---

## 19. Event and Refresh Integration

Projection refresh must align with `event-outbox-consistency.md`.

Recommended event usage:

- `document.created`
- `document.updated`
- `document.finalized`
- `workflow.transitioned`
- `task.assigned`
- `task.completed`
- `integration.submission_failed`

Rules:

- projection refresh handlers must be idempotent
- duplicate events must not corrupt projections
- projection failures must be retried or dead-lettered according to policy

---

## 20. Dashboard and Aggregate Rules

Aggregated projections require extra care.

Rules:

- aggregated values must declare source scope and refresh cadence
- dashboards may tolerate more staleness than transactional lists
- dashboards must not be used as the sole basis for protected workflow actions
- expensive aggregates should prefer scheduled refresh or precomputation

---

## 21. API Guidance

Recommended API categories:

- search endpoints by projection key
- list endpoints for known worklists
- dashboard summary endpoints
- projection health/admin endpoints
- rebuild or repair admin actions

API rules:

- search APIs should not expose internal storage details
- projection keys should be stable public contracts where intentionally exposed
- query parameter semantics must remain consistent across versions

---

## 22. Example Domain Mappings

### 22.1 Clinic

- patient lookup summary
- current-day queue projection
- recent encounter summary projection
- unpaid invoice queue

### 22.2 OMS

- open orders search
- fulfillment queue projection
- return review worklist
- shipment exception dashboard

### 22.3 POS

- recent sales summary
- open register exception list
- refund review queue
- daily location totals dashboard

### 22.4 ERP

- purchase request inbox
- invoice approval queue
- overdue payable summary
- month-end exception dashboard

---

## 23. Governance Rules

- every projection must have a contract, owner, and rebuild strategy
- every search-facing projection must define security behavior
- projection shape changes must be versioned
- projection logic must not become a hidden second source of truth
- new list APIs should prefer projection-backed contracts over direct raw document queries

---

## 24. Recommended Initial Implementation Sequence

1. define projection definition registry
2. implement projection status tracking
3. implement event-driven refresh worker contract
4. create first summary projection tables for core document and task lists
5. define search API contract and pagination rules
6. add rebuild and repair admin operations
7. add aggregate dashboard projections where needed

---

## 25. Final Summary

The search and projection architecture provides the platform with a domain-agnostic read-model layer.

Its core responsibilities are:

- compact search summaries
- projection-backed list retrieval
- eventual-consistency read models
- explicit projection contracts
- rebuildable and observable query infrastructure

This allows the platform to serve efficient search, queue, and dashboard experiences across domains while keeping canonical documents and workflow state as the authoritative source of truth.
