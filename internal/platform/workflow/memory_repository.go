package workflow

import (
	"sort"
	"time"

	"orbyte/internal/platform/shared"
)

type MemoryRepository struct {
	definitions map[string][]Definition
	tasks       []Task
	approvals   []Approval
	history     []HistoryEvent
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		definitions: map[string][]Definition{},
		tasks:       []Task{},
		approvals:   []Approval{},
		history:     []HistoryEvent{},
	}
}

func (r *MemoryRepository) SaveDefinition(def Definition) error {
	if _, ok := r.GetDefinition(def.Key); ok {
		return shared.Conflict("workflow definition already exists")
	}
	now := time.Now().UTC()
	def.Version = 1
	def.Status = "published"
	if def.CreatedAt.IsZero() {
		def.CreatedAt = now
	}
	if def.UpdatedAt.IsZero() {
		def.UpdatedAt = now
	}
	if def.PublishedAt.IsZero() {
		def.PublishedAt = now
	}
	r.definitions[def.Key] = append(r.definitions[def.Key], cloneDefinition(def))
	return nil
}

func (r *MemoryRepository) SaveDefinitionDraft(def Definition) (Definition, error) {
	if _, exists := r.definitions[def.Key]; exists {
		return Definition{}, shared.Conflict("workflow definition already exists")
	}
	now := time.Now().UTC()
	def.Version = 1
	def.Status = "draft"
	if def.CreatedAt.IsZero() {
		def.CreatedAt = now
	}
	def.UpdatedAt = now
	r.definitions[def.Key] = append(r.definitions[def.Key], cloneDefinition(def))
	return cloneDefinition(def), nil
}

func (r *MemoryRepository) DeleteDefinition(key string) error {
	delete(r.definitions, key)
	return nil
}

func (r *MemoryRepository) GetDefinition(key string) (Definition, bool) {
	items := r.definitions[key]
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Status == "published" {
			return cloneDefinition(items[i]), true
		}
	}
	return Definition{}, false
}

func (r *MemoryRepository) GetDefinitionVersion(key string, version int) (Definition, bool) {
	for _, item := range r.definitions[key] {
		if item.Version == version {
			return cloneDefinition(item), true
		}
	}
	return Definition{}, false
}

