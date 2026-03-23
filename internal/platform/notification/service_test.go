package notification

import (
	"testing"
	"time"
)

func TestSaveListAndSummary(t *testing.T) {
	svc := NewService()
	if _, err := svc.Save(Item{UserID: "user_1", Title: "Approval needed", Category: "workflow_approval"}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if _, err := svc.Save(Item{UserID: "user_1", Title: "Task assigned", Category: "workflow_task", Status: "read"}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	items := svc.List(Filter{UserID: "user_1"})
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	summary := svc.Summary("user_1")
	if summary.Total != 2 || summary.Unread != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestMarkReadAndDismiss(t *testing.T) {
	svc := NewService()
	item, err := svc.Save(Item{UserID: "user_1", Title: "Needs action"})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	read, err := svc.MarkRead(item.ID, "user_1", time.Now().UTC())
	if err != nil {
		t.Fatalf("mark read failed: %v", err)
	}
	if read.Status != "read" || read.ReadAt.IsZero() {
		t.Fatalf("expected read notification, got %+v", read)
	}
	dismissed, err := svc.Dismiss(item.ID, "user_1", time.Now().UTC())
	if err != nil {
		t.Fatalf("dismiss failed: %v", err)
	}
	if dismissed.Status != "dismissed" || dismissed.DismissedAt.IsZero() {
		t.Fatalf("expected dismissed notification, got %+v", dismissed)
	}
}
