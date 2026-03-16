package httpx

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/analytics"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/monitoring"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/workflow"

	"github.com/lestrrat-go/jwx/v3/jwk"
)

type testHarness struct {
	router    http.Handler
	cookie    *http.Cookie
	csrf      *http.Cookie
	ident     *identity.Service
	audit     *audit.Service
	cfg       *config.Service
	search    *search.Service
	analytics *analytics.Service
}

func newTestHarness(t *testing.T) testHarness {
	return newTestHarnessWithConfig(t, nil)
}

func newTestHarnessWithConfig(t *testing.T, entries []config.Entry) testHarness {
	t.Helper()
	t.Setenv("APP_JWT_SECRET", "test-secret")
	t.Setenv("APP_JWT_ISSUER", "test-suite")

	cfg := config.NewService()
	if len(entries) > 0 {
		cfg = config.NewServiceWithRepository(config.NewMemoryRepository(entries))
	}
	org := organization.NewService()
	ident := identity.NewService(org)
	modules := module.NewService()
	models := model.NewService()
	activities := activity.NewService()
	reportingSvc := reporting.NewService(models)
	referenceSvc := reference.NewService()
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	searchSvc := search.NewService()
	searchSvc.AttachSources(docs, models)
	reportingSvc.AttachDocumentSources(docs, searchSvc)
	loggerSvc := logging.NewServiceWithWriter(nil)
	obsSvc := observability.NewService()
	policySvc := policy.NewServiceWithConfig(cfg)
	fieldSecuritySvc := securityfields.NewService(policySvc)
	analyticsSvc := analytics.NewService(docs, flows, eventingSvc, searchSvc, auditSvc, obsSvc)
	monitoringSvc := monitoring.NewService(docs, eventingSvc, flows, searchSvc, obsSvc)
	integrationSvc := integration.NewService(obsSvc, loggerSvc)
	jobSvc := jobs.NewService()
	health := runtimehealth.NewTracker()
	health.SetBootstrapped(true)
	health.SetBackgroundStarted(true)
	searchSvc.AttachJobs(jobSvc)
	searchSvc.AttachFieldSecurity(fieldSecuritySvc)
	analyticsSvc.AttachJobs(jobSvc)
	jobCtx, cancelJobs := context.WithCancel(context.Background())
	jobSvc.Start(jobCtx)
	t.Cleanup(func() {
		cancelJobs()
		jobSvc.Stop()
	})
	integrationSvc.AttachPolicy(policySvc)
	integrationSvc.AttachJobs(jobSvc)
	for _, eventType := range []string{
		"document.updated",
		"document.submitted",
		"document.approved",
		"document.reject",
		"document.reopened",
		"document.cancelled",
	} {
		eventingSvc.RegisterHandler(eventType, eventing.NewDocumentProjectionHandler(docs, searchSvc))
	}
	actions := application.NewDocumentActions(docs, flows, policySvc, application.NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))
	modelActions := application.NewMemoryModelActions(models, activities, auditSvc, eventingSvc)
	tokenManager := identity.NewTokenManagerFromEnv()
	token, err := tokenManager.IssueSessionToken(ident.Sessions()[0])
	if err != nil {
		t.Fatalf("issue session token failed: %v", err)
	}
	csrfCookie, err := buildCSRFCookie(ident.Sessions()[0].ID)
	if err != nil {
		t.Fatalf("issue csrf cookie failed: %v", err)
	}
	for _, manifest := range builtInTestModuleManifests() {
		if err := modules.Register(manifest, "system"); err != nil {
			t.Fatalf("register module failed: %v", err)
		}
		for _, def := range manifest.ConfigDefinitions {
			if err := cfg.RegisterDefinition(def); err != nil {
				t.Fatalf("register config definition failed: %v", err)
			}
		}
		for _, def := range manifest.ReferenceTypes {
			if err := referenceSvc.RegisterType(def); err != nil {
				t.Fatalf("register reference type failed: %v", err)
			}
		}
		for _, record := range manifest.ReferenceRecords {
			if err := referenceSvc.UpsertRecord(record); err != nil {
				t.Fatalf("register reference record failed: %v", err)
			}
		}
		for _, ext := range manifest.DocumentExtensions {
			if err := docs.RegisterExtension(document.ExtensionDefinition{
				DocumentType:  ext.DocumentType,
				ModuleKey:     manifest.Key,
				DisplayName:   ext.DisplayName,
				SchemaVersion: ext.SchemaVersion,
			}); err != nil {
				t.Fatalf("register document extension failed: %v", err)
			}
		}
		for _, index := range manifest.SearchIndexes {
			if err := searchSvc.RegisterIndex(index); err != nil {
				t.Fatalf("register search index failed: %v", err)
			}
		}
	}
	for _, hook := range []policy.HookDefinition{
		{Key: "documents.extension.view", Kind: "access", Target: "document_extension_view", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"allowed_modules": []string{}, "denied_statuses": []string{"cancelled"}}},
		{Key: "documents.extension.write", Kind: "access", Target: "document_extension_write", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"allowed_modules": []string{}, "denied_statuses": []string{"cancelled"}}},
		{Key: "documents.workflow.transition", Kind: "workflow", Target: "document_transition", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"blocked_actions": []string{}, "allowed_actions": []string{}, "allowed_statuses": []string{}}},
		{Key: "documents.search.visibility", Kind: "search", Target: "document_search", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"hidden_statuses": []string{}, "allowed_types": []string{}}},
		{Key: "documents.fields.profile", Kind: "security", Target: "document_fields", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"fields": map[string]any{}}},
		{Key: "documents.numbering.assign", Kind: "numbering", Target: "document_numbering", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"prefix": "", "include_location": true, "include_date": true}},
		{Key: "documents.action.render", Kind: "ui", Target: "document_action_render", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"hidden_actions": []string{}, "primary_actions": []string{"submit", "approve"}}},
		{Key: "integration.submission.preflight", Kind: "integration", Target: "integration_submission", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"blocked_operation_types": []string{}, "required_system_status": "active"}},
		{Key: "models.fields.profile", Kind: "security", Target: "model_fields", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"fields": map[string]any{}}},
	} {
		if err := policySvc.Register(hook); err != nil {
			t.Fatalf("register policy hook failed: %v", err)
		}
	}
	if err := policySvc.SetEvaluator("documents.extension.view", func(req policy.Request) policy.Decision { return policy.Decision{Allowed: true} }); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	if err := policySvc.SetEvaluator("documents.extension.write", func(req policy.Request) policy.Decision { return policy.Decision{Allowed: true} }); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	if err := policySvc.SetEvaluator("documents.workflow.transition", func(req policy.Request) policy.Decision { return policy.Decision{Allowed: true} }); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	if err := policySvc.SetEvaluator("documents.search.visibility", func(req policy.Request) policy.Decision { return policy.Decision{Allowed: true} }); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	if err := policySvc.SetEvaluator("documents.fields.profile", func(req policy.Request) policy.Decision {
		fields := map[string]any{}
		channel, _ := req.Inputs["channel"].(string)
		switch channel {
		case "api", "ui", "search", "report":
			fields["patient_ssn"] = map[string]any{"visible": false, "editable": true, "export_visible": false, "search_visible": false}
			fields["extensions.analytics.secret_score"] = map[string]any{"visible": false, "editable": true, "export_visible": false, "search_visible": false}
			fields["locked_internal_note"] = map[string]any{"visible": false, "editable": false, "export_visible": false, "search_visible": false}
			fields["extensions.analytics.locked_secret"] = map[string]any{"visible": false, "editable": false, "export_visible": false, "search_visible": false}
		}
		return policy.Decision{Allowed: true, Output: map[string]any{"fields": fields}}
	}); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	if err := policySvc.SetEvaluator("documents.numbering.assign", func(req policy.Request) policy.Decision {
		return policy.Decision{Allowed: true, Output: map[string]any{"number": "TEST-0001"}}
	}); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	if err := policySvc.SetEvaluator("documents.action.render", func(req policy.Request) policy.Decision {
		action, _ := req.Inputs["action"].(string)
		if action == "submit" || action == "approve" {
			return policy.Decision{Allowed: true, Output: map[string]any{"placement": "primary"}}
		}
		return policy.Decision{Allowed: true, Output: map[string]any{"placement": "secondary"}}
	}); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	if err := policySvc.SetEvaluator("integration.submission.preflight", func(req policy.Request) policy.Decision {
		return policy.Decision{Allowed: true}
	}); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	if err := policySvc.SetEvaluator("models.fields.profile", func(req policy.Request) policy.Decision {
		return policy.Decision{Allowed: true}
	}); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	if err := models.Register(model.Definition{
		Key:                 "party",
		DisplayName:         "Party",
		Version:             "v1",
		CreatePermissionKey: "party.create",
		ListPermissionKey:   "party.list",
		ReadPermissionKey:   "party.read",
		UpdatePermissionKey: "party.update",
		DefaultSort:         "name",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string", Required: true},
			{Key: "email", Type: "string", Sensitive: true, DefaultMask: "partial_email", ReadPermissionKey: "party.email.read"},
			{Key: "internal_note", Type: "string", Sensitive: true, WritePermissionKey: "party.note.write"},
			{Key: "status", Type: "string", DefaultValue: "active"},
		},
		Relations: []model.RelationDefinition{
			{Key: "contacts", Type: "has_many", TargetModelKey: "party_contact", ForeignKey: "party_id"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	if err := models.Register(model.Definition{
		Key:                 "party_contact",
		DisplayName:         "Party Contact",
		Version:             "v1",
		CreatePermissionKey: "party.update",
		ListPermissionKey:   "party.read",
		ReadPermissionKey:   "party.read",
		UpdatePermissionKey: "party.update",
		DefaultSort:         "name",
		Fields: []model.FieldDefinition{
			{Key: "party_id", Type: "string", Required: true},
			{Key: "name", Type: "string", Required: true},
			{Key: "phone", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register related model failed: %v", err)
	}
	if _, err := models.Create("party", "user_admin", map[string]any{"name": "Walk In Customer", "email": "walkin@clinic.local", "status": "active"}); err != nil {
		t.Fatalf("seed model failed: %v", err)
	}
	for _, permissionKey := range []string{"party.create", "party.list", "party.read", "party.update"} {
		if err := ident.UpsertPermission(identity.Permission{Key: permissionKey, Module: "masterdata", Action: "manage", Resource: "party"}); err != nil {
			t.Fatalf("upsert model permission failed: %v", err)
		}
		if err := ident.GrantRolePermission(identity.RolePermission{RoleID: "role_admin", PermissionKey: permissionKey}); err != nil {
			t.Fatalf("grant model permission failed: %v", err)
		}
	}
	if err := ident.UpsertPermission(identity.Permission{Key: "search.manage", Module: "platform.core", Action: "manage", Resource: "search"}); err != nil {
		t.Fatalf("upsert search permission failed: %v", err)
	}
	if err := ident.GrantRolePermission(identity.RolePermission{RoleID: "role_admin", PermissionKey: "search.manage"}); err != nil {
		t.Fatalf("grant search permission failed: %v", err)
	}
	if err := reportingSvc.Register(reporting.DatasetDefinition{
		Key:        "masterdata.party.summary",
		Title:      "Party Summary",
		SourceKind: "model",
		ModelKey:   "party",
		Dimensions: []reporting.DimensionDefinition{{Key: "by_status", Label: "By Status", Path: "status"}},
		Measures:   []reporting.MeasureDefinition{{Key: "total", Label: "Total", Kind: "count"}},
	}); err != nil {
		t.Fatalf("register dataset failed: %v", err)
	}
	return testHarness{
		router:    NewRouter(cfg, org, ident, modules, models, activities, reportingSvc, referenceSvc, docs, flows, auditSvc, eventingSvc, searchSvc, loggerSvc, analyticsSvc, monitoringSvc, obsSvc, policySvc, integrationSvc, jobSvc, health, actions, modelActions),
		cookie:    &http.Cookie{Name: sessionCookieName, Value: token},
		csrf:      csrfCookie,
		ident:     ident,
		audit:     auditSvc,
		cfg:       cfg,
		search:    searchSvc,
		analytics: analyticsSvc,
	}
}

func (h testHarness) registerSearchIndex(def search.IndexDefinition) error {
	if h.search == nil {
		return nil
	}
	return h.search.RegisterIndex(def)
}

func builtInTestModuleManifests() []module.Manifest {
	cfg := config.NewService()
	httpDef, _ := cfg.Definition("platform.http")
	authDef, _ := cfg.Definition("identity.auth")
	searchTypesenseDef, _ := cfg.Definition("search.typesense")
	searchEmbeddingDef, _ := cfg.Definition("search.embedding")
	return []module.Manifest{
		{Key: "platform.core", Name: "Platform Core", Version: "1.0.0", DomainFamily: "platform", ConfigDefinitions: []config.Definition{httpDef, searchTypesenseDef, searchEmbeddingDef}},
		{Key: "identity", Name: "Identity", Version: "1.0.0", DomainFamily: "platform", DependencyRequirements: []module.DependencyRequirement{{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired}}, ConfigDefinitions: []config.Definition{authDef}},
		{
			Key:          "documents",
			Name:         "Documents",
			Version:      "1.1.0",
			DomainFamily: "platform",
			DependencyRequirements: []module.DependencyRequirement{
				{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
				{ModuleKey: "identity", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			},
			OwnedDocumentTypes: []string{"generic_request"},
			Security: module.SecurityDefinition{
				PolicyHooks: []module.PolicyHookDefinition{
					{Key: "documents.extension.view", Kind: "access", Target: "document_extension_view"},
					{Key: "documents.extension.write", Kind: "access", Target: "document_extension_write"},
					{Key: "documents.workflow.transition", Kind: "workflow", Target: "document_transition"},
					{Key: "documents.search.visibility", Kind: "search", Target: "document_search"},
					{Key: "documents.numbering.assign", Kind: "numbering", Target: "document_numbering"},
				},
			},
			SearchIndexes: []search.IndexDefinition{{
				Key:                 "documents.requests.search",
				Title:               "Requests",
				SourceKind:          "document",
				DocumentType:        "generic_request",
				Modes:               []string{"keyword", "vector", "hybrid"},
				OrganizationSplit:   true,
				RequiredPermissions: []string{"document.list"},
				QueryFilterFields:   []string{"status", "location_id", "document_type"},
				QuerySortFields:     []string{"status", "title"},
				Fields: []search.IndexFieldDefinition{
					{Key: "document_type", Path: "header.type", Type: "string", Facet: true},
					{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
					{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true},
				},
				VectorFields: []search.VectorFieldDefinition{{Key: "semantic", SourcePaths: []string{"body.payload.title"}, EmbeddingMode: "external", Dimensions: 8}},
			}},
			Frontend: module.FrontendDefinition{
				Menus: []module.MenuDefinition{{
					Key:                 "documents.requests",
					Label:               "Requests",
					ActionKey:           "documents.requests.list",
					Order:               10,
					RequiredPermissions: []string{"document.list"},
				}},
				Actions: []module.ActionDefinition{
					{
						Key:                 "documents.requests.list",
						Label:               "Requests",
						Kind:                "navigate",
						RoutePath:           "/documents",
						ViewKey:             "documents.requests.list",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.list"},
					},
					{
						Key:                 "documents.requests.detail",
						Label:               "Request Detail",
						Kind:                "navigate",
						RoutePath:           "/documents/detail",
						ViewKey:             "documents.requests.detail",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.read"},
					},
				},
				Views: []module.ViewDefinition{
					{
						Key:                 "documents.requests.list",
						Title:               "Requests",
						Kind:                "list",
						DocumentType:        "generic_request",
						ProjectionKey:       "document_summary",
						RequiredPermissions: []string{"document.list"},
						Columns: []module.ColumnDefinition{
							{Key: "id", Label: "Document", Path: "header.id"},
							{Key: "status", Label: "Status", Path: "header.status"},
						},
					},
					{
						Key:                 "documents.requests.detail",
						Title:               "Request Detail",
						Kind:                "detail",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.read"},
						AllowedActions:      []string{"submit", "approve", "reject", "reopen", "cancel"},
						Tabs: []module.TabDefinition{{
							Key: "summary", Title: "Summary", Sections: []module.SectionDefinition{{
								Key: "header", Title: "Header", Fields: []module.FieldDefinition{{Key: "doc_id", Label: "Document ID", Path: "header.id", Type: "string"}},
							}},
						}},
					},
				},
			},
			Offline: module.OfflineDefinition{
				Projections: []module.OfflineProjectionDefinition{{
					IndexKey:             "documents.requests.search",
					Title:                "Requests",
					RequiredPermissions:  []string{"document.list"},
					DefaultFilters:       []string{"status=draft"},
					DefaultIncludeFields: []string{"document_id", "status", "title"},
				}},
				Documents: []module.OfflineDocumentDefinition{{
					Type:                "generic_request",
					Title:               "Generic Request",
					CreatePermissionKey: "document.create",
					UpdatePermissionKey: "document.update_draft",
					RequiredPermissions: []string{"document.read"},
				}},
			},
		},
		{
			Key:                    "analytics",
			Name:                   "Analytics",
			Version:                "1.0.0",
			DomainFamily:           "platform",
			DependencyRequirements: []module.DependencyRequirement{{ModuleKey: "documents", VersionRange: ">=1.1.0,<2.0.0", Kind: module.DependencyKindRequired}},
			DocumentExtensions: []module.DocumentExtension{{
				DocumentType: "generic_request", SchemaVersion: "v1", DisplayName: "Analytics Extension", ReadPermissionKey: "analytics.read", WritePermissionKey: "analytics.manage_reports",
			}},
			Frontend: module.FrontendDefinition{
				Menus: []module.MenuDefinition{{
					Key:                 "analytics.cockpit",
					Label:               "Analytics Cockpit",
					ActionKey:           "analytics.cockpit",
					Order:               20,
					RequiredPermissions: []string{"analytics.read"},
				}},
				Actions: []module.ActionDefinition{{
					Key:                 "analytics.cockpit",
					Label:               "Analytics Cockpit",
					Kind:                "navigate",
					RoutePath:           "/analytics/cockpit",
					CustomEntryKey:      "analytics.cockpit",
					RenderMode:          module.RenderModeCustom,
					RequiredPermissions: []string{"analytics.read"},
				}},
				CustomEntries: []module.CustomEntryDefinition{{
					Key:                 "analytics.cockpit",
					Title:               "Analytics Cockpit",
					RoutePath:           "/analytics/cockpit",
					BundleKey:           "analytics-cockpit",
					ComponentExport:     "render",
					RequiredPermissions: []string{"analytics.read"},
				}},
			},
			MCP: module.MCPDefinition{
				Tools: []module.MCPToolDefinition{{
					Key:                 "analytics.snapshot.get",
					Title:               "Get Analytics Snapshot",
					Operation:           "analytics.snapshot.get",
					RequiredPermissions: []string{"analytics.read"},
					AppKey:              "analytics.cockpit",
				}},
				Resources: []module.MCPResourceDefinition{
					{Key: "analytics.snapshot.current", Title: "Current Analytics Snapshot", URI: "orbyte://analytics/snapshot/current", MIMEType: "application/json", Provider: "analytics.snapshot.current", RequiredPermissions: []string{"analytics.read"}},
					{Key: "analytics.cockpit.app", Title: "Analytics Cockpit App", URI: "orbyte://apps/analytics.cockpit", MIMEType: "text/html", Provider: "mcp.app", RequiredPermissions: []string{"analytics.read"}, AppKey: "analytics.cockpit"},
				},
				Apps: []module.MCPAppDefinition{{
					Key:                 "analytics.cockpit",
					Title:               "Analytics Cockpit",
					ResourceKey:         "analytics.cockpit.app",
					CustomEntryKey:      "analytics.cockpit",
					RequiredPermissions: []string{"analytics.read"},
				}},
			},
			Bundles: []module.BundleDefinition{{
				Key:    "analytics-cockpit",
				Script: AnalyticsCockpitBundle(),
			}},
		},
		{
			Key:          "reference_masterdata",
			Name:         "Reference Master Data",
			Version:      "1.0.0",
			DomainFamily: "platform",
			ReferenceTypes: []reference.TypeDefinition{
				{Key: "appointment_type", DisplayName: "Appointment Type", OwnerModuleKey: "reference_masterdata"},
			},
			ReferenceRecords: []reference.Record{
				{TypeKey: "appointment_type", Key: "consultation", DisplayName: "Consultation", Scope: "deployment", UpdatedAt: time.Now().UTC(), UpdatedBy: "system", Value: map[string]any{"reference_type": "appointment_type"}},
			},
			Offline: module.OfflineDefinition{
				References: []module.OfflineReferenceDefinition{
					{TypeKey: "appointment_type", Title: "Appointment Types"},
				},
			},
		},
		{
			Key:                    "monitoring",
			Name:                   "Monitoring",
			Version:                "1.0.0",
			DomainFamily:           "platform",
			DependencyRequirements: []module.DependencyRequirement{{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired}},
			Frontend: module.FrontendDefinition{
				Menus:   []module.MenuDefinition{{Key: "monitoring.overview", Label: "Monitoring", ActionKey: "monitoring.overview", Order: 30, RequiredPermissions: []string{"monitoring.read"}}},
				Actions: []module.ActionDefinition{{Key: "monitoring.overview", Label: "Monitoring Overview", Kind: "navigate", RoutePath: "/monitoring", ViewKey: "monitoring.overview", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"monitoring.read"}}},
				Views:   []module.ViewDefinition{{Key: "monitoring.overview", Title: "Monitoring Overview", Kind: "dashboard", ProjectionKey: "monitoring.summary", RequiredPermissions: []string{"monitoring.read"}, Cards: []module.CardDefinition{{Key: "docs", Label: "Documents", Path: "documents.total"}}}},
			},
		},
	}
}

func (h testHarness) request(method, path string, body []byte, authenticated bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.RemoteAddr = "192.0.2.10:1234"
	if authenticated && h.cookie != nil {
		req.AddCookie(h.cookie)
		if h.csrf != nil {
			req.AddCookie(h.csrf)
		}
		if requiresCSRFProtection(method) && h.csrf != nil {
			req.Header.Set("X-CSRF-Token", h.csrf.Value)
		}
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

func findCookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestLoginLogoutAndSessionRevocation(t *testing.T) {
	h := newTestHarness(t)

	loginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr := h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for login, got %d body=%s", rr.Code, rr.Body.String())
	}
	loginCookie := rr.Result().Cookies()[0]
	if loginCookie.Name != sessionCookieName || !loginCookie.HttpOnly {
		t.Fatalf("expected secure session cookie, got %+v", loginCookie)
	}

	authReq := httptest.NewRequest(http.MethodGet, "/platform/context", nil)
	authReq.AddCookie(loginCookie)
	authRR := httptest.NewRecorder()
	h.router.ServeHTTP(authRR, authReq)
	if authRR.Code != http.StatusOK {
		t.Fatalf("expected authenticated route after login, got %d", authRR.Code)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutReq.AddCookie(loginCookie)
	loginRespCookie := findCookieByName(rr.Result().Cookies(), csrfCookieName)
	if loginRespCookie == nil {
		t.Fatal("expected csrf cookie on login")
	}
	logoutReq.AddCookie(loginRespCookie)
	logoutReq.Header.Set("X-CSRF-Token", loginRespCookie.Value)
	logoutRR := httptest.NewRecorder()
	h.router.ServeHTTP(logoutRR, logoutReq)
	if logoutRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for logout, got %d body=%s", logoutRR.Code, logoutRR.Body.String())
	}
	cleared := logoutRR.Result().Cookies()[0]
	if cleared.Name != sessionCookieName || cleared.MaxAge != -1 {
		t.Fatalf("expected cleared cookie, got %+v", cleared)
	}

	rejectedReq := httptest.NewRequest(http.MethodGet, "/platform/context", nil)
	rejectedReq.AddCookie(loginCookie)
	rejectedRR := httptest.NewRecorder()
	h.router.ServeHTTP(rejectedRR, rejectedReq)
	if rejectedRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked session to be unauthorized, got %d", rejectedRR.Code)
	}

	events := h.audit.List()
	seenLogin := false
	seenLogout := false
	for _, event := range events {
		if event.Action == "auth.login" {
			seenLogin = true
		}
		if event.Action == "auth.logout" {
			seenLogout = true
		}
	}
	if !seenLogin || !seenLogout {
		t.Fatalf("expected auth audit events, got %+v", events)
	}
}

func TestGoogleLogin(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	pubKey, err := jwk.PublicKeyOf(key)
	if err != nil {
		t.Fatalf("public jwk failed: %v", err)
	}
	if err := pubKey.Set(jwk.KeyIDKey, "kid-1"); err != nil {
		t.Fatalf("set key id failed: %v", err)
	}
	set := jwk.NewSet()
	if err := set.AddKey(pubKey); err != nil {
		t.Fatalf("add key failed: %v", err)
	}
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(set)
	}))
	defer jwksServer.Close()

	h := newTestHarnessWithConfig(t, []config.Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value: map[string]any{
			"password_min_length":             8,
			"session_ttl_minutes":             60,
			"session_refresh_window_minutes":  10,
			"login_rate_limit_attempts":       5,
			"login_rate_limit_window_seconds": 60,
			"trusted_origins":                 []string{},
			"google_enabled":                  true,
			"google_client_id":                "client-123",
			"google_client_secret":            "secret-123",
			"google_redirect_url":             "https://app.example.com/auth/google/callback",
			"google_auth_url":                 "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                "https://oauth2.googleapis.com/token",
			"google_issuer":                   "https://accounts.google.com",
			"google_jwks_url":                 jwksServer.URL,
			"google_hosted_domain":            "",
			"google_timeout_seconds":          5,
		},
	}})
	if _, err := h.ident.CreateUser("user@example.com", "user-pass-123", "loc_hq", "role_admin", "deployment", ""); err != nil {
		t.Fatalf("create google-mapped user failed: %v", err)
	}
	token := signGoogleTestToken(t, key, "kid-1", map[string]any{
		"iss":            "https://accounts.google.com",
		"sub":            "sub-123",
		"aud":            "client-123",
		"email":          "user@example.com",
		"email_verified": true,
		"exp":            time.Now().UTC().Add(time.Hour).Unix(),
		"iat":            time.Now().UTC().Unix(),
	})
	body, _ := json.Marshal(map[string]any{"id_token": token, "location_id": "loc_hq"})
	rr := h.request(http.MethodPost, "/auth/google", body, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for google login, got %d body=%s", rr.Code, rr.Body.String())
	}
	linked, ok := h.ident.FindUserByUsername("user@example.com")
	if !ok || linked.AuthenticationSubject != "google:sub-123" {
		t.Fatalf("expected google subject linkage, got %+v", linked)
	}
}

func TestGoogleLoginAutoProvisionsUser(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	pubKey, err := jwk.PublicKeyOf(key)
	if err != nil {
		t.Fatalf("public jwk failed: %v", err)
	}
	if err := pubKey.Set(jwk.KeyIDKey, "kid-1"); err != nil {
		t.Fatalf("set key id failed: %v", err)
	}
	set := jwk.NewSet()
	if err := set.AddKey(pubKey); err != nil {
		t.Fatalf("add key failed: %v", err)
	}
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(set)
	}))
	defer jwksServer.Close()

	h := newTestHarnessWithConfig(t, []config.Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value: map[string]any{
			"password_min_length":                       8,
			"session_ttl_minutes":                       60,
			"session_refresh_window_minutes":            10,
			"login_rate_limit_attempts":                 5,
			"login_rate_limit_window_seconds":           60,
			"trusted_origins":                           []string{},
			"password_enabled":                          true,
			"google_enabled":                            true,
			"google_auto_provision_enabled":             true,
			"google_auto_provision_allowed_domains":     []string{"example.com"},
			"google_auto_provision_role_id":             "role_admin",
			"google_auto_provision_scope_type":          "deployment",
			"google_auto_provision_scope_id":            "",
			"google_auto_provision_default_location_id": "loc_hq",
			"google_client_id":                          "client-123",
			"google_client_secret":                      "secret-123",
			"google_redirect_url":                       "https://app.example.com/auth/google/callback",
			"google_auth_url":                           "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                          "https://oauth2.googleapis.com/token",
			"google_issuer":                             "https://accounts.google.com",
			"google_jwks_url":                           jwksServer.URL,
			"google_hosted_domain":                      "",
			"google_timeout_seconds":                    5,
		},
	}})
	token := signGoogleTestToken(t, key, "kid-1", map[string]any{
		"iss":            "https://accounts.google.com",
		"sub":            "sub-auto-123",
		"aud":            "client-123",
		"email":          "autoprovision@example.com",
		"email_verified": true,
		"exp":            time.Now().UTC().Add(time.Hour).Unix(),
		"iat":            time.Now().UTC().Unix(),
	})
	body, _ := json.Marshal(map[string]any{"id_token": token})
	rr := h.request(http.MethodPost, "/auth/google", body, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for auto provisioned google login, got %d body=%s", rr.Code, rr.Body.String())
	}
	user, ok := h.ident.FindUserByUsername("autoprovision@example.com")
	if !ok {
		t.Fatal("expected auto provisioned user")
	}
	if user.AuthenticationSubject != "google:sub-auto-123" || user.DefaultLocationID != "loc_hq" {
		t.Fatalf("unexpected auto provisioned user: %+v", user)
	}
}

