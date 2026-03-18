package templateoutput

type Repository interface {
	Versions(templateKey string) []Version
	ListVersions() []Version
	SaveVersion(version Version) error
	Bindings() []Binding
	SaveBinding(binding Binding) error
}
