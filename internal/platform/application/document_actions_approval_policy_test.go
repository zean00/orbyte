package application

import (
	"testing"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/workflow"
)

func TestApproveProgressesApprovalPolicyStagesBeforeFinalApproval(t *testing.T) {
	models := model.NewService()
	registerApprovalPolicyTestModels(t, models)
	policies := NewApprovalPolicyService(models)

	policyRecord, err := models.Create("approval_policy", "tester", map[string]any{
		"code":          "GENERIC-REQUEST",
		"name":          "Generic Request Approval",
		"document_type": "generic_request",
		"action":        "submit",
		"status":        "active",
	})
	if err != nil {
		t.Fatalf("create policy failed: %v", err)
	}
	_, err = models.Create("approval_policy_stage", "tester", map[string]any{
		"policy_id":                policyRecord.ID,
		"stage_key":                "supervisor",
		"sequence":                 1,
		"assignment_strategy":      "explicit_user",
		"explicit_user_id":         "user_supervisor",
		"requires_different_actor": true,
		"status":                   "active",
	})
	if err != nil {
		t.Fatalf("create stage 1 failed: %v", err)
	}
	_, err = models.Create("approval_policy_stage", "tester", map[string]any{
		"policy_id":           policyRecord.ID,
		"stage_key":           "finance",
		"sequence":            2,
		"assignment_strategy": "explicit_user",
		"explicit_user_id":    "user_finance",
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create stage 2 failed: %v", err)
	}

	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))
	actions.AttachApprovalPolicies(policies)

	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_requester", map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	record, err = actions.Submit(record.Header.ID, testActing("user_requester"), record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	approvals := flows.ListApprovals()
	if len(approvals) != 1 {
		t.Fatalf("expected 1 approval after submit, got %d", len(approvals))
	}
	if approvals[0].StageKey != "supervisor" {
		t.Fatalf("expected supervisor stage, got %s", approvals[0].StageKey)
	}
	if got := approvals[0].Metadata["resolved_assignee_user_id"]; got != "user_supervisor" {
		t.Fatalf("expected supervisor assignment, got %+v", got)
	}

	record, err = actions.Approve(record.Header.ID, testActing("user_supervisor"), record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("first approve failed: %v", err)
	}
	if record.Header.Status != "submitted" {
		t.Fatalf("expected document to remain submitted between stages, got %s", record.Header.Status)
	}
	approvals = flows.ListApprovals()
	if len(approvals) != 2 {
		t.Fatalf("expected 2 approvals after stage progression, got %d", len(approvals))
	}
	if approvals[0].Status != "approved" {
		t.Fatalf("expected first approval resolved, got %s", approvals[0].Status)
	}
	if approvals[1].StageKey != "finance" {
		t.Fatalf("expected finance stage, got %s", approvals[1].StageKey)
	}
	if got := approvals[1].Metadata["resolved_assignee_user_id"]; got != "user_finance" {
		t.Fatalf("expected finance assignment, got %+v", got)
	}

	record, err = actions.Approve(record.Header.ID, testActing("user_finance"), record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("final approve failed: %v", err)
	}
	if record.Header.Status != "approved" {
		t.Fatalf("expected approved status, got %s", record.Header.Status)
	}
}

func TestApproveWaitsForRequiredApproverCountBeforeAdvancingStage(t *testing.T) {
	models := model.NewService()
	registerApprovalPolicyTestModels(t, models)
	policies := NewApprovalPolicyService(models)

	policyRecord, err := models.Create("approval_policy", "tester", map[string]any{
		"code":          "GENERIC-REQUEST-QUORUM",
		"name":          "Generic Request Quorum",
		"document_type": "generic_request",
		"action":        "submit",
		"status":        "active",
	})
	if err != nil {
		t.Fatalf("create policy failed: %v", err)
	}
	_, err = models.Create("approval_policy_stage", "tester", map[string]any{
		"policy_id":                policyRecord.ID,
		"stage_key":                "committee",
		"sequence":                 1,
		"assignment_strategy":      "explicit_user",
		"explicit_user_id":         "user_committee_a",
		"required_approver_count":  2,
		"requires_different_actor": true,
		"status":                   "active",
	})
	if err != nil {
		t.Fatalf("create stage 1 failed: %v", err)
	}
	_, err = models.Create("approval_policy_stage", "tester", map[string]any{
		"policy_id":           policyRecord.ID,
		"stage_key":           "finance",
		"sequence":            2,
		"assignment_strategy": "explicit_user",
		"explicit_user_id":    "user_finance",
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create stage 2 failed: %v", err)
	}

	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))
	actions.AttachApprovalPolicies(policies)

	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_requester", map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	record, err = actions.Submit(record.Header.ID, testActing("user_requester"), record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	record, err = actions.Approve(record.Header.ID, testActing("user_committee_a"), record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("first committee approve failed: %v", err)
	}
	if record.Header.Status != "submitted" {
		t.Fatalf("expected submitted status after partial quorum, got %s", record.Header.Status)
	}
	if got := metadataStrings(record.Header.Metadata, "approval_stage_approver_ids"); len(got) != 0 {
		t.Fatalf("expected stage approver ids to be stored by stage map, got flat values %+v", got)
	}
	approverMap, _ := record.Header.Metadata["approval_stage_approver_ids"].(map[string]any)
	if got := metadataStrings(approverMap, "committee"); len(got) != 1 || got[0] != "user_committee_a" {
		t.Fatalf("expected first committee approver recorded, got %+v", approverMap)
	}
	approvals := flows.ListApprovals()
	if len(approvals) != 1 || approvals[0].StageKey != "committee" || approvals[0].Status != "pending" {
		t.Fatalf("expected committee approval to remain pending, got %+v", approvals)
	}

	record, err = actions.Approve(record.Header.ID, testActing("user_committee_b"), record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("second committee approve failed: %v", err)
	}
	approvals = flows.ListApprovals()
	if len(approvals) != 2 {
		t.Fatalf("expected finance stage after quorum, got %+v", approvals)
	}
	if approvals[0].Status != "approved" || approvals[1].StageKey != "finance" || approvals[1].Status != "pending" {
		t.Fatalf("unexpected approvals after quorum: %+v", approvals)
	}
}
