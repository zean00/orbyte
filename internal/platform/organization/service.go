package organization

import "time"

type Service struct {
	repo Repository
}

func NewService() *Service {
	now := time.Now().UTC()
	root := Organization{
		ID:        "org_default",
		Key:       "default",
		Name:      "Default Organization",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	locations := []Location{{
		ID:             "loc_hq",
		OrganizationID: "org_default",
		Key:            "hq",
		Name:           "Head Office",
		Type:           "location",
		Status:         "active",
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	return NewServiceWithRepository(NewMemoryRepository(root, locations))
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Root() Organization {
	return s.repo.Root()
}

func (s *Service) Locations() []Location {
	return s.repo.Locations()
}

func (s *Service) Resolve(locationID string) ScopeContext {
	ctx := ScopeContext{
		OrganizationID: s.repo.Root().ID,
		Source:         "default",
	}
	for _, location := range s.repo.Locations() {
		ctx.AllowedScopes = append(ctx.AllowedScopes, location.ID)
		if location.ID == locationID {
			ctx.LocationID = locationID
			ctx.Source = "requested_location"
		}
	}
	return ctx
}
