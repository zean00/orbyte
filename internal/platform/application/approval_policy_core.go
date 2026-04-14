package application

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

type ApprovalPolicyService struct {
	models *model.Service
}

type ApprovalPolicyResolution struct {
	Policy model.Record
	Stages []ApprovalPolicyStageResolution
}

type ApprovalPolicyStageResolution struct {
	PolicyID               string
	StageKey               string
	Sequence               int
	TotalStages            int
	RequiredApproverCount  int
	RoutingMode            string
	AssignmentStrategy     string
	AssignmentMode         string
	AssigneeRoleKey        string
	FallbackRoleKey        string
	ApproverGroupID        string
	ExplicitUserID         string
	CandidateRoleKeys      []string
	AssigneeUserID         string
	CandidateUserIDs       []string
	DueAfterSeconds        int
	EscalateAfterSeconds   int
	RequiresDifferentActor bool
}

func NewApprovalPolicyService(models *model.Service) *ApprovalPolicyService {
	return &ApprovalPolicyService{models: models}
}

func (s *ApprovalPolicyService) ResolveDocumentPolicy(record document.Record, action, workflowKey string) (ApprovalPolicyResolution, bool, error) {
	if s == nil || s.models == nil || strings.TrimSpace(record.Header.Type) == "" {
		return ApprovalPolicyResolution{}, false, nil
	}
	items, _, err := s.models.List("approval_policy", model.Query{
		Page:     1,
		PageSize: model.MaxPageSize,
		SortKey:  "priority",
		Desc:     true,
	})
	if err != nil {
		return ApprovalPolicyResolution{}, false, err
	}
	var best model.Record
	bestScore := -1
	for _, item := range items {
		score, ok := approvalPolicyMatchScore(item, record, action, workflowKey)
		if !ok {
			continue
		}
		if score > bestScore || (score == bestScore && item.UpdatedAt.After(best.UpdatedAt)) {
			best = item
			bestScore = score
		}
	}
	if bestScore < 0 {
		return ApprovalPolicyResolution{}, false, nil
	}
	stages, err := s.StagesForPolicy(best.ID)
	if err != nil {
		return ApprovalPolicyResolution{}, false, err
	}
	if len(stages) == 0 {
		stage := approvalPolicyFallbackStage(best)
		stage.AssigneeUserID, stage.CandidateUserIDs, err = s.resolveStageUsers(stage)
		if err != nil {
			return ApprovalPolicyResolution{}, false, err
		}
		stages = []ApprovalPolicyStageResolution{stage}
	}
	totalStages := len(stages)
	for i := range stages {
		stages[i].PolicyID = best.ID
		stages[i].TotalStages = totalStages
	}
	return ApprovalPolicyResolution{Policy: best, Stages: stages}, true, nil
}

