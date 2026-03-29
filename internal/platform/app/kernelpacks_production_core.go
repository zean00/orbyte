package app

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func productionCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{productionCoreKernelPackManifest()}
}

func productionCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "production_core",
		Name:         "Production Core",
		NameI18n:     localize("Production Core", "Inti Produksi"),
		Version:      "1.0.0",
		DomainFamily: "business",
		OwnedDocumentTypes: []string{
			"production_order",
			"production_issue",
			"production_output",
		},
		OwnedWorkflowKeys: []string{
			"production_order_flow",
			"production_issue_flow",
			"production_output_flow",
		},
		OwnedTemplateKeys: []string{
			"production.production_order.print.default",
			"production.production_issue.print.default",
			"production.production_output.print.default",
		},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "commercial_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "inventory_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Production Console",
			TitleI18n:       localize("Production Console", "Konsol Produksi"),
			Description:     "Recipe, production, issue, and finished-output operations.",
			DescriptionI18n: localize("Recipe, production, issue, and finished-output operations.", "Operasi resep, produksi, issue, dan output barang jadi."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "setup",
					Title:     "Production Setup",
					TitleI18n: localize("Production Setup", "Pengaturan Produksi"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("boms", "BOMs", "BOM", "/ui/production/boms", "Manage production BOMs.", "Kelola BOM produksi.", "production_bom.list"),
						adminConsoleLink("bom_versions", "BOM Versions", "Versi BOM", "/ui/production/bom-versions", "Manage BOM versions.", "Kelola versi BOM.", "production_bom_version.list"),
						adminConsoleLink("work_centers", "Work Centers", "Pusat Kerja", "/ui/production/work-centers", "Manage production work centers.", "Kelola pusat kerja produksi.", "production_work_center.list"),
						adminConsoleLink("catalog", "Catalog", "Katalog", "/ui/commercial/catalog", "Open finished items and components.", "Buka item jadi dan komponen.", "item.list"),
						adminConsoleLink("warehouses", "Warehouses", "Gudang", "/ui/inventory/warehouses", "Open production warehouses.", "Buka gudang produksi.", "warehouse.list"),
					},
				},
				{
					Key:       "operations",
					Title:     "Production Operations",
					TitleI18n: localize("Production Operations", "Operasi Produksi"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("orders", "Production Orders", "Order Produksi", "/ui/production/orders", "Open production orders.", "Buka order produksi.", "document.list"),
						adminConsoleLink("issues", "Production Issues", "Issue Produksi", "/ui/production/issues", "Open production material issues.", "Buka issue bahan produksi.", "document.list"),
						adminConsoleLink("outputs", "Production Outputs", "Output Produksi", "/ui/production/outputs", "Open finished outputs.", "Buka output barang jadi.", "document.list"),
					},
				},
				{
					Key:       "workflows",
					Title:     "Production Workflows",
					TitleI18n: localize("Production Workflows", "Workflow Produksi"),
					Kind:      module.AdminConsoleSectionWorkflowLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("production_order_flow", "Production Order Workflow", "Workflow Order Produksi", "/admin/workflows/designer?key=production_order_flow", "Review production order lifecycle.", "Tinjau siklus hidup order produksi.", "workflow.read"),
						adminConsoleLink("production_issue_flow", "Production Issue Workflow", "Workflow Issue Produksi", "/admin/workflows/designer?key=production_issue_flow", "Review production issue lifecycle.", "Tinjau siklus hidup issue produksi.", "workflow.read"),
						adminConsoleLink("production_output_flow", "Production Output Workflow", "Workflow Output Produksi", "/admin/workflows/designer?key=production_output_flow", "Review production output lifecycle.", "Tinjau siklus hidup output produksi.", "workflow.read"),
					},
				},
			},
		},
		Models: []model.Definition{
			productionModelDefinition("production_bom", "Production BOM", "production_bom", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Type: "string", Required: true},
				{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Type: "string"},
				{Key: "default_version_code", Label: "Default Version", LabelI18n: localize("Default Version", "Versi Default"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			productionModelDefinition("production_bom_version", "Production BOM Version", "production_bom_version", []model.FieldDefinition{
				{Key: "bom_id", Label: "BOM", LabelI18n: localize("BOM", "BOM"), Type: "string", Required: true},
				{Key: "bom_code", Label: "BOM Code", LabelI18n: localize("BOM Code", "Kode BOM"), Type: "string"},
				{Key: "version_code", Label: "Version", LabelI18n: localize("Version", "Versi"), Type: "string", Required: true},
				{Key: "yield_quantity", Label: "Yield Quantity", LabelI18n: localize("Yield Quantity", "Jumlah Hasil"), Type: "number"},
				{Key: "is_active", Label: "Active", LabelI18n: localize("Active", "Aktif"), Type: "bool"},
				{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Type: "object"},
				{Key: "stages", Label: "Stages", LabelI18n: localize("Stages", "Tahap"), Type: "object"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			productionModelDefinition("production_work_center", "Production Work Center", "production_work_center", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
		},
		Documents: []document.Definition{
			productionDocumentDefinition("production_order", "Production Order", "production_order_flow", "production_order_number"),
			productionDocumentDefinition("production_issue", "Production Issue", "production_issue_flow", "production_issue_number"),
			productionDocumentDefinition("production_output", "Production Output", "production_output_flow", "production_output_number"),
		},
		Workflows: []workflow.Definition{
			{
				Key:    "production_order_flow",
				States: []string{"draft", "submitted", "approved", "in_progress", "completed", "rejected", "cancelled"},
				Actions: []workflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
					{Action: "approve", FromState: "submitted", ToState: "approved", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "approved", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
			{
				Key:    "production_issue_flow",
				States: []string{"draft", "submitted", "issued", "rejected", "cancelled"},
				Actions: []workflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
					{Action: "approve", FromState: "submitted", ToState: "issued", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "issued", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
			{
				Key:    "production_output_flow",
				States: []string{"draft", "submitted", "posted", "rejected", "cancelled"},
				Actions: []workflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
					{Action: "approve", FromState: "submitted", ToState: "posted", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "posted", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
		},
		SearchIndexes: []search.IndexDefinition{
			procurementModelSearchIndex("production.boms.search", "Production BOM Search", "production_bom", "production.boms.list", []string{"code", "name", "finished_item_code", "status"}),
			procurementModelSearchIndex("production.bom_versions.search", "Production BOM Version Search", "production_bom_version", "production.bom_versions.list", []string{"bom_code", "version_code", "status"}),
			commercialDocumentSearchIndex("production.orders.search", "Production Order Search", "production_order", "production.orders.list", []search.IndexFieldDefinition{
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "finished_item_code", Path: "body.payload.finished_item_code", Type: "string", Searchable: true},
				{Key: "warehouse_code", Path: "body.payload.warehouse_code", Type: "string", Searchable: true},
				{Key: "status_summary", Path: "body.payload.status_summary", Type: "string", Facet: true},
			}),
			commercialDocumentSearchIndex("production.issues.search", "Production Issue Search", "production_issue", "production.issues.list", []search.IndexFieldDefinition{
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "finished_item_code", Path: "body.payload.finished_item_code", Type: "string", Searchable: true},
				{Key: "warehouse_code", Path: "body.payload.warehouse_code", Type: "string", Searchable: true},
			}),
			commercialDocumentSearchIndex("production.outputs.search", "Production Output Search", "production_output", "production.outputs.list", []search.IndexFieldDefinition{
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "finished_item_code", Path: "body.payload.finished_item_code", Type: "string", Searchable: true},
				{Key: "production_lot_code", Path: "body.payload.production_lot_code", Type: "string", Searchable: true},
				{Key: "warehouse_code", Path: "body.payload.warehouse_code", Type: "string", Searchable: true},
			}),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "production_bom.create", Action: "create", Resource: "production_bom", DisplayName: "Create BOMs", DisplayNameI18n: localize("Create BOMs", "Buat BOM")},
				{Key: "production_bom.list", Action: "list", Resource: "production_bom", DisplayName: "List BOMs", DisplayNameI18n: localize("List BOMs", "Daftar BOM")},
				{Key: "production_bom.read", Action: "read", Resource: "production_bom", DisplayName: "Read BOMs", DisplayNameI18n: localize("Read BOMs", "Lihat BOM")},
				{Key: "production_bom.update", Action: "update", Resource: "production_bom", DisplayName: "Update BOMs", DisplayNameI18n: localize("Update BOMs", "Perbarui BOM")},
				{Key: "production_bom_version.create", Action: "create", Resource: "production_bom_version", DisplayName: "Create BOM Versions", DisplayNameI18n: localize("Create BOM Versions", "Buat Versi BOM")},
				{Key: "production_bom_version.list", Action: "list", Resource: "production_bom_version", DisplayName: "List BOM Versions", DisplayNameI18n: localize("List BOM Versions", "Daftar Versi BOM")},
				{Key: "production_bom_version.read", Action: "read", Resource: "production_bom_version", DisplayName: "Read BOM Versions", DisplayNameI18n: localize("Read BOM Versions", "Lihat Versi BOM")},
				{Key: "production_bom_version.update", Action: "update", Resource: "production_bom_version", DisplayName: "Update BOM Versions", DisplayNameI18n: localize("Update BOM Versions", "Perbarui Versi BOM")},
				{Key: "production_work_center.create", Action: "create", Resource: "production_work_center", DisplayName: "Create Work Centers", DisplayNameI18n: localize("Create Work Centers", "Buat Pusat Kerja")},
				{Key: "production_work_center.list", Action: "list", Resource: "production_work_center", DisplayName: "List Work Centers", DisplayNameI18n: localize("List Work Centers", "Daftar Pusat Kerja")},
				{Key: "production_work_center.read", Action: "read", Resource: "production_work_center", DisplayName: "Read Work Centers", DisplayNameI18n: localize("Read Work Centers", "Lihat Pusat Kerja")},
				{Key: "production_work_center.update", Action: "update", Resource: "production_work_center", DisplayName: "Update Work Centers", DisplayNameI18n: localize("Update Work Centers", "Perbarui Pusat Kerja")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "production_planner",
					Name:           "Production Planner",
					NameI18n:       localize("Production Planner", "Perencana Produksi"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"production_bom.create", "production_bom.list", "production_bom.read", "production_bom.update", "production_bom_version.create", "production_bom_version.list", "production_bom_version.read", "production_bom_version.update", "production_work_center.create", "production_work_center.list", "production_work_center.read", "production_work_center.update", "document.create", "document.list", "document.read", "document.update_draft", "document.submit", "document.approve", "document.reject", "document.reopen", "document.cancel"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "production.boms", Label: "BOMs", LabelI18n: localize("BOMs", "BOM"), ActionKey: "production.boms.list", Order: 59, RequiredPermissions: []string{"production_bom.list"}},
				{Key: "production.bom_versions", Label: "BOM Versions", LabelI18n: localize("BOM Versions", "Versi BOM"), ActionKey: "production.bom_versions.list", Order: 60, RequiredPermissions: []string{"production_bom_version.list"}},
				{Key: "production.work_centers", Label: "Work Centers", LabelI18n: localize("Work Centers", "Pusat Kerja"), ActionKey: "production.work_centers.list", Order: 61, RequiredPermissions: []string{"production_work_center.list"}},
				{Key: "production.orders", Label: "Production Orders", LabelI18n: localize("Production Orders", "Order Produksi"), ActionKey: "production.orders.list", Order: 62, RequiredPermissions: []string{"document.list"}},
				{Key: "production.issues", Label: "Production Issues", LabelI18n: localize("Production Issues", "Issue Produksi"), ActionKey: "production.issues.list", Order: 63, RequiredPermissions: []string{"document.list"}},
				{Key: "production.outputs", Label: "Production Outputs", LabelI18n: localize("Production Outputs", "Output Produksi"), ActionKey: "production.outputs.list", Order: 64, RequiredPermissions: []string{"document.list"}},
			},
			Actions: productionFrontendActions(),
			Views:   productionFrontendViews(),
		},
		Templates: []module.TemplateDefinition{
			commercialTemplateDefinition("production.production_order.print.default", "Production Order Print", "production_order", "Production Order", []string{"finished_item_name", "planned_quantity", "warehouse_code", "lines"}),
			commercialTemplateDefinition("production.production_issue.print.default", "Production Issue Print", "production_issue", "Production Issue", []string{"finished_item_name", "issue_date", "lines"}),
			commercialTemplateDefinition("production.production_output.print.default", "Production Output Print", "production_output", "Production Output", []string{"finished_item_name", "output_quantity", "production_lot_code", "warehouse_code"}),
		},
	}
}

func productionModelDefinition(key, singular, permissionPrefix string, fields []model.FieldDefinition) model.Definition {
	defaultSort := "code"
	hasField := func(fieldKey string) bool {
		for _, field := range fields {
			if field.Key == fieldKey {
				return true
			}
		}
		return false
	}
	switch {
	case hasField("code"):
		defaultSort = "code"
	case hasField("version_code"):
		defaultSort = "version_code"
	case len(fields) > 0:
		defaultSort = fields[0].Key
	}
	return model.Definition{
		Key:                 key,
		DisplayName:         singular,
		DisplayNameI18n:     localize(singular, singular),
		OwnerModuleKey:      "production_core",
		Version:             "v1",
		CreatePermissionKey: permissionPrefix + ".create",
		ListPermissionKey:   permissionPrefix + ".list",
		ReadPermissionKey:   permissionPrefix + ".read",
		UpdatePermissionKey: permissionPrefix + ".update",
		DefaultSort:         defaultSort,
		Fields:              fields,
	}
}

func productionDocumentDefinition(documentType, displayName, workflowKey, numberingKey string) document.Definition {
	return document.Definition{
		Type:                   documentType,
		DisplayName:            displayName,
		SchemaVersion:          "v1",
		WorkflowKey:            workflowKey,
		NumberingKey:           numberingKey,
		OwnerModuleKey:         "production_core",
		AllowedLinkTypes:       []string{"production_for", "movement_for", "related_to"},
		AllowedAttachmentTypes: []string{"note", "image", "document"},
	}
}

func productionFrontendActions() []module.ActionDefinition {
	actions := []module.ActionDefinition{
		{Key: "production.boms.list", Label: "BOMs", LabelI18n: localize("BOMs", "BOM"), Kind: "navigate", RoutePath: "/production/boms", ViewKey: "production.boms.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_bom.list"}},
		{Key: "production.boms.detail", Label: "BOM Detail", LabelI18n: localize("BOM Detail", "Detail BOM"), Kind: "navigate", RoutePath: "/production/boms/detail", ViewKey: "production.boms.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_bom.read"}},
		{Key: "production.boms.form", Label: "BOM Form", LabelI18n: localize("BOM Form", "Form BOM"), Kind: "navigate", RoutePath: "/production/boms/form", ViewKey: "production.boms.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_bom.update"}},
		{Key: "production.bom_versions.list", Label: "BOM Versions", LabelI18n: localize("BOM Versions", "Versi BOM"), Kind: "navigate", RoutePath: "/production/bom-versions", ViewKey: "production.bom_versions.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_bom_version.list"}},
		{Key: "production.bom_versions.detail", Label: "BOM Version Detail", LabelI18n: localize("BOM Version Detail", "Detail Versi BOM"), Kind: "navigate", RoutePath: "/production/bom-versions/detail", ViewKey: "production.bom_versions.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_bom_version.read"}},
		{Key: "production.bom_versions.form", Label: "BOM Version Form", LabelI18n: localize("BOM Version Form", "Form Versi BOM"), Kind: "navigate", RoutePath: "/production/bom-versions/form", ViewKey: "production.bom_versions.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_bom_version.update"}},
		{Key: "production.work_centers.list", Label: "Work Centers", LabelI18n: localize("Work Centers", "Pusat Kerja"), Kind: "navigate", RoutePath: "/production/work-centers", ViewKey: "production.work_centers.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_work_center.list"}},
		{Key: "production.work_centers.detail", Label: "Work Center Detail", LabelI18n: localize("Work Center Detail", "Detail Pusat Kerja"), Kind: "navigate", RoutePath: "/production/work-centers/detail", ViewKey: "production.work_centers.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_work_center.read"}},
		{Key: "production.work_centers.form", Label: "Work Center Form", LabelI18n: localize("Work Center Form", "Form Pusat Kerja"), Kind: "navigate", RoutePath: "/production/work-centers/form", ViewKey: "production.work_centers.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"production_work_center.update"}},
	}
	actions = append(actions, commercialDocumentActions("production.orders", "production_order", "Production Orders", "Production Order", "New Production Order", "/production/orders")...)
	actions = append(actions, commercialDocumentActions("production.issues", "production_issue", "Production Issues", "Production Issue", "New Production Issue", "/production/issues")...)
	actions = append(actions, commercialDocumentActions("production.outputs", "production_output", "Production Outputs", "Production Output", "New Production Output", "/production/outputs")...)
	return actions
}

