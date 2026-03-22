# Release and Compatibility Policy

## Kernel Versioning

- The kernel follows semantic versioning.
- `internal/platform/module.KernelVersion` is the compatibility anchor for module manifests.
- Breaking manifest/runtime changes require a major kernel release.

## Module Author Guarantees

- Module manifests may declare `kernel_version_range`.
- Within the same major kernel line:
  - additive manifest fields are allowed
  - existing manifest semantics should remain backward compatible
  - removal or semantic redefinition of manifest fields requires deprecation first
- Required kernel capabilities must remain stable for the supported major line.

## Runtime and Migration Policy

- Minor releases may add new endpoints, manifest fields, config definitions, schema columns, and event fields in backward-compatible form.
- Major releases may remove deprecated contracts, routes, or manifest fields.
- Migrations must be ordered, repeatable, and bundled with the release that requires them.

## Deprecation Policy

- Public runtime or contract changes should be documented before removal.
- New contract versions should be introduced in parallel before retiring an older major version.

## Delivery Gate

Every release candidate should pass:

1. lint
2. tests
3. migration smoke test
4. contract checks
