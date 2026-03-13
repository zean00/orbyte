package securityfields

import (
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/policy"
)

func TestModelProfileDefaultsAndSanitize(t *testing.T) {
	svc := NewService(nil)
	def := model.Definition{
		Key: "party",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string"},
			{Key: "email", Type: "string", Sensitive: true, DefaultMask: "partial_email"},
			{Key: "credit_limit", Type: "number", ReadPermissionKey: "party.credit.read", ExportVisible: boolPtr(true)},
		},
	}
	profile := svc.ModelProfile(AccessContext{
		ActorID: "u1",
		PermissionChecker: func(permissionKey string) bool {
			return permissionKey != "party.credit.read"
		},
	}, def)
	if profile.Fields["email"].ExportVisible {
		t.Fatal("expected sensitive email to be non-exportable by default")
	}
	if profile.Fields["credit_limit"].Visible {
		t.Fatal("expected credit_limit to be hidden without permission")
	}
	record := svc.SanitizeModelRecord(profile, model.Record{Values: map[string]any{
		"name":         "Alice",
		"email":        "alice@example.com",
		"credit_limit": 100,
	}})
	if record.Values["email"] == "alice@example.com" {
		t.Fatal("expected email to be masked")
	}
	if _, ok := record.Values["credit_limit"]; ok {
		t.Fatal("expected hidden field to be removed")
	}
}

func TestModelProfilePolicyOverride(t *testing.T) {
	policies := policy.NewService()
	if err := policies.Register(policy.HookDefinition{Key: "models.fields.profile", Kind: "security", Target: "model_fields", AllowedScopes: []string{"deployment"}}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	if err := policies.SetEvaluator("models.fields.profile", func(req policy.Request) policy.Decision {
		return policy.Decision{
			Allowed: true,
			Output: map[string]any{
				"fields": map[string]any{
					"email": map[string]any{"visible": true, "mask": "hide"},
				},
			},
		}
	}); err != nil {
		t.Fatalf("set evaluator failed: %v", err)
	}
	svc := NewService(policies)
	def := model.Definition{
		Key: "party",
		Fields: []model.FieldDefinition{
			{Key: "email", Type: "string", Sensitive: true},
		},
	}
	profile := svc.ModelProfile(AccessContext{ActorID: "u1"}, def)
	if !profile.Fields["email"].Visible || profile.Fields["email"].Mask != "hide" {
		t.Fatalf("expected policy override, got %+v", profile.Fields["email"])
	}
}

func TestValidateModelWrite(t *testing.T) {
	svc := NewService(nil)
	def := model.Definition{
		Key: "party",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string"},
			{Key: "secret_note", Type: "string", WritePermissionKey: "party.secret.write"},
		},
	}
	profile := svc.ModelProfile(AccessContext{
		ActorID: "u1",
		PermissionChecker: func(permissionKey string) bool {
			return false
		},
	}, def)
	if err := svc.ValidateModelWrite(profile, map[string]any{"secret_note": "blocked"}, def); err == nil {
		t.Fatal("expected write validation to fail")
	}
}

func TestSanitizeModelRecordsAndReadOnlyWriteValidation(t *testing.T) {
	svc := NewService(nil)
	def := model.Definition{
		Key: "party",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string"},
			{Key: "email", Type: "string", Sensitive: true, DefaultMask: "hide"},
			{Key: "system_id", Type: "string", ReadOnly: true},
		},
	}
	profile := svc.ModelProfile(AccessContext{ActorID: "u1", SessionID: "s1"}, def)
	records := svc.SanitizeModelRecords(profile, []model.Record{
		{Values: map[string]any{"name": "Alice", "email": "alice@example.com"}},
		{Values: map[string]any{"name": "Bob", "email": "bob@example.com"}},
	})
	if len(records) != 2 || records[0].Values["email"] != "[redacted]" || records[1].Values["email"] != "[redacted]" {
		t.Fatalf("expected batch sanitize to mask model records, got %+v", records)
	}
	if err := svc.ValidateModelWrite(profile, map[string]any{"system_id": "sys-1"}, def); err == nil {
		t.Fatal("expected read-only model field write to fail")
	}
}