func TestGoogleOAuthBrowserFlow(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	pubKey, err := jwk.PublicKeyOf(key)
	if err != nil {
		t.Fatalf("public jwk failed: %v", err)
	}
	if err := pubKey.Set(jwk.KeyIDKey, "kid-1"); err != nil {
		t.Fatalf("set key id failed: %v", err)
	}
	set := jwk.NewSet()
	if err := set.AddKey(pubKey); err != nil {
		t.Fatalf("add key failed: %v", err)
	}
	var oauthServer *httptest.Server
	oauthServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/jwks":
			_ = json.NewEncoder(w).Encode(set)
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form failed: %v", err)
			}
			if got := r.Form.Get("code"); got != "good-code" {
				t.Fatalf("expected auth code, got %q", got)
			}
			if got := r.Form.Get("client_id"); got != "client-123" {
				t.Fatalf("expected client id, got %q", got)
			}
			if got := r.Form.Get("client_secret"); got != "secret-123" {
				t.Fatalf("expected client secret, got %q", got)
			}
			if got := r.Form.Get("redirect_uri"); got != "http://app.example.com/auth/google/callback" {
				t.Fatalf("expected redirect uri, got %q", got)
			}
			idToken := signGoogleTestToken(t, key, "kid-1", map[string]any{
				"iss":            "https://accounts.google.com",
				"sub":            "sub-456",
				"aud":            "client-123",
				"email":          "user@example.com",
				"email_verified": true,
				"exp":            time.Now().UTC().Add(time.Hour).Unix(),
				"iat":            time.Now().UTC().Unix(),
			})
			_ = json.NewEncoder(w).Encode(map[string]any{"id_token": idToken})
		default:
			http.NotFound(w, r)
		}
	}))
	defer oauthServer.Close()

	h := newTestHarnessWithConfig(t, []config.Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value: map[string]any{
			"password_min_length":             8,
			"session_ttl_minutes":             60,
			"session_refresh_window_minutes":  10,
			"login_rate_limit_attempts":       5,
			"login_rate_limit_window_seconds": 60,
			"trusted_origins":                 []string{},
			"google_enabled":                  true,
			"google_client_id":                "client-123",
			"google_client_secret":            "secret-123",
			"google_redirect_url":             "http://app.example.com/auth/google/callback",
			"google_auth_url":                 oauthServer.URL + "/auth",
			"google_token_url":                oauthServer.URL + "/token",
			"google_issuer":                   "https://accounts.google.com",
			"google_jwks_url":                 oauthServer.URL + "/jwks",
			"google_hosted_domain":            "",
			"google_timeout_seconds":          5,
		},
	}})
	if _, err := h.ident.CreateUser("user@example.com", "user-pass-123", "loc_hq", "role_admin", "deployment", ""); err != nil {
		t.Fatalf("create google-mapped user failed: %v", err)
	}

	startReq := httptest.NewRequest(http.MethodGet, "/auth/google/start?next="+url.QueryEscape("/ui#/orders"), nil)
	startRR := httptest.NewRecorder()
	h.router.ServeHTTP(startRR, startReq)
	if startRR.Code != http.StatusFound {
		t.Fatalf("expected redirect for google oauth start, got %d", startRR.Code)
	}
	location := startRR.Result().Header.Get("Location")
	if !strings.HasPrefix(location, oauthServer.URL+"/auth?") {
		t.Fatalf("expected google auth redirect, got %s", location)
	}
	stateCookie := findCookieByName(startRR.Result().Cookies(), googleOAuthStateCookieName)
	if stateCookie == nil || stateCookie.Value == "" {
		t.Fatal("expected google oauth state cookie")
	}
	nextCookie := findCookieByName(startRR.Result().Cookies(), googleOAuthNextCookieName)
	if nextCookie == nil || nextCookie.Value != "/ui#/orders" {
		t.Fatalf("expected next cookie, got %+v", nextCookie)
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=good-code&state="+url.QueryEscape(stateCookie.Value), nil)
	callbackReq.AddCookie(stateCookie)
	callbackReq.AddCookie(nextCookie)
	callbackReq.RemoteAddr = "192.0.2.10:1234"
	callbackRR := httptest.NewRecorder()
	h.router.ServeHTTP(callbackRR, callbackReq)
	if callbackRR.Code != http.StatusFound {
		t.Fatalf("expected redirect for google oauth callback, got %d body=%s", callbackRR.Code, callbackRR.Body.String())
	}
	if got := callbackRR.Result().Header.Get("Location"); got != "/ui#/orders" {
		t.Fatalf("expected redirect to next path, got %s", got)
	}
	sessionCookie := findCookieByName(callbackRR.Result().Cookies(), sessionCookieName)
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("expected session cookie from callback")
	}
	linked, ok := h.ident.FindUserByUsername("user@example.com")
	if !ok || linked.AuthenticationSubject != "google:sub-456" {
		t.Fatalf("expected google subject linkage, got %+v", linked)
	}
}

