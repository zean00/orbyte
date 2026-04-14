# Modules

<p class="page-intro">
This guide summarizes the current module inventory and what each area currently contributes to the runtime.
</p>

## How To Read This Page

<div class="quick-links" markdown>

- [**Core Modules**](#core-modules)
  See the foundation modules that shape the platform everywhere.
- [**Business Modules**](#current-business-modules)
  Review the current business-domain capability inventory.
- [**Profile Modules**](#profile-modules)
  Understand the current example under `internal/modules/`.
- [**Current Constraints**](#current-constraints)
  See where module maturity is still uneven.

</div>

## Module Model

Orbyte currently composes capability from:

- built-in kernel packs under `internal/platform/app/kernelpacks_*.go`
- profile-driven business manifests under `internal/modules/`

In practice, most current business capability is delivered through kernel packs, while `internal/modules/clinic.go` is the clearest current profile-module example.

> Practical takeaway: if you are reading the current product surface, built-in kernel packs matter more than profile modules.

## Core Modules

These are the most important foundation modules in the current runtime.

| Module area | Current role |
| --- | --- |
| `platform.core` | bootstrap, HTTP, admin, config, observability, base platform services |
| `identity` | users, roles, service principals, auth policy, sessions |
| `documents` | document types, flows, draft/update patterns, generic request support |
| `analytics` | dashboard widgets, report definitions, analytics operations, preview support |
| `masterdata` | parties, contacts, customer profile, shared business master records |
| `reference_masterdata` | reference/look-up style master data |
| `organization_structure` | org units, departments, cost centers |
| `workflow_approval_policy` | approval policy setup and related workflow governance |
| `monitoring` / `integration` | runtime monitoring and integration operations hooks |

## Current Business Modules

### CRM

| Module key | Current capabilities | Current behavior |
| --- | --- | --- |
| `crm_core` | queues, tickets, comments, ticket activities, SLA policies, assignment rules, customer 360, leads, opportunities, activities, CRM widgets, CRM MCP tools | combines service CRM, customer context, and basic sales CRM inside one module |

Notable behavior today:

- ticketing is operationally usable
- customer 360 joins party/profile/contact/ticket/opportunity/activity data
- MCP tools exist for ticket, customer, lead, and opportunity flows
- skills exist for CRM investigation and intake flows

### Commercial And Promotions

| Module key | Current capabilities | Current behavior |
| --- | --- | --- |
| `commercial_core` | commercial records, related views, business search, templates, posting defaults, sales-oriented metadata | broad shared commercial base used by several downstream flows |
| `discount_core` | discount rules and discount management views | model-driven discount setup and maintenance |
| `promotion_core` | promotion campaigns, promotion codes, promotion redemptions, promotion planning metadata | promotion setup and planning-oriented module surface |

### Procurement, Inventory, And Fulfillment

| Module key | Current capabilities | Current behavior |
| --- | --- | --- |
| `procurement_core` | procurement models, vendor setup, posting defaults, procurement views | generic procurement/admin flows plus MCP-facing wrappers |
| `inventory_core` | inventory setup, inventory operations, inventory manager roles | inventory master/operational views and health-oriented MCP helpers |
| `inventory_finance_core` | inventory finance controls, reconciliation cases, count sessions | links inventory operations with finance control/reconciliation behavior |
| `fulfillment_core` | sales fulfillment flows and operator views | fulfillment and operational follow-through |
| `delivery_core` | delivery order workflows and operator capabilities | delivery-oriented operational module |
| `returns_core` | sales return flows and operator views | return processing from the sales side |
| `supplier_returns_core` | supplier return workflows and operator views | supplier-facing returns flow |

### Planning, Production, Traceability, And Recall

| Module key | Current capabilities | Current behavior |
| --- | --- | --- |
| `planning_core` | planning runs, planning proposals, planner flows | planning-oriented records and MCP summary wrappers |
| `production_core` | production orders/issues/outputs and planner roles | production operational flows |
| `production_costing_core` | production routings, cost rates, captures, allocations, variance views | cost and variance visibility over production operations |
| `traceability_core` | traceability analysis surface | narrower analytics/analysis capability today |
| `recall_core` | recall cases, recall actions, recall workflows | recall-oriented operational workflows |

### POS

| Module key | Current capabilities | Current behavior |
| --- | --- | --- |
| `pos_core` | POS setup, cashier/manager roles, POS operations, POS demo/validation flows | strong demo and validation presence for retail/POS scenarios |

### Finance

| Module key | Current capabilities | Current behavior |
| --- | --- | --- |
| `finance_reporting_core` | accounting periods, aging, reconciliations, ledger, journals, reports | broad finance reporting and accounting visibility |
| `finance_manual_journal_core` | manual journal operations and approvals | finance adjustment/journal workflow surface |
| `finance_collections_core` | collection cases, settlement exceptions, party/vendor statement runs | collections and follow-up operations |
| `finance_asset_core` | fixed assets, prepaid expenses, transfers, revaluation, disposal, impairment | asset-lifecycle oriented module |
| `treasury_core` | treasury operations and manager views | treasury-focused operational capability |
| `retail_finance_core` | retail finance manager capability | finance behavior specialized toward retail context |

### Workforce And Payroll

| Module key | Current capabilities | Current behavior |
| --- | --- | --- |
| `employee_workforce` | employee profiles, assignments, compensation profiles, workforce roles | employee and workforce structure surface |
| `workforce_attendance` | shifts, rosters, corrections, overtime, attendance approvals | deeper operational attendance module |
| `leave_policy_core` | leave policy, entitlement, leave profiles, balances, accruals, self-service leave APIs | includes both admin and self-service surface behavior |
| `employee_payroll_core` | payroll setup and operations | payroll-oriented management workflows |
| `payroll_remittance_core` | remittance setup, operations, approvals | payroll remittance follow-through |
| `employee_spend_core` | spend setup and spend operations | expense/spend lifecycle coverage |

### Analytics And Reporting

| Module key | Current capabilities | Current behavior |
| --- | --- | --- |
| `analytics` | widgets, dashboards, report templates, analytics viewers and operations | generic analytics layer used by many scenarios |

## Profile Modules

### `clinic_registration`

Current status:

- implemented under `internal/modules/clinic.go`
- available through the `clinic` domain profile
- demonstrates non-kernel module packaging

Current capabilities include:

- patient, practitioner, and payer profiles
- clinic registration and encounter workflows
- clinic-specific datasets
- clinic-specific roles and permissions

## Current Module Behavior Pattern

Across the repo, most modules currently behave in one of these ways:

1. fully metadata-driven generic UI modules
2. metadata-driven modules with custom summary/custom-entry endpoints
3. metadata plus specialized MCP wrappers for agent workflows
4. metadata plus seeded demo scenarios for validation

CRM, analytics, POS, and planning are the clearest current examples of modules with stronger agent-facing and validation-facing behavior.

## Module Dependencies

Modules declare dependencies in their manifests. Common dependency patterns in the current codebase:

- most business modules depend on `platform.core`
- many operational modules depend on `masterdata`
- workflow-heavy modules depend on `documents` and/or approval policy modules
- specialized profile modules can depend on kernel packs such as identity, documents, and reference masterdata

## Module Enablement

Modules are:

- registered at bootstrap
- validated during startup
- persisted through the module service
- controllable through module lifecycle controls and MCP admin/business tooling

## Current Constraints

- built-in kernel packs are more mature than profile modules
- some module areas are wide foundational surfaces rather than complete end-user products
- MCP coverage is uneven across modules: some have richer hand-authored tools than others

## Related Guides

- [Features](./features.md)
- [Architecture](./architecture.md)
- [Module Development](./module-development.md)

## Recommended Next Pages

<div class="next-steps" markdown>

- [Module Development](./module-development.md) for how to extend this inventory
- [Surfaces](./surfaces.md) to see where these modules appear in the UI and agent/runtime layers

</div>
