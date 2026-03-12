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
