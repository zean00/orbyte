package templateoutput

type MemoryRepository struct {
	versions []Version
	bindings []Binding
	fixtures []TemplateFixture
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

func (r *MemoryRepository) Fixtures(templateKey, targetKind string) []TemplateFixture {
	items := make([]TemplateFixture, 0, len(r.fixtures))
	for _, item := range r.fixtures {
		if templateKey != "" && item.TemplateKey != "" && item.TemplateKey != templateKey {
			continue
		}
		if targetKind != "" && item.TargetKind != targetKind {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *MemoryRepository) SaveFixture(fixture TemplateFixture) error {
	for i, current := range r.fixtures {
		if current.FixtureKey == fixture.FixtureKey {
			r.fixtures[i] = fixture
			return nil
		}
	}
	r.fixtures = append(r.fixtures, fixture)
	return nil
}
