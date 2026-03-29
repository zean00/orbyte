package module

import (
	"testing"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func TestRegisterLocalExtensionRequiresBaseAndAdditiveContracts(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{
		Key:  "ledger.base",
		Role: ModuleRoleBase,
	}, "system"); err != nil {
		t.Fatalf("register base module failed: %v", err)
	}

	err := svc.Register(Manifest{
		Key:  "ledger.local.id",
		Role: ModuleRoleLocalExtension,
		LocalExtension: LocalExtensionDefinition{
			BaseModuleKey: "ledger.base",
			LocalityType:  "country",
			LocalityCode:  "ID",
		},
	}, "system")
	if err == nil {
		t.Fatal("expected missing required dependency to fail")
	}

	err = svc.Register(Manifest{
		Key:  "ledger.local.id",
		Role: ModuleRoleLocalExtension,
		LocalExtension: LocalExtensionDefinition{
			BaseModuleKey: "ledger.base",
			LocalityType:  "country",
			LocalityCode:  "ID",
		},
		DependencyRequirements: []DependencyRequirement{{
			ModuleKey: "ledger.base", Kind: DependencyKindRequired,
		}},
		Documents: []document.Definition{{
			Type: "ledger_entry", DisplayName: "Ledger Entry", SchemaVersion: "v1",
		}},
	}, "system")
	if err == nil {
		t.Fatal("expected canonical document definition to fail")
	}

	err = svc.Register(Manifest{
		Key:  "ledger.local.id",
		Role: ModuleRoleLocalExtension,
		LocalExtension: LocalExtensionDefinition{
			BaseModuleKey: "ledger.base",
			LocalityType:  "country",
			LocalityCode:  "ID",
			LocalityLabel: "Indonesia",
		},
		DependencyRequirements: []DependencyRequirement{{
			ModuleKey: "ledger.base", Kind: DependencyKindRequired,
		}},
		DocumentExtensions: []DocumentExtension{{
			DocumentType: "ledger_entry", SchemaVersion: "v1", DisplayName: "Localized Ledger Entry",
		}},
		ReferenceTypes: []reference.TypeDefinition{{
			Key: "tax_code", DisplayName: "Tax Code",
		}},
		ReferenceRecords: []reference.Record{{
			TypeKey: "tax_code", Key: "vat11", DisplayName: "VAT 11%",
		}},
		Workflows: []workflow.Definition{{
			Key: "ledger.local.id.review", States: []string{"draft"},
		}},
		OwnedWorkflowKeys: []string{"ledger.local.id.review"},
		Frontend: FrontendDefinition{
			Views: []ViewDefinition{{Key: "ledger.local.id.report", Title: "Localized Report", Kind: "list"}},
		},
	}, "system")
	if err != nil {
		t.Fatalf("register valid local extension failed: %v", err)
	}
}

func TestLocalExtensionActivationAndScopedResolution(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{Key: "ledger.base", Role: ModuleRoleBase}, "system"); err != nil {
		t.Fatalf("register base failed: %v", err)
	}
	for _, item := range []Manifest{
		{
			Key:  "ledger.local.default",
			Role: ModuleRoleLocalExtension,
			LocalExtension: LocalExtensionDefinition{
				BaseModuleKey: "ledger.base",
				LocalityType:  "country",
				LocalityCode:  "DEFAULT",
			},
			DependencyRequirements: []DependencyRequirement{{ModuleKey: "ledger.base", Kind: DependencyKindRequired}},
		},
		{
			Key:  "ledger.local.id",
			Role: ModuleRoleLocalExtension,
			LocalExtension: LocalExtensionDefinition{
				BaseModuleKey: "ledger.base",
				LocalityType:  "country",
				LocalityCode:  "ID",
			},
			DependencyRequirements: []DependencyRequirement{{ModuleKey: "ledger.base", Kind: DependencyKindRequired}},
		},
	} {
		if err := svc.Register(item, "system"); err != nil {
			t.Fatalf("register local extension failed: %v", err)
		}
	}

	if _, err := svc.ActivateLocalExtension("ledger.base", "ledger.local.default", "deployment", "", "system"); err != nil {
		t.Fatalf("activate deployment local extension failed: %v", err)
	}
	if _, err := svc.ActivateLocalExtension("ledger.base", "ledger.local.id", "organization", "org_default", "system"); err != nil {
		t.Fatalf("activate org local extension failed: %v", err)
	}

	resolved, ok := svc.ResolveActiveLocalExtension("ledger.base", "org_default", "", "")
	if !ok || resolved.ExtensionModuleKey != "ledger.local.id" || resolved.Scope != "organization" {
		t.Fatalf("expected organization-scoped extension, got %+v %v", resolved, ok)
	}
	resolved, ok = svc.ResolveActiveLocalExtension("ledger.base", "org_other", "", "")
	if !ok || resolved.ExtensionModuleKey != "ledger.local.default" || resolved.Scope != "deployment" {
		t.Fatalf("expected deployment-scoped fallback, got %+v %v", resolved, ok)
	}

	detail, ok := svc.GetForScope("ledger.local.id", "org_default", "", "")
	if !ok || detail.LocalExtensionState == nil || !detail.LocalExtensionState.Active {
		t.Fatalf("expected active local extension state, got %+v %v", detail, ok)
	}
}

