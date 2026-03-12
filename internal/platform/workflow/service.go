package workflow

import (
	"fmt"
	"time"

	"clinic/internal/platform/shared"
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
			{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit", TaskType: "review", CreateApproval: true},
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
				WorkflowKey:    workflowKey,
				Action:         action,
				FromState:      currentState,
				ToState:        rule.ToState,
				PermissionKey:  rule.PermissionKey,
				TaskType:       rule.TaskType,
				CreateApproval: rule.CreateApproval,
			}, nil
		}
	}
	return Transition{}, shared.Conflict("workflow action not allowed from current state")
}

func (s *Service) CreateSideEffects(transition Transition, targetType, targetID string, now time.Time) error {
	return s.ApplyMutation(s.PlanCreateSideEffects(transition, targetType, targetID, now))
}

func (s *Service) ListTasks() []Task {
	return s.repo.ListTasks()
}

func (s *Service) ListApprovals() []Approval {
	return s.repo.ListApprovals()
}

func (s *Service) ResolveApproval(targetID string) error {
	return s.ApplyMutation(s.PlanResolveArtifacts(targetID, "approved", "completed"))
}

func (s *Service) ResolveArtifacts(targetID, approvalStatus, taskStatus string) error {
	return s.ApplyMutation(s.PlanResolveArtifacts(targetID, approvalStatus, taskStatus))
}

func (s *Service) PlanCreateSideEffects(transition Transition, targetType, targetID string, now time.Time) Mutation {
	mutation := Mutation{}
	if transition.TaskType != "" {
		mutation.Tasks = append(mutation.Tasks, Task{
			ID:          fmt.Sprintf("task:%s:%s", targetID, transition.Action),
			WorkflowKey: transition.WorkflowKey,
			TargetType:  targetType,
			TargetID:    targetID,
			TaskType:    transition.TaskType,
			Status:      "open",
			CreatedAt:   now,
		})
	}
	if transition.CreateApproval {
		mutation.Approvals = append(mutation.Approvals, Approval{
			ID:          fmt.Sprintf("approval:%s:%s", targetID, transition.Action),
			WorkflowKey: transition.WorkflowKey,
			TargetType:  targetType,
			TargetID:    targetID,
			Status:      "pending",
			RequestedAt: now,
		})
	}
	return mutation
}

func (s *Service) PlanResolveArtifacts(targetID, approvalStatus, taskStatus string) Mutation {
	mutation := Mutation{}
	for _, approval := range s.repo.ListApprovals() {
		if approval.TargetID == targetID && approval.Status == "pending" {
			mutation.ApprovalUpdates = append(mutation.ApprovalUpdates, ApprovalStatusUpdate{ID: approval.ID, Status: approvalStatus})
		}
	}
	for _, task := range s.repo.ListTasks() {
		if task.TargetID == targetID && task.Status == "open" {
			mutation.TaskUpdates = append(mutation.TaskUpdates, TaskStatusUpdate{ID: task.ID, Status: taskStatus})
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
		if err := s.repo.UpdateApprovalStatus(update.ID, update.Status); err != nil {
			return err
		}
	}
	for _, update := range mutation.TaskUpdates {
		if err := s.repo.UpdateTaskStatus(update.ID, update.Status); err != nil {
			return err
		}
	}
	return nil
}
