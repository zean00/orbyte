package workflow

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/shared"
)

type Service struct {
	repo Repository
}

func NewService() *Service {
	svc := NewServiceWithRepository(NewMemoryRepository())
	_ = svc.Register(Definition{
		Key:    "generic_request_flow",
		States: []string{"draft", "submitted", "approved", "rejected", "cancelled"},
		Actions: []ActionRule{
			{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit", TaskType: "review", CreateApproval: true, AssignmentMode: "role_queue", AssigneeRoleKey: "approver", CandidateRoleKeys: []string{"approver"}, ApprovalStageKey: "review", DueAfterSeconds: 24 * 60 * 60, EscalateAfterSeconds: 48 * 60 * 60, LinkMode: "tokenized", LinkTTLSeconds: 24 * 60 * 60, LinkAllowedActions: []string{"approve", "reject"}},
			{Action: "approve", FromState: "submitted", ToState: "approved", PermissionKey: "document.approve"},
			{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
			{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
			{Action: "reopen", FromState: "approved", ToState: "draft", PermissionKey: "document.reopen"},
			{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
			{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
		},
	})
	return svc
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(def Definition) error {
	if err := validateDefinition(def); err != nil {
		return err
	}
	return s.repo.SaveDefinition(def)
}

func (s *Service) RegisterDraft(def Definition) (Definition, error) {
	if err := validateDefinition(def); err != nil {
		return Definition{}, err
	}
	return s.repo.SaveDefinitionDraft(def)
}

func (s *Service) Delete(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return shared.Validation("workflow key is required")
	}
	if _, ok := s.repo.GetDefinition(key); !ok {
		return shared.NotFound("workflow definition not found")
	}
	return s.repo.DeleteDefinition(key)
}

func (s *Service) Get(key string) (Definition, error) {
	def, ok := s.repo.GetDefinition(key)
	if !ok {
		return Definition{}, shared.NotFound("workflow definition not found")
	}
	return def, nil
}

func (s *Service) GetVersion(key string, version int) (Definition, error) {
	def, ok := s.repo.GetDefinitionVersion(key, version)
	if !ok {
		return Definition{}, shared.NotFound("workflow definition not found")
	}
	return def, nil
}

func (s *Service) ListDefinitions() []Definition {
	return s.repo.ListDefinitions()
}

func (s *Service) ListVersions(key string) []Definition {
	return s.repo.ListDefinitionVersions(key)
}

func (s *Service) CreateDraft(key, actorID string) (Definition, error) {
	return s.repo.CreateDraft(strings.TrimSpace(key), strings.TrimSpace(actorID))
}

func (s *Service) SaveDraft(def Definition, actorID string) (Definition, error) {
	if strings.TrimSpace(def.Status) == "" {
		def.Status = "draft"
	}
	if def.Status != "draft" {
		return Definition{}, shared.Validation("only workflow drafts may be updated")
	}
	if err := validateDefinition(def); err != nil {
		return Definition{}, err
	}
	return s.repo.SaveDraft(def, actorID)
}

func (s *Service) Publish(key string, version int, actorID string) (Definition, error) {
	def, ok := s.repo.GetDefinitionVersion(key, version)
	if !ok {
		return Definition{}, shared.NotFound("workflow draft not found")
	}
	if result := s.Validate(def); !result.Valid {
		return Definition{}, shared.Validation(strings.Join(result.Issues, "; "))
	}
	return s.repo.PublishDefinition(strings.TrimSpace(key), version, strings.TrimSpace(actorID))
}

func (s *Service) Validate(def Definition) ValidateResult {
	issues := validateIssues(def)
	return ValidateResult{Valid: len(issues) == 0, Issues: issues}
}

func (s *Service) Execute(workflowKey, currentState, action string) (Transition, error) {
	def, ok := s.repo.GetDefinition(workflowKey)
	if !ok {
		return Transition{}, shared.NotFound("workflow definition not found")
	}
	return transitionForDefinition(def, currentState, action)
}

func (s *Service) ExecuteVersion(workflowKey string, version int, currentState, action string) (Transition, error) {
	if version <= 0 {
		return s.Execute(workflowKey, currentState, action)
	}
	def, ok := s.repo.GetDefinitionVersion(workflowKey, version)
	if !ok {
		return Transition{}, shared.NotFound("workflow definition not found")
	}
	return transitionForDefinition(def, currentState, action)
}

func (s *Service) Simulate(def Definition, input SimulationInput) SimulationResult {
	result := SimulationResult{}
	if validation := s.Validate(def); !validation.Valid {
		result.Valid = false
		result.Issues = validation.Issues
		return result
	}
	transition, err := transitionForDefinition(def, input.CurrentState, input.Action)
	if err != nil {
		result.Valid = false
		result.Issues = []string{err.Error()}
		return result
	}
	now := time.Now().UTC()
	mutation := s.PlanCreateSideEffects(transition, "document", firstNonEmpty(input.DocumentID, "simulation"), input.ActorID, now)
	result.Valid = true
	result.Transition = transition
	if len(mutation.Tasks) > 0 {
		task := mutation.Tasks[0]
		result.PlannedTask = &task
		result.AppliedAssignments = cloneMap(task.Metadata)
	}
	if len(mutation.Approvals) > 0 {
		approval := mutation.Approvals[0]
		result.PlannedApproval = &approval
	}
	return result
}

func (s *Service) CreateSideEffects(transition Transition, targetType, targetID string, now time.Time) error {
	return s.ApplyMutation(s.PlanCreateSideEffects(transition, targetType, targetID, "", now))
}

func (s *Service) ListTasks() []Task {
	return s.repo.ListTasks()
}

func (s *Service) Task(id string) (Task, bool) {
	id = strings.TrimSpace(id)
	for _, item := range s.repo.ListTasks() {
		if item.ID == id {
			return item, true
		}
	}
	return Task{}, false
}

func (s *Service) ListApprovals() []Approval {
	return s.repo.ListApprovals()
}

func (s *Service) Approval(id string) (Approval, bool) {
	id = strings.TrimSpace(id)
	for _, item := range s.repo.ListApprovals() {
		if item.ID == id {
			return item, true
		}
	}
	return Approval{}, false
}

func (s *Service) ListHistory(targetType, targetID string) []HistoryEvent {
	return s.repo.ListHistory(strings.TrimSpace(targetType), strings.TrimSpace(targetID))
}

func (s *Service) ResolveApproval(targetID string) error {
	return s.ApplyMutation(s.PlanResolveArtifacts(targetID, "approved", "completed", "", time.Now().UTC()))
}

func (s *Service) ResolveArtifacts(targetID, approvalStatus, taskStatus string) error {
	return s.ApplyMutation(s.PlanResolveArtifacts(targetID, approvalStatus, taskStatus, "", time.Now().UTC()))
}

func (s *Service) PlanCreateSideEffects(transition Transition, targetType, targetID, actorID string, now time.Time) Mutation {
	mutation := Mutation{}
	var dueAt time.Time
	if transition.DueAfterSeconds > 0 {
		dueAt = now.Add(time.Duration(transition.DueAfterSeconds) * time.Second)
	}
	var escalateAt time.Time
	if transition.EscalateAfterSeconds > 0 {
		escalateAt = now.Add(time.Duration(transition.EscalateAfterSeconds) * time.Second)
	}
	if transition.TaskType != "" {
		taskMetadata := map[string]any{"action": transition.Action}
		mergeMetadata(taskMetadata, transition.Metadata)
		if len(transition.CandidateUserIDs) > 0 {
			taskMetadata["candidate_user_ids"] = append([]string(nil), transition.CandidateUserIDs...)
		}
		mutation.Tasks = append(mutation.Tasks, Task{
			ID:                fmt.Sprintf("task:%s:%s", targetID, transition.Action),
			WorkflowKey:       transition.WorkflowKey,
			WorkflowVersion:   transition.WorkflowVersion,
			TargetType:        targetType,
			TargetID:          targetID,
			TaskType:          transition.TaskType,
			Status:            "open",
			AssignmentMode:    transition.AssignmentMode,
			AssigneeUserID:    transition.AssigneeUserID,
			AssigneeRoleKey:   transition.AssigneeRoleKey,
			CandidateRoleKeys: append([]string(nil), transition.CandidateRoleKeys...),
			CreatedBy:         actorID,
			CreatedAt:         now,
			DueAt:             dueAt,
			EscalateAt:        escalateAt,
			Metadata:          taskMetadata,
		})
	}
	if transition.CreateApproval {
		allowedActions := append([]string(nil), transition.LinkAllowedActions...)
		if len(allowedActions) == 0 && !transition.LinkReviewOnly {
			allowedActions = []string{"approve", "reject"}
		}
		approvalMetadata := map[string]any{
			"action":                   transition.Action,
			"requires_different_actor": transition.RequiresDifferentActor,
			"step_up_required":         transition.StepUpRequired,
			"link_mode":                strings.TrimSpace(transition.LinkMode),
			"link_ttl_seconds":         transition.LinkTTLSeconds,
			"link_review_only":         transition.LinkReviewOnly,
			"link_require_step_up":     transition.LinkRequireStepUp,
			"link_allowed_actions":     allowedActions,
		}
		if transition.TaskType != "" {
			approvalMetadata["task_type"] = transition.TaskType
		}
		mergeMetadata(approvalMetadata, transition.Metadata)
		if transition.AssigneeUserID != "" {
			approvalMetadata["assignee_user_id"] = transition.AssigneeUserID
		}
		if len(transition.CandidateUserIDs) > 0 {
			approvalMetadata["candidate_user_ids"] = append([]string(nil), transition.CandidateUserIDs...)
		}
		mutation.Approvals = append(mutation.Approvals, Approval{
			ID:                fmt.Sprintf("approval:%s:%s", targetID, transition.Action),
			WorkflowKey:       transition.WorkflowKey,
			WorkflowVersion:   transition.WorkflowVersion,
			TargetType:        targetType,
			TargetID:          targetID,
			Status:            "pending",
			StageKey:          transition.ApprovalStageKey,
			RequestedBy:       actorID,
			RequestedAt:       now,
			CandidateRoleKeys: append([]string(nil), transition.CandidateRoleKeys...),
			DueAt:             dueAt,
			Metadata:          approvalMetadata,
		})
	}
	return mutation
}

func mergeMetadata(dst map[string]any, src map[string]any) {
	if dst == nil || len(src) == 0 {
		return
	}
	for key, value := range src {
		dst[key] = value
	}
}

func (s *Service) PlanResolveArtifacts(targetID, approvalStatus, taskStatus, actorID string, now time.Time) Mutation {
	mutation := Mutation{}
	for _, approval := range s.repo.ListApprovals() {
		if approval.TargetID == targetID && approval.Status == "pending" {
			mutation.ApprovalUpdates = append(mutation.ApprovalUpdates, ApprovalStatusUpdate{ID: approval.ID, Status: approvalStatus, ResolvedBy: actorID, ResolvedAt: now})
		}
	}
	for _, task := range s.repo.ListTasks() {
		if task.TargetID == targetID && task.Status == "open" {
			mutation.TaskUpdates = append(mutation.TaskUpdates, TaskStatusUpdate{ID: task.ID, Status: taskStatus, ResolvedBy: actorID, ResolvedAt: now})
		}
	}
	return mutation
}

func (s *Service) ApplyMutation(mutation Mutation) error {
	for _, task := range mutation.Tasks {
		if err := s.repo.SaveTask(task); err != nil {
			return err
		}
	}
	for _, approval := range mutation.Approvals {
		if err := s.repo.SaveApproval(approval); err != nil {
			return err
		}
	}
	for _, update := range mutation.ApprovalUpdates {
		if err := s.repo.UpdateApprovalStatus(update); err != nil {
			return err
		}
	}
	for _, update := range mutation.TaskUpdates {
		if err := s.repo.UpdateTaskStatus(update); err != nil {
			return err
		}
	}
	for _, event := range mutation.History {
		if err := s.repo.SaveHistory(event); err != nil {
			return err
		}
	}
	return nil
}

func validateDefinition(def Definition) error {
	if issues := validateIssues(def); len(issues) > 0 {
		return shared.Validation(strings.Join(issues, "; "))
	}
	return nil
}

func validateIssues(def Definition) []string {
	issues := []string{}
	if strings.TrimSpace(def.Key) == "" {
		issues = append(issues, "workflow key is required")
	}
	if len(def.States) == 0 {
		issues = append(issues, "workflow must define at least one state")
	}
	if len(def.Actions) == 0 {
		issues = append(issues, "workflow must define at least one action")
	}
	stateSet := map[string]bool{}
	for _, state := range def.States {
		state = strings.TrimSpace(state)
		if state == "" {
			issues = append(issues, "workflow state may not be blank")
			continue
		}
		stateSet[state] = true
	}
	for _, action := range def.Actions {
		if strings.TrimSpace(action.Action) == "" {
			issues = append(issues, "workflow action is required")
		}
		if !stateSet[action.FromState] {
			issues = append(issues, fmt.Sprintf("unknown from_state %q", action.FromState))
		}
		if !stateSet[action.ToState] {
			issues = append(issues, fmt.Sprintf("unknown to_state %q", action.ToState))
		}
	}
	return issues
}

func transitionForDefinition(def Definition, currentState, action string) (Transition, error) {
	for _, rule := range def.Actions {
		if rule.Action == action && rule.FromState == currentState {
			return Transition{
				WorkflowKey:            def.Key,
				WorkflowVersion:        def.Version,
				Action:                 action,
				FromState:              currentState,
				ToState:                rule.ToState,
				PermissionKey:          rule.PermissionKey,
				TaskType:               rule.TaskType,
				CreateApproval:         rule.CreateApproval,
				AssignmentStrategy:     rule.AssignmentStrategy,
				AssignmentMode:         rule.AssignmentMode,
				AssigneeRoleKey:        rule.AssigneeRoleKey,
				CandidateRoleKeys:      append([]string(nil), rule.CandidateRoleKeys...),
				FallbackRoleKey:        rule.FallbackRoleKey,
				ApprovalStageKey:       rule.ApprovalStageKey,
				DueAfterSeconds:        rule.DueAfterSeconds,
				EscalateAfterSeconds:   rule.EscalateAfterSeconds,
				RequiresDifferentActor: rule.RequiresDifferentActor,
				StepUpRequired:         rule.StepUpRequired,
				LinkMode:               rule.LinkMode,
				LinkTTLSeconds:         rule.LinkTTLSeconds,
				LinkReviewOnly:         rule.LinkReviewOnly,
				LinkRequireStepUp:      rule.LinkRequireStepUp,
				LinkAllowedActions:     append([]string(nil), rule.LinkAllowedActions...),
				Metadata:               cloneMap(rule.Metadata),
			}, nil
		}
	}
	return Transition{}, shared.Conflict("workflow action not allowed from current state")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
