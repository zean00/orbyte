package secretstore

import "sort"

type MemoryRepository struct {
	secrets map[string]Secret
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{secrets: map[string]Secret{}}
}

func (r *MemoryRepository) Get(ref string) (Secret, bool) {
	secret, ok := r.secrets[ref]
	return secret, ok
}

func (r *MemoryRepository) List() []Secret {
	items := make([]Secret, 0, len(r.secrets))
	for _, item := range r.secrets {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Ref < items[j].Ref })
	return items
}

func (r *MemoryRepository) Save(secret Secret) error {
	r.secrets[secret.Ref] = secret
	return nil
}
