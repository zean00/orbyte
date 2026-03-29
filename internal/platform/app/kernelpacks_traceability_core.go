package app

import "orbyte/internal/platform/module"

func traceabilityCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{traceabilityCoreKernelPackManifest()}
}

func traceabilityCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "traceability_core",
		Name:         "Traceability Core",
		NameI18n:     localize("Traceability Core", "Inti Ketertelusuran"),
		Version:      "1.0.0",
		DomainFamily: "business",
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "inventory_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "production_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindOptional},
			{ModuleKey: "delivery_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindOptional},
			{ModuleKey: "returns_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindOptional},
			{ModuleKey: "supplier_returns_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindOptional},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Traceability Console",
			TitleI18n:       localize("Traceability Console", "Konsol Ketertelusuran"),
			Description:     "Trace batches across inbound, production, delivery, and reverse-logistics flows.",
			DescriptionI18n: localize("Trace batches across inbound, production, delivery, and reverse-logistics flows.", "Telusuri batch pada alur inbound, produksi, pengiriman, dan logistik balik."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "operations",
					Title:     "Traceability Operations",
					TitleI18n: localize("Traceability Operations", "Operasi Ketertelusuran"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("batches", "Batches", "Batch", "/ui/inventory/batches", "Open inventory batches and inspect their trace graph.", "Buka batch inventori dan tinjau grafik telusurnya.", "inventory_batch.list"),
						adminConsoleLink("movements", "Movements", "Pergerakan", "/ui/inventory/movements", "Open stock movements feeding the trace chain.", "Buka pergerakan stok yang menjadi rantai telusur.", "document.list"),
					},
				},
			},
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "traceability.read", Action: "read", Resource: "traceability", DisplayName: "Read Traceability", DisplayNameI18n: localize("Read Traceability", "Lihat Ketertelusuran")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "traceability_analyst",
					Name:           "Traceability Analyst",
					NameI18n:       localize("Traceability Analyst", "Analis Ketertelusuran"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"traceability.read", "inventory_batch.list", "inventory_batch.read", "document.list", "document.read"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "traceability.batches", Label: "Traceability", LabelI18n: localize("Traceability", "Ketertelusuran"), ActionKey: "inventory.batches.list", Order: 64, RequiredPermissions: []string{"inventory_batch.list"}},
			},
		},
	}
}
