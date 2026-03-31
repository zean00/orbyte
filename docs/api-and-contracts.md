# API and Contracts

This guide describes the public and semi-public machine-facing surfaces of Orbyte.

## HTTP APIs

Orbyte exposes generic platform APIs for:

- authentication
- platform context
- models
- documents
- search
- offline support
- admin operations
- UI contracts

In development mode, OpenAPI is available at:

- `GET /dev/openapi.json`
- `GET /dev/swagger`

For product and release use, the canonical published contracts are the generated artifacts under:

- `contracts/openapi/<version>/openapi.json`
- `contracts/mcp/<version>/catalog.json`

## API Style

The current platform surface is intentionally generic and manifest-driven.

This means:

- modules register capabilities into the platform
- the platform exposes them through generic routes
- clients can discover supported model keys, document types, views, and indexes at runtime

## Important HTTP Areas

- `/auth/*`
- `/platform/context`
- `/models/*`
- `/documents/*`
- `/search/*`
- `/offline/*`
- `/ui/*`
- `/admin/api/*`
- `/mcp`

## Example HTTP Sessions

### Check Runtime Health

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

### Get Platform Context

```bash
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/platform/context
```

### Create A Generic Model Record

```bash
curl -X POST http://localhost:8080/models/party \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "values": {
      "name": "ACME Trading",
      "status": "active"
    }
  }'
```

### Submit A Document Action

```bash
curl -X POST http://localhost:8080/documents/<document-id>/actions \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "submit"
  }'
```

## MCP Surface

Orbyte also exposes MCP endpoints for machine-oriented clients.

For the normative target-state role of MCP as Orbyte's canonical business interface for external agents, see [MCP Target Architecture](./mcp-target-architecture.md).

MCP is useful for:

- external AI agents
- admin copilots
- automation clients
- governed operator tooling

The MCP surface can expose:

- tools
- resources
- apps
- streaming updates

The target direction is for MCP to cover:

- business discovery
- operational read access
- analytical business comprehension
- governed draft-first action
- control-plane and governance metadata

Examples of capability areas already represented include:

- templates
- analytics
- workflows
- configuration
- feature flags
- identity and permissions
- modules
- search runtime
- offline sync
- policy hooks
- reference data
- integrations

## Example MCP Sessions

### List Available Tools

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {}
}
```

### Call A Tool

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "integration.adapter.list",
    "arguments": {}
  }
}
```

### Read A Resource

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "resources/read",
  "params": {
    "uri": "orbyte://control-plane/readiness"
  }
}
```

## Recommended API Consumer Strategy

- discover available capabilities at runtime
- rely on versioned generated contracts for external integrations
- use idempotency for non-trivial writes
- keep business meaning at the contract level, not in ad hoc client conventions
- align machine integrations with the MCP target architecture rather than screen-level conventions

## Versioned Contracts

External contracts should be treated as versioned assets.

Current repository contract sources:

- `contracts/openapi/<version>/openapi.json`
- `contracts/mcp/<version>/catalog.json`
- `contracts/events/*.schema.json`
- `contracts/integration/*.schema.json`

Generate them with:

```bash
go run ./cmd/contractsgen
```

The contract generator uses the same profile-aware module bootstrap path as the server. That means the published OpenAPI and MCP artifacts include built-in kernel packs plus the business manifests selected by the active domain profile.

### Contract Source of Truth

- generated files in `contracts/` are the release artifacts
- `/dev/openapi.json` and `/dev/swagger` are development-only inspection surfaces
- the runtime MCP endpoint is discoverable live, but `contracts/mcp/<version>/catalog.json` is the versioned published catalog

## Compatibility Expectations

Consumers should:

- tolerate unknown fields
- avoid depending on unstable internal details
- prefer documented versioned contracts

Producers should:

- avoid breaking existing payload semantics in place
- publish new major versions when required
- document compatibility expectations

## Product Documentation Recommendation

As Orbyte matures as a product, publish:

- a stable public API reference
- authentication examples for user and service principal flows
- idempotent write examples
- event payload examples
- MCP tool catalog reference
- integration webhook or adapter examples

## Related Guides

- [Integration](./integration.md)
- [External Contract Governance](./contracts.md)
- [Release and Compatibility Policy](./release-policy.md)
