package search

import "sort"

type MemoryRepository struct {
	documents map[string]DocumentSummary
	statuses  map[string]ProjectionStatus
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{documents: map[string]DocumentSummary{}, statuses: map[string]ProjectionStatus{}}
}

func (r *MemoryRepository) SaveDocument(summary DocumentSummary) error {
	if current, ok := r.documents[summary.DocumentID]; ok {
		if current.Version > summary.Version {
			return nil
		}
		if current.Version == summary.Version && !summary.UpdatedAt.After(current.UpdatedAt) {
			return nil
		}
	}
	r.documents[summary.DocumentID] = summary
	return nil
}

func (r *MemoryRepository) ListDocuments() []DocumentSummary {
	items := make([]DocumentSummary, 0, len(r.documents))
	for _, item := range r.documents {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.Before(items[j].UpdatedAt)
	})
	return items
}

func (r *MemoryRepository) SaveProjectionStatus(status ProjectionStatus) error {
	r.statuses[status.ProjectionKey] = status
	return nil
}

func (r *MemoryRepository) ListProjectionStatuses() []ProjectionStatus {
	items := make([]ProjectionStatus, 0, len(r.statuses))
	for _, item := range r.statuses {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ProjectionKey < items[j].ProjectionKey })
	return items
}
