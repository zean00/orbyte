# Organization and Scope Specification

## 1. Purpose

This document defines the domain-agnostic organization and scope architecture for the platform kernel.

Its purpose is to provide a reusable structural model for organization hierarchy, operational locations, responsibility boundaries, and scope resolution across all domain packs such as clinic, ERP, OMS, and POS.

This specification assumes a single-tenant-per-deployment model.

---

## 2. Goals

- define the platform's structural organization model
- separate deployment ownership from operational scope
- standardize organization, branch, location, and operating unit concepts
- define how scope is resolved for identity, workflow, documents, search, and configuration
- support controlled cross-location and organization-wide operations
- keep the model domain-neutral and reusable

---

## 3. Non-Goals

This document does not define:

- multi-tenant isolation strategies
- legal entity accounting rules in detail
- HR reporting structures in full detail
- facility layout or room-level operational modeling unless needed by a domain pack
- geospatial or mapping system design

---

## 4. Design Principles

1. **Single deployment, multiple operational scopes**  
   The deployment is owned by one organization context, but work may occur across many branches and locations.

2. **Structure and access are related but distinct**  
   Organization hierarchy provides context; authorization decides what a user may do within that context.

3. **Scope must be explicit**  
   Operational scope should be represented directly, not inferred loosely from UI or naming.

4. **Domain-neutral structure first**  
   The kernel defines reusable organization primitives. Domain packs may map their own terms onto them.

5. **Cross-scope operations must be deliberate**  
   Actions spanning multiple locations or units should be explicit and auditable.

6. **Configuration and workflow depend on scope**  
   Scope resolution must be consistent for permissions, policies, numbering, templates, and search visibility.

---

## 5. Core Concepts

### 5.1 `Organization`

Represents the top-level operating business context for the deployment.

Examples:

- clinic operator
- retail company
- logistics operator
- service business

### 5.2 `Location`

Represents a physical or operational place where work occurs.

Examples:

- branch
- store
- clinic site
- warehouse
- service counter

### 5.3 `OperatingUnit`

Represents a logical operational grouping used for work ownership, process routing, or responsibility.

Examples:

- front office
- pharmacy unit
- fulfillment team
- finance team

An operating unit may or may not map one-to-one with a physical location.

### 5.4 `Scope`

Represents the boundary in which a rule, permission, document, task, search result, or configuration applies.

### 5.5 `ScopeContext`

Represents the resolved runtime scope used in request handling.

It may include:

- active organization
- active location
- active operating unit
- allowed scope set
- current user default scope

---

## 6. Structural Model

### 6.1 Deployment Structure

Under the single-tenant model, the default structural hierarchy is:

- `deployment`
- `organization`
- `location`
- `operating_unit` (optional and domain- or deployment-dependent)

The kernel should not assume that all domains need every layer equally.

### 6.2 Organization Rules

- one deployment has one active top-level organization context
- the organization owns default policies, settings, and structural metadata
- organization-wide operations may span many locations, subject to policy and authorization

### 6.3 Location Rules

- a location belongs to one organization
- a location may have a parent location where hierarchical grouping is needed
- a location may be active, inactive, or restricted
- location is a first-class scope input for authorization, workflow, numbering, search, and configuration

### 6.4 Operating Unit Rules

- an operating unit may belong to an organization or a location depending on structure
- operating units should be used only when they represent meaningful operational routing or control
- operating units must not be introduced as vague duplicates of roles or teams without clear responsibility purpose

---

## 7. Canonical Organization Model

### 7.1 `Organization` Fields

Recommended fields:

- `organization_id`
- `organization_key`
- `name`
- `status`
- `default_locale`
- `default_timezone`
- `default_currency_code` (nullable)
- `settings_ref`
- `created_at`
- `updated_at`

### 7.2 `Location` Fields

Recommended fields:

- `location_id`
- `organization_id`
- `location_key`
- `location_type`
- `name`
- `status`
- `parent_location_id` (nullable)
- `address_ref` (nullable)
- `operating_schedule_ref` (nullable)
- `created_at`
- `updated_at`

