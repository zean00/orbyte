package app

import (
	"fmt"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func inventoryCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "inventory_core",
		Name:         "Inventory Core",
		NameI18n:     localize("Inventory Core", "Inti Inventori"),
		Version:      "1.0.0",
		DomainFamily: "business",
		OwnedDocumentTypes: []string{
			"stock_receipt",
			"stock_issue",
			"stock_adjustment",
			"stock_transfer",
			"stock_movement",
		},
		OwnedWorkflowKeys: []string{
			"stock_receipt_flow",
			"stock_issue_flow",
			"stock_adjustment_flow",
			"stock_transfer_flow",
			"stock_movement_flow",
		},
		OwnedTemplateKeys: []string{
			"inventory.stock_receipt.print.default",
			"inventory.stock_issue.print.default",
			"inventory.stock_adjustment.print.default",
			"inventory.stock_transfer.print.default",
			"inventory.stock_movement.print.default",
		},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "commercial_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "procurement_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Inventory Console",
			TitleI18n:       localize("Inventory Console", "Konsol Inventori"),
			Description:     "Warehouse setup, stock operations, and inventory workflows.",
			DescriptionI18n: localize("Warehouse setup, stock operations, and inventory workflows.", "Setup gudang, operasi stok, dan workflow inventori."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:             "policy",
					Title:           "Inventory Policy",
					TitleI18n:       localize("Inventory Policy", "Kebijakan Inventori"),
					Kind:            module.AdminConsoleSectionSettingsForm,
					ConfigKey:       "inventory.policy",
					Description:     "Configure near-expiry handling defaults.",
					DescriptionI18n: localize("Configure near-expiry handling defaults.", "Atur nilai baku penanganan hampir kedaluwarsa."),
				},
				{
					Key:       "setup",
					Title:     "Inventory Setup",
					TitleI18n: localize("Inventory Setup", "Setup Inventori"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("items", "Items", "Item", "/ui/commercial/items", "Configure per-item inventory policy.", "Konfigurasi kebijakan inventori per item.", "item.list"),
						adminConsoleLink("warehouses", "Warehouses", "Gudang", "/ui/inventory/warehouses", "Manage warehouses.", "Kelola gudang.", "warehouse.list"),
						adminConsoleLink("batches", "Batches", "Batch", "/ui/inventory/batches", "Review known item batches.", "Tinjau batch item yang terdaftar.", "inventory_batch.list"),
					},
				},
				{
					Key:       "operations",
					Title:     "Inventory Operations",
					TitleI18n: localize("Inventory Operations", "Operasi Inventori"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("summary", "Inventory", "Inventori", "/ui/inventory", "Open inventory dashboard.", "Buka dashboard inventori.", "document.list"),
						adminConsoleLink("receipts", "Stock Receipts", "Penerimaan Stok", "/ui/inventory/receipts", "Open stock receipts.", "Buka penerimaan stok.", "document.list"),
						adminConsoleLink("issues", "Stock Issues", "Pengeluaran Stok", "/ui/inventory/issues", "Open stock issues.", "Buka pengeluaran stok.", "document.list"),
						adminConsoleLink("adjustments", "Adjustments", "Penyesuaian", "/ui/inventory/adjustments", "Open stock adjustments.", "Buka penyesuaian stok.", "document.list"),
						adminConsoleLink("transfers", "Transfers", "Transfer", "/ui/inventory/transfers", "Open stock transfers.", "Buka transfer stok.", "document.list"),
						adminConsoleLink("movements", "Movements", "Pergerakan", "/ui/inventory/movements", "Open stock movement ledger.", "Buka ledger pergerakan stok.", "document.list"),
					},
				},
				{
					Key:       "workflows",
					Title:     "Inventory Workflows",
					TitleI18n: localize("Inventory Workflows", "Workflow Inventori"),
					Kind:      module.AdminConsoleSectionWorkflowLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("stock_receipt_flow", "Stock Receipt Flow", "Workflow Penerimaan Stok", "/admin/workflows/designer?key=stock_receipt_flow", "Open stock receipt workflow.", "Buka workflow penerimaan stok.", "workflow.read"),
						adminConsoleLink("stock_issue_flow", "Stock Issue Flow", "Workflow Pengeluaran Stok", "/admin/workflows/designer?key=stock_issue_flow", "Open stock issue workflow.", "Buka workflow pengeluaran stok.", "workflow.read"),
						adminConsoleLink("stock_adjustment_flow", "Stock Adjustment Flow", "Workflow Penyesuaian", "/admin/workflows/designer?key=stock_adjustment_flow", "Open stock adjustment workflow.", "Buka workflow penyesuaian stok.", "workflow.read"),
						adminConsoleLink("stock_transfer_flow", "Stock Transfer Flow", "Workflow Transfer Stok", "/admin/workflows/designer?key=stock_transfer_flow", "Open stock transfer workflow.", "Buka workflow transfer stok.", "workflow.read"),
					},
				},
			},
		},
		Models: []model.Definition{
			inventoryModelDefinition("warehouse", "Warehouse", "warehouse", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Type: "string", DefaultValue: "storage"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			inventoryModelDefinition("inventory_batch", "Inventory Batch", "inventory_batch", []model.FieldDefinition{
				{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Type: "string", Required: true},
				{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Type: "string", Required: true},
				{Key: "batch_code", Label: "Batch Code", LabelI18n: localize("Batch Code", "Kode Batch"), Type: "string", Required: true},
				{Key: "expiration_date", Label: "Expiration Date", LabelI18n: localize("Expiration Date", "Tanggal Kedaluwarsa"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				{Key: "hold_reason", Label: "Hold Reason", LabelI18n: localize("Hold Reason", "Alasan Penahanan"), Type: "string"},
				{Key: "hold_notes", Label: "Hold Notes", LabelI18n: localize("Hold Notes", "Catatan Penahanan"), Type: "string"},
				{Key: "recall_reference", Label: "Recall Reference", LabelI18n: localize("Recall Reference", "Referensi Recall"), Type: "string"},
				{Key: "is_issuable", Label: "Issuable", LabelI18n: localize("Issuable", "Bisa Dikeluarkan"), Type: "bool"},
				{Key: "on_hand_quantity", Label: "On Hand", LabelI18n: localize("On Hand", "Stok Tersedia"), Type: "number"},
				{Key: "reserved_quantity", Label: "Reserved", LabelI18n: localize("Reserved", "Dicadangkan"), Type: "number"},
				{Key: "available_quantity", Label: "Available", LabelI18n: localize("Available", "Tersedia"), Type: "number"},
			}),
			inventoryModelDefinition("inventory_cost_layer", "Inventory Cost Layer", "inventory_cost_layer", []model.FieldDefinition{
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Type: "string", Required: true},
				{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Type: "string", Required: true},
				{Key: "batch_code", Label: "Batch Code", LabelI18n: localize("Batch Code", "Kode Batch"), Type: "string"},
				{Key: "source_document_type", Label: "Source Document Type", LabelI18n: localize("Source Document Type", "Tipe Dokumen Sumber"), Type: "string"},
				{Key: "source_document_id", Label: "Source Document", LabelI18n: localize("Source Document", "Dokumen Sumber"), Type: "string"},
				{Key: "movement_document_id", Label: "Movement Document", LabelI18n: localize("Movement Document", "Dokumen Pergerakan"), Type: "string"},
				{Key: "event_type", Label: "Event Type", LabelI18n: localize("Event Type", "Tipe Kejadian"), Type: "string"},
				{Key: "quantity_delta", Label: "Quantity Delta", LabelI18n: localize("Quantity Delta", "Delta Kuantitas"), Type: "number"},
				{Key: "unit_cost", Label: "Unit Cost", LabelI18n: localize("Unit Cost", "Biaya Satuan"), Type: "number"},
				{Key: "total_cost", Label: "Total Cost", LabelI18n: localize("Total Cost", "Total Biaya"), Type: "number"},
				{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Type: "string"},
				{Key: "valuation_method", Label: "Valuation Method", LabelI18n: localize("Valuation Method", "Metode Penilaian"), Type: "string", DefaultValue: "weighted_average"},
				{Key: "effective_at", Label: "Effective At", LabelI18n: localize("Effective At", "Efektif Pada"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "posted"},
			}),
			inventoryModelDefinition("inventory_valuation_snapshot", "Inventory Valuation Snapshot", "inventory_valuation_snapshot", []model.FieldDefinition{
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Type: "string", Required: true},
				{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Type: "string", Required: true},
				{Key: "quantity_on_hand", Label: "On Hand Quantity", LabelI18n: localize("On Hand Quantity", "Kuantitas On Hand"), Type: "number"},
				{Key: "average_unit_cost", Label: "Average Unit Cost", LabelI18n: localize("Average Unit Cost", "Biaya Rata-Rata"), Type: "number"},
				{Key: "inventory_value", Label: "Inventory Value", LabelI18n: localize("Inventory Value", "Nilai Inventori"), Type: "number"},
				{Key: "valuation_method", Label: "Valuation Method", LabelI18n: localize("Valuation Method", "Metode Penilaian"), Type: "string", DefaultValue: "weighted_average"},
				{Key: "last_calculated_at", Label: "Last Calculated At", LabelI18n: localize("Last Calculated At", "Terakhir Dihitung"), Type: "string"},
			}),
		},
		Documents: []document.Definition{
			inventoryDocumentDefinition("stock_receipt", "Stock Receipt", "stock_receipt_flow", "stock_receipt_number"),
			inventoryDocumentDefinition("stock_issue", "Stock Issue", "stock_issue_flow", "stock_issue_number"),
			inventoryDocumentDefinition("stock_adjustment", "Stock Adjustment", "stock_adjustment_flow", "stock_adjustment_number"),
			inventoryDocumentDefinition("stock_transfer", "Stock Transfer", "stock_transfer_flow", "stock_transfer_number"),
			inventoryDocumentDefinition("stock_movement", "Stock Movement", "stock_movement_flow", "stock_movement_number"),
		},
		Workflows: []workflow.Definition{
			inventoryWorkflowDefinition("stock_receipt_flow", "received", true, true),
			inventoryWorkflowDefinition("stock_issue_flow", "issued", true, true),
			inventoryWorkflowDefinition("stock_adjustment_flow", "posted", true, true),
			inventoryWorkflowDefinition("stock_transfer_flow", "transferred", true, true),
			inventoryWorkflowDefinition("stock_movement_flow", "posted", false, false),
		},
		SearchIndexes: []search.IndexDefinition{
			commercialModelSearchIndex("inventory.warehouses.search", "Warehouse Search", "warehouse", "inventory.warehouses.list", []string{"code", "name", "status"}),
			commercialModelSearchIndex("inventory.batches.search", "Batch Search", "inventory_batch", "inventory.batches.list", []string{"item_code", "warehouse_code", "batch_code", "status"}),
			commercialDocumentSearchIndex("inventory.receipts.search", "Stock Receipt Search", "stock_receipt", "inventory.receipts.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "warehouse_code", Path: "body.payload.warehouse_code", Type: "string", Searchable: true},
			}),
			commercialDocumentSearchIndex("inventory.issues.search", "Stock Issue Search", "stock_issue", "inventory.issues.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "warehouse_code", Path: "body.payload.warehouse_code", Type: "string", Searchable: true},
			}),
			commercialDocumentSearchIndex("inventory.adjustments.search", "Stock Adjustment Search", "stock_adjustment", "inventory.adjustments.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			}),
			commercialDocumentSearchIndex("inventory.transfers.search", "Stock Transfer Search", "stock_transfer", "inventory.transfers.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			}),
			commercialDocumentSearchIndex("inventory.movements.search", "Stock Movement Search", "stock_movement", "inventory.movements.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "item_code", Path: "body.payload.item_code", Type: "string", Searchable: true},
				{Key: "warehouse_code", Path: "body.payload.warehouse_code", Type: "string", Searchable: true},
				{Key: "batch_code", Path: "body.payload.batch_code", Type: "string", Searchable: true},
			}),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "warehouse.create", Action: "create", Resource: "warehouse", DisplayName: "Create Warehouses", DisplayNameI18n: localize("Create Warehouses", "Buat Gudang")},
				{Key: "warehouse.list", Action: "list", Resource: "warehouse", DisplayName: "List Warehouses", DisplayNameI18n: localize("List Warehouses", "Daftar Gudang")},
				{Key: "warehouse.read", Action: "read", Resource: "warehouse", DisplayName: "Read Warehouses", DisplayNameI18n: localize("Read Warehouses", "Lihat Gudang")},
				{Key: "warehouse.update", Action: "update", Resource: "warehouse", DisplayName: "Update Warehouses", DisplayNameI18n: localize("Update Warehouses", "Perbarui Gudang")},
				{Key: "inventory_batch.create", Action: "create", Resource: "inventory_batch", DisplayName: "Create Batches", DisplayNameI18n: localize("Create Batches", "Buat Batch")},
				{Key: "inventory_batch.list", Action: "list", Resource: "inventory_batch", DisplayName: "List Batches", DisplayNameI18n: localize("List Batches", "Daftar Batch")},
				{Key: "inventory_batch.read", Action: "read", Resource: "inventory_batch", DisplayName: "Read Batches", DisplayNameI18n: localize("Read Batches", "Lihat Batch")},
				{Key: "inventory_batch.update", Action: "update", Resource: "inventory_batch", DisplayName: "Update Batches", DisplayNameI18n: localize("Update Batches", "Perbarui Batch")},
				{Key: "inventory_cost_layer.create", Action: "create", Resource: "inventory_cost_layer", DisplayName: "Create Cost Layers", DisplayNameI18n: localize("Create Cost Layers", "Buat Layer Biaya")},
				{Key: "inventory_cost_layer.list", Action: "list", Resource: "inventory_cost_layer", DisplayName: "List Cost Layers", DisplayNameI18n: localize("List Cost Layers", "Daftar Layer Biaya")},
				{Key: "inventory_cost_layer.read", Action: "read", Resource: "inventory_cost_layer", DisplayName: "Read Cost Layers", DisplayNameI18n: localize("Read Cost Layers", "Lihat Layer Biaya")},
				{Key: "inventory_cost_layer.update", Action: "update", Resource: "inventory_cost_layer", DisplayName: "Update Cost Layers", DisplayNameI18n: localize("Update Cost Layers", "Perbarui Layer Biaya")},
				{Key: "inventory_valuation_snapshot.create", Action: "create", Resource: "inventory_valuation_snapshot", DisplayName: "Create Valuation Snapshots", DisplayNameI18n: localize("Create Valuation Snapshots", "Buat Snapshot Penilaian")},
				{Key: "inventory_valuation_snapshot.list", Action: "list", Resource: "inventory_valuation_snapshot", DisplayName: "List Valuation Snapshots", DisplayNameI18n: localize("List Valuation Snapshots", "Daftar Snapshot Penilaian")},
				{Key: "inventory_valuation_snapshot.read", Action: "read", Resource: "inventory_valuation_snapshot", DisplayName: "Read Valuation Snapshots", DisplayNameI18n: localize("Read Valuation Snapshots", "Lihat Snapshot Penilaian")},
				{Key: "inventory_valuation_snapshot.update", Action: "update", Resource: "inventory_valuation_snapshot", DisplayName: "Update Valuation Snapshots", DisplayNameI18n: localize("Update Valuation Snapshots", "Perbarui Snapshot Penilaian")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "inventory_manager",
					Name:           "Inventory Manager",
					NameI18n:       localize("Inventory Manager", "Manajer Inventori"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"warehouse.create", "warehouse.list", "warehouse.read", "warehouse.update", "inventory_batch.list", "inventory_batch.read", "document.create", "document.list", "document.read", "document.update_draft", "document.submit", "document.approve", "document.reject", "document.reopen", "document.cancel"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "inventory.dashboard", Label: "Inventory", LabelI18n: localize("Inventory", "Inventori"), ActionKey: "inventory.dashboard", Order: 50, RequiredPermissions: []string{"document.list"}},
				{Key: "inventory.warehouses", Label: "Warehouses", LabelI18n: localize("Warehouses", "Gudang"), ActionKey: "inventory.warehouses.list", Order: 51, RequiredPermissions: []string{"warehouse.list"}},
				{Key: "inventory.batches", Label: "Batches", LabelI18n: localize("Batches", "Batch"), ActionKey: "inventory.batches.list", Order: 52, RequiredPermissions: []string{"inventory_batch.list"}},
				{Key: "inventory.receipts", Label: "Stock Receipts", LabelI18n: localize("Stock Receipts", "Penerimaan Stok"), ActionKey: "inventory.receipts.list", Order: 53, RequiredPermissions: []string{"document.list"}},
				{Key: "inventory.issues", Label: "Stock Issues", LabelI18n: localize("Stock Issues", "Pengeluaran Stok"), ActionKey: "inventory.issues.list", Order: 54, RequiredPermissions: []string{"document.list"}},
				{Key: "inventory.adjustments", Label: "Adjustments", LabelI18n: localize("Adjustments", "Penyesuaian"), ActionKey: "inventory.adjustments.list", Order: 55, RequiredPermissions: []string{"document.list"}},
				{Key: "inventory.transfers", Label: "Transfers", LabelI18n: localize("Transfers", "Transfer"), ActionKey: "inventory.transfers.list", Order: 56, RequiredPermissions: []string{"document.list"}},
				{Key: "inventory.movements", Label: "Movements", LabelI18n: localize("Movements", "Pergerakan"), ActionKey: "inventory.movements.list", Order: 57, RequiredPermissions: []string{"document.list"}},
			},
			Actions: inventoryFrontendActions(),
			Views:   inventoryFrontendViews(),
		},
		Templates: []module.TemplateDefinition{
			commercialTemplateDefinition("inventory.stock_receipt.print.default", "Stock Receipt Print", "stock_receipt", "Stock Receipt", []string{"lines", "total_quantity"}),
			commercialTemplateDefinition("inventory.stock_issue.print.default", "Stock Issue Print", "stock_issue", "Stock Issue", []string{"lines", "total_quantity"}),
			commercialTemplateDefinition("inventory.stock_adjustment.print.default", "Stock Adjustment Print", "stock_adjustment", "Stock Adjustment", []string{"lines", "total_quantity"}),
			commercialTemplateDefinition("inventory.stock_transfer.print.default", "Stock Transfer Print", "stock_transfer", "Stock Transfer", []string{"lines", "total_quantity"}),
			commercialTemplateDefinition("inventory.stock_movement.print.default", "Stock Movement Print", "stock_movement", "Stock Movement", []string{"item_code", "warehouse_code", "batch_code", "quantity_delta"}),
		},
	}
}