func (r *MemoryRepository) ListDefinitions() []Definition {
	items := make([]Definition, 0, len(r.definitions))
	for key := range r.definitions {
		if def, ok := r.GetDefinition(key); ok {
			items = append(items, def)
			continue
		}
		versions := r.definitions[key]
		if len(versions) > 0 {
			items = append(items, cloneDefinition(versions[len(versions)-1]))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *MemoryRepository) ListDefinitionVersions(key string) []Definition {
	items := append([]Definition(nil), r.definitions[key]...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Version == items[j].Version {
			return items[i].Status < items[j].Status
		}
		return items[i].Version < items[j].Version
	})
	for i := range items {
		items[i] = cloneDefinition(items[i])
	}
	return items
}

func (r *MemoryRepository) CreateDraft(key, actorID string) (Definition, error) {
	published, ok := r.GetDefinition(key)
	if !ok {
		return Definition{}, shared.NotFound("workflow definition not found")
	}
	for _, item := range r.definitions[key] {
		if item.Status == "draft" {
			return Definition{}, shared.Conflict("workflow draft already exists")
		}
	}
	now := time.Now().UTC()
	draft := cloneDefinition(published)
	draft.Version++
	draft.Status = "draft"
	draft.CreatedAt = now
	draft.UpdatedAt = now
	draft.UpdatedBy = actorID
	draft.PublishedAt = time.Time{}
	draft.PublishedBy = ""
	r.definitions[key] = append(r.definitions[key], draft)
	return cloneDefinition(draft), nil
}

func (r *MemoryRepository) SaveDraft(def Definition, actorID string) (Definition, error) {
	items := r.definitions[def.Key]
	for i := range items {
		if items[i].Version == def.Version && items[i].Status == "draft" {
			updated := cloneDefinition(def)
			updated.Status = "draft"
			updated.CreatedAt = items[i].CreatedAt
			updated.UpdatedAt = time.Now().UTC()
			updated.UpdatedBy = actorID
			r.definitions[def.Key][i] = updated
			return cloneDefinition(updated), nil
		}
	}
	return Definition{}, shared.NotFound("workflow draft not found")
}

func (r *MemoryRepository) PublishDefinition(key string, version int, actorID string) (Definition, error) {
	items := r.definitions[key]
	index := -1
	for i := range items {
		if items[i].Version == version && items[i].Status == "draft" {
			index = i
			break
		}
	}
	if index < 0 {
		return Definition{}, shared.NotFound("workflow draft not found")
	}
	now := time.Now().UTC()
	draft := items[index]
	draft.Status = "published"
	draft.PublishedAt = now
	draft.PublishedBy = actorID
	draft.UpdatedAt = now
	draft.UpdatedBy = actorID
	r.definitions[key][index] = draft
	return cloneDefinition(draft), nil
}

func (r *MemoryRepository) SaveTask(task Task) error {
	r.tasks = append(r.tasks, cloneTask(task))
	return nil
}

func (r *MemoryRepository) ListTasks() []Task {
	items := append([]Task(nil), r.tasks...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	for i := range items {
		items[i] = cloneTask(items[i])
	}
	return items
}

func (r *MemoryRepository) UpdateTaskStatus(update TaskStatusUpdate) error {
	for i := range r.tasks {
		if r.tasks[i].ID == update.ID {
			r.tasks[i].Status = update.Status
			if !update.ResolvedAt.IsZero() {
				if r.tasks[i].Metadata == nil {
					r.tasks[i].Metadata = map[string]any{}
				}
				r.tasks[i].Metadata["resolved_at"] = update.ResolvedAt
			}
			if update.ResolvedBy != "" {
				if r.tasks[i].Metadata == nil {
					r.tasks[i].Metadata = map[string]any{}
				}
				r.tasks[i].Metadata["resolved_by"] = update.ResolvedBy
			}
			return nil
		}
	}
	return shared.NotFound("workflow task not found")
}

func (r *MemoryRepository) SaveApproval(approval Approval) error {
	r.approvals = append(r.approvals, cloneApproval(approval))
	return nil
}

func (r *MemoryRepository) ListApprovals() []Approval {
	items := append([]Approval(nil), r.approvals...)
	sort.Slice(items, func(i, j int) bool { return items[i].RequestedAt.Before(items[j].RequestedAt) })
	for i := range items {
		items[i] = cloneApproval(items[i])
	}
	return items
}

func (r *MemoryRepository) UpdateApprovalStatus(update ApprovalStatusUpdate) error {
	for i := range r.approvals {
		if r.approvals[i].ID == update.ID {
			r.approvals[i].Status = update.Status
			r.approvals[i].ResolvedBy = update.ResolvedBy
			if !update.ResolvedAt.IsZero() {
				r.approvals[i].ResolvedAt = update.ResolvedAt
			}
			return nil
		}
	}
	return shared.NotFound("workflow approval not found")
}

func (r *MemoryRepository) SaveHistory(event HistoryEvent) error {
	r.history = append(r.history, cloneHistoryEvent(event))
	return nil
}

func (r *MemoryRepository) ListHistory(targetType, targetID string) []HistoryEvent {
	items := make([]HistoryEvent, 0, len(r.history))
	for _, item := range r.history {
		if targetType != "" && item.TargetType != targetType {
			continue
		}
		if targetID != "" && item.TargetID != targetID {
			continue
		}
		items = append(items, cloneHistoryEvent(item))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.Before(items[j].OccurredAt) })
	return items
}

func cloneDefinition(def Definition) Definition {
	def.States = append([]string(nil), def.States...)
	def.Actions = append([]ActionRule(nil), def.Actions...)
	for i := range def.Actions {
		def.Actions[i].CandidateRoleKeys = append([]string(nil), def.Actions[i].CandidateRoleKeys...)
	}
	return def
}

func cloneTask(task Task) Task {
	task.CandidateRoleKeys = append([]string(nil), task.CandidateRoleKeys...)
	task.Metadata = cloneMap(task.Metadata)
	return task
}

func cloneApproval(approval Approval) Approval {
	approval.CandidateRoleKeys = append([]string(nil), approval.CandidateRoleKeys...)
	approval.Metadata = cloneMap(approval.Metadata)
	return approval
}

func cloneHistoryEvent(event HistoryEvent) HistoryEvent {
	event.AssignmentSummary = cloneMap(event.AssignmentSummary)
	event.Metadata = cloneMap(event.Metadata)
	return event
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
