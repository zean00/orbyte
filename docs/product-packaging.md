# Product Packaging

This guide describes how Orbyte can be presented and shipped as a product rather than only as a source repository.

## Recommended Packaging Model

Orbyte is easiest to adopt when packaged in layers:

- kernel
  - shared enterprise runtime
- standard platform modules
  - common business capabilities maintained by the product team
- industry packs
  - vertical bundles such as clinic, retail, logistics, or administrative operations
- connector packs
  - reusable integration packages and adapter templates
- implementation assets
  - deployment blueprints, walkthroughs, sample configs, and reference payloads

## Suggested Editions

### Core Edition

Includes:

- kernel runtime
- generic model and document APIs
- identity and configuration
- workflow, policy, audit, and integration foundations

Best for:

- platform engineering teams
- custom product builders

### Operations Edition

Includes:

- Core Edition
- admin and operations surfaces
- analytics and reporting baseline
- integration management and DataOps assets

Best for:

- internal enterprise platforms
- operational back-office systems

### AI-Ready Edition

Includes:

- Operations Edition
- curated MCP surface
- service principal and delegated machine access guidance
- approved machine-action playbooks

Best for:

- teams integrating established agent platforms and copilots

## Suggested Bundle Shapes

### ERP Starter Pack

- master data
- procurement
- sales
- inventory
- approval workflows
- accounting connector baseline

### POS Starter Pack

- catalog
- branch configuration
- transaction documents
- offline sync support
- payment and back-office connectors

### MIS Starter Pack

- service requests
- approval routing
- dashboard and report baseline
- communication and case management modules

## Packaging Principles

- keep the kernel stable
- ship modules as explicit capability bundles
- document connector maturity clearly
- distinguish standard product modules from reference-only examples
- make AI connectivity an approved integration layer, not the product center of gravity

## Documentation To Ship Alongside Product Packages

- edition comparison
- deployment topology guidance
- support boundaries
- compatibility matrix
- connector maturity matrix
- module pack inventory
- machine-access governance guide
