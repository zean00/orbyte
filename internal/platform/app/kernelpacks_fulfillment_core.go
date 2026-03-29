package app

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func fulfillmentCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{fulfillmentCoreKernelPackManifest()}
}

func fulfillmentCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "fulfillment_core",
		Name:         "Fulfillment Core",
		NameI18n:     localize("Fulfillment Core", "Inti Fulfillment"),
		Version:      "1.0.0",
		DomainFamily: "business",
		OwnedDocumentTypes: []string{
			"sales_fulfillment",
		},
		OwnedWorkflowKeys: []string{
			"sales_fulfillment_flow",
		},
		OwnedTemplateKeys: []string{
			"fulfillment.sales_fulfillment.print.default",
		},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "commercial_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "inventory_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Fulfillment Console",
			TitleI18n:       localize("Fulfillment Console", "Konsol Fulfillment"),
			Description:     "Sales fulfillment operations and fulfillment workflow shortcuts.",
			DescriptionI18n: localize("Sales fulfillment operations and fulfillment workflow shortcuts.", "Operasi fulfillment penjualan dan pintasan workflow fulfillment."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "operations",
					Title:     "Fulfillment Operations",
					TitleI18n: localize("Fulfillment Operations", "Operasi Fulfillment"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("fulfillments", "Fulfillments", "Fulfillment", "/ui/fulfillment/fulfillments", "Open sales fulfillments.", "Buka fulfillment penjualan.", "document.list"),
						adminConsoleLink("orders", "Orders", "Order", "/ui/commercial/orders", "Open sales orders to generate fulfillments.", "Buka order penjualan untuk membuat fulfillment.", "document.list"),
					},
				},
				{
					Key:       "workflows",
					Title:     "Fulfillment Workflows",
					TitleI18n: localize("Fulfillment Workflows", "Workflow Fulfillment"),
					Kind:      module.AdminConsoleSectionWorkflowLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("sales_fulfillment_flow", "Sales Fulfillment Workflow", "Workflow Fulfillment Penjualan", "/admin/workflows/designer?key=sales_fulfillment_flow", "Open the sales fulfillment workflow.", "Buka workflow fulfillment penjualan.", "workflow.read"),
					},
				},
				{
					Key:       "templates",
					Title:     "Fulfillment Templates",
					TitleI18n: localize("Fulfillment Templates", "Template Fulfillment"),
					Kind:      module.AdminConsoleSectionTemplateLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("sales_fulfillment_template", "Sales Fulfillment Print", "Cetak Fulfillment Penjualan", "/admin/templates/designer?key=fulfillment.sales_fulfillment.print.default", "Open the sales fulfillment print template.", "Buka template cetak fulfillment penjualan.", "configuration.read"),
					},
				},
			},
		},
		Documents: []document.Definition{
			{
				Type:                   "sales_fulfillment",
				DisplayName:            "Sales Fulfillment",
				SchemaVersion:          "v1",
				WorkflowKey:            "sales_fulfillment_flow",
				NumberingKey:           "sales_fulfillment_number",
				OwnerModuleKey:         "fulfillment_core",
				AllowedLinkTypes:       []string{"fulfillment_for", "movement_for", "delivery_for", "related_to", "return_for", "exchange_for"},
				AllowedAttachmentTypes: []string{"note", "image", "document"},
			},
		},
		Workflows: []workflow.Definition{
			{
				Key:    "sales_fulfillment_flow",
				States: []string{"draft", "submitted", "issued", "rejected", "cancelled"},
				Actions: []workflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
					{Action: "approve", FromState: "submitted", ToState: "issued", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
		},
		SearchIndexes: []search.IndexDefinition{
			commercialDocumentSearchIndex("fulfillment.fulfillments.search", "Sales Fulfillment Search", "sales_fulfillment", "fulfillment.fulfillments.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "party_name", Path: "body.payload.party_name", Type: "string", Searchable: true},
				{Key: "source_order_number", Path: "body.payload.source_order_number", Type: "string", Searchable: true},
				{Key: "fulfillment_date", Path: "body.payload.fulfillment_date", Type: "string", Sort: true},
				{Key: "reserved_quantity_total", Path: "body.payload.reserved_quantity_total", Type: "number", Sort: true},
			}),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "fulfillment.read", Action: "read", Resource: "fulfillment", DisplayName: "Read Fulfillments", DisplayNameI18n: localize("Read Fulfillments", "Lihat Fulfillment")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "fulfillment_operator",
					Name:           "Fulfillment Operator",
					NameI18n:       localize("Fulfillment Operator", "Operator Fulfillment"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"document.create", "document.list", "document.read", "document.update_draft", "document.submit", "document.approve", "document.reject", "document.reopen", "document.cancel"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "fulfillment.fulfillments", Label: "Fulfillments", LabelI18n: localize("Fulfillments", "Fulfillment"), ActionKey: "fulfillment.fulfillments.list", Order: 37, RequiredPermissions: []string{"document.list"}},
			},
			Actions: commercialDocumentActions("fulfillment.fulfillments", "sales_fulfillment", "Fulfillments", "Fulfillment", "New Fulfillment", "/fulfillment/fulfillments"),
			Views:   fulfillmentDocumentViews(),
		},
		Templates: []module.TemplateDefinition{
			commercialTemplateDefinition("fulfillment.sales_fulfillment.print.default", "Sales Fulfillment Print", "sales_fulfillment", "Sales Fulfillment", []string{"source_order_number", "party_name", "fulfillment_date", "lines"}),
		},
	}
}