func (s *ApprovalPolicyService) StagesForPolicy(policyID string) ([]ApprovalPolicyStageResolution, error) {
	if s == nil || s.models == nil || strings.TrimSpace(policyID) == "" {
		return nil, nil
	}
	items, _, err := s.models.List("approval_policy_stage", model.Query{
		Filters:  map[string]string{"policy_id": strings.TrimSpace(policyID)},
		SortKey:  "sequence",
		Desc:     false,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	stages := make([]ApprovalPolicyStageResolution, 0, len(items))
	for _, item := range items {
		if !strings.EqualFold(stringValue(item.Values["status"]), "active") {
			continue
		}
		stage := ApprovalPolicyStageResolution{
			PolicyID:               strings.TrimSpace(policyID),
			StageKey:               firstNonEmpty(strings.TrimSpace(stringValue(item.Values["stage_key"])), fmt.Sprintf("stage_%d", intValue(item.Values["sequence"]))),
			Sequence:               intValue(item.Values["sequence"]),
			RequiredApproverCount:  max(1, intValue(item.Values["required_approver_count"])),
			RoutingMode:            strings.TrimSpace(stringValue(item.Values["routing_mode"])),
			AssignmentStrategy:     strings.TrimSpace(stringValue(item.Values["assignment_strategy"])),
			AssignmentMode:         strings.TrimSpace(stringValue(item.Values["assignment_mode"])),
			AssigneeRoleKey:        strings.TrimSpace(stringValue(item.Values["assignee_role_key"])),
			FallbackRoleKey:        strings.TrimSpace(stringValue(item.Values["fallback_role_key"])),
			ApproverGroupID:        strings.TrimSpace(stringValue(item.Values["approver_group_id"])),
			ExplicitUserID:         strings.TrimSpace(stringValue(item.Values["explicit_user_id"])),
			CandidateRoleKeys:      splitCSV(item.Values["candidate_role_keys"]),
			DueAfterSeconds:        intValue(item.Values["due_after_seconds"]),
			EscalateAfterSeconds:   intValue(item.Values["escalate_after_seconds"]),
			RequiresDifferentActor: approvalPolicyBoolValue(item.Values["requires_different_actor"]),
		}
		stage.AssigneeUserID, stage.CandidateUserIDs, _ = s.resolveStageUsers(stage)
		stages = append(stages, stage)
	}
	slices.SortFunc(stages, func(a, b ApprovalPolicyStageResolution) int {
		if a.Sequence == b.Sequence {
			return strings.Compare(a.StageKey, b.StageKey)
		}
		return a.Sequence - b.Sequence
	})
	for i := range stages {
		if stages[i].Sequence <= 0 {
			stages[i].Sequence = i + 1
		}
		stages[i].TotalStages = len(stages)
	}
	return stages, nil
}

func (s *ApprovalPolicyService) resolveStageUsers(stage ApprovalPolicyStageResolution) (string, []string, error) {
	switch strings.TrimSpace(stage.AssignmentStrategy) {
	case "explicit_user":
		return stage.ExplicitUserID, nil, nil
	case "approver_group", "fallback_role_group":
		if stage.ApproverGroupID == "" {
			return "", nil, nil
		}
		items, _, err := s.models.List("approver_group_member", model.Query{
			Filters:  map[string]string{"approver_group_id": stage.ApproverGroupID},
			SortKey:  "user_id",
			Page:     1,
			PageSize: model.MaxPageSize,
		})
		if err != nil {
			return "", nil, err
		}
		candidates := make([]string, 0, len(items))
		for _, item := range items {
			if !strings.EqualFold(stringValue(item.Values["status"]), "active") {
				continue
			}
			userID := strings.TrimSpace(stringValue(item.Values["user_id"]))
			if userID != "" {
				candidates = append(candidates, userID)
			}
		}
		candidates = uniqueStrings(candidates)
		if len(candidates) == 1 {
			return candidates[0], nil, nil
		}
		return "", candidates, nil
	default:
		return "", nil, nil
	}
}

func approvalPolicyMatchScore(item model.Record, record document.Record, action, workflowKey string) (int, bool) {
	if !strings.EqualFold(strings.TrimSpace(stringValue(item.Values["status"])), "active") {
		return 0, false
	}
	score := 0
	if !matchesWildcardField(item.Values["document_type"], record.Header.Type, &score) {
		return 0, false
	}
	if !matchesWildcardField(item.Values["workflow_key"], firstNonEmpty(strings.TrimSpace(workflowKey), stringValue(record.Header.Metadata["workflow_key"])), &score) {
		return 0, false
	}
	if !matchesWildcardField(item.Values["action"], action, &score) {
		return 0, false
	}
	if !matchesScopedField(item.Values["organization_id"], record.Header.OrganizationID, &score) {
		return 0, false
	}
	if !matchesScopedField(item.Values["location_id"], record.Header.LocationID, &score) {
		return 0, false
	}
	if !matchesScopedField(item.Values["operating_unit_id"], firstNonEmpty(contextValue(record, "operating_unit_id"), stringValue(record.Header.Metadata["operating_unit_id"])), &score) {
		return 0, false
	}
	if !matchesScopedField(item.Values["department_id"], contextValue(record, "department_id"), &score) {
		return 0, false
	}
	if !matchesScopedField(item.Values["cost_center_id"], contextValue(record, "cost_center_id"), &score) {
		return 0, false
	}
	amountMinor := record.Header.TotalAmount.AmountMinor
	minimum := int64Value(item.Values["minimum_amount_minor"])
	maximum := int64Value(item.Values["maximum_amount_minor"])
	if minimum > 0 {
		if amountMinor < minimum {
			return 0, false
		}
		score++
	}
	if maximum > 0 {
		if amountMinor > maximum {
			return 0, false
		}
		score++
	}
	score += intValue(item.Values["priority"]) * 100
	return score, true
}

func approvalPolicyFallbackStage(policy model.Record) ApprovalPolicyStageResolution {
	stage := ApprovalPolicyStageResolution{
		PolicyID:               policy.ID,
		StageKey:               firstNonEmpty(strings.TrimSpace(stringValue(policy.Values["default_stage_key"])), "approval"),
		Sequence:               1,
		TotalStages:            1,
		RequiredApproverCount:  1,
		RoutingMode:            strings.TrimSpace(stringValue(policy.Values["routing_mode"])),
		AssignmentStrategy:     strings.TrimSpace(stringValue(policy.Values["assignment_strategy"])),
		AssignmentMode:         strings.TrimSpace(stringValue(policy.Values["assignment_mode"])),
		AssigneeRoleKey:        strings.TrimSpace(stringValue(policy.Values["assignee_role_key"])),
		FallbackRoleKey:        strings.TrimSpace(stringValue(policy.Values["fallback_role_key"])),
		ApproverGroupID:        strings.TrimSpace(stringValue(policy.Values["approver_group_id"])),
		ExplicitUserID:         strings.TrimSpace(stringValue(policy.Values["explicit_user_id"])),
		CandidateRoleKeys:      splitCSV(policy.Values["candidate_role_keys"]),
		DueAfterSeconds:        intValue(policy.Values["due_after_seconds"]),
		EscalateAfterSeconds:   intValue(policy.Values["escalate_after_seconds"]),
		RequiresDifferentActor: approvalPolicyBoolValue(policy.Values["requires_different_actor"]),
	}
	stage.AssigneeUserID = stage.ExplicitUserID
	return stage
}

func matchesWildcardField(raw any, expected string, score *int) bool {
	value := strings.TrimSpace(stringValue(raw))
	expected = strings.TrimSpace(expected)
	if value == "" || value == "*" {
		return true
	}
	if !strings.EqualFold(value, expected) {
		return false
	}
	*score += 4
	return true
}

func matchesScopedField(raw any, expected string, score *int) bool {
	value := strings.TrimSpace(stringValue(raw))
	expected = strings.TrimSpace(expected)
	if value == "" {
		return true
	}
	if !strings.EqualFold(value, expected) {
		return false
	}
	*score += 2
	return true
}

func contextValue(record document.Record, key string) string {
	if record.Body.Payload != nil {
		if text := strings.TrimSpace(stringValue(record.Body.Payload[key])); text != "" {
			return text
		}
	}
	if record.Header.Metadata != nil {
		return strings.TrimSpace(stringValue(record.Header.Metadata[key]))
	}
	return ""
}

func splitCSV(value any) []string {
	switch typed := value.(type) {
	case []string:
		return uniqueStrings(typed)
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(stringValue(item))
			if text != "" {
				items = append(items, text)
			}
		}
		return uniqueStrings(items)
	default:
		text := strings.TrimSpace(stringValue(value))
		if text == "" {
			return nil
		}
		return uniqueStrings(strings.Split(text, ","))
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(typed))
		return n
	default:
		return 0
	}
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return n
	default:
		return 0
	}
}

func approvalPolicyBoolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
