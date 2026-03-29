package app

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func procurementCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "procurement_core",
		Name:         "Procurement Core",
		NameI18n:     localize("Procurement Core", "Inti Pengadaan"),
		Version:      "1.0.0",
		DomainFamily: "business",
		OwnedDocumentTypes: []string{
			"purchase_request",
			"purchase_order",
			"goods_receipt",
			"vendor_bill",
			"payment_out",
			"vendor_credit_note",
		},
		OwnedWorkflowKeys: []string{
			"purchase_request_flow",
			"purchase_order_flow",
			"goods_receipt_flow",
			"vendor_bill_flow",
			"payment_out_flow",
			"vendor_credit_note_flow",
		},
		OwnedTemplateKeys: []string{
			"procurement.purchase_request.print.default",
			"procurement.purchase_order.print.default",
			"procurement.goods_receipt.print.default",
			"procurement.vendor_bill.print.default",
			"procurement.payment_out.print.default",
			"procurement.vendor_credit.print.default",
			"procurement.vendor_statement.print.default",
		},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "commercial_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Procurement Console",
			TitleI18n:       localize("Procurement Console", "Konsol Pengadaan"),
			Description:     "Buy-side setup, AP operations, workflows, and templates.",
			DescriptionI18n: localize("Buy-side setup, AP operations, workflows, and templates.", "Pengaturan sisi beli, operasi AP, workflow, dan template."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:                 "posting_defaults",
					Title:               "AP Posting Defaults",
					TitleI18n:           localize("AP Posting Defaults", "Default Posting AP"),
					Kind:                module.AdminConsoleSectionSettingsForm,
					ConfigKey:           "procurement.posting",
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:       "vendor_setup",
					Title:     "Vendor Setup",
					TitleI18n: localize("Vendor Setup", "Pengaturan Vendor"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("vendors", "Vendors", "Vendor", "/ui/procurement/vendors", "Manage vendor profiles.", "Kelola profil vendor.", "vendor.list"),
						adminConsoleLink("vendor_items", "Vendor Items", "Item Vendor", "/ui/procurement/vendor-items", "Manage supplier defaults per SKU.", "Kelola default pemasok per SKU.", "vendor_item.list"),
						adminConsoleLink("parties", "Parties", "Pihak", "/ui/masterdata/parties", "Open linked party master records.", "Buka data master pihak terkait.", "party.list"),
						adminConsoleLink("catalog", "Catalog", "Katalog", "/ui/commercial/catalog", "Open reused item/service catalog.", "Buka katalog item/layanan yang digunakan ulang.", "item.list"),
						adminConsoleLink("payment_methods", "Payment Methods", "Metode Pembayaran", "/ui/commercial/payment-methods", "Manage settlement defaults.", "Kelola default settlement.", "payment_method.list"),
					},
				},
				{
					Key:       "operations",
					Title:     "Procurement Operations",
					TitleI18n: localize("Procurement Operations", "Operasi Pengadaan"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("requests", "Purchase Requests", "Permintaan Pembelian", "/ui/procurement/requests", "Open purchase requests.", "Buka permintaan pembelian.", "document.list"),
						adminConsoleLink("orders", "Purchase Orders", "Pesanan Pembelian", "/ui/procurement/orders", "Open purchase orders.", "Buka pesanan pembelian.", "document.list"),
						adminConsoleLink("receipts", "Receipts", "Penerimaan", "/ui/procurement/receipts", "Open goods receipts.", "Buka penerimaan barang.", "document.list"),
						adminConsoleLink("bills", "Vendor Bills", "Tagihan Vendor", "/ui/procurement/bills", "Open vendor bills.", "Buka tagihan vendor.", "document.list"),
						adminConsoleLink("payments", "Payments Out", "Pembayaran Keluar", "/ui/procurement/payments", "Open outgoing payments.", "Buka pembayaran keluar.", "document.list"),
						adminConsoleLink("credits", "Vendor Credits", "Nota Kredit Vendor", "/ui/procurement/credits", "Open vendor credits.", "Buka nota kredit vendor.", "document.list"),
						adminConsoleLink("payables", "Payables", "Utang", "/ui/procurement/payables", "Open payables dashboard.", "Buka dashboard utang.", "document.list"),
					},
				},
				{
					Key:       "workflows",
					Title:     "Procurement Workflows",
					TitleI18n: localize("Procurement Workflows", "Workflow Pengadaan"),
					Kind:      module.AdminConsoleSectionWorkflowLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("purchase_request_flow", "Purchase Request Flow", "Workflow Permintaan Pembelian", "/admin/workflows/designer?key=purchase_request_flow", "Review the PR lifecycle.", "Tinjau siklus hidup PR.", "workflow.read"),
						adminConsoleLink("purchase_order_flow", "Purchase Order Flow", "Workflow Pesanan Pembelian", "/admin/workflows/designer?key=purchase_order_flow", "Review the PO lifecycle.", "Tinjau siklus hidup PO.", "workflow.read"),
						adminConsoleLink("goods_receipt_flow", "Goods Receipt Flow", "Workflow Penerimaan", "/admin/workflows/designer?key=goods_receipt_flow", "Review receipt lifecycle.", "Tinjau siklus hidup penerimaan.", "workflow.read"),
						adminConsoleLink("vendor_bill_flow", "Vendor Bill Flow", "Workflow Tagihan Vendor", "/admin/workflows/designer?key=vendor_bill_flow", "Review AP bill lifecycle.", "Tinjau siklus hidup tagihan AP.", "workflow.read"),
						adminConsoleLink("payment_out_flow", "Payment Out Flow", "Workflow Pembayaran Keluar", "/admin/workflows/designer?key=payment_out_flow", "Review payment out lifecycle.", "Tinjau siklus hidup pembayaran keluar.", "workflow.read"),
						adminConsoleLink("vendor_credit_note_flow", "Vendor Credit Flow", "Workflow Nota Kredit Vendor", "/admin/workflows/designer?key=vendor_credit_note_flow", "Review vendor credit lifecycle.", "Tinjau siklus hidup nota kredit vendor.", "workflow.read"),
					},
				},
				{
					Key:       "templates",
					Title:     "Procurement Templates",
					TitleI18n: localize("Procurement Templates", "Template Pengadaan"),
					Kind:      module.AdminConsoleSectionTemplateLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("purchase_order_template", "Purchase Order Print", "Cetak Pesanan Pembelian", "/admin/templates/designer?key=procurement.purchase_order.print.default", "Manage purchase-order print template.", "Kelola template cetak pesanan pembelian.", "template.read"),
						adminConsoleLink("vendor_bill_template", "Vendor Bill Print", "Cetak Tagihan Vendor", "/admin/templates/designer?key=procurement.vendor_bill.print.default", "Manage vendor-bill print template.", "Kelola template cetak tagihan vendor.", "template.read"),
						adminConsoleLink("payment_out_template", "Payment Out Print", "Cetak Pembayaran Keluar", "/admin/templates/designer?key=procurement.payment_out.print.default", "Manage payment-out print template.", "Kelola template cetak pembayaran keluar.", "template.read"),
						adminConsoleLink("vendor_statement_template", "Vendor Statement Print", "Cetak Laporan Vendor", "/admin/templates/designer?key=procurement.vendor_statement.print.default", "Manage vendor statement template.", "Kelola template laporan vendor.", "template.read"),
					},
				},
			},
		},
		Models: []model.Definition{
			{
				Key:                 "vendor_profile",
				DisplayName:         "Vendor Profile",
				DisplayNameI18n:     localize("Vendor Profile", "Profil Vendor"),
				OwnerModuleKey:      "procurement_core",
				Version:             "v1",
				CreatePermissionKey: "vendor.create",
				ListPermissionKey:   "vendor.list",
				ReadPermissionKey:   "vendor.read",
				UpdatePermissionKey: "vendor.update",
				DefaultSort:         "vendor_name",
				Fields: []model.FieldDefinition{
					{Key: "party_id", Label: "Party", LabelI18n: localize("Party", "Pihak"), Type: "string", Required: true},
					{Key: "vendor_name", Label: "Vendor Name", LabelI18n: localize("Vendor Name", "Nama Vendor"), Type: "string"},
					{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Type: "string"},
					{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Type: "string"},
					{Key: "payment_term_days", Label: "Payment Term Days", LabelI18n: localize("Payment Term Days", "Hari Termin Pembayaran"), Type: "number"},
					{Key: "payable_account_code", Label: "Payable Account", LabelI18n: localize("Payable Account", "Akun Utang"), Type: "string"},
					{Key: "expense_account_code", Label: "Expense Account", LabelI18n: localize("Expense Account", "Akun Beban"), Type: "string"},
					{Key: "default_payment_method_code", Label: "Default Payment Method", LabelI18n: localize("Default Payment Method", "Metode Pembayaran Default"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			},
			{
				Key:                 "vendor_item_profile",
				DisplayName:         "Vendor Item Profile",
				DisplayNameI18n:     localize("Vendor Item Profile", "Profil Item Vendor"),
				OwnerModuleKey:      "procurement_core",
				Version:             "v1",
				CreatePermissionKey: "vendor_item.create",
				ListPermissionKey:   "vendor_item.list",
				ReadPermissionKey:   "vendor_item.read",
				UpdatePermissionKey: "vendor_item.update",
				DefaultSort:         "item_code",
				Fields: []model.FieldDefinition{
					{Key: "vendor_id", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Type: "string", Required: true},
					{Key: "vendor_name", Label: "Vendor Name", LabelI18n: localize("Vendor Name", "Nama Vendor"), Type: "string"},
					{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Type: "string", Required: true},
					{Key: "vendor_item_code", Label: "Vendor Item Code", LabelI18n: localize("Vendor Item Code", "Kode Item Vendor"), Type: "string"},
					{Key: "preferred", Label: "Preferred", LabelI18n: localize("Preferred", "Utama"), Type: "bool"},
					{Key: "purchase_uom_code", Label: "Purchase UOM", LabelI18n: localize("Purchase UOM", "Satuan Beli"), Type: "string"},
					{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Type: "number"},
					{Key: "lead_time_days", Label: "Lead Time Days", LabelI18n: localize("Lead Time Days", "Hari Lead Time"), Type: "number"},
					{Key: "minimum_order_quantity", Label: "MOQ", LabelI18n: localize("MOQ", "MOQ"), Type: "number"},
					{Key: "pack_size", Label: "Pack Size", LabelI18n: localize("Pack Size", "Ukuran Kemasan"), Type: "number"},
					{Key: "last_purchase_price", Label: "Last Purchase Price", LabelI18n: localize("Last Purchase Price", "Harga Beli Terakhir"), Type: "number"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			},
		},
		Documents: []document.Definition{
			procurementDocumentDefinition("purchase_request", "Purchase Request", "purchase_request_flow", "purchase_request_number"),
			procurementDocumentDefinition("purchase_order", "Purchase Order", "purchase_order_flow", "purchase_order_number"),
			procurementDocumentDefinition("goods_receipt", "Goods Receipt", "goods_receipt_flow", "goods_receipt_number"),
			procurementDocumentDefinition("vendor_bill", "Vendor Bill", "vendor_bill_flow", "vendor_bill_number"),
			procurementDocumentDefinition("payment_out", "Payment Out", "payment_out_flow", "payment_out_number"),
			procurementDocumentDefinition("vendor_credit_note", "Vendor Credit Note", "vendor_credit_note_flow", "vendor_credit_note_number"),
		},
		Workflows: []workflow.Definition{
			procurementWorkflowDefinition("purchase_request_flow", "approved", true),
			procurementWorkflowDefinition("purchase_order_flow", "approved", true),
			procurementWorkflowDefinition("goods_receipt_flow", "received", true),
			procurementWorkflowDefinition("vendor_bill_flow", "issued", true),
			procurementWorkflowDefinition("payment_out_flow", "paid", true),
			procurementWorkflowDefinition("vendor_credit_note_flow", "issued", true),
		},
		SearchIndexes: append([]search.IndexDefinition{
			procurementModelSearchIndex("procurement.vendors.search", "Vendor Search", "vendor_profile", "procurement.vendors.list", []string{"vendor_name", "party_id", "status"}),
			procurementModelSearchIndex("procurement.vendor_items.search", "Vendor Item Search", "vendor_item_profile", "procurement.vendor_items.list", []string{"vendor_id", "vendor_name", "item_code", "vendor_item_code", "status"}),
		}, procurementDocumentSearchIndexes()...),
		Security: module.SecurityDefinition{
			Permissions: append([]module.PermissionDefinition{}, []module.PermissionDefinition{
				{Key: "vendor.create", Action: "create", Resource: "vendor", DisplayName: "Create Vendors", DisplayNameI18n: localize("Create Vendors", "Buat Vendor")},
				{Key: "vendor.list", Action: "list", Resource: "vendor", DisplayName: "List Vendors", DisplayNameI18n: localize("List Vendors", "Daftar Vendor")},
				{Key: "vendor.read", Action: "read", Resource: "vendor", DisplayName: "Read Vendors", DisplayNameI18n: localize("Read Vendors", "Lihat Vendor")},
				{Key: "vendor.update", Action: "update", Resource: "vendor", DisplayName: "Update Vendors", DisplayNameI18n: localize("Update Vendors", "Perbarui Vendor")},
				{Key: "vendor_item.create", Action: "create", Resource: "vendor_item", DisplayName: "Create Vendor Items", DisplayNameI18n: localize("Create Vendor Items", "Buat Item Vendor")},
				{Key: "vendor_item.list", Action: "list", Resource: "vendor_item", DisplayName: "List Vendor Items", DisplayNameI18n: localize("List Vendor Items", "Daftar Item Vendor")},
				{Key: "vendor_item.read", Action: "read", Resource: "vendor_item", DisplayName: "Read Vendor Items", DisplayNameI18n: localize("Read Vendor Items", "Lihat Item Vendor")},
				{Key: "vendor_item.update", Action: "update", Resource: "vendor_item", DisplayName: "Update Vendor Items", DisplayNameI18n: localize("Update Vendor Items", "Perbarui Item Vendor")},
			}...),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "procurement_manager",
					Name:           "Procurement Manager",
					NameI18n:       localize("Procurement Manager", "Manajer Pengadaan"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"vendor.create", "vendor.list", "vendor.read", "vendor.update", "vendor_item.create", "vendor_item.list", "vendor_item.read", "vendor_item.update", "document.create", "document.list", "document.read", "document.update_draft", "document.submit", "document.approve", "document.reject", "document.reopen", "document.cancel"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "procurement.vendors", Label: "Vendors", LabelI18n: localize("Vendors", "Vendor"), ActionKey: "procurement.vendors.list", Order: 40, RequiredPermissions: []string{"vendor.list"}},
				{Key: "procurement.vendor_items", Label: "Vendor Items", LabelI18n: localize("Vendor Items", "Item Vendor"), ActionKey: "procurement.vendor_items.list", Order: 41, RequiredPermissions: []string{"vendor_item.list"}},
				{Key: "procurement.requests", Label: "Purchase Requests", LabelI18n: localize("Purchase Requests", "Permintaan Pembelian"), ActionKey: "procurement.requests.list", Order: 42, RequiredPermissions: []string{"document.list"}},
				{Key: "procurement.orders", Label: "Purchase Orders", LabelI18n: localize("Purchase Orders", "Pesanan Pembelian"), ActionKey: "procurement.orders.list", Order: 43, RequiredPermissions: []string{"document.list"}},
				{Key: "procurement.receipts", Label: "Receipts", LabelI18n: localize("Receipts", "Penerimaan"), ActionKey: "procurement.receipts.list", Order: 44, RequiredPermissions: []string{"document.list"}},
				{Key: "procurement.bills", Label: "Vendor Bills", LabelI18n: localize("Vendor Bills", "Tagihan Vendor"), ActionKey: "procurement.bills.list", Order: 45, RequiredPermissions: []string{"document.list"}},
				{Key: "procurement.payments", Label: "Payments Out", LabelI18n: localize("Payments Out", "Pembayaran Keluar"), ActionKey: "procurement.payments.list", Order: 46, RequiredPermissions: []string{"document.list"}},
				{Key: "procurement.credits", Label: "Vendor Credits", LabelI18n: localize("Vendor Credits", "Nota Kredit Vendor"), ActionKey: "procurement.credits.list", Order: 47, RequiredPermissions: []string{"document.list"}},
				{Key: "procurement.payables", Label: "Payables", LabelI18n: localize("Payables", "Utang"), ActionKey: "procurement.payables.dashboard", Order: 48, RequiredPermissions: []string{"document.list"}},
			},
			Actions: procurementFrontendActions(),
			Views:   procurementFrontendViews(),
		},
		Templates: []module.TemplateDefinition{
			commercialTemplateDefinition("procurement.purchase_request.print.default", "Purchase Request Print", "purchase_request", "Purchase Request", []string{"vendor_name", "request_date", "currency_code", "total_amount", "lines"}),
			commercialTemplateDefinition("procurement.purchase_order.print.default", "Purchase Order Print", "purchase_order", "Purchase Order", []string{"vendor_name", "order_date", "currency_code", "total_amount", "lines"}),
			commercialTemplateDefinition("procurement.goods_receipt.print.default", "Goods Receipt Print", "goods_receipt", "Goods Receipt", []string{"vendor_name", "receipt_date", "source_purchase_order_number", "lines"}),
			commercialTemplateDefinition("procurement.vendor_bill.print.default", "Vendor Bill Print", "vendor_bill", "Vendor Bill", []string{"vendor_name", "bill_date", "due_date", "currency_code", "total_amount", "lines"}),
			commercialTemplateDefinition("procurement.payment_out.print.default", "Payment Out Print", "payment_out", "Payment Out", []string{"vendor_name", "payment_date", "payment_method_code", "amount_paid", "allocations"}),
			commercialTemplateDefinition("procurement.vendor_credit.print.default", "Vendor Credit Print", "vendor_credit_note", "Vendor Credit", []string{"vendor_name", "credit_date", "source_vendor_bill_number", "currency_code", "total_amount", "lines"}),
			commercialTemplateDefinition("procurement.vendor_statement.print.default", "Vendor Statement Print", "vendor_bill", "Vendor Statement", []string{"vendor_name", "bill_date", "due_date", "total_amount"}),
		},
	}
}

func procurementDocumentDefinition(documentType, displayName, workflowKey, numberingKey string) document.Definition {
	return document.Definition{
		Type:                   documentType,
		DisplayName:            displayName,
		SchemaVersion:          "v1",
		WorkflowKey:            workflowKey,
		NumberingKey:           numberingKey,
		OwnerModuleKey:         "procurement_core",
		AllowedLinkTypes:       []string{"source_request", "purchase_order_for", "receipt_for", "bill_for", "payment_for", "credit_for", "return_for", "movement_for", "posting_for"},
		AllowedAttachmentTypes: []string{"note", "image", "document"},
	}
}

func procurementWorkflowDefinition(key, approvedState string, allowCancelApproved bool) workflow.Definition {
	states := []string{"draft", "submitted", approvedState, "rejected", "cancelled"}
	actions := []workflow.ActionRule{
		{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
		{Action: "approve", FromState: "submitted", ToState: approvedState, PermissionKey: "document.approve"},
		{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
		{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
		{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
		{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
	}
	if allowCancelApproved {
		actions = append(actions, workflow.ActionRule{Action: "cancel", FromState: approvedState, ToState: "cancelled", PermissionKey: "document.cancel"})
	}
	return workflow.Definition{Key: key, States: states, Actions: actions}
}

func procurementDocumentActions(prefix, documentType, listLabel, detailLabel, newLabel, basePath string) []module.ActionDefinition {
	return commercialDocumentActions(prefix, documentType, listLabel, detailLabel, newLabel, basePath)
}

func procurementDocumentViews(prefix, documentType, listTitle, detailTitle, formTitle string, columns []module.ColumnDefinition, statusOptions []string, detailSections, formSections []module.SectionDefinition) []module.ViewDefinition {
	return commercialDocumentViews(prefix, documentType, listTitle, detailTitle, formTitle, columns, statusOptions, detailSections, formSections)
}

func procurementModelSearchIndex(key, title, modelKey, viewKey string, fieldKeys []string) search.IndexDefinition {
	return commercialModelSearchIndex(key, title, modelKey, viewKey, fieldKeys)
}

func procurementDocumentSearchIndexes() []search.IndexDefinition {
	return []search.IndexDefinition{
		commercialDocumentSearchIndex("procurement.requests.search", "Purchase Request Search", "purchase_request", "procurement.requests.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "vendor_name", Path: "body.payload.vendor_name", Type: "string", Searchable: true},
			{Key: "request_date", Path: "body.payload.request_date", Type: "string", Sort: true},
			{Key: "total_amount", Path: "body.payload.total_amount", Type: "number", Sort: true},
		}),
		commercialDocumentSearchIndex("procurement.orders.search", "Purchase Order Search", "purchase_order", "procurement.orders.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "vendor_name", Path: "body.payload.vendor_name", Type: "string", Searchable: true},
			{Key: "order_date", Path: "body.payload.order_date", Type: "string", Sort: true},
			{Key: "total_amount", Path: "body.payload.total_amount", Type: "number", Sort: true},
		}),
		commercialDocumentSearchIndex("procurement.receipts.search", "Goods Receipt Search", "goods_receipt", "procurement.receipts.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "vendor_name", Path: "body.payload.vendor_name", Type: "string", Searchable: true},
			{Key: "receipt_date", Path: "body.payload.receipt_date", Type: "string", Sort: true},
			{Key: "source_purchase_order_number", Path: "body.payload.source_purchase_order_number", Type: "string", Searchable: true},
		}),
		commercialDocumentSearchIndex("procurement.bills.search", "Vendor Bill Search", "vendor_bill", "procurement.bills.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "vendor_name", Path: "body.payload.vendor_name", Type: "string", Searchable: true},
			{Key: "bill_date", Path: "body.payload.bill_date", Type: "string", Sort: true},
			{Key: "due_date", Path: "body.payload.due_date", Type: "string", Sort: true},
			{Key: "total_amount", Path: "body.payload.total_amount", Type: "number", Sort: true},
		}),
		commercialDocumentSearchIndex("procurement.payments.search", "Payment Out Search", "payment_out", "procurement.payments.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "vendor_name", Path: "body.payload.vendor_name", Type: "string", Searchable: true},
			{Key: "payment_date", Path: "body.payload.payment_date", Type: "string", Sort: true},
			{Key: "amount_paid", Path: "body.payload.amount_paid", Type: "number", Sort: true},
		}),
		commercialDocumentSearchIndex("procurement.credits.search", "Vendor Credit Search", "vendor_credit_note", "procurement.credits.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "vendor_name", Path: "body.payload.vendor_name", Type: "string", Searchable: true},
			{Key: "credit_date", Path: "body.payload.credit_date", Type: "string", Sort: true},
			{Key: "source_vendor_bill_number", Path: "body.payload.source_vendor_bill_number", Type: "string", Searchable: true},
			{Key: "total_amount", Path: "body.payload.total_amount", Type: "number", Sort: true},
		}),
	}
}

func procurementFrontendActions() []module.ActionDefinition {
	actions := []module.ActionDefinition{
		{Key: "procurement.vendors.list", Label: "Vendors", LabelI18n: localize("Vendors", "Vendor"), Kind: "navigate", RoutePath: "/procurement/vendors", ViewKey: "procurement.vendors.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"vendor.list"}},
		{Key: "procurement.vendors.detail", Label: "Vendor Detail", LabelI18n: localize("Vendor Detail", "Detail Vendor"), Kind: "navigate", RoutePath: "/procurement/vendors/detail", ViewKey: "procurement.vendors.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"vendor.read"}},
		{Key: "procurement.vendors.form", Label: "Vendor Form", LabelI18n: localize("Vendor Form", "Form Vendor"), Kind: "navigate", RoutePath: "/procurement/vendors/form", ViewKey: "procurement.vendors.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"vendor.update"}},
		{Key: "procurement.vendor_items.list", Label: "Vendor Items", LabelI18n: localize("Vendor Items", "Item Vendor"), Kind: "navigate", RoutePath: "/procurement/vendor-items", ViewKey: "procurement.vendor_items.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"vendor_item.list"}},
		{Key: "procurement.vendor_items.detail", Label: "Vendor Item Detail", LabelI18n: localize("Vendor Item Detail", "Detail Item Vendor"), Kind: "navigate", RoutePath: "/procurement/vendor-items/detail", ViewKey: "procurement.vendor_items.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"vendor_item.read"}},
		{Key: "procurement.vendor_items.form", Label: "Vendor Item Form", LabelI18n: localize("Vendor Item Form", "Form Item Vendor"), Kind: "navigate", RoutePath: "/procurement/vendor-items/form", ViewKey: "procurement.vendor_items.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"vendor_item.update"}},
		{Key: "procurement.payables.dashboard", Label: "Payables", LabelI18n: localize("Payables", "Utang"), Kind: "navigate", RoutePath: "/procurement/payables", ViewKey: "procurement.payables.dashboard", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"document.list"}},
		{Key: "procurement.vendor_statement.dashboard", Label: "Vendor Statement", LabelI18n: localize("Vendor Statement", "Laporan Vendor"), Kind: "navigate", RoutePath: "/procurement/vendor-statement", ViewKey: "procurement.vendor_statement.dashboard", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"vendor.read", "document.list"}},
	}
	actions = append(actions, procurementDocumentActions("procurement.requests", "purchase_request", "Purchase Requests", "Purchase Request", "New Purchase Request", "/procurement/requests")...)
	actions = append(actions, procurementDocumentActions("procurement.orders", "purchase_order", "Purchase Orders", "Purchase Order", "New Purchase Order", "/procurement/orders")...)
	actions = append(actions, procurementDocumentActions("procurement.receipts", "goods_receipt", "Receipts", "Goods Receipt", "New Receipt", "/procurement/receipts")...)
	actions = append(actions, procurementDocumentActions("procurement.bills", "vendor_bill", "Vendor Bills", "Vendor Bill", "New Vendor Bill", "/procurement/bills")...)
	actions = append(actions, procurementDocumentActions("procurement.payments", "payment_out", "Payments Out", "Payment Out", "New Payment Out", "/procurement/payments")...)
	actions = append(actions, procurementDocumentActions("procurement.credits", "vendor_credit_note", "Vendor Credits", "Vendor Credit", "New Vendor Credit", "/procurement/credits")...)
	return actions
}

func procurementFrontendViews() []module.ViewDefinition {
	views := []module.ViewDefinition{
		commercialModelListView("procurement.vendors.list", "Vendors", "vendor_profile", []module.ColumnDefinition{
			{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "values.vendor_name"},
			{Key: "party_id", Label: "Party", LabelI18n: localize("Party", "Pihak"), Path: "values.party_id"},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code"},
			{Key: "payment_term_days", Label: "Payment Terms", LabelI18n: localize("Payment Terms", "Termin Pembayaran"), Path: "values.payment_term_days"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		}, []string{"active", "inactive"}),
		commercialModelDetailView("procurement.vendors.detail", "Vendor Detail", "vendor_profile", []module.FieldDefinition{
			{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "values.vendor_name", Type: "string"},
			{Key: "party_id", Label: "Party", LabelI18n: localize("Party", "Pihak"), Path: "values.party_id", Type: "string"},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string"},
			{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "values.tax_profile_code", Type: "string"},
			{Key: "payment_term_days", Label: "Payment Terms", LabelI18n: localize("Payment Terms", "Termin Pembayaran"), Path: "values.payment_term_days", Type: "number"},
			{Key: "payable_account_code", Label: "Payable Account", LabelI18n: localize("Payable Account", "Akun Utang"), Path: "values.payable_account_code", Type: "string"},
			{Key: "expense_account_code", Label: "Expense Account", LabelI18n: localize("Expense Account", "Akun Beban"), Path: "values.expense_account_code", Type: "string"},
			{Key: "default_payment_method_code", Label: "Default Payment Method", LabelI18n: localize("Default Payment Method", "Metode Pembayaran Default"), Path: "values.default_payment_method_code", Type: "string"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
		}),
		commercialModelFormView("procurement.vendors.form", "Vendor Form", "vendor_profile", []module.FieldDefinition{
			{Key: "party_id", Label: "Party", LabelI18n: localize("Party", "Pihak"), Path: "values.party_id", Type: "string", Widget: "select", Required: true},
			{Key: "vendor_name", Label: "Vendor Name", LabelI18n: localize("Vendor Name", "Nama Vendor"), Path: "values.vendor_name", Type: "string", Widget: "text"},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string", Widget: "text"},
			{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "values.tax_profile_code", Type: "string", Widget: "select"},
			{Key: "payment_term_days", Label: "Payment Terms", LabelI18n: localize("Payment Terms", "Termin Pembayaran"), Path: "values.payment_term_days", Type: "number"},
			{Key: "payable_account_code", Label: "Payable Account", LabelI18n: localize("Payable Account", "Akun Utang"), Path: "values.payable_account_code", Type: "string", Widget: "text"},
			{Key: "expense_account_code", Label: "Expense Account", LabelI18n: localize("Expense Account", "Akun Beban"), Path: "values.expense_account_code", Type: "string", Widget: "text"},
			{Key: "default_payment_method_code", Label: "Default Payment Method", LabelI18n: localize("Default Payment Method", "Metode Pembayaran Default"), Path: "values.default_payment_method_code", Type: "string", Widget: "select"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}),
		commercialModelListView("procurement.vendor_items.list", "Vendor Items", "vendor_item_profile", []module.ColumnDefinition{
			{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "values.vendor_name"},
			{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "values.item_code"},
			{Key: "vendor_item_code", Label: "Vendor Item Code", LabelI18n: localize("Vendor Item Code", "Kode Item Vendor"), Path: "values.vendor_item_code"},
			{Key: "preferred", Label: "Preferred", LabelI18n: localize("Preferred", "Utama"), Path: "values.preferred"},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Path: "values.priority"},
			{Key: "minimum_order_quantity", Label: "MOQ", LabelI18n: localize("MOQ", "MOQ"), Path: "values.minimum_order_quantity"},
			{Key: "pack_size", Label: "Pack Size", LabelI18n: localize("Pack Size", "Ukuran Kemasan"), Path: "values.pack_size"},
			{Key: "lead_time_days", Label: "Lead Time", LabelI18n: localize("Lead Time", "Lead Time"), Path: "values.lead_time_days"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		}, []string{"active", "inactive"}),
		commercialModelDetailView("procurement.vendor_items.detail", "Vendor Item Detail", "vendor_item_profile", []module.FieldDefinition{
			{Key: "vendor_id", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "values.vendor_id", Type: "string"},
			{Key: "vendor_name", Label: "Vendor Name", LabelI18n: localize("Vendor Name", "Nama Vendor"), Path: "values.vendor_name", Type: "string"},
			{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "values.item_code", Type: "string"},
			{Key: "vendor_item_code", Label: "Vendor Item Code", LabelI18n: localize("Vendor Item Code", "Kode Item Vendor"), Path: "values.vendor_item_code", Type: "string"},
			{Key: "preferred", Label: "Preferred", LabelI18n: localize("Preferred", "Utama"), Path: "values.preferred", Type: "bool"},
			{Key: "purchase_uom_code", Label: "Purchase UOM", LabelI18n: localize("Purchase UOM", "Satuan Beli"), Path: "values.purchase_uom_code", Type: "string"},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Path: "values.priority", Type: "number"},
			{Key: "lead_time_days", Label: "Lead Time Days", LabelI18n: localize("Lead Time Days", "Hari Lead Time"), Path: "values.lead_time_days", Type: "number"},
			{Key: "minimum_order_quantity", Label: "MOQ", LabelI18n: localize("MOQ", "MOQ"), Path: "values.minimum_order_quantity", Type: "number"},
			{Key: "pack_size", Label: "Pack Size", LabelI18n: localize("Pack Size", "Ukuran Kemasan"), Path: "values.pack_size", Type: "number"},
			{Key: "last_purchase_price", Label: "Last Purchase Price", LabelI18n: localize("Last Purchase Price", "Harga Beli Terakhir"), Path: "values.last_purchase_price", Type: "number"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
		}),
		commercialModelFormView("procurement.vendor_items.form", "Vendor Item Form", "vendor_item_profile", []module.FieldDefinition{
			{Key: "vendor_id", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "values.vendor_id", Type: "string", Widget: "select", Required: true},
			{Key: "vendor_name", Label: "Vendor Name", LabelI18n: localize("Vendor Name", "Nama Vendor"), Path: "values.vendor_name", Type: "string", Widget: "text"},
			{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "values.item_code", Type: "string", Widget: "select", Required: true},
			{Key: "vendor_item_code", Label: "Vendor Item Code", LabelI18n: localize("Vendor Item Code", "Kode Item Vendor"), Path: "values.vendor_item_code", Type: "string", Widget: "text"},
			{Key: "preferred", Label: "Preferred", LabelI18n: localize("Preferred", "Utama"), Path: "values.preferred", Type: "bool", Widget: "checkbox"},
			{Key: "purchase_uom_code", Label: "Purchase UOM", LabelI18n: localize("Purchase UOM", "Satuan Beli"), Path: "values.purchase_uom_code", Type: "string", Widget: "select"},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Path: "values.priority", Type: "number"},
			{Key: "lead_time_days", Label: "Lead Time Days", LabelI18n: localize("Lead Time Days", "Hari Lead Time"), Path: "values.lead_time_days", Type: "number"},
			{Key: "minimum_order_quantity", Label: "MOQ", LabelI18n: localize("MOQ", "MOQ"), Path: "values.minimum_order_quantity", Type: "number"},
			{Key: "pack_size", Label: "Pack Size", LabelI18n: localize("Pack Size", "Ukuran Kemasan"), Path: "values.pack_size", Type: "number"},
			{Key: "last_purchase_price", Label: "Last Purchase Price", LabelI18n: localize("Last Purchase Price", "Harga Beli Terakhir"), Path: "values.last_purchase_price", Type: "number"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}),
		procurementPayablesDashboardView(),
		procurementVendorStatementDashboardView(),
	}
	views = append(views, procurementDocumentViews("procurement.requests", "purchase_request", "Purchase Requests", "Purchase Request Detail", "Purchase Request Draft", procurementRequestColumns(), []string{"draft", "submitted", "approved", "rejected", "cancelled"}, procurementRequestSections(), procurementRequestFormSections())...)
	views = append(views, procurementDocumentViews("procurement.orders", "purchase_order", "Purchase Orders", "Purchase Order Detail", "Purchase Order Draft", procurementOrderColumns(), []string{"draft", "submitted", "approved", "partially_received", "received", "rejected", "cancelled"}, procurementOrderSections(), procurementOrderFormSections())...)
	views = append(views, procurementDocumentViews("procurement.receipts", "goods_receipt", "Receipts", "Receipt Detail", "Receipt Draft", procurementReceiptColumns(), []string{"draft", "submitted", "received", "rejected", "cancelled"}, procurementReceiptSections(), procurementReceiptFormSections())...)
	views = append(views, procurementDocumentViews("procurement.bills", "vendor_bill", "Vendor Bills", "Vendor Bill Detail", "Vendor Bill Draft", procurementBillColumns(), []string{"draft", "submitted", "issued", "partially_paid", "paid", "rejected", "cancelled"}, procurementBillSections(), procurementBillFormSections())...)
	views = append(views, procurementDocumentViews("procurement.payments", "payment_out", "Payments Out", "Payment Out Detail", "Payment Out Draft", procurementPaymentColumns(), []string{"draft", "submitted", "paid", "rejected", "cancelled"}, procurementPaymentSections(), procurementPaymentFormSections())...)
	views = append(views, procurementDocumentViews("procurement.credits", "vendor_credit_note", "Vendor Credits", "Vendor Credit Detail", "Vendor Credit Draft", procurementCreditColumns(), []string{"draft", "submitted", "issued", "rejected", "cancelled"}, procurementCreditSections(), procurementCreditFormSections())...)
	return views
}

func procurementRequestColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Request", LabelI18n: localize("Request", "Permintaan"), Path: "header.number"},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name"},
		{Key: "request_date", Label: "Request Date", LabelI18n: localize("Request Date", "Tanggal Permintaan"), Path: "body.payload.request_date"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func procurementOrderColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "PO", LabelI18n: localize("PO", "PO"), Path: "header.number"},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name"},
		{Key: "order_date", Label: "Order Date", LabelI18n: localize("Order Date", "Tanggal Pesanan"), Path: "body.payload.order_date"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func procurementReceiptColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Receipt", LabelI18n: localize("Receipt", "Penerimaan"), Path: "header.number"},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name"},
		{Key: "receipt_date", Label: "Receipt Date", LabelI18n: localize("Receipt Date", "Tanggal Penerimaan"), Path: "body.payload.receipt_date"},
		{Key: "source_purchase_order_number", Label: "Source PO", LabelI18n: localize("Source PO", "PO Sumber"), Path: "body.payload.source_purchase_order_number"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func procurementBillColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Bill", LabelI18n: localize("Bill", "Tagihan"), Path: "header.number"},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name"},
		{Key: "bill_date", Label: "Bill Date", LabelI18n: localize("Bill Date", "Tanggal Tagihan"), Path: "body.payload.bill_date"},
		{Key: "due_date", Label: "Due Date", LabelI18n: localize("Due Date", "Jatuh Tempo"), Path: "body.payload.due_date"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func procurementPaymentColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Payment", LabelI18n: localize("Payment", "Pembayaran"), Path: "header.number"},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name"},
		{Key: "payment_date", Label: "Payment Date", LabelI18n: localize("Payment Date", "Tanggal Pembayaran"), Path: "body.payload.payment_date"},
		{Key: "payment_method_code", Label: "Method", LabelI18n: localize("Method", "Metode"), Path: "body.payload.payment_method_code"},
		{Key: "amount_paid", Label: "Amount", LabelI18n: localize("Amount", "Jumlah"), Path: "body.payload.amount_paid"},
		{Key: "unapplied_amount", Label: "Unapplied", LabelI18n: localize("Unapplied", "Belum Dialokasikan"), Path: "body.payload.unapplied_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func procurementCreditColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Credit", LabelI18n: localize("Credit", "Kredit"), Path: "header.number"},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name"},
		{Key: "credit_date", Label: "Credit Date", LabelI18n: localize("Credit Date", "Tanggal Kredit"), Path: "body.payload.credit_date"},
		{Key: "source_vendor_bill_number", Label: "Source Bill", LabelI18n: localize("Source Bill", "Tagihan Sumber"), Path: "body.payload.source_vendor_bill_number"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func procurementPayablesDashboardView() module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 "procurement.payables.dashboard",
		Title:               "Payables",
		TitleI18n:           localize("Payables", "Utang"),
		Kind:                "dashboard",
		ProjectionKey:       "procurement.payables.summary",
		RequiredPermissions: []string{"document.list"},
		Cards: []module.CardDefinition{
			{Key: "open_bill_count", Label: "Open Bills", LabelI18n: localize("Open Bills", "Tagihan Terbuka"), Path: "open_bill_count", ActionKey: "procurement.bills.list"},
			{Key: "open_balance_total", Label: "Open Balance", LabelI18n: localize("Open Balance", "Saldo Terbuka"), Path: "open_balance_total", ActionKey: "procurement.bills.list"},
			{Key: "overdue_bill_count", Label: "Overdue Bills", LabelI18n: localize("Overdue Bills", "Tagihan Jatuh Tempo"), Path: "overdue_bill_count", ActionKey: "procurement.bills.list"},
			{Key: "overdue_balance_total", Label: "Overdue Balance", LabelI18n: localize("Overdue Balance", "Saldo Jatuh Tempo"), Path: "overdue_balance_total", ActionKey: "procurement.bills.list"},
			{Key: "due_today_total", Label: "Due Today", LabelI18n: localize("Due Today", "Jatuh Tempo Hari Ini"), Path: "due_today_total", ActionKey: "procurement.bills.list"},
			{Key: "current_balance_total", Label: "Current Balance", LabelI18n: localize("Current Balance", "Saldo Lancar"), Path: "current_balance_total", ActionKey: "procurement.bills.list"},
			{Key: "paid_amount_total", Label: "Paid", LabelI18n: localize("Paid", "Dibayar"), Path: "paid_amount_total", ActionKey: "procurement.payments.list"},
			{Key: "credited_amount_total", Label: "Credited", LabelI18n: localize("Credited", "Dikreditkan"), Path: "credited_amount_total", ActionKey: "procurement.credits.list"},
		},
	}
}

func procurementVendorStatementDashboardView() module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 "procurement.vendor_statement.dashboard",
		Title:               "Vendor Statement",
		TitleI18n:           localize("Vendor Statement", "Laporan Vendor"),
		Kind:                "dashboard",
		ProjectionKey:       "procurement.vendor_statement",
		RequiredPermissions: []string{"vendor.read", "document.list"},
		Cards: []module.CardDefinition{
			{Key: "open_bill_count", Label: "Open Bills", LabelI18n: localize("Open Bills", "Tagihan Terbuka"), Path: "open_bill_count", ActionKey: "procurement.bills.list"},
			{Key: "open_balance_total", Label: "Open Balance", LabelI18n: localize("Open Balance", "Saldo Terbuka"), Path: "open_balance_total", ActionKey: "procurement.bills.list"},
			{Key: "paid_amount_total", Label: "Paid", LabelI18n: localize("Paid", "Dibayar"), Path: "paid_amount_total", ActionKey: "procurement.payments.list"},
			{Key: "credited_amount_total", Label: "Credited", LabelI18n: localize("Credited", "Dikreditkan"), Path: "credited_amount_total", ActionKey: "procurement.credits.list"},
		},
	}
}

func procurementRequestSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Request Summary", TitleI18n: localize("Request Summary", "Ringkasan Permintaan"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Request Number", LabelI18n: localize("Request Number", "Nomor Permintaan"), Path: "header.number", Type: "string", ReadOnly: true},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name", Type: "string"},
		{Key: "request_date", Label: "Request Date", LabelI18n: localize("Request Date", "Tanggal Permintaan"), Path: "body.payload.request_date", Type: "string"},
		{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string"},
		{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "body.payload.tax_profile_code", Type: "string"},
		{Key: "payment_term_days", Label: "Payment Terms", LabelI18n: localize("Payment Terms", "Termin Pembayaran"), Path: "body.payload.payment_term_days", Type: "number"},
		{Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "body.payload.default_tax_code", Type: "string"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
		{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "procurement_lines"},
	}}}
}

func procurementRequestFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Request Draft", TitleI18n: localize("Request Draft", "Draft Permintaan"), Fields: []module.FieldDefinition{
		{Key: "vendor_id", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_id", Type: "string", Widget: "select"},
		{Key: "vendor_name", Label: "Vendor Name", LabelI18n: localize("Vendor Name", "Nama Vendor"), Path: "body.payload.vendor_name", Type: "string", Widget: "text"},
		{Key: "request_date", Label: "Request Date", LabelI18n: localize("Request Date", "Tanggal Permintaan"), Path: "body.payload.request_date", Type: "string", Widget: "text"},
		{Key: "needed_by_date", Label: "Needed By", LabelI18n: localize("Needed By", "Dibutuhkan Pada"), Path: "body.payload.needed_by_date", Type: "string", Widget: "text"},
		{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string", Widget: "text"},
		{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "body.payload.tax_profile_code", Type: "string", Widget: "select"},
		{Key: "payment_term_days", Label: "Payment Terms", LabelI18n: localize("Payment Terms", "Termin Pembayaran"), Path: "body.payload.payment_term_days", Type: "number"},
		{Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "body.payload.default_tax_code", Type: "string", Widget: "select"},
		{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "procurement_lines"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}

func procurementOrderSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Order Summary", TitleI18n: localize("Order Summary", "Ringkasan Pesanan"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "PO Number", LabelI18n: localize("PO Number", "Nomor PO"), Path: "header.number", Type: "string", ReadOnly: true},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name", Type: "string"},
		{Key: "order_date", Label: "Order Date", LabelI18n: localize("Order Date", "Tanggal Pesanan"), Path: "body.payload.order_date", Type: "string"},
		{Key: "source_purchase_request_number", Label: "Source PR", LabelI18n: localize("Source PR", "PR Sumber"), Path: "body.payload.source_purchase_request_number", Type: "string"},
		{Key: "payment_term_days", Label: "Payment Terms", LabelI18n: localize("Payment Terms", "Termin Pembayaran"), Path: "body.payload.payment_term_days", Type: "number"},
		{Key: "expected_receipt_date", Label: "Expected Receipt", LabelI18n: localize("Expected Receipt", "Perkiraan Penerimaan"), Path: "body.payload.expected_receipt_date", Type: "string"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
		{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "procurement_lines"},
	}}}
}

