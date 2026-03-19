package organization

type MemoryRepository struct {
	root           Organization
	locations      []Location
	operatingUnits []OperatingUnit
}

func NewMemoryRepository(root Organization, locations []Location, operatingUnits ...[]OperatingUnit) *MemoryRepository {
	items := []OperatingUnit{}
	if len(operatingUnits) > 0 {
		items = append(items, operatingUnits[0]...)
	}
	return &MemoryRepository{root: root, locations: append([]Location(nil), locations...), operatingUnits: append([]OperatingUnit(nil), items...)}
}

func (r *MemoryRepository) Root() Organization {
	return r.root
}

func (r *MemoryRepository) Locations() []Location {
	return append([]Location(nil), r.locations...)
}

func (r *MemoryRepository) OperatingUnits() []OperatingUnit {
	return append([]OperatingUnit(nil), r.operatingUnits...)
}

func (r *MemoryRepository) SaveOperatingUnit(unit OperatingUnit) error {
	for i, existing := range r.operatingUnits {
		if existing.ID == unit.ID {
			r.operatingUnits[i] = unit
			return nil
		}
	}
	r.operatingUnits = append(r.operatingUnits, unit)
	return nil
}
