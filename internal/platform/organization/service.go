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
	operatingUnits := []OperatingUnit{{
		ID:             "ou_hq_ops",
		OrganizationID: "org_default",
		LocationID:     "loc_hq",
		Key:            "hq_ops",
		Name:           "HQ Operations",
		Status:         "active",
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	return NewServiceWithRepository(NewMemoryRepository(root, locations, operatingUnits))
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

func (s *Service) OperatingUnits() []OperatingUnit {
	return s.repo.OperatingUnits()
}

func (s *Service) Resolve(locationID string) ScopeContext {
	return s.ResolveScope(locationID, "")
}

func (s *Service) ResolveScope(locationID, operatingUnitID string) ScopeContext {
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
	for _, unit := range s.repo.OperatingUnits() {
		if unit.Status != "active" {
			continue
		}
		ctx.AllowedOperatingUnits = append(ctx.AllowedOperatingUnits, unit.ID)
		if operatingUnitID == "" || unit.ID != operatingUnitID {
			continue
		}
		if ctx.LocationID == "" || unit.LocationID == "" || unit.LocationID == ctx.LocationID {
			ctx.OperatingUnitID = unit.ID
			ctx.Source = "requested_operating_unit"
			if ctx.LocationID == "" {
				ctx.LocationID = unit.LocationID
			}
		}
	}
	return ctx
}

func (s *Service) UpsertOperatingUnit(unit OperatingUnit) (OperatingUnit, error) {
	now := time.Now().UTC()
	if unit.ID == "" {
		unit.ID = "ou:" + unit.Key
	}
	if unit.OrganizationID == "" {
		unit.OrganizationID = s.Root().ID
	}
	if unit.Status == "" {
		unit.Status = "active"
	}
	if unit.CreatedAt.IsZero() {
		unit.CreatedAt = now
	}
	unit.UpdatedAt = now
	return unit, s.repo.SaveOperatingUnit(unit)
}
