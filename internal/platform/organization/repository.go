package organization

type Repository interface {
	Root() Organization
	Locations() []Location
	OperatingUnits() []OperatingUnit
	SaveOperatingUnit(unit OperatingUnit) error
}
