package model

import (
	"sort"
	"time"
)

type MemoryRepository struct {
	definitions map[string]Definition
	records     map[string]map[string]Record
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		definitions: map[string]Definition{},
		records:     map[string]map[string]Record{},
	}
}

func (r *MemoryRepository) SaveDefinition(def Definition) error {
	r.definitions[def.Key] = def
	return nil
}

func (r *MemoryRepository) ListDefinitions() []Definition {
	items := make([]Definition, 0, len(r.definitions))
	for _, item := range r.definitions {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *MemoryRepository) GetDefinition(key string) (Definition, bool) {
	item, ok := r.definitions[key]
	return item, ok
}

func (r *MemoryRepository) SaveRecord(record Record) error {
	if _, ok := r.records[record.ModelKey]; !ok {
		r.records[record.ModelKey] = map[string]Record{}
	}
	r.records[record.ModelKey][record.ID] = record
	return nil
}

func (r *MemoryRepository) DeleteRecord(modelKey, id string) error {
	if _, ok := r.records[modelKey]; !ok {
		return nil
	}
	delete(r.records[modelKey], id)
	return nil
}

func (r *MemoryRepository) GetRecord(modelKey, id string) (Record, bool) {
	items, ok := r.records[modelKey]
	if !ok {
		return Record{}, false
	}
	item, ok := items[id]
	return item, ok
}

func (r *MemoryRepository) ListRecords(modelKey string) []Record {
	items := make([]Record, 0)
	for _, item := range r.records[modelKey] {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (r *MemoryRepository) QueryRecords(def Definition, query Query) ([]Record, int, error) {
	items := r.ListRecords(def.Key)
	filtered := make([]Record, 0, len(items))
	for _, item := range items {
		if !matchesFilters(item, query.Filters) {
			continue
		}
		filtered = append(filtered, item)
	}
	sortKey := query.SortKey
	sort.Slice(filtered, func(i, j int) bool {
		left := stringValue(resolveField(filtered[i].Values, sortKey))
		right := stringValue(resolveField(filtered[j].Values, sortKey))
		switch sortKey {
		case "id":
			left, right = filtered[i].ID, filtered[j].ID
		case "created_at":
			left, right = filtered[i].CreatedAt.Format(time.RFC3339Nano), filtered[j].CreatedAt.Format(time.RFC3339Nano)
		case "updated_at":
			left, right = filtered[i].UpdatedAt.Format(time.RFC3339Nano), filtered[j].UpdatedAt.Format(time.RFC3339Nano)
		}
		if query.Desc {
			return left > right
		}
		return left < right
	})
	total := len(filtered)
	start := (query.Page - 1) * query.PageSize
	if start >= len(filtered) {
		return []Record{}, total, nil
	}
	end := start + query.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], total, nil
}