func productionFrontendViews() []module.ViewDefinition {
	views := []module.ViewDefinition{
		commercialModelListView("production.boms.list", "BOMs", "production_bom", []module.ColumnDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
			{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "values.finished_item_code"},
			{Key: "default_version_code", Label: "Default Version", LabelI18n: localize("Default Version", "Versi Default"), Path: "values.default_version_code"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		}, []string{"active", "inactive"}),
		commercialModelDetailView("production.boms.detail", "BOM Detail", "production_bom", []module.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
			{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "values.finished_item_code", Type: "string"},
			{Key: "default_version_code", Label: "Default Version", LabelI18n: localize("Default Version", "Versi Default"), Path: "values.default_version_code", Type: "string"},
			{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
		}),
		commercialModelFormView("production.boms.form", "BOM Form", "production_bom", []module.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
			{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "values.finished_item_code", Type: "string", Widget: "select", Required: true},
			{Key: "default_version_code", Label: "Default Version", LabelI18n: localize("Default Version", "Versi Default"), Path: "values.default_version_code", Type: "string", Widget: "text"},
			{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string", Widget: "textarea"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}),
		commercialModelListView("production.bom_versions.list", "BOM Versions", "production_bom_version", []module.ColumnDefinition{
			{Key: "bom_code", Label: "BOM", LabelI18n: localize("BOM", "BOM"), Path: "values.bom_code"},
			{Key: "version_code", Label: "Version", LabelI18n: localize("Version", "Versi"), Path: "values.version_code"},
			{Key: "yield_quantity", Label: "Yield", LabelI18n: localize("Yield", "Hasil"), Path: "values.yield_quantity"},
			{Key: "is_active", Label: "Active", LabelI18n: localize("Active", "Aktif"), Path: "values.is_active"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		}, []string{"active", "inactive"}),
		commercialModelDetailView("production.bom_versions.detail", "BOM Version Detail", "production_bom_version", []module.FieldDefinition{
			{Key: "bom_code", Label: "BOM", LabelI18n: localize("BOM", "BOM"), Path: "values.bom_code", Type: "string"},
			{Key: "version_code", Label: "Version", LabelI18n: localize("Version", "Versi"), Path: "values.version_code", Type: "string"},
			{Key: "yield_quantity", Label: "Yield", LabelI18n: localize("Yield", "Hasil"), Path: "values.yield_quantity", Type: "number"},
			{Key: "is_active", Label: "Active", LabelI18n: localize("Active", "Aktif"), Path: "values.is_active", Type: "bool"},
			{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "values.lines", Type: "object", Widget: "production_component_lines"},
			{Key: "stages", Label: "Stages", LabelI18n: localize("Stages", "Tahap"), Path: "values.stages", Type: "object", Widget: "production_stage_lines"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
		}),
		commercialModelFormView("production.bom_versions.form", "BOM Version Form", "production_bom_version", []module.FieldDefinition{
			{Key: "bom_id", Label: "BOM", LabelI18n: localize("BOM", "BOM"), Path: "values.bom_id", Type: "string", Widget: "select", Required: true},
			{Key: "bom_code", Label: "BOM Code", LabelI18n: localize("BOM Code", "Kode BOM"), Path: "values.bom_code", Type: "string", Widget: "text"},
			{Key: "version_code", Label: "Version", LabelI18n: localize("Version", "Versi"), Path: "values.version_code", Type: "string", Widget: "text", Required: true},
			{Key: "yield_quantity", Label: "Yield Quantity", LabelI18n: localize("Yield Quantity", "Jumlah Hasil"), Path: "values.yield_quantity", Type: "number"},
			{Key: "is_active", Label: "Active", LabelI18n: localize("Active", "Aktif"), Path: "values.is_active", Type: "bool"},
			{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "values.lines", Type: "object", Widget: "production_component_lines"},
			{Key: "stages", Label: "Stages", LabelI18n: localize("Stages", "Tahap"), Path: "values.stages", Type: "object", Widget: "production_stage_lines"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}),
		commercialModelListView("production.work_centers.list", "Work Centers", "production_work_center", []module.ColumnDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
			{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		}, []string{"active", "inactive"}),
		commercialModelDetailView("production.work_centers.detail", "Work Center Detail", "production_work_center", []module.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
			{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind", Type: "string"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
		}),
		commercialModelFormView("production.work_centers.form", "Work Center Form", "production_work_center", []module.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
			{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind", Type: "string", Widget: "text"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}),
	}
	views = append(views, commercialDocumentViews("production.orders", "production_order", "Production Orders", "Production Order Detail", "Production Order Draft", []module.ColumnDefinition{
		{Key: "number", Label: "Order", LabelI18n: localize("Order", "Order"), Path: "header.number"},
		{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "body.payload.finished_item_code"},
		{Key: "planned_quantity", Label: "Planned", LabelI18n: localize("Planned", "Direncanakan"), Path: "body.payload.planned_quantity"},
		{Key: "actual_output_quantity", Label: "Output", LabelI18n: localize("Output", "Output"), Path: "body.payload.actual_output_quantity"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}, []string{"draft", "submitted", "approved", "in_progress", "completed", "rejected", "cancelled"}, productionOrderSections(), productionOrderFormSections())...)
	views = append(views, commercialDocumentViews("production.issues", "production_issue", "Production Issues", "Production Issue Detail", "Production Issue Draft", []module.ColumnDefinition{
		{Key: "number", Label: "Issue", LabelI18n: localize("Issue", "Issue"), Path: "header.number"},
		{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "body.payload.finished_item_code"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code"},
		{Key: "total_quantity", Label: "Total Quantity", LabelI18n: localize("Total Quantity", "Total Jumlah"), Path: "body.payload.total_quantity"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}, []string{"draft", "submitted", "issued", "rejected", "cancelled"}, productionIssueSections(), productionIssueFormSections())...)
	views = append(views, commercialDocumentViews("production.outputs", "production_output", "Production Outputs", "Production Output Detail", "Production Output Draft", []module.ColumnDefinition{
		{Key: "number", Label: "Output", LabelI18n: localize("Output", "Output"), Path: "header.number"},
		{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "body.payload.finished_item_code"},
		{Key: "output_quantity", Label: "Output Quantity", LabelI18n: localize("Output Quantity", "Jumlah Output"), Path: "body.payload.output_quantity"},
		{Key: "production_lot_code", Label: "Lot", LabelI18n: localize("Lot", "Lot"), Path: "body.payload.production_lot_code"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}, []string{"draft", "submitted", "posted", "rejected", "cancelled"}, productionOutputSections(), productionOutputFormSections())...)
	for index := range views {
		if views[index].Key == "production.orders.detail" {
			views[index].AllowedActions = append(commercialDetailActions("production_order"), "register_production_issue", "register_production_output")
		}
	}
	return views
}

func productionOrderSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Production Summary", TitleI18n: localize("Production Summary", "Ringkasan Produksi"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "production_pattern", Label: "Pattern", LabelI18n: localize("Pattern", "Pola"), Path: "body.payload.production_pattern", Type: "string"},
		{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "body.payload.finished_item_code", Type: "string"},
		{Key: "bom_code", Label: "BOM", LabelI18n: localize("BOM", "BOM"), Path: "body.payload.bom_code", Type: "string"},
		{Key: "bom_version_code", Label: "Version", LabelI18n: localize("Version", "Versi"), Path: "body.payload.bom_version_code", Type: "string"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string"},
		{Key: "planned_quantity", Label: "Planned Quantity", LabelI18n: localize("Planned Quantity", "Jumlah Rencana"), Path: "body.payload.planned_quantity", Type: "number"},
		{Key: "reserved_quantity_total", Label: "Reserved Quantity", LabelI18n: localize("Reserved Quantity", "Jumlah Direservasi"), Path: "body.payload.reserved_quantity_total", Type: "number"},
		{Key: "shortage_quantity_total", Label: "Shortage Quantity", LabelI18n: localize("Shortage Quantity", "Jumlah Kekurangan"), Path: "body.payload.shortage_quantity_total", Type: "number"},
		{Key: "actual_output_quantity", Label: "Actual Output", LabelI18n: localize("Actual Output", "Output Aktual"), Path: "body.payload.actual_output_quantity", Type: "number"},
		{Key: "waste_quantity", Label: "Waste", LabelI18n: localize("Waste", "Waste"), Path: "body.payload.waste_quantity", Type: "number"},
		{Key: "lines", Label: "Components", LabelI18n: localize("Components", "Komponen"), Path: "body.payload.lines", Type: "object", Widget: "production_component_lines"},
		{Key: "stages", Label: "Stages", LabelI18n: localize("Stages", "Tahap"), Path: "body.payload.stages", Type: "object", Widget: "production_stage_lines"},
	}}}
}

func productionOrderFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Production Draft", TitleI18n: localize("Production Draft", "Draft Produksi"), Fields: []module.FieldDefinition{
		{Key: "production_pattern", Label: "Pattern", LabelI18n: localize("Pattern", "Pola"), Path: "body.payload.production_pattern", Type: "string", Widget: "select", Options: []string{"make_to_stock", "make_to_order"}},
		{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "body.payload.finished_item_code", Type: "string", Widget: "select", Required: true},
		{Key: "bom_id", Label: "BOM", LabelI18n: localize("BOM", "BOM"), Path: "body.payload.bom_id", Type: "string", Widget: "select"},
		{Key: "bom_version_id", Label: "BOM Version", LabelI18n: localize("BOM Version", "Versi BOM"), Path: "body.payload.bom_version_id", Type: "string", Widget: "select"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string", Widget: "select"},
		{Key: "planned_quantity", Label: "Planned Quantity", LabelI18n: localize("Planned Quantity", "Jumlah Rencana"), Path: "body.payload.planned_quantity", Type: "number", Required: true},
		{Key: "expected_output_quantity", Label: "Expected Output", LabelI18n: localize("Expected Output", "Output Harapan"), Path: "body.payload.expected_output_quantity", Type: "number"},
		{Key: "source_sales_order_id", Label: "Source Sales Order", LabelI18n: localize("Source Sales Order", "Order Penjualan Sumber"), Path: "body.payload.source_sales_order_id", Type: "string", Widget: "text"},
		{Key: "lines", Label: "Components", LabelI18n: localize("Components", "Komponen"), Path: "body.payload.lines", Type: "object", Widget: "production_component_lines"},
		{Key: "stages", Label: "Stages", LabelI18n: localize("Stages", "Tahap"), Path: "body.payload.stages", Type: "object", Widget: "production_stage_lines"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}

func productionIssueSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Production Issue Summary", TitleI18n: localize("Production Issue Summary", "Ringkasan Issue Produksi"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "body.payload.finished_item_code", Type: "string"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string"},
		{Key: "total_quantity", Label: "Total Quantity", LabelI18n: localize("Total Quantity", "Total Jumlah"), Path: "body.payload.total_quantity", Type: "number"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "production_issue_lines"},
	}}}
}

func productionIssueFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Production Issue Draft", TitleI18n: localize("Production Issue Draft", "Draft Issue Produksi"), Fields: []module.FieldDefinition{
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string", Widget: "select"},
		{Key: "issue_date", Label: "Issue Date", LabelI18n: localize("Issue Date", "Tanggal Issue"), Path: "body.payload.issue_date", Type: "string", Widget: "text"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "production_issue_lines"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}

func productionOutputSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Production Output Summary", TitleI18n: localize("Production Output Summary", "Ringkasan Output Produksi"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "body.payload.finished_item_code", Type: "string"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string"},
		{Key: "output_quantity", Label: "Output Quantity", LabelI18n: localize("Output Quantity", "Jumlah Output"), Path: "body.payload.output_quantity", Type: "number"},
		{Key: "waste_quantity", Label: "Waste Quantity", LabelI18n: localize("Waste Quantity", "Jumlah Waste"), Path: "body.payload.waste_quantity", Type: "number"},
		{Key: "production_lot_code", Label: "Production Lot", LabelI18n: localize("Production Lot", "Lot Produksi"), Path: "body.payload.production_lot_code", Type: "string"},
		{Key: "expiration_date", Label: "Expiration Date", LabelI18n: localize("Expiration Date", "Tanggal Kedaluwarsa"), Path: "body.payload.expiration_date", Type: "string"},
	}}}
}

func productionOutputFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Production Output Draft", TitleI18n: localize("Production Output Draft", "Draft Output Produksi"), Fields: []module.FieldDefinition{
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string", Widget: "select", Required: true},
		{Key: "output_date", Label: "Output Date", LabelI18n: localize("Output Date", "Tanggal Output"), Path: "body.payload.output_date", Type: "string", Widget: "text"},
		{Key: "finished_item_code", Label: "Finished Item", LabelI18n: localize("Finished Item", "Item Jadi"), Path: "body.payload.finished_item_code", Type: "string", Widget: "select", Required: true},
		{Key: "output_quantity", Label: "Output Quantity", LabelI18n: localize("Output Quantity", "Jumlah Output"), Path: "body.payload.output_quantity", Type: "number", Required: true},
		{Key: "waste_quantity", Label: "Waste Quantity", LabelI18n: localize("Waste Quantity", "Jumlah Waste"), Path: "body.payload.waste_quantity", Type: "number"},
		{Key: "production_lot_code", Label: "Production Lot", LabelI18n: localize("Production Lot", "Lot Produksi"), Path: "body.payload.production_lot_code", Type: "string", Widget: "text", Required: true},
		{Key: "expiration_date", Label: "Expiration Date", LabelI18n: localize("Expiration Date", "Tanggal Kedaluwarsa"), Path: "body.payload.expiration_date", Type: "string", Widget: "text"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}
