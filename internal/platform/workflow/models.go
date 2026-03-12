package workflow

import "time"

type Definition struct {
	Key     string       `json:"key"`
	States  []string     `json:"states"`
	Actions []ActionRule `json:"actions"`
}

type ActionRule struct {
	Action         string `json:"action"`
	FromState      string `json:"from_state"`
	ToState        string `json:"to_state"`
	PermissionKey  string `json:"permission_key,omitempty"`
	TaskType       string `json:"task_type,omitempty"`
	CreateApproval bool   `json:"create_approval,omitempty"`
}

type Transition struct {
	WorkflowKey    string `json:"workflow_key"`
	Action         string `json:"action"`
	FromState      string `json:"from_state"`
	ToState        string `json:"to_state"`
	PermissionKey  string `json:"permission_key,omitempty"`
	TaskType       string `json:"task_type,omitempty"`
	CreateApproval bool   `json:"create_approval,omitempty"`
}

type Task struct {
	ID          string    `json:"id"`
	WorkflowKey string    `json:"workflow_key"`
	TargetType  string    `json:"target_type"`
	TargetID    string    `json:"target_id"`
	TaskType    string    `json:"task_type"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type Approval struct {
	ID          string    `json:"id"`
	WorkflowKey string    `json:"workflow_key"`
	TargetType  string    `json:"target_type"`
	TargetID    string    `json:"target_id"`
	Status      string    `json:"status"`
	RequestedAt time.Time `json:"requested_at"`
}

type TaskStatusUpdate struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type ApprovalStatusUpdate struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type Mutation struct {
	Tasks           []Task                 `json:"tasks,omitempty"`
	Approvals       []Approval             `json:"approvals,omitempty"`
	TaskUpdates     []TaskStatusUpdate     `json:"task_updates,omitempty"`
	ApprovalUpdates []ApprovalStatusUpdate `json:"approval_updates,omitempty"`
}
