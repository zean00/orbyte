package model

import "testing"

func TestCreateAndUpdateRecordWithRules(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields: []FieldDefinition{
			{Key: "name", Type: "string", Required: true},
			{Key: "status", Type: "string", DefaultRuleKey: "status.default", ConstraintRuleKeys: []string{"status.allowed"}},
			{Key: "display_name", Type: "string", ComputeRuleKey: "display.compute"},
		},
	}); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	svc.SetDefaultEvaluator("status.default", func(_ RuleInput) (any, error) { return "active", nil })
	svc.SetComputeEvaluator("display.compute", func(input RuleInput) (any, error) { return input.Values["name"], nil })
	svc.SetConstraintEvaluator("status.allowed", func(input RuleInput) error {
		switch input.Values["status"] {
		case "active", "inactive":
			return nil
		default:
			return sharedValidation("bad status")
		}
	})

	record, err := svc.Create("party", "u1", map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if record.Values["status"] != "active" || record.Values["display_name"] != "Alice" {
		t.Fatalf("expected defaults and computes, got %+v", record.Values)
	}
	updated, err := svc.Update("party", record.ID, "u1", map[string]any{"name": "Alice B", "status": "inactive"}, record.Version)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Version != 2 || updated.Values["display_name"] != "Alice B" {
		t.Fatalf("expected recomputed update, got %+v", updated)
	}
}

func TestListQueryFiltersRecords(t *testing.T) {
	svc := NewService()
	_ = svc.Register(Definition{Key: "party", DisplayName: "Party", Version: "v1", DefaultSort: "name", Fields: []FieldDefinition{{Key: "name", Type: "string"}, {Key: "status", Type: "string"}}})
	_, _ = svc.Create("party", "u1", map[string]any{"name": "Alice", "status": "active"})
	_, _ = svc.Create("party", "u1", map[string]any{"name": "Bob", "status": "inactive"})
	items, total, err := svc.List("party", Query{Filters: map[string]string{"status": "active"}})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Values["name"] != "Alice" {
		t.Fatalf("expected filtered result, got total=%d items=%+v", total, items)
	}
}

func TestCreateRelatedRecordInjectsForeignKey(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields:      []FieldDefinition{{Key: "name", Type: "string", Required: true}},
		Relations:   []RelationDefinition{{Key: "contacts", Type: "has_many", TargetModelKey: "party_contact", ForeignKey: "party_id"}},
	}); err != nil {
		t.Fatalf("register party failed: %v", err)
	}
	if err := svc.Register(Definition{
		Key:         "party_contact",
		DisplayName: "Party Contact",
		Version:     "v1",
		Fields: []FieldDefinition{
			{Key: "party_id", Type: "string", Required: true},
			{Key: "name", Type: "string", Required: true},
		},
	}); err != nil {
		t.Fatalf("register contact failed: %v", err)
	}
	parent, err := svc.Create("party", "u1", map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("create parent failed: %v", err)
	}
	child, err := svc.CreateRelated("party", parent.ID, "contacts", "u1", map[string]any{"name": "Primary Contact"})
	if err != nil {
		t.Fatalf("create related failed: %v", err)
	}
	if child.Values["party_id"] != parent.ID {
		t.Fatalf("expected injected parent foreign key, got %+v", child.Values)
	}
	items, total, err := svc.Related("party", parent.ID, "contacts", Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("related query failed: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != child.ID {
		t.Fatalf("expected related child result, got total=%d items=%+v", total, items)
	}
}

