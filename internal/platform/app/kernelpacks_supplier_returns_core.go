package app

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func supplierReturnsCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{supplierReturnsCoreKernelPackManifest()}
}

func supplierReturnsCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "supplier_returns_core",
		Name:         "Supplier Returns Core",
		NameI18n:     localize("Supplier Returns Core", "Inti Retur Vendor"),
		Version:      "1.0.0",
		DomainFamily: "business",
		OwnedDocumentTypes: []string{
			"supplier_return",
		},
		OwnedWorkflowKeys: []string{
			"supplier_return_flow",
		},
		OwnedTemplateKeys: []string{
			"supplier_returns.supplier_return.print.default",
		},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "procurement_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "inventory_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Supplier Returns Console",
			TitleI18n:       localize("Supplier Returns Console", "Konsol Retur Vendor"),
			Description:     "Return-to-vendor operations across procurement, inventory, and AP credit flows.",
			DescriptionI18n: localize("Return-to-vendor operations across procurement, inventory, and AP credit flows.", "Operasi retur ke vendor lintas pengadaan, inventori, dan alur kredit AP."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "operations",
					Title:     "Return Operations",
					TitleI18n: localize("Return Operations", "Operasi Retur"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("supplier_returns", "Supplier Returns", "Retur Vendor", "/ui/supplier-returns/returns", "Open supplier returns.", "Buka retur vendor.", "document.list"),
						adminConsoleLink("receipts", "Goods Receipts", "Penerimaan", "/ui/procurement/receipts", "Open receipts to register returns.", "Buka penerimaan untuk mendaftarkan retur.", "document.list"),
						adminConsoleLink("bills", "Vendor Bills", "Tagihan Vendor", "/ui/procurement/bills", "Open vendor bills to register returns.", "Buka tagihan vendor untuk mendaftarkan retur.", "document.list"),
						adminConsoleLink("credits", "Vendor Credits", "Nota Kredit Vendor", "/ui/procurement/credits", "Open vendor credits.", "Buka nota kredit vendor.", "document.list"),
					},
				},
				{
					Key:       "workflows",
					Title:     "Return Workflows",
					TitleI18n: localize("Return Workflows", "Workflow Retur"),
					Kind:      module.AdminConsoleSectionWorkflowLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("supplier_return_flow", "Supplier Return Workflow", "Workflow Retur Vendor", "/admin/workflows/designer?key=supplier_return_flow", "Open the supplier return workflow.", "Buka workflow retur vendor.", "workflow.read"),
					},
				},
			},
		},
		Documents: []document.Definition{
			{
				Type:                   "supplier_return",
				DisplayName:            "Supplier Return",
				SchemaVersion:          "v1",
				WorkflowKey:            "supplier_return_flow",
				NumberingKey:           "supplier_return_number",
				OwnerModuleKey:         "supplier_returns_core",
				AllowedLinkTypes:       []string{"return_for", "credit_for", "movement_for", "related_to"},
				AllowedAttachmentTypes: []string{"note", "image", "document"},
			},
		},
		Workflows: []workflow.Definition{
			{
				Key:    "supplier_return_flow",
				States: []string{"draft", "submitted", "approved", "rejected", "cancelled"},
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
		},
		SearchIndexes: []search.IndexDefinition{
			commercialDocumentSearchIndex("supplier_returns.returns.search", "Supplier Return Search", "supplier_return", "supplier_returns.returns.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "vendor_name", Path: "body.payload.vendor_name", Type: "string", Searchable: true},
				{Key: "return_date", Path: "body.payload.return_date", Type: "string", Sort: true},
				{Key: "source_goods_receipt_number", Path: "body.payload.source_goods_receipt_number", Type: "string", Searchable: true},
				{Key: "source_vendor_bill_number", Path: "body.payload.source_vendor_bill_number", Type: "string", Searchable: true},
			}),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "supplier_return.read", Action: "read", Resource: "supplier_return", DisplayName: "Read Supplier Returns", DisplayNameI18n: localize("Read Supplier Returns", "Lihat Retur Vendor")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "supplier_returns_operator",
					Name:           "Supplier Returns Operator",
					NameI18n:       localize("Supplier Returns Operator", "Operator Retur Vendor"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"document.create", "document.list", "document.read", "document.update_draft", "document.submit", "document.approve", "document.reject", "document.reopen", "document.cancel"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "supplier_returns.returns", Label: "Supplier Returns", LabelI18n: localize("Supplier Returns", "Retur Vendor"), ActionKey: "supplier_returns.returns.list", Order: 49, RequiredPermissions: []string{"document.list"}},
			},
			Actions: commercialDocumentActions("supplier_returns.returns", "supplier_return", "Supplier Returns", "Supplier Return", "New Supplier Return", "/supplier-returns/returns"),
			Views:   commercialDocumentViews("supplier_returns.returns", "supplier_return", "Supplier Returns", "Supplier Return Detail", "Supplier Return Draft", supplierReturnColumns(), []string{"draft", "submitted", "approved", "rejected", "cancelled"}, supplierReturnSections(), supplierReturnFormSections()),
		},
		Templates: []module.TemplateDefinition{
			commercialTemplateDefinition("supplier_returns.supplier_return.print.default", "Supplier Return Print", "supplier_return", "Supplier Return", []string{"vendor_name", "return_date", "source_goods_receipt_number", "source_vendor_bill_number", "lines"}),
		},
	}
}

func supplierReturnColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Return", LabelI18n: localize("Return", "Retur"), Path: "header.number"},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name"},
		{Key: "return_date", Label: "Return Date", LabelI18n: localize("Return Date", "Tanggal Retur"), Path: "body.payload.return_date"},
		{Key: "source_goods_receipt_number", Label: "Source Receipt", LabelI18n: localize("Source Receipt", "Penerimaan Sumber"), Path: "body.payload.source_goods_receipt_number"},
		{Key: "source_vendor_bill_number", Label: "Source Bill", LabelI18n: localize("Source Bill", "Tagihan Sumber"), Path: "body.payload.source_vendor_bill_number"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func supplierReturnSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Supplier Return Summary", TitleI18n: localize("Supplier Return Summary", "Ringkasan Retur Vendor"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string", ReadOnly: true},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name", Type: "string"},
		{Key: "return_date", Label: "Return Date", LabelI18n: localize("Return Date", "Tanggal Retur"), Path: "body.payload.return_date", Type: "string"},
		{Key: "source_goods_receipt_number", Label: "Source Receipt", LabelI18n: localize("Source Receipt", "Penerimaan Sumber"), Path: "body.payload.source_goods_receipt_number", Type: "string"},
		{Key: "source_vendor_bill_number", Label: "Source Bill", LabelI18n: localize("Source Bill", "Tagihan Sumber"), Path: "body.payload.source_vendor_bill_number", Type: "string"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string"},
		{Key: "credit_note_status", Label: "Credit Status", LabelI18n: localize("Credit Status", "Status Kredit"), Path: "body.payload.credit_note_status", Type: "string"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "supplier_return_lines"},
	}}}
}

func supplierReturnFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Supplier Return Draft", TitleI18n: localize("Supplier Return Draft", "Draft Retur Vendor"), Fields: []module.FieldDefinition{
		{Key: "vendor_id", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_id", Type: "string", Widget: "select"},
		{Key: "vendor_name", Label: "Vendor Name", LabelI18n: localize("Vendor Name", "Nama Vendor"), Path: "body.payload.vendor_name", Type: "string", Widget: "text"},
		{Key: "return_date", Label: "Return Date", LabelI18n: localize("Return Date", "Tanggal Retur"), Path: "body.payload.return_date", Type: "string", Widget: "text"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "body.payload.warehouse_code", Type: "string", Widget: "select"},
		{Key: "source_goods_receipt_number", Label: "Source Receipt", LabelI18n: localize("Source Receipt", "Penerimaan Sumber"), Path: "body.payload.source_goods_receipt_number", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "source_vendor_bill_number", Label: "Source Bill", LabelI18n: localize("Source Bill", "Tagihan Sumber"), Path: "body.payload.source_vendor_bill_number", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "reason", Label: "Reason", LabelI18n: localize("Reason", "Alasan"), Path: "body.payload.reason", Type: "string", Widget: "textarea"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "supplier_return_lines"},
	}}}
}
