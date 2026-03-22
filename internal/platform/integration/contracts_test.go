package integration

import (
	"testing"

	contractfs "orbyte/contracts"
)

func TestContractSchemasParseAndDeclareVersionedMetadata(t *testing.T) {
	paths := []string{
		contractfs.EventSchemaPath("document.updated", "v1"),
		contractfs.EventSchemaPath("analytics.snapshot.captured", "v1"),
		contractfs.IntegrationSchemaPath("integration.submission", 1),
		contractfs.IntegrationSchemaPath("document.submit", 1),
	}

	for _, path := range paths {
		schema, err := contractfs.Load(path)
		if err != nil {
			t.Fatalf("load schema %s: %v", path, err)
		}
		if schema["$schema"] == "" {
			t.Fatalf("expected $schema in %s", path)
		}
		if schema["title"] == "" {
			t.Fatalf("expected title in %s", path)
		}
		if schema["type"] != "object" {
			t.Fatalf("expected object schema in %s, got %#v", path, schema["type"])
		}
		properties, _ := schema["properties"].(map[string]any)
		if len(properties) == 0 {
			t.Fatalf("expected properties in %s", path)
		}
	}
}

func TestRegisterContractLoadsEmbeddedSchemaArtifact(t *testing.T) {
	svc := NewService(nil, nil)
	if err := svc.RegisterContract(Contract{
		Key:       "integration.submission",
		Name:      "Integration Submission",
		Version:   1,
		Direction: "outbound",
		Intent:    "command",
		Status:    "active",
		SchemaRef: contractfs.IntegrationSchemaPath("integration.submission", 1),
	}); err != nil {
		t.Fatalf("register contract failed: %v", err)
	}
	contract, ok := svc.repo.GetContract("integration.submission", 1)
	if !ok {
		t.Fatal("expected stored contract")
	}
	if contract.SchemaRef != contractfs.IntegrationSchemaPath("integration.submission", 1) {
		t.Fatalf("expected schema ref to be stored, got %+v", contract)
	}
	if contract.Schema["title"] == "" {
		t.Fatalf("expected embedded schema to load, got %+v", contract.Schema)
	}
}

func TestRegisterContractRejectsMissingSchema(t *testing.T) {
	svc := NewService(nil, nil)
	if err := svc.RegisterContract(Contract{
		Key:       "missing.schema",
		Name:      "Missing Schema",
		Version:   1,
		Direction: "outbound",
		Intent:    "command",
		Status:    "active",
	}); err == nil {
		t.Fatal("expected missing schema contract to be rejected")
	}
}

func TestRegisterContractRejectsMismatchedEmbeddedSchemaRef(t *testing.T) {
	svc := NewService(nil, nil)
	if err := svc.RegisterContract(Contract{
		Key:       "document.submit",
		Name:      "Document Submit",
		Version:   1,
		Direction: "outbound",
		Intent:    "command",
		Status:    "active",
		SchemaRef: contractfs.IntegrationSchemaPath("integration.submission", 1),
	}); err == nil {
		t.Fatal("expected mismatched schema_ref contract to be rejected")
	}
}
