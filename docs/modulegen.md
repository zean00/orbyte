# Module Generator

`modulegen` scaffolds startup-registered modules for this repo.

## Commands

```bash
go run ./cmd/modulegen module validate --spec examples/modulegen/hybrid.yaml
go run ./cmd/modulegen module explain --profile backoffice --key crm --name "CRM" --kind hybrid
go run ./cmd/modulegen module plan --spec examples/modulegen/hybrid.yaml
go run ./cmd/modulegen module init --spec examples/modulegen/hybrid.yaml
go run ./cmd/modulegen module init --profile backoffice --key crm --name "CRM" --kind hybrid
```

Flags override YAML values:

```bash
go run ./cmd/modulegen module init \
  --key inventory \
  --name "Inventory" \
  --kind hybrid \
  --domain-family supply_chain \
  --with-search=true \
  --with-reporting=true \
  --with-custom-ui=true \
  --with-observability-helper=true \
  --with-reporting-helper=true \
  --with-manifest-test=true \
  --with-registration-test=false
```

## Profiles

Profiles are CLI presets that prefill feature, scaffold, and manifest stub options before any explicit flags are applied:

- `minimal`
  - small skeleton
  - tests on
  - UI/search/policy/observability/reporting off by default
- `backoffice`
  - generic UI, search, policy, observability, reporting on
  - custom UI off by default
- `search-heavy`
  - like `backoffice`, optimized for search/reporting-oriented modules
- `integration-first`
  - defaults to `integration` kind when one is not explicitly set
  - UI/policy/observability/reporting on
  - search and custom UI off by default

Example:

```bash
go run ./cmd/modulegen module init \
  --profile minimal \
  --kind model \
  --key reference_data \
  --name "Reference Data" \
  --with-ui=true \
  --with-generic-ui-stub=true
```

## Explain

Use `explain` to inspect the fully resolved spec before generating files. This prints the final YAML after:

1. kind defaults
2. profile preset
3. YAML spec values
4. explicit CLI overrides

Example:

```bash
go run ./cmd/modulegen module explain \
  --profile search-heavy \
  --kind hybrid \
  --key catalog \
  --name "Catalog"
```

## Output

The generator writes:

- `internal/modules/<module_key>/manifest.go`
- `internal/modules/<module_key>/service.go`
- `internal/modules/<module_key>/bundle.js`

Depending on the resolved feature and scaffold settings it can also write:

- `internal/modules/<module_key>/observability.go`
- `internal/modules/<module_key>/reporting.go`
- `internal/modules/<module_key>/service_test.go`
- `internal/modules/<module_key>/registration_test.go`

It also patches `internal/modules/registry.go` so the generated module is included in bootstrap.

## Configurable Scaffolds

Feature toggles control whether the module manifest includes search, UI, policy, observability, and reporting contracts.

Scaffold toggles control whether separate helper and test files are emitted:

```yaml
scaffold:
  observability_helper: true
  reporting_helper: true
  manifest_test: true
  registration_test: true

manifest:
  model_stub: true
  dataset_stub: true
  search_index_stub: true
  role_template_stub: true
  policy_hook_stub: true
  observability_stub: true
  generic_ui_stub: true
  custom_ui_stub: true
```

Default precedence is:

- kind defaults
- YAML spec
- CLI flags

Scaffold defaults follow the resolved features:

- `observability_helper` defaults to `features.observability`
- `reporting_helper` defaults to `features.reporting`
- both test files default to `features.tests`

The same scaffold toggles can be overridden from the CLI:

- `--with-observability-helper=true|false`
- `--with-reporting-helper=true|false`
- `--with-manifest-test=true|false`
- `--with-registration-test=true|false`

The generated manifest sections can also be controlled independently:

- `--with-model-stub=true|false`
- `--with-dataset-stub=true|false`
- `--with-search-index-stub=true|false`
- `--with-role-template-stub=true|false`
- `--with-policy-hook-stub=true|false`
- `--with-observability-stub=true|false`
- `--with-generic-ui-stub=true|false`
- `--with-custom-ui-stub=true|false`

## Kinds

- `document`: document-centric module skeleton
- `model`: model-centric module skeleton
- `hybrid`: model + document skeleton
- `integration`: integration-oriented module skeleton

## Notes

- Generic UI scaffolds are generated through the manifest by default.
- The custom bundle stub is generated when `custom_ui` is enabled.
- Reporting and observability helpers are thin wrappers around the generated manifest contracts, so they stay compile-safe and easy to customize.
- Feature flags control coarse capability defaults; `scaffold.*` and `manifest.*` let you trim or expand the actual generated skeleton per module.
