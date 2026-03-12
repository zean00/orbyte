package organization

type Repository interface {
	Root() Organization
	Locations() []Location
}
