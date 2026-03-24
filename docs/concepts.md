# Concepts

This guide explains the core ideas behind Orbyte.

## Product Position

Orbyte is a foundation platform for AI-native enterprise applications such as ERP, POS, MIS, and domain-specific operational systems.

It is not intended to be a single finished business application. It is the kernel and platform layer that business applications are built on.

## Who This Product Is For

The main audiences are:

- product teams building business applications on a shared platform
- enterprise implementation teams configuring and extending deployments
- platform engineers standardizing identity, workflow, configuration, and integration behavior
- AI platform teams that need safe access to enterprise data and actions

## What "AI-Native" Means Here

In Orbyte, AI-native does not mean the platform contains a built-in autonomous agent that runs the business.

It means the platform is prepared for AI connectivity by exposing:

- structured business data
- stable APIs
- governed tools and functions
- integration contracts
- policy controls
- service principal and delegation models
- auditability and operational safety

This makes Orbyte suitable for connection to external agent frameworks and established AI runtimes without coupling the business kernel to a specific agent product.

## Kernel vs Application

Think of Orbyte in two layers:

- kernel
  - shared enterprise runtime
  - identity, config, documents, workflows, search, integration, policy, observability
- application modules
  - domain-specific capabilities
  - for example clinic operations, commerce flows, supply chain, finance, or customer operations

The kernel should stay generic and durable. Business specialization should be delivered through modules.

## Product Packaging Model

A useful way to think about a shipped Orbyte product is:

- Orbyte kernel
  - the reusable runtime
- standard modules
  - platform-owned business packages
- customer or industry modules
  - organization-specific or sector-specific extensions
- external integration and AI layer
  - services, agents, and tools that consume the platform through contracts

## Core Building Blocks

Orbyte organizes enterprise behavior through several main concepts:

- modules
  - package business capabilities and platform extensions
- models
  - metadata-driven structured records
- documents
  - business transactions and stateful records with lifecycle and workflow
- workflows
  - routing, approval, and transition logic
- configuration
  - scoped runtime behavior by deployment, organization, and location
- policies
  - runtime governance and decision logic
- integrations
  - external systems, endpoints, contracts, mappings, submissions, retries, dead letters
- MCP and tools
  - governed machine-consumable surfaces for external agents and operator tooling

## Scope Model

Orbyte is designed for hierarchical enterprise operation. Important scopes include:

- deployment
- organization
- location

This scope model is used in configuration, permissions, runtime behavior, and policy decisions.

For product teams, this matters because the same module can behave differently across:

- an entire deployment
- a specific business entity
- a specific operating site

## Why Generic Models and Documents Both Exist

Orbyte uses both because enterprise systems usually need both:

- models are flexible and schema-driven for master data or generic operational entities
- documents are transactional and lifecycle-driven for business processes that need workflow, versioning, and state transitions

Examples:

- a customer or item can be represented as a model
- a sales order, goods receipt, invoice, or approval request is better represented as a document

## Product Philosophy

Orbyte is designed around these principles:

- configuration over hard-coding
- modules over forks
- contracts over implicit behavior
- policy over ad hoc conditionals
- auditability over hidden automation
- integration readiness over vendor lock-in

## Enterprise Design Principle

Orbyte should own:

- business state
- business rules
- identity and authorization decisions
- contracts and audit trails

External AI systems should own:

- reasoning
- conversation
- orchestration
- model selection

That separation keeps the platform durable and reduces lock-in.

## What Orbyte Is Not

Orbyte is not:

- a single-purpose ERP package
- a thin CRUD starter kit
- an agent framework
- a replacement for external LLM or agent orchestration platforms

It is the enterprise platform that those applications and integrations can build on.
