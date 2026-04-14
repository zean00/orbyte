package httpx

type uiWorklistTask struct {
	ID              string         `json:"id"`
	WorkflowKey     string         `json:"workflow_key"`
	WorkflowVersion int            `json:"workflow_version,omitempty"`
	TargetType      string         `json:"target_type"`
	TargetID        string         `json:"target_id"`
	DocumentType    string         `json:"document_type,omitempty"`
	TargetTitle     string         `json:"target_title,omitempty"`
	TargetNumber    string         `json:"target_number,omitempty"`
	TargetStatus    string         `json:"target_status,omitempty"`
	TargetUpdatedAt string         `json:"target_updated_at,omitempty"`
	OpenPath        string         `json:"open_path,omitempty"`
	TaskType        string         `json:"task_type"`
	Status          string         `json:"status"`
	AssignmentMode  string         `json:"assignment_mode,omitempty"`
	AssigneeUserID  string         `json:"assignee_user_id,omitempty"`
	AssigneeRoleKey string         `json:"assignee_role_key,omitempty"`
	IsMine          bool           `json:"is_mine,omitempty"`
	DueAt           string         `json:"due_at,omitempty"`
	EscalateAt      string         `json:"escalate_at,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type uiWorklistApproval struct {
	ID              string         `json:"id"`
	WorkflowKey     string         `json:"workflow_key"`
	WorkflowVersion int            `json:"workflow_version,omitempty"`
	TargetType      string         `json:"target_type"`
	TargetID        string         `json:"target_id"`
	DocumentType    string         `json:"document_type,omitempty"`
	TargetTitle     string         `json:"target_title,omitempty"`
	TargetNumber    string         `json:"target_number,omitempty"`
	TargetStatus    string         `json:"target_status,omitempty"`
	TargetUpdatedAt string         `json:"target_updated_at,omitempty"`
	OpenPath        string         `json:"open_path,omitempty"`
	Status          string         `json:"status"`
	StageKey        string         `json:"stage_key,omitempty"`
	RequestedBy     string         `json:"requested_by,omitempty"`
	DueAt           string         `json:"due_at,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}
