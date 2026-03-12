package search

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryBackend struct {
	mu          sync.RWMutex
	collections map[string]map[string]IndexedRecord
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{collections: map[string]map[string]IndexedRecord{}}
}

func (b *MemoryBackend) EnsureIndex(def IndexDefinition, organizationID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := collectionKey(def.Key, organizationID)
	if _, ok := b.collections[key]; !ok {
		b.collections[key] = map[string]IndexedRecord{}
	}
	return nil
}

func (b *MemoryBackend) Upsert(def IndexDefinition, organizationID string, record IndexedRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := collectionKey(def.Key, organizationID)
	if _, ok := b.collections[key]; !ok {
		b.collections[key] = map[string]IndexedRecord{}
	}
	current, ok := b.collections[key][record.ID]
	if ok {
		if current.Version > record.Version {
			return nil
		}
		if current.Version == record.Version && !record.UpdatedAt.After(current.UpdatedAt) {
			return nil
		}
	}
	b.collections[key][record.ID] = record
	return nil
}

func (b *MemoryBackend) Delete(def IndexDefinition, organizationID, sourceID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := collectionKey(def.Key, organizationID)
	delete(b.collections[key], sourceID)
	return nil
}

func (b *MemoryBackend) Search(def IndexDefinition, organizationID string, req QueryRequest) (QueryResult, error) {
	b.mu.RLock()
	items := make([]IndexedRecord, 0, len(b.collections[collectionKey(def.Key, organizationID)]))
	for _, item := range b.collections[collectionKey(def.Key, organizationID)] {
		items = append(items, item)
	}
	b.mu.RUnlock()

	mode := normalizeQueryMode(req.Mode, def)
	filtered := make([]QueryHit, 0, len(items))
	for _, item := range items {
		if !matchesIndexedFilters(item, req.Filters) {
			continue
		}
		score := 0.0
		if req.Query != "" && (mode == "keyword" || mode == "hybrid") {
			score += lexicalScore(def, item, req.Query)
		}
		if len(req.Vector) > 0 && (mode == "vector" || mode == "hybrid") {
			score += vectorScore(def, item, req.Vector, req.VectorField)
		}
		if req.Query == "" && len(req.Vector) == 0 {
			score = 1
		}
		if score <= 0 {
			continue
		}
		filtered = append(filtered, QueryHit{
			ID:         item.ID,
			SourceID:   item.SourceID,
			SourceKind: item.SourceKind,
			Score:      score,
			Fields:     includeFields(item, req.IncludeFields),
		})
	}

	sort.Slice(filtered, func(i, j int) bool {
		if req.SortBy != "" {
			left := stringValue(filtered[i].Fields[req.SortBy])
			right := stringValue(filtered[j].Fields[req.SortBy])
			if req.Desc {
				if left == right {
					return filtered[i].Score > filtered[j].Score
				}
				return left > right
			}
			if left == right {
				return filtered[i].Score > filtered[j].Score
			}
			return left < right
		}
		if filtered[i].Score == filtered[j].Score {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].Score > filtered[j].Score
	})

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	total := len(filtered)
	start := (page - 1) * pageSize
	if start >= total {
		return QueryResult{IndexKey: def.Key, Mode: mode, Total: total, Page: page, PageSize: pageSize, Hits: []QueryHit{}}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return QueryResult{IndexKey: def.Key, Mode: mode, Total: total, Page: page, PageSize: pageSize, Hits: filtered[start:end]}, nil
}

func collectionKey(indexKey, organizationID string) string {
	org := strings.TrimSpace(organizationID)
	if org == "" {
		org = "global"
	}
	return org + "__" + indexKey
}

func matchesIndexedFilters(item IndexedRecord, filters map[string]string) bool {
	for key, expected := range filters {
		expected = strings.TrimSpace(expected)
		if expected == "" {
			continue
		}
		switch key {
		case "organization_id":
			if item.OrganizationID != expected {
				return false
			}
		case "location_id":
			if item.LocationID != expected {
				return false
			}
		default:
			if !strings.EqualFold(stringValue(item.Fields[key]), expected) {
				return false
			}
		}
	}
	return true
}

func lexicalScore(def IndexDefinition, item IndexedRecord, query string) float64 {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return 0
	}
	score := 0.0
	for _, field := range def.Fields {
		if !field.Searchable {
			continue
		}
		value := strings.ToLower(stringValue(item.Fields[field.Key]))
		if value == "" {
			continue
		}
		if strings.Contains(value, query) {
			score += 1
		}
	}
	return score
}

func vectorScore(def IndexDefinition, item IndexedRecord, query []float32, fieldKey string) float64 {
	selected := fieldKey
	if strings.TrimSpace(selected) == "" && len(def.VectorFields) > 0 {
		selected = def.VectorFields[0].Key
	}
	vector := item.Vectors[selected]
	if len(vector) == 0 || len(query) == 0 {
		return 0
	}
	limit := len(vector)
	if len(query) < limit {
		limit = len(query)
	}
	dot := 0.0
	for i := 0; i < limit; i++ {
		dot += float64(vector[i] * query[i])
	}
	return math.Max(dot, 0)
}

func includeFields(item IndexedRecord, requested []string) map[string]any {
	if len(requested) == 0 {
		fields := cloneAnyMap(item.Fields)
		fields["location_id"] = item.LocationID
		fields["organization_id"] = item.OrganizationID
		fields["updated_at"] = item.UpdatedAt
		return fields
	}
	out := map[string]any{}
	for _, key := range requested {
		if value, ok := item.Fields[key]; ok {
			out[key] = value
		}
	}
	return out
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
