package app

import (
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

func discountCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{discountCoreKernelPackManifest()}
}

func discountCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "discount_core",
		Name:         "Discount Core",
		NameI18n:     localize("Discount Core", "Inti Diskon"),
		Version:      "1.0.0",
		DomainFamily: "business",
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "commercial_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Discount Console",
			TitleI18n:       localize("Discount Console", "Konsol Diskon"),
			Description:     "Discount rule policy and commercial promotion setup.",
			DescriptionI18n: localize("Discount rule policy and commercial promotion setup.", "Kebijakan aturan diskon dan pengaturan promosi komersial."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:                 "discount_policy",
					Kind:                module.AdminConsoleSectionSettingsForm,
					Title:               "Discount Policy",
					TitleI18n:           localize("Discount Policy", "Kebijakan Diskon"),
					Description:         "Configure stacking mode and business time zone for discount evaluation.",
					DescriptionI18n:     localize("Configure stacking mode and business time zone for discount evaluation.", "Atur mode stacking dan zona waktu bisnis untuk evaluasi diskon."),
					ConfigKey:           "discount.policy",
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:       "discount_setup",
					Kind:      module.AdminConsoleSectionResourceLinks,
					Title:     "Discount Setup",
					TitleI18n: localize("Discount Setup", "Setup Diskon"),
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("rules", "Discount Rules", "Aturan Diskon", "/ui/discount/rules", "Maintain configurable discount and promotion rules.", "Kelola aturan diskon dan promosi yang dapat dikonfigurasi.", "discount_rule.list"),
						adminConsoleLink("customers", "Customers", "Pelanggan", "/ui/masterdata/parties", "Maintain member tiers and customer discount eligibility.", "Kelola tier member dan kelayakan diskon pelanggan.", "party.list"),
						adminConsoleLink("items", "Items", "Item", "/ui/commercial/items", "Review items, categories, and variants used by discount targeting.", "Tinjau item, kategori, dan varian untuk target diskon.", "item.list"),
					},
				},
			},
		},
		Models: []model.Definition{
			discountRuleModelDefinition(),
		},
		SearchIndexes: []search.IndexDefinition{
			commercialModelSearchIndex("discount.rules.search", "Discount Rule Search", "discount_rule", "discount.rules.list", []string{"code", "name", "promotion_campaign_code", "campaign_name", "rule_kind", "scope", "event_code", "status"}),
		},
		Security: module.SecurityDefinition{
			Permissions: commercialModelPermissions("discount_rule", "Discount Rule"),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "discount_manager",
					Name:           "Discount Manager",
					NameI18n:       localize("Discount Manager", "Pengelola Diskon"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"discount_rule.create", "discount_rule.list", "discount_rule.read", "discount_rule.update"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "discount.rules", Label: "Discount Rules", LabelI18n: localize("Discount Rules", "Aturan Diskon"), ActionKey: "discount.rules.list", Order: 28, RequiredPermissions: []string{"discount_rule.list"}},
			},
			Actions: []module.ActionDefinition{
				{Key: "discount.rules.list", Label: "Discount Rules", LabelI18n: localize("Discount Rules", "Aturan Diskon"), Kind: "navigate", RoutePath: "/discount/rules", ViewKey: "discount.rules.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"discount_rule.list"}},
				{Key: "discount.rules.detail", Label: "Discount Rule Detail", LabelI18n: localize("Discount Rule Detail", "Detail Aturan Diskon"), Kind: "navigate", RoutePath: "/discount/rules/detail", ViewKey: "discount.rules.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"discount_rule.read"}},
				{Key: "discount.rules.form", Label: "Discount Rule Form", LabelI18n: localize("Discount Rule Form", "Form Aturan Diskon"), Kind: "navigate", RoutePath: "/discount/rules/form", ViewKey: "discount.rules.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"discount_rule.update"}},
			},
			Views: []module.ViewDefinition{
				discountRuleListView(),
				discountRuleDetailView(),
				discountRuleFormView(),
			},
		},
	}
}

