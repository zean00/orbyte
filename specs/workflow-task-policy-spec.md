# Workflow, Task, and Policy Specification

## 1. Purpose

This document defines the domain-agnostic workflow, task, and policy architecture for the platform kernel.

Its purpose is to ensure that governed actions, state transitions, approvals, task routing, and rule evaluation are handled consistently across all domain packs.

This specification applies to canonical records such as documents, entities, tasks, and workflow instances, as well as to domain-defined actions and policies.

---

## 2. Goals

- define a reusable state machine model
- define how actions are requested, validated, and executed
- define human and system task handling
- define approval and review mechanics
- define policy evaluation structure and timing
- standardize auditability and extensibility of governed actions

---

## 3. Non-Goals

This document does not define:

- domain-specific workflow states for clinic, ERP, OMS, or POS
- BPMN-level visual modeling requirements
- organization-specific approval trees in full detail
- UI layout for inboxes or task dashboards
- pricing or tax calculation formulas themselves

---

## 4. Design Principles

1. **All governed transitions are explicit**  
   Important state changes must occur through named actions, not hidden side effects.

2. **Workflow owns transition semantics**  
   A protected state change must be validated by the workflow engine before commit.

3. **Policies decide eligibility, not persistence**  
   Policy evaluation determines whether an action is allowed or what path applies, but does not directly write state.

4. **Tasks represent work, not truth**  
   Tasks coordinate human or system effort, but authoritative record state remains on the canonical object.

5. **Approvals are first-class**  
   Approval and review actions must be modeled explicitly and audited.

6. **Domain packs provide meaning**  
   The kernel provides mechanics; domains contribute state names, action catalogs, and rule definitions.

7. **Audit is mandatory**  
   Every meaningful workflow action must produce an attributable audit trail.

---

## 5. Core Concepts

### 5.1 Workflow

A workflow is a governed state machine attached to a canonical object such as a `Document`, `Entity`, or `Task`.

It defines:

- valid states
- valid transitions
- action names
- entry and exit rules
- approval requirements
- side-effect hooks

### 5.2 Action

An action is a named request to change the state or disposition of a governed object.

Examples:

- `submit`
- `approve`
- `reject`
- `reopen`
- `cancel`
- `post`
- `dispatch`

### 5.3 Task

A task is an assignable unit of work created by workflow or policy logic.

Tasks may be:

- human tasks
- system tasks
- review tasks
- exception tasks

### 5.4 Policy

A policy is a configurable decision rule used to evaluate permissions, guards, routing, thresholds, or eligibility.

### 5.5 Approval

An approval is a specialized governed action that confirms or rejects a proposed transition or decision point.

---

## 6. Workflow Model

### 6.1 Workflow Definition

Each workflow definition should include:

- `workflow_definition_key`
- `version`
- `target_type`
- `states`
- `actions`
- `transitions`
- `entry_rules`
- `exit_rules`
- `policy_bindings`
- `task_templates` (optional)
- `side_effect_hooks` (optional)
- `status`

### 6.2 State Model

The platform does not enforce domain-specific state names, but each state should be classified by semantics.

Recommended state semantic classes:

- `draft`
- `pending_review`
- `approved`
- `rejected`
- `finalized`
- `cancelled`
- `amended`
- `closed`

Domain packs may define concrete states as needed, but their semantics must remain clear.

### 6.3 Transition Model

Each transition should define:

- `from_state`
- `action_key`
- `to_state`
- `allowed_actor_types`
- `required_permissions`
- `guard_policy_keys`
- `approval_mode`
- `creates_tasks` (optional)
- `emits_events`
- `failure_behavior`

### 6.4 Transition Rules

- a state change must be triggered by an action
- transitions must be validated against current state
- transitions must check authorization and context
- transitions must check bound policies and guards
- transitions must be rejected if current record version is stale
- transitions must be auditable

---

## 7. Action Execution Model

### 7.1 Action Request Lifecycle

The standard action flow is:

1. actor requests action on a governed object
2. system resolves workflow definition and current state
3. authorization is checked
4. policy guards are evaluated
5. approval requirements are evaluated
6. transition or task outcome is determined
7. authoritative state is committed
8. audit and outbox events are written
9. follow-up tasks or async work may be created

### 7.2 Action Outcomes

An action may result in:

- direct state transition
- creation of a pending approval task
- creation of an exception task
- no-op rejection with explicit reason
- deferred async follow-up work

### 7.3 Action Request Fields

Recommended action request fields:

