package audit

import (
	"testing"
	"time"
)

func TestServiceRecordAndList(t *testing.T) {
	svc := NewService()
	if err := svc.Record(Event{ID: "a1", Action: "x", TargetType: "document", TargetID: "d1", ActorID: "u1", OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	items := svc.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 event, got %d", len(items))
	}
}

func TestServiceQueryFiltersAndNilService(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()
	events := []Event{
		{
			ID:               "a1",
			Action:           "document.update",
			TargetType:       "document",
			TargetID:         "d1",
			ActorID:          "user_delegate",
			ActorKind:        "user",
			OnBehalfOfUserID: "user_admin",
			OrganizationID:   "org_default",
			LocationID:       "loc_hq",
			OccurredAt:       now,
			CorrelationID:    "corr-1",
		},
		{
			ID:              "a2",
			Action:          "document.approve",
			TargetType:      "document",
			TargetID:        "d2",
			ActorID:         "svc_agent",
			ActorKind:       "service",
			OrganizationID:  "org_default",
			LocationID:      "loc_hq",
			OperatingUnitID: "ou-1",
			OccurredAt:      now.Add(time.Minute),
			CorrelationID:   "corr-2",
		},
	}
	for _, event := range events {
		if err := svc.Record(event); err != nil {
			t.Fatalf("record failed: %v", err)
		}
	}

	filtered := svc.Query(Query{
		TargetType:       "document",
		ActorKind:        "user",
		OnBehalfOfUserID: "user_admin",
		CorrelationID:    "corr-1",
		OccurredFrom:     now.Add(-time.Second),
		OccurredTo:       now.Add(30 * time.Second),
	})
	if len(filtered) != 1 || filtered[0].ID != "a1" {
		t.Fatalf("unexpected filtered audit events: %+v", filtered)
	}

	var nilSvc *Service
	if items := nilSvc.Query(Query{TargetType: "document"}); items != nil {
		t.Fatalf("expected nil query result, got %+v", items)
	}
}
