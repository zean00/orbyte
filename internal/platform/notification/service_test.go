package notification

import (
	"strings"
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

func TestNotificationValidationAndOwnershipBranches(t *testing.T) {
	svc := NewService()

	if _, err := svc.Save(Item{Title: "missing user"}); err == nil || !strings.Contains(err.Error(), "user_id") {
		t.Fatalf("expected missing user validation, got %v", err)
	}
	if _, err := svc.Save(Item{UserID: "user_1"}); err == nil || !strings.Contains(err.Error(), "title") {
		t.Fatalf("expected missing title validation, got %v", err)
	}

	item, err := svc.Save(Item{
		UserID:   " user_1 ",
		Title:    " hello ",
		Status:   "unknown",
		Metadata: map[string]any{"a": 1},
	})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if item.Status != "unread" || item.UserID != "user_1" || item.Title != "hello" {
		t.Fatalf("expected normalized notification, got %+v", item)
	}
	item.Metadata["a"] = 2
	stored, ok := svc.Find(item.ID)
	if !ok {
		t.Fatalf("expected stored item")
	}
	if stored.Metadata["a"] != 1 {
		t.Fatalf("expected metadata clone isolation, got %+v", stored.Metadata)
	}

	if _, err := svc.MarkRead("missing", "user_1", time.Now().UTC()); err == nil {
		t.Fatal("expected missing notification error")
	}
	if _, err := svc.MarkRead(item.ID, "user_2", time.Now().UTC()); err == nil {
		t.Fatal("expected ownership error")
	}
}

func TestNotificationDismissMarksReadTimestampWhenMissing(t *testing.T) {
	svc := NewService()
	item, err := svc.Save(Item{UserID: "user_1", Title: "Needs action"})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	dismissed, err := svc.Dismiss(item.ID, "", time.Now().UTC())
	if err != nil {
		t.Fatalf("dismiss failed: %v", err)
	}
	if dismissed.ReadAt.IsZero() || dismissed.DismissedAt.IsZero() {
		t.Fatalf("expected dismiss to set timestamps, got %+v", dismissed)
	}
}
