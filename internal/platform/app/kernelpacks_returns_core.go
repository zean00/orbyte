package app

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func returnsCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{returnsCoreKernelPackManifest()}
}

func returnsCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "returns_core",
		Name:         "Returns Core",
		NameI18n:     localize("Returns Core", "Inti Retur"),
		Version:      "1.0.0",
		DomainFamily: "business",
		OwnedDocumentTypes: []string{
			"sales_return",
			"return_receipt",
		},
		OwnedWorkflowKeys: []string{
			"sales_return_flow",
			"return_receipt_flow",
		},
		OwnedTemplateKeys: []string{
			"returns.sales_return.print.default",
			"returns.return_receipt.print.default",
		},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "commercial_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "inventory_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "fulfillment_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Returns Console",
			TitleI18n:       localize("Returns Console", "Konsol Retur"),
			Description:     "Customer return operations, receipts, and reverse logistics shortcuts.",
			DescriptionI18n: localize("Customer return operations, receipts, and reverse logistics shortcuts.", "Operasi retur pelanggan, penerimaan retur, dan pintasan logistik balik."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "operations",
					Title:     "Return Operations",
					TitleI18n: localize("Return Operations", "Operasi Retur"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("returns", "Returns", "Retur", "/ui/returns/returns", "Open customer returns.", "Buka retur pelanggan.", "document.list"),
						adminConsoleLink("receipts", "Return Receipts", "Penerimaan Retur", "/ui/returns/receipts", "Open warehouse return receipts.", "Buka penerimaan retur gudang.", "document.list"),
						adminConsoleLink("fulfillments", "Fulfillments", "Fulfillment", "/ui/fulfillment/fulfillments", "Open fulfillments to register returns.", "Buka fulfillment untuk mendaftarkan retur.", "document.list"),
					},
				},
				{
					Key:       "workflows",
					Title:     "Return Workflows",
					TitleI18n: localize("Return Workflows", "Workflow Retur"),
					Kind:      module.AdminConsoleSectionWorkflowLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("sales_return_flow", "Sales Return Workflow", "Workflow Retur Penjualan", "/admin/workflows/designer?key=sales_return_flow", "Open the sales return workflow.", "Buka workflow retur penjualan.", "workflow.read"),
						adminConsoleLink("return_receipt_flow", "Return Receipt Workflow", "Workflow Penerimaan Retur", "/admin/workflows/designer?key=return_receipt_flow", "Open the return receipt workflow.", "Buka workflow penerimaan retur.", "workflow.read"),
					},
				},
			},
		},
		Documents: []document.Definition{
			returnsDocumentDefinition("sales_return", "Sales Return", "sales_return_flow", "sales_return_number"),
			returnsDocumentDefinition("return_receipt", "Return Receipt", "return_receipt_flow", "return_receipt_number"),
		},
		Workflows: []workflow.Definition{
			{
				Key:    "sales_return_flow",
				States: []string{"draft", "submitted", "approved", "received", "rejected", "cancelled"},
				Actions: []workflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
					{Action: "approve", FromState: "submitted", ToState: "approved", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
			{
				Key:    "return_receipt_flow",
				States: []string{"draft", "submitted", "received", "rejected", "cancelled"},
				Actions: []workflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
					{Action: "approve", FromState: "submitted", ToState: "received", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "received", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
		},
		SearchIndexes: []search.IndexDefinition{
			commercialDocumentSearchIndex("returns.sales_returns.search", "Sales Return Search", "sales_return", "returns.returns.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "party_name", Path: "body.payload.party_name", Type: "string", Searchable: true},
				{Key: "source_fulfillment_number", Path: "body.payload.source_fulfillment_number", Type: "string", Searchable: true},
				{Key: "return_date", Path: "body.payload.return_date", Type: "string", Sort: true},
			}),
			commercialDocumentSearchIndex("returns.return_receipts.search", "Return Receipt Search", "return_receipt", "returns.receipts.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "party_name", Path: "body.payload.party_name", Type: "string", Searchable: true},
				{Key: "source_return_number", Path: "body.payload.source_return_number", Type: "string", Searchable: true},
				{Key: "receipt_date", Path: "body.payload.receipt_date", Type: "string", Sort: true},
			}),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "returns.read", Action: "read", Resource: "returns", DisplayName: "Read Returns", DisplayNameI18n: localize("Read Returns", "Lihat Retur")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "returns_operator",
					Name:           "Returns Operator",
					NameI18n:       localize("Returns Operator", "Operator Retur"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"document.create", "document.list", "document.read", "document.update_draft", "document.submit", "document.approve", "document.reject", "document.reopen", "document.cancel"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "returns.sales_returns", Label: "Returns", LabelI18n: localize("Returns", "Retur"), ActionKey: "returns.returns.list", Order: 38, RequiredPermissions: []string{"document.list"}},
				{Key: "returns.return_receipts", Label: "Return Receipts", LabelI18n: localize("Return Receipts", "Penerimaan Retur"), ActionKey: "returns.receipts.list", Order: 39, RequiredPermissions: []string{"document.list"}},
			},
			Actions: append(
				returnsDocumentActions("returns.returns", "sales_return", "Returns", "Return", "New Return", "/returns/returns"),
				returnsDocumentActions("returns.receipts", "return_receipt", "Return Receipts", "Return Receipt", "New Return Receipt", "/returns/receipts")...,
			),
			Views: append(
				returnsDocumentViews("returns.returns", "sales_return", "Returns", "Return Detail", "Return Draft", returnColumns(), []string{"draft", "submitted", "approved", "received", "rejected", "cancelled"}, returnDetailSections(), returnFormSections()),
				returnsDocumentViews("returns.receipts", "return_receipt", "Return Receipts", "Return Receipt Detail", "Return Receipt Draft", returnReceiptColumns(), []string{"draft", "submitted", "received", "rejected", "cancelled"}, returnReceiptDetailSections(), returnReceiptFormSections())...,
			),
		},
		Templates: []module.TemplateDefinition{
			commercialTemplateDefinition("returns.sales_return.print.default", "Sales Return Print", "sales_return", "Sales Return", []string{"party_name", "return_date", "source_fulfillment_number", "lines"}),
			commercialTemplateDefinition("returns.return_receipt.print.default", "Return Receipt Print", "return_receipt", "Return Receipt", []string{"party_name", "receipt_date", "source_return_number", "lines"}),
		},
	}
}

