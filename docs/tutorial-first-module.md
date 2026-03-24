# Tutorial: Build Your First Module

This tutorial walks through a simple first module so a new team can understand the Orbyte extension model in practice.

## Goal

Create a small business module called `inventory` that demonstrates:

- a module manifest
- one model
- optional search and UI scaffolding
- startup registration

## Outcome

By the end of the tutorial you will have:

- generated a module skeleton
- reviewed the generated files
- understood where to add business behavior

## Step 1: Choose A Module Shape

For a first tutorial, use a `model` or `hybrid` module.

- `model`
  - good for master-data-first domains
- `hybrid`
  - good when the domain needs both structured records and document-style transactions

For this example, use `hybrid`.

## Step 2: Generate The Module

```bash
go run ./cmd/modulegen module init \
  --profile backoffice \
  --key inventory \
  --name "Inventory" \
  --kind hybrid \
  --domain-family supply_chain
```

## Step 3: Review The Generated Files

The generated module will typically create files under:

- `internal/modules/inventory/manifest.go`
- `internal/modules/inventory/service.go`
- `internal/modules/inventory/bundle.js`

Depending on options, it may also generate reporting, observability, and test helpers.

## Step 4: Understand What Was Registered

The generator updates the module registry so the module can be included at startup.

At this point, the platform can boot with the new module available to the manifest-driven runtime.

## Step 5: Add Business Definitions

Typical next edits:

- define model fields
- define document types if needed
- define workflow definitions if approvals are required
- add permissions and role templates
- add a search index if lookup matters
- add reporting datasets if operational analytics matter

## Step 6: Decide What The Kernel Owns

For an inventory domain, good module-owned concerns might include:

- items
- stock movements
- storage locations
- cycle count requests
- adjustment approvals

Kernel-owned concerns should remain in the shared platform:

- identity
- config resolution
- service principal auth
- audit
- integration machinery
- policy runtime

## Step 7: Test The Module

Run:

```bash
make test
make lint
```

Then validate that:

- the app starts
- the module appears in runtime metadata
- the model or document definitions are available
- the generated routes and views resolve as expected

## Step 8: Expand The Module Deliberately

Do not add everything at once.

Recommended progression:

1. get the core model right
2. add permissions
3. add workflow where needed
4. add search and reporting
5. expose MCP tools only after the business action is stable
6. add integrations last

## Example Next Iteration

After the initial skeleton, an inventory module could evolve into:

- model: `item`
- model: `warehouse_bin`
- document: `stock_adjustment`
- workflow: `stock_adjustment_approval`
- search index: `inventory_items`
- report dataset: `stock_movements`
- integration endpoint: ERP or WMS synchronization

## Common Mistakes

- making the first module too large
- mixing unrelated domains into one module
- exposing write tools before permissions and policy are complete
- skipping tests around workflow and integration behavior

## Related Guides

- [Module System](./module-system.md)
- [Module Generator](./modulegen.md)
- [Architecture](./architecture.md)
