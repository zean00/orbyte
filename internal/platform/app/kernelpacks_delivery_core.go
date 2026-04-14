package app

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func deliveryCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{deliveryCoreKernelPackManifest()}
}

func deliveryCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "delivery_core",
		Name:         "Delivery Core",
		NameI18n:     localize("Delivery Core", "Inti Pengiriman"),
		Version:      "1.0.0",
		DomainFamily: "business",
		OwnedDocumentTypes: []string{
			"delivery_order",
		},
		OwnedWorkflowKeys: []string{
			"delivery_order_flow",
		},
		OwnedTemplateKeys: []string{
			"delivery.delivery_order.print.default",
		},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "commercial_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "fulfillment_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Delivery Console",
			TitleI18n:       localize("Delivery Console", "Konsol Pengiriman"),
			Description:     "Outbound delivery operations, shipment tracking, and proof-of-delivery shortcuts.",
			DescriptionI18n: localize("Outbound delivery operations, shipment tracking, and proof-of-delivery shortcuts.", "Operasi pengiriman keluar, pelacakan kiriman, dan pintasan bukti pengiriman."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "operations",
					Title:     "Delivery Operations",
					TitleI18n: localize("Delivery Operations", "Operasi Pengiriman"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("deliveries", "Deliveries", "Pengiriman", "/ui/delivery/orders", "Open delivery orders.", "Buka order pengiriman.", "document.list"),
						adminConsoleLink("fulfillments", "Fulfillments", "Fulfillment", "/ui/fulfillment/fulfillments", "Open fulfillments to register deliveries.", "Buka fulfillment untuk membuat pengiriman.", "document.list"),
					},
				},
				{
					Key:       "workflows",
					Title:     "Delivery Workflows",
					TitleI18n: localize("Delivery Workflows", "Workflow Pengiriman"),
					Kind:      module.AdminConsoleSectionWorkflowLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("delivery_order_flow", "Delivery Order Workflow", "Workflow Order Pengiriman", "/admin/workflows/designer?key=delivery_order_flow", "Open the delivery workflow.", "Buka workflow pengiriman.", "workflow.read"),
					},
				},
			},
		},
		Documents: []document.Definition{
			{
				Type:                   "delivery_order",
				DisplayName:            "Delivery Order",
				SchemaVersion:          "v1",
				WorkflowKey:            "delivery_order_flow",
				NumberingKey:           "delivery_order_number",
				OwnerModuleKey:         "delivery_core",
				AllowedLinkTypes:       []string{"delivery_for", "fulfillment_for", "return_for", "related_to"},
				AllowedAttachmentTypes: []string{"note", "image", "document"},
			},
		},
		Workflows: []workflow.Definition{
			{
				Key:    "delivery_order_flow",
				States: []string{"draft", "submitted", "dispatched", "delivered", "rejected", "cancelled"},
				Actions: []workflow.ActionRule{
					{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
					{Action: "approve", FromState: "submitted", ToState: "dispatched", PermissionKey: "document.approve"},
					{Action: "mark_delivered", FromState: "dispatched", ToState: "delivered", PermissionKey: "document.approve"},
					{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
					{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
					{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
					{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
				},
			},
		},
		SearchIndexes: []search.IndexDefinition{
			commercialDocumentSearchIndex("delivery.orders.search", "Delivery Order Search", "delivery_order", "delivery.orders.list", []search.IndexFieldDefinition{
				{Key: "number", Path: "header.number", Type: "string", Searchable: true},
				{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
				{Key: "party_name", Path: "body.payload.party_name", Type: "string", Searchable: true},
				{Key: "source_fulfillment_number", Path: "body.payload.source_fulfillment_number", Type: "string", Searchable: true},
				{Key: "delivery_date", Path: "body.payload.delivery_date", Type: "string", Sort: true},
				{Key: "carrier_name", Path: "body.payload.carrier_name", Type: "string", Searchable: true},
				{Key: "tracking_number", Path: "body.payload.tracking_number", Type: "string", Searchable: true},
			}),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "delivery.read", Action: "read", Resource: "delivery", DisplayName: "Read Deliveries", DisplayNameI18n: localize("Read Deliveries", "Lihat Pengiriman")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "delivery_operator",
					Name:           "Delivery Operator",
					NameI18n:       localize("Delivery Operator", "Operator Pengiriman"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"delivery.read", "document.list", "document.read", "document.submit", "document.approve"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "delivery.orders", Label: "Deliveries", LabelI18n: localize("Deliveries", "Pengiriman"), ActionKey: "delivery.orders.list", Order: 37, RequiredPermissions: []string{"document.list"}},
			},
			Actions: commercialDocumentActions("delivery.orders", "delivery_order", "Deliveries", "Delivery", "New Delivery", "/delivery/orders"),
			Views:   deliveryDocumentViews(),
		},
		Templates: []module.TemplateDefinition{
			commercialTemplateDefinition("delivery.delivery_order.print.default", "Delivery Order Print", "delivery_order", "Delivery Order", []string{"source_fulfillment_number", "party_name", "delivery_date", "carrier_name", "tracking_number", "lines"}),
		},
	}
}

func deliveryDocumentViews() []module.ViewDefinition {
	views := commercialDocumentViews(
		"delivery.orders",
		"delivery_order",
		"Deliveries",
		"Delivery Detail",
		"Delivery Draft",
		[]module.ColumnDefinition{
			{Key: "number", Label: "Delivery", LabelI18n: localize("Delivery", "Pengiriman"), Path: "header.number"},
			{Key: "source_fulfillment_number", Label: "Fulfillment", LabelI18n: localize("Fulfillment", "Fulfillment"), Path: "body.payload.source_fulfillment_number"},
			{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name"},
			{Key: "carrier_name", Label: "Carrier", LabelI18n: localize("Carrier", "Kurir"), Path: "body.payload.carrier_name"},
			{Key: "tracking_number", Label: "Tracking", LabelI18n: localize("Tracking", "Pelacakan"), Path: "body.payload.tracking_number"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
		},
		[]string{"draft", "submitted", "dispatched", "delivered", "rejected", "cancelled"},
		deliveryDetailSections(),
		deliveryFormSections(),
	)
	if len(views) > 1 {
		views[1].AllowedActions = []string{"submit", "approve", "mark_delivered", "reject", "reopen", "cancel"}
	}
	return views
}

func deliveryDetailSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Delivery Summary", TitleI18n: localize("Delivery Summary", "Ringkasan Pengiriman"), Fields: []module.FieldDefinition{
		{Key: "number", Label: "Number", LabelI18n: localize("Number", "Nomor"), Path: "header.number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string"},
		{Key: "source_fulfillment_number", Label: "Fulfillment", LabelI18n: localize("Fulfillment", "Fulfillment"), Path: "body.payload.source_fulfillment_number", Type: "string"},
		{Key: "source_sales_order_number", Label: "Order", LabelI18n: localize("Order", "Order"), Path: "body.payload.source_sales_order_number", Type: "string"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string"},
		{Key: "delivery_date", Label: "Delivery Date", LabelI18n: localize("Delivery Date", "Tanggal Pengiriman"), Path: "body.payload.delivery_date", Type: "string"},
		{Key: "shipment_status", Label: "Shipment Status", LabelI18n: localize("Shipment Status", "Status Kiriman"), Path: "body.payload.shipment_status", Type: "string"},
		{Key: "carrier_name", Label: "Carrier", LabelI18n: localize("Carrier", "Kurir"), Path: "body.payload.carrier_name", Type: "string"},
		{Key: "tracking_number", Label: "Tracking Number", LabelI18n: localize("Tracking Number", "Nomor Pelacakan"), Path: "body.payload.tracking_number", Type: "string"},
		{Key: "dispatch_date", Label: "Dispatch Date", LabelI18n: localize("Dispatch Date", "Tanggal Dispatch"), Path: "body.payload.dispatch_date", Type: "string"},
		{Key: "delivered_date", Label: "Delivered Date", LabelI18n: localize("Delivered Date", "Tanggal Terkirim"), Path: "body.payload.delivered_date", Type: "string"},
		{Key: "proof_of_delivery", Label: "Proof Of Delivery", LabelI18n: localize("Proof Of Delivery", "Bukti Pengiriman"), Path: "body.payload.proof_of_delivery", Type: "string"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "delivery_lines"},
	}}}
}

func deliveryFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Delivery Draft", TitleI18n: localize("Delivery Draft", "Draft Pengiriman"), Fields: []module.FieldDefinition{
		{Key: "source_fulfillment_number", Label: "Fulfillment", LabelI18n: localize("Fulfillment", "Fulfillment"), Path: "body.payload.source_fulfillment_number", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "source_sales_order_number", Label: "Order", LabelI18n: localize("Order", "Order"), Path: "body.payload.source_sales_order_number", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string", Widget: "text", ReadOnly: true},
		{Key: "delivery_date", Label: "Delivery Date", LabelI18n: localize("Delivery Date", "Tanggal Pengiriman"), Path: "body.payload.delivery_date", Type: "string", Widget: "text"},
		{Key: "carrier_name", Label: "Carrier", LabelI18n: localize("Carrier", "Kurir"), Path: "body.payload.carrier_name", Type: "string", Widget: "text"},
		{Key: "tracking_number", Label: "Tracking Number", LabelI18n: localize("Tracking Number", "Nomor Pelacakan"), Path: "body.payload.tracking_number", Type: "string", Widget: "text"},
		{Key: "proof_of_delivery", Label: "Proof Of Delivery", LabelI18n: localize("Proof Of Delivery", "Bukti Pengiriman"), Path: "body.payload.proof_of_delivery", Type: "string", Widget: "textarea"},
		{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "delivery_lines"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
	}}}
}
