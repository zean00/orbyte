package workflow

import (
	"sort"

	"orbyte/internal/platform/shared"
)

type MemoryRepository struct {
	definitions map[string]Definition
	tasks       []Task
	approvals   []Approval
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{definitions: map[string]Definition{}, tasks: []Task{}, approvals: []Approval{}}
}

func (r *MemoryRepository) SaveDefinition(def Definition) error {
	if _, exists := r.definitions[def.Key]; exists {
		return shared.Conflict("workflow definition already exists")
	}
	r.definitions[def.Key] = def
	return nil
}

func (r *MemoryRepository) GetDefinition(key string) (Definition, bool) {
	def, ok := r.definitions[key]
	return def, ok
}

func (r *MemoryRepository) ListDefinitions() []Definition {
	items := make([]Definition, 0, len(r.definitions))
	for _, item := range r.definitions {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	return items
}

func (r *MemoryRepository) SaveTask(task Task) error {
	r.tasks = append(r.tasks, task)
	return nil
}

func (r *MemoryRepository) ListTasks() []Task {
	items := append([]Task(nil), r.tasks...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) UpdateTaskStatus(taskID, status string) error {
	for i := range r.tasks {
		if r.tasks[i].ID == taskID {
			r.tasks[i].Status = status
			return nil
		}
	}
	return shared.NotFound("workflow task not found")
}

func (r *MemoryRepository) SaveApproval(approval Approval) error {
	r.approvals = append(r.approvals, approval)
	return nil
}

func (r *MemoryRepository) ListApprovals() []Approval {
	items := append([]Approval(nil), r.approvals...)
	sort.Slice(items, func(i, j int) bool { return items[i].RequestedAt.Before(items[j].RequestedAt) })
	return items
}

func (r *MemoryRepository) UpdateApprovalStatus(approvalID, status string) error {
	for i := range r.approvals {
		if r.approvals[i].ID == approvalID {
			r.approvals[i].Status = status
			return nil
		}
	}
	return shared.NotFound("workflow approval not found")
}
