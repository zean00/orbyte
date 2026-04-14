package application

import (
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

func TestApprovalPolicyServiceResolveDocumentPolicy(t *testing.T) {
	models := model.NewService()
	registerApprovalPolicyTestModels(t, models)
	service := NewApprovalPolicyService(models)

	if _, err := models.Create("approval_policy", "tester", map[string]any{
		"code":          "GENERIC",
		"name":          "Generic",
		"document_type": "*",
		"action":        "submit",
		"priority":      1,
		"status":        "active",
	}); err != nil {
		t.Fatalf("create generic policy failed: %v", err)
	}
	best, err := models.Create("approval_policy", "tester", map[string]any{
		"code":                 "PROC-PO",
		"name":                 "Purchase Order Policy",
		"document_type":        "purchase_order",
		"action":               "submit",
		"location_id":          "loc_hq",
		"minimum_amount_minor": 10000,
		"priority":             5,
		"status":               "active",
	})
	if err != nil {
		t.Fatalf("create scoped policy failed: %v", err)
	}
	if _, err := models.Create("approval_policy_stage", "tester", map[string]any{
		"policy_id":           best.ID,
		"stage_key":           "finance",
		"sequence":            1,
		"assignment_strategy": "explicit_user",
		"explicit_user_id":    "user_finance",
		"due_after_seconds":   3600,
		"status":              "active",
	}); err != nil {
		t.Fatalf("create policy stage failed: %v", err)
	}

	record := document.Record{
		Header: document.Header{
			ID:             "doc-1",
			Type:           "purchase_order",
			Status:         "draft",
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
			TotalAmount:    sharedMoney(20000),
			Metadata:       map[string]any{"workflow_key": "purchase_order_flow"},
		},
	}
	resolution, ok, err := service.ResolveDocumentPolicy(record, "submit", "purchase_order_flow")
	if err != nil {
		t.Fatalf("resolve policy failed: %v", err)
	}
	if !ok {
		t.Fatal("expected policy match")
	}
	if resolution.Policy.ID != best.ID {
		t.Fatalf("expected scoped policy %s, got %s", best.ID, resolution.Policy.ID)
	}
	if len(resolution.Stages) != 1 || resolution.Stages[0].AssigneeUserID != "user_finance" {
		t.Fatalf("unexpected stage resolution: %+v", resolution.Stages)
	}
}

func TestApprovalPolicyServiceResolveDocumentPolicyMatchesWorkflowKeyOnSubmit(t *testing.T) {
	models := model.NewService()
	registerApprovalPolicyTestModels(t, models)
	service := NewApprovalPolicyService(models)

	policyRecord, err := models.Create("approval_policy", "tester", map[string]any{
		"code":          "REQUEST-FLOW",
		"name":          "Request Flow Policy",
		"document_type": "generic_request",
		"workflow_key":  "generic_request_flow",
		"action":        "submit",
		"priority":      9,
		"status":        "active",
	})
	if err != nil {
		t.Fatalf("create policy failed: %v", err)
	}
	if _, err := models.Create("approval_policy_stage", "tester", map[string]any{
		"policy_id":           policyRecord.ID,
		"stage_key":           "review",
		"sequence":            1,
		"assignment_strategy": "explicit_user",
		"explicit_user_id":    "user_approver",
		"status":              "active",
	}); err != nil {
		t.Fatalf("create stage failed: %v", err)
	}

	record := document.Record{
		Header: document.Header{
			ID:             "doc-submit",
			Type:           "generic_request",
			Status:         "draft",
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
			TotalAmount:    sharedMoney(1000),
			Metadata:       map[string]any{},
		},
	}
	resolution, ok, err := service.ResolveDocumentPolicy(record, "submit", "generic_request_flow")
	if err != nil {
		t.Fatalf("resolve policy failed: %v", err)
	}
	if !ok {
		t.Fatal("expected workflow-scoped policy match")
	}
	if resolution.Policy.ID != policyRecord.ID {
		t.Fatalf("expected policy %s, got %s", policyRecord.ID, resolution.Policy.ID)
	}
}

func TestApprovalPolicyServiceResolveDocumentPolicyFallbackApproverGroup(t *testing.T) {
	models := model.NewService()
	registerApprovalPolicyTestModels(t, models)
	service := NewApprovalPolicyService(models)

	policyRecord, err := models.Create("approval_policy", "tester", map[string]any{
		"code":                "GROUP-FALLBACK",
		"name":                "Group Fallback",
		"document_type":       "generic_request",
		"action":              "submit",
		"assignment_strategy": "approver_group",
		"approver_group_id":   "finance_group",
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create policy failed: %v", err)
	}
	for _, userID := range []string{"user_finance_a", "user_finance_b"} {
		if _, err := models.Create("approver_group_member", "tester", map[string]any{
			"approver_group_id": "finance_group",
			"user_id":           userID,
			"status":            "active",
		}); err != nil {
			t.Fatalf("create group member failed: %v", err)
		}
	}

	record := document.Record{
		Header: document.Header{
			ID:             "doc-group",
			Type:           "generic_request",
			Status:         "draft",
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
		},
	}
	resolution, ok, err := service.ResolveDocumentPolicy(record, "submit", "generic_request_flow")
	if err != nil {
		t.Fatalf("resolve policy failed: %v", err)
	}
	if !ok {
		t.Fatal("expected fallback policy match")
	}
	if resolution.Policy.ID != policyRecord.ID {
		t.Fatalf("expected policy %s, got %s", policyRecord.ID, resolution.Policy.ID)
	}
	if len(resolution.Stages) != 1 {
		t.Fatalf("expected one fallback stage, got %d", len(resolution.Stages))
	}
	if got := resolution.Stages[0].CandidateUserIDs; len(got) != 2 || got[0] != "user_finance_a" || got[1] != "user_finance_b" {
		t.Fatalf("unexpected fallback candidate users: %+v", got)
	}
}

func registerApprovalPolicyTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	defs := []model.Definition{
		{
			Key:                 "approval_policy",
			DisplayName:         "Approval Policy",
			DisplayNameI18n:     i18n.LocalizedText{"en": "Approval Policy"},
			Version:             "v1",
			CreatePermissionKey: "approval_policy.create",
			ListPermissionKey:   "approval_policy.list",
			ReadPermissionKey:   "approval_policy.read",
			UpdatePermissionKey: "approval_policy.update",
			DefaultSort:         "priority",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Label: "Code"},
				{Key: "name", Type: "string", Label: "Name"},
				{Key: "document_type", Type: "string", Label: "Document Type"},
				{Key: "workflow_key", Type: "string", Label: "Workflow Key"},
				{Key: "action", Type: "string", Label: "Action"},
				{Key: "organization_id", Type: "string", Label: "Organization"},
				{Key: "location_id", Type: "string", Label: "Location"},
				{Key: "operating_unit_id", Type: "string", Label: "Operating Unit"},
				{Key: "department_id", Type: "string", Label: "Department"},
				{Key: "cost_center_id", Type: "string", Label: "Cost Center"},
				{Key: "minimum_amount_minor", Type: "number", Label: "Minimum Amount"},
				{Key: "maximum_amount_minor", Type: "number", Label: "Maximum Amount"},
				{Key: "routing_mode", Type: "string", Label: "Routing Mode"},
				{Key: "assignment_strategy", Type: "string", Label: "Assignment Strategy"},
				{Key: "assignment_mode", Type: "string", Label: "Assignment Mode"},
				{Key: "assignee_role_key", Type: "string", Label: "Assignee Role"},
				{Key: "fallback_role_key", Type: "string", Label: "Fallback Role"},
				{Key: "approver_group_id", Type: "string", Label: "Approver Group"},
				{Key: "explicit_user_id", Type: "string", Label: "Explicit User"},
				{Key: "candidate_role_keys", Type: "string", Label: "Candidate Roles"},
				{Key: "priority", Type: "number", Label: "Priority"},
				{Key: "status", Type: "string", Label: "Status"},
			},
		},
		{
			Key:                 "approval_policy_stage",
			DisplayName:         "Approval Policy Stage",
			DisplayNameI18n:     i18n.LocalizedText{"en": "Approval Policy Stage"},
			Version:             "v1",
			CreatePermissionKey: "approval_policy.create",
			ListPermissionKey:   "approval_policy.list",
			ReadPermissionKey:   "approval_policy.read",
			UpdatePermissionKey: "approval_policy.update",
			DefaultSort:         "sequence",
			Fields: []model.FieldDefinition{
				{Key: "policy_id", Type: "string", Label: "Policy"},
				{Key: "stage_key", Type: "string", Label: "Stage"},
				{Key: "sequence", Type: "number", Label: "Sequence"},
				{Key: "required_approver_count", Type: "number", Label: "Required Approver Count"},
				{Key: "routing_mode", Type: "string", Label: "Routing Mode"},
				{Key: "assignment_strategy", Type: "string", Label: "Assignment Strategy"},
				{Key: "assignment_mode", Type: "string", Label: "Assignment Mode"},
				{Key: "assignee_role_key", Type: "string", Label: "Assignee Role"},
				{Key: "fallback_role_key", Type: "string", Label: "Fallback Role"},
				{Key: "approver_group_id", Type: "string", Label: "Approver Group"},
				{Key: "explicit_user_id", Type: "string", Label: "Explicit User"},
				{Key: "candidate_role_keys", Type: "string", Label: "Candidate Roles"},
				{Key: "due_after_seconds", Type: "number", Label: "Due After Seconds"},
				{Key: "escalate_after_seconds", Type: "number", Label: "Escalate After Seconds"},
				{Key: "requires_different_actor", Type: "boolean", Label: "Requires Different Actor"},
				{Key: "status", Type: "string", Label: "Status"},
			},
		},
		{
			Key:                 "approver_group_member",
			DisplayName:         "Approver Group Member",
			DisplayNameI18n:     i18n.LocalizedText{"en": "Approver Group Member"},
			Version:             "v1",
			CreatePermissionKey: "approval_policy.create",
			ListPermissionKey:   "approval_policy.list",
			ReadPermissionKey:   "approval_policy.read",
			UpdatePermissionKey: "approval_policy.update",
			DefaultSort:         "user_id",
			Fields: []model.FieldDefinition{
				{Key: "approver_group_id", Type: "string", Label: "Approver Group"},
				{Key: "user_id", Type: "string", Label: "User"},
				{Key: "status", Type: "string", Label: "Status"},
			},
		},
	}
	for _, def := range defs {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s failed: %v", def.Key, err)
		}
	}
}

func sharedMoney(amountMinor int64) shared.Money {
	return shared.Money{AmountMinor: amountMinor, Currency: "IDR"}
}
