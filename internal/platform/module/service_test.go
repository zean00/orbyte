package module

import (
	"testing"

	"orbyte/internal/platform/config"
)

func TestRegisterValidatesFrontendContracts(t *testing.T) {
	svc := NewService()

	err := svc.Register(Manifest{
		Key: "documents",
		Frontend: FrontendDefinition{
			Menus: []MenuDefinition{{
				Key: "documents.requests", Label: "Requests", ActionKey: "documents.requests.list",
			}},
			Actions: []ActionDefinition{{
				Key: "documents.requests.list", Label: "Requests", RoutePath: "/documents", ViewKey: "documents.requests.list", RenderMode: RenderModeGeneric,
			}},
			Views: []ViewDefinition{{
				Key: "documents.requests.list", Title: "Requests", Kind: "list",
			}},
		},
	}, "system")
	if err != nil {
		t.Fatalf("register valid manifest failed: %v", err)
	}

	err = svc.Register(Manifest{
		Key: "analytics",
		Frontend: FrontendDefinition{
			Actions: []ActionDefinition{{
				Key: "analytics.cockpit", Label: "Cockpit", RoutePath: "/documents", CustomEntryKey: "analytics.cockpit", RenderMode: RenderModeCustom,
			}},
			CustomEntries: []CustomEntryDefinition{{
				Key: "analytics.cockpit", RoutePath: "/analytics/cockpit", BundleKey: "analytics-cockpit", ComponentExport: "render",
			}},
		},
		Bundles: []BundleDefinition{{Key: "analytics-cockpit", Script: "console.log('x');"}},
	}, "system")
	if err == nil {
		t.Fatal("expected duplicate route to be rejected")
	}
}

func TestResolveRouteSkipsDisabledModules(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{
		Key: "analytics",
		Frontend: FrontendDefinition{
			Actions: []ActionDefinition{{
				Key: "analytics.cockpit", Label: "Cockpit", RoutePath: "/analytics/cockpit", CustomEntryKey: "analytics.cockpit", RenderMode: RenderModeCustom,
			}},
			CustomEntries: []CustomEntryDefinition{{
				Key: "analytics.cockpit", RoutePath: "/analytics/cockpit", BundleKey: "analytics-cockpit", ComponentExport: "render",
			}},
		},
		Bundles: []BundleDefinition{{Key: "analytics-cockpit", Script: "console.log('x');"}},
	}, "system"); err != nil {
		t.Fatalf("register analytics manifest failed: %v", err)
	}

	if _, ok := svc.ResolveRoute("/analytics/cockpit"); !ok {
		t.Fatal("expected enabled route to resolve")
	}
	if _, err := svc.Disable("analytics", "tester"); err != nil {
		t.Fatalf("disable module failed: %v", err)
	}
	if _, ok := svc.ResolveRoute("/analytics/cockpit"); ok {
		t.Fatal("expected disabled module route to be hidden")
	}
}

func TestEnableValidatesDependencyVersionRanges(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{
		Key:     "platform.core",
		Version: "1.0.0",
	}, "system"); err != nil {
		t.Fatalf("register platform failed: %v", err)
	}
	if err := svc.Register(Manifest{
		Key:     "documents",
		Version: "1.1.0",
	}, "system"); err != nil {
		t.Fatalf("register documents failed: %v", err)
	}
	if err := svc.Register(Manifest{
		Key:     "analytics",
		Version: "1.0.0",
		DependencyRequirements: []DependencyRequirement{{
			ModuleKey: "documents", VersionRange: ">=2.0.0,<3.0.0", Kind: DependencyKindRequired,
		}},
	}, "system"); err != nil {
		t.Fatalf("register analytics failed: %v", err)
	}

	if _, err := svc.Enable("analytics", "system"); err == nil {
		t.Fatal("expected incompatible dependency version to block enable")
	}
}

