# Glossary

This glossary defines the main terms used across the Orbyte documentation.

## ACP

Agent Connectivity Protocol runtime bridge. In Orbyte, ACP is the provider and session layer used to connect external agent runtimes to the platform. It handles provider configuration, session lifecycle, prompt transport, approvals, and streamed interaction. ACP is not the business capability model.

## Actor

The effective identity performing an operation. An actor can be a user, service principal, or delegated machine context.

## Adapter

A runtime component that knows how to communicate with an external system through a specific protocol or connector shape.

## Audit Event

A record of who performed an action, on which target, and when.

## Contract

A versioned definition of an external payload, event, or machine-facing capability.

## Control Plane

Administrative and governance-facing capabilities used to inspect or change runtime behavior.

## Dead Letter

A failed integration submission that could not be completed successfully and requires operator review or replay.

## Deployment Scope

The broadest built-in configuration scope, typically used for infrastructure-wide defaults.

## Document

A transactional business record with lifecycle, versioning, and optional workflow behavior.

## Effective Value

A runtime-resolved configuration value after scope inheritance and fallback are applied.

## Endpoint

A configured destination or connection point for an external system integration.

## Eventing

The platform capability for recording, publishing, and reacting to domain events and related asynchronous runtime activity.

## Feature Flag

A runtime toggle used to selectively enable or change behavior.

## Kernel

The reusable core runtime of Orbyte, independent from any one business application.

## Location Scope

A site-level or branch-level context used for runtime configuration, policy, and operational behavior.

## Manifest

The structured module definition that declares what a module contributes to the platform.

## MCP

Model Context Protocol. In Orbyte, MCP is the canonical machine-facing business contract used to expose governed business discovery, analysis, and action capability to external agents and copilots. It should express business semantics, not frontend mechanics.

## Model

A metadata-driven record type used for master data or generic structured entities.

## Module

A package of business capability that extends the kernel through manifest-declared assets such as models, documents, workflows, UI, search, policy, and integrations.

## Organization Scope

An enterprise or business-unit level context used for runtime configuration and policy resolution.

## Outbox

A durable record of follow-up events or actions that must be processed asynchronously after a successful write transaction.

## Policy Hook

A configurable governance point where runtime decisions can be evaluated using built-in logic or Rego modules.

## Service Principal

A non-human identity used by external services, integrations, or AI runtimes to access the platform programmatically.

## Submission

An integration attempt or request sent from Orbyte toward an external system through a contract and adapter.

## Tool

A machine-invokable action exposed through MCP or another governed runtime surface.

## Workflow

A versioned definition of routing, approval, and transition behavior for business processes.
