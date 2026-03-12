package organization

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresOrganizationRepository(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	rootRows := sqlmock.NewRows([]string{"organization_id", "organization_key", "name", "status", "created_at", "updated_at"}).AddRow("org1", "default", "Org", "active", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT organization_id, organization_key, name, status, created_at, updated_at")).WillReturnRows(rootRows)
	if repo.Root().ID != "org1" {
		t.Fatal("expected root")
	}
	locRows := sqlmock.NewRows([]string{"location_id", "organization_id", "location_key", "name", "location_type", "status", "parent_location_id", "created_at", "updated_at"}).AddRow("loc1", "org1", "hq", "HQ", "location", "active", "", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT location_id, organization_id, location_key, name, location_type, status, COALESCE(parent_location_id, ''), created_at, updated_at")).WillReturnRows(locRows)
	if len(repo.Locations()) != 1 {
		t.Fatal("expected locations")
	}
}
