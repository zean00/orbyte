# External Contract Governance

## Contract Types

- HTTP platform APIs are described by the development OpenAPI document served at `/dev/openapi.json`.
- Externally relevant events and integration payloads are described by versioned JSON Schemas under [`contracts/`](../contracts).

## Versioning Rules

- HTTP-compatible additive changes may ship in the same major API line.
- Event payloads must include a stable event type and schema version.
- Breaking changes require a new major contract version or a parallel versioned contract artifact.

## Compatibility Rules

- Consumers must tolerate unknown fields.
- Producers must not remove or change the meaning of existing required fields in the same major line.
- New required fields require a new major version.

## Contract Testing

- OpenAPI runtime metadata is verified by automated tests.
- JSON Schema artifacts are parsed and checked by automated tests.
- Contract changes should include sample payload updates and compatibility notes.

## Contract Registry

- `contracts/events/*.schema.json` defines event contracts.
- `contracts/integration/*.schema.json` defines integration submission contracts.
- Artifact names follow `<contract>.<version>.schema.json`.
