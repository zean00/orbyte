package search

import "sort"

type MemoryRepository struct {
	documents map[string]DocumentSummary
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{documents: map[string]DocumentSummary{}}
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
