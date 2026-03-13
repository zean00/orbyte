package app

import (
	"context"
	"os"
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
	t.Setenv("APP_JWT_SECRET", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("APP_AUTH_DEV_MODE", "true")
	app, err := New()
	if err != nil {
		t.Fatalf("unexpected startup error: %v", err)
	}
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

func TestNewAppFailsWhenDatabaseConfiguredButUnavailable(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://bad:bad@127.0.0.1:1/clinic?sslmode=disable")
	t.Setenv("APP_JWT_SECRET", "test-secret")
	if _, err := New(); err == nil {
		t.Fatal("expected startup error when DATABASE_URL is configured but unavailable")
	}
}

func TestNewAppFailsWhenJWTSecretMissingInDatabaseMode(t *testing.T) {
	originalOpen := os.Getenv("DATABASE_URL")
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:5432/clinic?sslmode=disable")
	t.Setenv("APP_JWT_SECRET", "")
	t.Setenv("APP_AUTH_DEV_MODE", "true")
	defer func() {
		_ = os.Setenv("DATABASE_URL", originalOpen)
	}()
	if err := ensureJWTSecret(true); err == nil {
		t.Fatal("expected startup error when APP_JWT_SECRET is missing in database mode")
	}
}

func TestNewAppFailsWhenJWTSecretMissingWithoutExplicitDevMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("APP_JWT_SECRET", "")
	t.Setenv("APP_AUTH_DEV_MODE", "")
	if err := ensureJWTSecret(false); err == nil {
		t.Fatal("expected startup error without jwt secret outside explicit dev mode")
	}
}

func TestNewAppSeedsRandomDevelopmentJWTSecretWithExplicitDevMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("APP_JWT_SECRET", "")
	t.Setenv("APP_AUTH_DEV_MODE", "true")
	if err := ensureJWTSecret(false); err != nil {
		t.Fatalf("unexpected ensure jwt secret error: %v", err)
	}
	first := os.Getenv("APP_JWT_SECRET")
	if first == "" {
		t.Fatal("expected development jwt secret to be seeded")
	}
	t.Setenv("APP_JWT_SECRET", "")
	if err := ensureJWTSecret(false); err != nil {
		t.Fatalf("unexpected ensure jwt secret error: %v", err)
	}
	second := os.Getenv("APP_JWT_SECRET")
	if second == "" {
		t.Fatal("expected second development jwt secret to be seeded")
	}
	if first == second {
		t.Fatal("expected random per-process development jwt secret")
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

	if err := seedPlatformKernel(cfg, ident, modules, models, reportingSvc, searchSvc, docs, flows, policies, "bootstrap-123!"); err != nil {
		t.Fatalf("seed platform kernel failed: %v", err)
	}

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

func TestSeedPlatformKernelIsIdempotentForRestart(t *testing.T) {
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

	if err := seedPlatformKernel(cfg, ident, modules, models, reportingSvc, searchSvc, docs, flows, policies, "bootstrap-123!"); err != nil {
		t.Fatalf("first seed failed: %v", err)
	}
	if err := seedPlatformKernel(cfg, ident, modules, models, reportingSvc, searchSvc, docs, flows, policies, "bootstrap-123!"); err != nil {
		t.Fatalf("second seed failed: %v", err)
	}
}

func defaultBootstrapSessionTTL() time.Duration {
	return 8 * time.Hour
}