### 7.3 `OperatingUnit` Fields

Recommended fields:

- `operating_unit_id`
- `organization_id`
- `location_id` (nullable)
- `unit_key`
- `unit_type`
- `name`
- `status`
- `created_at`
- `updated_at`

---

## 8. Scope Types

The platform should support the following scope types.

### 8.1 Structural Scopes

- `deployment`
- `organization`
- `location`
- `operating_unit`

### 8.2 Functional Scopes

- `module`
- `workflow_definition`
- `template`
- `external_system`

### 8.3 Contextual Scopes

- `assignment`
- `document_owner_context`
- `task_queue`

Rules:

- structural scopes define where work belongs
- functional scopes define where rules or resources apply
- contextual scopes define runtime constraints for specific operations

---

## 9. Scope Resolution Model

Scope resolution determines the effective organizational context for a request or background action.

### 9.1 Inputs to Scope Resolution

Scope may be resolved from:

- authenticated user identity
- role bindings
- requested location or unit context
- target document or task ownership
- API request parameters where allowed
- configuration defaults

### 9.2 Scope Resolution Output

The resolved scope context should include:

- `organization_id`
- `active_location_id` (nullable)
- `active_operating_unit_id` (nullable)
- `allowed_location_ids`
- `allowed_operating_unit_ids`
- `source_of_resolution`

### 9.3 Scope Resolution Rules

- resolution must be deterministic
- invalid or conflicting scope inputs must be rejected
- a user must not escalate scope simply by requesting another location id unless authorized
- system and service principals must also resolve scope explicitly

---

## 10. Scope in Authorization

Scope is a core authorization input.

### 10.1 Scope Enforcement Rules

- role bindings may be global or scope-bound
- access to records should be filtered by allowed structural scope where applicable
- workflow actions may depend on current location, operating unit, or assignment context
- search visibility must respect scope filters server-side

### 10.2 Cross-Scope Access

Cross-scope access should be explicit and controlled.

Examples:

- regional manager viewing multiple locations
- central finance processing organization-wide records
- admin maintaining configuration across all sites

Rules:

- cross-scope access requires explicit permission and binding
- cross-scope actions must remain auditable

---

## 11. Scope in Documents and Workflow

Documents and tasks often belong to an organizational scope.

### 11.1 Document Scope Rules

- every governed document should carry organization scope
- documents may additionally carry location scope
- operating unit scope may be added when routing or ownership requires it
- document scope should influence numbering, authorization, projection visibility, and template selection

### 11.2 Workflow Scope Rules

- workflow definitions may behave differently by organization or location configuration
- task routing may use location or operating unit scope
- approvals may require actors from a matching or supervising scope

---

## 12. Scope in Configuration and Feature Flags

Configuration and flags depend on structural scope resolution.

### 12.1 Configuration Scope Rules

- deployment-level configuration applies by default
- organization-level configuration may refine the deployment default
- location-level configuration may override where the configuration contract allows it
- operating-unit-level configuration should be used only where clearly justified

### 12.2 Feature Flag Scope Rules

- flags may apply deployment-wide or to narrower scopes
- scoped rollout must remain deterministic and observable
- protected business features should not rely on ambiguous scope activation

---

## 13. Scope in Search and Projections

Search and projection visibility depend on resolved scope.

Rules:

- projection definitions should declare whether they are scope-sensitive
- scope filtering must happen server-side
- projection rows should carry enough scope metadata for correct filtering
- dashboards spanning multiple locations must clearly state their scope basis

---

## 14. Scope in Integration

External integrations may be organization-wide or location-specific.

Rules:

- external system configuration may bind at deployment, organization, or location scope
- outbound submissions should preserve relevant scope metadata
- inbound integration handling must resolve target scope explicitly before mutating state
- reconciliation should show the affected scope clearly

---

## 15. Operational Calendar and Scheduling Context

Many domains need operational schedule awareness tied to scope.

