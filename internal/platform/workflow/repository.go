package workflow

type Repository interface {
	SaveDefinition(def Definition) error
	GetDefinition(key string) (Definition, bool)
	ListDefinitions() []Definition
	SaveTask(task Task) error
	ListTasks() []Task
	UpdateTaskStatus(taskID, status string) error
	SaveApproval(approval Approval) error
	ListApprovals() []Approval
	UpdateApprovalStatus(approvalID, status string) error
}
