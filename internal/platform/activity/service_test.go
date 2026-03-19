package activity

import "testing"

func TestFollowAndTimeline(t *testing.T) {
	svc := NewService()

	follower, err := svc.Follow("document", "doc-1", "user-1")
	if err != nil {
		t.Fatalf("follow failed: %v", err)
	}
	if follower.ActorID != "user-1" || follower.TargetID != "doc-1" {
		t.Fatalf("unexpected follower: %+v", follower)
	}

	if _, err := svc.AddMessage("document", "doc-1", "", "hello", map[string]any{"source": "test"}); err != nil {
		t.Fatalf("add message failed: %v", err)
	}
	activity, err := svc.CreateActivity("document", "doc-1", "", "user-1", "Review document")
	if err != nil {
		t.Fatalf("create activity failed: %v", err)
	}
	completed, err := svc.CompleteActivity(activity.ID)
	if err != nil {
		t.Fatalf("complete activity failed: %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("expected completed activity, got %+v", completed)
	}

	timeline := svc.Timeline("document", "doc-1")
	if len(timeline) != 2 {
		t.Fatalf("expected message and activity timeline entries, got %+v", timeline)
	}
	if timeline[0].Kind != "message" || timeline[1].Kind != "activity" {
		t.Fatalf("unexpected timeline order: %+v", timeline)
	}
}

func TestFollowValidation(t *testing.T) {
	svc := NewService()
	if _, err := svc.Follow("", "doc-1", "user-1"); err == nil {
		t.Fatal("expected follow validation error")
	}
}