func TestDisableRejectsActiveLocalExtensionBinding(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{Key: "ledger.base", Role: ModuleRoleBase}, "system"); err != nil {
		t.Fatalf("register base failed: %v", err)
	}
	if err := svc.Register(Manifest{
		Key:  "ledger.local.id",
		Role: ModuleRoleLocalExtension,
		LocalExtension: LocalExtensionDefinition{
			BaseModuleKey: "ledger.base",
			LocalityType:  "country",
			LocalityCode:  "ID",
		},
		DependencyRequirements: []DependencyRequirement{{ModuleKey: "ledger.base", Kind: DependencyKindRequired}},
	}, "system"); err != nil {
		t.Fatalf("register local extension failed: %v", err)
	}
	if _, err := svc.ActivateLocalExtension("ledger.base", "ledger.local.id", "deployment", "", "system"); err != nil {
		t.Fatalf("activate local extension failed: %v", err)
	}
	if _, err := svc.Disable("ledger.local.id", "system"); err == nil {
		t.Fatal("expected active local extension binding to block disable")
	}
}

func TestRegisterAllowsLocalExtensionBeforeBaseRegistration(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{
		Key:  "ledger.local.id",
		Role: ModuleRoleLocalExtension,
		LocalExtension: LocalExtensionDefinition{
			BaseModuleKey: "ledger.base",
			LocalityType:  "country",
			LocalityCode:  "ID",
		},
		DependencyRequirements: []DependencyRequirement{{ModuleKey: "ledger.base", Kind: DependencyKindRequired}},
	}, "system"); err != nil {
		t.Fatalf("expected local extension registration to tolerate out-of-order base, got %v", err)
	}
	if err := svc.Register(Manifest{
		Key:  "ledger.base",
		Role: ModuleRoleBase,
	}, "system"); err != nil {
		t.Fatalf("register base module failed: %v", err)
	}
}

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

func TestRegisterRejectsMissingKernelCapability(t *testing.T) {
	svc := NewService()
	svc.SetKernelCapabilities(map[string]bool{CapabilityGenericUI: true})
	err := svc.Register(Manifest{
		Key:                  "search_mod",
		Version:              "1.0.0",
		KernelVersionRange:   ">=1.0.0,<2.0.0",
		RequiredCapabilities: []string{CapabilitySearchRuntime},
	}, "system")
	if err == nil {
		t.Fatal("expected missing kernel capability to block register")
	}
}