func TestDetailIncludesDependencyDiagnostics(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{
		Key:     "platform.core",
		Version: "1.0.0",
	}, "system"); err != nil {
		t.Fatalf("register platform failed: %v", err)
	}
	if err := svc.Register(Manifest{
		Key:     "analytics",
		Version: "1.0.0",
		DependencyRequirements: []DependencyRequirement{{
			ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: DependencyKindRequired,
		}},
	}, "system"); err != nil {
		t.Fatalf("register analytics failed: %v", err)
	}

	detail, ok := svc.Get("analytics")
	if !ok {
		t.Fatal("expected analytics detail")
	}
	if len(detail.DependencyDiagnostics) != 1 || !detail.DependencyDiagnostics[0].Compatible {
		t.Fatalf("expected compatible dependency diagnostics, got %+v", detail.DependencyDiagnostics)
	}
}

func TestDisableRejectsEnabledDependents(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{Key: "documents", Version: "1.1.0"}, "system"); err != nil {
		t.Fatalf("register documents failed: %v", err)
	}
	if err := svc.Register(Manifest{
		Key:     "analytics",
		Version: "1.0.0",
		DependencyRequirements: []DependencyRequirement{{
			ModuleKey: "documents", VersionRange: ">=1.0.0,<2.0.0", Kind: DependencyKindRequired,
		}},
	}, "system"); err != nil {
		t.Fatalf("register analytics failed: %v", err)
	}
	if _, err := svc.Disable("documents", "system"); err == nil {
		t.Fatal("expected enabled dependents to block disable")
	}
}

