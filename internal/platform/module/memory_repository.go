package module

import "sort"

type MemoryRepository struct {
	items       map[string]InstalledModule
	activations map[string]LocalExtensionActivation
}

func NewMemoryRepository(items []InstalledModule) *MemoryRepository {
	stored := make(map[string]InstalledModule, len(items))
	for _, item := range items {
		stored[item.Key] = item
	}
	return &MemoryRepository{items: stored, activations: map[string]LocalExtensionActivation{}}
}

func (r *MemoryRepository) Get(key string) (InstalledModule, bool) {
	item, ok := r.items[key]
	return item, ok
}

func (r *MemoryRepository) List() []InstalledModule {
	items := make([]InstalledModule, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (r *MemoryRepository) Save(item InstalledModule) error {
	r.items[item.Key] = item
	return nil
}

func (r *MemoryRepository) GetActivation(baseModuleKey, scope, scopeID string) (LocalExtensionActivation, bool) {
	item, ok := r.activations[activationKey(baseModuleKey, scope, scopeID)]
	return item, ok
}

func (r *MemoryRepository) ListActivations() []LocalExtensionActivation {
	items := make([]LocalExtensionActivation, 0, len(r.activations))
	for _, item := range r.activations {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].BaseModuleKey == items[j].BaseModuleKey {
			if items[i].Scope == items[j].Scope {
				if items[i].ScopeID == items[j].ScopeID {
					return items[i].ExtensionModuleKey < items[j].ExtensionModuleKey
				}
				return items[i].ScopeID < items[j].ScopeID
			}
			return items[i].Scope < items[j].Scope
		}
		return items[i].BaseModuleKey < items[j].BaseModuleKey
	})
	return items
}

func (r *MemoryRepository) SaveActivation(item LocalExtensionActivation) error {
	r.activations[activationKey(item.BaseModuleKey, item.Scope, item.ScopeID)] = item
	return nil
}

func (r *MemoryRepository) DeleteActivation(baseModuleKey, scope, scopeID string) error {
	delete(r.activations, activationKey(baseModuleKey, scope, scopeID))
	return nil
}

func activationKey(baseModuleKey, scope, scopeID string) string {
	return baseModuleKey + "|" + scope + "|" + scopeID
}