func inventoryModelDefinition(key, singular, permissionPrefix string, fields []model.FieldDefinition) model.Definition {
	return model.Definition{
		Key:                 key,
		DisplayName:         singular,
		DisplayNameI18n:     localize(singular, singular),
		OwnerModuleKey:      "inventory_core",
		Version:             "v1",
		CreatePermissionKey: permissionPrefix + ".create",
		ListPermissionKey:   permissionPrefix + ".list",
		ReadPermissionKey:   permissionPrefix + ".read",
		UpdatePermissionKey: permissionPrefix + ".update",
		DefaultSort:         fields[0].Key,
		Fields:              fields,
	}
}

func inventoryDocumentDefinition(documentType, displayName, workflowKey, numberingKey string) document.Definition {
	return document.Definition{
		Type:                   documentType,
		DisplayName:            displayName,
		SchemaVersion:          "v1",
		WorkflowKey:            workflowKey,
		NumberingKey:           numberingKey,
		OwnerModuleKey:         "inventory_core",
		AllowedLinkTypes:       []string{"movement_for", "receipt_for", "issue_for", "transfer_for"},
		AllowedAttachmentTypes: []string{"note", "image", "document"},
	}
}

func inventoryWorkflowDefinition(key, approvedState string, allowCancelApproved, allowSubmit bool) workflow.Definition {
	states := []string{"draft", "submitted", approvedState, "rejected", "cancelled"}
	actions := []workflow.ActionRule{
		{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
		{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
		{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
	}
	if allowSubmit {
		actions = append(actions,
			workflow.ActionRule{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
			workflow.ActionRule{Action: "approve", FromState: "submitted", ToState: approvedState, PermissionKey: "document.approve"},
			workflow.ActionRule{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
		)
	} else {
		actions = append(actions, workflow.ActionRule{Action: "approve", FromState: "draft", ToState: approvedState, PermissionKey: "document.approve"})
	}
	if allowCancelApproved {
		actions = append(actions, workflow.ActionRule{Action: "cancel", FromState: approvedState, ToState: "cancelled", PermissionKey: "document.cancel"})
	}
	return workflow.Definition{Key: key, States: states, Actions: actions}
}

func inventoryFrontendActions() []module.ActionDefinition {
	actions := []module.ActionDefinition{
		{Key: "inventory.dashboard", Label: "Inventory", LabelI18n: localize("Inventory", "Inventori"), Kind: "navigate", RoutePath: "/inventory", ViewKey: "inventory.dashboard", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"document.list"}},
		{Key: "inventory.warehouses.list", Label: "Warehouses", LabelI18n: localize("Warehouses", "Gudang"), Kind: "navigate", RoutePath: "/inventory/warehouses", ViewKey: "inventory.warehouses.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"warehouse.list"}},
		{Key: "inventory.warehouses.detail", Label: "Warehouse Detail", LabelI18n: localize("Warehouse Detail", "Detail Gudang"), Kind: "navigate", RoutePath: "/inventory/warehouses/detail", ViewKey: "inventory.warehouses.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"warehouse.read"}},
		{Key: "inventory.warehouses.form", Label: "Warehouse Form", LabelI18n: localize("Warehouse Form", "Form Gudang"), Kind: "navigate", RoutePath: "/inventory/warehouses/form", ViewKey: "inventory.warehouses.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"warehouse.update"}},
		{Key: "inventory.batches.list", Label: "Batches", LabelI18n: localize("Batches", "Batch"), Kind: "navigate", RoutePath: "/inventory/batches", ViewKey: "inventory.batches.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"inventory_batch.list"}},
		{Key: "inventory.batches.detail", Label: "Batch Detail", LabelI18n: localize("Batch Detail", "Detail Batch"), Kind: "navigate", RoutePath: "/inventory/batches/detail", ViewKey: "inventory.batches.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"inventory_batch.read"}},
		{Key: "inventory.batches.form", Label: "Batch Form", LabelI18n: localize("Batch Form", "Form Batch"), Kind: "navigate", RoutePath: "/inventory/batches/form", ViewKey: "inventory.batches.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"inventory_batch.update"}},
		{Key: "inventory.movements.list", Label: "Movements", LabelI18n: localize("Movements", "Pergerakan"), Kind: "navigate", RoutePath: "/inventory/movements", ViewKey: "inventory.movements.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"document.list"}},
		{Key: "inventory.movements.detail", Label: "Movement Detail", LabelI18n: localize("Movement Detail", "Detail Pergerakan"), Kind: "navigate", RoutePath: "/inventory/movements/detail", ViewKey: "inventory.movements.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"document.read"}},
	}
	actions = append(actions, commercialDocumentActions("inventory.receipts", "stock_receipt", "Stock Receipts", "Stock Receipt", "New Stock Receipt", "/inventory/receipts")...)
	actions = append(actions, commercialDocumentActions("inventory.issues", "stock_issue", "Stock Issues", "Stock Issue", "New Stock Issue", "/inventory/issues")...)
	actions = append(actions, commercialDocumentActions("inventory.adjustments", "stock_adjustment", "Adjustments", "Stock Adjustment", "New Stock Adjustment", "/inventory/adjustments")...)
	actions = append(actions, commercialDocumentActions("inventory.transfers", "stock_transfer", "Transfers", "Stock Transfer", "New Stock Transfer", "/inventory/transfers")...)
	return actions
}

func inventoryFrontendViews() []module.ViewDefinition {
	views := []module.ViewDefinition{
		module.ViewDefinition{
			Key:                 "inventory.dashboard",
			Title:               "Inventory",
			TitleI18n:           localize("Inventory", "Inventori"),
			Kind:                "dashboard",
			ProjectionKey:       "inventory.summary",
			RequiredPermissions: []string{"document.list"},
			Cards: []module.CardDefinition{
				{Key: "tracked_item_count", Label: "Tracked Items", LabelI18n: localize("Tracked Items", "Item Terlacak"), Path: "tracked_item_count", ActionKey: "commercial.items.list"},
				{Key: "warehouse_count", Label: "Warehouses", LabelI18n: localize("Warehouses", "Gudang"), Path: "warehouse_count", ActionKey: "inventory.warehouses.list"},
				{Key: "batch_count", Label: "Open Batches", LabelI18n: localize("Open Batches", "Batch Aktif"), Path: "batch_count", ActionKey: "inventory.batches.list"},
				{Key: "total_on_hand", Label: "On Hand", LabelI18n: localize("On Hand", "Stok Tersedia"), Path: "total_on_hand", ActionKey: "inventory.movements.list"},
				{Key: "expired_batch_count", Label: "Expired Batches", LabelI18n: localize("Expired Batches", "Batch Kedaluwarsa"), Path: "expired_batch_count", ActionKey: "inventory.batches.list"},
				{Key: "near_expiry_batch_count", Label: "Near Expiry", LabelI18n: localize("Near Expiry", "Hampir Kedaluwarsa"), Path: "near_expiry_batch_count", ActionKey: "inventory.batches.list"},
				{Key: "quarantined_batch_count", Label: "Quarantined", LabelI18n: localize("Quarantined", "Dikarantina"), Path: "quarantined_batch_count", ActionKey: "inventory.batches.list"},
				{Key: "blocked_batch_count", Label: "Blocked", LabelI18n: localize("Blocked", "Diblokir"), Path: "blocked_batch_count", ActionKey: "inventory.batches.list"},
				{Key: "recalled_batch_count", Label: "Recalled", LabelI18n: localize("Recalled", "Direcall"), Path: "recalled_batch_count", ActionKey: "inventory.batches.list"},
			},
		},
		commercialModelListView("inventory.warehouses.list", "Warehouses", "warehouse", []module.ColumnDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
			{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		}, []string{"active", "inactive"}),
		commercialModelDetailView("inventory.warehouses.detail", "Warehouse Detail", "warehouse", []module.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
			{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind", Type: "string"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
		}),
		commercialModelFormView("inventory.warehouses.form", "Warehouse Form", "warehouse", []module.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
			{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind", Type: "string", Widget: "select", Options: []string{"storage", "staging", "returns"}},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}),
		commercialModelListView("inventory.batches.list", "Batches", "inventory_batch", []module.ColumnDefinition{
			{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "values.item_code"},
			{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code"},
			{Key: "batch_code", Label: "Batch", LabelI18n: localize("Batch", "Batch"), Path: "values.batch_code"},
			{Key: "expiration_date", Label: "Expiration", LabelI18n: localize("Expiration", "Kedaluwarsa"), Path: "values.expiration_date"},
			{Key: "available_quantity", Label: "Available", LabelI18n: localize("Available", "Tersedia"), Path: "values.available_quantity"},
			{Key: "is_issuable", Label: "Issuable", LabelI18n: localize("Issuable", "Bisa Dikeluarkan"), Path: "values.is_issuable"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		}, []string{"active", "near_expiry", "expired", "quarantined", "blocked", "recalled"}),
		commercialModelDetailView("inventory.batches.detail", "Batch Detail", "inventory_batch", []module.FieldDefinition{
			{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "values.item_code", Type: "string"},
			{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code", Type: "string"},
			{Key: "batch_code", Label: "Batch", LabelI18n: localize("Batch", "Batch"), Path: "values.batch_code", Type: "string"},
			{Key: "expiration_date", Label: "Expiration", LabelI18n: localize("Expiration", "Kedaluwarsa"), Path: "values.expiration_date", Type: "string"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
			{Key: "is_issuable", Label: "Issuable", LabelI18n: localize("Issuable", "Bisa Dikeluarkan"), Path: "values.is_issuable", Type: "bool"},
			{Key: "on_hand_quantity", Label: "On Hand", LabelI18n: localize("On Hand", "Stok Tersedia"), Path: "values.on_hand_quantity", Type: "number"},
			{Key: "reserved_quantity", Label: "Reserved", LabelI18n: localize("Reserved", "Dicadangkan"), Path: "values.reserved_quantity", Type: "number"},
			{Key: "available_quantity", Label: "Available", LabelI18n: localize("Available", "Tersedia"), Path: "values.available_quantity", Type: "number"},
			{Key: "hold_reason", Label: "Hold Reason", LabelI18n: localize("Hold Reason", "Alasan Penahanan"), Path: "values.hold_reason", Type: "string"},
			{Key: "hold_notes", Label: "Hold Notes", LabelI18n: localize("Hold Notes", "Catatan Penahanan"), Path: "values.hold_notes", Type: "string"},
			{Key: "recall_reference", Label: "Recall Reference", LabelI18n: localize("Recall Reference", "Referensi Recall"), Path: "values.recall_reference", Type: "string"},
		}),
		commercialModelFormView("inventory.batches.form", "Batch Form", "inventory_batch", []module.FieldDefinition{
			{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "values.item_code", Type: "string", Widget: "select", Required: true},
			{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code", Type: "string", Widget: "select", Required: true},
			{Key: "batch_code", Label: "Batch", LabelI18n: localize("Batch", "Batch"), Path: "values.batch_code", Type: "string", Widget: "text", Required: true},
			{Key: "expiration_date", Label: "Expiration", LabelI18n: localize("Expiration", "Kedaluwarsa"), Path: "values.expiration_date", Type: "string", Widget: "text"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "near_expiry", "expired", "quarantined", "blocked", "recalled"}},
			{Key: "hold_reason", Label: "Hold Reason", LabelI18n: localize("Hold Reason", "Alasan Penahanan"), Path: "values.hold_reason", Type: "string", Widget: "text"},
			{Key: "hold_notes", Label: "Hold Notes", LabelI18n: localize("Hold Notes", "Catatan Penahanan"), Path: "values.hold_notes", Type: "string", Widget: "textarea"},
			{Key: "recall_reference", Label: "Recall Reference", LabelI18n: localize("Recall Reference", "Referensi Recall"), Path: "values.recall_reference", Type: "string", Widget: "text"},
		}),
	}
	views = append(views, commercialDocumentViews("inventory.receipts", "stock_receipt", "Stock Receipts", "Stock Receipt Detail", "Stock Receipt Draft", inventoryLineColumns(), []string{"draft", "submitted", "received", "rejected", "cancelled"}, inventoryDetailSections("stock_receipt", "inventory_lines"), inventoryFormSections("stock_receipt", "inventory_lines"))...)
	views = append(views, commercialDocumentViews("inventory.issues", "stock_issue", "Stock Issues", "Stock Issue Detail", "Stock Issue Draft", inventoryLineColumns(), []string{"draft", "submitted", "issued", "rejected", "cancelled"}, inventoryDetailSections("stock_issue", "inventory_lines"), inventoryFormSections("stock_issue", "inventory_lines"))...)
	views = append(views, commercialDocumentViews("inventory.adjustments", "stock_adjustment", "Adjustments", "Stock Adjustment Detail", "Stock Adjustment Draft", inventoryLineColumns(), []string{"draft", "submitted", "posted", "rejected", "cancelled"}, inventoryDetailSections("stock_adjustment", "inventory_lines"), inventoryFormSections("stock_adjustment", "inventory_lines"))...)
	views = append(views, commercialDocumentViews("inventory.transfers", "stock_transfer", "Transfers", "Stock Transfer Detail", "Stock Transfer Draft", inventoryTransferColumns(), []string{"draft", "submitted", "transferred", "rejected", "cancelled"}, inventoryTransferSections(), inventoryTransferFormSections())...)
	views = append(views,
		module.ViewDefinition{
			Key:                 "inventory.movements.list",
			Title:               "Movements",
			TitleI18n:           localize("Movements", "Pergerakan"),
			Kind:                "list",
			DocumentType:        "stock_movement",
			ProjectionKey:       "document_summary",
			RequiredPermissions: []string{"document.list"},
			Columns: []module.ColumnDefinition{
				{Key: "number", Label: "Movement", LabelI18n: localize("Movement", "Pergerakan"), Path: "header.number"},
				{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "body.payload.item_code"},
				{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code"},
				{Key: "batch_code", Label: "Batch", LabelI18n: localize("Batch", "Batch"), Path: "body.payload.batch_code"},
				{Key: "quantity_delta", Label: "Delta", LabelI18n: localize("Delta", "Selisih"), Path: "body.payload.quantity_delta"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
			},
			Filters:         []module.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: []string{"posted", "cancelled"}}},
			DefaultPageSize: 10,
			EmptyState:      "No stock movements exist yet.",
			EmptyStateI18n:  localize("No stock movements exist yet.", "Belum ada pergerakan stok."),
		},
		module.ViewDefinition{
			Key:                 "inventory.movements.detail",
			Title:               "Movement Detail",
			TitleI18n:           localize("Movement Detail", "Detail Pergerakan"),
			Kind:                "detail",
			DocumentType:        "stock_movement",
			RequiredPermissions: []string{"document.read"},
			Sections: []module.SectionDefinition{{Key: "summary", Title: "Movement Summary", TitleI18n: localize("Movement Summary", "Ringkasan Pergerakan"), Fields: []module.FieldDefinition{
				{Key: "number", Label: "Movement Number", LabelI18n: localize("Movement Number", "Nomor Pergerakan"), Path: "header.number", Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
				{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "body.payload.item_code", Type: "string"},
				{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string"},
				{Key: "batch_code", Label: "Batch", LabelI18n: localize("Batch", "Batch"), Path: "body.payload.batch_code", Type: "string"},
				{Key: "expiration_date", Label: "Expiration", LabelI18n: localize("Expiration", "Kedaluwarsa"), Path: "body.payload.expiration_date", Type: "string"},
				{Key: "quantity_delta", Label: "Delta", LabelI18n: localize("Delta", "Selisih"), Path: "body.payload.quantity_delta", Type: "number"},
				{Key: "movement_reason", Label: "Reason", LabelI18n: localize("Reason", "Alasan"), Path: "body.payload.movement_reason", Type: "string"},
			}}},
		},
	)
	return views
}

func inventoryLineColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Document", LabelI18n: localize("Document", "Dokumen"), Path: "header.number"},
		{Key: "total_quantity", Label: "Quantity", LabelI18n: localize("Quantity", "Jumlah"), Path: "body.payload.total_quantity"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func inventoryTransferColumns() []module.ColumnDefinition {
	return inventoryLineColumns()
}

func inventoryDetailSections(documentType, widget string) []module.SectionDefinition {
	title := fmt.Sprintf("%s Summary", humanizeTitle(documentType))
	return []module.SectionDefinition{{Key: "summary", Title: title, TitleI18n: localize(title, title), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string"},
		{Key: "total_quantity", Label: "Total Quantity", LabelI18n: localize("Total Quantity", "Total Jumlah"), Path: "body.payload.total_quantity", Type: "number"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: widget},
	}}}
}

func inventoryFormSections(documentType, widget string) []module.SectionDefinition {
	title := fmt.Sprintf("%s Draft", humanizeTitle(documentType))
	return []module.SectionDefinition{{Key: "edit", Title: title, TitleI18n: localize(title, title), Fields: []module.FieldDefinition{
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string", Widget: "select"},
		{Key: "movement_date", Label: "Date", LabelI18n: localize("Date", "Tanggal"), Path: "body.payload.movement_date", Type: "string", Widget: "text"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: widget},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}

func inventoryTransferSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Transfer Summary", TitleI18n: localize("Transfer Summary", "Ringkasan Transfer"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "total_quantity", Label: "Total Quantity", LabelI18n: localize("Total Quantity", "Total Jumlah"), Path: "body.payload.total_quantity", Type: "number"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "inventory_transfer_lines"},
	}}}
}

func inventoryTransferFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Transfer Draft", TitleI18n: localize("Transfer Draft", "Draft Transfer"), Fields: []module.FieldDefinition{
		{Key: "movement_date", Label: "Date", LabelI18n: localize("Date", "Tanggal"), Path: "body.payload.movement_date", Type: "string", Widget: "text"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "inventory_transfer_lines"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}
