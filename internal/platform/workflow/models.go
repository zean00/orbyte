package workflow

import "time"

type Definition struct {
	Key     string       `json:"key"`
	States  []string     `json:"states"`
	Actions []ActionRule `json:"actions"`
}

type ActionRule struct {
	Action                   string   `json:"action"`
	FromState                string   `json:"from_state"`
	ToState                  string   `json:"to_state"`
	PermissionKey            string   `json:"permission_key,omitempty"`
	TaskType                 string   `json:"task_type,omitempty"`
	CreateApproval           bool     `json:"create_approval,omitempty"`
	AssignmentMode           string   `json:"assignment_mode,omitempty"`
	AssigneeRoleKey          string   `json:"assignee_role_key,omitempty"`
	CandidateRoleKeys        []string `json:"candidate_role_keys,omitempty"`
	ApprovalStageKey         string   `json:"approval_stage_key,omitempty"`
	DueAfterSeconds          int      `json:"due_after_seconds,omitempty"`
	EscalateAfterSeconds     int      `json:"escalate_after_seconds,omitempty"`
	RequiresDifferentActor   bool     `json:"requires_different_actor,omitempty"`
	StepUpRequired           bool     `json:"step_up_required,omitempty"`
}

type Transition struct {
	WorkflowKey              string   `json:"workflow_key"`
	Action                   string   `json:"action"`
	FromState                string   `json:"from_state"`
	ToState                  string   `json:"to_state"`
	PermissionKey            string   `json:"permission_key,omitempty"`
	TaskType                 string   `json:"task_type,omitempty"`
	CreateApproval           bool     `json:"create_approval,omitempty"`
	AssignmentMode           string   `json:"assignment_mode,omitempty"`
	AssigneeRoleKey          string   `json:"assignee_role_key,omitempty"`
	CandidateRoleKeys        []string `json:"candidate_role_keys,omitempty"`
	ApprovalStageKey         string   `json:"approval_stage_key,omitempty"`
	DueAfterSeconds          int      `json:"due_after_seconds,omitempty"`
	EscalateAfterSeconds     int      `json:"escalate_after_seconds,omitempty"`
	RequiresDifferentActor   bool     `json:"requires_different_actor,omitempty"`
	StepUpRequired           bool     `json:"step_up_required,omitempty"`
}

type Task struct {
	ID                string         `json:"id"`
	WorkflowKey       string         `json:"workflow_key"`
	TargetType        string         `json:"target_type"`
	TargetID          string         `json:"target_id"`
	TaskType          string         `json:"task_type"`
	Status            string         `json:"status"`
	AssignmentMode    string         `json:"assignment_mode,omitempty"`
	AssigneeUserID    string         `json:"assignee_user_id,omitempty"`
	AssigneeRoleKey   string         `json:"assignee_role_key,omitempty"`
	CandidateRoleKeys []string       `json:"candidate_role_keys,omitempty"`
	CreatedBy         string         `json:"created_by,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	DueAt             time.Time      `json:"due_at,omitempty"`
	EscalateAt        time.Time      `json:"escalate_at,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type Approval struct {
	ID                string         `json:"id"`
	WorkflowKey       string         `json:"workflow_key"`
	TargetType        string         `json:"target_type"`
	TargetID          string         `json:"target_id"`
	Status            string         `json:"status"`
	StageKey          string         `json:"stage_key,omitempty"`
	RequestedBy       string         `json:"requested_by,omitempty"`
	RequestedAt       time.Time      `json:"requested_at"`
	ResolvedBy        string         `json:"resolved_by,omitempty"`
	ResolvedAt        time.Time      `json:"resolved_at,omitempty"`
	CandidateRoleKeys []string       `json:"candidate_role_keys,omitempty"`
	DueAt             time.Time      `json:"due_at,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type TaskStatusUpdate struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	ResolvedBy string    `json:"resolved_by,omitempty"`
	ResolvedAt time.Time `json:"resolved_at,omitempty"`
}

type ApprovalStatusUpdate struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	ResolvedBy string    `json:"resolved_by,omitempty"`
	ResolvedAt time.Time `json:"resolved_at,omitempty"`
}

type Mutation struct {
	Tasks           []Task                 `json:"tasks,omitempty"`
	Approvals       []Approval             `json:"approvals,omitempty"`
	TaskUpdates     []TaskStatusUpdate     `json:"task_updates,omitempty"`
	ApprovalUpdates []ApprovalStatusUpdate `json:"approval_updates,omitempty"`
}