func TestUpdateCompositePatchesRelationChildren(t *testing.T) {
	svc := NewService()
	_ = svc.Register(Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields:      []FieldDefinition{{Key: "name", Type: "string", Required: true}},
		Relations:   []RelationDefinition{{Key: "contacts", Type: "has_many", TargetModelKey: "party_contact", ForeignKey: "party_id"}},
	})
	_ = svc.Register(Definition{
		Key:         "party_contact",
		DisplayName: "Party Contact",
		Version:     "v1",
		Fields: []FieldDefinition{
			{Key: "party_id", Type: "string", Required: true},
			{Key: "name", Type: "string", Required: true},
		},
	})
	parent, related, err := svc.CreateComposite("party", "u1", CompositeMutation{
		Values: map[string]any{"name": "Alice"},
		Relations: map[string][]ChildMutation{
			"contacts": {{Values: map[string]any{"name": "Old Contact"}}},
		},
	})
	if err != nil {
		t.Fatalf("create composite failed: %v", err)
	}
	if len(related["contacts"]) != 1 {
		t.Fatalf("expected initial child, got %+v", related)
	}
	updated, related, err := svc.UpdateComposite("party", parent.ID, "u1", CompositeMutation{
		ExpectedVersion: parent.Version,
		Values:          map[string]any{"name": "Alice B"},
		Relations: map[string][]ChildMutation{
			"contacts": {{Values: map[string]any{"name": "New Contact"}}},
		},
	})
	if err != nil {
		t.Fatalf("update composite failed: %v", err)
	}
	if updated.Values["name"] != "Alice B" {
		t.Fatalf("expected parent update, got %+v", updated.Values)
	}
	items, total, err := svc.Related("party", parent.ID, "contacts", Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("related query failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected patched child set, got total=%d items=%+v related=%+v", total, items, related)
	}
	foundOld := false
	foundNew := false
	for _, item := range items {
		switch item.Values["name"] {
		case "Old Contact":
			foundOld = true
		case "New Contact":
			foundNew = true
		}
	}
	if !foundOld || !foundNew {
		t.Fatalf("expected old and new children to coexist under patch semantics, got %+v", items)
	}
}

