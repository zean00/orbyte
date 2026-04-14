package app

import (
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

func promotionCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{promotionCoreKernelPackManifest()}
}

func promotionCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "promotion_core",
		Name:         "Promotion Core",
		NameI18n:     localize("Promotion Core", "Inti Promosi"),
		Version:      "1.0.0",
		DomainFamily: "business",
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "documents", VersionRange: ">=1.1.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "discount_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "pos_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindOptional},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Promotion Console",
			TitleI18n:       localize("Promotion Console", "Konsol Promosi"),
			Description:     "Campaign setup, promo code operations, and redemption auditing on top of discount rules.",
			DescriptionI18n: localize("Campaign setup, promo code operations, and redemption auditing on top of discount rules.", "Setup kampanye, operasi kode promo, dan audit redemption di atas aturan diskon."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "promotion_setup",
					Title:     "Promotion Setup",
					TitleI18n: localize("Promotion Setup", "Setup Promosi"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("campaigns", "Promotion Campaigns", "Kampanye Promosi", "/ui/promotion/campaigns", "Maintain promotion campaign lifecycle and scope.", "Kelola siklus hidup dan cakupan kampanye promosi.", "promotion_campaign.list"),
						adminConsoleLink("codes", "Promotion Codes", "Kode Promosi", "/ui/promotion/codes", "Maintain promo codes and customer eligibility.", "Kelola kode promo dan kelayakan pelanggan.", "promotion_code.list"),
						adminConsoleLink("redemptions", "Promotion Redemptions", "Redemption Promosi", "/ui/promotion/redemptions", "Review applied promotion usage.", "Tinjau penggunaan promosi yang diterapkan.", "promotion_redemption.list"),
						adminConsoleLink("discount_rules", "Discount Rules", "Aturan Diskon", "/ui/discount/rules", "Open the discount rules that promotions activate.", "Buka aturan diskon yang diaktifkan promosi.", "discount_rule.list"),
						adminConsoleLink("pos_stores", "POS Stores", "Toko POS", "/ui/pos/stores", "Review store scope used by promotions.", "Tinjau cakupan toko yang dipakai promosi.", "pos_store.list"),
					},
				},
			},
		},
		Models: []model.Definition{
			promotionCampaignModelDefinition(),
			promotionCodeModelDefinition(),
			promotionRedemptionModelDefinition(),
		},
		SearchIndexes: []search.IndexDefinition{
			commercialModelSearchIndex("promotion.campaigns.search", "Promotion Campaign Search", "promotion_campaign", "promotion.campaigns.list", []string{"code", "name", "trigger_mode", "status"}),
			commercialModelSearchIndex("promotion.codes.search", "Promotion Code Search", "promotion_code", "promotion.codes.list", []string{"code", "promotion_campaign_code", "status"}),
			commercialModelSearchIndex("promotion.redemptions.search", "Promotion Redemption Search", "promotion_redemption", "promotion.redemptions.list", []string{"promotion_campaign_code", "promotion_code", "source_document_type", "source_document_id", "status"}),
		},
		Security: module.SecurityDefinition{
			Permissions: append(
				append(commercialModelPermissions("promotion_campaign", "Promotion Campaign"), commercialModelPermissions("promotion_code", "Promotion Code")...),
				commercialModelPermissions("promotion_redemption", "Promotion Redemption")...,
			),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "promotion_manager",
					Name:           "Promotion Manager",
					NameI18n:       localize("Promotion Manager", "Pengelola Promosi"),
					AllowedScopes:  []string{"deployment", "organization", "location"},
					PermissionKeys: []string{"promotion_campaign.create", "promotion_campaign.list", "promotion_campaign.read", "promotion_campaign.update", "promotion_code.create", "promotion_code.list", "promotion_code.read", "promotion_code.update", "promotion_redemption.list", "promotion_redemption.read", "discount_rule.list"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "promotion.campaigns", Label: "Promotion Campaigns", LabelI18n: localize("Promotion Campaigns", "Kampanye Promosi"), ActionKey: "promotion.campaigns.list", Order: 29, RequiredPermissions: []string{"promotion_campaign.list"}},
				{Key: "promotion.codes", Label: "Promotion Codes", LabelI18n: localize("Promotion Codes", "Kode Promosi"), ActionKey: "promotion.codes.list", Order: 30, RequiredPermissions: []string{"promotion_code.list"}},
				{Key: "promotion.redemptions", Label: "Promotion Redemptions", LabelI18n: localize("Promotion Redemptions", "Redemption Promosi"), ActionKey: "promotion.redemptions.list", Order: 31, RequiredPermissions: []string{"promotion_redemption.list"}},
			},
			Actions: []module.ActionDefinition{
				{Key: "promotion.campaigns.list", Label: "Promotion Campaigns", LabelI18n: localize("Promotion Campaigns", "Kampanye Promosi"), Kind: "navigate", RoutePath: "/promotion/campaigns", ViewKey: "promotion.campaigns.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"promotion_campaign.list"}},
				{Key: "promotion.campaigns.detail", Label: "Promotion Campaign Detail", LabelI18n: localize("Promotion Campaign Detail", "Detail Kampanye Promosi"), Kind: "navigate", RoutePath: "/promotion/campaigns/detail", ViewKey: "promotion.campaigns.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"promotion_campaign.read"}},
				{Key: "promotion.campaigns.form", Label: "Promotion Campaign Form", LabelI18n: localize("Promotion Campaign Form", "Form Kampanye Promosi"), Kind: "navigate", RoutePath: "/promotion/campaigns/form", ViewKey: "promotion.campaigns.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"promotion_campaign.update"}},
				{Key: "promotion.plans.detail", Label: "Promotion Plan Detail", LabelI18n: localize("Promotion Plan Detail", "Detail Rencana Promosi"), Kind: "navigate", RoutePath: "/promotion/plans/detail", ViewKey: "promotion.plans.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"document.read"}},
				{Key: "promotion.plans.form", Label: "Promotion Plan Form", LabelI18n: localize("Promotion Plan Form", "Form Rencana Promosi"), Kind: "navigate", RoutePath: "/promotion/plans/form", ViewKey: "promotion.plans.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"document.update_draft"}},
				{Key: "promotion.codes.list", Label: "Promotion Codes", LabelI18n: localize("Promotion Codes", "Kode Promosi"), Kind: "navigate", RoutePath: "/promotion/codes", ViewKey: "promotion.codes.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"promotion_code.list"}},
				{Key: "promotion.codes.detail", Label: "Promotion Code Detail", LabelI18n: localize("Promotion Code Detail", "Detail Kode Promosi"), Kind: "navigate", RoutePath: "/promotion/codes/detail", ViewKey: "promotion.codes.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"promotion_code.read"}},
				{Key: "promotion.codes.form", Label: "Promotion Code Form", LabelI18n: localize("Promotion Code Form", "Form Kode Promosi"), Kind: "navigate", RoutePath: "/promotion/codes/form", ViewKey: "promotion.codes.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"promotion_code.update"}},
				{Key: "promotion.redemptions.list", Label: "Promotion Redemptions", LabelI18n: localize("Promotion Redemptions", "Redemption Promosi"), Kind: "navigate", RoutePath: "/promotion/redemptions", ViewKey: "promotion.redemptions.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"promotion_redemption.list"}},
				{Key: "promotion.redemptions.detail", Label: "Promotion Redemption Detail", LabelI18n: localize("Promotion Redemption Detail", "Detail Redemption Promosi"), Kind: "navigate", RoutePath: "/promotion/redemptions/detail", ViewKey: "promotion.redemptions.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"promotion_redemption.read"}},
				{Key: "promotion.redemptions.form", Label: "Promotion Redemption Form", LabelI18n: localize("Promotion Redemption Form", "Form Redemption Promosi"), Kind: "navigate", RoutePath: "/promotion/redemptions/form", ViewKey: "promotion.redemptions.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"promotion_redemption.update"}},
			},
			Views: []module.ViewDefinition{
				commercialModelListView("promotion.campaigns.list", "Promotion Campaigns", "promotion_campaign", []module.ColumnDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
					{Key: "trigger_mode", Label: "Trigger Mode", LabelI18n: localize("Trigger Mode", "Mode Pemicu"), Path: "values.trigger_mode"},
					{Key: "sales_channels", Label: "Channels", LabelI18n: localize("Channels", "Kanal"), Path: "values.sales_channels"},
					{Key: "store_codes", Label: "Stores", LabelI18n: localize("Stores", "Toko"), Path: "values.store_codes"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"draft", "active", "inactive", "expired"}),
				commercialModelDetailView("promotion.campaigns.detail", "Promotion Campaign Detail", "promotion_campaign", promotionCampaignFields(false)),
				commercialModelFormView("promotion.campaigns.form", "Promotion Campaign Form", "promotion_campaign", promotionCampaignFields(true)),
				{
					Key:                 "promotion.plans.detail",
					Title:               "Promotion Plan Detail",
					TitleI18n:           localize("Promotion Plan Detail", "Detail Rencana Promosi"),
					Kind:                "detail",
					DocumentType:        "generic_request",
					RequiredPermissions: []string{"document.read"},
					AllowedActions:      []string{"submit", "approve", "reject", "reopen", "cancel"},
					Sections: []module.SectionDefinition{
						{
							Key: "promotion_plan_header", Title: "Header", TitleI18n: localize("Header", "Header"), Fields: []module.FieldDefinition{
								{Key: "doc_id", Label: "Document ID", LabelI18n: localize("Document ID", "ID Dokumen"), Path: "header.id", Type: "string", ReadOnly: true},
								{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
								{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", ReadOnly: true},
							},
						},
						{
							Key: "promotion_plan_payload", Title: "Promotion Plan", TitleI18n: localize("Promotion Plan", "Rencana Promosi"), Fields: promotionPlanRequestFields(false),
						},
					},
				},
				{
					Key:                 "promotion.plans.form",
					Title:               "Promotion Plan Form",
					TitleI18n:           localize("Promotion Plan Form", "Form Rencana Promosi"),
					Kind:                "form",
					DocumentType:        "generic_request",
					RequiredPermissions: []string{"document.update_draft"},
					Sections: []module.SectionDefinition{{
						Key: "promotion_plan_fields", Title: "Promotion Plan", TitleI18n: localize("Promotion Plan", "Rencana Promosi"), Fields: promotionPlanRequestFields(true),
					}},
				},
				commercialModelListView("promotion.codes.list", "Promotion Codes", "promotion_code", []module.ColumnDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
					{Key: "promotion_campaign_code", Label: "Campaign", LabelI18n: localize("Campaign", "Kampanye"), Path: "values.promotion_campaign_code"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
					{Key: "total_redemptions", Label: "Redemptions", LabelI18n: localize("Redemptions", "Redemption"), Path: "values.total_redemptions"},
				}, []string{"active", "inactive", "expired"}),
				commercialModelDetailView("promotion.codes.detail", "Promotion Code Detail", "promotion_code", promotionCodeFields(false)),
				commercialModelFormView("promotion.codes.form", "Promotion Code Form", "promotion_code", promotionCodeFields(true)),
				commercialModelListView("promotion.redemptions.list", "Promotion Redemptions", "promotion_redemption", []module.ColumnDefinition{
					{Key: "promotion_campaign_code", Label: "Campaign", LabelI18n: localize("Campaign", "Kampanye"), Path: "values.promotion_campaign_code"},
					{Key: "promotion_code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.promotion_code"},
					{Key: "source_document_type", Label: "Source Type", LabelI18n: localize("Source Type", "Tipe Sumber"), Path: "values.source_document_type"},
					{Key: "source_document_id", Label: "Source Document", LabelI18n: localize("Source Document", "Dokumen Sumber"), Path: "values.source_document_id"},
					{Key: "party_id", Label: "Party", LabelI18n: localize("Party", "Pihak"), Path: "values.party_id"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "released"}),
				commercialModelDetailView("promotion.redemptions.detail", "Promotion Redemption Detail", "promotion_redemption", promotionRedemptionFields(false)),
				commercialModelFormView("promotion.redemptions.form", "Promotion Redemption Form", "promotion_redemption", promotionRedemptionFields(true)),
			},
		},
	}
}

func promotionCampaignModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "promotion_campaign",
		DisplayName:         "Promotion Campaign",
		DisplayNameI18n:     localize("Promotion Campaign", "Kampanye Promosi"),
		OwnerModuleKey:      "promotion_core",
		Version:             "v1",
		CreatePermissionKey: "promotion_campaign.create",
		ListPermissionKey:   "promotion_campaign.list",
		ReadPermissionKey:   "promotion_campaign.read",
		UpdatePermissionKey: "promotion_campaign.update",
		DefaultSort:         "code",
		Fields: []model.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
			{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Type: "string"},
			{Key: "trigger_mode", Label: "Trigger Mode", LabelI18n: localize("Trigger Mode", "Mode Pemicu"), Type: "string", Required: true, DefaultValue: "auto"},
			{Key: "start_at", Label: "Start At", LabelI18n: localize("Start At", "Mulai"), Type: "string"},
			{Key: "end_at", Label: "End At", LabelI18n: localize("End At", "Selesai"), Type: "string"},
			{Key: "sales_channels", Label: "Sales Channels", LabelI18n: localize("Sales Channels", "Kanal Penjualan"), Type: "string"},
			{Key: "store_codes", Label: "Store Codes", LabelI18n: localize("Store Codes", "Kode Toko"), Type: "string"},
			{Key: "global_usage_cap", Label: "Global Usage Cap", LabelI18n: localize("Global Usage Cap", "Batas Penggunaan Global"), Type: "number"},
			{Key: "per_customer_usage_cap", Label: "Per Customer Usage Cap", LabelI18n: localize("Per Customer Usage Cap", "Batas Penggunaan per Pelanggan"), Type: "number"},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Type: "number"},
			{Key: "banner_text", Label: "Banner Text", LabelI18n: localize("Banner Text", "Teks Banner"), Type: "string"},
			{Key: "combinability_mode", Label: "Combinability", LabelI18n: localize("Combinability", "Kombinasi"), Type: "string", DefaultValue: "inherit"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "draft"},
			{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
		},
	}
}

func promotionCodeModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "promotion_code",
		DisplayName:         "Promotion Code",
		DisplayNameI18n:     localize("Promotion Code", "Kode Promosi"),
		OwnerModuleKey:      "promotion_core",
		Version:             "v1",
		CreatePermissionKey: "promotion_code.create",
		ListPermissionKey:   "promotion_code.list",
		ReadPermissionKey:   "promotion_code.read",
		UpdatePermissionKey: "promotion_code.update",
		DefaultSort:         "code",
		Fields: []model.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
			{Key: "promotion_campaign_code", Label: "Campaign", LabelI18n: localize("Campaign", "Kampanye"), Type: "string", Required: true},
			{Key: "start_at", Label: "Start At", LabelI18n: localize("Start At", "Mulai"), Type: "string"},
			{Key: "end_at", Label: "End At", LabelI18n: localize("End At", "Selesai"), Type: "string"},
			{Key: "party_ids", Label: "Party IDs", LabelI18n: localize("Party IDs", "ID Pihak"), Type: "string"},
			{Key: "member_statuses", Label: "Member Statuses", LabelI18n: localize("Member Statuses", "Status Member"), Type: "string"},
			{Key: "member_tiers", Label: "Member Tiers", LabelI18n: localize("Member Tiers", "Tier Member"), Type: "string"},
			{Key: "total_redemption_limit", Label: "Total Redemption Limit", LabelI18n: localize("Total Redemption Limit", "Batas Redemption Total"), Type: "number"},
			{Key: "per_customer_redemption_limit", Label: "Per Customer Redemption Limit", LabelI18n: localize("Per Customer Redemption Limit", "Batas Redemption per Pelanggan"), Type: "number"},
			{Key: "total_redemptions", Label: "Total Redemptions", LabelI18n: localize("Total Redemptions", "Total Redemption"), Type: "number"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
		},
	}
}

func promotionRedemptionModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "promotion_redemption",
		DisplayName:         "Promotion Redemption",
		DisplayNameI18n:     localize("Promotion Redemption", "Redemption Promosi"),
		OwnerModuleKey:      "promotion_core",
		Version:             "v1",
		CreatePermissionKey: "promotion_redemption.create",
		ListPermissionKey:   "promotion_redemption.list",
		ReadPermissionKey:   "promotion_redemption.read",
		UpdatePermissionKey: "promotion_redemption.update",
		DefaultSort:         "redeemed_at",
		Fields: []model.FieldDefinition{
			{Key: "promotion_campaign_code", Label: "Campaign", LabelI18n: localize("Campaign", "Kampanye"), Type: "string", Required: true},
			{Key: "promotion_code", Label: "Promotion Code", LabelI18n: localize("Promotion Code", "Kode Promosi"), Type: "string"},
			{Key: "source_document_type", Label: "Source Type", LabelI18n: localize("Source Type", "Tipe Sumber"), Type: "string", Required: true},
			{Key: "source_document_id", Label: "Source Document ID", LabelI18n: localize("Source Document ID", "ID Dokumen Sumber"), Type: "string", Required: true},
			{Key: "party_id", Label: "Party ID", LabelI18n: localize("Party ID", "ID Pihak"), Type: "string"},
			{Key: "sales_channel", Label: "Sales Channel", LabelI18n: localize("Sales Channel", "Kanal Penjualan"), Type: "string"},
			{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Type: "string"},
			{Key: "discount_amount_total", Label: "Discount Amount", LabelI18n: localize("Discount Amount", "Jumlah Diskon"), Type: "number"},
			{Key: "redeemed_at", Label: "Redeemed At", LabelI18n: localize("Redeemed At", "Diredeem Pada"), Type: "string"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
		},
	}
}

func promotionCampaignFields(form bool) []module.FieldDefinition {
	widget := func(value string) string {
		if form {
			return value
		}
		return ""
	}
	return []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: widget("text"), Required: form},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: widget("text"), Required: form},
		{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string", Widget: widget("textarea")},
		{Key: "trigger_mode", Label: "Trigger Mode", LabelI18n: localize("Trigger Mode", "Mode Pemicu"), Path: "values.trigger_mode", Type: "string", Widget: widget("select"), Options: []string{"auto", "code"}},
		{Key: "start_at", Label: "Start At", LabelI18n: localize("Start At", "Mulai"), Path: "values.start_at", Type: "string", Widget: widget("text")},
		{Key: "end_at", Label: "End At", LabelI18n: localize("End At", "Selesai"), Path: "values.end_at", Type: "string", Widget: widget("text")},
		{Key: "sales_channels", Label: "Sales Channels", LabelI18n: localize("Sales Channels", "Kanal Penjualan"), Path: "values.sales_channels", Type: "string", Widget: widget("text")},
		{Key: "store_codes", Label: "Store Codes", LabelI18n: localize("Store Codes", "Kode Toko"), Path: "values.store_codes", Type: "string", Widget: widget("textarea")},
		{Key: "global_usage_cap", Label: "Global Usage Cap", LabelI18n: localize("Global Usage Cap", "Batas Penggunaan Global"), Path: "values.global_usage_cap", Type: "number", Widget: widget("number")},
		{Key: "per_customer_usage_cap", Label: "Per Customer Usage Cap", LabelI18n: localize("Per Customer Usage Cap", "Batas Penggunaan per Pelanggan"), Path: "values.per_customer_usage_cap", Type: "number", Widget: widget("number")},
		{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Path: "values.priority", Type: "number", Widget: widget("number")},
		{Key: "banner_text", Label: "Banner Text", LabelI18n: localize("Banner Text", "Teks Banner"), Path: "values.banner_text", Type: "string", Widget: widget("text")},
		{Key: "combinability_mode", Label: "Combinability", LabelI18n: localize("Combinability", "Kombinasi"), Path: "values.combinability_mode", Type: "string", Widget: widget("select"), Options: []string{"inherit", "exclusive", "stackable"}},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widget("select"), Options: []string{"draft", "active", "inactive", "expired"}},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string", Widget: widget("textarea")},
	}
}

func promotionPlanRequestFields(form bool) []module.FieldDefinition {
	widget := func(value string) string {
		if form {
			return value
		}
		return ""
	}
	return []module.FieldDefinition{
		{Key: "title", Label: "Title", LabelI18n: localize("Title", "Judul"), Path: "body.payload.title", Type: "string", Widget: widget("text"), Required: form},
		{Key: "summary", Label: "Summary", LabelI18n: localize("Summary", "Ringkasan"), Path: "body.payload.summary", Type: "string", Widget: widget("textarea")},
		{Key: "campaign_kind", Label: "Campaign Kind", LabelI18n: localize("Campaign Kind", "Jenis Kampanye"), Path: "body.payload.campaign_kind", Type: "string", Widget: widget("text")},
		{Key: "target_products", Label: "Target Products", LabelI18n: localize("Target Products", "Produk Target"), Path: "body.payload.target_products", Type: "string", Widget: widget("textarea")},
		{Key: "target_segment", Label: "Target Segment", LabelI18n: localize("Target Segment", "Segmen Target"), Path: "body.payload.target_segment", Type: "string", Widget: widget("text")},
		{Key: "replaced_campaign", Label: "Replaced Campaign", LabelI18n: localize("Replaced Campaign", "Kampanye Diganti"), Path: "body.payload.replaced_campaign", Type: "string", Widget: widget("text")},
		{Key: "request_kind", Label: "Request Kind", LabelI18n: localize("Request Kind", "Jenis Permintaan"), Path: "body.payload.request_kind", Type: "string", Widget: widget("text"), ReadOnly: !form},
		{Key: "viewer_hint", Label: "Viewer Hint", LabelI18n: localize("Viewer Hint", "Hint Viewer"), Path: "body.payload.viewer_hint", Type: "string", Widget: widget("text"), ReadOnly: !form},
	}
}

func promotionCodeFields(form bool) []module.FieldDefinition {
	widget := func(value string) string {
		if form {
			return value
		}
		return ""
	}
	return []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: widget("text"), Required: form},
		{Key: "promotion_campaign_code", Label: "Campaign", LabelI18n: localize("Campaign", "Kampanye"), Path: "values.promotion_campaign_code", Type: "string", Widget: widget("select"), Required: form},
		{Key: "start_at", Label: "Start At", LabelI18n: localize("Start At", "Mulai"), Path: "values.start_at", Type: "string", Widget: widget("text")},
		{Key: "end_at", Label: "End At", LabelI18n: localize("End At", "Selesai"), Path: "values.end_at", Type: "string", Widget: widget("text")},
		{Key: "party_ids", Label: "Party IDs", LabelI18n: localize("Party IDs", "ID Pihak"), Path: "values.party_ids", Type: "string", Widget: widget("textarea")},
		{Key: "member_statuses", Label: "Member Statuses", LabelI18n: localize("Member Statuses", "Status Member"), Path: "values.member_statuses", Type: "string", Widget: widget("textarea")},
		{Key: "member_tiers", Label: "Member Tiers", LabelI18n: localize("Member Tiers", "Tier Member"), Path: "values.member_tiers", Type: "string", Widget: widget("textarea")},
		{Key: "total_redemption_limit", Label: "Total Redemption Limit", LabelI18n: localize("Total Redemption Limit", "Batas Redemption Total"), Path: "values.total_redemption_limit", Type: "number", Widget: widget("number")},
		{Key: "per_customer_redemption_limit", Label: "Per Customer Redemption Limit", LabelI18n: localize("Per Customer Redemption Limit", "Batas Redemption per Pelanggan"), Path: "values.per_customer_redemption_limit", Type: "number", Widget: widget("number")},
		{Key: "total_redemptions", Label: "Total Redemptions", LabelI18n: localize("Total Redemptions", "Total Redemption"), Path: "values.total_redemptions", Type: "number", Widget: widget("number")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widget("select"), Options: []string{"active", "inactive", "expired"}},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string", Widget: widget("textarea")},
	}
}

