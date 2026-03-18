package templateoutput

type MemoryRepository struct {
	versions []Version
	bindings []Binding
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{}
}

func (r *MemoryRepository) Versions(templateKey string) []Version {
	items := make([]Version, 0)
	for _, item := range r.versions {
		if item.TemplateKey == templateKey {
			items = append(items, item)
		}
	}
	return items
}

func (r *MemoryRepository) ListVersions() []Version {
	return append([]Version(nil), r.versions...)
}

func (r *MemoryRepository) SaveVersion(version Version) error {
	for i, current := range r.versions {
		if current.TemplateKey == version.TemplateKey && current.Version == version.Version {
			r.versions[i] = version
			return nil
		}
	}
	r.versions = append(r.versions, version)
	return nil
}

func (r *MemoryRepository) Bindings() []Binding {
	return append([]Binding(nil), r.bindings...)
}

func (r *MemoryRepository) SaveBinding(binding Binding) error {
	for i, current := range r.bindings {
		if current.ID == binding.ID {
			r.bindings[i] = binding
			return nil
		}
	}
	r.bindings = append(r.bindings, binding)
	return nil
}
