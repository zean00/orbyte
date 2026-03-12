package module

import "sort"

type MemoryRepository struct {
	items map[string]InstalledModule
}

func NewMemoryRepository(items []InstalledModule) *MemoryRepository {
	stored := make(map[string]InstalledModule, len(items))
	for _, item := range items {
		stored[item.Key] = item
	}
	return &MemoryRepository{items: stored}
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