func returnsDocumentDefinition(documentType, displayName, workflowKey, numberingKey string) document.Definition {
	return document.Definition{
		Type:                   documentType,
		DisplayName:            displayName,
		SchemaVersion:          "v1",
		WorkflowKey:            workflowKey,
		NumberingKey:           numberingKey,
		OwnerModuleKey:         "returns_core",
		AllowedLinkTypes:       []string{"return_for", "movement_for", "refund_for", "invoice_for", "fulfillment_for", "delivery_for", "exchange_for", "related_to"},
		AllowedAttachmentTypes: []string{"note", "image", "document"},
	}
}

func returnsDocumentActions(prefix, documentType, listLabel, detailLabel, newLabel, basePath string) []module.ActionDefinition {
	return commercialDocumentActions(prefix, documentType, listLabel, detailLabel, newLabel, basePath)
}

func returnsDocumentViews(prefix, documentType, listTitle, detailTitle, formTitle string, columns []module.ColumnDefinition, statusOptions []string, detailSections, formSections []module.SectionDefinition) []module.ViewDefinition {
	views := commercialDocumentViews(prefix, documentType, listTitle, detailTitle, formTitle, columns, statusOptions, detailSections, formSections)
	if len(views) > 1 {
		views[1].AllowedActions = returnsDetailActions(documentType)
	}
	return views
}

func returnsDetailActions(documentType string) []string {
	actions := []string{"submit", "approve", "reject", "reopen", "cancel"}
	switch documentType {
	case "sales_return":
		return append(actions, "register_return_receipt", "issue_credit_note", "register_refund", "create_replacement_order")
	default:
		return actions
	}
}

func returnColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Return", LabelI18n: localize("Return", "Retur"), Path: "header.number"},
		{Key: "source_fulfillment_number", Label: "Fulfillment", LabelI18n: localize("Fulfillment", "Fulfillment"), Path: "body.payload.source_fulfillment_number"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name"},
		{Key: "total_quantity", Label: "Quantity", LabelI18n: localize("Quantity", "Jumlah"), Path: "body.payload.total_quantity"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func returnReceiptColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Receipt", LabelI18n: localize("Receipt", "Penerimaan"), Path: "header.number"},
		{Key: "source_return_number", Label: "Return", LabelI18n: localize("Return", "Retur"), Path: "body.payload.source_return_number"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name"},
		{Key: "total_quantity", Label: "Quantity", LabelI18n: localize("Quantity", "Jumlah"), Path: "body.payload.total_quantity"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func returnDetailSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Return Summary", TitleI18n: localize("Return Summary", "Ringkasan Retur"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "source_fulfillment_number", Label: "Fulfillment", LabelI18n: localize("Fulfillment", "Fulfillment"), Path: "body.payload.source_fulfillment_number", Type: "string"},
		{Key: "source_invoice_number", Label: "Invoice", LabelI18n: localize("Invoice", "Faktur"), Path: "body.payload.source_invoice_number", Type: "string"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string"},
		{Key: "return_date", Label: "Return Date", LabelI18n: localize("Return Date", "Tanggal Retur"), Path: "body.payload.return_date", Type: "string"},
		{Key: "return_status", Label: "Return Status", LabelI18n: localize("Return Status", "Status Retur"), Path: "body.payload.return_status", Type: "string"},
		{Key: "resolution_type", Label: "Resolution", LabelI18n: localize("Resolution", "Resolusi"), Path: "body.payload.resolution_type", Type: "string"},
		{Key: "credit_note_status", Label: "Credit Note Status", LabelI18n: localize("Credit Note Status", "Status Nota Kredit"), Path: "body.payload.credit_note_status", Type: "string"},
		{Key: "refund_status", Label: "Refund Status", LabelI18n: localize("Refund Status", "Status Refund"), Path: "body.payload.refund_status", Type: "string"},
		{Key: "replacement_order_status", Label: "Replacement Order Status", LabelI18n: localize("Replacement Order Status", "Status Order Pengganti"), Path: "body.payload.replacement_order_status", Type: "string"},
		{Key: "source_replacement_order_number", Label: "Replacement Order", LabelI18n: localize("Replacement Order", "Order Pengganti"), Path: "body.payload.source_replacement_order_number", Type: "string"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "return_lines"},
	}}}
}

func returnFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Return Draft", TitleI18n: localize("Return Draft", "Draft Retur"), Fields: []module.FieldDefinition{
		{Key: "source_fulfillment_number", Label: "Fulfillment", LabelI18n: localize("Fulfillment", "Fulfillment"), Path: "body.payload.source_fulfillment_number", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "source_invoice_number", Label: "Invoice", LabelI18n: localize("Invoice", "Faktur"), Path: "body.payload.source_invoice_number", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "return_date", Label: "Return Date", LabelI18n: localize("Return Date", "Tanggal Retur"), Path: "body.payload.return_date", Type: "string", Widget: "text"},
		{Key: "resolution_type", Label: "Resolution", LabelI18n: localize("Resolution", "Resolusi"), Path: "body.payload.resolution_type", Type: "string", Widget: "select", Options: []string{"refund", "exchange"}},
		{Key: "reason", Label: "Reason", LabelI18n: localize("Reason", "Alasan"), Path: "body.payload.reason", Type: "string", Widget: "textarea"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "return_lines"},
	}}}
}

func returnReceiptDetailSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Return Receipt Summary", TitleI18n: localize("Return Receipt Summary", "Ringkasan Penerimaan Retur"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "source_return_number", Label: "Return", LabelI18n: localize("Return", "Retur"), Path: "body.payload.source_return_number", Type: "string"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string"},
		{Key: "receipt_date", Label: "Receipt Date", LabelI18n: localize("Receipt Date", "Tanggal Penerimaan"), Path: "body.payload.receipt_date", Type: "string"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "return_lines"},
	}}}
}

func returnReceiptFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Return Receipt Draft", TitleI18n: localize("Return Receipt Draft", "Draft Penerimaan Retur"), Fields: []module.FieldDefinition{
		{Key: "source_return_number", Label: "Return", LabelI18n: localize("Return", "Retur"), Path: "body.payload.source_return_number", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "receipt_date", Label: "Receipt Date", LabelI18n: localize("Receipt Date", "Tanggal Penerimaan"), Path: "body.payload.receipt_date", Type: "string", Widget: "text"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "return_lines"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}