func TestRoleTemplatesExposeDeterministicAssignments(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{
		Key: "documents",
		Security: SecurityDefinition{
			Permissions: []PermissionDefinition{{
				Key: "document.read", Action: "read", Resource: "document",
			}},
			RoleTemplates: []RoleTemplateDefinition{{
				Key: "document_clerk", Name: "Document Clerk", PermissionKeys: []string{"document.read"},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register manifest failed: %v", err)
	}
	items := svc.RoleTemplates()
	if len(items) != 1 || items[0].RoleID != "role:documents:document_clerk" {
		t.Fatalf("expected deterministic role template assignment, got %+v", items)
	}
}

func TestCompatibilityReportIncludesLifecycleState(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{Key: "platform.core", Version: "1.0.0"}, "system"); err != nil {
		t.Fatalf("register platform failed: %v", err)
	}
	if err := svc.Register(Manifest{
		Key:     "analytics",
		Version: "1.0.0",
		DependencyRequirements: []DependencyRequirement{{
			ModuleKey: "platform.core", VersionRange: ">=2.0.0,<3.0.0", Kind: DependencyKindRequired,
		}},
	}, "system"); err != nil {
		t.Fatalf("register analytics failed: %v", err)
	}
	items := svc.CompatibilityReport()
	for _, item := range items {
		if item.Manifest.Key == "analytics" && item.LifecycleState != "blocked" {
			t.Fatalf("expected blocked lifecycle state, got %+v", item)
		}
	}
}

func TestServiceAccessorsExposeRegisteredContracts(t *testing.T) {
	svc := NewService()
	manifest := Manifest{
		Key:               "documents",
		ConfigDefinitions: []config.Definition{{Key: "documents.test", ModuleKey: "documents", Category: "general", DisplayName: "Test", AllowedScopes: []string{"deployment"}, DefaultValue: map[string]any{"enabled": true}}},
		Security: SecurityDefinition{
			Permissions: []PermissionDefinition{{Key: "document.read", Action: "read", Resource: "document"}},
		},
		Observability: ObservabilityDefinition{
			Metrics: []MetricDefinition{{Key: "documents.total", Type: "counter"}},
		},
		Frontend: FrontendDefinition{
			Menus:   []MenuDefinition{{Key: "documents.menu", Label: "Documents", ActionKey: "documents.list"}},
			Actions: []ActionDefinition{{Key: "documents.list", Label: "List", RoutePath: "/documents", ViewKey: "documents.view", RenderMode: RenderModeGeneric}},
			Views:   []ViewDefinition{{Key: "documents.view", Title: "Documents", Kind: "list"}},
			CustomEntries: []CustomEntryDefinition{{
				Key: "documents.custom", RoutePath: "/documents/custom", BundleKey: "documents-bundle", ComponentExport: "render",
			}},
		},
		Bundles: []BundleDefinition{{Key: "documents-bundle", Script: "console.log('bundle');"}},
	}
	if err := svc.Register(manifest, "system"); err != nil {
		t.Fatalf("register manifest failed: %v", err)
	}
	if len(svc.Definitions()) != 1 || len(svc.SecurityDefinitions()) != 1 || len(svc.ObservabilityDefinitions()) != 1 {
		t.Fatalf("expected config/security/observability definitions, got %+v %+v %+v", svc.Definitions(), svc.SecurityDefinitions(), svc.ObservabilityDefinitions())
	}
	if len(svc.Menus()) != 1 || len(svc.Actions()) != 1 || len(svc.Views()) != 1 || len(svc.CustomEntries()) != 1 {
		t.Fatalf("expected frontend contracts, got %+v %+v %+v %+v", svc.Menus(), svc.Actions(), svc.Views(), svc.CustomEntries())
	}
	if bundle, ok := svc.Bundle("documents-bundle"); !ok || bundle.Key != "documents-bundle" {
		t.Fatalf("expected bundle lookup, got %+v %v", bundle, ok)
	}
	if _, ok := svc.View("documents.view"); !ok {
		t.Fatal("expected view lookup")
	}
	if !svc.EnabledMap()["documents"] {
		t.Fatalf("expected enabled map for documents, got %+v", svc.EnabledMap())
	}
}

func TestSurfaceAwareFrontendAccessors(t *testing.T) {
	svc := NewService()
	manifest := Manifest{
		Key: "surfaces",
		Frontend: FrontendDefinition{
			Menus: []MenuDefinition{
				{Key: "user.menu", Label: "User", ActionKey: "user.action"},
				{Key: "admin.menu", Label: "Admin", ActionKey: "admin.action", Surface: UISurfaceAdmin},
			},
			Actions: []ActionDefinition{
				{Key: "user.action", Label: "User", RoutePath: "/user", ViewKey: "user.view", RenderMode: RenderModeGeneric},
				{Key: "admin.action", Label: "Admin", RoutePath: "/admin/modules", Surface: UISurfaceAdmin},
			},
			Views: []ViewDefinition{
				{Key: "user.view", Title: "User View", Kind: "list"},
			},
		},
	}
	if err := svc.Register(manifest, "system"); err != nil {
		t.Fatalf("register manifest failed: %v", err)
	}

	if len(svc.MenusForSurface(UISurfaceUser)) != 1 {
		t.Fatalf("expected only user menus, got %+v", svc.MenusForSurface(UISurfaceUser))
	}
	if len(svc.MenusForSurface(UISurfaceAdmin)) != 1 {
		t.Fatalf("expected only admin menus, got %+v", svc.MenusForSurface(UISurfaceAdmin))
	}
	if _, ok := svc.ResolveRouteForSurface("/admin/modules", UISurfaceUser); ok {
		t.Fatal("expected admin route to be hidden from user surface")
	}
	if route, ok := svc.ResolveRouteForSurface("/admin/modules", UISurfaceAdmin); !ok || route.Action.Key != "admin.action" {
		t.Fatalf("expected admin route to resolve for admin surface, got %+v %v", route, ok)
	}
}

func TestRegisterRejectsUnknownRoleTemplatePermissionAndInvalidDependencyRange(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{
		Key: "bad.roles",
		Security: SecurityDefinition{
			RoleTemplates: []RoleTemplateDefinition{{
				Key: "missing_perm", Name: "Missing", PermissionKeys: []string{"perm.missing"},
			}},
		},
	}, "system"); err == nil {
		t.Fatal("expected unknown role template permission to fail")
	}

	if err := svc.Register(Manifest{
		Key:     "bad.dependency",
		Version: "1.0.0",
		DependencyRequirements: []DependencyRequirement{{
			ModuleKey: "core", VersionRange: ">=bad", Kind: DependencyKindRequired,
		}},
	}, "system"); err == nil {
		t.Fatal("expected invalid dependency version range to fail")
	}
}

func TestRegisterValidatesMCPContracts(t *testing.T) {
	svc := NewService()
	err := svc.Register(Manifest{
		Key: "analytics",
		Frontend: FrontendDefinition{
			CustomEntries: []CustomEntryDefinition{{
				Key: "analytics.cockpit", RoutePath: "/analytics/cockpit", BundleKey: "analytics-bundle", ComponentExport: "render",
			}},
		},
		Bundles: []BundleDefinition{{Key: "analytics-bundle", Script: "console.log('bundle');"}},
		MCP: MCPDefinition{
			Resources: []MCPResourceDefinition{{
				Key: "analytics.cockpit.app", Title: "Analytics App", URI: "orbyte://apps/analytics.cockpit",
			}},
			Apps: []MCPAppDefinition{{
				Key: "analytics.cockpit", Title: "Analytics Cockpit", ResourceKey: "analytics.cockpit.app", CustomEntryKey: "analytics.cockpit",
			}},
			Tools: []MCPToolDefinition{{
				Key: "analytics.snapshot.get", Title: "Get Snapshot", Operation: "analytics.snapshot.get", AppKey: "analytics.cockpit",
			}},
		},
	}, "system")
	if err != nil {
		t.Fatalf("register valid mcp manifest failed: %v", err)
	}

	if _, ok := svc.MCPTool("analytics.snapshot.get"); !ok {
		t.Fatal("expected mcp tool lookup")
	}
	if _, ok := svc.MCPResourceByURI("orbyte://apps/analytics.cockpit"); !ok {
		t.Fatal("expected mcp resource lookup by uri")
	}
	if _, ok := svc.MCPApp("analytics.cockpit"); !ok {
		t.Fatal("expected mcp app lookup")
	}
}

func TestRegisterRejectsMCPToolWithUnknownApp(t *testing.T) {
	svc := NewService()
	err := svc.Register(Manifest{
		Key: "analytics",
		MCP: MCPDefinition{
			Tools: []MCPToolDefinition{{
				Key: "analytics.snapshot.get", Title: "Get Snapshot", Operation: "analytics.snapshot.get", AppKey: "missing.app",
			}},
		},
	}, "system")
	if err == nil {
		t.Fatal("expected unknown mcp app reference to fail")
	}
}

func TestSelfServiceAPIAccessorsAndValidation(t *testing.T) {
	svc := NewService()
	manifest := Manifest{
		Key:                "documents",
		OwnedDocumentTypes: []string{"generic_request"},
		Frontend: FrontendDefinition{
			DocumentFlows: []DocumentFlowDefinition{{
				Key:                 "documents.self_service.requests.intake",
				Title:               "Self-Service Request",
				Surface:             UISurfaceSelfService,
				RoutePath:           "/self-service/requests/new",
				PrimaryDocumentType: "generic_request",
				Steps: []DocumentFlowStepDefinition{{
					Key:   "request",
					Title: "Request",
					Documents: []DocumentFlowDocumentDefinition{{
						Key:           "request",
						Title:         "Request",
						DocumentType:  "generic_request",
						PrimaryOutput: true,
					}},
				}},
			}},
		},
		SelfService: SelfServiceDefinition{
			APIs: []SelfServiceAPIDefinition{{
				Key:                 "documents.self_service.requests.create",
				Title:               "Create Self-Service Request",
				Method:              "POST",
				RoutePath:           "/document-flows/documents.self_service.requests.intake/commit",
				HandlerKind:         "flow_commit",
				DocumentType:        "generic_request",
				FlowKey:             "documents.self_service.requests.intake",
				RequiredPermissions: []string{"document.create"},
			}},
		},
	}
	if err := svc.Register(manifest, "system"); err != nil {
		t.Fatalf("register manifest failed: %v", err)
	}

	items := svc.SelfServiceAPIs()
	if len(items) != 1 || items[0].Key != "documents.self_service.requests.create" {
		t.Fatalf("expected self-service api accessor result, got %+v", items)
	}
	item, ok := svc.SelfServiceAPI("documents.self_service.requests.create")
	if !ok || item.FlowKey != "documents.self_service.requests.intake" {
		t.Fatalf("expected self-service api lookup, got %+v %v", item, ok)
	}

	err := svc.Register(Manifest{
		Key: "duplicate",
		SelfService: SelfServiceDefinition{
			APIs: []SelfServiceAPIDefinition{{
				Key:         "documents.self_service.requests.create",
				Title:       "Duplicate",
				Method:      "POST",
				RoutePath:   "/duplicate",
				HandlerKind: "ui_data",
			}},
		},
	}, "system")
	if err == nil {
		t.Fatal("expected duplicate self-service api key to be rejected")
	}
}
