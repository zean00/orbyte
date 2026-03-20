package workflow

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepositoryWorkflowLifecycle(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	def := Definition{
		Key:    "generic_request_flow",
		States: []string{"draft", "submitted"},
		Actions: []ActionRule{{
			Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit", TaskType: "review", CreateApproval: true,
		}, {
			Action: "approve", FromState: "submitted", ToState: "approved", PermissionKey: "document.approve",
		}},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_definition_versions")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_definitions")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := repo.SaveDefinition(def); err != nil {
		t.Fatalf("save definition failed: %v", err)
	}

	getRows := sqlmock.NewRows([]string{"workflow_key", "version_no", "states_json", "actions_json", "updated_at"}).
		AddRow("generic_request_flow", 1, []byte(`["draft","submitted"]`), []byte(`[{"action":"submit","from_state":"draft","to_state":"submitted","permission_key":"document.submit","task_type":"review","create_approval":true},{"action":"approve","from_state":"submitted","to_state":"approved","permission_key":"document.approve"}]`), now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT workflow_key, version_no, states_json, actions_json, updated_at")).WithArgs("generic_request_flow").WillReturnRows(getRows)
	loaded, ok := repo.GetDefinition("generic_request_flow")
	if !ok || loaded.Key != "generic_request_flow" || loaded.Version != 1 {
		t.Fatal("expected workflow definition")
	}

	versionRows := sqlmock.NewRows([]string{"workflow_key", "version_no", "status", "states_json", "actions_json", "created_at", "updated_at", "updated_by", "published_at", "published_by"}).
		AddRow("generic_request_flow", 1, "published", []byte(`["draft","submitted"]`), []byte(`[{"action":"submit","from_state":"draft","to_state":"submitted","permission_key":"document.submit","task_type":"review","create_approval":true},{"action":"approve","from_state":"submitted","to_state":"approved","permission_key":"document.approve"}]`), now, now, "system", now, "system")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT workflow_key, version_no, status, states_json, actions_json, created_at, updated_at,")).WithArgs("generic_request_flow").WillReturnRows(versionRows)
	if len(repo.ListDefinitionVersions("generic_request_flow")) != 1 {
		t.Fatal("expected one workflow version")
	}

	taskRows := sqlmock.NewRows([]string{"task_id", "workflow_key", "workflow_version", "target_type", "target_id", "task_type", "status", "assignment_mode", "assignee_user_id", "assignee_role_key", "candidate_role_keys_json", "created_by", "created_at", "due_at", "escalate_at", "metadata_json"}).AddRow("task:1", "generic_request_flow", 1, "document", "doc1", "review", "open", "", "", "", []byte(`[]`), "", now, nil, nil, []byte(`{}`))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_tasks")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveTask(Task{ID: "task:1", WorkflowKey: "generic_request_flow", WorkflowVersion: 1, TargetType: "document", TargetID: "doc1", TaskType: "review", Status: "open", CreatedAt: now}); err != nil {
		t.Fatalf("save task failed: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT task_id, workflow_key, COALESCE(workflow_version,0), target_type")).WillReturnRows(taskRows)
	if len(repo.ListTasks()) != 1 {
		t.Fatal("expected listed workflow task")
	}

	approvalRows := sqlmock.NewRows([]string{"approval_id", "workflow_key", "workflow_version", "target_type", "target_id", "status", "stage_key", "requested_by", "requested_at", "resolved_by", "resolved_at", "candidate_role_keys_json", "due_at", "metadata_json"}).AddRow("approval:1", "generic_request_flow", 1, "document", "doc1", "pending", "", "", now, "", nil, []byte(`[]`), nil, []byte(`{}`))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_approvals")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveApproval(Approval{ID: "approval:1", WorkflowKey: "generic_request_flow", WorkflowVersion: 1, TargetType: "document", TargetID: "doc1", Status: "pending", RequestedAt: now}); err != nil {
		t.Fatalf("save approval failed: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT approval_id, workflow_key, COALESCE(workflow_version,0), target_type")).WillReturnRows(approvalRows)
	if len(repo.ListApprovals()) != 1 {
		t.Fatal("expected listed workflow approval")
	}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE workflow_tasks")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.UpdateTaskStatus(TaskStatusUpdate{ID: "task:1", Status: "completed"}); err != nil {
		t.Fatalf("update task failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("UPDATE workflow_approvals")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.UpdateApprovalStatus(ApprovalStatusUpdate{ID: "approval:1", Status: "approved"}); err != nil {
		t.Fatalf("update approval failed: %v", err)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_history")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveHistory(HistoryEvent{ID: "h1", WorkflowKey: "generic_request_flow", WorkflowVersion: 1, TargetType: "document", TargetID: "doc1", Action: "submit", OccurredAt: now}); err != nil {
		t.Fatalf("save history failed: %v", err)
	}
	historyRows := sqlmock.NewRows([]string{"history_id", "workflow_key", "workflow_version", "target_type", "target_id", "action", "from_state", "to_state", "actor_id", "occurred_at", "decision_code", "decision_reason", "assignment_summary_json", "metadata_json"}).
		AddRow("h1", "generic_request_flow", 1, "document", "doc1", "submit", "draft", "submitted", "u1", now, "", "", []byte(`{}`), []byte(`{}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT history_id, workflow_key, COALESCE(workflow_version,0), target_type")).WithArgs("document", "doc1").WillReturnRows(historyRows)
	if len(repo.ListHistory("document", "doc1")) != 1 {
		t.Fatal("expected workflow history")
	}
}
