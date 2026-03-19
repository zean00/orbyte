package organization

import "testing"

func TestServiceResolve(t *testing.T) {
	svc := NewService()
	root := svc.Root()
	if root.ID == "" {
		t.Fatal("expected root organization")
	}
	ctx := svc.Resolve("loc_hq")
	if ctx.LocationID != "loc_hq" {
		t.Fatalf("expected loc_hq, got %s", ctx.LocationID)
	}
	if len(svc.Locations()) != 1 {
		t.Fatal("expected one location")
	}
}

func TestMemoryRepositoryCopiesLocations(t *testing.T) {
	repo := NewMemoryRepository(Organization{ID: "org"}, []Location{{ID: "loc1"}})
	items := repo.Locations()
	items[0].ID = "changed"
	if repo.Locations()[0].ID != "loc1" {
		t.Fatal("expected repository copy semantics")
	}
}

func TestServiceOperatingUnitsAndUpsert(t *testing.T) {
	svc := NewService()
	if len(svc.OperatingUnits()) != 1 {
		t.Fatalf("expected seeded operating units, got %+v", svc.OperatingUnits())
	}

	ctx := svc.ResolveScope("", "ou_hq_ops")
	if ctx.OperatingUnitID != "ou_hq_ops" || ctx.LocationID != "loc_hq" {
		t.Fatalf("unexpected resolved scope: %+v", ctx)
	}

	unit, err := svc.UpsertOperatingUnit(OperatingUnit{
		Key:        "branch_ops",
		Name:       "Branch Ops",
		LocationID: "loc_hq",
	})
	if err != nil {
		t.Fatalf("upsert operating unit failed: %v", err)
	}
	if unit.ID == "" || unit.OrganizationID != "org_default" || unit.Status != "active" {
		t.Fatalf("unexpected upserted unit: %+v", unit)
	}
	if len(svc.OperatingUnits()) != 2 {
		t.Fatalf("expected two operating units, got %+v", svc.OperatingUnits())
	}
}

func TestMemoryRepositoryCopiesOperatingUnits(t *testing.T) {
	repo := NewMemoryRepository(Organization{ID: "org"}, nil, []OperatingUnit{{ID: "ou1"}})
	items := repo.OperatingUnits()
	items[0].ID = "changed"
	if repo.OperatingUnits()[0].ID != "ou1" {
		t.Fatal("expected operating unit copy semantics")
	}
	unit := OperatingUnit{ID: "ou2", OrganizationID: "org", Key: "ops", Name: "Ops", Status: "active"}
	if err := repo.SaveOperatingUnit(unit); err != nil {
		t.Fatalf("save operating unit failed: %v", err)
	}
	if len(repo.OperatingUnits()) != 2 {
		t.Fatalf("expected saved operating unit, got %+v", repo.OperatingUnits())
	}
}
