package app

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func employeeSpendCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                  "employee_spend_core",
		Name:                 "Employee Spend Core",
		NameI18n:             localize("Employee Spend Core", "Inti Pengeluaran Karyawan"),
		Version:              "1.0.0",
		DomainFamily:         "business",
		Description:          "Employee travel requests, cash advances, expense claims, liquidations, and reimbursement settlement.",
		DescriptionI18n:      localize("Employee travel requests, cash advances, expense claims, liquidations, and reimbursement settlement.", "Permintaan perjalanan, uang muka, klaim biaya, likuidasi, dan settlement reimbursement karyawan."),
		BusinessCapabilities: []string{"travel requests", "cash advances", "expense claims", "advance liquidation", "reimbursement payments"},
		OwnedDocumentTypes:   []string{"travel_request", "cash_advance", "expense_claim", "advance_liquidation", "reimbursement_payment"},
		OwnedWorkflowKeys:    []string{"travel_request_flow", "cash_advance_flow", "expense_claim_flow", "advance_liquidation_flow", "reimbursement_payment_flow"},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "employee_workforce", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "organization_structure", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "workflow_approval_policy", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "treasury_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "finance_manual_journal_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Employee Spend Console",
			TitleI18n:       localize("Employee Spend Console", "Konsol Pengeluaran Karyawan"),
			Description:     "Travel and employee spend setup, operations, liquidations, and reimbursement payments.",
			DescriptionI18n: localize("Travel and employee spend setup, operations, liquidations, and reimbursement payments.", "Setup perjalanan dan pengeluaran karyawan, operasi, likuidasi, dan pembayaran reimbursement."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "spend_setup",
					Title:     "Spend Setup",
					TitleI18n: localize("Spend Setup", "Setup Pengeluaran"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("expense_categories", "Expense Categories", "Kategori Biaya", "/ui/employee-spend/categories", "Open expense categories.", "Buka kategori biaya.", "expense_category.list"),
						adminConsoleLink("expense_policies", "Expense Policies", "Kebijakan Biaya", "/ui/employee-spend/policies", "Open expense policies.", "Buka kebijakan biaya.", "expense_policy.list"),
						adminConsoleLink("travel_policies", "Travel Policies", "Kebijakan Perjalanan", "/ui/employee-spend/travel-policies", "Open travel policies.", "Buka kebijakan perjalanan.", "travel_policy.list"),
						adminConsoleLink("rate_rules", "Rate Rules", "Aturan Tarif", "/ui/employee-spend/rate-rules", "Open expense rate rules.", "Buka aturan tarif biaya.", "expense_rate_rule.list"),
						adminConsoleLink("profiles", "Spend Profiles", "Profil Pengeluaran", "/ui/employee-spend/profiles", "Open employee spend profiles.", "Buka profil pengeluaran karyawan.", "employee_spend_profile.list"),
					},
				},
				{
					Key:       "spend_operations",
					Title:     "Spend Operations",
					TitleI18n: localize("Spend Operations", "Operasi Pengeluaran"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("travel_requests", "Travel Requests", "Permintaan Perjalanan", "/ui/employee-spend/travel-requests", "Open travel requests.", "Buka permintaan perjalanan.", "document.list"),
						adminConsoleLink("cash_advances", "Cash Advances", "Uang Muka", "/ui/employee-spend/cash-advances", "Open cash advances.", "Buka uang muka.", "document.list"),
						adminConsoleLink("expense_claims", "Expense Claims", "Klaim Biaya", "/ui/employee-spend/expense-claims", "Open expense claims.", "Buka klaim biaya.", "document.list"),
						adminConsoleLink("liquidations", "Advance Liquidations", "Likuidasi Uang Muka", "/ui/employee-spend/liquidations", "Open advance liquidations.", "Buka likuidasi uang muka.", "document.list"),
						adminConsoleLink("reimbursements", "Reimbursement Payments", "Pembayaran Reimbursement", "/ui/employee-spend/reimbursements", "Open reimbursement payments.", "Buka pembayaran reimbursement.", "document.list"),
					},
				},
			},
		},
		Models: []model.Definition{
			employeeSpendModelDefinition("expense_category", "Expense Category", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "category_group", Type: "string"},
				{Key: "requires_receipt", Type: "bool"},
				{Key: "expense_account_code", Type: "string"},
				{Key: "payable_account_code", Type: "string"},
				{Key: "tax_code", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			employeeSpendModelDefinition("expense_policy", "Expense Policy", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "default_currency_code", Type: "string", DefaultValue: "IDR"},
				{Key: "default_payment_method_code", Type: "string"},
				{Key: "default_payable_account_code", Type: "string"},
				{Key: "default_expense_account_code", Type: "string"},
				{Key: "default_treasury_account_id", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			employeeSpendModelDefinition("travel_policy", "Travel Policy", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "default_expense_policy_id", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			employeeSpendModelDefinition("expense_rate_rule", "Expense Rate Rule", []model.FieldDefinition{
				{Key: "policy_id", Type: "string"},
				{Key: "expense_category_code", Type: "string", Required: true},
				{Key: "rate_key", Type: "string", Required: true},
				{Key: "rate_amount", Type: "number"},
				{Key: "currency_code", Type: "string", DefaultValue: "IDR"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			employeeSpendModelDefinition("employee_spend_profile", "Employee Spend Profile", []model.FieldDefinition{
				{Key: "employee_id", Type: "string", Required: true},
				{Key: "expense_policy_id", Type: "string"},
				{Key: "travel_policy_id", Type: "string"},
				{Key: "default_currency_code", Type: "string", DefaultValue: "IDR"},
				{Key: "default_payment_method_code", Type: "string"},
				{Key: "payable_account_code", Type: "string"},
				{Key: "expense_account_code", Type: "string"},
				{Key: "treasury_account_id", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
		},
		Documents: []document.Definition{
			employeeSpendDocumentDefinition("travel_request", "Travel Request", "travel_request_flow", "travel_request_number"),
			employeeSpendDocumentDefinition("cash_advance", "Cash Advance", "cash_advance_flow", "cash_advance_number"),
			employeeSpendDocumentDefinition("expense_claim", "Expense Claim", "expense_claim_flow", "expense_claim_number"),
			employeeSpendDocumentDefinition("advance_liquidation", "Advance Liquidation", "advance_liquidation_flow", "advance_liquidation_number"),
			employeeSpendDocumentDefinition("reimbursement_payment", "Reimbursement Payment", "reimbursement_payment_flow", "reimbursement_payment_number"),
		},
		Workflows: []workflow.Definition{
			commercialWorkflowDefinition("travel_request_flow", "approved", true),
			commercialWorkflowDefinition("cash_advance_flow", "issued", true),
			commercialWorkflowDefinition("expense_claim_flow", "approved", true),
			commercialWorkflowDefinition("advance_liquidation_flow", "approved", true),
			commercialWorkflowDefinition("reimbursement_payment_flow", "paid", true),
		},
		SearchIndexes: append(
			[]search.IndexDefinition{
				commercialModelSearchIndex("employee_spend.categories.search", "Expense Category Search", "expense_category", "employee_spend.categories.list", []string{"code", "name", "status"}),
				commercialModelSearchIndex("employee_spend.policies.search", "Expense Policy Search", "expense_policy", "employee_spend.policies.list", []string{"code", "name", "status"}),
				commercialModelSearchIndex("employee_spend.travel_policies.search", "Travel Policy Search", "travel_policy", "employee_spend.travel_policies.list", []string{"code", "name", "status"}),
				commercialModelSearchIndex("employee_spend.rate_rules.search", "Expense Rate Rule Search", "expense_rate_rule", "employee_spend.rate_rules.list", []string{"expense_category_code", "rate_key", "status"}),
				commercialModelSearchIndex("employee_spend.profiles.search", "Employee Spend Profile Search", "employee_spend_profile", "employee_spend.profiles.list", []string{"employee_id", "expense_policy_id", "status"}),
			},
			employeeSpendDocumentSearchIndexes()...,
		),
		Security: module.SecurityDefinition{
			Permissions: append(
				append(
					append(
						commercialModelPermissions("expense_category", "Expense Category"),
						commercialModelPermissions("expense_policy", "Expense Policy")...,
					),
					append(
						commercialModelPermissions("travel_policy", "Travel Policy"),
						append(commercialModelPermissions("expense_rate_rule", "Expense Rate Rule"), commercialModelPermissions("employee_spend_profile", "Employee Spend Profile")...)...,
					)...,
				),
				[]module.PermissionDefinition{
					{Key: "employee_spend.read", Action: "read", Resource: "employee_spend", DisplayName: "Read Employee Spend", DisplayNameI18n: localize("Read Employee Spend", "Lihat Pengeluaran Karyawan")},
				}...,
			),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "employee_spend_manager",
					Name:           "Employee Spend Manager",
					NameI18n:       localize("Employee Spend Manager", "Manajer Pengeluaran Karyawan"),
					AllowedScopes:  []string{"deployment", "organization", "location"},
					PermissionKeys: []string{"expense_category.create", "expense_category.list", "expense_category.read", "expense_category.update", "expense_policy.create", "expense_policy.list", "expense_policy.read", "expense_policy.update", "travel_policy.create", "travel_policy.list", "travel_policy.read", "travel_policy.update", "expense_rate_rule.create", "expense_rate_rule.list", "expense_rate_rule.read", "expense_rate_rule.update", "employee_spend_profile.create", "employee_spend_profile.list", "employee_spend_profile.read", "employee_spend_profile.update", "employee_spend.read", "document.create", "document.list", "document.read", "document.update_draft", "document.submit", "document.approve", "document.reject", "document.reopen", "document.cancel"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "employee_spend.categories", Label: "Expense Categories", LabelI18n: localize("Expense Categories", "Kategori Biaya"), ActionKey: "employee_spend.categories.list", Order: 10, RequiredPermissions: []string{"expense_category.list"}},
				{Key: "employee_spend.policies", Label: "Expense Policies", LabelI18n: localize("Expense Policies", "Kebijakan Biaya"), ActionKey: "employee_spend.policies.list", Order: 11, RequiredPermissions: []string{"expense_policy.list"}},
				{Key: "employee_spend.travel_policies", Label: "Travel Policies", LabelI18n: localize("Travel Policies", "Kebijakan Perjalanan"), ActionKey: "employee_spend.travel_policies.list", Order: 12, RequiredPermissions: []string{"travel_policy.list"}},
				{Key: "employee_spend.rate_rules", Label: "Rate Rules", LabelI18n: localize("Rate Rules", "Aturan Tarif"), ActionKey: "employee_spend.rate_rules.list", Order: 13, RequiredPermissions: []string{"expense_rate_rule.list"}},
				{Key: "employee_spend.profiles", Label: "Spend Profiles", LabelI18n: localize("Spend Profiles", "Profil Pengeluaran"), ActionKey: "employee_spend.profiles.list", Order: 14, RequiredPermissions: []string{"employee_spend_profile.list"}},
				{Key: "employee_spend.travel_requests", Label: "Travel Requests", LabelI18n: localize("Travel Requests", "Permintaan Perjalanan"), ActionKey: "employee_spend.travel_requests.list", Order: 15, RequiredPermissions: []string{"document.list"}},
				{Key: "employee_spend.cash_advances", Label: "Cash Advances", LabelI18n: localize("Cash Advances", "Uang Muka"), ActionKey: "employee_spend.cash_advances.list", Order: 16, RequiredPermissions: []string{"document.list"}},
				{Key: "employee_spend.expense_claims", Label: "Expense Claims", LabelI18n: localize("Expense Claims", "Klaim Biaya"), ActionKey: "employee_spend.expense_claims.list", Order: 17, RequiredPermissions: []string{"document.list"}},
				{Key: "employee_spend.liquidations", Label: "Advance Liquidations", LabelI18n: localize("Advance Liquidations", "Likuidasi Uang Muka"), ActionKey: "employee_spend.liquidations.list", Order: 18, RequiredPermissions: []string{"document.list"}},
				{Key: "employee_spend.reimbursements", Label: "Reimbursement Payments", LabelI18n: localize("Reimbursement Payments", "Pembayaran Reimbursement"), ActionKey: "employee_spend.reimbursements.list", Order: 19, RequiredPermissions: []string{"document.list"}},
			},
			Actions: employeeSpendActions(),
			Views:   employeeSpendViews(),
		},
	}
}

func employeeSpendDocumentDefinition(documentType, displayName, workflowKey, numberingKey string) document.Definition {
	return document.Definition{
		Type:                   documentType,
		DisplayName:            displayName,
		SchemaVersion:          "v1",
		WorkflowKey:            workflowKey,
		NumberingKey:           numberingKey,
		OwnerModuleKey:         "employee_spend_core",
		AllowedLinkTypes:       []string{"related_to", "travel_for", "advance_for", "claim_for", "liquidation_for", "payment_for"},
		AllowedAttachmentTypes: []string{"note", "image", "document"},
	}
}

func employeeSpendModelDefinition(key, displayName string, fields []model.FieldDefinition) model.Definition {
	return model.Definition{
		Key:                 key,
		DisplayName:         displayName,
		OwnerModuleKey:      "employee_spend_core",
		Version:             "v1",
		CreatePermissionKey: key + ".create",
		ListPermissionKey:   key + ".list",
		ReadPermissionKey:   key + ".read",
		UpdatePermissionKey: key + ".update",
		DefaultSort:         employeeSpendFieldSortKey(fields),
		Fields:              fields,
	}
}

func employeeSpendDocumentSearchIndexes() []search.IndexDefinition {
	return []search.IndexDefinition{
		commercialDocumentSearchIndex("employee_spend.travel_requests.search", "Travel Request Search", "travel_request", "employee_spend.travel_requests.list", []search.IndexFieldDefinition{{Key: "employee_id", Path: "body.payload.employee_id", Type: "string", Searchable: true}, {Key: "party_id", Path: "body.payload.party_id", Type: "string", Searchable: true}, {Key: "request_date", Path: "body.payload.request_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
		commercialDocumentSearchIndex("employee_spend.cash_advances.search", "Cash Advance Search", "cash_advance", "employee_spend.cash_advances.list", []search.IndexFieldDefinition{{Key: "employee_id", Path: "body.payload.employee_id", Type: "string", Searchable: true}, {Key: "party_id", Path: "body.payload.party_id", Type: "string", Searchable: true}, {Key: "request_date", Path: "body.payload.request_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
		commercialDocumentSearchIndex("employee_spend.expense_claims.search", "Expense Claim Search", "expense_claim", "employee_spend.expense_claims.list", []search.IndexFieldDefinition{{Key: "employee_id", Path: "body.payload.employee_id", Type: "string", Searchable: true}, {Key: "party_id", Path: "body.payload.party_id", Type: "string", Searchable: true}, {Key: "claim_date", Path: "body.payload.claim_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
		commercialDocumentSearchIndex("employee_spend.liquidations.search", "Advance Liquidation Search", "advance_liquidation", "employee_spend.liquidations.list", []search.IndexFieldDefinition{{Key: "employee_id", Path: "body.payload.employee_id", Type: "string", Searchable: true}, {Key: "cash_advance_id", Path: "body.payload.cash_advance_id", Type: "string", Searchable: true}, {Key: "liquidation_date", Path: "body.payload.liquidation_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
		commercialDocumentSearchIndex("employee_spend.reimbursements.search", "Reimbursement Payment Search", "reimbursement_payment", "employee_spend.reimbursements.list", []search.IndexFieldDefinition{{Key: "employee_id", Path: "body.payload.employee_id", Type: "string", Searchable: true}, {Key: "party_id", Path: "body.payload.party_id", Type: "string", Searchable: true}, {Key: "payment_date", Path: "body.payload.payment_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
	}
}

func employeeSpendModelActions(prefix, listLabel, detailLabel, formLabel, modelKey string) []module.ActionDefinition {
	return []module.ActionDefinition{
		{Key: "employee_spend." + prefix + ".list", Label: listLabel, LabelI18n: localize(listLabel, listLabel), Kind: "navigate", RoutePath: "/employee-spend/" + prefix, ViewKey: "employee_spend." + prefix + ".list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".list"}},
		{Key: "employee_spend." + prefix + ".detail", Label: detailLabel, LabelI18n: localize(detailLabel, detailLabel), Kind: "navigate", RoutePath: "/employee-spend/" + prefix + "/detail", ViewKey: "employee_spend." + prefix + ".detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".read"}},
		{Key: "employee_spend." + prefix + ".form", Label: formLabel, LabelI18n: localize(formLabel, formLabel), Kind: "navigate", RoutePath: "/employee-spend/" + prefix + "/form", ViewKey: "employee_spend." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".update"}},
	}
}

func employeeSpendModelViews(prefix, title, modelKey string, columns []module.ColumnDefinition, fields []module.FieldDefinition) []module.ViewDefinition {
	return []module.ViewDefinition{
		commercialModelListView("employee_spend."+prefix+".list", title, modelKey, columns, []string{"active", "inactive"}),
		commercialModelDetailView("employee_spend."+prefix+".detail", title+" Detail", modelKey, fields),
		commercialModelFormView("employee_spend."+prefix+".form", title+" Form", modelKey, fields),
	}
}

func employeeSpendActions() []module.ActionDefinition {
	actions := []module.ActionDefinition{}
	actions = append(actions, employeeSpendModelActions("categories", "Expense Categories", "Expense Category Detail", "Expense Category Form", "expense_category")...)
	actions = append(actions, employeeSpendModelActions("policies", "Expense Policies", "Expense Policy Detail", "Expense Policy Form", "expense_policy")...)
	actions = append(actions, employeeSpendModelActions("travel_policies", "Travel Policies", "Travel Policy Detail", "Travel Policy Form", "travel_policy")...)
	actions = append(actions, employeeSpendModelActions("rate_rules", "Rate Rules", "Rate Rule Detail", "Rate Rule Form", "expense_rate_rule")...)
	actions = append(actions, employeeSpendModelActions("profiles", "Spend Profiles", "Spend Profile Detail", "Spend Profile Form", "employee_spend_profile")...)
	actions = append(actions, commercialDocumentActions("employee_spend.travel_requests", "travel_request", "Travel Requests", "Travel Request Detail", "New Travel Request", "/employee-spend/travel-requests")...)
	actions = append(actions, commercialDocumentActions("employee_spend.cash_advances", "cash_advance", "Cash Advances", "Cash Advance Detail", "New Cash Advance", "/employee-spend/cash-advances")...)
	actions = append(actions, commercialDocumentActions("employee_spend.expense_claims", "expense_claim", "Expense Claims", "Expense Claim Detail", "New Expense Claim", "/employee-spend/expense-claims")...)
	actions = append(actions, commercialDocumentActions("employee_spend.liquidations", "advance_liquidation", "Advance Liquidations", "Advance Liquidation Detail", "New Advance Liquidation", "/employee-spend/liquidations")...)
	actions = append(actions, commercialDocumentActions("employee_spend.reimbursements", "reimbursement_payment", "Reimbursement Payments", "Reimbursement Payment Detail", "New Reimbursement Payment", "/employee-spend/reimbursements")...)
	return actions
}

func employeeSpendViews() []module.ViewDefinition {
	views := []module.ViewDefinition{}
	views = append(views, employeeSpendModelViews("categories", "Expense Categories", "expense_category", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "status", Label: "Status", Path: "values.status"}}, employeeSpendSetupFields("expense_category"))...)
	views = append(views, employeeSpendModelViews("policies", "Expense Policies", "expense_policy", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "status", Label: "Status", Path: "values.status"}}, employeeSpendSetupFields("expense_policy"))...)
	views = append(views, employeeSpendModelViews("travel_policies", "Travel Policies", "travel_policy", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "status", Label: "Status", Path: "values.status"}}, employeeSpendSetupFields("travel_policy"))...)
	views = append(views, employeeSpendModelViews("rate_rules", "Rate Rules", "expense_rate_rule", []module.ColumnDefinition{{Key: "expense_category_code", Label: "Category", Path: "values.expense_category_code"}, {Key: "rate_key", Label: "Rate", Path: "values.rate_key"}, {Key: "status", Label: "Status", Path: "values.status"}}, employeeSpendSetupFields("expense_rate_rule"))...)
	views = append(views, employeeSpendModelViews("profiles", "Spend Profiles", "employee_spend_profile", []module.ColumnDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id"}, {Key: "expense_policy_id", Label: "Expense Policy", Path: "values.expense_policy_id"}, {Key: "status", Label: "Status", Path: "values.status"}}, employeeSpendSetupFields("employee_spend_profile"))...)
	views = append(views, commercialDocumentViews("employee_spend.travel_requests", "travel_request", "Travel Requests", "Travel Request Detail", "Travel Request Form", employeeSpendDocumentColumns("travel_request"), []string{"draft", "submitted", "approved", "rejected", "cancelled"}, employeeSpendDocumentSections("travel_request"), employeeSpendDocumentFormSections("travel_request"))...)
	views = append(views, commercialDocumentViews("employee_spend.cash_advances", "cash_advance", "Cash Advances", "Cash Advance Detail", "Cash Advance Form", employeeSpendDocumentColumns("cash_advance"), []string{"draft", "submitted", "issued", "rejected", "cancelled"}, employeeSpendDocumentSections("cash_advance"), employeeSpendDocumentFormSections("cash_advance"))...)
	views = append(views, commercialDocumentViews("employee_spend.expense_claims", "expense_claim", "Expense Claims", "Expense Claim Detail", "Expense Claim Form", employeeSpendDocumentColumns("expense_claim"), []string{"draft", "submitted", "approved", "rejected", "cancelled"}, employeeSpendDocumentSections("expense_claim"), employeeSpendDocumentFormSections("expense_claim"))...)
	views = append(views, commercialDocumentViews("employee_spend.liquidations", "advance_liquidation", "Advance Liquidations", "Advance Liquidation Detail", "Advance Liquidation Form", employeeSpendDocumentColumns("advance_liquidation"), []string{"draft", "submitted", "approved", "rejected", "cancelled"}, employeeSpendDocumentSections("advance_liquidation"), employeeSpendDocumentFormSections("advance_liquidation"))...)
	views = append(views, commercialDocumentViews("employee_spend.reimbursements", "reimbursement_payment", "Reimbursement Payments", "Reimbursement Payment Detail", "Reimbursement Payment Form", employeeSpendDocumentColumns("reimbursement_payment"), []string{"draft", "submitted", "paid", "rejected", "cancelled"}, employeeSpendDocumentSections("reimbursement_payment"), employeeSpendDocumentFormSections("reimbursement_payment"))...)
	return views
}

func employeeSpendSetupFields(modelKey string) []module.FieldDefinition {
	switch modelKey {
	case "expense_category":
		return []module.FieldDefinition{
			{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: "text", Required: true},
			{Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: "text", Required: true},
			{Key: "category_group", Label: "Group", Path: "values.category_group", Type: "string", Widget: "text"},
			{Key: "requires_receipt", Label: "Requires Receipt", Path: "values.requires_receipt", Type: "bool", Widget: "checkbox"},
			{Key: "expense_account_code", Label: "Expense Account", Path: "values.expense_account_code", Type: "string", Widget: "text"},
			{Key: "payable_account_code", Label: "Payable Account", Path: "values.payable_account_code", Type: "string", Widget: "text"},
			{Key: "tax_code", Label: "Tax Code", Path: "values.tax_code", Type: "string", Widget: "text"},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}
	case "expense_policy":
		return []module.FieldDefinition{
			{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: "text", Required: true},
			{Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: "text", Required: true},
			{Key: "organization_id", Label: "Organization", Path: "values.organization_id", Type: "string", Widget: "text"},
			{Key: "location_id", Label: "Location", Path: "values.location_id", Type: "string", Widget: "text"},
			{Key: "default_currency_code", Label: "Default Currency", Path: "values.default_currency_code", Type: "string", Widget: "text"},
			{Key: "default_payment_method_code", Label: "Default Payment Method", Path: "values.default_payment_method_code", Type: "string", Widget: "text"},
			{Key: "default_payable_account_code", Label: "Default Payable Account", Path: "values.default_payable_account_code", Type: "string", Widget: "text"},
			{Key: "default_expense_account_code", Label: "Default Expense Account", Path: "values.default_expense_account_code", Type: "string", Widget: "text"},
			{Key: "default_treasury_account_id", Label: "Default Treasury Account", Path: "values.default_treasury_account_id", Type: "string", Widget: "text"},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}
	case "travel_policy":
		return []module.FieldDefinition{
			{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: "text", Required: true},
			{Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: "text", Required: true},
			{Key: "organization_id", Label: "Organization", Path: "values.organization_id", Type: "string", Widget: "text"},
			{Key: "location_id", Label: "Location", Path: "values.location_id", Type: "string", Widget: "text"},
			{Key: "default_expense_policy_id", Label: "Default Expense Policy", Path: "values.default_expense_policy_id", Type: "string", Widget: "text"},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}
	case "expense_rate_rule":
		return []module.FieldDefinition{
			{Key: "policy_id", Label: "Policy", Path: "values.policy_id", Type: "string", Widget: "text"},
			{Key: "expense_category_code", Label: "Category", Path: "values.expense_category_code", Type: "string", Widget: "text", Required: true},
			{Key: "rate_key", Label: "Rate Key", Path: "values.rate_key", Type: "string", Widget: "text", Required: true},
			{Key: "rate_amount", Label: "Rate Amount", Path: "values.rate_amount", Type: "number", Widget: "text"},
			{Key: "currency_code", Label: "Currency", Path: "values.currency_code", Type: "string", Widget: "text"},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}
	default:
		return []module.FieldDefinition{
			{Key: "employee_id", Label: "Employee", Path: "values.employee_id", Type: "string", Widget: "text", Required: true},
			{Key: "expense_policy_id", Label: "Expense Policy", Path: "values.expense_policy_id", Type: "string", Widget: "text"},
			{Key: "travel_policy_id", Label: "Travel Policy", Path: "values.travel_policy_id", Type: "string", Widget: "text"},
			{Key: "default_currency_code", Label: "Default Currency", Path: "values.default_currency_code", Type: "string", Widget: "text"},
			{Key: "default_payment_method_code", Label: "Default Payment Method", Path: "values.default_payment_method_code", Type: "string", Widget: "text"},
			{Key: "payable_account_code", Label: "Payable Account", Path: "values.payable_account_code", Type: "string", Widget: "text"},
			{Key: "expense_account_code", Label: "Expense Account", Path: "values.expense_account_code", Type: "string", Widget: "text"},
			{Key: "treasury_account_id", Label: "Treasury Account", Path: "values.treasury_account_id", Type: "string", Widget: "text"},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
		}
	}
}

func employeeSpendDocumentColumns(documentType string) []module.ColumnDefinition {
	columns := []module.ColumnDefinition{
		{Key: "number", Label: "Number", Path: "header.number"},
		{Key: "status", Label: "Status", Path: "header.status"},
		{Key: "employee_id", Label: "Employee", Path: "body.payload.employee_id"},
		{Key: "party_id", Label: "Party", Path: "body.payload.party_id"},
	}
	switch documentType {
	case "travel_request":
		return append(columns, module.ColumnDefinition{Key: "request_date", Label: "Request Date", Path: "body.payload.request_date"}, module.ColumnDefinition{Key: "total_amount", Label: "Estimated Total", Path: "body.payload.total_amount"})
	case "cash_advance":
		return append(columns, module.ColumnDefinition{Key: "request_date", Label: "Request Date", Path: "body.payload.request_date"}, module.ColumnDefinition{Key: "approved_amount", Label: "Approved Amount", Path: "body.payload.approved_amount"})
	case "expense_claim":
		return append(columns, module.ColumnDefinition{Key: "claim_date", Label: "Claim Date", Path: "body.payload.claim_date"}, module.ColumnDefinition{Key: "reimbursable_amount", Label: "Reimbursable", Path: "body.payload.reimbursable_amount"})
	case "advance_liquidation":
		return append(columns, module.ColumnDefinition{Key: "liquidation_date", Label: "Liquidation Date", Path: "body.payload.liquidation_date"}, module.ColumnDefinition{Key: "net_settlement_amount", Label: "Net Settlement", Path: "body.payload.net_settlement_amount"})
	default:
		return append(columns, module.ColumnDefinition{Key: "payment_date", Label: "Payment Date", Path: "body.payload.payment_date"}, module.ColumnDefinition{Key: "amount_paid", Label: "Amount Paid", Path: "body.payload.amount_paid"})
	}
}

func employeeSpendDocumentSections(documentType string) []module.SectionDefinition {
	return []module.SectionDefinition{{
		Key: "summary", Title: "Summary", TitleI18n: localize("Summary", "Ringkasan"), Fields: employeeSpendDocumentFields(documentType, false),
	}}
}

func employeeSpendDocumentFormSections(documentType string) []module.SectionDefinition {
	return []module.SectionDefinition{{
		Key: "edit", Title: "Edit", TitleI18n: localize("Edit", "Ubah"), Fields: employeeSpendDocumentFields(documentType, true),
	}}
}

func employeeSpendDocumentFields(documentType string, form bool) []module.FieldDefinition {
	fields := []module.FieldDefinition{
		{Key: "employee_id", Label: "Employee", Path: "body.payload.employee_id", Type: "string", Widget: spendWidgetForForm(form, "text"), Required: true},
		{Key: "party_id", Label: "Party", Path: "body.payload.party_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
		{Key: "organization_unit_id", Label: "Organization Unit", Path: "body.payload.organization_unit_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
		{Key: "department_id", Label: "Department", Path: "body.payload.department_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
		{Key: "cost_center_id", Label: "Cost Center", Path: "body.payload.cost_center_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
		{Key: "currency_code", Label: "Currency", Path: "body.payload.currency_code", Type: "string", Widget: spendWidgetForForm(form, "text")},
		{Key: "notes", Label: "Notes", Path: "body.payload.notes", Type: "string", Widget: spendWidgetForForm(form, "textarea")},
	}
	switch documentType {
	case "travel_request":
		return append(fields,
			module.FieldDefinition{Key: "request_date", Label: "Request Date", Path: "body.payload.request_date", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "travel_start_date", Label: "Travel Start Date", Path: "body.payload.travel_start_date", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "travel_end_date", Label: "Travel End Date", Path: "body.payload.travel_end_date", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "destination", Label: "Destination", Path: "body.payload.destination", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "purpose", Label: "Purpose", Path: "body.payload.purpose", Type: "string", Widget: spendWidgetForForm(form, "textarea")},
			module.FieldDefinition{Key: "estimated_lines", Label: "Estimated Lines", Path: "body.payload.estimated_lines", Type: "object", Widget: spendWidgetForForm(form, "json")},
			module.FieldDefinition{Key: "total_amount", Label: "Estimated Total", Path: "body.payload.total_amount", Type: "number", Widget: spendWidgetForForm(form, "text")},
		)
	case "cash_advance":
		return append(fields,
			module.FieldDefinition{Key: "travel_request_id", Label: "Travel Request", Path: "body.payload.travel_request_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "request_date", Label: "Request Date", Path: "body.payload.request_date", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "requested_amount", Label: "Requested Amount", Path: "body.payload.requested_amount", Type: "number", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "approved_amount", Label: "Approved Amount", Path: "body.payload.approved_amount", Type: "number", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "outstanding_amount", Label: "Outstanding Amount", Path: "body.payload.outstanding_amount", Type: "number", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "advance_lines", Label: "Advance Lines", Path: "body.payload.advance_lines", Type: "object", Widget: spendWidgetForForm(form, "json")},
		)
	case "expense_claim":
		return append(fields,
			module.FieldDefinition{Key: "travel_request_id", Label: "Travel Request", Path: "body.payload.travel_request_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "claim_date", Label: "Claim Date", Path: "body.payload.claim_date", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "claim_lines", Label: "Claim Lines", Path: "body.payload.claim_lines", Type: "object", Widget: spendWidgetForForm(form, "json")},
			module.FieldDefinition{Key: "total_amount", Label: "Total Amount", Path: "body.payload.total_amount", Type: "number", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "approved_amount", Label: "Approved Amount", Path: "body.payload.approved_amount", Type: "number", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "reimbursable_amount", Label: "Reimbursable Amount", Path: "body.payload.reimbursable_amount", Type: "number", Widget: spendWidgetForForm(form, "text")},
		)
	case "advance_liquidation":
		return append(fields,
			module.FieldDefinition{Key: "travel_request_id", Label: "Travel Request", Path: "body.payload.travel_request_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "cash_advance_id", Label: "Cash Advance", Path: "body.payload.cash_advance_id", Type: "string", Widget: spendWidgetForForm(form, "text"), Required: true},
			module.FieldDefinition{Key: "expense_claim_id", Label: "Expense Claim", Path: "body.payload.expense_claim_id", Type: "string", Widget: spendWidgetForForm(form, "text"), Required: true},
			module.FieldDefinition{Key: "liquidation_date", Label: "Liquidation Date", Path: "body.payload.liquidation_date", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "advance_amount", Label: "Advance Amount", Path: "body.payload.advance_amount", Type: "number", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "claim_total_amount", Label: "Claim Total", Path: "body.payload.claim_total_amount", Type: "number", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "net_settlement_amount", Label: "Net Settlement", Path: "body.payload.net_settlement_amount", Type: "number", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "settlement_direction", Label: "Settlement Direction", Path: "body.payload.settlement_direction", Type: "string", Widget: spendWidgetForForm(form, "text")},
		)
	default:
		return append(fields,
			module.FieldDefinition{Key: "travel_request_id", Label: "Travel Request", Path: "body.payload.travel_request_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "cash_advance_id", Label: "Cash Advance", Path: "body.payload.cash_advance_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "source_liquidation_id", Label: "Source Liquidation", Path: "body.payload.source_liquidation_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "payment_date", Label: "Payment Date", Path: "body.payload.payment_date", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "payment_method_code", Label: "Payment Method", Path: "body.payload.payment_method_code", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "treasury_account_id", Label: "Treasury Account", Path: "body.payload.treasury_account_id", Type: "string", Widget: spendWidgetForForm(form, "text")},
			module.FieldDefinition{Key: "amount_paid", Label: "Amount Paid", Path: "body.payload.amount_paid", Type: "number", Widget: spendWidgetForForm(form, "text")},
		)
	}
}

func spendWidgetForForm(form bool, widget string) string {
	if !form {
		return ""
	}
	return widget
}

func employeeSpendFieldSortKey(fields []model.FieldDefinition) string {
	for _, field := range fields {
		if field.Key == "code" || field.Key == "employee_id" || field.Key == "name" {
			return field.Key
		}
	}
	return "code"
}