func procurementOrderFormSections() []module.SectionDefinition {
	return procurementRequestFormSections()
}

func procurementReceiptSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Receipt Summary", TitleI18n: localize("Receipt Summary", "Ringkasan Penerimaan"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Receipt Number", LabelI18n: localize("Receipt Number", "Nomor Penerimaan"), Path: "header.number", Type: "string", ReadOnly: true},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name", Type: "string"},
		{Key: "receipt_date", Label: "Receipt Date", LabelI18n: localize("Receipt Date", "Tanggal Penerimaan"), Path: "body.payload.receipt_date", Type: "string"},
		{Key: "source_purchase_order_number", Label: "Source PO", LabelI18n: localize("Source PO", "PO Sumber"), Path: "body.payload.source_purchase_order_number", Type: "string"},
		{Key: "landed_cost_amount", Label: "Landed Cost", LabelI18n: localize("Landed Cost", "Biaya Pendaratan"), Path: "body.payload.landed_cost_amount", Type: "number"},
		{Key: "lines", Label: "Receipt Lines", LabelI18n: localize("Receipt Lines", "Baris Penerimaan"), Path: "body.payload.lines", Type: "object", Widget: "procurement_receipt_lines"},
		{Key: "landed_cost_lines", Label: "Landed Cost Lines", LabelI18n: localize("Landed Cost Lines", "Baris Biaya Pendaratan"), Path: "body.payload.landed_cost_lines", Type: "object", Widget: "json"},
	}}}
}

func procurementReceiptFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Receipt Draft", TitleI18n: localize("Receipt Draft", "Draft Penerimaan"), Fields: []module.FieldDefinition{
		{Key: "vendor_id", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_id", Type: "string", Widget: "select"},
		{Key: "vendor_name", Label: "Vendor Name", LabelI18n: localize("Vendor Name", "Nama Vendor"), Path: "body.payload.vendor_name", Type: "string", Widget: "text"},
		{Key: "receipt_date", Label: "Receipt Date", LabelI18n: localize("Receipt Date", "Tanggal Penerimaan"), Path: "body.payload.receipt_date", Type: "string", Widget: "text"},
		{Key: "source_purchase_order_id", Label: "Source PO", LabelI18n: localize("Source PO", "PO Sumber"), Path: "body.payload.source_purchase_order_id", Type: "string", Widget: "text"},
		{Key: "lines", Label: "Receipt Lines", LabelI18n: localize("Receipt Lines", "Baris Penerimaan"), Path: "body.payload.lines", Type: "object", Widget: "procurement_receipt_lines"},
		{Key: "landed_cost_lines", Label: "Landed Cost Lines", LabelI18n: localize("Landed Cost Lines", "Baris Biaya Pendaratan"), Path: "body.payload.landed_cost_lines", Type: "object", Widget: "json"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}

func procurementBillSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Bill Summary", TitleI18n: localize("Bill Summary", "Ringkasan Tagihan"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Bill Number", LabelI18n: localize("Bill Number", "Nomor Tagihan"), Path: "header.number", Type: "string", ReadOnly: true},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name", Type: "string"},
		{Key: "bill_date", Label: "Bill Date", LabelI18n: localize("Bill Date", "Tanggal Tagihan"), Path: "body.payload.bill_date", Type: "string"},
		{Key: "due_date", Label: "Due Date", LabelI18n: localize("Due Date", "Jatuh Tempo"), Path: "body.payload.due_date", Type: "string"},
		{Key: "source_purchase_order_number", Label: "Source PO", LabelI18n: localize("Source PO", "PO Sumber"), Path: "body.payload.source_purchase_order_number", Type: "string"},
		{Key: "source_goods_receipt_number", Label: "Source Receipt", LabelI18n: localize("Source Receipt", "Penerimaan Sumber"), Path: "body.payload.source_goods_receipt_number", Type: "string"},
		{Key: "landed_cost_amount", Label: "Landed Cost", LabelI18n: localize("Landed Cost", "Biaya Pendaratan"), Path: "body.payload.landed_cost_amount", Type: "number"},
		{Key: "purchase_price_variance_amount", Label: "Purchase Variance", LabelI18n: localize("Purchase Variance", "Selisih Harga Beli"), Path: "body.payload.purchase_price_variance_amount", Type: "number"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
		{Key: "paid_amount", Label: "Paid", LabelI18n: localize("Paid", "Dibayar"), Path: "body.payload.paid_amount", Type: "number"},
		{Key: "credited_amount", Label: "Credited", LabelI18n: localize("Credited", "Dikreditkan"), Path: "body.payload.credited_amount", Type: "number"},
		{Key: "balance_due_amount", Label: "Balance Due", LabelI18n: localize("Balance Due", "Saldo Jatuh Tempo"), Path: "body.payload.balance_due_amount", Type: "number"},
		{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "procurement_lines"},
		{Key: "landed_cost_lines", Label: "Landed Cost Lines", LabelI18n: localize("Landed Cost Lines", "Baris Biaya Pendaratan"), Path: "body.payload.landed_cost_lines", Type: "object", Widget: "json"},
	}}}
}

func procurementBillFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Bill Draft", TitleI18n: localize("Bill Draft", "Draft Tagihan"), Fields: []module.FieldDefinition{
		{Key: "vendor_id", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_id", Type: "string", Widget: "select"},
		{Key: "vendor_name", Label: "Vendor Name", LabelI18n: localize("Vendor Name", "Nama Vendor"), Path: "body.payload.vendor_name", Type: "string", Widget: "text"},
		{Key: "bill_date", Label: "Bill Date", LabelI18n: localize("Bill Date", "Tanggal Tagihan"), Path: "body.payload.bill_date", Type: "string", Widget: "text"},
		{Key: "due_date", Label: "Due Date", LabelI18n: localize("Due Date", "Jatuh Tempo"), Path: "body.payload.due_date", Type: "string", Widget: "text"},
		{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string", Widget: "text"},
		{Key: "payment_term_days", Label: "Payment Terms", LabelI18n: localize("Payment Terms", "Termin Pembayaran"), Path: "body.payload.payment_term_days", Type: "number"},
		{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "body.payload.tax_profile_code", Type: "string", Widget: "select"},
		{Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "body.payload.default_tax_code", Type: "string", Widget: "select"},
		{Key: "payable_account_code", Label: "Payable Account", LabelI18n: localize("Payable Account", "Akun Utang"), Path: "body.payload.payable_account_code", Type: "string", Widget: "text"},
		{Key: "expense_account_code", Label: "Expense Account", LabelI18n: localize("Expense Account", "Akun Beban"), Path: "body.payload.expense_account_code", Type: "string", Widget: "text"},
		{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "procurement_lines"},
		{Key: "landed_cost_lines", Label: "Landed Cost Lines", LabelI18n: localize("Landed Cost Lines", "Baris Biaya Pendaratan"), Path: "body.payload.landed_cost_lines", Type: "object", Widget: "json"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}

func procurementPaymentSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Payment Summary", TitleI18n: localize("Payment Summary", "Ringkasan Pembayaran"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Payment Number", LabelI18n: localize("Payment Number", "Nomor Pembayaran"), Path: "header.number", Type: "string", ReadOnly: true},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name", Type: "string"},
		{Key: "payment_date", Label: "Payment Date", LabelI18n: localize("Payment Date", "Tanggal Pembayaran"), Path: "body.payload.payment_date", Type: "string"},
		{Key: "payment_method_code", Label: "Method", LabelI18n: localize("Method", "Metode"), Path: "body.payload.payment_method_code", Type: "string"},
		{Key: "amount_paid", Label: "Amount Paid", LabelI18n: localize("Amount Paid", "Jumlah Dibayar"), Path: "body.payload.amount_paid", Type: "number"},
		{Key: "unapplied_amount", Label: "Unapplied", LabelI18n: localize("Unapplied", "Belum Dialokasikan"), Path: "body.payload.unapplied_amount", Type: "number"},
		{Key: "allocations", Label: "Allocations", LabelI18n: localize("Allocations", "Alokasi"), Path: "body.payload.allocations", Type: "object", Widget: "procurement_allocations"},
	}}}
}

func procurementPaymentFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Payment Draft", TitleI18n: localize("Payment Draft", "Draft Pembayaran"), Fields: []module.FieldDefinition{
		{Key: "vendor_id", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_id", Type: "string", Widget: "select"},
		{Key: "vendor_name", Label: "Vendor Name", LabelI18n: localize("Vendor Name", "Nama Vendor"), Path: "body.payload.vendor_name", Type: "string", Widget: "text"},
		{Key: "payment_date", Label: "Payment Date", LabelI18n: localize("Payment Date", "Tanggal Pembayaran"), Path: "body.payload.payment_date", Type: "string", Widget: "text"},
		{Key: "payment_method_code", Label: "Method", LabelI18n: localize("Method", "Metode"), Path: "body.payload.payment_method_code", Type: "string", Widget: "select"},
		{Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Path: "body.payload.clearing_account_code", Type: "string", Widget: "text"},
		{Key: "amount_paid", Label: "Amount Paid", LabelI18n: localize("Amount Paid", "Jumlah Dibayar"), Path: "body.payload.amount_paid", Type: "number"},
		{Key: "allocations", Label: "Allocations", LabelI18n: localize("Allocations", "Alokasi"), Path: "body.payload.allocations", Type: "object", Widget: "procurement_allocations"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}

func procurementCreditSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Vendor Credit Summary", TitleI18n: localize("Vendor Credit Summary", "Ringkasan Kredit Vendor"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Credit Number", LabelI18n: localize("Credit Number", "Nomor Kredit"), Path: "header.number", Type: "string", ReadOnly: true},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
		{Key: "vendor_name", Label: "Vendor", LabelI18n: localize("Vendor", "Vendor"), Path: "body.payload.vendor_name", Type: "string"},
		{Key: "credit_date", Label: "Credit Date", LabelI18n: localize("Credit Date", "Tanggal Kredit"), Path: "body.payload.credit_date", Type: "string"},
		{Key: "source_vendor_bill_number", Label: "Source Bill", LabelI18n: localize("Source Bill", "Tagihan Sumber"), Path: "body.payload.source_vendor_bill_number", Type: "string"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
		{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "procurement_lines"},
	}}}
}

func procurementCreditFormSections() []module.SectionDefinition { return procurementBillFormSections() }
