# External Contract Governance

## Contract Types

- HTTP platform APIs are published as versioned OpenAPI artifacts under `contracts/openapi/<version>/openapi.json`.
- MCP machine-facing capability catalogs are published as versioned artifacts under `contracts/mcp/<version>/catalog.json`.
- Externally relevant events and integration payloads are described by versioned JSON Schemas under the repository `contracts/` directory.

Development-only convenience routes still exist for local exploration:

- `GET /dev/openapi.json`
- `GET /dev/swagger`

These routes are not the canonical release contract. The canonical contract is the generated artifact committed under `contracts/`.

## Versioning Rules

- HTTP-compatible additive changes may ship in the same major API line.
- MCP-compatible additive changes may ship in the same major contract line.
- Event payloads must include a stable event type and schema version.
- Breaking changes require a new major contract version or a parallel versioned contract artifact.

## Compatibility Rules

- Consumers must tolerate unknown fields.
- Producers must not remove or change the meaning of existing required fields in the same major line.
- New required fields require a new major version.

## Contract Testing

- Generated OpenAPI and MCP artifacts are regenerated and compared by automated tests.
- Contract generation uses the same default profile module path as the server runtime, so profile-backed modules are included in published artifacts.
- JSON Schema artifacts are parsed and checked by automated tests.
- Contract changes should include sample payload updates and compatibility notes.

## Contract Registry

- `contracts/openapi/<version>/openapi.json` defines the published HTTP platform contract.
- `contracts/mcp/<version>/catalog.json` defines the published MCP contract catalog.
- `contracts/openapi/latest/openapi.json` and `contracts/mcp/latest/catalog.json` provide moving aliases for current development.
- `contracts/events/*.schema.json` defines event contracts.
- `contracts/integration/*.schema.json` defines integration submission contracts.
- Artifact names follow `<contract>.<version>.schema.json`.

## Generation Workflow

Generate release artifacts with:

```bash
go run ./cmd/contractsgen
```

Or via the repo workflow:

```bash
make contracts
```

The generator boots the platform in deterministic local mode, resolves the active business profile, and emits versioned OpenAPI and MCP artifacts from the same manifest set that the default server runtime uses.
