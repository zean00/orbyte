package audit

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresAuditRepository(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.Save(Event{ID: "a1", Action: "submit", TargetType: "document", TargetID: "d1", ActorID: "u1", OccurredAt: now, Metadata: map[string]any{"x": 1}}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	rows := sqlmock.NewRows([]string{"audit_event_id", "action", "target_type", "target_id", "actor_id", "from_state", "to_state", "occurred_at", "metadata_json", "correlation_id"}).AddRow("a1", "submit", "document", "d1", "u1", "draft", "submitted", now, []byte(`{"x":1}`), "c1")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT audit_event_id, action, target_type, target_id, actor_id,")).WillReturnRows(rows)
	if len(repo.List()) != 1 {
		t.Fatal("expected one audit event")
	}
}