func discountRuleModelDefinition() model.Definition {
	return model.Definition{
		Key:                 "discount_rule",
		DisplayName:         "Discount Rule",
		DisplayNameI18n:     localize("Discount Rule", "Aturan Diskon"),
		OwnerModuleKey:      "discount_core",
		Version:             "v1",
		CreatePermissionKey: "discount_rule.create",
		ListPermissionKey:   "discount_rule.list",
		ReadPermissionKey:   "discount_rule.read",
		UpdatePermissionKey: "discount_rule.update",
		DefaultSort:         "priority",
		Fields: []model.FieldDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
			{Key: "promotion_campaign_code", Label: "Promotion Campaign", LabelI18n: localize("Promotion Campaign", "Kampanye Promosi"), Type: "string"},
			{Key: "campaign_name", Label: "Campaign", LabelI18n: localize("Campaign", "Kampanye"), Type: "string"},
			{Key: "event_code", Label: "Event Code", LabelI18n: localize("Event Code", "Kode Event"), Type: "string"},
			{Key: "scope", Label: "Scope", LabelI18n: localize("Scope", "Cakupan"), Type: "string", Required: true, DefaultValue: "line"},
			{Key: "rule_kind", Label: "Rule Kind", LabelI18n: localize("Rule Kind", "Jenis Aturan"), Type: "string", Required: true},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Type: "number"},
			{Key: "start_at", Label: "Start At", LabelI18n: localize("Start At", "Mulai"), Type: "string"},
			{Key: "end_at", Label: "End At", LabelI18n: localize("End At", "Selesai"), Type: "string"},
			{Key: "weekdays", Label: "Weekdays", LabelI18n: localize("Weekdays", "Hari"), Type: "string"},
			{Key: "start_time", Label: "Start Time", LabelI18n: localize("Start Time", "Jam Mulai"), Type: "string"},
			{Key: "end_time", Label: "End Time", LabelI18n: localize("End Time", "Jam Selesai"), Type: "string"},
			{Key: "party_ids", Label: "Party IDs", LabelI18n: localize("Party IDs", "ID Pihak"), Type: "string"},
			{Key: "customer_types", Label: "Customer Types", LabelI18n: localize("Customer Types", "Tipe Pelanggan"), Type: "string"},
			{Key: "member_statuses", Label: "Member Statuses", LabelI18n: localize("Member Statuses", "Status Member"), Type: "string"},
			{Key: "member_tiers", Label: "Member Tiers", LabelI18n: localize("Member Tiers", "Tier Member"), Type: "string"},
			{Key: "item_codes", Label: "Item Codes", LabelI18n: localize("Item Codes", "Kode Item"), Type: "string"},
			{Key: "product_codes", Label: "Product Codes", LabelI18n: localize("Product Codes", "Kode Produk"), Type: "string"},
			{Key: "variant_signatures", Label: "Variant Signatures", LabelI18n: localize("Variant Signatures", "Signature Varian"), Type: "string"},
			{Key: "category_codes", Label: "Category Codes", LabelI18n: localize("Category Codes", "Kode Kategori"), Type: "string"},
			{Key: "reward_item_codes", Label: "Reward Item Codes", LabelI18n: localize("Reward Item Codes", "Kode Item Reward"), Type: "string"},
			{Key: "excluded_item_codes", Label: "Excluded Item Codes", LabelI18n: localize("Excluded Item Codes", "Kode Item Dikecualikan"), Type: "string"},
			{Key: "excluded_product_codes", Label: "Excluded Product Codes", LabelI18n: localize("Excluded Product Codes", "Kode Produk Dikecualikan"), Type: "string"},
			{Key: "excluded_category_codes", Label: "Excluded Category Codes", LabelI18n: localize("Excluded Category Codes", "Kode Kategori Dikecualikan"), Type: "string"},
			{Key: "minimum_order_total", Label: "Minimum Order Total", LabelI18n: localize("Minimum Order Total", "Minimum Total Order"), Type: "number"},
			{Key: "minimum_line_quantity", Label: "Minimum Line Quantity", LabelI18n: localize("Minimum Line Quantity", "Minimum Kuantitas Baris"), Type: "number"},
			{Key: "buy_quantity", Label: "Buy Quantity", LabelI18n: localize("Buy Quantity", "Kuantitas Beli"), Type: "number"},
			{Key: "reward_quantity", Label: "Reward Quantity", LabelI18n: localize("Reward Quantity", "Kuantitas Reward"), Type: "number"},
			{Key: "discount_percent", Label: "Discount Percent", LabelI18n: localize("Discount Percent", "Persen Diskon"), Type: "number"},
			{Key: "discount_amount", Label: "Discount Amount", LabelI18n: localize("Discount Amount", "Jumlah Diskon"), Type: "number"},
			{Key: "fixed_price", Label: "Fixed Price", LabelI18n: localize("Fixed Price", "Harga Tetap"), Type: "number"},
			{Key: "reward_percent", Label: "Reward Percent", LabelI18n: localize("Reward Percent", "Persen Reward"), Type: "number"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
		},
	}
}

