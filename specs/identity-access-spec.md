# Identity and Access Specification

## 1. Purpose

This document defines the domain-agnostic identity and access architecture for the platform kernel.

Its purpose is to provide a reusable security and authorization model for authentication, sessions, user identity, role-based access control, scope restrictions, and protected action enforcement across all domain packs.

This specification assumes a single-tenant-per-deployment model.

---

## 2. Goals

- define a reusable identity model for platform and domain modules
- define authentication and session handling boundaries
- define authorization using RBAC plus contextual and state-aware controls
- support location and operational scope restrictions
- ensure protected actions are enforced consistently on the server
- make identity-related changes auditable and observable

---

## 3. Non-Goals

This document does not define:

- detailed UI login screen behavior
- external identity provider implementation specifics
- full IAM federation protocols in detail
- infrastructure perimeter security controls
- multi-tenant identity isolation beyond single deployment assumptions

---

## 4. Design Principles

1. **Server-side enforcement is authoritative**  
   Frontend controls improve UX but never replace backend authorization.

2. **Authentication and authorization are separate concerns**  
   Authentication establishes identity. Authorization determines allowed actions.

3. **RBAC alone is not enough**  
   Protected actions may also depend on context, workflow state, ownership, and scope.

4. **Least privilege by default**  
   Users and services should receive only the permissions needed for their role.

5. **Identity and access decisions must be auditable**  
   Logins, permission changes, privileged actions, and access failures should be traceable.

6. **Scopes are first-class**  
   Location, branch, organization, and assignment scope must be enforced through explicit rules.

7. **Kernel defines mechanics, domains define meaning**  
   The kernel defines user, role, permission, and scope mechanisms. Domain packs define domain-specific permissions and context rules.

---

## 5. Core Concepts

### 5.1 `User`

Represents an authenticated system actor.

Responsibilities:

- login identity
- session ownership
- action attribution
- role bindings
- default scope association

### 5.2 `Role`

Represents a named collection of permissions.

Roles are reusable grouping mechanisms, not hardcoded business meanings.

### 5.3 `Permission`

Represents an atomic or grouped authorization capability.

Examples:

- create document draft
- approve governed action
- manage configuration
- view sensitive fields
- trigger reconciliation retry

### 5.4 `Session`

Represents an authenticated runtime context for a user or client.

### 5.5 `Scope`

Represents the operational boundary within which access applies.

Examples:

- deployment-wide
- organization-wide
- location-specific
- workflow-assignment specific

### 5.6 `Authorization Decision`

Represents the evaluated result of an access check.

Typical result shapes:

- `allow`
- `deny`
- `allow_with_constraints`

---

## 6. Identity Model

### 6.1 User Model

Recommended user fields:

- `user_id`
- `username`
- `status`
- `party_id` (nullable)
- `default_location_id` (nullable)
- `authentication_subject` (nullable)
- `last_login_at` (nullable)
- `created_at`
- `updated_at`

### 6.2 User Statuses

Recommended statuses:

- `active`
- `inactive`
- `suspended`
- `locked`
- `pending_activation`

### 6.3 User Rules

- a user must have a stable internal identity
- status must affect login eligibility and session validity
- user deactivation must block new authenticated activity
- historical action attribution must remain intact even if a user later becomes inactive

---

## 7. Authentication Model

The platform should support authentication as a pluggable boundary while keeping session semantics stable.

### 7.1 Authentication Responsibilities

- verify identity credentials or delegated authentication result
- establish authenticated principal
- create or refresh session context
- enforce account status rules
- record authentication audit events

### 7.2 Authentication Methods

Possible supported methods:

- username and password
- delegated SSO or identity provider integration
- service or system credential flow
- step-up or secondary verification for privileged actions where needed

### 7.3 Authentication Rules

- authentication result must resolve to a stable internal `User`
- failed login attempts should be observable and subject to protective policy
- authentication method selection must not weaken authorization rules

---

## 8. Session Model

### 8.1 Session Purpose

Sessions carry authenticated runtime context for API and UI access.

### 8.2 Session Fields

Recommended session fields:

- `session_id`
- `user_id`
- `status`
- `issued_at`
- `expires_at`
- `last_seen_at`
- `authentication_method`
- `client_metadata` (nullable)
- `current_location_scope` (nullable)
- `revoked_at` (nullable)