func fulfillmentDocumentViews() []module.ViewDefinition {
	views := commercialDocumentViews(
		"fulfillment.fulfillments",
		"sales_fulfillment",
		"Fulfillments",
		"Fulfillment Detail",
		"Fulfillment Draft",
		[]module.ColumnDefinition{
			{Key: "number", Label: "Fulfillment", LabelI18n: localize("Fulfillment", "Fulfillment"), Path: "header.number"},
			{Key: "source_order_number", Label: "Order", LabelI18n: localize("Order", "Order"), Path: "body.payload.source_order_number"},
			{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name"},
			{Key: "reserved_quantity_total", Label: "Reserved", LabelI18n: localize("Reserved", "Direservasi"), Path: "body.payload.reserved_quantity_total"},
			{Key: "fulfilled_quantity_total", Label: "Issued", LabelI18n: localize("Issued", "Dikeluarkan"), Path: "body.payload.fulfilled_quantity_total"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
		},
		[]string{"draft", "submitted", "issued", "rejected", "cancelled"},
		fulfillmentDetailSections(),
		fulfillmentFormSections(),
	)
	if len(views) > 1 {
		views[1].AllowedActions = append(commercialDetailActions("sales_fulfillment"), "register_delivery", "register_return")
	}
	return views
}

func fulfillmentDetailSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Fulfillment Summary", TitleI18n: localize("Fulfillment Summary", "Ringkasan Fulfillment"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "source_order_number", Label: "Order", LabelI18n: localize("Order", "Order"), Path: "body.payload.source_order_number", Type: "string"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string"},
		{Key: "fulfillment_date", Label: "Fulfillment Date", LabelI18n: localize("Fulfillment Date", "Tanggal Fulfillment"), Path: "body.payload.fulfillment_date", Type: "string"},
		{Key: "fulfillment_status", Label: "Fulfillment Status", LabelI18n: localize("Fulfillment Status", "Status Fulfillment"), Path: "body.payload.fulfillment_status", Type: "string"},
		{Key: "reserved_quantity_total", Label: "Reserved Quantity", LabelI18n: localize("Reserved Quantity", "Jumlah Reservasi"), Path: "body.payload.reserved_quantity_total", Type: "number"},
		{Key: "fulfilled_quantity_total", Label: "Issued Quantity", LabelI18n: localize("Issued Quantity", "Jumlah Dikeluarkan"), Path: "body.payload.fulfilled_quantity_total", Type: "number"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "fulfillment_lines"},
	}}}
}

func fulfillmentFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Fulfillment Draft", TitleI18n: localize("Fulfillment Draft", "Draft Fulfillment"), Fields: []module.FieldDefinition{
		{Key: "source_order_number", Label: "Order", LabelI18n: localize("Order", "Order"), Path: "body.payload.source_order_number", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "fulfillment_date", Label: "Fulfillment Date", LabelI18n: localize("Fulfillment Date", "Tanggal Fulfillment"), Path: "body.payload.fulfillment_date", Type: "string", Widget: "text"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "fulfillment_lines", HelpText: "Reserved lines are generated from the sales order and can be adjusted before issue."},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}