func discountRuleListView() module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 "discount.rules.list",
		Title:               "Discount Rules",
		TitleI18n:           localize("Discount Rules", "Aturan Diskon"),
		Kind:                "list",
		ModelKey:            "discount_rule",
		RequiredPermissions: []string{"discount_rule.list"},
		Columns: []module.ColumnDefinition{
			{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
			{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
			{Key: "promotion_campaign_code", Label: "Promotion Campaign", LabelI18n: localize("Promotion Campaign", "Kampanye Promosi"), Path: "values.promotion_campaign_code"},
			{Key: "campaign_name", Label: "Campaign", LabelI18n: localize("Campaign", "Kampanye"), Path: "values.campaign_name"},
			{Key: "rule_kind", Label: "Rule Kind", LabelI18n: localize("Rule Kind", "Jenis Aturan"), Path: "values.rule_kind"},
			{Key: "scope", Label: "Scope", LabelI18n: localize("Scope", "Cakupan"), Path: "values.scope"},
			{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Path: "values.priority"},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
		},
		Filters: []module.FilterDefinition{
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: []string{"active", "inactive"}},
			{Key: "scope", Label: "Scope", LabelI18n: localize("Scope", "Cakupan"), Type: "enum", Options: []string{"line", "order"}},
			{Key: "rule_kind", Label: "Rule Kind", LabelI18n: localize("Rule Kind", "Jenis Aturan"), Type: "string"},
		},
	}
}

func discountRuleDetailView() module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 "discount.rules.detail",
		Title:               "Discount Rule Detail",
		TitleI18n:           localize("Discount Rule Detail", "Detail Aturan Diskon"),
		Kind:                "detail",
		ModelKey:            "discount_rule",
		RequiredPermissions: []string{"discount_rule.read"},
		Tabs: []module.TabDefinition{{
			Key: "summary", Title: "Summary", TitleI18n: localize("Summary", "Ringkasan"), Sections: []module.SectionDefinition{{
				Key: "rule", Title: "Rule", TitleI18n: localize("Rule", "Aturan"), Fields: discountRuleFieldDefinitions(false),
			}},
		}},
	}
}

func discountRuleFormView() module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 "discount.rules.form",
		Title:               "Discount Rule Form",
		TitleI18n:           localize("Discount Rule Form", "Form Aturan Diskon"),
		Kind:                "form",
		ModelKey:            "discount_rule",
		RequiredPermissions: []string{"discount_rule.update"},
		Sections: []module.SectionDefinition{{
			Key: "edit", Title: "Edit Rule", TitleI18n: localize("Edit Rule", "Ubah Aturan"), Fields: discountRuleFieldDefinitions(true),
		}},
	}
}

