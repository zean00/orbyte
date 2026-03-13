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

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_definitions (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveDefinition(def); err != nil {
		t.Fatalf("save definition failed: %v", err)
	}

	getRows := sqlmock.NewRows([]string{"workflow_key", "states_json", "actions_json"}).
		AddRow("generic_request_flow", []byte(`["draft","submitted"]`), []byte(`[{"action":"submit","from_state":"draft","to_state":"submitted","permission_key":"document.submit","task_type":"review","create_approval":true},{"action":"approve","from_state":"submitted","to_state":"approved","permission_key":"document.approve"}]`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT workflow_key, states_json, actions_json")).WithArgs("generic_request_flow").WillReturnRows(getRows)
	loaded, ok := repo.GetDefinition("generic_request_flow")
	if !ok || loaded.Key != "generic_request_flow" {
		t.Fatal("expected workflow definition")
	}

	listRows := sqlmock.NewRows([]string{"workflow_key", "states_json", "actions_json"}).
		AddRow("generic_request_flow", []byte(`["draft","submitted"]`), []byte(`[{"action":"submit","from_state":"draft","to_state":"submitted","permission_key":"document.submit","task_type":"review","create_approval":true},{"action":"approve","from_state":"submitted","to_state":"approved","permission_key":"document.approve"}]`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT workflow_key, states_json, actions_json FROM workflow_definitions")).WillReturnRows(listRows)
	if len(repo.ListDefinitions()) != 1 {
		t.Fatal("expected listed workflow definition")
	}

	taskRows := sqlmock.NewRows([]string{"task_id", "workflow_key", "target_type", "target_id", "task_type", "status", "assignment_mode", "assignee_user_id", "assignee_role_key", "candidate_role_keys_json", "created_by", "created_at", "due_at", "escalate_at", "metadata_json"}).AddRow("task:1", "generic_request_flow", "document", "doc1", "review", "open", "", "", "", []byte(`[]`), "", now, nil, nil, []byte(`{}`))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_tasks (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveTask(Task{ID: "task:1", WorkflowKey: "generic_request_flow", TargetType: "document", TargetID: "doc1", TaskType: "review", Status: "open", CreatedAt: now}); err != nil {
		t.Fatalf("save task failed: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT task_id, workflow_key, target_type, target_id, task_type, status, COALESCE(assignment_mode,''), COALESCE(assignee_user_id,''), COALESCE(assignee_role_key,''), COALESCE(candidate_role_keys_json,'[]'::jsonb), COALESCE(created_by,''), created_at, due_at, escalate_at, COALESCE(metadata_json,'{}'::jsonb) FROM workflow_tasks")).WillReturnRows(taskRows)
	if len(repo.ListTasks()) != 1 {
		t.Fatal("expected listed workflow task")
	}

	approvalRows := sqlmock.NewRows([]string{"approval_id", "workflow_key", "target_type", "target_id", "status", "stage_key", "requested_by", "requested_at", "resolved_by", "resolved_at", "candidate_role_keys_json", "due_at", "metadata_json"}).AddRow("approval:1", "generic_request_flow", "document", "doc1", "pending", "", "", now, "", nil, []byte(`[]`), nil, []byte(`{}`))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_approvals (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveApproval(Approval{ID: "approval:1", WorkflowKey: "generic_request_flow", TargetType: "document", TargetID: "doc1", Status: "pending", RequestedAt: now}); err != nil {
		t.Fatalf("save approval failed: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT approval_id, workflow_key, target_type, target_id, status, COALESCE(stage_key,''), COALESCE(requested_by,''), requested_at, COALESCE(resolved_by,''), resolved_at, COALESCE(candidate_role_keys_json,'[]'::jsonb), due_at, COALESCE(metadata_json,'{}'::jsonb) FROM workflow_approvals")).WillReturnRows(approvalRows)
	if len(repo.ListApprovals()) != 1 {
		t.Fatal("expected listed workflow approval")
	}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE workflow_tasks SET status = $1, metadata_json = COALESCE(metadata_json,'{}'::jsonb) || jsonb_build_object('resolved_by', NULLIF($2,''), 'resolved_at', $3) WHERE task_id = $4")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.UpdateTaskStatus(TaskStatusUpdate{ID: "task:1", Status: "completed"}); err != nil {
		t.Fatalf("update task failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("UPDATE workflow_approvals SET status = $1, resolved_by = NULLIF($2,''), resolved_at = $3 WHERE approval_id = $4")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.UpdateApprovalStatus(ApprovalStatusUpdate{ID: "approval:1", Status: "approved"}); err != nil {
		t.Fatalf("update approval failed: %v", err)
	}
}
