# Orbyte Documentation

This documentation describes Orbyte as a product: what it is, how it works, how to run it, how to extend it, and how to integrate external systems and AI agents with it.

## Documentation Map

- [Getting Started](./getting-started.md)
  - local setup
  - runtime modes
  - bootstrap behavior
  - first admin access
- [Concepts](./concepts.md)
  - product intent
  - platform mental model
  - scope of the kernel
- [Use Cases](./use-cases.md)
  - ERP, POS, MIS, and vertical application patterns
  - AI assistant and agent integration patterns
- [Architecture](./architecture.md)
  - runtime layers
  - service graph
  - data flow
  - extension model
  - rendered diagrams
- [MCP Target Architecture](./mcp-target-architecture.md)
  - ACP, MCP, and agent workspace roles
  - business discovery and analysis capability model
  - governed machine action model
  - advisor-pack strategy and roadmap
- [Features](./features.md)
  - core capabilities
  - enterprise platform functions
  - AI integration readiness
- [Components](./components.md)
  - major services
  - runtime surfaces
  - storage and processing components
- [Configuration](./configuration.md)
  - configuration model
  - scopes
  - built-in configuration areas
  - operational environment variables
- [Glossary](./glossary.md)
  - core platform terminology
  - shared vocabulary for product, implementation, and AI teams
- [Security and Governance](./security-and-governance.md)
  - identity and authorization model
  - policy hooks
  - machine access controls
- [Product Packaging](./product-packaging.md)
  - editions, bundles, and packaging strategy
- [Module System](./module-system.md)
  - manifest model
  - module lifecycle
  - developing modules
  - module generator
- [First Module Tutorial](./tutorial-first-module.md)
  - generate and review a starter module
  - understand where business behavior belongs
- [Reference Implementations](./reference-implementations.md)
  - ERP, POS, MIS, and vertical packaging patterns
- [Walkthroughs](./walkthroughs.md)
  - end-to-end business, integration, and AI-assisted flows
- [Deployment](./deployment.md)
  - local deployment
  - container deployment
  - production considerations
- [Integration](./integration.md)
  - external systems
  - contracts
  - MCP and agent connectivity
  - events and data exchange
- [Operations](./operations.md)
  - observability
  - runtime health
  - backup and restore
  - support activities
- [API and Contracts](./api-and-contracts.md)
  - HTTP APIs
  - MCP surface
  - schemas and compatibility
- [Sample Payloads](./sample-payloads.md)
  - representative JSON payloads for demos and implementation guidance

## Developer-Focused References

- [Development Workflow](./development.md)
- [Module Generator](./modulegen.md)
- [External Contract Governance](./contracts.md)
- [Release and Compatibility Policy](./release-policy.md)

## Docs Tooling

This repository includes a `mkdocs` site configuration and can be built locally with:

```bash
make docs-build
```

To preview the site locally:

```bash
make docs-serve
```

These commands expect the local docs environment to already be installed.

## Recommended Reading Order

If you are new to the platform:

1. [Getting Started](./getting-started.md)
2. [Concepts](./concepts.md)
3. [Use Cases](./use-cases.md)
4. [Architecture](./architecture.md)
5. [MCP Target Architecture](./mcp-target-architecture.md)
6. [Features](./features.md)
7. [Configuration](./configuration.md)
8. [Glossary](./glossary.md)
9. [Security and Governance](./security-and-governance.md)
10. [Module System](./module-system.md)
11. [Integration](./integration.md)
12. [Deployment](./deployment.md)

If you are evaluating Orbyte as a product:

1. [Concepts](./concepts.md)
2. [Use Cases](./use-cases.md)
3. [Features](./features.md)
4. [Architecture](./architecture.md)
5. [MCP Target Architecture](./mcp-target-architecture.md)
6. [Security and Governance](./security-and-governance.md)
7. [Integration](./integration.md)
8. [Deployment](./deployment.md)

If you are building on top of Orbyte:

1. [Getting Started](./getting-started.md)
2. [Configuration](./configuration.md)
3. [Glossary](./glossary.md)
4. [Module System](./module-system.md)
5. [First Module Tutorial](./tutorial-first-module.md)
6. [Reference Implementations](./reference-implementations.md)
7. [MCP Target Architecture](./mcp-target-architecture.md)
8. [API and Contracts](./api-and-contracts.md)
9. [Operations](./operations.md)
