package app

import (
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/module"
)

func platformCoreKernelPackManifest(httpDefinition config.Definition) module.Manifest {
	return module.Manifest{
		Key:                 "platform.core",
		Name:                "Platform Core",
		NameI18n:            localize("Platform Core", "Inti Platform"),
		Version:             "1.0.0",
		DomainFamily:        "platform",
		OwnedPermissionKeys: []string{"platform.context.read", "module.read", "module.manage", "configuration.read", "configuration.manage", "search.manage", "template.read", "template.manage", "template.publish", "template.bind", "template.render"},
		ConfigDefinitions:   []config.Definition{httpDefinition},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "platform.context.read", Action: "read", Resource: "context", DisplayName: "Read Platform Context", DisplayNameI18n: localize("Read Platform Context", "Lihat Konteks Platform")},
				{Key: "module.read", Action: "read", Resource: "module", DisplayName: "Read Modules", DisplayNameI18n: localize("Read Modules", "Lihat Modul")},
				{Key: "module.manage", Action: "manage", Resource: "module", DisplayName: "Manage Modules", DisplayNameI18n: localize("Manage Modules", "Kelola Modul"), RiskLevel: "high"},
				{Key: "configuration.read", Action: "read", Resource: "configuration", DisplayName: "Read Configuration", DisplayNameI18n: localize("Read Configuration", "Lihat Konfigurasi")},
				{Key: "configuration.manage", Action: "manage", Resource: "configuration", DisplayName: "Manage Configuration", DisplayNameI18n: localize("Manage Configuration", "Kelola Konfigurasi"), RiskLevel: "high"},
				{Key: "search.manage", Action: "manage", Resource: "search", DisplayName: "Manage Search Indexes", DisplayNameI18n: localize("Manage Search Indexes", "Kelola Indeks Pencarian"), RiskLevel: "high"},
				{Key: "template.read", Action: "read", Resource: "template", DisplayName: "Read Templates", DisplayNameI18n: localize("Read Templates", "Lihat Template")},
				{Key: "template.manage", Action: "manage", Resource: "template", DisplayName: "Manage Templates", DisplayNameI18n: localize("Manage Templates", "Kelola Template"), RiskLevel: "high"},
				{Key: "template.publish", Action: "publish", Resource: "template", DisplayName: "Publish Templates", DisplayNameI18n: localize("Publish Templates", "Publikasikan Template"), RiskLevel: "high"},
				{Key: "template.bind", Action: "bind", Resource: "template", DisplayName: "Bind Templates", DisplayNameI18n: localize("Bind Templates", "Atur Binding Template"), RiskLevel: "high"},
				{Key: "template.render", Action: "render", Resource: "template_output", DisplayName: "Render Template Outputs", DisplayNameI18n: localize("Render Template Outputs", "Render Output Template")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{{
				Key:            "platform_operator",
				Name:           "Platform Operator",
				NameI18n:       localize("Platform Operator", "Operator Platform"),
				AllowedScopes:  []string{"deployment"},
				PermissionKeys: []string{"platform.context.read", "module.read", "configuration.read", "template.read", "template.render"},
			}},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{
					Key:                 "admin.modules",
					Label:               "Modules",
					LabelI18n:           localize("Modules", "Modul"),
					ActionKey:           "admin.modules",
					Order:               10,
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"module.read"},
				},
				{
					Key:                 "admin.auth",
					Label:               "Authentication",
					LabelI18n:           localize("Authentication", "Autentikasi"),
					ActionKey:           "admin.auth",
					Order:               20,
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:                 "admin.configuration",
					Label:               "Configuration",
					LabelI18n:           localize("Configuration", "Konfigurasi"),
					ActionKey:           "admin.configuration",
					Order:               30,
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:                 "admin.definitions",
					Label:               "Definitions",
					LabelI18n:           localize("Definitions", "Definisi"),
					ActionKey:           "admin.definitions",
					Order:               40,
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:                 "admin.templates",
					Label:               "Templates",
					LabelI18n:           localize("Templates", "Template"),
					ActionKey:           "admin.templates",
					Order:               45,
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"template.read"},
				},
				{
					Key:                 "admin.workflows",
					Label:               "Workflows",
					LabelI18n:           localize("Workflows", "Workflow"),
					ActionKey:           "admin.workflows",
					Order:               47,
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:                 "admin.security",
					Label:               "Security",
					LabelI18n:           localize("Security", "Keamanan"),
					ActionKey:           "admin.security",
					Order:               50,
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:                 "admin.observability",
					Label:               "Observability",
					LabelI18n:           localize("Observability", "Observabilitas"),
					ActionKey:           "admin.observability",
					Order:               60,
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"module.read"},
				},
			},
			Actions: []module.ActionDefinition{
				{
					Key:                 "admin.modules",
					Label:               "Modules",
					LabelI18n:           localize("Modules", "Modul"),
					Kind:                "navigate",
					RoutePath:           "/admin/modules",
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"module.read"},
				},
				{
					Key:                 "admin.auth",
					Label:               "Authentication",
					LabelI18n:           localize("Authentication", "Autentikasi"),
					Kind:                "navigate",
					RoutePath:           "/admin/auth",
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:                 "admin.configuration",
					Label:               "Configuration",
					LabelI18n:           localize("Configuration", "Konfigurasi"),
					Kind:                "navigate",
					RoutePath:           "/admin/config",
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:                 "admin.definitions",
					Label:               "Definitions",
					LabelI18n:           localize("Definitions", "Definisi"),
					Kind:                "navigate",
					RoutePath:           "/admin/definitions",
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:                 "admin.templates",
					Label:               "Templates",
					LabelI18n:           localize("Templates", "Template"),
					Kind:                "navigate",
					RoutePath:           "/admin/templates",
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"template.read"},
				},
				{
					Key:                 "admin.workflows",
					Label:               "Workflows",
					LabelI18n:           localize("Workflows", "Workflow"),
					Kind:                "navigate",
					RoutePath:           "/admin/workflows",
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:                 "admin.security",
					Label:               "Security",
					LabelI18n:           localize("Security", "Keamanan"),
					Kind:                "navigate",
					RoutePath:           "/admin/security",
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:                 "admin.observability",
					Label:               "Observability",
					LabelI18n:           localize("Observability", "Observabilitas"),
					Kind:                "navigate",
					RoutePath:           "/admin/observability",
					Surface:             module.UISurfaceAdmin,
					RequiredPermissions: []string{"module.read"},
				},
			},
		},
		Observability: module.ObservabilityDefinition{
			Metrics: []module.MetricDefinition{
				{Key: "http.requests.total", Type: "counter", Description: "Total HTTP requests"},
				{Key: "http.request.duration", Type: "timing", Description: "HTTP request duration"},
			},
			LogEvents: []module.LogEventDefinition{{
				Key: "http.request.completed", Category: "http", Severity: "info", RequiredFields: []string{"correlation_id", "method", "path", "status"},
			}},
		},
	}
}

func identityKernelPackManifest(authDefinition config.Definition) module.Manifest {
	return module.Manifest{
		Key:          "identity",
		Name:         "Identity and Access",
		NameI18n:     localize("Identity and Access", "Identitas dan Akses"),
		Version:      "1.0.0",
		DomainFamily: "platform",
		DependencyRequirements: []module.DependencyRequirement{{
			ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired,
		}},
		OwnedPermissionKeys: []string{"identity.manage_sessions", "identity.manage_users"},
		ConfigDefinitions:   []config.Definition{authDefinition},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "identity.manage_sessions", Action: "manage", Resource: "session", DisplayName: "Manage Sessions", DisplayNameI18n: localize("Manage Sessions", "Kelola Sesi"), RiskLevel: "high"},
				{Key: "identity.manage_users", Action: "manage", Resource: "user", DisplayName: "Manage Users", DisplayNameI18n: localize("Manage Users", "Kelola Pengguna"), RiskLevel: "high"},
			},
		},
	}
}
