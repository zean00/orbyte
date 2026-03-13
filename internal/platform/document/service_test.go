package document

import (
	"testing"

	"orbyte/internal/platform/shared"
)

func TestCreateDocument(t *testing.T) {
	svc := NewService()
	record, err := svc.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "hello"})
	if err != nil {
		t.Fatalf("expected create to succeed, got error: %v", err)
	}
	if record.Header.Type != "generic_request" {
		t.Fatalf("unexpected document type: %s", record.Header.Type)
	}
	if record.Body.SchemaVersion != "v1" {
		t.Fatalf("unexpected schema version: %s", record.Body.SchemaVersion)
	}
}

func TestDefinitionLookup(t *testing.T) {
	svc := NewService()
	def, err := svc.Definition("generic_request")
	if err != nil {
		t.Fatalf("expected definition lookup to succeed, got error: %v", err)
	}
	if def.WorkflowKey != "generic_request_flow" {
		t.Fatalf("unexpected workflow key: %s", def.WorkflowKey)
	}
}

func TestReplaceLinesAndLoadRecord(t *testing.T) {
	svc := NewService()
	record, err := svc.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "hello"})
	if err != nil {
		t.Fatalf("expected create to succeed, got error: %v", err)
	}
	err = svc.ReplaceLines(record.Header.ID, []Line{
		{LineType: "service", Payload: map[string]any{"code": "CONSULT"}, Amount: shared.Money{AmountMinor: 10000, Currency: "IDR"}},
		{LineType: "service", Payload: map[string]any{"code": "ADMIN"}, Amount: shared.Money{AmountMinor: 5000, Currency: "IDR"}},
	})
	if err != nil {
		t.Fatalf("replace lines failed: %v", err)
	}
	loaded, err := svc.Get(record.Header.ID)
	if err != nil {
		t.Fatalf("expected record lookup: %v", err)
	}
	if len(loaded.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(loaded.Lines))
	}
	if loaded.Lines[0].LineNo != 1 || loaded.Lines[1].LineNo != 2 {
		t.Fatal("expected sequential line numbers")
	}
}

func TestAddLinkAndAttachment(t *testing.T) {
	svc := NewService()
	record, err := svc.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "hello"})
	if err != nil {
		t.Fatalf("expected create to succeed, got error: %v", err)
	}
	related, err := svc.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "related"})
	if err != nil {
		t.Fatalf("expected related create to succeed, got error: %v", err)
	}
	link, err := svc.AddLink(record.Header.ID, related.Header.ID, "related_to", map[string]any{"source": "test"})
	if err != nil {
		t.Fatalf("add link failed: %v", err)
	}
	if link.LinkType != "related_to" {
		t.Fatalf("unexpected link type: %s", link.LinkType)
	}
	attachment, err := svc.AddAttachment(record.Header.ID, "document", "x.pdf", "application/pdf", "object://doc/x.pdf", 42)
	if err != nil {
		t.Fatalf("add attachment failed: %v", err)
	}
	if attachment.AttachmentType != "document" {
		t.Fatalf("unexpected attachment type: %s", attachment.AttachmentType)
	}
	loaded, err := svc.Get(record.Header.ID)
	if err != nil {
		t.Fatalf("expected record lookup: %v", err)
	}
	if len(loaded.Links) != 1 || len(loaded.Attachments) != 1 {
		t.Fatal("expected persisted links and attachments")
	}
}

func TestAddLinkRejectsDisallowedType(t *testing.T) {
	svc := NewService()
	record, _ := svc.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "hello"})
	related, _ := svc.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "related"})
	if _, err := svc.AddLink(record.Header.ID, related.Header.ID, "forbidden", nil); err == nil {
		t.Fatal("expected disallowed link type error")
	}
}

func TestRegisterExtensionAndRenderViews(t *testing.T) {
	svc := NewService()
	if err := svc.RegisterExtension(ExtensionDefinition{DocumentType: "generic_request", ModuleKey: "analytics", DisplayName: "Analytics", SchemaVersion: "v1"}); err != nil {
		t.Fatalf("register extension failed: %v", err)
	}
	record, err := svc.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "hello"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	record, err = svc.ReplaceExtension(record.Header.ID, "analytics", map[string]any{"score": 9})
	if err != nil {
		t.Fatalf("replace extension failed: %v", err)
	}
	if got := ExtensionPayload(record.Body.Payload, "analytics"); got["score"] != 9 {
		t.Fatalf("expected stored extension payload, got %+v", got)
	}
	normal := svc.Render(record, ViewNormal, map[string]bool{"analytics": true})
	if _, ok := normal.Body.Payload["extensions"]; ok {
		t.Fatal("expected normal view to hide extensions")
	}
	expanded := svc.Render(record, ViewExpanded, map[string]bool{"analytics": true})
	if got := ExtensionPayload(expanded.Body.Payload, "analytics"); got["score"] != 9 {
		t.Fatalf("expected expanded view to expose enabled extension, got %+v", got)
	}
	hidden := svc.Render(record, ViewExpanded, map[string]bool{"analytics": false})
	if got := ExtensionPayload(hidden.Body.Payload, "analytics"); got != nil {
		t.Fatalf("expected disabled extension to be hidden, got %+v", got)
	}
	raw := svc.Render(record, ViewRaw, map[string]bool{"analytics": false})
	if got := ExtensionPayload(raw.Body.Payload, "analytics"); got["score"] != 9 {
		t.Fatalf("expected raw view to expose hidden extension, got %+v", got)
	}
}
