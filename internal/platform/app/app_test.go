package app

import (
	"context"
	"testing"
	"time"

	"clinic/internal/platform/config"
	"clinic/internal/platform/document"
	"clinic/internal/platform/identity"
	"clinic/internal/platform/model"
	"clinic/internal/platform/module"
	"clinic/internal/platform/organization"
	"clinic/internal/platform/policy"
	"clinic/internal/platform/reporting"
	"clinic/internal/platform/search"
	"clinic/internal/platform/workflow"
)

func TestNewAppBootstrap(t *testing.T) {
	app := New()
	if app.Address() == "" {
		t.Fatal("expected address")
	}
	if app.Handler() == nil {
		t.Fatal("expected handler")
	}
	if len(app.Observability.MetricDefinitions()) == 0 {
		t.Fatal("expected observability metric definitions")
	}
	if len(app.Analytics.ListReportDefinitions()) == 0 {
		t.Fatal("expected bootstrapped analytics report definitions")
	}
	app.StartBackground(context.Background())
	if err := app.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestSeedPlatformKernelSeedsEmptyServices(t *testing.T) {
	org := organization.NewService()
	cfg := config.NewService()
	ident := identity.NewServiceWithRepository(org, identity.NewMemoryRepository(nil, nil, nil, nil, nil, nil, nil, nil))
	modules := module.NewService()
	models := model.NewService()
	reportingSvc := reporting.NewService(models)
	searchSvc := search.NewService()
	docs := document.NewServiceWithRepository(document.NewMemoryRepository())
	flows := workflow.NewServiceWithRepository(workflow.NewMemoryRepository())
	policies := policy.NewService()

	seedPlatformKernel(cfg, ident, modules, models, reportingSvc, searchSvc, docs, flows, policies, "bootstrap-123!")

	if _, ok := ident.FindUserByUsername("admin"); !ok {
		t.Fatal("expected bootstrap admin user")
	}
	if _, err := ident.AuthenticatePassword("admin", "bootstrap-123!", "loc_hq", nil, defaultBootstrapSessionTTL()); err != nil {
		t.Fatalf("expected seeded bootstrap admin credential to authenticate: %v", err)
	}
	def, err := docs.Definition("generic_request")
	if err != nil {
		t.Fatalf("expected seeded document definition: %v", err)
	}
	if len(def.AllowedLinkTypes) == 0 || len(def.AllowedAttachmentTypes) == 0 {
		t.Fatalf("expected seeded document definition capabilities, got %+v", def)
	}
	if len(flows.ListKeys()) == 0 {
		t.Fatal("expected seeded workflow definition")
	}
	if len(policies.Definitions()) == 0 {
		t.Fatal("expected seeded policy hook definitions")
	}
	if _, ok := models.Definition("party"); !ok {
		t.Fatal("expected seeded model definition")
	}
	if len(reportingSvc.Definitions()) == 0 {
		t.Fatal("expected seeded dataset definitions")
	}
	if len(searchSvc.IndexDefinitions()) == 0 {
		t.Fatal("expected seeded search indexes")
	}
}

func defaultBootstrapSessionTTL() time.Duration {
	return 8 * time.Hour
}