### 15.1 Schedule Use Cases

- location operating hours
- unit availability windows
- service cutoff times
- business-date resolution

### 15.2 Schedule Rules

- schedules are scope-linked metadata, not standalone authorization rules
- workflows and policies may consult schedules when needed
- business-date policies may vary by location or operating unit

---

## 16. Naming and Mapping Rules

Domain packs may use familiar business terms, but must map them onto kernel concepts explicitly.

Examples:

- clinic `branch` -> `Location`
- clinic `poli/unit` -> `OperatingUnit` or domain-specific queue context
- retail `store` -> `Location`
- logistics `warehouse zone` -> domain-specific structure or `OperatingUnit`, depending responsibility meaning

Rules:

- domain aliases must not change kernel semantics
- if a domain concept has different operational meaning, it should remain domain-specific rather than forcing the kernel model to stretch incorrectly

---

## 17. Audit and Observability

Structural and scope changes are operationally significant.

### 17.1 Auditable Events

- organization settings changed
- location created, updated, activated, or retired
- operating unit created or restructured
- scope-bound role binding changed
- cross-scope privileged action executed

### 17.2 Recommended Metrics

- active locations by type
- inactive or misconfigured scope records
- cross-scope action count
- scope resolution failure count
- configuration override count by scope

---

## 18. Storage Guidance

Recommended table families:

- `organizations`
- `locations`
- `operating_units`
- `location_hierarchy`
- `scope_bindings` (optional where generalized binding storage is useful)
- `organization_settings`

Suggested indexing priorities:

- locations by `organization_id`, `status`, `location_type`
- operating units by `organization_id`, `location_id`, `status`
- hierarchy relationships by parent and child ids

---

## 19. API Guidance

Recommended API categories:

- organization profile APIs
- location management APIs
- operating unit management APIs
- scope resolution/context APIs
- location and unit lookup APIs

Examples:

- `GET /organization`
- `GET /locations`
- `POST /locations`
- `GET /auth/context`
- `GET /scope/resolve`

Protected structural changes should use explicit action-style APIs where governance matters.

---

## 20. Example Domain Mappings

### 20.1 Clinic

- one clinic operator organization
- multiple clinic branches as `Location`
- registration desk or outpatient unit as `OperatingUnit` where needed
- queue, schedule, and workflow routing constrained by branch scope

### 20.2 OMS

- one commerce organization
- fulfillment centers and pickup sites as `Location`
- fulfillment team as `OperatingUnit`
- order routing and inventory workflow depend on location scope

### 20.3 POS

- one retail operator organization
- stores as `Location`
- cashier desk or back office as `OperatingUnit` where operationally needed
- receipt templates and device settings vary by location

### 20.4 ERP

- one business organization
- offices or branches as `Location`
- finance or procurement as `OperatingUnit`
- approval routing may differ by organization-wide or branch-level scope

---

## 21. Governance Rules

- structural scope concepts must remain domain-neutral
- every scope-sensitive module must document how scope is resolved and enforced
- location and operating unit proliferation should be controlled and justified
- scope aliases from domain packs must map explicitly to kernel concepts
- cross-scope behavior must be deliberate, testable, and auditable

---

## 22. Recommended Initial Implementation Sequence

1. define organization, location, and operating unit models
2. implement structural lookup and hierarchy rules
3. implement scope resolution service
4. integrate scope into role bindings and authorization
5. add scope fields to document and task models
6. integrate scope-aware configuration and projection filtering
7. add cross-scope audit and admin APIs

---

## 23. Final Summary

The organization and scope architecture provides the structural context for the platform kernel.

Its core model is:

- one deployment operates within one organization context
- work occurs across locations and optional operating units
- scope is resolved explicitly for requests and background work
- authorization, workflow, configuration, search, and integrations depend on scope
- cross-scope operations are controlled and auditable

This gives the platform a reusable structural model that can support clinic, ERP, OMS, POS, and future domains without embedding domain-specific organizational assumptions into the kernel itself.
