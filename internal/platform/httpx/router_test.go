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
	"strconv"
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
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/idempotency"
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
	policy    *policy.Service
	search    *search.Service
	workflows *workflow.Service
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
	flags := featureflags.NewService()
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
	idempotencySvc := idempotency.NewService()
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
	actions := application.NewDocumentActions(docs, flows, ident, policySvc, application.NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))
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
		{Key: "documents.workflow.assignment", Kind: "workflow", Target: "document_assignment", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"assignee_role_key": "", "candidate_role_keys": []string{}, "assignment_mode": ""}},
		{Key: "documents.workflow.sla", Kind: "workflow", Target: "document_sla", AllowedScopes: []string{"deployment", "location"}, DefaultRule: map[string]any{"due_after_seconds": 0, "escalate_after_seconds": 0}},
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
	if err := policySvc.SetEvaluator("documents.workflow.assignment", func(req policy.Request) policy.Decision { return policy.Decision{Allowed: true} }); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	if err := policySvc.SetEvaluator("documents.workflow.sla", func(req policy.Request) policy.Decision { return policy.Decision{Allowed: true} }); err != nil {
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
		router:    NewRouter(cfg, flags, org, ident, modules, models, activities, reportingSvc, referenceSvc, docs, flows, auditSvc, eventingSvc, searchSvc, loggerSvc, analyticsSvc, monitoringSvc, obsSvc, policySvc, integrationSvc, idempotencySvc, jobSvc, health, actions, modelActions),
		cookie:    &http.Cookie{Name: sessionCookieName, Value: token},
		csrf:      csrfCookie,
		ident:     ident,
		audit:     auditSvc,
		cfg:       cfg,
		policy:    policySvc,
		search:    searchSvc,
		workflows: flows,
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
		{
			Key:               "platform.core",
			Name:              "Platform Core",
			Version:           "1.0.0",
			DomainFamily:      "platform",
			ConfigDefinitions: []config.Definition{httpDef, searchTypesenseDef, searchEmbeddingDef},
			Frontend: module.FrontendDefinition{
				Menus: []module.MenuDefinition{
					{
						Key:                 "admin.modules",
						Label:               "Modules",
						ActionKey:           "admin.modules",
						Order:               10,
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"module.read"},
					},
					{
						Key:                 "admin.auth",
						Label:               "Authentication",
						ActionKey:           "admin.auth",
						Order:               20,
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"configuration.read"},
					},
				},
				Actions: []module.ActionDefinition{
					{
						Key:                 "admin.modules",
						Label:               "Modules",
						Kind:                "navigate",
						RoutePath:           "/admin/modules",
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"module.read"},
					},
					{
						Key:                 "admin.auth",
						Label:               "Authentication",
						Kind:                "navigate",
						RoutePath:           "/admin/auth",
						Surface:             module.UISurfaceAdmin,
						RequiredPermissions: []string{"configuration.read"},
					},
				},
			},
		},
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
			SelfService: module.SelfServiceDefinition{
				APIs: []module.SelfServiceAPIDefinition{
					{
						Key:                 "documents.self_service.requests.list",
						Title:               "List Self-Service Requests",
						Method:              "GET",
						RoutePath:           "/ui/data/documents?type=generic_request",
						HandlerKind:         "ui_data",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.list"},
						AudienceKinds:       []string{"customer", "employee", "patient"},
						ResponseContractKey: "documents.generic_request.list",
					},
					{
						Key:                 "documents.self_service.requests.get",
						Title:               "Get Self-Service Request",
						Method:              "GET",
						RoutePath:           "/ui/data/documents/{documentID}",
						HandlerKind:         "ui_data",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.read"},
						AudienceKinds:       []string{"customer", "employee", "patient"},
						ResponseContractKey: "documents.generic_request.detail",
					},
					{
						Key:                 "documents.self_service.requests.create",
						Title:               "Create Self-Service Request",
						Method:              "POST",
						RoutePath:           "/document-flows/documents.self_service.requests.intake/commit",
						HandlerKind:         "flow_commit",
						DocumentType:        "generic_request",
						FlowKey:             "documents.self_service.requests.intake",
						RequiredPermissions: []string{"document.create"},
						AudienceKinds:       []string{"customer", "employee", "patient"},
						RequestContractKey:  "documents.generic_request.create",
						ResponseContractKey: "documents.generic_request.detail",
						Idempotent:          true,
					},
					{
						Key:                 "documents.self_service.requests.submit",
						Title:               "Submit Self-Service Request",
						Method:              "POST",
						RoutePath:           "/documents/{documentID}/actions",
						HandlerKind:         "document_action",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.submit"},
						AudienceKinds:       []string{"customer", "employee", "patient"},
						RequestContractKey:  "documents.generic_request.action.submit",
						ResponseContractKey: "documents.generic_request.detail",
						Idempotent:          true,
					},
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
				}, {
					Key:                 "documents.self_service.requests",
					Label:               "My Requests",
					Surface:             module.UISurfaceSelfService,
					ActionKey:           "documents.self_service.requests.list",
					Order:               8,
					RequiredPermissions: []string{"document.list"},
				}, {
					Key:                 "documents.worklist",
					Label:               "Worklist",
					Surface:             module.UISurfaceWorklist,
					ActionKey:           "documents.worklist.tasks",
					Order:               5,
					RequiredPermissions: []string{"document.list"},
				}},
				Actions: []module.ActionDefinition{
					{
						Key:                 "documents.requests.create",
						Label:               "New Request",
						Kind:                "navigate",
						RoutePath:           "/documents/new",
						FlowKey:             "documents.requests.intake",
						RenderMode:          module.RenderModeFlow,
						RequiredPermissions: []string{"document.create"},
					},
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
					{
						Key:                 "documents.self_service.requests.create",
						Label:               "New Request",
						Surface:             module.UISurfaceSelfService,
						Kind:                "navigate",
						RoutePath:           "/self-service/requests/new",
						FlowKey:             "documents.self_service.requests.intake",
						RenderMode:          module.RenderModeFlow,
						RequiredPermissions: []string{"document.create"},
					},
					{
						Key:                 "documents.self_service.requests.list",
						Label:               "My Requests",
						Surface:             module.UISurfaceSelfService,
						Kind:                "navigate",
						RoutePath:           "/self-service/requests",
						ViewKey:             "documents.self_service.requests.list",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.list"},
					},
					{
						Key:                 "documents.self_service.requests.detail",
						Label:               "Request Detail",
						Surface:             module.UISurfaceSelfService,
						Kind:                "navigate",
						RoutePath:           "/self-service/requests/detail",
						ViewKey:             "documents.self_service.requests.detail",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.read"},
					},
					{
						Key:                 "documents.self_service.requests.form",
						Label:               "Request Draft",
						Surface:             module.UISurfaceSelfService,
						Kind:                "navigate",
						RoutePath:           "/self-service/requests/form",
						ViewKey:             "documents.self_service.requests.form",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.update_draft"},
					},
					{
						Key:                 "documents.worklist.tasks",
						Label:               "Task Queue",
						Surface:             module.UISurfaceWorklist,
						Kind:                "navigate",
						RoutePath:           "/worklist",
						ViewKey:             "documents.worklist.tasks",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.list"},
					},
					{
						Key:                 "documents.worklist.approvals",
						Label:               "Approval Queue",
						Surface:             module.UISurfaceWorklist,
						Kind:                "navigate",
						RoutePath:           "/worklist/approvals",
						ViewKey:             "documents.worklist.approvals",
						RenderMode:          module.RenderModeGeneric,
						RequiredPermissions: []string{"document.list"},
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
						Printable:           true,
						PrintPurpose:        "official",
						PrintChannel:        "print",
						RequiredPermissions: []string{"document.read"},
						AllowedActions:      []string{"submit", "approve", "reject", "reopen", "cancel"},
						Tabs: []module.TabDefinition{{
							Key: "summary", Title: "Summary", Sections: []module.SectionDefinition{{
								Key: "header", Title: "Header", Fields: []module.FieldDefinition{{Key: "doc_id", Label: "Document ID", Path: "header.id", Type: "string"}},
							}},
						}},
					},
					{
						Key:                 "documents.self_service.requests.list",
						Title:               "My Requests",
						Surface:             module.UISurfaceSelfService,
						Kind:                "list",
						DocumentType:        "generic_request",
						ProjectionKey:       "document_summary",
						RequiredPermissions: []string{"document.list"},
						Columns: []module.ColumnDefinition{
							{Key: "id", Label: "Request", Path: "header.id"},
							{Key: "status", Label: "Status", Path: "header.status"},
							{Key: "updated_at", Label: "Updated", Path: "header.updated_at"},
						},
						Filters: []module.FilterDefinition{
							{Key: "status", Label: "Status", Type: "enum", Options: []string{"draft", "submitted", "approved", "rejected", "cancelled"}},
						},
						DefaultPageSize: 10,
						EmptyState:      "No self-service requests yet.",
					},
					{
						Key:                 "documents.self_service.requests.detail",
						Title:               "My Request",
						Surface:             module.UISurfaceSelfService,
						Kind:                "detail",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.read"},
						AllowedActions:      []string{"submit", "reopen", "cancel"},
						Tabs: []module.TabDefinition{{
							Key: "summary", Title: "Summary", Sections: []module.SectionDefinition{
								{
									Key: "header", Title: "Header", Fields: []module.FieldDefinition{
										{Key: "doc_id", Label: "Request ID", Path: "header.id", Type: "string"},
										{Key: "status", Label: "Status", Path: "header.status", Type: "string"},
									},
								},
								{
									Key: "payload", Title: "Payload", Fields: []module.FieldDefinition{
										{Key: "title", Label: "Title", Path: "body.payload.title", Type: "string"},
									},
								},
							},
						}},
						ActionPlacements: []module.ActionPlacementDefinition{
							{ActionKey: "submit", Zone: "primary"},
							{ActionKey: "reopen", Zone: "secondary"},
							{ActionKey: "cancel", Zone: "secondary", Style: "warn"},
						},
					},
					{
						Key:                 "documents.self_service.requests.form",
						Title:               "Request Form",
						Surface:             module.UISurfaceSelfService,
						Kind:                "form",
						DocumentType:        "generic_request",
						RequiredPermissions: []string{"document.update_draft"},
						Sections: []module.SectionDefinition{{
							Key: "request_fields", Title: "Request", Fields: []module.FieldDefinition{
								{Key: "title", Label: "Title", Path: "body.payload.title", Type: "string", Widget: "textarea", Placeholder: "Describe your request"},
							},
						}},
					},
					{
						Key:                 "documents.worklist.tasks",
						Title:               "Task Queue",
						Surface:             module.UISurfaceWorklist,
						Kind:                "queue",
						ProjectionKey:       "workflow.tasks",
						RequiredPermissions: []string{"document.list"},
						EmptyState:          "No open workflow tasks.",
					},
					{
						Key:                 "documents.worklist.approvals",
						Title:               "Approval Queue",
						Surface:             module.UISurfaceWorklist,
						Kind:                "queue",
						ProjectionKey:       "workflow.approvals",
						RequiredPermissions: []string{"document.list"},
						EmptyState:          "No pending workflow approvals.",
					},
				},
				DocumentFlows: []module.DocumentFlowDefinition{{
					Key:                 "documents.requests.intake",
					Title:               "Request Intake",
					RoutePath:           "/documents/new",
					PrimaryDocumentType: "generic_request",
					RequiredPermissions: []string{"document.create"},
					Steps: []module.DocumentFlowStepDefinition{
						{
							Key:   "intake",
							Title: "Primary Request",
							Documents: []module.DocumentFlowDocumentDefinition{{
								Key:           "request",
								Title:         "Request",
								DocumentType:  "generic_request",
								PrimaryOutput: true,
								Sections: []module.SectionDefinition{{
									Key: "request", Title: "Request", Fields: []module.FieldDefinition{
										{Key: "title", Label: "Title", Path: "body.payload.title", Type: "string"},
										{Key: "request_kind", Label: "Request Kind", Path: "body.payload.request_kind", Type: "string", Options: []string{"review", "followup"}},
									},
								}},
							}},
							NextRules: []module.DocumentFlowBranchRule{
								{Path: "documents.request.payload.request_kind", Equals: "review", NextStepKey: "review"},
								{Path: "documents.request.payload.request_kind", Equals: "followup", NextStepKey: "followup"},
							},
						},
						{
							Key:   "review",
							Title: "Review",
							Documents: []module.DocumentFlowDocumentDefinition{
								{Key: "review_note", Title: "Review Note", DocumentType: "generic_request", LinkType: "related_to", Fields: []module.FieldDefinition{{Key: "title", Label: "Title", Path: "body.payload.title", Type: "string"}}},
								{Key: "review_checklist", Title: "Checklist", DocumentType: "generic_request", LinkType: "related_to", Fields: []module.FieldDefinition{{Key: "title", Label: "Title", Path: "body.payload.title", Type: "string"}}},
							},
						},
						{
							Key:   "followup",
							Title: "Followup",
							Documents: []module.DocumentFlowDocumentDefinition{
								{Key: "followup_plan", Title: "Plan", DocumentType: "generic_request", LinkType: "related_to", Fields: []module.FieldDefinition{{Key: "title", Label: "Title", Path: "body.payload.title", Type: "string"}}},
							},
						},
					},
				}, {
					Key:                 "documents.self_service.requests.intake",
					Title:               "Self-Service Request",
					Surface:             module.UISurfaceSelfService,
					RoutePath:           "/self-service/requests/new",
					PrimaryDocumentType: "generic_request",
					RequiredPermissions: []string{"document.create"},
					Steps: []module.DocumentFlowStepDefinition{{
						Key:   "request",
						Title: "Request",
						Documents: []module.DocumentFlowDocumentDefinition{{
							Key:           "request",
							Title:         "Request",
							DocumentType:  "generic_request",
							PrimaryOutput: true,
							Sections: []module.SectionDefinition{{
								Key: "request_core", Title: "Request", Fields: []module.FieldDefinition{
									{Key: "title", Label: "Title", Path: "body.payload.title", Type: "string", Widget: "textarea", Placeholder: "Describe your request"},
								},
							}},
						}},
					}},
				}},
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
			Templates: []module.TemplateDefinition{{
				Key:                 "documents.generic_request.default",
				Title:               "Generic Request Print",
				TargetKind:          "document",
				TargetKey:           "generic_request",
				RendererKind:        "html",
				DefaultFormat:       "html",
				Formats:             []string{"html"},
				Purpose:             "official",
				Channel:             "print",
				RequiredPermissions: []string{"template.read", "template.render"},
				DefaultBody:         `<article><h1>{{ .document.Header.Type }}</h1><p>{{ index .document.Body.Payload "title" }}</p></article>`,
			}},
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