- `action_key`
- `target_type`
- `target_id`
- `expected_version` or `etag`
- `actor_id`
- `reason` (nullable)
- `context`
- `idempotency_key`

---

## 8. Task Model

### 8.1 Task Purpose

Tasks represent work that must be performed before a workflow can proceed or complete.

Tasks are not the source of truth for the business object; they are workflow coordination artifacts.

### 8.2 Task Types

Recommended task categories:

- `approval`
- `review`
- `data_correction`
- `fulfillment`
- `exception_resolution`
- `reconciliation`
- `system_followup`

### 8.3 Task Assignment Model

Tasks may be assigned to:

- a specific user
- a role queue
- a location-scoped role queue
- a system worker type

### 8.4 Task States

Recommended task states:

- `open`
- `claimed`
- `in_progress`
- `completed`
- `cancelled`
- `expired`

### 8.5 Task Fields

Recommended task fields:

- `task_id`
- `task_type`
- `workflow_instance_id`
- `owner_type`
- `owner_id`
- `assignee_user_id` (nullable)
- `assignee_role_id` (nullable)
- `scope_location_id` (nullable)
- `status`
- `priority`
- `due_at` (nullable)
- `created_at`
- `claimed_at` (nullable)
- `completed_at` (nullable)
- `resolution_code` (nullable)
- `resolution_note` (nullable)

### 8.6 Task Rules

- a task must be attributable to a parent workflow or action context
- task completion may trigger a workflow action but is not itself the workflow state transition unless explicitly modeled that way
- stale or obsolete tasks must be cancellable when workflow state changes invalidate them
- task state changes must be audited when meaningful

---

## 9. Approval Model

### 9.1 Approval Modes

The platform should support at least these approval modes:

- `none`
- `single_step`
- `multi_step_sequential`
- `multi_step_parallel`
- `policy_selected`

### 9.2 Approval Step Definition

Each approval step should define:

- `step_key`
- `step_order`
- `required_role` or `assignee_rule`
- `min_approvals`
- `approval_action_key`
- `rejection_action_key`
- `escalation_policy_key` (nullable)

### 9.3 Approval Rules

- approval authority must be checked at time of action
- approvals must record actor, time, and outcome
- rejected approval chains must define whether the record returns to draft, enters rejected, or creates correction work
- skipped or auto-approved steps must still be attributable to policy or system reason

---

## 10. Policy Model

### 10.1 Policy Purpose

Policies provide configurable decision logic used by workflow, authorization, assignment, and calculation-adjacent modules.

The kernel provides the evaluation framework. Domain packs and deployment configuration provide concrete policies.

### 10.2 Policy Categories

Recommended categories:

- `authorization_policy`
- `transition_guard_policy`
- `approval_routing_policy`
- `assignment_policy`
- `numbering_policy`
- `eligibility_policy`
- `exception_policy`
- `reopen_policy`
- `cancellation_policy`

### 10.3 Policy Definition Fields

Each policy definition should include:

- `policy_key`
- `policy_type`
- `version`
- `scope_type`
- `scope_id` (nullable)
- `target_type`
- `status`
- `effective_from`
- `effective_to` (nullable)
- `definition_ref`

### 10.4 Policy Evaluation Result

Each evaluation should produce a structured result such as:

- `result` (`allow`, `deny`, `require_task`, `require_approval`, `route`, `defer`)
- `reason_code`
- `reason_message`
- `derived_values` (optional)
- `policy_key`
- `policy_version`

### 10.5 Policy Rules

- policies must be deterministic for a given input context
- policies must be versioned
- policy changes should not silently change historical decisions already committed
- evaluation inputs must be auditable where decisions affect protected actions

---

## 11. Policy Evaluation Points

Policies may be evaluated at the following points:

- before action is accepted
- during transition guard checks
- when determining approval path
- when assigning tasks
- when deciding whether reopen or cancel is allowed
- when selecting numbering rules
- when handling exception or escalation paths

The workflow engine must define which policy evaluation points are authoritative for each action.

---

## 12. Authorization and Workflow Interaction

Authorization and workflow are related but distinct.

- authorization answers whether the actor may attempt the action
- workflow answers whether the action is valid in the current state
- policy answers whether context-specific conditions permit, route, or constrain the action

All three must pass for a protected action to succeed.

---

## 13. State Transition Algorithm

For a protected action, the recommended evaluation order is:

1. load canonical object and current version
2. verify actor identity and base permission
3. resolve workflow definition and current state
4. verify action is valid from current state
5. evaluate contextual authorization constraints
6. evaluate transition guard policies
7. determine approval requirement or task routing
8. commit state change or task creation
9. write audit trail
10. write domain events / outbox records

This order should remain consistent across modules.

---

## 14. Override and Exception Handling

The platform may support controlled override behavior for authorized actors.

### 14.1 Override Rules

- overrides must be explicit actions, not hidden bypasses
- overrides require dedicated permissions or policy grants
- overrides must capture reason and actor attribution
- overridden transitions must remain auditable and searchable

### 14.2 Exception Tasks

When standard workflow cannot continue, the platform may create exception tasks for:

- data correction
- supervisor review
- policy conflict resolution
- reconciliation

---

## 15. Audit Requirements

The following must be auditable:

- action requests
- successful transitions
- rejected transitions where meaningful
- approval decisions
- task assignments and task completion where meaningful
- policy decisions affecting protected actions
- override and exception handling

Recommended audit fields:

- `event_id`
- `target_type`
- `target_id`
- `workflow_instance_id` (nullable)
- `action_key`
- `actor_id`
- `actor_role` (nullable)
- `timestamp`
- `from_state` (nullable)
- `to_state` (nullable)
- `policy_refs` (nullable)
- `reason`
- `result_status`

---

## 16. API Guidance

### 16.1 Action APIs

Protected state changes should use explicit action endpoints rather than generic update endpoints.

Examples:

- `POST /documents/{id}/actions/submit`
- `POST /documents/{id}/actions/approve`
- `POST /documents/{id}/actions/reject`
- `POST /tasks/{id}/actions/claim`
- `POST /tasks/{id}/actions/complete`

### 16.2 Task APIs

Task APIs should support:

- list open tasks by role, user, and scope
- claim task
- release task
- complete task
- cancel obsolete task

### 16.3 Policy APIs

Policy management APIs should support:

- register or update policy versions
- activate or retire policies
- inspect current effective policy bindings
- simulate policy evaluation for diagnostics where allowed

---

## 17. Storage Guidance

Recommended table families:

- `workflow_definitions`
- `workflow_states`
- `workflow_transitions`
- `workflow_instances`
- `workflow_actions`
- `approval_steps`
- `approval_actions`
- `tasks`
- `task_assignments`
- `policy_definitions`
- `policy_bindings`
- `policy_evaluation_logs` (optional or sampled)

Suggested indexing priorities:

- workflow instances by `owner_type`, `owner_id`
- tasks by `status`, `assignee_role_id`, `assignee_user_id`, `scope_location_id`
- policy definitions by `policy_type`, `target_type`, `status`, `effective_from`

---

## 18. Event Guidance

Important workflow and task events may include:

- `workflow.started`
- `workflow.transitioned`
- `workflow.approval_requested`
- `workflow.approved`
- `workflow.rejected`
- `task.created`
- `task.claimed`
- `task.completed`
- `task.cancelled`
- `policy.override_used`

These events should follow the event envelope and outbox rules defined in `event-outbox-consistency.md`.

---

## 19. Example Domain Mappings

### 19.1 Clinic

- `encounter submit` -> action on `Document`
- `doctor sign-off` -> approval step
- `pharmacy review` -> task
- `reopen with reason` -> controlled override policy

### 19.2 OMS

- `release order` -> workflow action
- `fraud review` -> exception task
- `warehouse pick approval` -> approval or task depending domain policy

### 19.3 POS

- `complete sale` -> finalize action
- `void sale` -> controlled cancellation policy
- `supervisor approval for discount` -> approval step driven by threshold policy

### 19.4 ERP

- `approve purchase request` -> multi-step approval
- `reopen closed period transaction` -> restricted override policy
- `assign AP review` -> role queue task

---

## 20. Governance Rules

- every governed canonical type must declare whether it uses workflow
- every protected action must define permission and workflow semantics
- every workflow definition must define transition guards and audit behavior
- every task type must define assignment and completion semantics
- every policy type must define evaluation inputs and result schema
- domain packs may extend workflow catalogs, but must not bypass kernel enforcement

---

## 21. Final Summary

The workflow, task, and policy architecture gives the platform a reusable control layer for governed business actions.

Its core model is:

- actions are explicit
- workflows own transitions
- tasks coordinate work
- policies drive context-sensitive decisions
- approvals are first-class
- all meaningful decisions are auditable

This allows clinic, ERP, OMS, POS, and future domain packs to implement their own business processes on top of the same platform mechanics without contaminating the kernel with domain-specific rules.
