package workflow

type Repository interface {
	SaveDefinition(def Definition) error
	GetDefinition(key string) (Definition, bool)
	ListDefinitions() []Definition
	SaveTask(task Task) error
	ListTasks() []Task
	UpdateTaskStatus(update TaskStatusUpdate) error
	SaveApproval(approval Approval) error
	ListApprovals() []Approval
	UpdateApprovalStatus(update ApprovalStatusUpdate) error
}
