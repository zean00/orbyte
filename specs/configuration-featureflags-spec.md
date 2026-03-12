# Configuration and Feature Flags Specification

## 1. Purpose

This document defines the domain-agnostic configuration and feature flag architecture for the platform kernel.

Its purpose is to provide a safe, observable, and reusable control plane for deployment settings, module settings, scoped overrides, and runtime feature toggles across all domain packs.

This specification assumes a single-tenant-per-deployment model.

---

## 2. Goals

- define a stable configuration model for the platform
- separate static code behavior from runtime-managed settings
- support scoped configuration and controlled overrides
- define feature flag semantics for optional behavior
- standardize configuration validation, versioning, and rollout control
- ensure configuration changes are auditable and operationally visible

---

## 3. Non-Goals

This document does not define:

- secret storage implementation details
- infrastructure provisioning configuration
- UI design for administration consoles
- dynamic pricing or workflow policy formulas themselves
- customer-level multi-tenant configuration isolation

---

## 4. Design Principles

1. **Configuration is data, not code**  
   Runtime-adjustable behavior should be expressed through managed configuration instead of code edits.

2. **Flags are temporary when possible**  
   Feature flags should support controlled rollout and rollback, not become permanent unmanaged branching.

3. **Scope must be explicit**  
   Every configuration value and feature flag must declare where it applies.

4. **Effective value resolution must be deterministic**  
   The same input context must always resolve to the same effective configuration.

5. **Changes must be auditable**  
   Configuration changes are operationally significant and must be attributable.

6. **Validation before activation**  
   Invalid configuration must not become effective silently.

7. **Secrets are separate**  
   Sensitive credentials should be referenced, not stored alongside general configuration where avoidable.

---

## 5. Configuration Model

### 5.1 Configuration Categories

The platform should support at least these configuration categories:

- `platform`
- `module`
- `workflow`
- `integration`
- `output_template`
- `reference_sync`
- `operational`
- `security`

### 5.2 Configuration Object

Each configuration entry should include:

- `configuration_key`
- `configuration_category`
- `scope_type`
- `scope_id` (nullable)
- `version`
- `status`
- `value_ref` or `value_payload`
- `schema_key`
- `effective_from`
- `effective_to` (nullable)
- `updated_by`
- `updated_at`

### 5.3 Configuration Statuses

Recommended statuses:

- `draft`
- `active`
- `retired`
- `invalid`
- `scheduled`

---

## 6. Scope Model

Because the platform is single-tenant-per-deployment, scope resolution is simpler but still necessary.

Recommended scope types:

- `deployment`
- `organization`
- `location`
- `module`
- `workflow_definition`
- `external_system`
- `template`
- `user_role` (optional and restricted)

Rules:

- the default root scope is `deployment`
- narrower scopes may override broader scopes only where the configuration contract allows it
- not every configuration key may be overridden at every scope

---

## 7. Effective Configuration Resolution

The platform must define deterministic effective configuration resolution.

### 7.1 Resolution Order

Recommended default precedence from lowest to highest:

1. system default
2. deployment scope
3. organization scope
4. location scope
5. module or feature-specific scope
6. context-specific override where explicitly allowed

### 7.2 Resolution Rules

- resolution order must be documented per configuration family
- missing values may fall back to default values if allowed
- conflicting active values at the same scope must be rejected
- effective configuration should be cacheable but invalidatable on change

### 7.3 Effective Configuration Output

Resolved output should include:

- `effective_value`
- `effective_source_scope`
- `effective_version`
- `resolved_at`

---

## 8. Configuration Schema and Validation

Each configuration key must be bound to a validation schema or contract.

### 8.1 Schema Responsibilities

- required fields
- value types
- allowed ranges
- allowed enum values
- override permission rules
- compatibility rules across versions

### 8.2 Validation Rules

- configuration must be validated before activation
- schema violations must block activation
- cross-key dependency validation may be required for complex modules
- validation failures must be visible to operators

### 8.3 Compatibility Rules

- incompatible configuration version changes must be explicit
- migration of configuration values should be controlled and auditable
- retired configuration contracts should remain readable for historical traceability

---

## 9. Feature Flag Model

Feature flags are controlled runtime toggles used to manage optional or staged behavior.

### 9.1 Feature Flag Object

Each feature flag should include:

- `flag_key`
- `flag_type`
- `scope_type`
- `scope_id` (nullable)
- `enabled`
- `rollout_metadata` (nullable)
- `status`
- `effective_from`
- `effective_to` (nullable)
- `updated_by`
- `updated_at`

### 9.2 Flag Types

Recommended flag types:

- `release_flag`
- `ops_flag`
- `experiment_flag`
- `kill_switch`
- `domain_activation_flag`

### 9.3 Flag Rules

- every flag must have a clear owner and purpose
- flags that alter protected business behavior must be documented and auditable
- kill switches should fail safe
- long-lived flags should be periodically reviewed and removed if no longer needed

---

## 10. Configuration vs Feature Flag Rules

Use configuration when:

- a setting represents stable business or operational parameters
- structured values are needed
- the value should be persisted and versioned as part of system behavior

Use feature flags when:

- a behavior needs fast enable/disable control
- a module or workflow path is being rolled out incrementally
- an operational kill switch is needed

Do not use feature flags as a substitute for full structured configuration.

---

## 11. Configuration Domains

### 11.1 Platform Configuration

Examples:

- timezone defaults
- numbering defaults
- attachment limits
- session behavior
- audit retention policy references

### 11.2 Module Configuration

Examples:

- document rules by module
- field visibility policies
- default statuses or templates
- module-specific search behavior

### 11.3 Workflow Configuration

Examples:

- approval enablement
- transition routing rules
- escalation timing
- reopen or cancel limits

### 11.4 Integration Configuration

Examples:

- external system activation
- mapping profile selection
- connector endpoints
- retry profile selection
- submission throttling limits

### 11.5 Operational Configuration

Examples:

- worker concurrency
- retry backoff profiles
- projection refresh settings
- maintenance mode states

---

## 12. Secret and Sensitive Configuration Rules

Secrets must be handled separately from ordinary configuration where possible.

Rules:

- configuration should store secret references, not raw secret values, unless no better option exists
- access to secret-linked configuration must be restricted
- secret rotation must not require business data model changes
- secret usage should be auditable at the operational level where practical

Examples:

- API credential reference
- signing key reference
- external connector token reference

---

## 13. Change Management

Configuration and flag changes are operationally significant changes.

### 13.1 Change Actions

Supported actions should include:

- create draft
- validate
- activate
- schedule activation
- retire
- rollback or restore prior version

### 13.2 Change Rules

- changes should be versioned, not silently overwritten
- protected configuration changes may require approval policies
- effective activation time must be explicit for scheduled changes
- activation should emit audit and operational events

---

## 14. Audit and Observability

The platform must track:

- who changed a configuration or flag
- what changed
- when it changed
- what scope it affected
- whether validation passed
- which version became effective

Recommended events:

- `configuration.created`
- `configuration.validated`
- `configuration.activated`
- `configuration.retired`
- `feature_flag.enabled`
- `feature_flag.disabled`

Recommended metrics:

- active flags by type
- invalid configuration count
- scheduled change count
- configuration activation failure count

---

## 15. Storage Guidance

Recommended table families:

- `configuration_definitions`
- `configuration_values`
- `configuration_versions`
- `configuration_validation_results`
- `feature_flags`
- `feature_flag_versions`
- `configuration_change_events`

Suggested indexing priorities:

- configuration by `configuration_key`, `scope_type`, `scope_id`, `status`
- feature flags by `flag_key`, `scope_type`, `scope_id`, `status`
- scheduled activations by `effective_from`, `status`

---

## 16. API Guidance

Configuration APIs should support:

- list definitions and current effective values
- validate draft values
- activate and retire values
- preview effective resolution by scope
- inspect version history

Feature flag APIs should support:

- list active flags
- enable or disable flags
- schedule flag activation or expiration
- inspect rollout metadata and history

Protected configuration changes should use explicit action endpoints, not raw generic patch semantics only.

---

## 17. Caching and Runtime Refresh

Effective configuration and flags may be cached for performance.

Rules:

- cache invalidation must be triggered on activation or retirement
- workers and background processes must refresh relevant configuration safely
- stale configuration use should be bounded and observable
- protected operational toggles should support fast refresh where needed

---

## 18. Example Domain Usage

### 18.1 Clinic

- enable approval requirement for a document type
- select official print template by location
- activate healthcare adapter connector profile

### 18.2 OMS

- enable order fraud review workflow
- set fulfillment timeout thresholds
- activate marketplace connector per operation type

### 18.3 POS

- enable manager override requirement for discount thresholds
- set receipt printing template per location
- use kill switch for a payment terminal connector

### 18.4 ERP

- activate multi-step approval for purchase requests
- set posting cutoff policy reference
- choose tax submission profile

---

## 19. Governance Rules

- every configuration key must have an owner, schema, and scope policy
- every feature flag must have an owner and intended removal or review policy
- configuration must not become an unbounded free-form blob without schema control
- flags must not permanently replace proper module design
- modules must prefer resolved configuration services over ad hoc direct reads

---

## 20. Recommended Initial Configuration Areas

The recommended first wave of managed configuration is:

- deployment metadata
- numbering defaults
- workflow activation settings
- output template selection
- integration endpoint and profile references
- worker and retry settings
- attachment and storage limits
- session and security policy references

The recommended first wave of feature flags is:

- module activation flag
- kill switch for external adapters
- staged workflow enablement
- experimental UI or action path flag

---

## 21. Final Summary

The configuration and feature flag architecture provides the platform with a controlled runtime behavior layer.

Its core model is:

- configuration holds structured, versioned settings
- feature flags provide controlled toggles
- scope and precedence are explicit
- validation happens before activation
- changes are auditable and observable

This allows the platform to remain stable in code while still supporting operational control, incremental rollout, module variation, and safe adaptation across domain packs.