func discountRuleFieldDefinitions(form bool) []module.FieldDefinition {
	widget := func(defaultWidget string) string {
		if form {
			return defaultWidget
		}
		return ""
	}
	return []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: widget("text"), Required: form},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: widget("text"), Required: form},
		{Key: "promotion_campaign_code", Label: "Promotion Campaign", LabelI18n: localize("Promotion Campaign", "Kampanye Promosi"), Path: "values.promotion_campaign_code", Type: "string", Widget: widget("select")},
		{Key: "campaign_name", Label: "Campaign", LabelI18n: localize("Campaign", "Kampanye"), Path: "values.campaign_name", Type: "string", Widget: widget("text")},
		{Key: "event_code", Label: "Event Code", LabelI18n: localize("Event Code", "Kode Event"), Path: "values.event_code", Type: "string", Widget: widget("text")},
		{Key: "scope", Label: "Scope", LabelI18n: localize("Scope", "Cakupan"), Path: "values.scope", Type: "string", Widget: widget("select"), Options: []string{"line", "order"}},
		{Key: "rule_kind", Label: "Rule Kind", LabelI18n: localize("Rule Kind", "Jenis Aturan"), Path: "values.rule_kind", Type: "string", Widget: widget("select"), Options: []string{"line_percent", "line_fixed_amount", "line_fixed_price", "bulk_percent", "threshold_item_price", "order_percent", "bxgy", "category_percent"}},
		{Key: "priority", Label: "Priority", LabelI18n: localize("Priority", "Prioritas"), Path: "values.priority", Type: "number", Widget: widget("number")},
		{Key: "start_at", Label: "Start At", LabelI18n: localize("Start At", "Mulai"), Path: "values.start_at", Type: "string", Widget: widget("text")},
		{Key: "end_at", Label: "End At", LabelI18n: localize("End At", "Selesai"), Path: "values.end_at", Type: "string", Widget: widget("text")},
		{Key: "weekdays", Label: "Weekdays", LabelI18n: localize("Weekdays", "Hari"), Path: "values.weekdays", Type: "string", Widget: widget("text")},
		{Key: "start_time", Label: "Start Time", LabelI18n: localize("Start Time", "Jam Mulai"), Path: "values.start_time", Type: "string", Widget: widget("text")},
		{Key: "end_time", Label: "End Time", LabelI18n: localize("End Time", "Jam Selesai"), Path: "values.end_time", Type: "string", Widget: widget("text")},
		{Key: "party_ids", Label: "Party IDs", LabelI18n: localize("Party IDs", "ID Pihak"), Path: "values.party_ids", Type: "string", Widget: widget("textarea")},
		{Key: "customer_types", Label: "Customer Types", LabelI18n: localize("Customer Types", "Tipe Pelanggan"), Path: "values.customer_types", Type: "string", Widget: widget("textarea")},
		{Key: "member_statuses", Label: "Member Statuses", LabelI18n: localize("Member Statuses", "Status Member"), Path: "values.member_statuses", Type: "string", Widget: widget("textarea")},
		{Key: "member_tiers", Label: "Member Tiers", LabelI18n: localize("Member Tiers", "Tier Member"), Path: "values.member_tiers", Type: "string", Widget: widget("textarea")},
		{Key: "item_codes", Label: "Item Codes", LabelI18n: localize("Item Codes", "Kode Item"), Path: "values.item_codes", Type: "string", Widget: widget("textarea")},
		{Key: "product_codes", Label: "Product Codes", LabelI18n: localize("Product Codes", "Kode Produk"), Path: "values.product_codes", Type: "string", Widget: widget("textarea")},
		{Key: "variant_signatures", Label: "Variant Signatures", LabelI18n: localize("Variant Signatures", "Signature Varian"), Path: "values.variant_signatures", Type: "string", Widget: widget("textarea")},
		{Key: "category_codes", Label: "Category Codes", LabelI18n: localize("Category Codes", "Kode Kategori"), Path: "values.category_codes", Type: "string", Widget: widget("textarea")},
		{Key: "reward_item_codes", Label: "Reward Item Codes", LabelI18n: localize("Reward Item Codes", "Kode Item Reward"), Path: "values.reward_item_codes", Type: "string", Widget: widget("textarea")},
		{Key: "excluded_item_codes", Label: "Excluded Item Codes", LabelI18n: localize("Excluded Item Codes", "Kode Item Dikecualikan"), Path: "values.excluded_item_codes", Type: "string", Widget: widget("textarea")},
		{Key: "excluded_product_codes", Label: "Excluded Product Codes", LabelI18n: localize("Excluded Product Codes", "Kode Produk Dikecualikan"), Path: "values.excluded_product_codes", Type: "string", Widget: widget("textarea")},
		{Key: "excluded_category_codes", Label: "Excluded Category Codes", LabelI18n: localize("Excluded Category Codes", "Kode Kategori Dikecualikan"), Path: "values.excluded_category_codes", Type: "string", Widget: widget("textarea")},
		{Key: "minimum_order_total", Label: "Minimum Order Total", LabelI18n: localize("Minimum Order Total", "Minimum Total Order"), Path: "values.minimum_order_total", Type: "number", Widget: widget("number")},
		{Key: "minimum_line_quantity", Label: "Minimum Line Quantity", LabelI18n: localize("Minimum Line Quantity", "Minimum Kuantitas Baris"), Path: "values.minimum_line_quantity", Type: "number", Widget: widget("number")},
		{Key: "buy_quantity", Label: "Buy Quantity", LabelI18n: localize("Buy Quantity", "Kuantitas Beli"), Path: "values.buy_quantity", Type: "number", Widget: widget("number")},
		{Key: "reward_quantity", Label: "Reward Quantity", LabelI18n: localize("Reward Quantity", "Kuantitas Reward"), Path: "values.reward_quantity", Type: "number", Widget: widget("number")},
		{Key: "discount_percent", Label: "Discount Percent", LabelI18n: localize("Discount Percent", "Persen Diskon"), Path: "values.discount_percent", Type: "number", Widget: widget("number")},
		{Key: "discount_amount", Label: "Discount Amount", LabelI18n: localize("Discount Amount", "Jumlah Diskon"), Path: "values.discount_amount", Type: "number", Widget: widget("number")},
		{Key: "fixed_price", Label: "Fixed Price", LabelI18n: localize("Fixed Price", "Harga Tetap"), Path: "values.fixed_price", Type: "number", Widget: widget("number")},
		{Key: "reward_percent", Label: "Reward Percent", LabelI18n: localize("Reward Percent", "Persen Reward"), Path: "values.reward_percent", Type: "number", Widget: widget("number")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widget("select"), Options: []string{"active", "inactive"}},
	}
}
