# Sample Payloads

This guide provides representative payloads for product documentation, demos, and implementation planning.

## Model Record Create

```json
{
  "values": {
    "name": "ACME Trading",
    "status": "active",
    "party_type": "supplier"
  }
}
```

## Document Create

```json
{
  "document_type": "purchase_request",
  "organization_id": "org_demo",
  "location_id": "loc_hq",
  "payload": {
    "requestor_id": "user_demo",
    "department": "procurement",
    "currency": "IDR",
    "items": [
      {
        "sku": "ITM-001",
        "description": "Barcode Scanner",
        "qty": 4,
        "uom": "unit"
      }
    ]
  }
}
```

## Document Action

```json
{
  "action": "submit",
  "comment": "Ready for approval"
}
```

## MCP Tool Call

```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "method": "tools/call",
  "params": {
    "name": "workflow.runtime.history.get",
    "arguments": {
      "target_type": "document",
      "target_id": "doc:purchase_request:20260324:001"
    }
  }
}
```

## Integration Submission

```json
{
  "external_system_key": "http_bridge",
  "contract_key": "document.submit",
  "contract_version": 1,
  "operation_type": "purchase_request.submit",
  "idempotency_key": "purchase-request:doc-001:v1",
  "payload": {
    "document_id": "doc-001",
    "document_type": "purchase_request",
    "status": "submitted"
  }
}
```

## Configuration Entry

```json
{
  "key": "identity.auth",
  "scope": "deployment",
  "scope_id": "",
  "value": {
    "password_enabled": true,
    "google_enabled": false,
    "session_ttl_minutes": 480
  }
}
```

## Notes

These payloads are representative examples for documentation and planning. Product deployments should version and validate any externally relied-on payload against the platform contract model.