func TestDocumentProfileSanitizeValidateAndCache(t *testing.T) {
	policies := policy.NewService()
	if err := policies.Register(policy.HookDefinition{
		Key:           "documents.fields.profile",
		Kind:          "security",
		Target:        "document_fields",
		AllowedScopes: []string{"deployment"},
		DefaultRule:   map[string]any{"fields": map[string]any{}},
	}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	evaluations := 0
	if err := policies.SetEvaluator("documents.fields.profile", func(req policy.Request) policy.Decision {
		evaluations++
		return policy.Decision{
			Allowed: true,
			Output: map[string]any{
				"fields": map[string]any{
					"patient_ssn":                     map[string]any{"visible": false, "editable": true, "export_visible": false},
					"extensions.analytics.secret":     map[string]any{"visible": false, "editable": true, "search_visible": false},
					"locked_internal_note":            map[string]any{"visible": true, "editable": false, "mask": "hide"},
					"extensions.analytics.locked_tag": map[string]any{"visible": true, "editable": false, "mask": "last4"},
				},
			},
		}
	}); err != nil {
		t.Fatalf("set evaluator failed: %v", err)
	}

	svc := NewService(policies)
	record := document.Record{
		Header: document.Header{
			ID:             "doc-1",
			Type:           "visit_note",
			Status:         "draft",
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
		},
		Body: document.Body{
			Payload: map[string]any{
				"title":                "Visible Title",
				"patient_ssn":          "999-11-2222",
				"locked_internal_note": "nurse-only",
				"extensions": map[string]any{
					"analytics": map[string]any{
						"secret":     "s3cr3t",
						"locked_tag": "ABCD1234",
						"score":      7,
					},
				},
			},
		},
	}

	ctx := AccessContext{
		ActorID:        "u1",
		SessionID:      "sess-1",
		OrganizationID: "org_default",
		LocationID:     "loc_hq",
		Channel:        "report",
		State:          "draft",
	}
	profile := svc.DocumentProfile(ctx, record)
	cached := svc.DocumentProfile(ctx, record)
	if evaluations != 1 {
		t.Fatalf("expected document profile to be cached, got %d evaluations", evaluations)
	}
	if len(profile.Fields) != len(cached.Fields) {
		t.Fatalf("expected cached document profile to match, got %+v vs %+v", profile, cached)
	}

	sanitized := svc.SanitizeDocumentPayload(profile, record.Body.Payload)
	if _, ok := sanitized["patient_ssn"]; ok {
		t.Fatalf("expected patient_ssn to be hidden, got %+v", sanitized)
	}
	if sanitized["locked_internal_note"] != "[redacted]" {
		t.Fatalf("expected locked_internal_note to be masked, got %+v", sanitized["locked_internal_note"])
	}
	extensions := sanitized["extensions"].(map[string]any)["analytics"].(map[string]any)
	if _, ok := extensions["secret"]; ok {
		t.Fatalf("expected hidden extension field to be removed, got %+v", extensions)
	}
	if extensions["locked_tag"] != "****1234" {
		t.Fatalf("expected locked_tag to be last4-masked, got %+v", extensions["locked_tag"])
	}
	if extensions["score"] != 7 {
		t.Fatalf("expected non-sensitive extension field to remain visible, got %+v", extensions)
	}

	if err := svc.ValidateDocumentWrite(profile, map[string]any{"locked_internal_note": "blocked"}, ""); err == nil {
		t.Fatal("expected protected document root field write to fail")
	}
	if err := svc.ValidateDocumentWrite(profile, map[string]any{"locked_tag": "blocked"}, "extensions.analytics"); err == nil {
		t.Fatal("expected protected document extension field write to fail")
	}
	if err := svc.ValidateDocumentWrite(profile, map[string]any{"score": 9}, "extensions.analytics"); err != nil {
		t.Fatalf("expected allowed extension field write, got %v", err)
	}
}

func TestDocumentProfileCacheKeyIncludesDocumentID(t *testing.T) {
	policies := policy.NewService()
	if err := policies.Register(policy.HookDefinition{
		Key:           "documents.fields.profile",
		Kind:          "security",
		Target:        "document_fields",
		AllowedScopes: []string{"deployment"},
		DefaultRule:   map[string]any{"fields": map[string]any{}},
	}); err != nil {
		t.Fatalf("register hook failed: %v", err)
	}
	if err := policies.SetEvaluator("documents.fields.profile", func(req policy.Request) policy.Decision {
		fields := map[string]any{}
		if req.Inputs["document_id"] == "doc-1" {
			fields["owner_only"] = map[string]any{"visible": false}
		}
		return policy.Decision{Allowed: true, Output: map[string]any{"fields": fields}}
	}); err != nil {
		t.Fatalf("set evaluator failed: %v", err)
	}
	svc := NewService(policies)
	ctx := AccessContext{
		ActorID:        "u1",
		SessionID:      "sess-1",
		OrganizationID: "org_default",
		LocationID:     "loc_hq",
		Channel:        "ui",
		State:          "draft",
	}
	recordA := document.Record{
		Header: document.Header{ID: "doc-1", Type: "visit_note", Status: "draft", OrganizationID: "org_default", LocationID: "loc_hq"},
		Body:   document.Body{Payload: map[string]any{"owner_only": "hidden on doc-1"}},
	}
	recordB := document.Record{
		Header: document.Header{ID: "doc-2", Type: "visit_note", Status: "draft", OrganizationID: "org_default", LocationID: "loc_hq"},
		Body:   document.Body{Payload: map[string]any{"owner_only": "visible on doc-2"}},
	}

	profileA := svc.DocumentProfile(ctx, recordA)
	profileB := svc.DocumentProfile(ctx, recordB)
	if access, ok := profileA.Fields["owner_only"]; !ok || access.Visible {
		t.Fatalf("expected doc-1 owner_only to be hidden, got %+v", profileA.Fields["owner_only"])
	}
	if access, ok := profileB.Fields["owner_only"]; ok && !access.Visible {
		t.Fatalf("expected doc-2 to keep owner_only visible, got %+v", access)
	}
	sanitizedA := svc.SanitizeDocumentPayload(profileA, recordA.Body.Payload)
	sanitizedB := svc.SanitizeDocumentPayload(profileB, recordB.Body.Payload)
	if _, ok := sanitizedA["owner_only"]; ok {
		t.Fatalf("expected doc-1 owner_only to be hidden, got %+v", sanitizedA)
	}
	if sanitizedB["owner_only"] != "visible on doc-2" {
		t.Fatalf("expected doc-2 owner_only to remain visible, got %+v", sanitizedB)
	}
}

func TestNilServiceAndMasks(t *testing.T) {
	var svc *Service
	def := model.Definition{Key: "party", Fields: []model.FieldDefinition{{Key: "email", Type: "string", Sensitive: true}}}
	modelProfile := svc.ModelProfile(AccessContext{}, def)
	if modelProfile.ResourceKey != "party" {
		t.Fatalf("expected nil service model profile fallback, got %+v", modelProfile)
	}
	record := document.Record{Header: document.Header{Type: "visit_note"}}
	documentProfile := svc.DocumentProfile(AccessContext{}, record)
	if documentProfile.ResourceKey != "visit_note" {
		t.Fatalf("expected nil service document profile fallback, got %+v", documentProfile)
	}

	if got := ApplyMask("last4", "123"); got != "123" {
		t.Fatalf("expected short last4 value to remain unchanged, got %v", got)
	}
	if got := ApplyMask("partial_email", "invalid"); got != "[redacted]" {
		t.Fatalf("expected invalid email to redact, got %v", got)
	}
	if got := ApplyMask("none", "keep"); got != "keep" {
		t.Fatalf("expected none mask to keep value, got %v", got)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
