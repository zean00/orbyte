# Module SDK Examples

These starter specs exercise the supported `modulegen` starter packs:

- `document-workflow.yaml`
- `masterdata-search.yaml`
- `integration-adapter.yaml`

Typical flow:

```bash
go run ./cmd/modulegen module explain --spec examples/modules/document-workflow.yaml
go run ./cmd/modulegen module plan --spec examples/modules/document-workflow.yaml
go run ./cmd/modulegen module lint --profile all
```