### 8.3 Session Rules

- session expiration must be explicit
- revoked sessions must no longer authorize requests
- sensitive operations may require re-authentication or step-up verification
- session creation, refresh, and revocation must be auditable

---

## 9. Authorization Model

The platform uses layered authorization:

- base role-based permissions
- contextual constraints
- scope restrictions
- workflow and document-state rules
- field or action sensitivity rules where needed

### 9.1 Base RBAC

RBAC defines broad permission sets by role.

Examples:

- create draft
- edit draft
- view projection
- execute approval action
- manage templates
- manage integrations

### 9.2 Contextual Constraints

Contextual rules may depend on:

- acting user
- assigned role
- location or branch
- ownership
- workflow assignment
- document state
- operation type

### 9.3 State-Aware Authorization

Authorization may differ by state.

Examples:

- a user may edit draft but not finalized records
- a reviewer may approve only submitted records
- a supervisor may reopen only finalized records under policy

---

## 10. Permission Model

### 10.1 Permission Structure

Recommended permission fields:

- `permission_key`
- `module_key`
- `resource_kind`
- `action_kind`
- `description`
- `status`

Permission examples:

- `document.create`
- `document.submit`
- `workflow.approve`
- `task.claim`
- `configuration.manage`
- `integration.replay`

### 10.2 Permission Rules

- permissions must be stable and machine-readable
- domain packs may contribute additional permission keys
- permissions should represent capability, not individual record identity

---

## 11. Role Model

### 11.1 Role Structure

Recommended role fields:

- `role_id`
- `role_key`
- `name`
- `scope_type`
- `status`
- `created_at`
- `updated_at`

### 11.2 Role Bindings

Role bindings associate users with roles in scope.

Recommended fields:

- `role_binding_id`
- `user_id`
- `role_id`
- `scope_type`
- `scope_id` (nullable)
- `effective_from`
- `effective_to` (nullable)
- `status`

### 11.3 Role Rules

- a user may have multiple role bindings
- role bindings may be time-bounded
- role bindings may be scoped by location or other allowed context
- expired bindings must no longer grant access

---

## 12. Scope Model

Because the platform is single-tenant-per-deployment, scope focuses on operational partitioning rather than tenant isolation.

### 12.1 Scope Types

Recommended scope types:

- `deployment`
- `organization`
- `location`
- `module`
- `workflow_assignment`
- `document_owner_context` (derived, not necessarily persisted as a standalone scope object)

### 12.2 Scope Rules

- scope restrictions must be evaluated server-side
- scope should be explicit in role bindings and authorization context where relevant
- a user may have access to one location but not another
- global roles should be used sparingly

---

## 13. Authorization Decision Flow

Protected actions should follow a consistent authorization path.

Recommended order:

1. authenticate session or principal
2. verify user status and session status
3. resolve base permissions from active role bindings
4. resolve applicable scope constraints
5. evaluate contextual constraints
6. evaluate workflow or document-state constraints if applicable
7. return allow, deny, or allow-with-constraints

The final protected action must still pass workflow and policy checks defined elsewhere.

---

## 14. Document and Workflow Access Rules

Identity and access must work closely with the document and workflow kernels.

### 14.1 Document Access

Access may depend on:

- document type
- document state
- location ownership
- relationship to the document
- field sensitivity

### 14.2 Workflow Access

Workflow actions may require:

- base permission
- current workflow state eligibility
- assignment to a task or role queue
- supervisor or override privilege

### 14.3 Task Access

Task access may depend on:

- assigned user
- assigned role
- location scope
- task type
- current task status

---

## 15. Sensitive Data Access

The platform should support restricted access to sensitive records or fields.

### 15.1 Sensitivity Controls

Examples:

- sensitive field masking
- restricted record visibility
- elevated access requirement
- print/export restrictions

### 15.2 Sensitive Access Rules

- sensitive access rules must be explicit and auditable
- field masking and row visibility should be enforced server-side where practical
- elevated access or override flows should record reason and actor

---

## 16. Service and System Identity

Not all actions come from interactive users.

### 16.1 Service Principal Model

The platform should support service or system identities for:

- background jobs
- projection workers
- integration connectors
- scheduled maintenance actions

Recommended service identity fields:

