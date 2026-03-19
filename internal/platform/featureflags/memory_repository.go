package featureflags

import "sort"

type MemoryRepository struct {
	definitions map[string]Definition
	values      map[string]Value
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		definitions: map[string]Definition{},
		values:      map[string]Value{},
	}
}

func (r *MemoryRepository) SaveDefinition(def Definition) error {
	r.definitions[def.Key] = def
	return nil
}

func (r *MemoryRepository) GetDefinition(key string) (Definition, bool) {
	item, ok := r.definitions[key]
	return item, ok
}

func (r *MemoryRepository) ListDefinitions() []Definition {
	items := make([]Definition, 0, len(r.definitions))
	for _, item := range r.definitions {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *MemoryRepository) SaveValue(value Value) error {
	r.values[valueStoreKey(value.FlagKey, value.Scope, value.ScopeID)] = value
	return nil
}

func (r *MemoryRepository) GetValue(flagKey, scope, scopeID string) (Value, bool) {
	item, ok := r.values[valueStoreKey(flagKey, scope, scopeID)]
	return item, ok
}

func (r *MemoryRepository) ListValues() []Value {
	items := make([]Value, 0, len(r.values))
	for _, item := range r.values {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].FlagKey == items[j].FlagKey {
			if items[i].Scope == items[j].Scope {
				return items[i].ScopeID < items[j].ScopeID
			}
			return items[i].Scope < items[j].Scope
		}
		return items[i].FlagKey < items[j].FlagKey
	})
	return items
}

func valueStoreKey(flagKey, scope, scopeID string) string {
	return flagKey + "|" + scope + "|" + scopeID
}
