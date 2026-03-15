package app

import (
	"context"
	"os"
	"testing"
	"time"

	"orbyte/internal/modules"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func TestNewAppBootstrap(t *testing.T) {
	t.Setenv("APP_JWT_SECRET", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("APP_AUTH_DEV_MODE", "true")
	app, err := New(Options{Profile: modules.ProfileAll})
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
	if snapshot := app.RuntimeHealth.Snapshot(context.Background()); snapshot.Ready {
		t.Fatal("expected app to remain unready before background services start")
	}
	app.StartBackground(context.Background())
	if snapshot := app.RuntimeHealth.Snapshot(context.Background()); !snapshot.Ready {
		t.Fatalf("expected app to become ready after background start, got %+v", snapshot)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestNewAppFailsWhenDatabaseConfiguredButUnavailable(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://bad:bad@127.0.0.1:1/clinic?sslmode=disable")
	t.Setenv("APP_JWT_SECRET", "test-secret")
	if _, err := New(Options{Profile: modules.ProfileAll}); err == nil {
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
	referenceSvc := reference.NewService()
	searchSvc := search.NewService()
	docs := document.NewServiceWithRepository(document.NewMemoryRepository())
	flows := workflow.NewServiceWithRepository(workflow.NewMemoryRepository())
	policies := policy.NewService()

	if err := seedPlatformKernel(cfg, ident, modules, models, reportingSvc, referenceSvc, searchSvc, docs, flows, policies, nil, "bootstrap-123!"); err != nil {
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
	if len(referenceSvc.Types()) == 0 {
		t.Fatal("expected seeded reference types")
	}
}

func TestSeedPlatformKernelIsIdempotentForRestart(t *testing.T) {
	org := organization.NewService()
	cfg := config.NewService()
	ident := identity.NewServiceWithRepository(org, identity.NewMemoryRepository(nil, nil, nil, nil, nil, nil, nil, nil))
	modules := module.NewService()
	models := model.NewService()
	reportingSvc := reporting.NewService(models)
	referenceSvc := reference.NewService()
	searchSvc := search.NewService()
	docs := document.NewServiceWithRepository(document.NewMemoryRepository())
	flows := workflow.NewServiceWithRepository(workflow.NewMemoryRepository())
	policies := policy.NewService()

	if err := seedPlatformKernel(cfg, ident, modules, models, reportingSvc, referenceSvc, searchSvc, docs, flows, policies, nil, "bootstrap-123!"); err != nil {
		t.Fatalf("first seed failed: %v", err)
	}
	if err := seedPlatformKernel(cfg, ident, modules, models, reportingSvc, referenceSvc, searchSvc, docs, flows, policies, nil, "bootstrap-123!"); err != nil {
		t.Fatalf("second seed failed: %v", err)
	}
}

func TestValidateBusinessManifestsRejectsDuplicateKeys(t *testing.T) {
	builtIn := []module.Manifest{{Key: "platform.core"}}
	if err := validateBusinessManifests(builtIn, []module.Manifest{{Key: "platform.core"}}); err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestValidateBusinessManifestsRejectsMissingDependencies(t *testing.T) {
	builtIn := []module.Manifest{{Key: "platform.core"}}
	if err := validateBusinessManifests(builtIn, []module.Manifest{{
		Key: "clinic",
		DependencyRequirements: []module.DependencyRequirement{{
			ModuleKey: "documents",
			Kind:      module.DependencyKindRequired,
		}},
	}}); err == nil {
		t.Fatal("expected missing dependency error")
	}
}

func TestNewAppBootstrapsClinicProfileBusinessSlice(t *testing.T) {
	t.Setenv("APP_JWT_SECRET", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("APP_AUTH_DEV_MODE", "true")
	manifests, err := modules.ForProfile(modules.ProfileClinic)
	if err != nil {
		t.Fatalf("load clinic profile failed: %v", err)
	}
	app, err := New(Options{
		Profile:           modules.ProfileClinic,
		BusinessManifests: manifests,
	})
	if err != nil {
		t.Fatalf("unexpected startup error: %v", err)
	}
	defer func() {
		_ = app.Close()
	}()
	if _, ok := app.Models.Definition("patient_profile"); !ok {
		t.Fatal("expected clinic patient model definition")
	}
	if _, err := app.Documents.Definition("clinic_registration"); err != nil {
		t.Fatalf("expected clinic registration document definition: %v", err)
	}
	if _, err := app.Workflows.Get("clinic_registration_flow"); err != nil {
		t.Fatalf("expected clinic workflow definition: %v", err)
	}
	if _, ok := app.Reference.Type("appointment_type"); !ok {
		t.Fatal("expected seeded appointment reference type")
	}
	if _, ok := app.Modules.Get("clinic_registration"); !ok {
		t.Fatal("expected clinic module to be registered")
	}
}

func defaultBootstrapSessionTTL() time.Duration {
	return 8 * time.Hour
}
