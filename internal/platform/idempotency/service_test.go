package idempotency

import (
	"errors"
	"testing"

	"orbyte/internal/platform/shared"
)

func TestExecuteCachesSuccessfulOutcome(t *testing.T) {
	svc := NewService()
	calls := 0

	first, err := svc.Execute("document.create", "idem-1", "user_admin", map[string]any{"title": "A"}, func() (Outcome, error) {
		calls++
		return Outcome{StatusCode: 201, Response: map[string]any{"id": "doc_1"}}, nil
	})
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}
	second, err := svc.Execute("document.create", "idem-1", "user_admin", map[string]any{"title": "A"}, func() (Outcome, error) {
		calls++
		return Outcome{StatusCode: 201, Response: map[string]any{"id": "doc_2"}}, nil
	})
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one execution, got %d", calls)
	}
	if first.Response["id"] != "doc_1" || second.Response["id"] != "doc_1" {
		t.Fatalf("expected cached response, got first=%+v second=%+v", first, second)
	}
	if len(svc.List()) != 1 || svc.List()[0].Status != "succeeded" {
		t.Fatalf("expected succeeded record, got %+v", svc.List())
	}
}

func TestExecuteRejectsHashMismatchAndStoresFailure(t *testing.T) {
	svc := NewService()
	_, err := svc.Execute("document.create", "idem-2", "user_admin", map[string]any{"title": "A"}, func() (Outcome, error) {
		return Outcome{StatusCode: 201, Response: map[string]any{"id": "doc_1"}}, nil
	})
	if err != nil {
		t.Fatalf("seed execute failed: %v", err)
	}
	_, err = svc.Execute("document.create", "idem-2", "user_admin", map[string]any{"title": "B"}, func() (Outcome, error) {
		return Outcome{}, nil
	})
	if err == nil {
		t.Fatal("expected conflict on request hash mismatch")
	}

	_, err = svc.Execute("document.submit", "idem-3", "user_admin", map[string]any{"id": "doc_1"}, func() (Outcome, error) {
		return Outcome{}, shared.Forbidden("not allowed")
	})
	if err == nil {
		t.Fatal("expected failing execute error")
	}
	record := svc.List()[1]
	if record.Status != "failed" || record.ResponseCode != 403 {
		t.Fatalf("expected failed record with 403, got %+v", record)
	}
}

func TestExecuteWithoutKeyAndStatusHelpers(t *testing.T) {
	svc := NewService()
	calls := 0
	_, err := svc.Execute("document.create", "", "user_admin", map[string]any{"title": "A"}, func() (Outcome, error) {
		calls++
		return Outcome{}, nil
	})
	if err != nil {
		t.Fatalf("execute without key failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected direct execution without key, got %d", calls)
	}

	if status := httpStatusForError(shared.Validation("bad")); status != 400 {
		t.Fatalf("expected validation status 400, got %d", status)
	}
	if status := httpStatusForError(shared.NotFound("missing")); status != 404 {
		t.Fatalf("expected not found status 404, got %d", status)
	}
	if status := httpStatusForError(errors.New("boom")); status != 500 {
		t.Fatalf("expected generic status 500, got %d", status)
	}
}
