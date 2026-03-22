package httpx

import (
	"strings"
	"time"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func workflowRoutingPreview(ident *identity.Service, def workflow.Definition, input workflow.SimulationInput) map[string]any {
	preview := map[string]any{
		"valid": false,
	}
	if ident == nil {
		preview["error"] = "identity service unavailable"
		return preview
	}
	transition, err := workflowTransitionForAdmin(def, input.CurrentState, input.Action)
	if err != nil {
		preview["error"] = err.Error()
		return preview
	}
	requesterUserID := stringMapValue(input.AdditionalInput, "requester_user_id")
	if requesterUserID == "" {
		requesterUserID = strings.TrimSpace(input.ActorID)
	}
	previousApproverID := stringMapValue(input.AdditionalInput, "previous_approver_id")
	sourceUserID := ""
	switch transition.AssignmentStrategy {
	case "requester_manager":
		sourceUserID = requesterUserID
	case "previous_approver_manager":
		sourceUserID = adminFirstNonEmpty(previousApproverID, input.ActorID)
	}
	preview["transition"] = transition
	preview["source_user_id"] = sourceUserID
	preview["valid"] = true
	switch transition.AssignmentStrategy {
	case "", "static_user", "static_role":
		preview["mode"] = adminFirstNonEmpty(transition.AssignmentMode, "static")
		preview["fallback_role_key"] = transition.FallbackRoleKey
		preview["candidate_role_keys"] = append([]string(nil), transition.CandidateRoleKeys...)
	case "requester_manager", "previous_approver_manager":
		if sourceUserID == "" {
			preview["valid"] = false
			preview["error"] = "source user is required for manager routing"
			return preview
		}
		if resolution, ok := ident.ResolveManager(sourceUserID, input.OrganizationID, input.LocationID, "", time.Now().UTC()); ok {
			preview["resolved_via"] = resolution.Via
			preview["resolved_assignee_user_id"] = resolution.Manager.ID
			preview["resolved_assignee_username"] = resolution.Manager.Username
			preview["line"] = resolution.Line
			return preview
		}
		preview["fallback_used"] = true
		fallthrough
	case "role_fallback":
		roleKey := adminFirstNonEmpty(transition.FallbackRoleKey, transition.AssigneeRoleKey)
		preview["fallback_role_key"] = roleKey
		candidates := ident.ResolveRoleCandidates(roleKey, input.OrganizationID, input.LocationID, "", time.Now().UTC())
		if len(candidates) == 0 {
			preview["valid"] = false
			preview["error"] = "no matching assignee candidates"
			return preview
		}
		if len(candidates) == 1 {
			preview["resolved_via"] = "fallback_role"
			preview["resolved_assignee_user_id"] = candidates[0].ID
			preview["resolved_assignee_username"] = candidates[0].Username
			return preview
		}
		preview["resolved_via"] = "fallback_role"
		preview["resolved_candidate_user_ids"] = mapUsers(candidates, func(item identity.User) string { return item.ID })
		preview["resolved_candidate_usernames"] = mapUsers(candidates, func(item identity.User) string { return item.Username })
		return preview
	default:
		preview["valid"] = false
		preview["error"] = "unsupported assignment strategy"
	}
	return preview
}

func workflowPolicyRuntimeIssues(policySvc *policy.Service, _ workflow.Definition, organizationID, locationID string) []string {
	if policySvc == nil {
		return nil
	}
	requiredHooks := []string{
		"documents.workflow.transition",
		"documents.workflow.assignment",
		"documents.workflow.sla",
	}
	issues := make([]string, 0, len(requiredHooks))
	for _, hookKey := range requiredHooks {
		runtime, ok := policySvc.Runtime(hookKey, organizationID, locationID)
		if !ok {
			issues = append(issues, "policy hook "+hookKey+" is not registered")
			continue
		}
		if runtime.Engine == policy.EngineRego {
			if !runtime.CompileValid {
				issues = append(issues, "policy hook "+hookKey+" compile invalid: "+firstNonEmpty(runtime.CompileError, "rego source is not configured"))
				continue
			}
			if !runtime.EvalValid {
				issues = append(issues, "policy hook "+hookKey+" runtime invalid: "+firstNonEmpty(runtime.EvalError, "rego policy must return a decision object"))
			}
			continue
		}
		if !runtime.EvalValid {
			issues = append(issues, "policy hook "+hookKey+" runtime invalid: "+firstNonEmpty(runtime.EvalError, "policy evaluator is not configured"))
		}
	}
	return issues
}

func workflowTransitionForAdmin(def workflow.Definition, currentState, action string) (workflow.Transition, error) {
	for _, rule := range def.Actions {
		if rule.Action == action && rule.FromState == currentState {
			return workflow.Transition{
				WorkflowKey:          def.Key,
				WorkflowVersion:      def.Version,
				Action:               rule.Action,
				FromState:            rule.FromState,
				ToState:              rule.ToState,
				PermissionKey:        rule.PermissionKey,
				TaskType:             rule.TaskType,
				CreateApproval:       rule.CreateApproval,
				AssignmentStrategy:   rule.AssignmentStrategy,
				AssignmentMode:       rule.AssignmentMode,
				AssigneeRoleKey:      rule.AssigneeRoleKey,
				CandidateRoleKeys:    append([]string(nil), rule.CandidateRoleKeys...),
				FallbackRoleKey:      rule.FallbackRoleKey,
				ApprovalStageKey:     rule.ApprovalStageKey,
				DueAfterSeconds:      rule.DueAfterSeconds,
				EscalateAfterSeconds: rule.EscalateAfterSeconds,
			}, nil
		}
	}
	return workflow.Transition{}, shared.Conflict("workflow action not allowed from current state")
}

func stringMapValue(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func adminFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mapUsers[T any](items []identity.User, mapper func(identity.User) T) []T {
	output := make([]T, 0, len(items))
	for _, item := range items {
		output = append(output, mapper(item))
	}
	return output
}

func firstTemplateScope(scopes []string) string {
	if len(scopes) == 0 {
		return "deployment"
	}
	return scopes[0]
}
