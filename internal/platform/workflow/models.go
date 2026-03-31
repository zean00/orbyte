package workflow

import "time"

type Definition struct {
	Key         string       `json:"key"`
	Version     int          `json:"version,omitempty"`
	Status      string       `json:"status,omitempty"`
	States      []string     `json:"states"`
	Actions     []ActionRule `json:"actions"`
	CreatedAt   time.Time    `json:"created_at,omitempty"`
	UpdatedAt   time.Time    `json:"updated_at,omitempty"`
	UpdatedBy   string       `json:"updated_by,omitempty"`
	PublishedAt time.Time    `json:"published_at,omitempty"`
	PublishedBy string       `json:"published_by,omitempty"`
}

type ActionRule struct {
	Action                 string         `json:"action"`
	FromState              string         `json:"from_state"`
	ToState                string         `json:"to_state"`
	PermissionKey          string         `json:"permission_key,omitempty"`
	TaskType               string         `json:"task_type,omitempty"`
	CreateApproval         bool           `json:"create_approval,omitempty"`
	AssignmentStrategy     string         `json:"assignment_strategy,omitempty"`
	AssignmentMode         string         `json:"assignment_mode,omitempty"`
	AssigneeRoleKey        string         `json:"assignee_role_key,omitempty"`
	CandidateRoleKeys      []string       `json:"candidate_role_keys,omitempty"`
	FallbackRoleKey        string         `json:"fallback_role_key,omitempty"`
	ApprovalStageKey       string         `json:"approval_stage_key,omitempty"`
	DueAfterSeconds        int            `json:"due_after_seconds,omitempty"`
	EscalateAfterSeconds   int            `json:"escalate_after_seconds,omitempty"`
	RequiresDifferentActor bool           `json:"requires_different_actor,omitempty"`
	StepUpRequired         bool           `json:"step_up_required,omitempty"`
	LinkMode               string         `json:"link_mode,omitempty"`
	LinkTTLSeconds         int            `json:"link_ttl_seconds,omitempty"`
	LinkReviewOnly         bool           `json:"link_review_only,omitempty"`
	LinkRequireStepUp      bool           `json:"link_require_step_up,omitempty"`
	LinkAllowedActions     []string       `json:"link_allowed_actions,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
}

type Transition struct {
	WorkflowKey            string         `json:"workflow_key"`
	WorkflowVersion        int            `json:"workflow_version,omitempty"`
	Action                 string         `json:"action"`
	FromState              string         `json:"from_state"`
	ToState                string         `json:"to_state"`
	PermissionKey          string         `json:"permission_key,omitempty"`
	TaskType               string         `json:"task_type,omitempty"`
	CreateApproval         bool           `json:"create_approval,omitempty"`
	AssignmentStrategy     string         `json:"assignment_strategy,omitempty"`
	AssignmentMode         string         `json:"assignment_mode,omitempty"`
	AssigneeRoleKey        string         `json:"assignee_role_key,omitempty"`
	CandidateRoleKeys      []string       `json:"candidate_role_keys,omitempty"`
	FallbackRoleKey        string         `json:"fallback_role_key,omitempty"`
	ApprovalStageKey       string         `json:"approval_stage_key,omitempty"`
	DueAfterSeconds        int            `json:"due_after_seconds,omitempty"`
	EscalateAfterSeconds   int            `json:"escalate_after_seconds,omitempty"`
	RequiresDifferentActor bool           `json:"requires_different_actor,omitempty"`
	StepUpRequired         bool           `json:"step_up_required,omitempty"`
	LinkMode               string         `json:"link_mode,omitempty"`
	LinkTTLSeconds         int            `json:"link_ttl_seconds,omitempty"`
	LinkReviewOnly         bool           `json:"link_review_only,omitempty"`
	LinkRequireStepUp      bool           `json:"link_require_step_up,omitempty"`
	LinkAllowedActions     []string       `json:"link_allowed_actions,omitempty"`
	AssigneeUserID         string         `json:"assignee_user_id,omitempty"`
	CandidateUserIDs       []string       `json:"candidate_user_ids,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
}

type Task struct {
	ID                string         `json:"id"`
	WorkflowKey       string         `json:"workflow_key"`
	WorkflowVersion   int            `json:"workflow_version,omitempty"`
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
	WorkflowVersion   int            `json:"workflow_version,omitempty"`
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
	History         []HistoryEvent         `json:"history,omitempty"`
}

type HistoryEvent struct {
	ID                string         `json:"id"`
	WorkflowKey       string         `json:"workflow_key"`
	WorkflowVersion   int            `json:"workflow_version,omitempty"`
	TargetType        string         `json:"target_type"`
	TargetID          string         `json:"target_id"`
	Action            string         `json:"action"`
	FromState         string         `json:"from_state,omitempty"`
	ToState           string         `json:"to_state,omitempty"`
	ActorID           string         `json:"actor_id,omitempty"`
	OccurredAt        time.Time      `json:"occurred_at"`
	DecisionCode      string         `json:"decision_code,omitempty"`
	DecisionReason    string         `json:"decision_reason,omitempty"`
	AssignmentSummary map[string]any `json:"assignment_summary,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type ValidateResult struct {
	Valid  bool     `json:"valid"`
	Issues []string `json:"issues,omitempty"`
}

type SimulationInput struct {
	CurrentState    string         `json:"current_state"`
	Action          string         `json:"action"`
	ActorID         string         `json:"actor_id,omitempty"`
	OrganizationID  string         `json:"organization_id,omitempty"`
	LocationID      string         `json:"location_id,omitempty"`
	DocumentID      string         `json:"document_id,omitempty"`
	DocumentType    string         `json:"document_type,omitempty"`
	Rule            map[string]any `json:"rule,omitempty"`
	AdditionalInput map[string]any `json:"additional_input,omitempty"`
}

type SimulationResult struct {
	Valid              bool           `json:"valid"`
	Issues             []string       `json:"issues,omitempty"`
	Transition         Transition     `json:"transition,omitempty"`
	TransitionDecision map[string]any `json:"transition_decision,omitempty"`
	AssignmentDecision map[string]any `json:"assignment_decision,omitempty"`
	SLADecision        map[string]any `json:"sla_decision,omitempty"`
	PlannedTask        *Task          `json:"planned_task,omitempty"`
	PlannedApproval    *Approval      `json:"planned_approval,omitempty"`
	AppliedRule        map[string]any `json:"applied_rule,omitempty"`
	AppliedAssignments map[string]any `json:"applied_assignments,omitempty"`
	AppliedSLA         map[string]any `json:"applied_sla,omitempty"`
}