func promotionRedemptionFields(form bool) []module.FieldDefinition {
	widget := func(value string) string {
		if form {
			return value
		}
		return ""
	}
	return []module.FieldDefinition{
		{Key: "promotion_campaign_code", Label: "Campaign", LabelI18n: localize("Campaign", "Kampanye"), Path: "values.promotion_campaign_code", Type: "string", Widget: widget("text"), Required: form},
		{Key: "promotion_code", Label: "Promotion Code", LabelI18n: localize("Promotion Code", "Kode Promosi"), Path: "values.promotion_code", Type: "string", Widget: widget("text")},
		{Key: "source_document_type", Label: "Source Type", LabelI18n: localize("Source Type", "Tipe Sumber"), Path: "values.source_document_type", Type: "string", Widget: widget("text"), Required: form},
		{Key: "source_document_id", Label: "Source Document ID", LabelI18n: localize("Source Document ID", "ID Dokumen Sumber"), Path: "values.source_document_id", Type: "string", Widget: widget("text"), Required: form},
		{Key: "party_id", Label: "Party ID", LabelI18n: localize("Party ID", "ID Pihak"), Path: "values.party_id", Type: "string", Widget: widget("text")},
		{Key: "sales_channel", Label: "Sales Channel", LabelI18n: localize("Sales Channel", "Kanal Penjualan"), Path: "values.sales_channel", Type: "string", Widget: widget("text")},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code", Type: "string", Widget: widget("text")},
		{Key: "discount_amount_total", Label: "Discount Amount", LabelI18n: localize("Discount Amount", "Jumlah Diskon"), Path: "values.discount_amount_total", Type: "number", Widget: widget("number")},
		{Key: "redeemed_at", Label: "Redeemed At", LabelI18n: localize("Redeemed At", "Diredeem Pada"), Path: "values.redeemed_at", Type: "string", Widget: widget("text")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widget("select"), Options: []string{"active", "released"}},
	}
}
