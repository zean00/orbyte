# Walkthroughs

This guide contains narrative end-to-end flows that help readers understand how Orbyte behaves as a real enterprise platform.

## Walkthrough 1: Create And Approve A Business Document

Scenario:

- an operator creates a request
- the request is submitted
- workflow routes it for approval
- an approver acts
- the platform records state changes and audit trails

Flow:

1. operator authenticates
2. operator creates a document through `/documents`
3. platform validates the payload against the registered document definition
4. document is saved in draft status
5. operator calls a submit action through `/documents/{id}/actions`
6. workflow logic evaluates the route
7. approval tasks are created
8. approver acts
9. document status changes
10. audit and domain events are recorded

What this demonstrates:

- document lifecycle
- workflow coupling
- permission checks
- auditability

## Walkthrough 2: Integration Submission To An External System

Scenario:

- a business action produces an outbound integration
- the platform sends the submission through a configured endpoint
- the external system succeeds or fails

Flow:

1. business action completes
2. platform records a domain event
3. an integration submission is created
4. adapter validates configuration and payload
5. the adapter executes against the target endpoint
6. the attempt is recorded
7. on success, the submission is marked processed
8. on failure, retry logic or dead-letter handling applies

What this demonstrates:

- explicit integration state
- retries and dead letters
- operational observability

## Walkthrough 3: External AI Copilot Uses MCP

Scenario:

- an external copilot wants to inspect integration health and help an operator resolve failures

Flow:

1. copilot authenticates through a service principal or approved delegated flow
2. copilot calls MCP `tools/list`
3. copilot calls `integration.dead_letter.list`
4. copilot reads supporting control-plane resources
5. copilot prepares a recommendation for the operator
6. if permitted and approved, copilot calls a replay tool
7. audit records capture the machine action

What this demonstrates:

- governed AI access
- machine-readable control plane
- safe external orchestration

## Walkthrough 4: Build A New Business Capability Through A Module

Scenario:

- a product team needs an inventory capability

Flow:

1. team generates a new module skeleton
2. team defines model and document contracts
3. team adds permissions and workflows
4. team adds search and reporting
5. team exposes selected APIs and tools
6. team tests and deploys the module

What this demonstrates:

- manifest-driven growth
- separation between kernel and domain logic

## Recommended Future Walkthroughs

As the product matures, add concrete walkthroughs with real payload examples for:

- POS offline sync
- multi-location configuration overrides
- report generation and delivery
- service principal onboarding
- policy hook rollout