func (h testHarness) requestWithCookies(method, path string, body []byte, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.RemoteAddr = "192.0.2.10:1234"
	var csrf *http.Cookie
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		req.AddCookie(cookie)
		if cookie.Name == csrfCookieName {
			csrf = cookie
		}
	}
	if requiresCSRFProtection(method) && csrf != nil {
		req.Header.Set("X-CSRF-Token", csrf.Value)
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

func TestAdminReportingLinesCRUD(t *testing.T) {
	h := newTestHarness(t)
	manager, err := h.ident.CreateUser("rl-manager", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create manager failed: %v", err)
	}
	subject, err := h.ident.CreateUser("rl-subject", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create subject failed: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"subject_user_id":   subject.ID,
		"manager_user_id":   manager.ID,
		"relationship_type": "primary_manager",
		"organization_id":   "org_default",
		"location_id":       "loc_hq",
		"status":            "active",
	})
	create := h.request(http.MethodPost, "/admin/api/reporting-lines", body, true)
	if create.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", create.Code, create.Body.String())
	}

	list := h.request(http.MethodGet, "/admin/api/reporting-lines?subject_user_id="+url.QueryEscape(subject.ID), nil, true)
	if list.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", list.Code, list.Body.String())
	}
	var payload struct {
		Items []identity.ReportingLine `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ManagerUserID != manager.ID {
		t.Fatalf("unexpected reporting lines: %+v", payload.Items)
	}

	graph := h.request(http.MethodGet, "/admin/api/hierarchy/graph?subject_user_id="+url.QueryEscape(subject.ID), nil, true)
	if graph.Code != http.StatusOK {
		t.Fatalf("expected hierarchy graph 200, got %d body=%s", graph.Code, graph.Body.String())
	}
}

func TestAdminWorkflowSimulationIncludesRoutingPreview(t *testing.T) {
	h := newTestHarness(t)

	body, _ := json.Marshal(map[string]any{
		"current_state":   "draft",
		"action":          "submit",
		"actor_id":        "user_admin",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"additional_input": map[string]any{
			"requester_user_id": "user_admin",
		},
	})
	rr := h.request(http.MethodPost, "/admin/api/workflows/generic_request_flow/versions/1/simulate", body, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if _, ok := payload["routing_preview"]; !ok {
		t.Fatalf("expected routing_preview, got %+v", payload)
	}
	if _, ok := payload["simulation"]; !ok {
		t.Fatalf("expected simulation payload, got %+v", payload)
	}
}

func TestAdminWorkflowDraftLifecycleAndErrorRoutes(t *testing.T) {
	h := newTestHarness(t)

	versions := h.request(http.MethodGet, "/admin/api/workflows/generic_request_flow/versions", nil, true)
	if versions.Code != http.StatusOK {
		t.Fatalf("expected workflow versions to succeed, got %d body=%s", versions.Code, versions.Body.String())
	}

	createDraft := h.request(http.MethodPost, "/admin/api/workflows/generic_request_flow/drafts", nil, true)
	if createDraft.Code != http.StatusCreated {
		t.Fatalf("expected workflow draft create to succeed, got %d body=%s", createDraft.Code, createDraft.Body.String())
	}

	validate := h.request(http.MethodPost, "/admin/api/workflows/generic_request_flow/versions/1/validate", nil, true)
	if validate.Code != http.StatusOK {
		t.Fatalf("expected workflow validate to succeed, got %d body=%s", validate.Code, validate.Body.String())
	}

	invalidSim := h.request(http.MethodPost, "/admin/api/workflows/generic_request_flow/versions/1/simulate", []byte("{"), true)
	if invalidSim.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid simulation request to fail, got %d body=%s", invalidSim.Code, invalidSim.Body.String())
	}

	invalidDraft := h.request(http.MethodPut, "/admin/api/workflows/generic_request_flow/versions/2", []byte("{"), true)
	if invalidDraft.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid workflow draft body to fail, got %d body=%s", invalidDraft.Code, invalidDraft.Body.String())
	}

	publish := h.request(http.MethodPost, "/admin/api/workflows/generic_request_flow/versions/2/publish", nil, true)
	if publish.Code != http.StatusOK {
		t.Fatalf("expected workflow publish to succeed, got %d body=%s", publish.Code, publish.Body.String())
	}

	notFound := h.request(http.MethodGet, "/admin/api/workflows/generic_request_flow/versions/2/unknown", nil, true)
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("expected unknown workflow route to fail, got %d body=%s", notFound.Code, notFound.Body.String())
	}
}

func TestRootRedirectsToUI(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/", nil, false)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 for root redirect, got %d body=%s", rr.Code, rr.Body.String())
	}
	if location := rr.Header().Get("Location"); location != "/ui" {
		t.Fatalf("expected root redirect to /ui, got %q", location)
	}
}

func TestDevOpenAPIRoutesDisabledOutsideDevMode(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/dev/openapi.json", nil, false)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when dev docs are disabled, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/dev/swagger", nil, false)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 swagger when dev docs are disabled, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDevOpenAPIDocumentIncludesRuntimeHeadlessContracts(t *testing.T) {
	t.Setenv("APP_AUTH_DEV_MODE", "true")
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/dev/openapi.json", nil, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for openapi doc, got %d body=%s", rr.Code, rr.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode openapi doc failed: %v", err)
	}
	if payload["openapi"] != "3.1.0" {
		t.Fatalf("expected openapi 3.1.0, got %#v", payload["openapi"])
	}

	paths, _ := payload["paths"].(map[string]any)
	if _, ok := paths["/ui/bootstrap"]; !ok {
		t.Fatal("expected /ui/bootstrap in openapi paths")
	}
	if _, ok := paths["/models/{modelKey}"]; !ok {
		t.Fatal("expected /models/{modelKey} in openapi paths")
	}

	runtimeMeta, _ := payload["x-orbyte-runtime"].(map[string]any)
	modelKeys, _ := runtimeMeta["model_keys"].([]any)
	if !containsAnyString(modelKeys, "party") {
		t.Fatalf("expected runtime model_keys to include party, got %#v", modelKeys)
	}
	documentTypes, _ := runtimeMeta["document_types"].([]any)
	if !containsAnyString(documentTypes, "generic_request") {
		t.Fatalf("expected runtime document_types to include generic_request, got %#v", documentTypes)
	}

	modelPath, _ := paths["/models/{modelKey}"].(map[string]any)
	getOp, _ := modelPath["get"].(map[string]any)
	params, _ := getOp["parameters"].([]any)
	if len(params) == 0 {
		t.Fatal("expected model path parameters")
	}
	param, _ := params[0].(map[string]any)
	schema, _ := param["schema"].(map[string]any)
	enumValues, _ := schema["enum"].([]any)
	if !containsAnyString(enumValues, "party") {
		t.Fatalf("expected modelKey enum to include party, got %#v", enumValues)
	}

	uiPath, _ := paths["/ui/bootstrap"].(map[string]any)
	uiGet, _ := uiPath["get"].(map[string]any)
	if uiGet["x-orbyte-support-level"] != "public-headless" {
		t.Fatalf("expected /ui/bootstrap support level public-headless, got %#v", uiGet["x-orbyte-support-level"])
	}
}

func TestDevSwaggerUIRoute(t *testing.T) {
	t.Setenv("APP_AUTH_DEV_MODE", "true")
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/dev/swagger", nil, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for swagger ui, got %d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "/dev/openapi.json") {
		t.Fatalf("expected swagger ui to point at openapi json, got %s", body)
	}
	if !strings.Contains(body, "SwaggerUIBundle") {
		t.Fatalf("expected swagger ui script in body, got %s", body)
	}
}

func containsAnyString(items []any, expected string) bool {
	for _, item := range items {
		if value, ok := item.(string); ok && value == expected {
			return true
		}
	}
	return false
}

func TestNavigationPreferencesAndRoleDefaults(t *testing.T) {
	h := newTestHarness(t)

	if _, err := h.ident.SetRoleDefaultRoutes("role_admin", "/monitoring", "/admin/auth"); err != nil {
		t.Fatalf("set role defaults failed: %v", err)
	}

	rr := h.request(http.MethodGet, "/ui/bootstrap", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected ui bootstrap to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if got := payload["default_path"]; got != "/monitoring" {
		t.Fatalf("expected role-based ui default path, got %v", got)
	}

	rr = h.request(http.MethodGet, "/admin/api/bootstrap", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected admin bootstrap to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if got := payload["default_path"]; got != "/admin/auth" {
		t.Fatalf("expected role-based admin default path, got %v", got)
	}

	body, _ := json.Marshal(map[string]any{"preferred_user_route": "/documents", "preferred_admin_route": "/admin/modules"})
	rr = h.request(http.MethodPut, "/me/preferences/navigation", body, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected preference update to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/bootstrap", nil, true)
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if got := payload["default_path"]; got != "/documents" {
		t.Fatalf("expected user override ui default path, got %v", got)
	}

	rr = h.request(http.MethodGet, "/admin/api/bootstrap", nil, true)
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if got := payload["default_path"]; got != "/admin/modules" {
		t.Fatalf("expected user override admin default path, got %v", got)
	}
}

func TestUIPreferencesRoundTrip(t *testing.T) {
	h := newTestHarness(t)

	body, _ := json.Marshal(map[string]any{
		"surface":      "worklist",
		"route_path":   "/worklist",
		"view_key":     "worklist.tasks",
		"filters":      map[string]any{"tasks": map[string]any{"mine": "1", "due": "overdue"}},
		"columns":      []string{"target", "status"},
		"column_order": []string{"status", "target"},
		"density":      "compact",
	})
	rr := h.request(http.MethodPut, "/me/preferences/ui", body, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected ui preference update to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/me/preferences/ui?surface=worklist&route_path=%2Fworklist", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected ui preference read to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if payload["density"] != "compact" {
		t.Fatalf("expected compact density, got %+v", payload)
	}
	if payload["route_path"] != "/worklist" {
		t.Fatalf("expected route_path to round trip, got %+v", payload)
	}
	filters, ok := payload["filters"].(map[string]any)
	if !ok || filters["tasks"] == nil {
		t.Fatalf("expected stored filters, got %+v", payload)
	}
}

func TestOpsSearchRuntimeAndSchemaRoutes(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/ops/search/indexes", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected search index ops list to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if _, ok := payload["runtime_items"].([]any); !ok {
		t.Fatalf("expected runtime_items in ops search list, got %+v", payload)
	}

	rr = h.request(http.MethodGet, "/ops/search/indexes/documents.requests.search/runtime", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected search runtime detail to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	record, err := document.NewService().Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "Repair Me"})
	if err != nil {
		t.Fatalf("create helper document failed: %v", err)
	}
	h.search.RefreshDocument(record)

	rr = h.request(http.MethodGet, "/ops/search/indexes/documents.requests.search/consistency", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected search consistency route to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodPost, "/ops/search/indexes/documents.requests.search/schema/plan", mustJSON(t, map[string]any{"version": "v2"}), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected schema plan route to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.request(http.MethodPost, "/ops/search/indexes/documents.requests.search/schema/build", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected schema build route to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.request(http.MethodPost, "/ops/search/indexes/documents.requests.search/schema/activate", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected schema activate route to succeed, got %d body=%s", rr.Code, rr.Body.String())
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

func TestServicePrincipalManagementRoutes(t *testing.T) {
	h := newTestHarness(t)

	createBody := mustJSON(t, map[string]any{
		"key":                     "billing_worker",
		"allowed_operation_types": []string{"billing.sync", "outbox.dispatch"},
	})
	rr := h.request(http.MethodPost, "/service-principals", createBody, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected service principal create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	var created identity.ServicePrincipal
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode service principal: %v", err)
	}
	if created.ID == "" || created.Key != "billing_worker" {
		t.Fatalf("unexpected service principal payload: %+v", created)
	}

	rr = h.request(http.MethodGet, "/service-principals", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected list to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	rr = h.request(http.MethodGet, "/service-principals/"+created.ID, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected detail to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodPost, "/service-principals/"+created.ID+"/tokens", mustJSON(t, map[string]any{"ttl_seconds": 120}), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected token issuance to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var tokenPayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &tokenPayload)
	if strings.TrimSpace(tokenPayload["token"].(string)) == "" {
		t.Fatal("expected issued token")
	}

	rr = h.request(http.MethodPut, "/service-principals/"+created.ID+"/status", mustJSON(t, map[string]any{"status": "disabled"}), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status update to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDocumentLinksAndAttachmentsRoutes(t *testing.T) {
	h := newTestHarness(t)

	first := h.request(http.MethodPost, "/documents", mustJSON(t, map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "Primary"},
	}), true)
	second := h.request(http.MethodPost, "/documents", mustJSON(t, map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "Related"},
	}), true)
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("expected document creation to succeed, got %d and %d", first.Code, second.Code)
	}

	var primary document.Record
	var related document.Record
	_ = json.Unmarshal(first.Body.Bytes(), &primary)
	_ = json.Unmarshal(second.Body.Bytes(), &related)

	link := h.request(http.MethodPost, "/documents/"+primary.Header.ID+"/links", mustJSON(t, map[string]any{
		"linked_document_id": related.Header.ID,
		"link_type":          "related_to",
		"metadata":           map[string]any{"source": "test"},
	}), true)
	if link.Code != http.StatusCreated {
		t.Fatalf("expected link create to succeed, got %d body=%s", link.Code, link.Body.String())
	}
	var linkPayload document.Link
	_ = json.Unmarshal(link.Body.Bytes(), &linkPayload)

	attachment := h.request(http.MethodPost, "/documents/"+primary.Header.ID+"/attachments", mustJSON(t, map[string]any{
		"attachment_type": "document",
		"file_name":       "summary.pdf",
		"content_type":    "application/pdf",
		"storage_key":     "object://docs/summary.pdf",
		"size_bytes":      42,
	}), true)
	if attachment.Code != http.StatusCreated {
		t.Fatalf("expected attachment create to succeed, got %d body=%s", attachment.Code, attachment.Body.String())
	}
	var attachmentPayload document.Attachment
	_ = json.Unmarshal(attachment.Body.Bytes(), &attachmentPayload)

	listLinks := h.request(http.MethodGet, "/documents/"+primary.Header.ID+"/links", nil, true)
	listAttachments := h.request(http.MethodGet, "/documents/"+primary.Header.ID+"/attachments", nil, true)
	if listLinks.Code != http.StatusOK || listAttachments.Code != http.StatusOK {
		t.Fatalf("expected metadata list routes to succeed, got %d and %d", listLinks.Code, listAttachments.Code)
	}

	removeLink := h.request(http.MethodDelete, "/documents/"+primary.Header.ID+"/links/"+linkPayload.ID, nil, true)
	removeAttachment := h.request(http.MethodDelete, "/documents/"+primary.Header.ID+"/attachments/"+attachmentPayload.ID, nil, true)
	if removeLink.Code != http.StatusNoContent || removeAttachment.Code != http.StatusNoContent {
		t.Fatalf("expected metadata delete routes to succeed, got %d and %d", removeLink.Code, removeAttachment.Code)
	}
}

func TestIntegrationSubmissionIdempotency(t *testing.T) {
	h := newTestHarness(t)

	body := mustJSON(t, map[string]any{
		"idempotency_key": "submission-1",
		"system_key":      "fake_erp",
		"operation_type":  "submit_document",
		"document_id":     "doc-1",
		"correlation_id":  "corr-1",
		"payload":         map[string]any{"foo": "bar"},
	})
	first := h.request(http.MethodPost, "/admin/api/integrations/submissions", body, true)
	second := h.request(http.MethodPost, "/admin/api/integrations/submissions", body, true)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("expected idempotent submission requests to succeed, got %d and %d", first.Code, second.Code)
	}

	var firstPayload map[string]map[string]any
	var secondPayload map[string]map[string]any
	_ = json.Unmarshal(first.Body.Bytes(), &firstPayload)
	_ = json.Unmarshal(second.Body.Bytes(), &secondPayload)
	if firstPayload["record"]["id"] != secondPayload["record"]["id"] {
		t.Fatalf("expected cached record id, got %v and %v", firstPayload["record"]["id"], secondPayload["record"]["id"])
	}

	idempotencyList := h.request(http.MethodGet, "/admin/api/idempotency/records?operation=integration.submission.create", nil, true)
	if idempotencyList.Code != http.StatusOK {
		t.Fatalf("expected idempotency record listing to succeed, got %d body=%s", idempotencyList.Code, idempotencyList.Body.String())
	}
}

func TestAdminFeatureFlagRoutes(t *testing.T) {
	h := newTestHarness(t)

	defs := h.request(http.MethodGet, "/admin/api/feature-flags/definitions", nil, true)
	if defs.Code != http.StatusOK {
		t.Fatalf("expected feature flag definitions to load, got %d body=%s", defs.Code, defs.Body.String())
	}

	update := h.request(http.MethodPut, "/admin/api/feature-flags/platform.admin_console/value", mustJSON(t, map[string]any{
		"scope":    "location",
		"scope_id": "loc_hq",
		"enabled":  false,
		"status":   "active",
	}), true)
	if update.Code != http.StatusOK {
		t.Fatalf("expected feature flag update to succeed, got %d body=%s", update.Code, update.Body.String())
	}

	values := h.request(http.MethodGet, "/admin/api/feature-flags/values", nil, true)
	effective := h.request(http.MethodGet, "/admin/api/feature-flags/effective?location_id=loc_hq", nil, true)
	targeting := h.request(http.MethodGet, "/admin/api/feature-flags/targeting?flag_key=platform.admin_console&location_id=loc_hq", nil, true)
	if values.Code != http.StatusOK || effective.Code != http.StatusOK || targeting.Code != http.StatusOK {
		t.Fatalf("expected feature flag reads to succeed, got %d, %d, and %d", values.Code, effective.Code, targeting.Code)
	}
}

func TestAdminConfigBundleCompareMatrixAndReadinessRoutes(t *testing.T) {
	h := newTestHarness(t)

	compare := h.request(http.MethodGet, "/admin/api/config/compare?left_organization_id=&left_location_id=&right_organization_id=org_default&right_location_id=loc_hq", nil, true)
	if compare.Code != http.StatusOK {
		t.Fatalf("expected config compare to succeed, got %d body=%s", compare.Code, compare.Body.String())
	}

	exportBody := mustJSON(t, map[string]any{
		"name":          "smoke-bundle",
		"config_keys":   []string{"identity.auth"},
		"config_scopes": []string{"deployment"},
		"include_flags": true,
		"flag_keys":     []string{"platform.admin_console"},
	})
	exported := h.request(http.MethodPost, "/admin/api/config/bundles/export", exportBody, true)
	if exported.Code != http.StatusOK {
		t.Fatalf("expected bundle export to succeed, got %d body=%s", exported.Code, exported.Body.String())
	}
	var exportPayload map[string]any
	_ = json.Unmarshal(exported.Body.Bytes(), &exportPayload)
	bundle := exportPayload["bundle"]
	validate := h.request(http.MethodPost, "/admin/api/config/bundles/validate", mustJSON(t, map[string]any{"bundle": bundle}), true)
	if validate.Code != http.StatusOK {
		t.Fatalf("expected bundle validate to succeed, got %d body=%s", validate.Code, validate.Body.String())
	}

	applyBundle := map[string]any{
		"name": "apply-bundle",
		"config_entries": []any{
			map[string]any{
				"key":        "identity.auth",
				"module_key": "identity",
				"category":   "security",
				"scope":      "location",
				"scope_id":   "loc_hq",
				"value":      map[string]any{"login_rate_limit_attempts": 3},
			},
		},
		"feature_flags": []any{
			map[string]any{
				"flag_key": "platform.admin_console",
				"scope":    "location",
				"scope_id": "loc_hq",
				"enabled":  false,
				"status":   "active",
			},
		},
	}
	applied := h.request(http.MethodPost, "/admin/api/config/bundles/apply", mustJSON(t, map[string]any{"bundle": applyBundle}), true)
	if applied.Code != http.StatusOK {
		t.Fatalf("expected bundle apply to succeed, got %d body=%s", applied.Code, applied.Body.String())
	}

	matrix := h.request(http.MethodGet, "/admin/api/security/role-permission-matrix", nil, true)
	if matrix.Code != http.StatusOK {
		t.Fatalf("expected role permission matrix to succeed, got %d body=%s", matrix.Code, matrix.Body.String())
	}

	grant := h.request(http.MethodPut, "/admin/api/security/roles/role_admin/permissions/audit.read/value", nil, true)
	if grant.Code != http.StatusOK {
		t.Fatalf("expected role permission grant to succeed, got %d body=%s", grant.Code, grant.Body.String())
	}
	revoke := h.request(http.MethodDelete, "/admin/api/security/roles/role_admin/permissions/audit.read/value", nil, true)
	if revoke.Code != http.StatusOK {
		t.Fatalf("expected role permission revoke to succeed, got %d body=%s", revoke.Code, revoke.Body.String())
	}

	readiness := h.request(http.MethodGet, "/admin/api/readiness", nil, true)
	if readiness.Code != http.StatusOK {
		t.Fatalf("expected admin readiness to succeed, got %d body=%s", readiness.Code, readiness.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(readiness.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode readiness payload failed: %v", err)
	}
	if blocked, _ := payload["blocked_for_apply"].(bool); blocked {
		t.Fatalf("expected healthy compatibility diagnostics to remain non-blocking, got %s", readiness.Body.String())
	}
	health, _ := payload["health"].(map[string]any)
	if ready, _ := health["ready"].(bool); !ready {
		t.Fatalf("expected admin readiness health snapshot to be ready, got %s", readiness.Body.String())
	}
}

func TestProjectionStatusRoute(t *testing.T) {
	h := newTestHarness(t)

	created := h.request(http.MethodPost, "/documents", mustJSON(t, map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "Projection Track"},
	}), true)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected document create to succeed, got %d body=%s", created.Code, created.Body.String())
	}

	status := h.request(http.MethodGet, "/ops/projections/status", nil, true)
	if status.Code != http.StatusOK {
		t.Fatalf("expected projection status route to succeed, got %d body=%s", status.Code, status.Body.String())
	}
}

func TestAuditQueryAndTimelineRoutes(t *testing.T) {
	h := newTestHarness(t)

	created := h.request(http.MethodPost, "/documents", mustJSON(t, map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "Audited"},
	}), true)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected document create to succeed, got %d body=%s", created.Code, created.Body.String())
	}
	var record document.Record
	_ = json.Unmarshal(created.Body.Bytes(), &record)

	updated := h.request(http.MethodPut, "/documents/"+record.Header.ID, mustJSON(t, map[string]any{
		"payload": map[string]any{"title": "Audited V2"},
	}), true)
	if updated.Code != http.StatusOK {
		t.Fatalf("expected document update to succeed, got %d body=%s", updated.Code, updated.Body.String())
	}

	list := h.request(http.MethodGet, "/ops/audit-events?target_type=document&target_id="+record.Header.ID+"&action=document.update", nil, true)
	timeline := h.request(http.MethodGet, "/ops/audit-events/document/"+record.Header.ID, nil, true)
	if list.Code != http.StatusOK || timeline.Code != http.StatusOK {
		t.Fatalf("expected audit routes to succeed, got %d and %d", list.Code, timeline.Code)
	}
}

func TestDelegationRoutesAndDelegatedDocumentAudit(t *testing.T) {
	h := newTestHarness(t)
	delegate, err := h.ident.CreateUser("delegate_clerk", "clerk-pass-123", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create delegate user failed: %v", err)
	}

	grantResp := h.request(http.MethodPost, "/me/delegations/outgoing", mustJSON(t, map[string]any{
		"delegate_user_id":        delegate.ID,
		"location_id":             "loc_hq",
		"allowed_permission_keys": []string{"document.create", "document.update_draft", "document.submit", "document.approve"},
		"allowed_document_types":  []string{"generic_request"},
		"expires_at":              time.Now().UTC().Add(time.Hour),
		"reason":                  "cover shift",
	}), true)
	if grantResp.Code != http.StatusCreated {
		t.Fatalf("expected delegation grant create to succeed, got %d body=%s", grantResp.Code, grantResp.Body.String())
	}
	var grant identity.DelegationGrant
	if err := json.Unmarshal(grantResp.Body.Bytes(), &grant); err != nil {
		t.Fatalf("decode delegation grant: %v", err)
	}

	loginResp := h.request(http.MethodPost, "/auth/login", mustJSON(t, map[string]any{
		"username":    "delegate_clerk",
		"password":    "clerk-pass-123",
		"location_id": "loc_hq",
	}), false)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("expected clerk login to succeed, got %d body=%s", loginResp.Code, loginResp.Body.String())
	}
	clerkSession := findCookieByName(loginResp.Result().Cookies(), sessionCookieName)
	clerkCSRF := findCookieByName(loginResp.Result().Cookies(), csrfCookieName)
	if clerkSession == nil || clerkCSRF == nil {
		t.Fatal("expected clerk auth cookies on login")
	}

	incoming := h.requestWithCookies(http.MethodGet, "/me/delegations/incoming", nil, clerkSession, clerkCSRF)
	if incoming.Code != http.StatusOK {
		t.Fatalf("expected incoming delegations to load, got %d body=%s", incoming.Code, incoming.Body.String())
	}

	acceptResp := h.requestWithCookies(http.MethodPost, "/me/delegations/incoming/"+grant.ID+"/accept", nil, clerkSession, clerkCSRF)
	if acceptResp.Code != http.StatusOK {
		t.Fatalf("expected delegation accept to succeed, got %d body=%s", acceptResp.Code, acceptResp.Body.String())
	}

	activateResp := h.requestWithCookies(http.MethodPost, "/auth/delegation/activate", mustJSON(t, map[string]any{"grant_id": grant.ID}), clerkSession, clerkCSRF)
	if activateResp.Code != http.StatusOK {
		t.Fatalf("expected delegation activate to succeed, got %d body=%s", activateResp.Code, activateResp.Body.String())
	}
	delegationCookie := findCookieByName(activateResp.Result().Cookies(), delegationCookieName)
	if delegationCookie == nil {
		t.Fatal("expected delegation cookie after activation")
	}

	contextResp := h.requestWithCookies(http.MethodGet, "/auth/context", nil, clerkSession, clerkCSRF, delegationCookie)
	if contextResp.Code != http.StatusOK {
		t.Fatalf("expected auth context to succeed, got %d body=%s", contextResp.Code, contextResp.Body.String())
	}
	var authContext map[string]any
	if err := json.Unmarshal(contextResp.Body.Bytes(), &authContext); err != nil {
		t.Fatalf("decode auth context: %v", err)
	}
	if authContext["actor_user_id"] != delegate.ID || authContext["effective_user_id"] != "user_admin" || authContext["delegation_active"] != true {
		t.Fatalf("unexpected auth context: %+v", authContext)
	}

	created := h.requestWithCookies(http.MethodPost, "/documents", mustJSON(t, map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "Delegated Request"},
	}), clerkSession, clerkCSRF, delegationCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected delegated document create to succeed, got %d body=%s", created.Code, created.Body.String())
	}
	var record document.Record
	if err := json.Unmarshal(created.Body.Bytes(), &record); err != nil {
		t.Fatalf("decode delegated document create: %v", err)
	}

	updated := h.requestWithCookies(http.MethodPut, "/documents/"+record.Header.ID, mustJSON(t, map[string]any{
		"payload":          map[string]any{"title": "Delegated Request Updated"},
		"expected_version": record.Header.Version,
		"expected_etag":    record.Header.ETag,
	}), clerkSession, clerkCSRF, delegationCookie)
	if updated.Code != http.StatusOK {
		t.Fatalf("expected delegated document update to succeed, got %d body=%s", updated.Code, updated.Body.String())
	}
	if err := json.Unmarshal(updated.Body.Bytes(), &record); err != nil {
		t.Fatalf("decode delegated document update: %v", err)
	}

	submitResp := h.requestWithCookies(http.MethodPost, "/documents/"+record.Header.ID+"/actions", mustJSON(t, map[string]any{
		"action":           "submit",
		"expected_version": record.Header.Version,
		"expected_etag":    record.Header.ETag,
	}), clerkSession, clerkCSRF, delegationCookie)
	if submitResp.Code != http.StatusOK {
		t.Fatalf("expected delegated submit to succeed, got %d body=%s", submitResp.Code, submitResp.Body.String())
	}
	if err := json.Unmarshal(submitResp.Body.Bytes(), &record); err != nil {
		t.Fatalf("decode delegated document submit: %v", err)
	}

	approveResp := h.requestWithCookies(http.MethodPost, "/documents/"+record.Header.ID+"/actions", mustJSON(t, map[string]any{
		"action":           "approve",
		"expected_version": record.Header.Version,
		"expected_etag":    record.Header.ETag,
	}), clerkSession, clerkCSRF, delegationCookie)
	if approveResp.Code != http.StatusOK {
		t.Fatalf("expected delegated approve to succeed, got %d body=%s", approveResp.Code, approveResp.Body.String())
	}

	auditEvents := h.audit.Query(audit.Query{TargetType: "document", TargetID: record.Header.ID, OnBehalfOfUserID: "user_admin"})
	if len(auditEvents) == 0 {
		t.Fatal("expected delegated audit events for document")
	}
	actions := map[string]bool{
		"document.create":  false,
		"document.update":  false,
		"document.submit":  false,
		"document.approve": false,
	}
	for _, event := range auditEvents {
		if _, ok := actions[event.Action]; !ok {
			continue
		}
		if event.ActorID != delegate.ID || event.OnBehalfOfUserID != "user_admin" || event.DelegationGrantID != grant.ID {
			t.Fatalf("unexpected delegated audit event: %+v", event)
		}
		actions[event.Action] = true
	}
	for action, seen := range actions {
		if !seen {
			t.Fatalf("expected audit event for %s, got %+v", action, auditEvents)
		}
	}

	exitResp := h.requestWithCookies(http.MethodPost, "/auth/delegation/exit", nil, clerkSession, clerkCSRF, delegationCookie)
	if exitResp.Code != http.StatusOK {
		t.Fatalf("expected delegation exit to succeed, got %d body=%s", exitResp.Code, exitResp.Body.String())
	}
	contextResp = h.requestWithCookies(http.MethodGet, "/auth/context", nil, clerkSession, clerkCSRF)
	if contextResp.Code != http.StatusOK {
		t.Fatalf("expected self auth context after exit, got %d body=%s", contextResp.Code, contextResp.Body.String())
	}
	authContext = map[string]any{}
	if err := json.Unmarshal(contextResp.Body.Bytes(), &authContext); err != nil {
		t.Fatalf("decode self auth context: %v", err)
	}
	if authContext["actor_user_id"] != delegate.ID || authContext["effective_user_id"] != delegate.ID || authContext["delegation_active"] != false {
		t.Fatalf("unexpected self auth context after exit: %+v", authContext)
	}
}

func TestOperatingUnitAdminRoute(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodPut, "/admin/api/operating-units/ou_clinic_ops/value", mustJSON(t, map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"key":             "clinic_ops",
		"name":            "Clinic Ops",
		"status":          "active",
	}), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected operating unit upsert to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	list := h.request(http.MethodGet, "/admin/api/operating-units", nil, true)
	if list.Code != http.StatusOK {
		t.Fatalf("expected operating unit list to succeed, got %d body=%s", list.Code, list.Body.String())
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
	healthRR := h.request(http.MethodGet, "/ops/health", nil, true)
	if healthRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for /ops/health, got %d body=%s", healthRR.Code, healthRR.Body.String())
	}
	if !strings.Contains(healthRR.Body.String(), "\"summary\"") {
		t.Fatalf("expected structured health response, got %s", healthRR.Body.String())
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

func TestOpsTraceEndpointStitchesCorrelatedDocumentSubmit(t *testing.T) {
	h := newTestHarness(t)

	createBody, _ := json.Marshal(map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "Trace Request"},
	})
	created := h.request(http.MethodPost, "/documents", createBody, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create to succeed, got %d body=%s", created.Code, created.Body.String())
	}
	var record document.Record
	if err := json.Unmarshal(created.Body.Bytes(), &record); err != nil {
		t.Fatalf("decode created document failed: %v", err)
	}

	traceID := "trace-doc-submit"
	submitBody, _ := json.Marshal(map[string]any{"action": "submit", "expected_version": record.Header.Version, "expected_etag": record.Header.ETag})
	req := httptest.NewRequest(http.MethodPost, "/documents/"+record.Header.ID+"/actions", bytes.NewReader(submitBody))
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(h.cookie)
	req.AddCookie(h.csrf)
	req.Header.Set("X-CSRF-Token", h.csrf.Value)
	req.Header.Set("X-Correlation-ID", traceID)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected submit to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	trace := h.request(http.MethodGet, "/ops/traces/"+traceID, nil, true)
	if trace.Code != http.StatusOK {
		t.Fatalf("expected trace endpoint to succeed, got %d body=%s", trace.Code, trace.Body.String())
	}
	body := trace.Body.String()
	for _, marker := range []string{"\"kind\":\"http\"", "\"kind\":\"audit\"", "\"kind\":\"domain_event\"", "\"kind\":\"workflow_history\"", "\"kind\":\"outbox\""} {
		if !strings.Contains(body, marker) {
			t.Fatalf("expected trace to contain %s, got %s", marker, body)
		}
	}

	correlationAudit := h.request(http.MethodGet, "/ops/audit-events/correlation/"+traceID, nil, true)
	if correlationAudit.Code != http.StatusOK {
		t.Fatalf("expected correlation audit route to succeed, got %d body=%s", correlationAudit.Code, correlationAudit.Body.String())
	}
	if !strings.Contains(correlationAudit.Body.String(), traceID) {
		t.Fatalf("expected correlation audit payload to include trace id, got %s", correlationAudit.Body.String())
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
	router := NewRouter(cfg, featureflags.NewService(), org, ident, module.NewService(), models, activity.NewService(), reportingSvc, reference.NewService(), docs, flows, auditSvc, eventingSvc, searchSvc, loggerSvc, analyticsSvc, monitoringSvc, obsSvc, policySvc, integrationSvc, idempotency.NewService(), jobSvc, health, application.NewDocumentActions(docs, flows, ident, policySvc, application.NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc)), application.NewMemoryModelActions(models, activity.NewService(), auditSvc, eventingSvc))

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

func TestIntegrationAdminConfigAndOpsHealthRoutes(t *testing.T) {
	h := newTestHarness(t)

	adapters := h.request(http.MethodGet, "/admin/api/integrations/adapters", nil, true)
	if adapters.Code != http.StatusOK {
		t.Fatalf("expected adapter list to succeed, got %d body=%s", adapters.Code, adapters.Body.String())
	}

	updateConfig := h.request(http.MethodPut, "/admin/api/integrations/systems/http_bridge/config", mustJSON(t, map[string]any{
		"settings": map[string]any{
			"url":          "https://example.test/submit",
			"bearer_token": "secret-token-123",
		},
	}), true)
	if updateConfig.Code != http.StatusOK {
		t.Fatalf("expected config update to succeed, got %d body=%s", updateConfig.Code, updateConfig.Body.String())
	}

	getConfig := h.request(http.MethodGet, "/admin/api/integrations/systems/http_bridge/config", nil, true)
	if getConfig.Code != http.StatusOK {
		t.Fatalf("expected config read to succeed, got %d body=%s", getConfig.Code, getConfig.Body.String())
	}
	if strings.Contains(getConfig.Body.String(), "secret-token-123") {
		t.Fatalf("expected redacted config response, got %s", getConfig.Body.String())
	}

	health := h.request(http.MethodGet, "/ops/integrations/health", nil, true)
	if health.Code != http.StatusOK {
		t.Fatalf("expected integration health route to succeed, got %d body=%s", health.Code, health.Body.String())
	}
}

func TestIntegrationSubmissionDetailRoutes(t *testing.T) {
	h := newTestHarness(t)

	create := h.request(http.MethodPost, "/admin/api/integrations/submissions", mustJSON(t, map[string]any{
		"system_key":       "fake_erp",
		"contract_key":     "document.submit",
		"contract_version": 1,
		"operation_type":   "sync_customer",
		"idempotency_key":  "integration-detail-1",
		"payload":          map[string]any{"customer_id": "cust-1"},
		"process_now":      true,
	}), true)
	if create.Code != http.StatusAccepted && create.Code != http.StatusOK {
		t.Fatalf("expected submission create to succeed, got %d body=%s", create.Code, create.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(create.Body.Bytes(), &payload)
	record := payload["record"].(map[string]any)
	id := record["id"].(string)

	detail := h.request(http.MethodGet, "/admin/api/integrations/submissions/"+id, nil, true)
	if detail.Code != http.StatusOK {
		t.Fatalf("expected submission detail to succeed, got %d body=%s", detail.Code, detail.Body.String())
	}

	attempts := h.request(http.MethodGet, "/admin/api/integrations/submissions/"+id+"/attempts", nil, true)
	if attempts.Code != http.StatusOK {
		t.Fatalf("expected submission attempts to succeed, got %d body=%s", attempts.Code, attempts.Body.String())
	}

	opsDetail := h.request(http.MethodGet, "/ops/integrations/deliveries/"+id, nil, true)
	if opsDetail.Code != http.StatusOK {
		t.Fatalf("expected ops submission detail to succeed, got %d body=%s", opsDetail.Code, opsDetail.Body.String())
	}
}

func TestIntegrationAdminListingAndErrorRoutes(t *testing.T) {
	h := newTestHarness(t)

	for _, path := range []string{
		"/admin/api/integrations/systems",
		"/admin/api/integrations/endpoints",
		"/admin/api/integrations/contracts",
		"/admin/api/integrations/mappings",
		"/admin/api/integrations/submissions",
	} {
		rr := h.request(http.MethodGet, path, nil, true)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected %s to succeed, got %d body=%s", path, rr.Code, rr.Body.String())
		}
	}

	missingSubmission := h.request(http.MethodGet, "/admin/api/integrations/submissions/sub-missing", nil, true)
	if missingSubmission.Code != http.StatusNotFound {
		t.Fatalf("expected missing submission detail to fail, got %d body=%s", missingSubmission.Code, missingSubmission.Body.String())
	}

	unknownSubmissionDetail := h.request(http.MethodGet, "/admin/api/integrations/submissions/sub-missing/unknown", nil, true)
	if unknownSubmissionDetail.Code != http.StatusNotFound {
		t.Fatalf("expected unknown submission detail route to fail, got %d body=%s", unknownSubmissionDetail.Code, unknownSubmissionDetail.Body.String())
	}

	invalidSystemUpdate := h.request(http.MethodPut, "/admin/api/integrations/systems/http_bridge/config", []byte("{"), true)
	if invalidSystemUpdate.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid system config body to fail, got %d body=%s", invalidSystemUpdate.Code, invalidSystemUpdate.Body.String())
	}

	invalidEndpointUpdate := h.request(http.MethodPut, "/admin/api/integrations/endpoints/http_bridge_submit/config", []byte("{"), true)
	if invalidEndpointUpdate.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid endpoint config body to fail, got %d body=%s", invalidEndpointUpdate.Code, invalidEndpointUpdate.Body.String())
	}

	notFoundConfig := h.request(http.MethodGet, "/admin/api/integrations/systems/http_bridge/unknown", nil, true)
	if notFoundConfig.Code != http.StatusNotFound {
		t.Fatalf("expected unknown integration system route to fail, got %d body=%s", notFoundConfig.Code, notFoundConfig.Body.String())
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
	if len(bootstrapPayload["menus"].([]any)) == 0 {
		t.Fatal("expected admin menus in bootstrap payload")
	}
	if _, ok := bootstrapPayload["current_user_id"].(string); !ok {
		t.Fatalf("expected current_user_id in admin bootstrap, got %v", bootstrapPayload["current_user_id"])
	}
	if _, ok := bootstrapPayload["user_actions"].([]any); !ok {
		t.Fatalf("expected user_actions in admin bootstrap, got %T", bootstrapPayload["user_actions"])
	}
	if bootstrapPayload["default_path"] != "/admin/modules" {
		t.Fatalf("expected admin default path to target admin modules, got %v", bootstrapPayload["default_path"])
	}
	if bootstrapPayload["ui_access"] != true {
		t.Fatalf("expected admin bootstrap to expose ui access, got %v", bootstrapPayload["ui_access"])
	}
	if uiPath, _ := bootstrapPayload["ui_path"].(string); !strings.HasPrefix(uiPath, "/ui#") {
		t.Fatalf("expected admin bootstrap ui_path to point to user workspace, got %v", bootstrapPayload["ui_path"])
	}

	rr = h.request(http.MethodGet, "/ui/bootstrap", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for ui bootstrap, got %d body=%s", rr.Code, rr.Body.String())
	}
	var uiBootstrap map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &uiBootstrap)
	for _, raw := range uiBootstrap["menus"].([]any) {
		item := raw.(map[string]any)
		if item["key"] == "admin.modules" {
			t.Fatal("expected admin menu to be hidden from user bootstrap")
		}
	}
	if uiBootstrap["admin_access"] != true {
		t.Fatalf("expected user bootstrap to expose admin access for admin principal, got %v", uiBootstrap["admin_access"])
	}
	if adminPath, _ := uiBootstrap["admin_path"].(string); !strings.HasPrefix(adminPath, "/admin#") {
		t.Fatalf("expected user bootstrap admin_path to point to admin workspace, got %v", uiBootstrap["admin_path"])
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

	rr = h.request(http.MethodGet, "/ui/routes/resolve?path=/admin/modules", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected structured route resolution for admin route in user UI, got %d body=%s", rr.Code, rr.Body.String())
	}
	var routePayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &routePayload)
	if routePayload["status"] != "not_found" {
		t.Fatalf("expected admin route in user UI to resolve as not_found, got %+v", routePayload)
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

	rr = h.request(http.MethodGet, "/admin/api/templates/definitions", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected template definitions endpoint, got %d body=%s", rr.Code, rr.Body.String())
	}
	var templatePayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &templatePayload)
	if len(templatePayload["items"].([]any)) == 0 {
		t.Fatal("expected template definitions in admin payload")
	}
}

func TestAdminHierarchySummaryChainAndUpdateErrors(t *testing.T) {
	h := newTestHarness(t)

	summary := h.request(http.MethodGet, "/admin/api/hierarchy/summary", nil, true)
	if summary.Code != http.StatusOK {
		t.Fatalf("expected hierarchy summary to succeed, got %d body=%s", summary.Code, summary.Body.String())
	}

	chain := h.request(http.MethodGet, "/admin/api/hierarchy/chain", nil, true)
	if chain.Code != http.StatusBadRequest {
		t.Fatalf("expected missing hierarchy chain user_id to fail, got %d body=%s", chain.Code, chain.Body.String())
	}

	invalidUpdate := h.request(http.MethodPut, "/admin/api/reporting-lines/line-1", []byte("{"), true)
	if invalidUpdate.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid reporting line update to fail, got %d body=%s", invalidUpdate.Code, invalidUpdate.Body.String())
	}
}

func TestTemplateRoutesManageDraftBindingAndRender(t *testing.T) {
	h := newTestHarness(t)

	draftBody, _ := json.Marshal(map[string]any{
		"body":  `<article><h1>{{ .document.Header.Type }}</h1><p>{{ index .document.Body.Payload "title" }}</p></article>`,
		"style": `article{font-family:Arial}`,
	})
	rr := h.request(http.MethodPut, "/admin/api/templates/documents.generic_request.default/actions/draft", draftBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected draft save to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	version := payload["version"].(map[string]any)
	if version["status"] != "draft" {
		t.Fatalf("expected draft version, got %+v", version)
	}

	bindingBody, _ := json.Marshal(map[string]any{
		"template_key": "documents.generic_request.default",
		"scope_type":   "deployment",
		"target_kind":  "document",
		"target_key":   "generic_request",
		"purpose":      "official",
		"channel":      "print",
		"is_default":   true,
		"is_official":  true,
	})
	rr = h.request(http.MethodPut, "/admin/api/template-bindings", bindingBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected binding save to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	docBody, _ := json.Marshal(map[string]any{"type": "generic_request", "organization_id": "org_default", "location_id": "loc_hq", "payload": map[string]any{"title": "Printed Request"}})
	rr = h.request(http.MethodPost, "/documents", docBody, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected document create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	header := created["header"].(map[string]any)

	renderBody, _ := json.Marshal(map[string]any{
		"target_kind": "document",
		"target_key":  "generic_request",
		"target_id":   header["id"],
		"format":      "html",
		"purpose":     "official",
		"channel":     "print",
	})
	rr = h.request(http.MethodPost, "/outputs/render", renderBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected render to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	output := payload["output"].(map[string]any)
	if !strings.Contains(output["html"].(string), "Printed Request") {
		t.Fatalf("expected rendered html to contain document content, got %s", output["html"])
	}
}

func TestTemplateRenderSupportsVisualPreviewOverrides(t *testing.T) {
	h := newTestHarness(t)

	renderBody, _ := json.Marshal(map[string]any{
		"template_key":  "documents.generic_request.default",
		"target_kind":   "document",
		"target_key":    "generic_request",
		"format":        "html",
		"sample":        true,
		"renderer_kind": "visual",
		"body":          `{"schema_version":"visual-grid/v1","title":"Preview Receipt","settings":{"paper_preset":"receipt-80","density":"compact"},"sections":[{"id":"body","title":"Rows","rows":[{"columns":[{"span":12,"blocks":[{"type":"text","text":"Preview Receipt","font_size":"xl"},{"type":"field","label":"Number","path":"document.header.number"},{"type":"table","rows_path":"document.lines","columns":[{"label":"Label","path":"payload.name"},{"label":"Amount","path":"amount"}]}]}]}]}]}`,
	})
	rr := h.request(http.MethodPost, "/outputs/render", renderBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected preview render to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	output := payload["output"].(map[string]any)
	if !strings.Contains(output["html"].(string), "Preview Receipt") || !strings.Contains(output["html"].(string), "SAMPLE-0001") {
		t.Fatalf("expected preview override html, got %s", output["html"])
	}
}

func TestTemplateResolveAndPDFRender(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/outputs/templates/resolve?target_kind=document&target_key=generic_request&purpose=official&channel=print", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected template resolve to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resolved map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resolved)
	if ok, _ := resolved["resolved"].(bool); !ok {
		t.Fatalf("expected template to resolve, got %+v", resolved)
	}

	renderBody, _ := json.Marshal(map[string]any{
		"template_key": "documents.generic_request.default",
		"target_kind":  "document",
		"target_key":   "generic_request",
		"format":       "pdf",
		"sample":       true,
	})
	rr = h.request(http.MethodPost, "/outputs/render", renderBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected pdf render to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("expected application/pdf, got %q", got)
	}
	if body := rr.Body.Bytes(); len(body) < 16 || !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Fatalf("expected pdf body, got %q", string(body))
	}
}

func TestTemplateAdminPreviewFixturesAndCompare(t *testing.T) {
	h := newTestHarness(t)

	fixtureBody, _ := json.Marshal(map[string]any{
		"name":         "Request Fixture",
		"target_kind":  "document",
		"template_key": "documents.generic_request.default",
		"source_type":  "sample",
		"payload": map[string]any{
			"header": map[string]any{"number": "FIXTURE-100"},
			"body":   map[string]any{"payload": map[string]any{"title": "Fixture Request"}},
			"lines":  []any{},
		},
	})
	rr := h.request(http.MethodPut, "/admin/api/template-fixtures", fixtureBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected fixture save to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	fixture := payload["fixture"].(map[string]any)
	fixtureKey := fixture["fixture_key"].(string)

	draftBody, _ := json.Marshal(map[string]any{
		"body":        `<article><h1>{{ index .document.header "number" }}</h1></article>`,
		"style":       `article{font-family:Arial}`,
		"change_note": "admin preview draft",
	})
	rr = h.request(http.MethodPut, "/admin/api/templates/documents.generic_request.default/actions/draft", draftBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected draft save to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	version := payload["version"].(map[string]any)
	versionNo := int(version["version"].(float64))

	compareURL := "/admin/api/templates/compare?template_key=documents.generic_request.default&left=1&right=" + strconv.Itoa(versionNo)
	rr = h.request(http.MethodGet, compareURL, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected template compare to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	previewBody, _ := json.Marshal(map[string]any{
		"template_key": "documents.generic_request.default",
		"target_kind":  "document",
		"target_key":   "generic_request",
		"draft":        true,
		"fixture_key":  fixtureKey,
	})
	rr = h.request(http.MethodPost, "/admin/api/templates/preview", previewBody, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected admin preview to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	preview := payload["preview"].(map[string]any)
	if preview["data_source"] != "fixture" {
		t.Fatalf("expected fixture preview data source, got %+v", preview)
	}
	outputs := preview["outputs"].([]any)
	if len(outputs) != 3 {
		t.Fatalf("expected html/pdf/print outputs, got %+v", outputs)
	}
	htmlOutput := outputs[0].(map[string]any)
	if !strings.Contains(htmlOutput["html"].(string), "FIXTURE-100") {
		t.Fatalf("expected preview html to contain fixture data, got %+v", htmlOutput)
	}

	debugURL := "/admin/api/templates/binding-debug?template_key=documents.generic_request.default&target_kind=document&target_key=generic_request&mode=draft"
	rr = h.request(http.MethodGet, debugURL, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected binding debug to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdminShellRedirectsUnauthenticatedUsersToUI(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/admin", nil, false)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect for unauthenticated admin shell, got %d body=%s", rr.Code, rr.Body.String())
	}
	if location := rr.Header().Get("Location"); location != "/ui" {
		t.Fatalf("expected unauthenticated admin shell redirect to /ui, got %q", location)
	}
}

func TestAdminShellIncludesNavigationDefaultsUI(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/admin", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected authenticated admin shell, got %d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, expected := range []string{
		`id="navigation-heading"`,
		`id="navigation-settings"`,
		`/users/`,
		`/roles/`,
		`/role-bindings/`,
		`save-user-navigation`,
		`save-role-navigation`,
		`save-binding-priority`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected admin shell to include %q, body=%s", expected, body)
		}
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
	if payload["shell_kind"] != "workspace" {
		t.Fatalf("expected workspace shell, got %+v", payload["shell_kind"])
	}
	if payload["preferred_path"] == nil {
		t.Fatalf("expected preferred_path field in ui bootstrap, got %+v", payload)
	}
	if _, ok := payload["fallback_paths"].(map[string]any); !ok {
		t.Fatalf("expected fallback_paths map in ui bootstrap, got %+v", payload["fallback_paths"])
	}
	if _, ok := payload["capabilities"].(map[string]any); !ok {
		t.Fatalf("expected capabilities in ui bootstrap, got %+v", payload["capabilities"])
	}
	flows, ok := payload["flows"].([]any)
	if !ok || len(flows) == 0 {
		t.Fatalf("expected ui bootstrap to include document flows, got %+v", payload["flows"])
	}

	rr = h.request(http.MethodGet, "/ui/routes/resolve?path=/documents", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected route resolution to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var route map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &route)
	if route["status"] != "ok" {
		t.Fatalf("expected route status ok, got %+v", route)
	}
	if route["render_mode"].(string) != string(module.RenderModeGeneric) {
		t.Fatalf("expected generic render mode, got %+v", route)
	}

	rr = h.request(http.MethodGet, "/ui/views/documents.requests.list", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected view lookup to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/views/documents.requests.detail", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected detail view lookup to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var detailView map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &detailView)
	if detailView["printable"] != true {
		t.Fatalf("expected request detail view to be explicitly printable, got %+v", detailView)
	}

	rr = h.request(http.MethodGet, "/ui/assets/modules/analytics-cockpit.js", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected bundle asset to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "ClinicModuleBundles") {
		t.Fatalf("expected bundle script payload, got %s", rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/routes/resolve?path=/analytics/cockpit", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected analytics custom route resolution, got %d body=%s", rr.Code, rr.Body.String())
	}
	route = map[string]any{}
	_ = json.Unmarshal(rr.Body.Bytes(), &route)
	if route["status"] != "ok" {
		t.Fatalf("expected custom route status ok, got %+v", route)
	}
	customEntry := route["custom_entry"].(map[string]any)
	if _, ok := customEntry["printable"]; ok {
		t.Fatalf("expected analytics custom entry to remain non-printable by default, got %+v", customEntry)
	}
	if _, ok := customEntry["print_target_kind"]; ok {
		t.Fatalf("expected analytics custom entry to omit print target metadata, got %+v", customEntry)
	}

	rr = h.request(http.MethodGet, "/ui/routes/resolve?path=/documents/new", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected flow route resolution, got %d body=%s", rr.Code, rr.Body.String())
	}
	route = map[string]any{}
	_ = json.Unmarshal(rr.Body.Bytes(), &route)
	if route["status"] != "ok" {
		t.Fatalf("expected flow route status ok, got %+v", route)
	}
	if route["render_mode"].(string) != string(module.RenderModeFlow) {
		t.Fatalf("expected flow render mode, got %+v", route)
	}
	if _, ok := route["flow"].(map[string]any); !ok {
		t.Fatalf("expected flow contract in route resolution, got %+v", route)
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

func TestUIWorklistSurfaceBootstrapAndData(t *testing.T) {
	h := newTestHarness(t)
	create := h.request(http.MethodPost, "/documents", mustJSON(t, map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload":         map[string]any{"title": "Queue Target"},
	}), true)
	if create.Code != http.StatusCreated {
		t.Fatalf("expected worklist document creation to succeed, got %d body=%s", create.Code, create.Body.String())
	}
	var record document.Record
	_ = json.Unmarshal(create.Body.Bytes(), &record)

	now := time.Now().UTC()
	if err := h.workflows.ApplyMutation(workflow.Mutation{
		Tasks: []workflow.Task{{
			ID:             "task:worklist",
			WorkflowKey:    "generic_request_flow",
			TargetType:     "document",
			TargetID:       record.Header.ID,
			TaskType:       "review",
			Status:         "open",
			AssigneeUserID: "user_admin",
			CreatedAt:      now,
			DueAt:          now.Add(-time.Hour),
		}},
		Approvals: []workflow.Approval{{
			ID:          "approval:worklist",
			WorkflowKey: "generic_request_flow",
			TargetType:  "document",
			TargetID:    record.Header.ID,
			Status:      "pending",
			StageKey:    "review",
			RequestedBy: "user_admin",
			RequestedAt: now,
			DueAt:       now.Add(2 * time.Hour),
		}},
		History: []workflow.HistoryEvent{{
			ID:          "history:worklist",
			WorkflowKey: "generic_request_flow",
			TargetType:  "document",
			TargetID:    record.Header.ID,
			Action:      "submit",
			FromState:   "draft",
			ToState:     "submitted",
			ActorID:     "user_admin",
			OccurredAt:  now,
		}},
	}); err != nil {
		t.Fatalf("seed worklist workflow data failed: %v", err)
	}

	rr := h.request(http.MethodGet, "/ui/bootstrap?surface=worklist", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected worklist bootstrap to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if payload["surface"] != string(module.UISurfaceWorklist) {
		t.Fatalf("expected worklist surface, got %+v", payload["surface"])
	}
	if len(payload["available_surfaces"].([]any)) == 0 {
		t.Fatalf("expected available surfaces, got %+v", payload)
	}

	rr = h.request(http.MethodGet, "/ui/routes/resolve?path=/worklist&surface=worklist", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected worklist route resolution, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/data/worklist/tasks", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected worklist tasks endpoint, got %d body=%s", rr.Code, rr.Body.String())
	}
	var tasks map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &tasks)
	if len(tasks["items"].([]any)) != 1 {
		t.Fatalf("expected one task item, got %+v", tasks)
	}
	firstTask := tasks["items"].([]any)[0].(map[string]any)
	if firstTask["document_type"] != "generic_request" {
		t.Fatalf("expected enriched document_type, got %+v", firstTask)
	}
	if firstTask["target_title"] != "Queue Target" {
		t.Fatalf("expected enriched target_title, got %+v", firstTask)
	}
	if firstTask["is_mine"] != true {
		t.Fatalf("expected task to be marked as mine, got %+v", firstTask)
	}

	rr = h.request(http.MethodGet, "/ui/data/worklist/tasks?mine=1&due=overdue", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected filtered worklist tasks endpoint, got %d body=%s", rr.Code, rr.Body.String())
	}
	tasks = map[string]any{}
	_ = json.Unmarshal(rr.Body.Bytes(), &tasks)
	if len(tasks["items"].([]any)) != 1 {
		t.Fatalf("expected filtered worklist task to remain visible, got %+v", tasks)
	}

	rr = h.request(http.MethodGet, "/ui/data/worklist/summary", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected worklist summary endpoint, got %d body=%s", rr.Code, rr.Body.String())
	}
	var summary map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &summary)
	taskSummary := summary["tasks"].(map[string]any)
	if taskSummary["mine"].(float64) != 1 {
		t.Fatalf("expected task summary mine count, got %+v", taskSummary)
	}
	if taskSummary["overdue"].(float64) != 1 {
		t.Fatalf("expected task summary overdue count, got %+v", taskSummary)
	}

	rr = h.request(http.MethodGet, "/ui/data/worklist/context?target_type=document&target_id="+url.QueryEscape(record.Header.ID)+"&work_item_kind=task&work_item_id=task%3Aworklist", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected worklist context endpoint, got %d body=%s", rr.Code, rr.Body.String())
	}
	var contextPayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &contextPayload)
	if contextPayload["current_task"] == nil {
		t.Fatalf("expected current_task in worklist context, got %+v", contextPayload)
	}
	if len(contextPayload["history"].([]any)) != 1 {
		t.Fatalf("expected workflow history in worklist context, got %+v", contextPayload)
	}
}

func TestUISelfServiceSurfaceBootstrapAndDiscovery(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/ui/bootstrap?surface=self_service", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected self-service bootstrap to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if payload["surface"] != string(module.UISurfaceSelfService) {
		t.Fatalf("expected self_service surface, got %+v", payload["surface"])
	}
	if len(payload["available_surfaces"].([]any)) == 0 {
		t.Fatalf("expected available surfaces in self-service bootstrap, got %+v", payload)
	}
	apis, ok := payload["self_service_apis"].([]any)
	if !ok || len(apis) == 0 {
		t.Fatalf("expected self-service APIs in bootstrap, got %+v", payload["self_service_apis"])
	}

	rr = h.request(http.MethodGet, "/ui/routes/resolve?path=/self-service/requests&surface=self_service", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected self-service route resolution, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/self-service/apis", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected self-service API discovery, got %d body=%s", rr.Code, rr.Body.String())
	}
	var listPayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &listPayload)
	items, ok := listPayload["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected discovered self-service APIs, got %+v", listPayload)
	}
	foundCreate := false
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["key"] == "documents.self_service.requests.create" {
			foundCreate = true
			if item["flow_key"] != "documents.self_service.requests.intake" {
				t.Fatalf("expected self-service create API to expose flow key, got %+v", item)
			}
		}
	}
	if !foundCreate {
		t.Fatalf("expected self-service create API in discovery payload, got %+v", items)
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
	if rr.Code != http.StatusOK {
		t.Fatalf("expected structured disabled module route response, got %d body=%s", rr.Code, rr.Body.String())
	}
	var routePayload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &routePayload)
	if routePayload["status"] != "not_found" {
		t.Fatalf("expected disabled module route to resolve as not_found, got %+v", routePayload)
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

func TestDocumentFlowCommitCreatesBranchDocuments(t *testing.T) {
	h := newTestHarness(t)

	body, _ := json.Marshal(map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"documents": map[string]any{
			"request": map[string]any{
				"title":        "Main Request",
				"request_kind": "review",
			},
			"review_note": map[string]any{
				"title": "Review Note",
			},
			"review_checklist": map[string]any{
				"title": "Checklist",
			},
		},
	})
	rr := h.request(http.MethodPost, "/document-flows/documents.requests.intake/commit", body, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected flow commit to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if payload["primary_document_id"].(string) == "" {
		t.Fatalf("expected primary document id, got %+v", payload)
	}
	items := payload["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("expected three created documents for review branch, got %+v", payload)
	}

	rr = h.request(http.MethodGet, "/documents", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected document list to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if len(payload["items"].([]any)) < 3 {
		t.Fatalf("expected committed flow documents to be persisted, got %+v", payload)
	}
}

func TestDocumentFlowCommitCreatesAlternateBranchDocuments(t *testing.T) {
	h := newTestHarness(t)

	body, _ := json.Marshal(map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"documents": map[string]any{
			"request": map[string]any{
				"title":        "Main Request",
				"request_kind": "followup",
			},
			"followup_plan": map[string]any{
				"title": "Follow-up Plan",
			},
		},
	})
	rr := h.request(http.MethodPost, "/document-flows/documents.requests.intake/commit", body, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected alternate flow commit to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	items := payload["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected two created documents for followup branch, got %+v", payload)
	}
}

func TestUIDocumentDetailReturnsFlowInstanceForPrimaryAndSecondary(t *testing.T) {
	h := newTestHarness(t)

	body, _ := json.Marshal(map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"documents": map[string]any{
			"request": map[string]any{
				"title":        "Main Request",
				"request_kind": "review",
			},
			"review_note": map[string]any{
				"title": "Review Note",
			},
			"review_checklist": map[string]any{
				"title": "Checklist",
			},
		},
	})
	rr := h.request(http.MethodPost, "/document-flows/documents.requests.intake/commit", body, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected flow commit to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	primaryID := created["primary_document_id"].(string)

	rr = h.request(http.MethodGet, "/ui/data/documents/"+primaryID, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected primary ui detail data, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	flowInstance, ok := payload["flow_instance"].(map[string]any)
	if !ok {
		t.Fatalf("expected flow_instance for primary document, got %+v", payload)
	}
	if flowInstance["primary_document_id"] != primaryID {
		t.Fatalf("expected matching primary document id, got %+v", flowInstance)
	}
	items := flowInstance["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("expected three flow items, got %+v", flowInstance)
	}
	secondaryID := ""
	for _, raw := range items {
		item := raw.(map[string]any)
		definition := item["definition"].(map[string]any)
		if definition["key"] != "review_note" {
			continue
		}
		record := item["record"].(map[string]any)
		header := record["header"].(map[string]any)
		secondaryID = header["id"].(string)
	}
	if secondaryID == "" {
		t.Fatalf("expected secondary review_note id in flow instance, got %+v", flowInstance)
	}

	rr = h.request(http.MethodGet, "/ui/data/documents/"+secondaryID, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected secondary ui detail data, got %d body=%s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	flowInstance, ok = payload["flow_instance"].(map[string]any)
	if !ok {
		t.Fatalf("expected flow_instance for secondary document, got %+v", payload)
	}
	if flowInstance["active_document_key"] != "review_note" {
		t.Fatalf("expected secondary tab to be active, got %+v", flowInstance)
	}
}

func TestUIDocumentListProjectionAndNotFoundBranches(t *testing.T) {
	h := newTestHarness(t)

	for _, body := range []map[string]any{
		{
			"type":            "generic_request",
			"organization_id": "org_default",
			"location_id":     "loc_hq",
			"payload":         map[string]any{"title": "Generic One"},
		},
		{
			"type":            "generic_request",
			"organization_id": "org_default",
			"location_id":     "loc_hq",
			"payload":         map[string]any{"title": "Generic Two"},
		},
	} {
		created := h.request(http.MethodPost, "/documents", mustJSON(t, body), true)
		if created.Code != http.StatusCreated {
			t.Fatalf("expected document create to succeed, got %d body=%s", created.Code, created.Body.String())
		}
	}

	filtered := h.request(http.MethodGet, "/ui/data/documents?type=generic_request&sort=updated_at", nil, true)
	if filtered.Code != http.StatusOK {
		t.Fatalf("expected filtered ui document list to succeed, got %d body=%s", filtered.Code, filtered.Body.String())
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(filtered.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode filtered documents failed: %v", err)
	}
	if len(payload.Items) == 0 {
		t.Fatalf("expected filtered documents, got %s", filtered.Body.String())
	}
	for _, item := range payload.Items {
		header := item["header"].(map[string]any)
		if header["type"] != "generic_request" {
			t.Fatalf("expected only generic_request items, got %+v", payload.Items)
		}
	}

	projections := h.request(http.MethodGet, "/ui/data/projections/documents", nil, true)
	if projections.Code != http.StatusOK {
		t.Fatalf("expected ui document projections to succeed, got %d body=%s", projections.Code, projections.Body.String())
	}

	missing := h.request(http.MethodGet, "/ui/data/documents/", nil, true)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected empty ui document id to fail, got %d body=%s", missing.Code, missing.Body.String())
	}
}

func TestDocumentFlowCommitUpdatesExistingFlowInstance(t *testing.T) {
	h := newTestHarness(t)

	createBody, _ := json.Marshal(map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"documents": map[string]any{
			"request": map[string]any{
				"title":        "Main Request",
				"request_kind": "review",
			},
			"review_note": map[string]any{
				"title": "Review Note",
			},
			"review_checklist": map[string]any{
				"title": "Checklist",
			},
		},
	})
	rr := h.request(http.MethodPost, "/document-flows/documents.requests.intake/commit", createBody, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected flow create to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	primaryID := created["primary_document_id"].(string)

	updateBody, _ := json.Marshal(map[string]any{
		"organization_id":     "org_default",
		"location_id":         "loc_hq",
		"primary_document_id": primaryID,
		"documents": map[string]any{
			"request": map[string]any{
				"title":        "Main Request Updated",
				"request_kind": "review",
			},
			"review_note": map[string]any{
				"title": "Review Note Updated",
			},
			"review_checklist": map[string]any{
				"title": "Checklist Updated",
			},
		},
	})
	rr = h.request(http.MethodPost, "/document-flows/documents.requests.intake/commit", updateBody, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected flow update to succeed, got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = h.request(http.MethodGet, "/ui/data/documents/"+primaryID, nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected primary ui detail data after update, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	record := payload["record"].(map[string]any)
	bodyMap := record["body"].(map[string]any)
	payloadMap := bodyMap["payload"].(map[string]any)
	if payloadMap["title"] != "Main Request Updated" {
		t.Fatalf("expected updated primary payload, got %+v", payloadMap)
	}
	flowInstance := payload["flow_instance"].(map[string]any)
	if len(flowInstance["items"].([]any)) != 3 {
		t.Fatalf("expected updated flow instance to retain three items, got %+v", flowInstance)
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