- `service_principal_id`
- `principal_key`
- `status`
- `allowed_operation_types`
- `credential_ref`

### 16.2 Service Identity Rules

- service identities must use least privilege
- service actions must be attributable separately from human users
- service principals must not reuse broad administrator permissions unnecessarily

---

## 17. Privileged Access and Override Controls

The platform should model elevated access explicitly.

### 17.1 Privileged Actions

Examples:

- reopen finalized document
- force-cancel workflow item
- activate sensitive configuration
- replay failed integration submission

### 17.2 Privileged Access Rules

- privileged actions require explicit permissions or policies
- high-risk actions may require step-up authentication or approval
- reasons and actor attribution must be recorded
- privilege use must be auditable and reviewable

---

## 18. Audit and Observability

The identity and access layer must emit audit and operational signals.

### 18.1 Auditable Events

- login success and failure
- logout and session revocation
- password or credential changes where applicable
- role binding created, updated, expired, or revoked
- privileged action attempts
- authorization denial for protected actions where meaningful

### 18.2 Recommended Metrics

- login success and failure rates
- active session count
- session revocation count
- authorization denial rate
- privileged action usage count
- locked or suspended user count

---

## 19. Storage Guidance

Recommended table families:

- `users`
- `roles`
- `permissions`
- `role_bindings`
- `sessions`
- `service_principals`
- `access_audit_events`
- `sensitive_access_events`

Suggested indexing priorities:

- users by `username`, `status`
- role bindings by `user_id`, `scope_type`, `scope_id`, `status`
- sessions by `user_id`, `status`, `expires_at`
- permissions by `permission_key`

---

## 20. API Guidance

Recommended API categories:

- authentication and session APIs
- current identity/context APIs
- user and role administration APIs
- permission inspection APIs
- session revocation APIs
- privileged access review or audit APIs

Examples:

- `POST /auth/login`
- `POST /auth/logout`
- `POST /auth/refresh`
- `GET /auth/me`
- `POST /users/{id}/role-bindings`
- `POST /sessions/{id}/actions/revoke`

Protected identity changes should use explicit action-style endpoints where governance matters.

---

## 21. Integration Guidance

Identity and access interact with other kernel areas.

- `workflow-task-policy-spec.md`
  - authorization is checked before protected workflow actions
- `document-kernel-spec.md`
  - document state and scope inform access decisions
- `configuration-featureflags-spec.md`
  - auth/session behavior may be controlled by managed configuration
- `integration-kernel-spec.md`
  - adapters and workers should use service identities

---

## 22. Example Domain Mappings

### 22.1 Clinic

- registration staff role can create and submit registration drafts in assigned locations
- doctor role can approve or finalize only allowed clinical actions by assignment and policy
- cashier role can access billing/payment views without unrestricted clinical details

### 22.2 OMS

- order review role can approve held orders in assigned fulfillment scopes
- warehouse role can work shipment tasks only for authorized locations

### 22.3 POS

- cashier role can create sales and refunds within assigned store scope
- supervisor role can approve void or discount overrides

### 22.4 ERP

- AP clerk role can process invoice queues but not perform protected period reopen actions
- finance manager role can approve higher-risk posting actions under policy

---

## 23. Governance Rules

- every protected action must declare required permissions and contextual access requirements
- every domain pack must register domain-specific permissions explicitly
- global administrator permissions should be minimized and auditable
- service identities must be reviewed like human privileged roles
- authorization logic must be testable and not hidden only in UI code

---

## 24. Recommended Initial Implementation Sequence

1. define user, role, permission, and role binding models
2. implement authentication and session lifecycle
3. implement base RBAC evaluation
4. add scope-aware authorization checks
5. integrate document and workflow state-aware enforcement
6. add service principal support
7. add sensitive access and privileged override auditing
8. expose admin and audit APIs

---

## 25. Final Summary

The identity and access architecture provides the platform with a reusable security control layer.

Its core model is:

- users authenticate through stable identity mechanisms
- sessions carry runtime context
- roles grant permissions
- scope and context constrain access
- protected actions are enforced server-side
- privileged and sensitive access is auditable

This allows the platform to support clinic, ERP, OMS, POS, and future domain packs on the same security and authorization foundation without embedding domain-specific access rules into the kernel itself.
