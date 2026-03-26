package templateoutput

type MemoryRepository struct {
	defs     map[string]Definition
	versions []Version
	bindings []Binding
	fixtures []TemplateFixture
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{defs: map[string]Definition{}}
}

func (r *MemoryRepository) Definitions() []Definition {
	items := make([]Definition, 0, len(r.defs))
	for _, item := range r.defs {
		items = append(items, cloneDefinition(item))
	}
	return items
}

func (r *MemoryRepository) GetDefinition(key string) (Definition, bool) {
	item, ok := r.defs[key]
	if !ok {
		return Definition{}, false
	}
	return cloneDefinition(item), true
}

func (r *MemoryRepository) SaveDefinition(def Definition) error {
	r.defs[def.Key] = cloneDefinition(def)
	return nil
}

func (r *MemoryRepository) DeleteDefinition(key string) error {
	delete(r.defs, key)
	return nil
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

func (r *MemoryRepository) DeleteVersions(templateKey string) error {
	filtered := r.versions[:0]
	for _, item := range r.versions {
		if item.TemplateKey != templateKey {
			filtered = append(filtered, item)
		}
	}
	r.versions = filtered
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

func (r *MemoryRepository) DeleteBinding(bindingID string) error {
	filtered := r.bindings[:0]
	for _, item := range r.bindings {
		if item.ID != bindingID {
			filtered = append(filtered, item)
		}
	}
	r.bindings = filtered
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

func (r *MemoryRepository) DeleteFixtures(templateKey string) error {
	filtered := r.fixtures[:0]
	for _, item := range r.fixtures {
		if item.TemplateKey != templateKey {
			filtered = append(filtered, item)
		}
	}
	r.fixtures = filtered
	return nil
}

func cloneDefinition(def Definition) Definition {
	def.Formats = append([]string(nil), def.Formats...)
	def.RelatedSources = append([]RelatedSource(nil), def.RelatedSources...)
	def.AllowedScopes = append([]string(nil), def.AllowedScopes...)
	def.RequiredPermissions = append([]string(nil), def.RequiredPermissions...)
	return def
}
