package reference

import "sort"

type MemoryRepository struct {
	types   map[string]TypeDefinition
	records map[string]map[string]Record
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		types:   map[string]TypeDefinition{},
		records: map[string]map[string]Record{},
	}
}

func (r *MemoryRepository) SaveType(def TypeDefinition) error {
	r.types[def.Key] = def
	return nil
}

func (r *MemoryRepository) GetType(key string) (TypeDefinition, bool) {
	def, ok := r.types[key]
	return def, ok
}

func (r *MemoryRepository) ListTypes() []TypeDefinition {
	items := make([]TypeDefinition, 0, len(r.types))
	for _, item := range r.types {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *MemoryRepository) SaveRecord(record Record) error {
	if _, ok := r.records[record.TypeKey]; !ok {
		r.records[record.TypeKey] = map[string]Record{}
	}
	r.records[record.TypeKey][recordKey(record)] = record
	return nil
}

func (r *MemoryRepository) ListRecords(typeKey string) []Record {
	items := r.records[typeKey]
	out := make([]Record, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Key == out[j].Key {
			if out[i].Scope == out[j].Scope {
				return out[i].ScopeID < out[j].ScopeID
			}
			return out[i].Scope < out[j].Scope
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func recordKey(record Record) string {
	return record.Key + "|" + record.Scope + "|" + record.ScopeID
}
