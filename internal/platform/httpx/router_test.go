package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"clinic/internal/platform/activity"
	"clinic/internal/platform/analytics"
	application "clinic/internal/platform/application"
	"clinic/internal/platform/audit"
	"clinic/internal/platform/config"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/identity"
	"clinic/internal/platform/integration"
	"clinic/internal/platform/jobs"
	"clinic/internal/platform/logging"
	"clinic/internal/platform/model"
	"clinic/internal/platform/module"
	"clinic/internal/platform/monitoring"
	"clinic/internal/platform/observability"
	"clinic/internal/platform/organization"
	"clinic/internal/platform/policy"
	"clinic/internal/platform/reporting"
	"clinic/internal/platform/search"
	"clinic/internal/platform/workflow"
)

type testHarness struct {
	router http.Handler
	cookie *http.Cookie
	csrf   *http.Cookie
	ident  *identity.Service
	audit  *audit.Service
	cfg    *config.Service
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
	analyticsSvc := analytics.NewService(docs, flows, eventingSvc, searchSvc, auditSvc, obsSvc)
	monitoringSvc := monitoring.NewService(docs, eventingSvc, flows, searchSvc, obsSvc)
	integrationSvc := integration.NewService(obsSvc, loggerSvc)
	jobSvc := jobs.NewService()
	searchSvc.AttachJobs(jobSvc)
	integrationSvc.AttachPolicy(policySvc)
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
		{Key: "documents.numbering.assign", Kind: "numbering", Target: "document_numbering", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"prefix": "", "include_location": true, "include_date": true}},
		{Key: "documents.action.render", Kind: "ui", Target: "document_action_render", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"hidden_actions": []string{}, "primary_actions": []string{"submit", "approve"}}},
		{Key: "integration.submission.preflight", Kind: "integration", Target: "integration_submission", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"blocked_operation_types": []string{}, "required_system_status": "active"}},
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
			{Key: "email", Type: "string"},
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
		router: NewRouter(cfg, org, ident, modules, models, activities, reportingSvc, docs, flows, auditSvc, eventingSvc, searchSvc, loggerSvc, analyticsSvc, monitoringSvc, obsSvc, policySvc, integrationSvc, jobSvc, actions, modelActions),
		cookie: &http.Cookie{Name: sessionCookieName, Value: token},
		csrf:   csrfCookie,
		ident:  ident,
		audit:  auditSvc,
		cfg:    cfg,
	}
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
			Bundles: []module.BundleDefinition{{
				Key:    "analytics-cockpit",
				Script: AnalyticsCockpitBundle(),
			}},
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
}

func TestAdminModuleAndConfigRoutes(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/admin/api/modules", nil, true)
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
