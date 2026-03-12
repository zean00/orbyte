package integration

import "sort"

type MemoryRepository struct {
	systems     map[string]ExternalSystem
	submissions map[string]SubmissionRecord
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		systems:     map[string]ExternalSystem{},
		submissions: map[string]SubmissionRecord{},
	}
}

func (r *MemoryRepository) SaveSystem(system ExternalSystem) error {
	if system.Settings == nil {
		system.Settings = map[string]any{}
	}
	r.systems[system.Key] = system
	return nil
}

func (r *MemoryRepository) ListSystems() []ExternalSystem {
	items := make([]ExternalSystem, 0, len(r.systems))
	for _, item := range r.systems {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *MemoryRepository) GetSystem(key string) (ExternalSystem, bool) {
	item, ok := r.systems[key]
	return item, ok
}

func (r *MemoryRepository) SaveSubmission(record SubmissionRecord) error {
	if record.Payload == nil {
		record.Payload = map[string]any{}
	}
	if record.Result == nil {
		record.Result = map[string]any{}
	}
	r.submissions[record.ID] = record
	return nil
}

func (r *MemoryRepository) ListSubmissions() []SubmissionRecord {
	items := make([]SubmissionRecord, 0, len(r.submissions))
	for _, item := range r.submissions {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) GetSubmission(id string) (SubmissionRecord, bool) {
	item, ok := r.submissions[id]
	return item, ok
}
