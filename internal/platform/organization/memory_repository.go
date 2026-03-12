package organization

type MemoryRepository struct {
	root      Organization
	locations []Location
}

func NewMemoryRepository(root Organization, locations []Location) *MemoryRepository {
	return &MemoryRepository{root: root, locations: append([]Location(nil), locations...)}
}

func (r *MemoryRepository) Root() Organization {
	return r.root
}

func (r *MemoryRepository) Locations() []Location {
	return append([]Location(nil), r.locations...)
}
