# Module System

This guide explains how Orbyte modules work and how to build them.

## Why Modules Exist

Modules are the main product extension mechanism in Orbyte.

They allow teams to add business capability without rewriting the kernel. A module can describe business data, process behavior, user-facing surfaces, policy hooks, search definitions, analytics assets, and machine-consumable tools.

## What a Module Can Contribute

A module manifest can define:

- configuration definitions
- reference types and records
- models
- documents
- workflows
- datasets
- search indexes
- permissions and role templates
- policy hooks
- frontend menus, actions, and views
- self-service APIs
- offline packages
- templates
- MCP tools, resources, and apps

## Module Lifecycle

At a high level:

1. a manifest is registered in the repo
2. the app boots and validates the manifest
3. the kernel seeds module-owned capabilities
4. the module becomes available to runtime surfaces

Modules can also be enabled or disabled through module lifecycle controls.

## Mental Model

The easiest way to think about a module is:

- a contract package
- a capability package
- a UI package
- a policy package

You are not only adding code. You are declaring what the platform should know, expose, govern, and operate for a business domain.

## Profiles

Orbyte can start with different business profiles through `APP_DOMAIN_PROFILE`.

Profiles determine which business module manifests are included at bootstrap time.

That profile layer sits on top of the built-in kernel packs that always provide the platform's core modules. In other words:

- kernel packs define the core platform foundation
- profile manifests define domain-specific business capability

## Developing a Module

When you build a business module, you are typically adding it to the profile-driven module registry under `internal/modules`.

When you build kernel-level capability, you should usually extend the built-in kernel packs instead of introducing a business module.

### Approach 1: Use the Module Generator

The recommended path is `modulegen`.

Examples:

```bash
go run ./cmd/modulegen module validate --spec examples/modulegen/hybrid.yaml
go run ./cmd/modulegen module explain --profile backoffice --key crm --name "CRM" --kind hybrid
go run ./cmd/modulegen module init --spec examples/modulegen/hybrid.yaml
```

This scaffolds files under:

- `internal/modules/<module_key>/`

It also patches the module registry so the new module is included at startup.

See [Module Generator](./modulegen.md) for command details.

### Approach 2: Author the Manifest Manually

You can also create a module by hand if you need full control.

Typical steps:

1. create a module package under `internal/modules/<module_key>/`
2. define a manifest
3. register it through the module registry and the appropriate profile
4. add tests
5. run `make test` and `make lint`

## Example Module Design

A simple procurement module might contribute:

- model: `vendor`
- document: `purchase_request`
- workflow: `purchase_request_approval`
- permissions: `procurement.request.create`, `procurement.request.approve`
- search index: `purchase_requests`
- report dataset: `procurement_requests`
- MCP tool: `procurement.request.submit`

That keeps the business capability self-contained while still using shared kernel services.

## Module Authoring Checklist

Before shipping a module, confirm:

- domain vocabulary is clear
- permissions are explicit
- configuration keys are scoped correctly
- workflow states are stable
- contracts are versioned where needed
- reporting and search outputs are intentional
- AI-facing tools are permission-gated and auditable

## Design Guidance

When designing modules:

- keep kernel-level concerns out of business modules
- prefer manifest-driven definitions over one-off custom handlers
- define permissions explicitly
- define contracts for integration and MCP exposure deliberately
- add policy hooks for runtime governance where needed
- think about search, reporting, and offline support early if the business domain needs them

## Good Module Boundaries

Strong module candidates:

- customer management
- inventory
- order management
- receivables
- procurement
- clinic operations
- field service

Poor module boundaries:

- random collections of unrelated views
- infrastructure settings that belong in the kernel packs
- business logic that bypasses the module manifest model entirely

## Module Compatibility

Modules should:

- declare compatibility with the kernel version line
- avoid breaking manifest assumptions across minor releases
- prefer additive evolution

See [Release and Compatibility Policy](./release-policy.md).

## Testing Modules

At minimum:

- validate manifest registration
- test permissions and policy behavior
- test model and document flows
- test reporting or search contributions if present
- test integration contracts if the module exposes them

## Recommended Module Evolution Strategy

- start with one narrow domain
- prove the manifest shape and workflows
- expose only the minimum useful API and MCP surface
- add analytics and integrations after the core business flow is stable
- avoid large cross-domain modules that become a second kernel

## Related Guides

- [Architecture](./architecture.md)
- [Features](./features.md)
- [Module Generator](./modulegen.md)