func TestGoogleOAuthCallbackRejectsInvalidState(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value: map[string]any{
			"password_min_length":             8,
			"session_ttl_minutes":             60,
			"session_refresh_window_minutes":  10,
			"login_rate_limit_attempts":       5,
			"login_rate_limit_window_seconds": 60,
			"trusted_origins":                 []string{},
			"google_enabled":                  true,
			"google_client_id":                "client-123",
			"google_client_secret":            "secret-123",
			"google_redirect_url":             "http://app.example.com/auth/google/callback",
			"google_auth_url":                 "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                "https://oauth2.googleapis.com/token",
			"google_issuer":                   "https://accounts.google.com",
			"google_jwks_url":                 "https://www.googleapis.com/oauth2/v3/certs",
			"google_hosted_domain":            "",
			"google_timeout_seconds":          5,
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=good-code&state=wrong-state", nil)
	req.AddCookie(&http.Cookie{Name: googleOAuthStateCookieName, Value: "expected-state"})
	req.AddCookie(&http.Cookie{Name: googleOAuthNextCookieName, Value: "/ui#/reports"})
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected redirect on invalid state, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Result().Header.Get("Location"); got != "/ui?auth_error=google_login_failed" {
		t.Fatalf("expected auth error redirect, got %s", got)
	}
}

func TestGoogleOAuthCallbackRedirectsOnTokenExchangeFailure(t *testing.T) {
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			http.Error(w, "exchange failed", http.StatusBadRequest)
		case "/jwks":
			_, _ = w.Write([]byte(`{"keys":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer oauthServer.Close()

	h := newTestHarnessWithConfig(t, []config.Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value: map[string]any{
			"password_min_length":             8,
			"session_ttl_minutes":             60,
			"session_refresh_window_minutes":  10,
			"login_rate_limit_attempts":       5,
			"login_rate_limit_window_seconds": 60,
			"trusted_origins":                 []string{},
			"google_enabled":                  true,
			"google_client_id":                "client-123",
			"google_client_secret":            "secret-123",
			"google_redirect_url":             "http://app.example.com/auth/google/callback",
			"google_auth_url":                 oauthServer.URL + "/auth",
			"google_token_url":                oauthServer.URL + "/token",
			"google_issuer":                   "https://accounts.google.com",
			"google_jwks_url":                 oauthServer.URL + "/jwks",
			"google_hosted_domain":            "",
			"google_timeout_seconds":          5,
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=bad-code&state=expected-state", nil)
	req.AddCookie(&http.Cookie{Name: googleOAuthStateCookieName, Value: "expected-state"})
	req.AddCookie(&http.Cookie{Name: googleOAuthNextCookieName, Value: "/ui#/reports"})
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected redirect on token exchange failure, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Result().Header.Get("Location"); got != "/ui?auth_error=google_login_failed" {
		t.Fatalf("expected auth error redirect, got %s", got)
	}
}

func TestAuthOptionsReflectGoogleConfiguration(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value: map[string]any{
			"password_min_length":             8,
			"session_ttl_minutes":             60,
			"session_refresh_window_minutes":  10,
			"login_rate_limit_attempts":       5,
			"login_rate_limit_window_seconds": 60,
			"trusted_origins":                 []string{},
			"password_enabled":                false,
			"login_title":                     "Welcome to Orbyte",
			"login_subtitle":                  "Use your company account.",
			"google_button_label":             "Sign in with Google Workspace",
			"google_enabled":                  true,
			"google_client_id":                "client-123",
			"google_client_secret":            "secret-123",
			"google_redirect_url":             "http://app.example.com/auth/google/callback",
			"google_auth_url":                 "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                "https://oauth2.googleapis.com/token",
			"google_issuer":                   "https://accounts.google.com",
			"google_jwks_url":                 "https://www.googleapis.com/oauth2/v3/certs",
			"google_hosted_domain":            "",
			"google_timeout_seconds":          5,
		},
	}})

	rr := h.request(http.MethodGet, "/auth/options", nil, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected auth options to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if payload["google_enabled"] != true {
		t.Fatalf("expected google_enabled=true, got %+v", payload)
	}
	if payload["password_enabled"] != false {
		t.Fatalf("expected password_enabled=false, got %+v", payload)
	}
	if payload["login_title"] != "Welcome to Orbyte" || payload["login_subtitle"] != "Use your company account." || payload["google_button_label"] != "Sign in with Google Workspace" {
		t.Fatalf("expected branded auth options, got %+v", payload)
	}
}

func TestUIShellAccessibleWithoutAuthentication(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/ui", nil, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected unauthenticated ui shell to load, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Platform Access") && !strings.Contains(rr.Body.String(), "Continue with Google") {
		t.Fatalf("expected login-capable ui shell, got %s", rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/bootstrap", nil, false)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated ui bootstrap to be rejected, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPasswordLoginDisabledByConfiguration(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value: map[string]any{
			"password_min_length":             8,
			"session_ttl_minutes":             60,
			"session_refresh_window_minutes":  10,
			"login_rate_limit_attempts":       5,
			"login_rate_limit_window_seconds": 60,
			"trusted_origins":                 []string{},
			"password_enabled":                false,
			"login_title":                     "SSO Only",
			"login_subtitle":                  "Use your identity provider.",
			"google_button_label":             "Continue with Google",
			"google_enabled":                  true,
			"google_client_id":                "client-123",
			"google_client_secret":            "secret-123",
			"google_redirect_url":             "http://app.example.com/auth/google/callback",
			"google_auth_url":                 "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                "https://oauth2.googleapis.com/token",
			"google_issuer":                   "https://accounts.google.com",
			"google_jwks_url":                 "https://www.googleapis.com/oauth2/v3/certs",
			"google_hosted_domain":            "",
			"google_timeout_seconds":          5,
		},
	}})

	body, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr := h.request(http.MethodPost, "/auth/login", body, false)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected password login to be forbidden, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "password authentication is disabled") {
		t.Fatalf("expected disabled password message, got %s", rr.Body.String())
	}
}

func TestRuntimeAuthSettingsUpdateTakesEffectImmediately(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value: map[string]any{
			"password_enabled":                true,
			"password_min_length":             8,
			"session_ttl_minutes":             60,
			"session_refresh_window_minutes":  10,
			"login_rate_limit_attempts":       5,
			"login_rate_limit_window_seconds": 60,
			"trusted_origins":                 []string{},
			"google_enabled":                  true,
			"google_client_id":                "client-123",
			"google_client_secret":            "secret-123",
			"google_redirect_url":             "http://app.example.com/auth/google/callback",
			"google_auth_url":                 "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                "https://oauth2.googleapis.com/token",
			"google_issuer":                   "https://accounts.google.com",
			"google_jwks_url":                 "https://www.googleapis.com/oauth2/v3/certs",
			"google_hosted_domain":            "",
			"google_timeout_seconds":          5,
		},
	}})

	updateBody, _ := json.Marshal(map[string]any{
		"scope": "deployment",
		"value": map[string]any{
			"password_enabled":                false,
			"password_min_length":             8,
			"session_ttl_minutes":             60,
			"session_refresh_window_minutes":  10,
			"login_rate_limit_attempts":       5,
			"login_rate_limit_window_seconds": 60,
			"trusted_origins":                 []string{},
			"google_enabled":                  true,
			"google_client_id":                "client-123",
			"google_client_secret":            "secret-123",
			"google_redirect_url":             "http://app.example.com/auth/google/callback",
			"google_auth_url":                 "https://accounts.example.test/o/oauth2/v2/auth",
			"google_token_url":                "https://oauth2.googleapis.com/token",
			"google_issuer":                   "https://accounts.google.com",
			"google_jwks_url":                 "https://www.googleapis.com/oauth2/v3/certs",
			"google_hosted_domain":            "",
			"google_timeout_seconds":          5,
		},
	})
	rr := h.request(http.MethodPut, "/admin/api/auth/settings", updateBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected auth settings update to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	loginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr = h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected password login to reflect updated policy, got %d body=%s", rr.Code, rr.Body.String())
	}

	startReq := httptest.NewRequest(http.MethodGet, "/auth/google/start?next="+url.QueryEscape("/ui#/reports"), nil)
	startRR := httptest.NewRecorder()
	h.router.ServeHTTP(startRR, startReq)
	if startRR.Code != http.StatusFound {
		t.Fatalf("expected google oauth start redirect, got %d body=%s", startRR.Code, startRR.Body.String())
	}
	if got := startRR.Result().Header.Get("Location"); !strings.HasPrefix(got, "https://accounts.example.test/o/oauth2/v2/auth?") {
		t.Fatalf("expected updated google auth url, got %s", got)
	}
	nextCookie := findCookieByName(startRR.Result().Cookies(), googleOAuthNextCookieName)
	if nextCookie == nil || nextCookie.Value != "/ui#/reports" {
		t.Fatalf("expected deep-link ui route to be preserved, got %+v", nextCookie)
	}
}

func TestLastSeenUpdatedOnAuthenticatedRequest(t *testing.T) {
	h := newTestHarness(t)
	before, ok := h.ident.FindSession("sess_admin")
	if !ok {
		t.Fatal("expected bootstrap session")
	}
	rr := h.request(http.MethodGet, "/platform/context", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected authenticated request, got %d", rr.Code)
	}
	after, ok := h.ident.FindSession("sess_admin")
	if !ok {
		t.Fatal("expected bootstrap session after request")
	}
	if !after.LastSeenAt.After(before.LastSeenAt) {
		t.Fatalf("expected last_seen_at to move forward: before=%s after=%s", before.LastSeenAt, after.LastSeenAt)
	}
}

func TestSessionRevokeRoute(t *testing.T) {
	h := newTestHarness(t)
	loginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr := h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d", rr.Code)
	}
	var loginResp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &loginResp)
	sessionID := loginResp["session"].(map[string]any)["id"].(string)
	loginCookie := rr.Result().Cookies()[0]

	rr = h.request(http.MethodPost, "/sessions/"+sessionID+"/actions/revoke", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected session revoke to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/platform/context", nil)
	req.AddCookie(loginCookie)
	revokedRR := httptest.NewRecorder()
	h.router.ServeHTTP(revokedRR, req)
	if revokedRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked session to fail, got %d", revokedRR.Code)
	}
}

func TestPasswordChangeRoute(t *testing.T) {
	h := newTestHarness(t)
	loginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr := h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d", rr.Code)
	}
	loginCookie := rr.Result().Cookies()[0]

	changeBody, _ := json.Marshal(map[string]any{"current_password": "admin123!", "new_password": "better-admin-123"})
	req := httptest.NewRequest(http.MethodPost, "/auth/password/change", bytes.NewReader(changeBody))
	req.AddCookie(loginCookie)
	csrfCookie := findCookieByName(rr.Result().Cookies(), csrfCookieName)
	if csrfCookie == nil {
		t.Fatal("expected csrf cookie on login")
	}
	req.AddCookie(csrfCookie)
	req.Header.Set("X-CSRF-Token", csrfCookie.Value)
	changeRR := httptest.NewRecorder()
	h.router.ServeHTTP(changeRR, req)
	if changeRR.Code != http.StatusOK {
		t.Fatalf("expected password change to succeed, got %d body=%s", changeRR.Code, changeRR.Body.String())
	}

	reuseReq := httptest.NewRequest(http.MethodGet, "/platform/context", nil)
	reuseReq.AddCookie(loginCookie)
	reuseRR := httptest.NewRecorder()
	h.router.ServeHTTP(reuseRR, reuseReq)
	if reuseRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected old session to be revoked after password change, got %d", reuseRR.Code)
	}

	oldLoginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr = h.request(http.MethodPost, "/auth/login", oldLoginBody, false)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password to fail, got %d", rr.Code)
	}

	newLoginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "better-admin-123", "location_id": "loc_hq"})
	rr = h.request(http.MethodPost, "/auth/login", newLoginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected new password to work, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminProvisionUserAndResetPassword(t *testing.T) {
	h := newTestHarness(t)

	createBody, _ := json.Marshal(map[string]any{
		"username":            "clerk",
		"password":            "clerk-pass-123",
		"default_location_id": "loc_hq",
		"role_id":             "role_admin",
		"scope_type":          "deployment",
	})
	rr := h.request(http.MethodPost, "/users", createBody, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected user create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	userID := created["id"].(string)

	loginBody, _ := json.Marshal(map[string]any{"username": "clerk", "password": "clerk-pass-123", "location_id": "loc_hq"})
	rr = h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected created user to log in, got %d body=%s", rr.Code, rr.Body.String())
	}
	clerkCookie := rr.Result().Cookies()[0]

	resetBody, _ := json.Marshal(map[string]any{"new_password": "clerk-pass-456"})
	rr = h.request(http.MethodPost, "/users/"+userID+"/actions/reset-password", resetBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected admin reset password to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	clerkReq := httptest.NewRequest(http.MethodGet, "/platform/context", nil)
	clerkReq.AddCookie(clerkCookie)
	clerkRR := httptest.NewRecorder()
	h.router.ServeHTTP(clerkRR, clerkReq)
	if clerkRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected reset password to revoke prior sessions, got %d", clerkRR.Code)
	}

	oldLoginBody, _ := json.Marshal(map[string]any{"username": "clerk", "password": "clerk-pass-123", "location_id": "loc_hq"})
	rr = h.request(http.MethodPost, "/auth/login", oldLoginBody, false)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password to fail after reset, got %d", rr.Code)
	}
	newLoginBody, _ := json.Marshal(map[string]any{"username": "clerk", "password": "clerk-pass-456", "location_id": "loc_hq"})
	rr = h.request(http.MethodPost, "/auth/login", newLoginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected reset password to work, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDisableUserRevokesSessionsAndBlocksLogin(t *testing.T) {
	h := newTestHarness(t)

	createBody, _ := json.Marshal(map[string]any{
		"username":            "clerk",
		"password":            "clerk-pass-123",
		"default_location_id": "loc_hq",
		"role_id":             "role_admin",
		"scope_type":          "deployment",
	})
	rr := h.request(http.MethodPost, "/users", createBody, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected user create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	userID := created["id"].(string)

	loginBody, _ := json.Marshal(map[string]any{"username": "clerk", "password": "clerk-pass-123", "location_id": "loc_hq"})
	rr = h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected clerk login to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	clerkCookie := rr.Result().Cookies()[0]

	disableBody, _ := json.Marshal(map[string]any{"status": "disabled"})
	rr = h.request(http.MethodPost, "/users/"+userID+"/actions/set-status", disableBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected disable user to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/platform/context", nil)
	req.AddCookie(clerkCookie)
	blockedRR := httptest.NewRecorder()
	h.router.ServeHTTP(blockedRR, req)
	if blockedRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected disabled user's session to be revoked, got %d", blockedRR.Code)
	}

	rr = h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected disabled user login to be forbidden, got %d body=%s", rr.Code, rr.Body.String())
	}

	enableBody, _ := json.Marshal(map[string]any{"status": "active"})
	rr = h.request(http.MethodPost, "/users/"+userID+"/actions/set-status", enableBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected enable user to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected enabled user login to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminUserAndSessionInspectionRoutes(t *testing.T) {
	h := newTestHarness(t)

	createBody, _ := json.Marshal(map[string]any{
		"username":            "clerk",
		"password":            "clerk-pass-123",
		"default_location_id": "loc_hq",
		"role_id":             "role_admin",
		"scope_type":          "deployment",
	})
	rr := h.request(http.MethodPost, "/users", createBody, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected user create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	userID := created["id"].(string)

	loginBody, _ := json.Marshal(map[string]any{"username": "clerk", "password": "clerk-pass-123", "location_id": "loc_hq"})
	rr = h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected clerk login to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var loginResp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &loginResp)
	sessionID := loginResp["session"].(map[string]any)["id"].(string)

	for _, path := range []string{"/users", "/users/" + userID, "/sessions", "/sessions/" + sessionID, "/sessions?user_id=" + userID} {
		rr = h.request(http.MethodGet, path, nil, true)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d body=%s", path, rr.Code, rr.Body.String())
		}
	}

	rr = h.request(http.MethodGet, "/users/"+userID, nil, false)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated user inspection to fail, got %d", rr.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{
		{
			Key:      "platform.http",
			Category: "platform",
			Scope:    "deployment",
			Value:    map[string]any{"address": ":8080"},
		},
		{
			Key:      "identity.auth",
			Category: "identity",
			Scope:    "deployment",
			Value: map[string]any{
				"password_min_length":             8,
				"session_ttl_minutes":             480,
				"session_refresh_window_minutes":  60,
				"login_rate_limit_attempts":       2,
				"login_rate_limit_window_seconds": 300,
			},
		},
	})
	body, _ := json.Marshal(map[string]any{"username": "admin", "password": "wrong", "location_id": "loc_hq"})
	for i := 0; i < 2; i++ {
		rr := h.request(http.MethodPost, "/auth/login", body, false)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected invalid login to be unauthorized, got %d", rr.Code)
		}
	}
	rr := h.request(http.MethodPost, "/auth/login", body, false)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected rate-limited login to be forbidden, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestSessionRefreshPolicy(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{
		{
			Key:      "platform.http",
			Category: "platform",
			Scope:    "deployment",
			Value:    map[string]any{"address": ":8080"},
		},
		{
			Key:      "identity.auth",
			Category: "identity",
			Scope:    "deployment",
			Value: map[string]any{
				"password_min_length":             8,
				"session_ttl_minutes":             1,
				"session_refresh_window_minutes":  1,
				"login_rate_limit_attempts":       5,
				"login_rate_limit_window_seconds": 300,
			},
		},
	})

	loginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr := h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var loginResp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &loginResp)
	sessionID := loginResp["session"].(map[string]any)["id"].(string)
	session, ok := h.ident.FindSession(sessionID)
	if !ok {
		t.Fatal("expected session after login")
	}
	originalExpiry := session.ExpiresAt

	refreshReq := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	refreshReq.RemoteAddr = "192.0.2.10:1234"
	sessionCookie := findCookieByName(rr.Result().Cookies(), sessionCookieName)
	if sessionCookie == nil {
		t.Fatal("expected session cookie on login")
	}
	refreshReq.AddCookie(sessionCookie)
	refreshCSRFCookie := findCookieByName(rr.Result().Cookies(), csrfCookieName)
	if refreshCSRFCookie == nil {
		t.Fatal("expected csrf cookie on login")
	}
	refreshReq.AddCookie(refreshCSRFCookie)
	refreshReq.Header.Set("X-CSRF-Token", refreshCSRFCookie.Value)
	refreshRR := httptest.NewRecorder()
	h.router.ServeHTTP(refreshRR, refreshReq)
	if refreshRR.Code != http.StatusOK {
		t.Fatalf("expected session refresh to succeed, got %d body=%s", refreshRR.Code, refreshRR.Body.String())
	}

	if _, ok := h.ident.FindSession(sessionID); !ok {
		t.Fatal("expected refreshed session")
	}
	var refreshResp map[string]any
	_ = json.Unmarshal(refreshRR.Body.Bytes(), &refreshResp)
	refreshedID := refreshResp["session"].(map[string]any)["id"].(string)
	if refreshedID == sessionID {
		t.Fatal("expected refresh to rotate session id")
	}
	oldSession, ok := h.ident.FindSession(sessionID)
	if !ok || oldSession.Status != "revoked" {
		t.Fatalf("expected original session to be revoked after refresh, got %+v", oldSession)
	}
	newSession, ok := h.ident.FindSession(refreshedID)
	if !ok {
		t.Fatal("expected rotated session to exist")
	}
	if newSession.ExpiresAt.Before(originalExpiry) {
		t.Fatalf("expected rotated session expiry to be preserved or extended, before=%s after=%s", originalExpiry, newSession.ExpiresAt)
	}
}

func TestLoginRateLimitUsesClientIPAcrossDifferentPorts(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{
		{
			Key:      "platform.http",
			Category: "platform",
			Scope:    "deployment",
			Value:    map[string]any{"address": ":8080"},
		},
		{
			Key:      "identity.auth",
			Category: "identity",
			Scope:    "deployment",
			Value: map[string]any{
				"password_min_length":             8,
				"session_ttl_minutes":             480,
				"session_refresh_window_minutes":  60,
				"login_rate_limit_attempts":       2,
				"login_rate_limit_window_seconds": 300,
			},
		},
	})
	body, _ := json.Marshal(map[string]any{"username": "admin", "password": "wrong", "location_id": "loc_hq"})
	for _, remoteAddr := range []string{"192.0.2.10:1111", "192.0.2.10:2222"} {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.RemoteAddr = remoteAddr
		rr := httptest.NewRecorder()
		h.router.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected invalid login to be unauthorized, got %d", rr.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.0.2.10:3333"
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected rate-limited login across port changes, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCookieMutationsRequireCSRFFromBrowserSessions(t *testing.T) {
	h := newTestHarness(t)
	loginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr := h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d", rr.Code)
	}
	sessionCookie := findCookieByName(rr.Result().Cookies(), sessionCookieName)
	if sessionCookie == nil {
		t.Fatal("expected session cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(sessionCookie)
	blockedRR := httptest.NewRecorder()
	h.router.ServeHTTP(blockedRR, req)
	if blockedRR.Code != http.StatusForbidden {
		t.Fatalf("expected missing csrf token to be forbidden, got %d body=%s", blockedRR.Code, blockedRR.Body.String())
	}
}

func TestSessionRefreshRejectsOutsideRefreshWindow(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{{
		Key:      "identity.auth",
		Category: "identity",
		Scope:    "deployment",
		Value: map[string]any{
			"password_min_length":             8,
			"session_ttl_minutes":             60,
			"session_refresh_window_minutes":  1,
			"login_rate_limit_attempts":       5,
			"login_rate_limit_window_seconds": 300,
			"trusted_origins":                 []any{},
		},
	}})

	loginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr := h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	refreshReq := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	refreshReq.RemoteAddr = "192.0.2.10:1234"
	sessionCookie := findCookieByName(rr.Result().Cookies(), sessionCookieName)
	csrfCookie := findCookieByName(rr.Result().Cookies(), csrfCookieName)
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatal("expected auth cookies on login")
	}
	refreshReq.AddCookie(sessionCookie)
	refreshReq.AddCookie(csrfCookie)
	refreshReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	refreshRR := httptest.NewRecorder()
	h.router.ServeHTTP(refreshRR, refreshReq)
	if refreshRR.Code != http.StatusConflict {
		t.Fatalf("expected refresh outside window to be rejected, got %d body=%s", refreshRR.Code, refreshRR.Body.String())
	}
}

func TestSessionRefreshRejectsRevokedSession(t *testing.T) {
	h := newTestHarness(t)

	loginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr := h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	var loginResp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &loginResp)
	sessionID := loginResp["session"].(map[string]any)["id"].(string)
	if _, err := h.ident.RevokeSession(sessionID, time.Now().UTC()); err != nil {
		t.Fatalf("revoke session failed: %v", err)
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	refreshReq.RemoteAddr = "192.0.2.10:1234"
	sessionCookie := findCookieByName(rr.Result().Cookies(), sessionCookieName)
	csrfCookie := findCookieByName(rr.Result().Cookies(), csrfCookieName)
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatal("expected auth cookies on login")
	}
	refreshReq.AddCookie(sessionCookie)
	refreshReq.AddCookie(csrfCookie)
	refreshReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	refreshRR := httptest.NewRecorder()
	h.router.ServeHTTP(refreshRR, refreshReq)
	if refreshRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked session refresh to be unauthorized, got %d body=%s", refreshRR.Code, refreshRR.Body.String())
	}
	if !strings.Contains(refreshRR.Body.String(), "session not active") {
		t.Fatalf("expected revoked session rejection reason, got %s", refreshRR.Body.String())
	}
}

func TestAdminAuthSettingsRequireCSRFFromBrowserSessions(t *testing.T) {
	h := newTestHarness(t)
	req := httptest.NewRequest(http.MethodPut, "/admin/api/auth/settings", bytes.NewReader([]byte(`{"scope":"deployment","value":{"password_enabled":true}}`)))
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(h.cookie)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected missing csrf token to be forbidden for admin auth settings, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestTrustedOriginProtectionForBrowserMutations(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{
		{
			Key:      "platform.http",
			Category: "platform",
			Scope:    "deployment",
			Value:    map[string]any{"address": ":8080"},
		},
		{
			Key:      "identity.auth",
			Category: "identity",
			Scope:    "deployment",
			Value: map[string]any{
				"password_min_length":             8,
				"session_ttl_minutes":             480,
				"session_refresh_window_minutes":  60,
				"login_rate_limit_attempts":       5,
				"login_rate_limit_window_seconds": 300,
				"trusted_origins":                 []any{"https://app.example.com"},
			},
		},
	})
	loginBody, _ := json.Marshal(map[string]any{"username": "admin", "password": "admin123!", "location_id": "loc_hq"})
	rr := h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d", rr.Code)
	}
	sessionCookie := findCookieByName(rr.Result().Cookies(), sessionCookieName)
	csrfCookie := findCookieByName(rr.Result().Cookies(), csrfCookieName)
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatal("expected auth and csrf cookies")
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(sessionCookie)
	req.AddCookie(csrfCookie)
	req.Header.Set("X-CSRF-Token", csrfCookie.Value)
	req.Header.Set("Origin", "https://evil.example.com")
	blockedRR := httptest.NewRecorder()
	h.router.ServeHTTP(blockedRR, req)
	if blockedRR.Code != http.StatusForbidden {
		t.Fatalf("expected untrusted origin to be forbidden, got %d body=%s", blockedRR.Code, blockedRR.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(sessionCookie)
	req.AddCookie(csrfCookie)
	req.Header.Set("X-CSRF-Token", csrfCookie.Value)
	req.Header.Set("Origin", "https://app.example.com")
	allowedRR := httptest.NewRecorder()
	h.router.ServeHTTP(allowedRR, req)
	if allowedRR.Code != http.StatusOK {
		t.Fatalf("expected trusted origin logout to succeed, got %d body=%s", allowedRR.Code, allowedRR.Body.String())
	}
}

func TestSessionReviewIncludesAnomalySignals(t *testing.T) {
	h := newTestHarness(t)
	createBody, _ := json.Marshal(map[string]any{
		"username":            "clerk",
		"password":            "clerk-pass-123",
		"default_location_id": "loc_hq",
		"role_id":             "role_admin",
		"scope_type":          "deployment",
	})
	rr := h.request(http.MethodPost, "/users", createBody, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected user create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	loginBody := []byte(`{"username":"clerk","password":"clerk-pass-123","location_id":"loc_hq"}`)
	rr = h.request(http.MethodPost, "/auth/login", loginBody, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected first login to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	altReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	altReq.RemoteAddr = "198.51.100.20:8888"
	altReq.Header.Set("User-Agent", "alt-client")
	altRR := httptest.NewRecorder()
	h.router.ServeHTTP(altRR, altReq)
	if altRR.Code != http.StatusOK {
		t.Fatalf("expected second login to succeed, got %d body=%s", altRR.Code, altRR.Body.String())
	}
	var loginResp map[string]any
	_ = json.Unmarshal(altRR.Body.Bytes(), &loginResp)
	sessionID := loginResp["session"].(map[string]any)["id"].(string)

	rr = h.request(http.MethodGet, "/sessions/"+sessionID, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected session inspection to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var sessionResp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &sessionResp)
	review := sessionResp["review"].(map[string]any)
	flags := review["flags"].([]any)
	if len(flags) == 0 {
		t.Fatal("expected anomaly review flags")
	}
}

func TestPasswordPolicyConfigEnforced(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{
		{
			Key:      "platform.http",
			Category: "platform",
			Scope:    "deployment",
			Value:    map[string]any{"address": ":8080"},
		},
		{
			Key:      "identity.auth",
			Category: "identity",
			Scope:    "deployment",
			Value: map[string]any{
				"password_min_length":             12,
				"session_ttl_minutes":             480,
				"session_refresh_window_minutes":  60,
				"login_rate_limit_attempts":       5,
				"login_rate_limit_window_seconds": 300,
			},
		},
	})
	createBody, _ := json.Marshal(map[string]any{
		"username":            "shortpass",
		"password":            "too-short",
		"default_location_id": "loc_hq",
		"role_id":             "role_admin",
		"scope_type":          "deployment",
	})
	rr := h.request(http.MethodPost, "/users", createBody, true)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected configured password policy violation, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHealthzAndContext(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/healthz", nil, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /healthz, got %d", rr.Code)
	}
	if rr.Header().Get("X-Correlation-ID") == "" {
		t.Fatal("expected correlation header for /healthz")
	}
	rr = h.request(http.MethodGet, "/readyz", nil, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /readyz, got %d body=%s", rr.Code, rr.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/documents", nil)
	req.AddCookie(h.cookie)
	req.AddCookie(h.csrf)
	req.Header.Set("X-CSRF-Token", h.csrf.Value)
	metricRR := httptest.NewRecorder()
	h.router.ServeHTTP(metricRR, req)

	rr = h.request(http.MethodGet, "/platform/context", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /platform/context, got %d", rr.Code)
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if _, ok := payload["sessions"]; ok {
		t.Fatal("expected redacted platform context without sessions")
	}
	if _, ok := payload["service_principals"]; ok {
		t.Fatal("expected redacted platform context without service principals")
	}
	statsRR := h.request(http.MethodGet, "/ops/stats", nil, true)
	if statsRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for /ops/stats, got %d body=%s", statsRR.Code, statsRR.Body.String())
	}
	var statsPayload map[string]any
	_ = json.Unmarshal(statsRR.Body.Bytes(), &statsPayload)
	if _, ok := statsPayload["jobs"]; !ok {
		t.Fatal("expected job stats in ops payload")
	}
	metricsRR := h.request(http.MethodGet, "/metrics", nil, true)
	if metricsRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for /metrics, got %d body=%s", metricsRR.Code, metricsRR.Body.String())
	}
	body := metricsRR.Body.String()
	if !strings.Contains(body, "http_route_family_documents_requests_total") {
		t.Fatalf("expected route-family metrics in /metrics, got %s", body)
	}
}

func TestReadyzReturnsUnavailableWhenRuntimeHealthIsDegraded(t *testing.T) {
	t.Setenv("APP_JWT_SECRET", "test-secret")
	cfg := config.NewService()
	org := organization.NewService()
	ident := identity.NewService(org)
	models := model.NewService()
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	searchSvc := search.NewService()
	loggerSvc := logging.NewServiceWithWriter(nil)
	obsSvc := observability.NewService()
	policySvc := policy.NewServiceWithConfig(cfg)
	reportingSvc := reporting.NewService(models)
	monitoringSvc := monitoring.NewService(docs, eventingSvc, flows, searchSvc, obsSvc)
	analyticsSvc := analytics.NewService(docs, flows, eventingSvc, searchSvc, auditSvc, obsSvc)
	integrationSvc := integration.NewService(obsSvc, loggerSvc)
	jobSvc := jobs.NewService()
	health := runtimehealth.NewTracker()
	health.SetBootstrapped(true)
	health.SetBackgroundStarted(true)
	health.MarkFailure("jobs", errors.New("boom"))
	health.MarkFailure("jobs", errors.New("boom"))
	health.MarkFailure("jobs", errors.New("boom"))
	router := NewRouter(cfg, org, ident, module.NewService(), models, activity.NewService(), reportingSvc, reference.NewService(), docs, flows, auditSvc, eventingSvc, searchSvc, loggerSvc, analyticsSvc, monitoringSvc, obsSvc, policySvc, integrationSvc, jobSvc, health, application.NewDocumentActions(docs, flows, policySvc, application.NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc)), application.NewMemoryModelActions(models, activity.NewService(), auditSvc, eventingSvc))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for degraded readyz, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestQueuedIntegrationActionsStillRecordAuditEvents(t *testing.T) {
	h := newTestHarness(t)

	createBody, _ := json.Marshal(map[string]any{
		"system_key":     "fake_erp",
		"operation_type": "sync_customer",
		"document_id":    "doc-1",
		"correlation_id": "corr-1",
		"payload":        map[string]any{"customer_id": "cust-1"},
	})
	rr := h.request(http.MethodPost, "/admin/api/integrations/submissions", createBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected submission create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	record := created["record"].(map[string]any)
	id := record["id"].(string)

	rr = h.request(http.MethodPost, "/admin/api/integrations/submissions/"+id+"/actions/process", nil, true)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected queued process action, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.request(http.MethodPost, "/admin/api/integrations/submissions/"+id+"/actions/retry", nil, true)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected queued retry action, got %d body=%s", rr.Code, rr.Body.String())
	}

	var processAudit bool
	var retryAudit bool
	for _, event := range h.audit.List() {
		switch event.Action {
		case "integration.submission.process":
			if event.TargetID == id {
				processAudit = true
			}
		case "integration.submission.retry":
			if event.TargetID == id {
				retryAudit = true
			}
		}
	}
	if !processAudit {
		t.Fatal("expected audit event for queued process action")
	}
	if !retryAudit {
		t.Fatal("expected audit event for queued retry action")
	}
}

func TestAdminModuleAndConfigRoutes(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/admin/api/bootstrap", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin bootstrap, got %d body=%s", rr.Code, rr.Body.String())
	}
	var bootstrapPayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &bootstrapPayload)
	if len(bootstrapPayload["roles"].([]any)) == 0 {
		t.Fatal("expected roles in admin bootstrap payload")
	}

	rr = h.request(http.MethodGet, "/admin/api/modules", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin modules, got %d body=%s", rr.Code, rr.Body.String())
	}
	var modulesPayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &modulesPayload)
	if len(modulesPayload["items"].([]any)) == 0 {
		t.Fatal("expected modules in admin payload")
	}

	rr = h.request(http.MethodGet, "/admin/api/config/definitions", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for config definitions, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.request(http.MethodGet, "/admin/api/auth/settings", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth settings, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.request(http.MethodGet, "/admin/api/config/effective?organization_id=org_default&location_id=loc_hq", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for effective config, got %d body=%s", rr.Code, rr.Body.String())
	}

	updateBody, _ := json.Marshal(map[string]any{
		"scope":    "location",
		"scope_id": "loc_hq",
		"value": map[string]any{
			"login_rate_limit_attempts": 2,
		},
	})
	rr = h.request(http.MethodPut, "/admin/api/config/entries/identity.auth/value", updateBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for config update, got %d body=%s", rr.Code, rr.Body.String())
	}
	effective, ok := h.cfg.Resolve("identity.auth", "org_default", "loc_hq")
	if !ok || configIntValue(effective.Value["login_rate_limit_attempts"]) != 2 {
		t.Fatalf("expected updated effective config, got %+v", effective)
	}

	rr = h.request(http.MethodGet, "/admin/api/security/role-templates", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected role templates endpoint, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.request(http.MethodGet, "/admin/api/security/policy-hooks", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected policy hooks endpoint, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.request(http.MethodGet, "/admin/api/observability/contracts", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected observability contracts endpoint, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminAuthSettingsPreserveRedactedSecret(t *testing.T) {
	h := newTestHarnessWithConfig(t, []config.Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value: map[string]any{
			"password_enabled":                          false,
			"login_title":                               "SSO Access",
			"login_subtitle":                            "Use your company account.",
			"google_button_label":                       "Continue with Google",
			"google_enabled":                            true,
			"google_auto_provision_enabled":             true,
			"google_auto_provision_allowed_domains":     []string{"example.com"},
			"google_auto_provision_role_id":             "role_admin",
			"google_auto_provision_scope_type":          "deployment",
			"google_auto_provision_default_location_id": "loc_hq",
			"google_client_id":                          "client-123",
			"google_client_secret":                      "secret-123",
			"google_redirect_url":                       "https://app.example.com/auth/google/callback",
			"google_auth_url":                           "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                          "https://oauth2.googleapis.com/token",
			"google_jwks_url":                           "https://www.googleapis.com/oauth2/v3/certs",
			"google_issuer":                             "https://accounts.google.com",
			"google_timeout_seconds":                    5,
		},
	}})

	rr := h.request(http.MethodGet, "/admin/api/auth/settings", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth settings, got %d body=%s", rr.Code, rr.Body.String())
	}
	var authPayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &authPayload)
	entry := authPayload["entry"].(map[string]any)
	value := entry["value"].(map[string]any)
	if value["google_client_secret"] != "[redacted]" {
		t.Fatalf("expected redacted secret in auth settings response, got %+v", value)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"scope": "deployment",
		"value": map[string]any{
			"password_enabled":                          false,
			"login_title":                               "Updated SSO Access",
			"login_subtitle":                            "Use Google Workspace.",
			"google_button_label":                       "Sign in with Google",
			"google_enabled":                            true,
			"google_auto_provision_enabled":             true,
			"google_auto_provision_allowed_domains":     []string{"example.com"},
			"google_auto_provision_role_id":             "role_admin",
			"google_auto_provision_scope_type":          "deployment",
			"google_auto_provision_scope_id":            "",
			"google_auto_provision_default_location_id": "loc_hq",
			"google_client_id":                          "client-123",
			"google_client_secret":                      "[redacted]",
			"google_redirect_url":                       "https://app.example.com/auth/google/callback",
			"google_auth_url":                           "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                          "https://oauth2.googleapis.com/token",
			"google_jwks_url":                           "https://www.googleapis.com/oauth2/v3/certs",
			"google_issuer":                             "https://accounts.google.com",
			"google_hosted_domain":                      "",
			"google_timeout_seconds":                    10,
		},
	})
	rr = h.request(http.MethodPut, "/admin/api/auth/settings", updateBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth settings update, got %d body=%s", rr.Code, rr.Body.String())
	}
	effective, ok := h.cfg.Resolve("identity.auth", "", "")
	if !ok {
		t.Fatal("expected effective auth config")
	}
	if effective.Value["google_client_secret"] != "secret-123" {
		t.Fatalf("expected secret to be preserved, got %+v", effective.Value["google_client_secret"])
	}
	if effective.Value["login_title"] != "Updated SSO Access" {
		t.Fatalf("expected updated title, got %+v", effective.Value["login_title"])
	}
}

func TestAdminAuthSettingsRejectInvalidConfigValue(t *testing.T) {
	h := newTestHarness(t)

	updateBody := []byte(`{"scope":"deployment","value":{"google_timeout_seconds":"bad"}}`)
	rr := h.request(http.MethodPut, "/admin/api/auth/settings", updateBody, true)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid auth settings update to fail, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "google_timeout_seconds must be an integer") {
		t.Fatalf("expected validation error, got %s", rr.Body.String())
	}
}

func TestAdminRoutesHandleFailureCases(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/admin/api/modules/does-not-exist", nil, true)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected missing module lookup to return 404, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodPost, "/admin/api/modules/analytics/actions/unknown", nil, true)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected unknown module action to return 404, got %d body=%s", rr.Code, rr.Body.String())
	}

	req := httptest.NewRequest(http.MethodPut, "/admin/api/auth/settings", bytes.NewReader([]byte(`{"scope":"deployment",`)))
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(h.cookie)
	req.AddCookie(h.csrf)
	req.Header.Set("X-CSRF-Token", h.csrf.Value)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid auth settings JSON to return 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	disableReq := httptest.NewRequest(http.MethodPost, "/admin/api/modules/identity/actions/disable", nil)
	disableReq.RemoteAddr = "192.0.2.10:1234"
	disableReq.AddCookie(h.cookie)
	disableReq.AddCookie(h.csrf)
	disableReq.Header.Set("X-CSRF-Token", h.csrf.Value)
	disableRR := httptest.NewRecorder()
	h.router.ServeHTTP(disableRR, disableReq)
	if disableRR.Code != http.StatusConflict {
		t.Fatalf("expected identity module disable conflict, got %d body=%s", disableRR.Code, disableRR.Body.String())
	}
	if !strings.Contains(disableRR.Body.String(), "enabled dependents") {
		t.Fatalf("expected dependent-module conflict, got %s", disableRR.Body.String())
	}
}

func TestAdminPolicyHookFailureCases(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/admin/api/security/policy-hooks/does-not-exist", nil, true)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected missing policy hook to return 404, got %d body=%s", rr.Code, rr.Body.String())
	}

	req := httptest.NewRequest(http.MethodPut, "/admin/api/security/policy-hooks/documents.workflow.transition", bytes.NewReader([]byte(`{"scope":"deployment",`)))
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(h.cookie)
	req.AddCookie(h.csrf)
	req.Header.Set("X-CSRF-Token", h.csrf.Value)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid policy hook update JSON to return 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/api/security/policy-hooks/documents.workflow.transition/rego", bytes.NewReader([]byte(`{"scope":"deployment",`)))
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(h.cookie)
	req.AddCookie(h.csrf)
	req.Header.Set("X-CSRF-Token", h.csrf.Value)
	rr = httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid rego update JSON to return 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	regoBody := []byte(`{"scope":"deployment","source":"package bad\n default decision = "}`)
	rr = h.request(http.MethodPut, "/admin/api/security/policy-hooks/documents.workflow.transition/rego", regoBody, true)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected non-rego-backed policy hook update to fail, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "not Rego-backed") {
		t.Fatalf("expected non-rego-backed error, got %s", rr.Body.String())
	}
}

func TestAdminIntegrationFailureCases(t *testing.T) {
	h := newTestHarness(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/integrations/submissions", bytes.NewReader([]byte(`{"system_key":"fake_erp",`)))
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(h.cookie)
	req.AddCookie(h.csrf)
	req.Header.Set("X-CSRF-Token", h.csrf.Value)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid integration submission JSON to return 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	createBody, _ := json.Marshal(map[string]any{
		"system_key":     "fake_erp",
		"operation_type": "",
		"document_id":    "doc-1",
		"correlation_id": "corr-1",
		"payload":        map[string]any{"customer_id": "cust-1"},
	})
	rr = h.request(http.MethodPost, "/admin/api/integrations/submissions", createBody, true)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected missing operation type to return 400, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "operation_type is required") {
		t.Fatalf("expected missing operation type error, got %s", rr.Body.String())
	}

	rr = h.request(http.MethodPost, "/admin/api/integrations/submissions/sub-missing/actions/unknown", nil, true)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected unknown integration action to return 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUIBootstrapAndRouteResolution(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/ui/bootstrap", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected ui bootstrap to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	menus := payload["menus"].([]any)
	if len(menus) == 0 {
		t.Fatal("expected visible menus in ui bootstrap")
	}
	if payload["default_path"].(string) == "" {
		t.Fatal("expected default_path in ui bootstrap")
	}

	rr = h.request(http.MethodGet, "/ui/routes/resolve?path=/documents", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected route resolution to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var route map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &route)
	if route["render_mode"].(string) != string(module.RenderModeGeneric) {
		t.Fatalf("expected generic render mode, got %+v", route)
	}

	rr = h.request(http.MethodGet, "/ui/views/documents.requests.list", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected view lookup to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/assets/modules/analytics-cockpit.js", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected bundle asset to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "ClinicModuleBundles") {
		t.Fatalf("expected bundle script payload, got %s", rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/routes/resolve?path=/monitoring", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected monitoring dashboard route resolution, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/actions/render?action=submit&document_id=missing", nil, true)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected render action endpoint to validate document lookup, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUIDisabledModuleHidden(t *testing.T) {
	h := newTestHarness(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/modules/analytics/actions/disable", nil)
	req.AddCookie(h.cookie)
	req.AddCookie(h.csrf)
	req.Header.Set("X-CSRF-Token", h.csrf.Value)
	disableRR := httptest.NewRecorder()
	h.router.ServeHTTP(disableRR, req)
	if disableRR.Code != http.StatusOK {
		t.Fatalf("expected analytics disable to succeed, got %d body=%s", disableRR.Code, disableRR.Body.String())
	}

	rr := h.request(http.MethodGet, "/ui/bootstrap", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected ui bootstrap to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	items := payload["menus"].([]any)
	for _, item := range items {
		menu := item.(map[string]any)
		if menu["key"] == "analytics.cockpit" {
			t.Fatal("expected disabled module menu to be hidden")
		}
	}

	rr = h.request(http.MethodGet, "/ui/routes/resolve?path=/analytics/cockpit", nil, true)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected disabled module route to be hidden, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/assets/modules/analytics-cockpit.js", nil, true)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected disabled module bundle to be hidden, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUIReportingQueryAndModelDetailData(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/ui/data/reporting/query?source=models/party&dimensions=status&measures=count&group_by=status", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected ad hoc model reporting to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if payload["dataset_key"].(string) == "" {
		t.Fatalf("expected dataset key in ad hoc model reporting, got %+v", payload)
	}

	docBody, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "report source"},
	})
	rr = h.request(http.MethodPost, "/documents", docBody, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected document create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/data/reporting/query?source=documents&dimensions=header.status&measures=count&group_by=header.status", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected ad hoc document reporting to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.request(http.MethodGet, "/ui/data/reporting/query?source=document_projections&dimensions=status&measures=count&group_by=status", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected ad hoc projection reporting to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/data/models?model=party", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected model list to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	items, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("expected model items payload, got %+v", payload)
	}
	if len(items) == 0 {
		t.Fatal("expected seeded party records")
	}
	partyID := items[0].(map[string]any)["id"].(string)
	rr = h.request(http.MethodGet, "/ui/data/models/party/"+partyID, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected model detail to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if _, ok := payload["model_definitions"].(map[string]any); !ok {
		t.Fatalf("expected model definitions in model detail payload, got %+v", payload)
	}
	record := payload["record"].(map[string]any)
	values := record["values"].(map[string]any)
	if _, ok := values["email"]; ok {
		t.Fatalf("expected sensitive email to be hidden from ui detail payload, got %+v", values)
	}

	rr = h.request(http.MethodGet, "/ui/data/reporting/query?source=models/party&dimensions=email&measures=count&group_by=email", nil, true)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected sensitive reporting dimension to be blocked, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/data/reporting/query?source=documents&dimensions=body.payload.patient_ssn&measures=count&group_by=body.payload.patient_ssn", nil, true)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected sensitive document reporting dimension to be blocked, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/data/reporting/query?source=documents&dimensions=body.payload.title&measures=sum:body.payload.patient_ssn&group_by=body.payload.title", nil, true)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected sensitive document reporting measure to be blocked, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDocumentFieldSecurityHidesPayloadAndExtensionFields(t *testing.T) {
	h := newTestHarness(t)

	createBody, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload": map[string]any{
			"title":       "Sensitive Visit",
			"patient_ssn": "999-11-2222",
		},
	})
	created := h.request(http.MethodPost, "/documents", createBody, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected document create to succeed, got %d body=%s", created.Code, created.Body.String())
	}
	var record document.Record
	if err := json.Unmarshal(created.Body.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal document create failed: %v", err)
	}
	if _, ok := record.Body.Payload["patient_ssn"]; ok {
		t.Fatalf("expected sensitive payload field to be hidden from create response, got %+v", record.Body.Payload)
	}

	extBody, _ := json.Marshal(map[string]any{
		"payload": map[string]any{
			"score":        9,
			"secret_score": 99,
		},
		"expected_version": record.Header.Version,
		"expected_etag":    record.Header.ETag,
	})
	updated := h.request(http.MethodPut, "/documents/"+record.Header.ID+"/extensions/analytics", extBody, true)
	if updated.Code != http.StatusOK {
		t.Fatalf("expected extension update to succeed, got %d body=%s", updated.Code, updated.Body.String())
	}
	var updatedRecord document.Record
	if err := json.Unmarshal(updated.Body.Bytes(), &updatedRecord); err != nil {
		t.Fatalf("unmarshal extension update failed: %v", err)
	}
	ext := document.ExtensionPayload(updatedRecord.Body.Payload, "analytics")
	if _, ok := ext["secret_score"]; ok {
		t.Fatalf("expected sensitive extension field to be hidden, got %+v", ext)
	}
	if ext["score"] != float64(9) {
		t.Fatalf("expected non-sensitive extension field to remain visible, got %+v", ext)
	}

	detail := h.request(http.MethodGet, "/ui/data/documents/"+record.Header.ID, nil, true)
	if detail.Code != http.StatusOK {
		t.Fatalf("expected ui document detail to succeed, got %d body=%s", detail.Code, detail.Body.String())
	}
	if strings.Contains(detail.Body.String(), "patient_ssn") || strings.Contains(detail.Body.String(), "secret_score") {
		t.Fatalf("expected ui document detail to hide sensitive fields, got %s", detail.Body.String())
	}
}

func TestDocumentFieldSecurityRejectsProtectedWritesAndSearchExcludesFields(t *testing.T) {
	h := newTestHarness(t)

	body, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload": map[string]any{
			"title":                "Blocked Visit",
			"locked_internal_note": "blocked",
		},
	})
	rr := h.request(http.MethodPost, "/documents", body, true)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected protected document field write to be blocked, got %d body=%s", rr.Code, rr.Body.String())
	}

	searchBody, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload": map[string]any{
			"title":       "Searchable Title",
			"patient_ssn": "123-45-6789",
		},
	})
	created := h.request(http.MethodPost, "/documents", searchBody, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected document create with hidden field to succeed, got %d body=%s", created.Code, created.Body.String())
	}

	index := search.IndexDefinition{
		Key:          "documents.secure.search",
		Title:        "Secure Documents",
		SourceKind:   "document",
		DocumentType: "generic_request",
		Modes:        []string{"keyword"},
		Fields: []search.IndexFieldDefinition{
			{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true},
			{Key: "patient_ssn", Path: "body.payload.patient_ssn", Type: "string", Searchable: true},
		},
	}
	if err := h.registerSearchIndex(index); err != nil {
		t.Fatalf("register secure search index failed: %v", err)
	}

	okBody, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload": map[string]any{
			"title": "Only Title Visible",
		},
	})
	created = h.request(http.MethodPost, "/documents", okBody, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected safe document create to succeed, got %d body=%s", created.Code, created.Body.String())
	}

	if _, err := h.search.RebuildIndex("documents.secure.search"); err != nil {
		t.Fatalf("expected secure search rebuild to succeed, got %v", err)
	}

	queryBody, _ := json.Marshal(map[string]any{"query": "123-45-6789"})
	result := h.request(http.MethodPost, "/search/indexes/documents.secure.search/query", queryBody, true)
	if result.Code != http.StatusOK {
		t.Fatalf("expected secure search query to succeed, got %d body=%s", result.Code, result.Body.String())
	}
	if strings.Contains(result.Body.String(), "patient_ssn") {
		t.Fatalf("expected secure search results to exclude sensitive field, got %s", result.Body.String())
	}
}

func TestSearchIndexRoutes(t *testing.T) {
	h := newTestHarness(t)

	body, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "vector enabled request"},
	})
	rr := h.request(http.MethodPost, "/documents", body, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected document create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/search/indexes", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected search index list to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	queryBody, _ := json.Marshal(map[string]any{"query": "request"})
	rr = h.request(http.MethodPost, "/search/indexes/documents.requests.search/query", queryBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected keyword search to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	queryBody, _ = json.Marshal(map[string]any{"vector_text": "enabled request"})
	rr = h.request(http.MethodPost, "/search/indexes/documents.requests.search/query/vector", queryBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected vector search to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ops/search/indexes", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected search ops list to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodPost, "/ops/search/indexes/documents.requests.search/rebuild", nil, true)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected async search rebuild to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDocumentExtensionViewsAndUpdates(t *testing.T) {
	h := newTestHarness(t)

	body, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "hello"},
	})
	rr := h.request(http.MethodPost, "/documents", body, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	header := created["header"].(map[string]any)
	id := header["id"].(string)
	etag := header["etag"].(string)
	version := int(header["version"].(float64))

	extBody, _ := json.Marshal(map[string]any{
		"expected_version": version,
		"expected_etag":    etag,
		"payload":          map[string]any{"score": 7},
	})
	rr = h.request(http.MethodPut, "/documents/"+id+"/extensions/analytics", extBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected extension update to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/documents/"+id, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected default document read, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	bodyPayload := payload["body"].(map[string]any)["payload"].(map[string]any)
	if _, ok := bodyPayload["extensions"]; ok {
		t.Fatal("expected normal view to hide extensions")
	}

	rr = h.request(http.MethodGet, "/documents/"+id+"?view=expanded", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected expanded document read, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	bodyPayload = payload["body"].(map[string]any)["payload"].(map[string]any)
	extensions := bodyPayload["extensions"].(map[string]any)
	if extensions["analytics"].(map[string]any)["score"].(float64) != 7 {
		t.Fatalf("expected expanded extension payload, got %+v", extensions)
	}

	rr = h.request(http.MethodGet, "/documents/"+id+"?view=raw", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected raw document read, got %d body=%s", rr.Code, rr.Body.String())
	}

	csrfHeader := h.csrf.Value
	req := httptest.NewRequest(http.MethodPost, "/admin/api/modules/analytics/actions/disable", nil)
	req.AddCookie(h.cookie)
	req.AddCookie(h.csrf)
	req.Header.Set("X-CSRF-Token", csrfHeader)
	disableRR := httptest.NewRecorder()
	h.router.ServeHTTP(disableRR, req)
	if disableRR.Code != http.StatusOK {
		t.Fatalf("expected module disable to succeed, got %d body=%s", disableRR.Code, disableRR.Body.String())
	}

	rr = h.request(http.MethodGet, "/documents/"+id+"?view=expanded", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected expanded document read after disable, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	bodyPayload = payload["body"].(map[string]any)["payload"].(map[string]any)
	if ext, ok := bodyPayload["extensions"].(map[string]any); ok && len(ext) > 0 {
		t.Fatalf("expected disabled module extension to be hidden, got %+v", ext)
	}
}

func configIntValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func TestObservabilityUsesPrometheusSafeStatusMetricNames(t *testing.T) {
	h := newTestHarness(t)

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}

	metrics := h.request(http.MethodGet, "/metrics", nil, true)
	if metrics.Code != http.StatusOK {
		t.Fatalf("expected 200 for /metrics, got %d", metrics.Code)
	}
	body := metrics.Body.String()
	if !strings.Contains(body, "http_responses_404_total") {
		t.Fatalf("expected 404 metric key in prometheus output, got %s", body)
	}
	if strings.Contains(body, "Not Found") {
		t.Fatalf("expected prometheus-safe metric names, got %s", body)
	}
}

func TestProtectedRoutesRequireAuthentication(t *testing.T) {
	h := newTestHarness(t)
	body, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "hello"},
	})
	for _, tc := range []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodGet, path: "/platform/context"},
		{method: http.MethodGet, path: "/documents"},
		{method: http.MethodPost, path: "/documents", body: body},
		{method: http.MethodGet, path: "/ops/dashboard"},
		{method: http.MethodGet, path: "/metrics"},
	} {
		rr := h.request(tc.method, tc.path, tc.body, false)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s, got %d", tc.method, tc.path, rr.Code)
		}
	}
}

func TestHeaderSpoofingDoesNotBypassAuth(t *testing.T) {
	h := newTestHarness(t)
	body, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "hello"},
	})
	req := httptest.NewRequest(http.MethodPost, "/documents", bytes.NewReader(body))
	req.Header.Set("X-User-ID", "user_admin")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected spoofed header to fail with 401, got %d", rr.Code)
	}
}

func TestCorrelationHeaderPropagation(t *testing.T) {
	h := newTestHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Correlation-ID", "corr-123")
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Header().Get("X-Correlation-ID") != "corr-123" {
		t.Fatalf("expected propagated correlation id, got %s", rr.Header().Get("X-Correlation-ID"))
	}
}

func TestDocumentRoutesAndOps(t *testing.T) {
	h := newTestHarness(t)
	body, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "hello"},
	})
	rr := h.request(http.MethodPost, "/documents", body, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	header := created["header"].(map[string]any)
	id := header["id"].(string)
	etag := header["etag"].(string)

	updateBody, _ := json.Marshal(map[string]any{"expected_version": 1, "expected_etag": etag, "payload": map[string]any{"title": "updated"}})
	rr = h.request(http.MethodPut, "/documents/"+id, updateBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	header = created["header"].(map[string]any)
	etag = header["etag"].(string)
	version := int(header["version"].(float64))

	submitBody, _ := json.Marshal(map[string]any{"action": "submit", "expected_version": 2, "expected_etag": etag})
	rr = h.request(http.MethodPost, "/documents/"+id+"/actions", submitBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 on submit, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	header = created["header"].(map[string]any)
	etag = header["etag"].(string)
	version = int(header["version"].(float64))

	approveBody, _ := json.Marshal(map[string]any{"action": "approve", "expected_version": version, "expected_etag": etag})
	rr = h.request(http.MethodPost, "/documents/"+id+"/actions", approveBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 on approve, got %d body=%s", rr.Code, rr.Body.String())
	}

	rejectDocBody, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "reject-me"},
	})
	rr = h.request(http.MethodPost, "/documents", rejectDocBody, true)
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	header = created["header"].(map[string]any)
	rejectID := header["id"].(string)
	rejectETag := header["etag"].(string)
	submitBody, _ = json.Marshal(map[string]any{"action": "submit", "expected_version": 1, "expected_etag": rejectETag})
	rr = h.request(http.MethodPost, "/documents/"+rejectID+"/actions", submitBody, true)
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	header = created["header"].(map[string]any)
	rejectETag = header["etag"].(string)
	rejectVersion := int(header["version"].(float64))
	rejectBody, _ := json.Marshal(map[string]any{"action": "reject", "expected_version": rejectVersion, "expected_etag": rejectETag})
	rr = h.request(http.MethodPost, "/documents/"+rejectID+"/actions", rejectBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 on reject, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	header = created["header"].(map[string]any)
	rejectETag = header["etag"].(string)
	rejectVersion = int(header["version"].(float64))
	reopenBody, _ := json.Marshal(map[string]any{"action": "reopen", "expected_version": rejectVersion, "expected_etag": rejectETag})
	rr = h.request(http.MethodPost, "/documents/"+rejectID+"/actions", reopenBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 on reopen, got %d body=%s", rr.Code, rr.Body.String())
	}

	cancelDocBody, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "cancel-me"},
	})
	rr = h.request(http.MethodPost, "/documents", cancelDocBody, true)
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	header = created["header"].(map[string]any)
	cancelID := header["id"].(string)
	cancelETag := header["etag"].(string)
	cancelBody, _ := json.Marshal(map[string]any{"action": "cancel", "expected_version": 1, "expected_etag": cancelETag})
	rr = h.request(http.MethodPost, "/documents/"+cancelID+"/actions", cancelBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 on cancel, got %d body=%s", rr.Code, rr.Body.String())
	}

	for _, path := range []string{"/documents", "/documents/" + id, "/ops/audit-events", "/ops/domain-events", "/ops/outbox", "/ops/dead-letters", "/ops/projections/documents", "/ops/workflow/tasks", "/ops/workflow/approvals", "/ops/dashboard", "/ops/analytics", "/ops/analytics/trends", "/ops/stats", "/metrics"} {
		rr = h.request(http.MethodGet, path, nil, true)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, rr.Code)
		}
	}
	rr = h.request(http.MethodPost, "/ops/analytics/snapshots", nil, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for analytics snapshot capture, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/snapshots", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics snapshots, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/trends", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics trends, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/query?window=current_state&limit=1", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics query, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/breakdown/documents?group_by=document_type", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics breakdown, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/rollups?granularity=daily&limit=10", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics rollups, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/rollups/breakdown/documents?granularity=daily&group_by=document_type", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics rollup breakdown, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/compare?window=current_state&limit=2", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics compare, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/facts?location_id=loc_hq", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics facts, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/reporting/documents/export?dimension=document_type", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics export, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "text/csv" {
		t.Fatalf("expected csv content type, got %s", rr.Header().Get("Content-Type"))
	}
	if rr.Header().Get("Content-Disposition") == "" {
		t.Fatal("expected content disposition header")
	}
	rr = h.request(http.MethodGet, "/ops/analytics/reporting/documents/export?dimension=document_type&format=xlsx", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics xlsx export, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/reporting/documents/export?dimension=document_type&format=pdf", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for analytics pdf export, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("expected pdf content type, got %s", rr.Header().Get("Content-Type"))
	}
	rr = h.request(http.MethodPost, "/ops/analytics/reports?name=Daily+Documents&dimension=document_type&format=csv&schedule=daily&delivery_channel=download", nil, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for report definition, got %d", rr.Code)
	}
	var reportDef map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &reportDef)
	reportID := reportDef["id"].(string)
	rr = h.request(http.MethodGet, "/ops/analytics/reports", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for report definitions, got %d", rr.Code)
	}
	rr = h.request(http.MethodPost, "/ops/analytics/reports/run?report_id="+reportID, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for report run, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/report-runs", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for report runs, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/report-artifacts", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for report artifacts, got %d", rr.Code)
	}
	var artifactsResp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &artifactsResp)
	items := artifactsResp["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected report artifacts")
	}
	artifactID := items[0].(map[string]any)["id"].(string)
	rr = h.request(http.MethodGet, "/ops/analytics/report-artifacts/"+artifactID, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for report artifact download, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Disposition") == "" {
		t.Fatal("expected artifact download headers")
	}
	rr = h.request(http.MethodPost, "/ops/analytics/report-artifacts/deliver?artifact_id="+artifactID+"&channel=download", nil, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for artifact delivery, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/report-deliveries", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for report deliveries, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/analytics/report-delivery-dead-letters", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for report delivery dead letters, got %d", rr.Code)
	}
	rr = h.request(http.MethodPost, "/ops/analytics/report-deliveries/retry?artifact_id="+artifactID+"&channel=download", nil, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for report delivery retry, got %d", rr.Code)
	}
	rr = h.request(http.MethodPost, "/ops/analytics/report-artifacts/deliver?artifact_id="+artifactID+"&channel=email&recipient=user@example.com", nil, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for email delivery, got %d", rr.Code)
	}
	rr = h.request(http.MethodPost, "/ops/analytics/report-retention/cleanup?before=2099-01-01T00:00:00Z", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for report retention cleanup, got %d", rr.Code)
	}
	rr = h.request(http.MethodPost, "/ops/outbox/dispatch", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for outbox dispatch, got %d", rr.Code)
	}
	rr = h.request(http.MethodGet, "/ops/projections/documents", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for projections, got %d", rr.Code)
	}
}

func TestModelRoutes(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/models/party", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected model list to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	body, _ := json.Marshal(map[string]any{"values": map[string]any{"name": "Acme Clinic", "email": "ops@acme.test", "status": "active"}})
	rr = h.request(http.MethodPost, "/models/party", body, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected model create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Record model.Record `json:"record"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal model create failed: %v", err)
	}
	if _, ok := payload.Record.Values["email"]; ok {
		t.Fatalf("expected sensitive email to be hidden from model create response, got %+v", payload.Record.Values)
	}
}

func TestModelFieldSecurityRejectsProtectedWrites(t *testing.T) {
	h := newTestHarness(t)

	body, _ := json.Marshal(map[string]any{"values": map[string]any{"name": "Acme Clinic", "internal_note": "private"}})
	rr := h.request(http.MethodPost, "/models/party", body, true)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected protected field write to be blocked, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestModelRelationCreateRoute(t *testing.T) {
	h := newTestHarness(t)

	list := h.request(http.MethodGet, "/models/party", nil, true)
	if list.Code != http.StatusOK {
		t.Fatalf("expected model list to succeed, got %d body=%s", list.Code, list.Body.String())
	}
	var payload struct {
		Items []model.Record `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(payload.Items) == 0 {
		t.Fatal("expected seeded party record")
	}

	body, _ := json.Marshal(map[string]any{"values": map[string]any{"name": "Finance Contact", "phone": "021-0001"}})
	rr := h.request(http.MethodPost, "/models/party/"+payload.Items[0].ID+"/relations/contacts", body, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected related create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	detail := h.request(http.MethodGet, "/models/party/"+payload.Items[0].ID, nil, true)
	if detail.Code != http.StatusOK {
		t.Fatalf("expected model detail to succeed, got %d body=%s", detail.Code, detail.Body.String())
	}
	if !strings.Contains(detail.Body.String(), "Finance Contact") {
		t.Fatalf("expected related contact in detail payload, got body=%s", detail.Body.String())
	}

	related := h.request(http.MethodGet, "/models/party/"+payload.Items[0].ID+"/relations/contacts?name=finance", nil, true)
	if related.Code != http.StatusOK {
		t.Fatalf("expected relation list to succeed, got %d body=%s", related.Code, related.Body.String())
	}
	if !strings.Contains(related.Body.String(), "Finance Contact") {
		t.Fatalf("expected filtered relation payload, got body=%s", related.Body.String())
	}
}

func TestModelCompositeUpdateRoute(t *testing.T) {
	h := newTestHarness(t)
	createBody, _ := json.Marshal(map[string]any{
		"values": map[string]any{"name": "Acme Clinic", "email": "ops@acme.test", "status": "active"},
		"relations": map[string]any{
			"contacts": []map[string]any{
				{"values": map[string]any{"name": "Ops Contact", "phone": "021-0002"}},
			},
		},
	})
	created := h.request(http.MethodPost, "/models/party", createBody, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected composite model create to succeed, got %d body=%s", created.Code, created.Body.String())
	}
	var createdPayload struct {
		Record model.Record `json:"record"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdPayload); err != nil {
		t.Fatalf("unmarshal create failed: %v", err)
	}

	detail := h.request(http.MethodGet, "/models/party/"+createdPayload.Record.ID, nil, true)
	if detail.Code != http.StatusOK {
		t.Fatalf("expected detail fetch to succeed, got %d body=%s", detail.Code, detail.Body.String())
	}
	var detailPayload struct {
		Record model.Record `json:"record"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &detailPayload); err != nil {
		t.Fatalf("unmarshal detail failed: %v", err)
	}
	related := h.request(http.MethodGet, "/models/party/"+createdPayload.Record.ID+"/relations/contacts", nil, true)
	if related.Code != http.StatusOK {
		t.Fatalf("expected related query to succeed, got %d body=%s", related.Code, related.Body.String())
	}
	var relatedPayload struct {
		Items []model.Record `json:"items"`
	}
	if err := json.Unmarshal(related.Body.Bytes(), &relatedPayload); err != nil {
		t.Fatalf("unmarshal related failed: %v", err)
	}
	if len(relatedPayload.Items) != 1 {
		t.Fatalf("expected seeded related contact, got %+v", relatedPayload)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"expected_version": detailPayload.Record.Version,
		"values":           map[string]any{"name": "Acme Updated", "email": "ops@acme.test", "status": "active"},
		"relations": map[string]any{
			"contacts": []map[string]any{
				{
					"id":               relatedPayload.Items[0].ID,
					"expected_version": relatedPayload.Items[0].Version,
					"values":           map[string]any{"name": "Ops Contact Updated", "phone": "021-0003"},
				},
				{
					"values": map[string]any{"name": "Finance Contact", "phone": "021-0004"},
				},
			},
		},
	})
	updated := h.request(http.MethodPut, "/models/party/"+createdPayload.Record.ID, updateBody, true)
	if updated.Code != http.StatusOK {
		t.Fatalf("expected composite model update to succeed, got %d body=%s", updated.Code, updated.Body.String())
	}
	finalDetail := h.request(http.MethodGet, "/models/party/"+createdPayload.Record.ID, nil, true)
	if finalDetail.Code != http.StatusOK {
		t.Fatalf("expected final detail fetch to succeed, got %d body=%s", finalDetail.Code, finalDetail.Body.String())
	}
	if !strings.Contains(finalDetail.Body.String(), "Ops Contact Updated") || !strings.Contains(finalDetail.Body.String(), "Finance Contact") {
		t.Fatalf("expected composite related updates in payload, got body=%s", finalDetail.Body.String())
	}
}

func TestConsistencyOpsEndpoints(t *testing.T) {
	h := newTestHarness(t)
	body, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "Consistency Probe"},
		"total_amount":    map[string]any{"amount_minor": 1000, "currency": "IDR"},
	})
	created := h.request(http.MethodPost, "/documents", body, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected 201 create, got %d", created.Code)
	}

	rr := h.request(http.MethodGet, "/ops/consistency/projections", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 projection consistency, got %d", rr.Code)
	}

	rr = h.request(http.MethodPost, "/ops/consistency/projections/document-summary/rebuild", nil, true)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 projection rebuild, got %d", rr.Code)
	}
	var job map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &job)
	jobID := job["id"].(string)
	for i := 0; i < 20; i++ {
		rr = h.request(http.MethodGet, "/ops/jobs/"+jobID, nil, true)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 job status, got %d", rr.Code)
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &job)
		if job["status"] == "succeeded" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job["status"] != "succeeded" {
		t.Fatalf("expected projection rebuild job to succeed, got %+v", job)
	}

	rr = h.request(http.MethodGet, "/ops/consistency/analytics", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 analytics consistency, got %d", rr.Code)
	}

	rr = h.request(http.MethodPost, "/ops/consistency/analytics/rebuild", nil, true)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 analytics rebuild, got %d", rr.Code)
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &job)
	jobID = job["id"].(string)
	for i := 0; i < 20; i++ {
		rr = h.request(http.MethodGet, "/ops/jobs/"+jobID, nil, true)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 analytics job status, got %d", rr.Code)
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &job)
		if job["status"] == "succeeded" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job["status"] != "succeeded" {
		t.Fatalf("expected analytics rebuild job to succeed, got %+v", job)
	}
}

func TestDocumentUpdateVersionConflict(t *testing.T) {
	h := newTestHarness(t)
	body, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "hello"},
	})
	rr := h.request(http.MethodPost, "/documents", body, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	id := created["header"].(map[string]any)["id"].(string)

	updateBody, _ := json.Marshal(map[string]any{"expected_version": 99, "payload": map[string]any{"title": "updated"}})
	rr = h.request(http.MethodPut, "/documents/"+id, updateBody, true)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 on update conflict, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDocumentSubmitVersionConflict(t *testing.T) {
	h := newTestHarness(t)
	body, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "hello"},
	})
	rr := h.request(http.MethodPost, "/documents", body, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	id := created["header"].(map[string]any)["id"].(string)

	submitBody, _ := json.Marshal(map[string]any{"action": "submit", "expected_version": 99})
	rr = h.request(http.MethodPost, "/documents/"+id+"/actions", submitBody, true)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUnsupportedDocumentAction(t *testing.T) {
	h := newTestHarness(t)
	body, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "hello"},
	})
	rr := h.request(http.MethodPost, "/documents", body, true)
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	id := created["header"].(map[string]any)["id"].(string)

	rr = h.request(http.MethodPost, "/documents/"+id+"/actions", []byte(`{"action":"bad"}`), true)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAdminConfigValidationEndpointAndJobRequeue(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/admin/api/config/validate", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for config validation, got %d body=%s", rr.Code, rr.Body.String())
	}
	var validation struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &validation); err != nil {
		t.Fatalf("decode config validation failed: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("expected valid config report, got body=%s", rr.Body.String())
	}

	rr = h.request(http.MethodPost, "/ops/search/indexes/documents.requests.search/rebuild", nil, true)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for enqueue rebuild, got %d body=%s", rr.Code, rr.Body.String())
	}
	var enqueued jobs.Job
	if err := json.Unmarshal(rr.Body.Bytes(), &enqueued); err != nil {
		t.Fatalf("decode enqueued job failed: %v", err)
	}
	rr = h.request(http.MethodPost, "/ops/jobs/"+enqueued.ID+"/requeue", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for job requeue, got %d body=%s", rr.Code, rr.Body.String())
	}
	var queued jobs.Job
	if err := json.Unmarshal(rr.Body.Bytes(), &queued); err != nil {
		t.Fatalf("decode queued job failed: %v", err)
	}
	if queued.Status != jobs.StatusQueued {
		t.Fatalf("expected queued job after requeue, got %+v", queued)
	}
}

func signGoogleTestToken(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid})
	if err != nil {
		t.Fatalf("marshal header failed: %v", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + payload
	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}
	return fmt.Sprintf("%s.%s", signingInput, base64.RawURLEncoding.EncodeToString(signature))
}
