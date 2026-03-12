package activity

import "testing"

func TestTimelineIncludesMessagesAndActivities(t *testing.T) {
	svc := NewService()
	if _, err := svc.AddMessage("model:party", "p1", "u1", "Created", nil); err != nil {
		t.Fatalf("add message failed: %v", err)
	}
	act, err := svc.CreateActivity("model:party", "p1", "u1", "u2", "Verify data")
	if err != nil {
		t.Fatalf("create activity failed: %v", err)
	}
	if _, err := svc.CompleteActivity(act.ID); err != nil {
		t.Fatalf("complete activity failed: %v", err)
	}
	items := svc.Timeline("model:party", "p1")
	if len(items) != 2 {
		t.Fatalf("expected two timeline items, got %d", len(items))
	}
}
