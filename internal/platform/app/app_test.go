package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
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
	"orbyte/internal/platform/templateoutput"
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
	if app.Profile() != modules.ProfileAll {
		t.Fatalf("expected profile %q, got %q", modules.ProfileAll, app.Profile())
	}
	if len(app.BusinessModuleKeys()) != 0 {
		t.Fatalf("expected no business module keys for all profile bootstrap, got %+v", app.BusinessModuleKeys())
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
	templateSvc := templateoutput.NewService(docs, reportingSvc)
	flows := workflow.NewServiceWithRepository(workflow.NewMemoryRepository())
	policies := policy.NewService()

	if err := seedPlatformKernel(cfg, ident, modules, models, reportingSvc, templateSvc, referenceSvc, searchSvc, docs, flows, policies, nil, "bootstrap-123!"); err != nil {
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
	templateSvc := templateoutput.NewService(docs, reportingSvc)
	flows := workflow.NewServiceWithRepository(workflow.NewMemoryRepository())
	policies := policy.NewService()

	if err := seedPlatformKernel(cfg, ident, modules, models, reportingSvc, templateSvc, referenceSvc, searchSvc, docs, flows, policies, nil, "bootstrap-123!"); err != nil {
		t.Fatalf("first seed failed: %v", err)
	}
	if err := seedPlatformKernel(cfg, ident, modules, models, reportingSvc, templateSvc, referenceSvc, searchSvc, docs, flows, policies, nil, "bootstrap-123!"); err != nil {
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

func TestValidateBusinessManifestsAllowsLocalExtensionBeforeBase(t *testing.T) {
	builtIn := []module.Manifest{{Key: "platform.core", Role: module.ModuleRoleBase}}
	err := validateBusinessManifests(builtIn, []module.Manifest{
		{
			Key:  "ledger.local.id",
			Role: module.ModuleRoleLocalExtension,
			LocalExtension: module.LocalExtensionDefinition{
				BaseModuleKey: "ledger.base",
				LocalityType:  "country",
				LocalityCode:  "ID",
			},
			DependencyRequirements: []module.DependencyRequirement{{
				ModuleKey: "ledger.base",
				Kind:      module.DependencyKindRequired,
			}},
		},
		{
			Key:  "ledger.base",
			Role: module.ModuleRoleBase,
		},
	})
	if err != nil {
		t.Fatalf("expected out-of-order local extension manifests to validate, got %v", err)
	}
}

func TestValidateBusinessManifestsRejectsLocalExtensionTargetingNonBaseRole(t *testing.T) {
	builtIn := []module.Manifest{{Key: "platform.core", Role: module.ModuleRoleBase}}
	err := validateBusinessManifests(builtIn, []module.Manifest{
		{
			Key:  "ledger.local.id",
			Role: module.ModuleRoleLocalExtension,
			LocalExtension: module.LocalExtensionDefinition{
				BaseModuleKey: "ledger.base",
				LocalityType:  "country",
				LocalityCode:  "ID",
			},
			DependencyRequirements: []module.DependencyRequirement{{
				ModuleKey: "ledger.base",
				Kind:      module.DependencyKindRequired,
			}},
		},
		{
			Key:  "ledger.base",
			Role: module.ModuleRoleStandard,
		},
	})
	if err == nil {
		t.Fatal("expected non-base target role to fail validation")
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
	if app.Profile() != modules.ProfileClinic {
		t.Fatalf("expected clinic profile, got %q", app.Profile())
	}
	if keys := app.BusinessModuleKeys(); len(keys) == 0 || keys[0] != "clinic_registration" {
		t.Fatalf("expected clinic business module keys, got %+v", keys)
	}
}

func TestNewAppWiresAdminReadinessAndDevDocs(t *testing.T) {
	t.Setenv("APP_JWT_SECRET", "test-secret")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("APP_AUTH_DEV_MODE", "true")
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_BOOTSTRAP_ADMIN_PASSWORD", "admin123!")

	app, err := New(Options{Profile: modules.ProfileAll})
	if err != nil {
		t.Fatalf("unexpected startup error: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			t.Fatalf("unexpected close error: %v", err)
		}
	}()
	app.StartBackground(context.Background())

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar failed: %v", err)
	}
	client := &http.Client{Jar: jar}

	loginReq, err := http.NewRequest(http.MethodPost, server.URL+"/auth/login", bytes.NewReader([]byte(`{"username":"admin","password":"admin123!","location_id":"loc_hq"}`)))
	if err != nil {
		t.Fatalf("build login request failed: %v", err)
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	_ = loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d", loginResp.StatusCode)
	}

	readinessResp, err := client.Get(server.URL + "/admin/api/readiness")
	if err != nil {
		t.Fatalf("admin readiness request failed: %v", err)
	}
	defer readinessResp.Body.Close()
	if readinessResp.StatusCode != http.StatusOK {
		t.Fatalf("expected admin readiness to succeed, got %d", readinessResp.StatusCode)
	}
	var readiness map[string]any
	if err := json.NewDecoder(readinessResp.Body).Decode(&readiness); err != nil {
		t.Fatalf("decode admin readiness failed: %v", err)
	}
	if blocked, _ := readiness["blocked_for_apply"].(bool); blocked {
		t.Fatalf("expected healthy app not to be blocked for apply, got %+v", readiness)
	}
	health, _ := readiness["health"].(map[string]any)
	if ready, _ := health["ready"].(bool); !ready {
		t.Fatalf("expected admin readiness to reuse runtime health snapshot, got %+v", readiness)
	}

	openapiResp, err := client.Get(server.URL + "/dev/openapi.json")
	if err != nil {
		t.Fatalf("openapi request failed: %v", err)
	}
	defer openapiResp.Body.Close()
	if openapiResp.StatusCode != http.StatusOK {
		t.Fatalf("expected openapi route to succeed, got %d", openapiResp.StatusCode)
	}
	var openapi map[string]any
	if err := json.NewDecoder(openapiResp.Body).Decode(&openapi); err != nil {
		t.Fatalf("decode openapi failed: %v", err)
	}
	runtimeMeta, _ := openapi["x-orbyte-runtime"].(map[string]any)
	if !containsStringAny(runtimeMeta["registered_modules"], "platform.core") {
		t.Fatalf("expected runtime modules metadata, got %+v", runtimeMeta)
	}
	if !containsStringAny(runtimeMeta["document_types"], "generic_request") {
		t.Fatalf("expected runtime document metadata, got %+v", runtimeMeta)
	}
	if !containsStringAny(runtimeMeta["search_indexes"], "documents.requests.search") {
		t.Fatalf("expected runtime search metadata, got %+v", runtimeMeta)
	}
}

func containsStringAny(raw any, expected string) bool {
	items, _ := raw.([]any)
	for _, item := range items {
		if value, ok := item.(string); ok && value == expected {
			return true
		}
	}
	return false
}

func defaultBootstrapSessionTTL() time.Duration {
	return 8 * time.Hour
}
