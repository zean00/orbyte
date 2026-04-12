# Configuration

<p class="page-intro">
This guide describes the current runtime configuration model, built-in configuration keys, scope behavior, and the environment variables that matter in local and deployed installs.
</p>

## What To Read First

<div class="quick-links" markdown>

- [**Built-In Config Areas**](#built-in-configuration-areas)
  See the important top-level keys currently shipped by the platform.
- [**MCP Runtime**](#platformmcp-in-detail)
  Review exposure modes, discovery settings, and governance controls.
- [**Environment Variables**](#environment-variables)
  See which process-level settings matter at startup.
- [**Operational Notes**](#operational-notes)
  Understand the practical deployment and local-dev implications.

</div>

## Configuration Model

Orbyte configuration is defined in code and stored as runtime entries.

The current model has three parts:

- configuration definitions
  - declared in `internal/platform/config/service.go`
  - describe key, category, scope, fields, defaults, and sensitivity
- configuration entries
  - stored by the config repository
  - in memory or PostgreSQL depending on runtime mode
- effective values
  - resolved values after scope inheritance

## Configuration Scopes

Current built-in scope levels:

- `deployment`
- `organization`
- `location`

Typical resolution order:

1. location override
2. organization override
3. deployment entry
4. built-in default

> Practical rule: keep infrastructure defaults at `deployment`, business policy at `organization`, and branch/site variation at `location`.

## Built-In Configuration Areas

The current built-in definitions include:

| Key | Purpose | Typical scope |
| --- | --- | --- |
| `platform.http` | HTTP bind address | deployment |
| `platform.acp` | ACP provider configuration | deployment |
| `platform.mcp` | MCP runtime, exposure, discovery, governance, skills | deployment |
| `platform.db` | DB instrumentation and read strategy settings | deployment |
| `search.typesense` | Typesense connection | deployment |
| `search.embedding` | embedding provider defaults | deployment |
| `eventing.nats` | outbound event sink settings | deployment |
| `identity.auth` | password, sessions, TOTP, Google login policy | deployment |

## Environment Variables

These process-level variables are currently meaningful:

| Variable | Purpose |
| --- | --- |
| `APP_ADDRESS` | server bind address |
| `APP_ENV` | runtime environment hint |
| `APP_JWT_SECRET` | JWT signing secret, required for startup |
| `APP_BOOTSTRAP_ADMIN_PASSWORD` | bootstrap admin password |
| `APP_AUTH_DEV_MODE` | enables local loopback/dev auth behavior |
| `APP_DOMAIN_PROFILE` | selects business manifest profile |
| `DATABASE_URL` | PostgreSQL connection string |
| `SMTP_*` | report/email delivery |
| `OBJECT_STORE_*` | object-store delivery |

## `platform.mcp` In Detail

`platform.mcp` is the main current machine-access configuration area.

Current default fields:

| Field | Current meaning |
| --- | --- |
| `enabled` | turns MCP runtime on or off |
| `exposure_mode` | `full`, `compact`, or `minimal` |
| `discovery_mode` | `keyword`, `vector`, or `hybrid` |
| `tool_discovery_mode` | optional override for tool discovery |
| `playbook_discovery_mode` | optional override for skill/playbook discovery |
| `discovery_indexing_enabled` | enables indexed discovery refresh/use |
| `governance_enabled` | turns MCP governance checks on |
| `default_action_mode` | default action posture such as `draft_only` |
| `tool_states_json` | explicit per-tool state overrides |
| `blocked_action_classes_json` | blocked action classes |
| `blocked_tool_keys_json` | blocked tool names |
| `blocked_document_types_json` | blocked document types |
| `allowed_submit_document_types_json` | submit allowlist |
| `domain_policy_overrides_json` | domain-level governance overrides |
| `playbooks_json` | skill/playbook definitions |
| `default_capabilities_json` | default capability shaping |

### Exposure modes

- `full`
  - exposes full MCP business/direct tool inventory
  - does not expose the minimal meta-tool surface
- `compact`
  - shaped direct catalog for compact discovery
  - also hides minimal meta-tools from the primary exposed catalog
- `minimal`
  - exposes only the minimal meta-tool surface
  - current primary minimal tools:
    - `skills.find`
    - `skills.describe`
    - `tools.find`
    - `tools.describe`
    - `tools.call`

<div class="orbyte-note">
`minimal` is the current discovery-first agent mode. `full` remains the broad direct-tool mode.
</div>

### Discovery modes

- `keyword`
  - lexical/in-memory fallback and deterministic matching
- `vector`
  - vector-backed discovery when a semantic embedder is actually configured
- `hybrid`
  - combined structured/lexical/vector path

If a semantic embedder is not configured, the current runtime downgrades vector-style discovery to keyword behavior instead of pretending hash embeddings are semantic.

## `platform.acp` In Detail

`platform.acp` controls agent-provider session integration.

Current fields:

- `enabled`
- `providers_json`

Each provider entry describes the ACP command/runtime used for shell-native sessions. In current workflows, OpenCode is the main validation provider, but ACP is provider-shaped rather than hard-coded to one agent runtime.

## `identity.auth` In Detail

The auth policy currently includes:

- password policy
- session TTL and idle timeout
- refresh window
- login rate limits
- trusted origins
- password/TOTP toggles
- Google sign-in and auto-provisioning settings
- login UI labels

This is the main place to configure:

- password login behavior
- Google login
- auto-provision defaults
- TOTP enforcement

## `platform.db`, `search.typesense`, `search.embedding`, and `eventing.nats`

### `platform.db`

Controls:

- slow query threshold
- top operation limits
- slow query limits
- named read strategy JSON

### `search.typesense`

Controls:

- whether Typesense is enabled
- endpoint
- API key
- timeout

### `search.embedding`

Controls:

- embedding provider
- embedding dimensions

### `eventing.nats`

Controls:

- whether NATS publication is enabled
- broker URL
- sink name
- subject prefix
- timeout

## Example Local `.env` Shape

```bash
APP_ADDRESS=127.0.0.1:18110
APP_ENV=development
APP_JWT_SECRET=dev-secret
APP_BOOTSTRAP_ADMIN_PASSWORD=admin123!
APP_AUTH_DEV_MODE=true
DATABASE_URL=postgres://orbyte:orbyte@127.0.0.1:55432/orbyte?sslmode=disable
```

## Example MCP Runtime Entry

```json
{
  "enabled": true,
  "exposure_mode": "minimal",
  "discovery_mode": "keyword",
  "discovery_indexing_enabled": true,
  "governance_enabled": true,
  "default_action_mode": "draft_only",
  "playbooks_json": "[]"
}
```

## Quick Reference

| If you need to change... | Look at |
| --- | --- |
| agent/runtime providers | `platform.acp` |
| MCP exposure and discovery | `platform.mcp` |
| auth and Google login | `identity.auth` |
| search backend | `search.typesense` |
| embeddings | `search.embedding` |
| NATS publication | `eventing.nats` |

## Operational Notes

- `APP_JWT_SECRET` is required for startup.
- In-memory development usually pairs with `APP_AUTH_DEV_MODE=true`.
- PostgreSQL mode is required for the more realistic demo and validation flows.
- MCP and ACP are runtime-configurable rather than compile-time switches.
- Sensitive config fields are declared as sensitive in definitions and should not be treated as ordinary text settings.

## Where Configuration Is Used

Configuration currently affects:

- HTTP bind behavior
- auth and session policy
- ACP provider availability
- MCP exposure/discovery/governance
- search and embedding behavior
- event publication
- DB instrumentation

## Related Guides

- [Getting Started](./getting-started.md)
- [Architecture](./architecture.md)
- [Agent Integration](./agent-integration.md)
- [Operations](./operations.md)

## Recommended Next Pages

<div class="next-steps" markdown>

- [Agent Integration](./agent-integration.md) for MCP and ACP runtime behavior
- [Architecture](./architecture.md) for where config is consumed in the service graph

</div>
