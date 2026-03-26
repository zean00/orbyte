package templateoutput

type Repository interface {
	Definitions() []Definition
	GetDefinition(key string) (Definition, bool)
	SaveDefinition(def Definition) error
	DeleteDefinition(key string) error
	Versions(templateKey string) []Version
	ListVersions() []Version
	SaveVersion(version Version) error
	DeleteVersions(templateKey string) error
	Bindings() []Binding
	SaveBinding(binding Binding) error
	DeleteBinding(bindingID string) error
	Fixtures(templateKey, targetKind string) []TemplateFixture
	SaveFixture(fixture TemplateFixture) error
	DeleteFixtures(templateKey string) error
}
