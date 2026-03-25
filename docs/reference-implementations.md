# Reference Implementations

This guide describes how Orbyte can be packaged into reference solutions for common enterprise application categories.

**Note:** The sections below describe reference patterns and design guidance. The only currently implemented business module is `clinic_registration` (see Vertical Industry Platform section). All other patterns are guides for building new modules, not pre-built bundles.

## Why Reference Implementations Matter

A platform becomes easier to adopt when users can see how its generic kernel maps to recognizable product shapes.

Reference implementations help with:

- evaluation
- onboarding
- implementation planning
- module design discipline

## Reference Pattern: ERP Core

### Suggested Module Families

- `master_data`
- `sales`
- `procurement`
- `inventory`
- `approvals`
- `reporting`
- `integration_accounting`

### Suggested Kernel Usage

- models for customers, suppliers, items, chart segments
- documents for requests, orders, receipts, invoices
- workflows for approval and exception handling
- integrations for finance, warehouse, and logistics systems

### AI Opportunities

- approval assistant
- procurement copilot
- order exception triage
- operational status summaries

## Reference Pattern: POS and Retail Operations

### Suggested Module Families

- `catalog`
- `store_operations`
- `pos_transactions`
- `inventory_sync`
- `branch_reporting`
- `payment_integrations`

### Suggested Kernel Usage

- location-scoped config for branch behavior
- offline packaging and sync for store operations
- documents for sale, refund, transfer, adjustment
- search for catalog and transaction lookup

### AI Opportunities

- cashier support assistant
- branch anomaly detection workflow
- stock discrepancy resolution support

## Reference Pattern: MIS and Administrative Platform

### Suggested Module Families

- `service_requests`
- `case_management`
- `administration`
- `communications`
- `executive_reporting`

### Suggested Kernel Usage

- models for administrative entities
- documents for requests and cases
- workflow for routing and approvals
- analytics for operational dashboards

### AI Opportunities

- staff copilot
- case summarization
- operator guidance

## Reference Pattern: Vertical Industry Platform

### Implemented Example

- **clinic** - Full `clinic_registration` module with patient profiles, practitioner profiles, payer profiles, clinic registration documents, clinic encounter documents, and related workflows

### Reference Patterns (Not Yet Implemented)

- field service
- logistics
- education operations
- regulated back-office workflows

### Suggested Design Rule

Keep shared governance, identity, configuration, workflow, and integration in the kernel. Put vertical vocabulary and domain flows in modules.

## Product Packaging Advice

If Orbyte is commercialized or distributed as a product, package it as:

- kernel
- standard module bundles
- industry packs
- connector packs
- implementation templates

This is easier to adopt than presenting the platform as a blank slate.
