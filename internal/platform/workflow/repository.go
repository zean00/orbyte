package workflow

type Repository interface {
	SaveDefinition(def Definition) error
	SaveDefinitionDraft(def Definition) (Definition, error)
	DeleteDefinition(key string) error
	GetDefinition(key string) (Definition, bool)
	GetDefinitionVersion(key string, version int) (Definition, bool)
	ListDefinitions() []Definition
	ListDefinitionVersions(key string) []Definition
	CreateDraft(key, actorID string) (Definition, error)
	SaveDraft(def Definition, actorID string) (Definition, error)
	PublishDefinition(key string, version int, actorID string) (Definition, error)
	SaveTask(task Task) error
	ListTasks() []Task
	UpdateTaskStatus(update TaskStatusUpdate) error
	SaveApproval(approval Approval) error
	ListApprovals() []Approval
	UpdateApprovalStatus(update ApprovalStatusUpdate) error
	SaveHistory(event HistoryEvent) error
	ListHistory(targetType, targetID string) []HistoryEvent
}
