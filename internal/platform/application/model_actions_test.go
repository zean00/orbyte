package application

import (
	"testing"

	"clinic/internal/platform/activity"
	"clinic/internal/platform/audit"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/model"
)

func TestMemoryModelActionsCreateUpdateAndPatchRelation(t *testing.T) {
	models := model.NewService()
	_ = models.Register(model.Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields:      []model.FieldDefinition{{Key: "name", Type: "string", Required: true}},
		Relations:   []model.RelationDefinition{{Key: "contacts", Type: "has_many", TargetModelKey: "party_contact", ForeignKey: "party_id"}},
	})
	_ = models.Register(model.Definition{
		Key:         "party_contact",
		DisplayName: "Party Contact",
		Version:     "v1",
		Fields: []model.FieldDefinition{
			{Key: "party_id", Type: "string", Required: true},
			{Key: "name", Type: "string", Required: true},
		},
	})
	activities := activity.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewMemoryModelActions(models, activities, auditSvc, eventingSvc)

	record, related, err := actions.CreateComposite("party", "u1", model.CompositeMutation{
		Values: map[string]any{"name": "Alice"},
		Relations: map[string][]model.ChildMutation{
			"contacts": {{Values: map[string]any{"name": "Primary"}}},
		},
	})
	if err != nil {
		t.Fatalf("create composite failed: %v", err)
	}
	if len(related["contacts"]) != 1 {
		t.Fatalf("expected related child, got %+v", related)
	}
	if len(auditSvc.List()) != 1 || len(eventingSvc.ListEvents()) != 1 {
		t.Fatalf("expected runtime artifacts, got audit=%d events=%d", len(auditSvc.List()), len(eventingSvc.ListEvents()))
	}
	if len(activities.Timeline("model:party", record.ID)) == 0 {
		t.Fatal("expected activity timeline entry")
	}

	record, related, err = actions.UpdateComposite("party", record.ID, "u1", model.CompositeMutation{
		ExpectedVersion: record.Version,
		Values:          map[string]any{"name": "Alice B"},
		Relations: map[string][]model.ChildMutation{
			"contacts": {{Values: map[string]any{"name": "Secondary"}}},
		},
	})
	if err != nil {
		t.Fatalf("update composite failed: %v", err)
	}
	if record.Values["name"] != "Alice B" || len(related["contacts"]) != 1 {
		t.Fatalf("expected updated record and relation patch, got %+v %+v", record, related)
	}

	record, related, err = actions.PatchRelation("party", record.ID, "contacts", "u1", []model.ChildMutation{
		{Values: map[string]any{"name": "Tertiary"}},
	})
	if err != nil {
		t.Fatalf("patch relation failed: %v", err)
	}
	if len(related["contacts"]) != 1 {
		t.Fatalf("expected patched relation payload, got %+v", related)
	}
}

func TestModelActionHelpers(t *testing.T) {
	record := model.Record{ModelKey: "party", ID: "party:1", Version: 2}
	related := map[string][]model.Record{"contacts": {{ID: "c1"}}}
	if keys := relationKeys(related); len(keys) != 1 || keys[0] != "contacts" {
		t.Fatalf("expected relation keys, got %+v", keys)
	}
	if len(relationKeys(nil)) != 0 {
		t.Fatal("expected empty relation keys")
	}

	auditEvent := buildModelAuditEvent("create", record, related, "u1")
	if auditEvent.Action != "model.create" || auditEvent.Metadata["model_key"] != "party" {
		t.Fatalf("unexpected audit event %+v", auditEvent)
	}
	domainEvent := buildModelDomainEvent("update", record, related, "u1")
	if domainEvent.Type != "model.record.updated" || domainEvent.Payload["model_key"] != "party" {
		t.Fatalf("unexpected domain event %+v", domainEvent)
	}
}