func TestCreateAndUpdateValidationErrors(t *testing.T) {
	svc := NewService()
	_ = svc.Register(Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields: []FieldDefinition{
			{Key: "name", Type: "string", Required: true},
			{Key: "status", Type: "string", ConstraintRuleKeys: []string{"status.allowed"}},
		},
	})
	svc.SetConstraintEvaluator("status.allowed", func(input RuleInput) error {
		if input.Values["status"] == "bad" {
			return sharedValidation("bad status")
		}
		return nil
	})

	if _, err := svc.Create("party", "", map[string]any{"name": ""}); err == nil {
		t.Fatal("expected required field validation")
	}
	record, err := svc.Create("party", "", map[string]any{"name": "Alice", "status": "active"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if record.CreatedBy != "system" || record.UpdatedBy != "system" {
		t.Fatalf("expected fallback actor, got %+v", record)
	}
	if _, err := svc.Update("party", record.ID, "u1", map[string]any{"name": "Alice", "status": "bad"}, record.Version); err == nil {
		t.Fatal("expected constraint validation")
	}
	if _, err := svc.Update("party", record.ID, "u1", map[string]any{"name": "Alice", "status": "active"}, 99); err == nil {
		t.Fatal("expected version mismatch")
	}
}

func TestApplyRelationsDeleteAndNestedChildren(t *testing.T) {
	svc := NewService()
	_ = svc.Register(Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields:      []FieldDefinition{{Key: "name", Type: "string", Required: true}},
		Relations:   []RelationDefinition{{Key: "contacts", Type: "has_many", TargetModelKey: "party_contact", ForeignKey: "party_id"}},
	})
	_ = svc.Register(Definition{
		Key:         "party_contact",
		DisplayName: "Party Contact",
		Version:     "v1",
		Fields: []FieldDefinition{
			{Key: "party_id", Type: "string", Required: true},
			{Key: "name", Type: "string", Required: true},
		},
		Relations: []RelationDefinition{{Key: "phones", Type: "has_many", TargetModelKey: "contact_phone", ForeignKey: "contact_id"}},
	})
	_ = svc.Register(Definition{
		Key:         "contact_phone",
		DisplayName: "Contact Phone",
		Version:     "v1",
		Fields: []FieldDefinition{
			{Key: "contact_id", Type: "string", Required: true},
			{Key: "number", Type: "string", Required: true},
		},
	})
	parent, related, err := svc.CreateComposite("party", "u1", CompositeMutation{
		Values: map[string]any{"name": "Alice"},
		Relations: map[string][]ChildMutation{
			"contacts": {{
				Values: map[string]any{"name": "Primary"},
				Relations: map[string][]ChildMutation{
					"phones": {{Values: map[string]any{"number": "123"}}},
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("create composite failed: %v", err)
	}
	contact := related["contacts"][0]
	phones, total, err := svc.Related("party_contact", contact.ID, "phones", Query{Page: 1, PageSize: 20})
	if err != nil || total != 1 || phones[0].Values["number"] != "123" {
		t.Fatalf("expected nested related phone, got total=%d items=%+v err=%v", total, phones, err)
	}
	_, _, err = svc.UpdateComposite("party", parent.ID, "u1", CompositeMutation{
		ExpectedVersion: parent.Version,
		Values:          map[string]any{"name": "Alice"},
		Relations: map[string][]ChildMutation{
			"contacts": {{Operation: "delete", ID: contact.ID}},
		},
	})
	if err != nil {
		t.Fatalf("delete child mutation failed: %v", err)
	}
	items, total, err := svc.Related("party", parent.ID, "contacts", Query{Page: 1, PageSize: 20})
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("expected deleted relation children, got total=%d items=%+v err=%v", total, items, err)
	}
}

func TestDefinitionsRepositoryAccessorsAndRegisterValidation(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(repo)
	if svc.Repository() != repo {
		t.Fatal("expected repository accessor")
	}
	alt := NewMemoryRepository()
	if svc.WithRepository(alt).Repository() != alt {
		t.Fatal("expected WithRepository to swap repository")
	}
	if svc.WithRepository(nil) != svc {
		t.Fatal("expected nil repository swap to return same service")
	}

	if err := svc.Register(Definition{}); err == nil {
		t.Fatal("expected invalid definition failure")
	}
	if err := svc.Register(Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields:      []FieldDefinition{{Key: "", Type: "string"}},
	}); err == nil {
		t.Fatal("expected invalid field definition failure")
	}
	if err := svc.Register(Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Relations:   []RelationDefinition{{Key: "contacts", Type: "has_many"}},
	}); err == nil {
		t.Fatal("expected invalid relation definition failure")
	}

	if err := svc.Register(Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields:      []FieldDefinition{{Key: "name", Type: "string"}},
	}); err != nil {
		t.Fatalf("register valid definition failed: %v", err)
	}
	if len(svc.Definitions()) != 1 {
		t.Fatalf("expected definition listing, got %+v", svc.Definitions())
	}
	if _, ok := svc.Definition("party"); !ok {
		t.Fatal("expected definition lookup")
	}
	if err := svc.WithRawRecordSave(Record{ModelKey: "party", ID: "party:raw", Version: 1, Values: map[string]any{"name": "Raw"}}); err != nil {
		t.Fatalf("expected raw record save, got %v", err)
	}
	if _, err := svc.Get("party", "party:raw"); err != nil {
		t.Fatalf("expected saved raw record lookup, got %v", err)
	}
	if _, err := svc.Get("party", "missing"); err == nil {
		t.Fatal("expected missing record error")
	}
}

func TestNormalizeQueryBoundsAndValidation(t *testing.T) {
	def := Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		DefaultSort: "name",
		Fields:      []FieldDefinition{{Key: "name", Type: "string"}, {Key: "status", Type: "string"}},
	}
	query, err := NormalizeQuery(def, Query{Filters: map[string]string{"name": "Alice", "": "", "status": ""}, Page: 0, PageSize: 999})
	if err != nil {
		t.Fatalf("normalize query failed: %v", err)
	}
	if query.Page != 1 || query.PageSize != MaxPageSize || query.SortKey != "name" {
		t.Fatalf("expected bounded query, got %+v", query)
	}
	if len(query.Filters) != 1 || query.Filters["name"] != "Alice" {
		t.Fatalf("expected cleaned filters, got %+v", query.Filters)
	}
	if _, err := NormalizeQuery(def, Query{SortKey: "unknown"}); err == nil {
		t.Fatal("expected unsupported sort key failure")
	}
	if _, err := NormalizeQuery(def, Query{Filters: map[string]string{"unknown": "x"}}); err == nil {
		t.Fatal("expected unsupported filter key failure")
	}
}

type sharedValidation string

func (e sharedValidation) Error() string { return string(e) }
