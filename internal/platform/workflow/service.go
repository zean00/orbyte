package workflow

import (
	"fmt"
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
		States: []string{"draft", "submitted", "approved", "rejected"},
		Actions: []ActionRule{
			{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit", TaskType: "review", CreateApproval: true, AssignmentMode: "role_queue", AssigneeRoleKey: "approver", CandidateRoleKeys: []string{"approver"}, ApprovalStageKey: "review", DueAfterSeconds: 24 * 60 * 60, EscalateAfterSeconds: 48 * 60 * 60},
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
	if def.Key == "" {
		return shared.Validation("workflow key is required")
	}
	return s.repo.SaveDefinition(def)
}

func (s *Service) Get(key string) (Definition, error) {
	def, ok := s.repo.GetDefinition(key)
	if !ok {
		return Definition{}, shared.NotFound("workflow definition not found")
	}
	return def, nil
}

func (s *Service) Execute(workflowKey, currentState, action string) (Transition, error) {
	def, ok := s.repo.GetDefinition(workflowKey)
	if !ok {
		return Transition{}, shared.NotFound("workflow definition not found")
	}
	for _, rule := range def.Actions {
		if rule.Action == action && rule.FromState == currentState {
			return Transition{
				WorkflowKey:            workflowKey,
				Action:                 action,
				FromState:              currentState,
				ToState:                rule.ToState,
				PermissionKey:          rule.PermissionKey,
				TaskType:               rule.TaskType,
				CreateApproval:         rule.CreateApproval,
				AssignmentMode:         rule.AssignmentMode,
				AssigneeRoleKey:        rule.AssigneeRoleKey,
				CandidateRoleKeys:      append([]string(nil), rule.CandidateRoleKeys...),
				ApprovalStageKey:       rule.ApprovalStageKey,
				DueAfterSeconds:        rule.DueAfterSeconds,
				EscalateAfterSeconds:   rule.EscalateAfterSeconds,
				RequiresDifferentActor: rule.RequiresDifferentActor,
				StepUpRequired:         rule.StepUpRequired,
			}, nil
		}
	}
	return Transition{}, shared.Conflict("workflow action not allowed from current state")
}

func (s *Service) CreateSideEffects(transition Transition, targetType, targetID string, now time.Time) error {
	return s.ApplyMutation(s.PlanCreateSideEffects(transition, targetType, targetID, "", now))
}

func (s *Service) ListTasks() []Task {
	return s.repo.ListTasks()
}

func (s *Service) ListApprovals() []Approval {
	return s.repo.ListApprovals()
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
		mutation.Tasks = append(mutation.Tasks, Task{
			ID:                fmt.Sprintf("task:%s:%s", targetID, transition.Action),
			WorkflowKey:       transition.WorkflowKey,
			TargetType:        targetType,
			TargetID:          targetID,
			TaskType:          transition.TaskType,
			Status:            "open",
			AssignmentMode:    transition.AssignmentMode,
			AssigneeRoleKey:   transition.AssigneeRoleKey,
			CandidateRoleKeys: append([]string(nil), transition.CandidateRoleKeys...),
			CreatedBy:         actorID,
			CreatedAt:         now,
			DueAt:             dueAt,
			EscalateAt:        escalateAt,
			Metadata:          map[string]any{"action": transition.Action},
		})
	}
	if transition.CreateApproval {
		mutation.Approvals = append(mutation.Approvals, Approval{
			ID:                fmt.Sprintf("approval:%s:%s", targetID, transition.Action),
			WorkflowKey:       transition.WorkflowKey,
			TargetType:        targetType,
			TargetID:          targetID,
			Status:            "pending",
			StageKey:          transition.ApprovalStageKey,
			RequestedBy:       actorID,
			RequestedAt:       now,
			CandidateRoleKeys: append([]string(nil), transition.CandidateRoleKeys...),
			DueAt:             dueAt,
			Metadata: map[string]any{
				"action":                   transition.Action,
				"requires_different_actor": transition.RequiresDifferentActor,
				"step_up_required":         transition.StepUpRequired,
			},
		})
	}
	return mutation
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
	return nil
}
