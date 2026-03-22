package contracts

import "testing"

func TestSchemaPathHelpersAndExists(t *testing.T) {
	if got := IntegrationSchemaPath("document.submit", 1); got != "integration/document.submit.v1.schema.json" {
		t.Fatalf("unexpected integration schema path: %q", got)
	}
	if got := EventSchemaPath("document.updated", "v1"); got != "events/document.updated.v1.schema.json" {
		t.Fatalf("unexpected event schema path: %q", got)
	}
	if !Exists(IntegrationSchemaPath("document.submit", 1)) {
		t.Fatal("expected embedded integration schema to exist")
	}
	if Exists("integration/does-not-exist.v1.schema.json") {
		t.Fatal("expected missing schema to report false")
	}
}

func TestValidateIntegrationContractRules(t *testing.T) {
	valid := map[string]any{
		"title": "document.submit.v1",
		"properties": map[string]any{
			"contract_key":     map[string]any{"const": "document.submit"},
			"contract_version": map[string]any{"const": 1},
		},
	}
	if err := ValidateIntegrationContract(IntegrationSchemaPath("document.submit", 1), "document.submit", 1, valid); err != nil {
		t.Fatalf("expected valid contract schema, got %v", err)
	}

	for _, tc := range []struct {
		name   string
		path   string
		schema map[string]any
	}{
		{
			name:   "path mismatch",
			path:   IntegrationSchemaPath("integration.submission", 1),
			schema: valid,
		},
		{
			name: "title mismatch",
			path: IntegrationSchemaPath("document.submit", 1),
			schema: map[string]any{
				"title": "integration.submission.v1",
			},
		},
		{
			name: "contract key mismatch",
			path: IntegrationSchemaPath("document.submit", 1),
			schema: map[string]any{
				"properties": map[string]any{
					"contract_key": map[string]any{"const": "integration.submission"},
				},
			},
		},
		{
			name: "contract version mismatch",
			path: IntegrationSchemaPath("document.submit", 1),
			schema: map[string]any{
				"properties": map[string]any{
					"contract_version": map[string]any{"const": 2},
				},
			},
		},
	} {
		if err := ValidateIntegrationContract(tc.path, "document.submit", 1, tc.schema); err == nil {
			t.Fatalf("expected %s to fail validation", tc.name)
		}
	}
}

func TestValidateObjectNestedIssues(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"id", "items"},
		"properties": map[string]any{
			"id":      map[string]any{"type": "string"},
			"enabled": map[string]any{"type": "boolean"},
			"kind":    map[string]any{"const": "demo"},
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"name"},
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
						"rank": map[string]any{"type": "integer"},
					},
				},
			},
		},
	}

	payload := map[string]any{
		"id":      42,
		"enabled": "yes",
		"kind":    "wrong",
		"items": []any{
			map[string]any{"rank": 1.5},
		},
	}

	issues := ValidateObject(schema, payload)
	if len(issues) < 4 {
		t.Fatalf("expected multiple validation issues, got %+v", issues)
	}
	seen := map[string]string{}
	for _, issue := range issues {
		seen[issue.Field] = issue.Code
	}
	for field := range map[string]struct{}{
		"id":            {},
		"enabled":       {},
		"kind":          {},
		"items[0].name": {},
		"items[0].rank": {},
	} {
		if _, ok := seen[field]; !ok {
			t.Fatalf("expected issue for %s, got %+v", field, issues)
		}
	}
}

func TestValidateEventSchemaHandlesValidMissingAndInvalidPayloads(t *testing.T) {
	valid, err := ValidateEventSchema("", "document.updated", "v1", map[string]any{
		"type":           "document.updated",
		"schema_version": "v1",
		"aggregate_id":   "doc-1",
		"occurred_at":    "2026-01-01T00:00:00Z",
		"payload": map[string]any{
			"document_id": "doc-1",
			"status":      "submitted",
		},
	})
	if err != nil {
		t.Fatalf("expected valid embedded schema to load: %v", err)
	}
	if len(valid) != 0 {
		t.Fatalf("expected valid event payload, got %+v", valid)
	}

	missing, err := ValidateEventSchema("events/does.not.exist.v1.schema.json", "missing.event", "v1", map[string]any{})
	if err != nil {
		t.Fatalf("expected missing schema to be ignored, got %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil issues for missing schema, got %+v", missing)
	}

	invalid, err := ValidateEventSchema("", "document.updated", "v1", map[string]any{
		"type":           "document.updated",
		"schema_version": "v1",
		"aggregate_id":   "doc-1",
		"occurred_at":    "2026-01-01T00:00:00Z",
		"payload": map[string]any{
			"document_id": 7,
		},
	})
	if err != nil {
		t.Fatalf("unexpected event validation error: %v", err)
	}
	if len(invalid) == 0 {
		t.Fatal("expected invalid event payload to report issues")
	}
}
