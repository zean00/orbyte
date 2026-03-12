package config

import "sort"

type MemoryRepository struct {
	entries map[string]Entry
}

func NewMemoryRepository(entries []Entry) *MemoryRepository {
	stored := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		stored[entryStoreKey(entry.Key, entry.Scope, entry.ScopeID)] = entry
	}
	return &MemoryRepository{entries: stored}
}

func (r *MemoryRepository) Get(key, scope, scopeID string) (Entry, bool) {
	entry, ok := r.entries[entryStoreKey(key, scope, scopeID)]
	return entry, ok
}

func (r *MemoryRepository) List() []Entry {
	entries := make([]Entry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Key == entries[j].Key {
			if entries[i].Scope == entries[j].Scope {
				return entries[i].ScopeID < entries[j].ScopeID
			}
			return entries[i].Scope < entries[j].Scope
		}
		return entries[i].Key < entries[j].Key
	})
	return entries
}

func (r *MemoryRepository) Save(entry Entry) error {
	r.entries[entryStoreKey(entry.Key, entry.Scope, entry.ScopeID)] = entry
	return nil
}

func entryStoreKey(key, scope, scopeID string) string {
	return key + "|" + scope + "|" + scopeID
}
