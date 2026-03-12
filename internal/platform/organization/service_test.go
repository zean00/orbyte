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