func TestCompatibilityReportIncludesKernelDiagnostics(t *testing.T) {
	svc := NewService()
	svc.SetKernelCapabilities(map[string]bool{CapabilityGenericUI: true})
	if err := svc.Register(Manifest{
		Key:                "platform.core",
		Version:            "1.0.0",
		KernelVersionRange: ">=1.0.0,<2.0.0",
	}, "system"); err != nil {
		t.Fatalf("register platform failed: %v", err)
	}
	svc.manifests["compat_mod"] = Manifest{
		Key:                  "compat_mod",
		Version:              "1.0.0",
		KernelVersionRange:   ">=2.0.0,<3.0.0",
		RequiredCapabilities: []string{CapabilitySearchRuntime},
	}
	_ = svc.repo.Save(InstalledModule{Key: "compat_mod", Enabled: true})
	items := svc.CompatibilityReport()
	for _, item := range items {
		if item.Manifest.Key == "compat_mod" {
			if len(item.KernelDiagnostics) == 0 || item.LifecycleState != "blocked" {
				t.Fatalf("expected kernel diagnostics to block module, got %+v", item)
			}
			return
		}
	}
	t.Fatal("expected compat_mod detail")
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

func TestServiceExposesDocumentFlowTemplateOfflineAndMCPContracts(t *testing.T) {
	svc := NewService()
	manifest := Manifest{
		Key: "platform.contracts",
		Documents: []document.Definition{{
			Type: "generic_request", DisplayName: "Generic Request", SchemaVersion: "v1",
		}},
		Frontend: FrontendDefinition{
			CustomEntries: []CustomEntryDefinition{{
				Key: "analytics.studio.entry", RoutePath: "/analytics/studio", BundleKey: "analytics-bundle", ComponentExport: "render",
			}},
			DocumentFlows: []DocumentFlowDefinition{
				{
					Key: "intake.user", Title: "Intake User", RoutePath: "/flows/intake", PrimaryDocumentType: "generic_request",
					Steps: []DocumentFlowStepDefinition{{
						Key: "capture", Title: "Capture",
						Documents: []DocumentFlowDocumentDefinition{{
							Key: "primary", Title: "Primary Request", DocumentType: "generic_request", PrimaryOutput: true,
						}},
					}},
				},
				{
					Key: "intake.admin", Title: "Intake Admin", Surface: UISurfaceAdmin, RoutePath: "/admin/flows/intake", PrimaryDocumentType: "generic_request",
					Steps: []DocumentFlowStepDefinition{{
						Key: "capture", Title: "Capture",
						Documents: []DocumentFlowDocumentDefinition{{
							Key: "primary", Title: "Primary Request", DocumentType: "generic_request", PrimaryOutput: true,
						}},
					}},
				},
			},
		},
		Templates: []TemplateDefinition{{
			Key:          "documents.generic_request.official",
			Title:        "Official Request",
			TargetKind:   "document",
			TargetKey:    "generic_request",
			RendererKind: "html",
			DefaultBody:  "<p>request</p>",
		}},
		MCP: MCPDefinition{
			Tools:     []MCPToolDefinition{{Key: "analytics.query.execute", Title: "Execute Query", Operation: "analytics.query.execute"}},
			Resources: []MCPResourceDefinition{{Key: "analytics.snapshot", Title: "Analytics Snapshot", URI: "orbyte://apps/analytics.studio", Provider: "mcp.app", AppKey: "analytics.studio"}},
			Apps:      []MCPAppDefinition{{Key: "analytics.studio", Title: "Analytics Studio", ResourceKey: "analytics.snapshot", CustomEntryKey: "analytics.studio.entry"}},
		},
		Observability: ObservabilityDefinition{
			Projections: []ProjectionDefinition{{Key: "document_summary"}},
		},
		ReferenceTypes: []reference.TypeDefinition{{Key: "country", DisplayName: "Country"}},
		SearchIndexes: []search.IndexDefinition{{
			Key:           "documents.summary",
			Title:         "Document Summary",
			SourceKind:    "projection",
			ProjectionKey: "document_summary",
			Modes:         []string{"keyword"},
			Fields:        []search.IndexFieldDefinition{{Key: "status", Path: "status", Type: "string"}},
		}},
		Offline: OfflineDefinition{
			References:  []OfflineReferenceDefinition{{TypeKey: "country", Title: "Countries"}},
			Projections: []OfflineProjectionDefinition{{IndexKey: "documents.summary", Title: "Document Summary"}},
		},
		Bundles: []BundleDefinition{{Key: "analytics-bundle", Script: "console.log('analytics')"}},
	}
	if err := svc.Register(manifest, "system"); err != nil {
		t.Fatalf("register manifest failed: %v", err)
	}

	if len(svc.DocumentFlows()) != 2 {
		t.Fatalf("expected both document flows, got %+v", svc.DocumentFlows())
	}
	if len(svc.DocumentFlowsForSurface(UISurfaceUser)) != 1 || len(svc.DocumentFlowsForSurface(UISurfaceAdmin)) != 1 {
		t.Fatalf("unexpected surface-filtered flows: user=%+v admin=%+v", svc.DocumentFlowsForSurface(UISurfaceUser), svc.DocumentFlowsForSurface(UISurfaceAdmin))
	}
	if flow, ok := svc.DocumentFlow("intake.user"); !ok || flow.Key != "intake.user" {
		t.Fatalf("expected document flow lookup, got %+v %v", flow, ok)
	}
	if flow, ok := svc.DocumentFlowForSurface("intake.admin", UISurfaceAdmin); !ok || flow.Key != "intake.admin" {
		t.Fatalf("expected surface-scoped document flow lookup, got %+v %v", flow, ok)
	}
	if _, ok := svc.DocumentFlowForSurface("intake.admin", UISurfaceUser); ok {
		t.Fatal("expected admin flow to be hidden from user surface")
	}

	if len(svc.Templates()) != 1 {
		t.Fatalf("expected template list, got %+v", svc.Templates())
	}
	if tpl, ok := svc.Template("documents.generic_request.official"); !ok || tpl.Key != "documents.generic_request.official" {
		t.Fatalf("expected template lookup, got %+v %v", tpl, ok)
	}

	if len(svc.MCPTools()) != 1 || len(svc.MCPResources()) != 1 || len(svc.MCPApps()) != 1 {
		t.Fatalf("expected mcp definitions, got %+v %+v %+v", svc.MCPTools(), svc.MCPResources(), svc.MCPApps())
	}
	if tool, ok := svc.MCPTool("analytics.query.execute"); !ok || tool.Key != "analytics.query.execute" {
		t.Fatalf("expected mcp tool lookup, got %+v %v", tool, ok)
	}
	if resource, ok := svc.MCPResourceByKey("analytics.snapshot"); !ok || resource.URI != "orbyte://apps/analytics.studio" {
		t.Fatalf("expected mcp resource-by-key lookup, got %+v %v", resource, ok)
	}
	if _, ok := svc.MCPResourceByURI("orbyte://apps/analytics.studio"); !ok {
		t.Fatal("expected mcp resource-by-uri lookup")
	}
	if app, ok := svc.MCPApp("analytics.studio"); !ok || app.ResourceKey != "analytics.snapshot" {
		t.Fatalf("expected mcp app lookup, got %+v %v", app, ok)
	}

	if len(svc.OfflineReferences()) != 1 || len(svc.OfflineProjections()) != 1 {
		t.Fatalf("expected offline definitions, got %+v %+v", svc.OfflineReferences(), svc.OfflineProjections())
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

func TestRegisterAllowsCrossManifestRolePermissionsRegardlessOfManifestSortOrder(t *testing.T) {
	svc := NewService()
	if err := svc.Register(Manifest{
		Key: "z.documents",
		Security: SecurityDefinition{
			Permissions: []PermissionDefinition{
				{Key: "document.list", Action: "list", Resource: "document"},
			},
		},
	}, "system"); err != nil {
		t.Fatalf("register provider manifest: %v", err)
	}
	if err := svc.Register(Manifest{
		Key: "a.delivery",
		Security: SecurityDefinition{
			RoleTemplates: []RoleTemplateDefinition{{
				Key: "delivery_operator", Name: "Delivery Operator", PermissionKeys: []string{"document.list"},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register consumer manifest: %v", err)
	}
	if err := svc.Register(Manifest{Key: "m.other"}, "system"); err != nil {
		t.Fatalf("register unrelated manifest should not re-fail due to sorted lint order: %v", err)
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
				Key: "analytics.cockpit.app", Title: "Analytics App", URI: "orbyte://apps/analytics.cockpit", Provider: "mcp.app", AppKey: "analytics.cockpit",
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

func TestRegisterRejectsMCPResourceWithoutProvider(t *testing.T) {
	svc := NewService()
	err := svc.Register(Manifest{
		Key: "analytics",
		MCP: MCPDefinition{
			Resources: []MCPResourceDefinition{{
				Key: "analytics.snapshot.current", Title: "Snapshot", URI: "orbyte://analytics/snapshot/current",
			}},
		},
	}, "system")
	if err == nil {
		t.Fatal("expected missing mcp resource provider to fail")
	}
}

func TestRegisterRejectsDuplicateMCPResourceURIInManifest(t *testing.T) {
	svc := NewService()
	err := svc.Register(Manifest{
		Key: "analytics",
		MCP: MCPDefinition{
			Resources: []MCPResourceDefinition{
				{Key: "analytics.snapshot.current", Title: "Snapshot A", URI: "orbyte://analytics/snapshot/current", Provider: "analytics.snapshot.current"},
				{Key: "analytics.snapshot.current.copy", Title: "Snapshot B", URI: "orbyte://analytics/snapshot/current", Provider: "analytics.snapshot.current"},
			},
		},
	}, "system")
	if err == nil {
		t.Fatal("expected duplicate mcp resource uri in manifest to fail")
	}
}

func TestRegisterRejectsMCPAppResourceWithoutAppProvider(t *testing.T) {
	svc := NewService()
	err := svc.Register(Manifest{
		Key: "analytics",
		MCP: MCPDefinition{
			Resources: []MCPResourceDefinition{{
				Key: "analytics.snapshot.current", Title: "Snapshot", URI: "orbyte://analytics/snapshot/current", Provider: "analytics.snapshot.current", AppKey: "analytics.cockpit",
			}},
		},
	}, "system")
	if err == nil {
		t.Fatal("expected app_key with non-mcp.app provider to fail")
	}
}

func TestSelfServiceAPIAccessorsAndValidation(t *testing.T) {
	svc := NewService()
	manifest := Manifest{
		Key:                "documents",
		OwnedDocumentTypes: []string{"generic_request"},
		Documents: []document.Definition{{
			Type:          "generic_request",
			DisplayName:   "Generic Request",
			SchemaVersion: "v1",
		}},
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
