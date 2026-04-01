package app

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func payrollRemittanceCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                  "payroll_remittance_core",
		Name:                 "Payroll Remittance Core",
		NameI18n:             localize("Payroll Remittance Core", "Inti Remittance Payroll"),
		Version:              "1.0.0",
		DomainFamily:         "business",
		Description:          "Statutory remittance setup, payroll-derived liabilities, remittance batches, and treasury-settled remittance payments.",
		DescriptionI18n:      localize("Statutory remittance setup, payroll-derived liabilities, remittance batches, and treasury-settled remittance payments.", "Setup remittance statutory, liabilitas dari payroll, batch remittance, dan pembayaran remittance melalui treasury."),
		BusinessCapabilities: []string{"remittance authorities", "obligation types", "payroll remittance liabilities", "remittance batches", "remittance payments"},
		OwnedDocumentTypes:   []string{"payroll_remittance_liability", "payroll_remittance_adjustment", "payroll_remittance_batch", "payroll_remittance_payment"},
		OwnedWorkflowKeys:    []string{"payroll_remittance_liability_flow", "payroll_remittance_adjustment_flow", "payroll_remittance_batch_flow", "payroll_remittance_payment_flow"},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "employee_payroll_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "workflow_approval_policy", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "treasury_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "finance_manual_journal_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "organization_structure", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Payroll Remittance Console",
			TitleI18n:       localize("Payroll Remittance Console", "Konsol Remittance Payroll"),
			Description:     "Statutory remittance setup, liabilities, adjustments, batches, and payments.",
			DescriptionI18n: localize("Statutory remittance setup, liabilities, adjustments, batches, and payments.", "Setup remittance statutory, liabilitas, penyesuaian, batch, dan pembayaran."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "remittance_setup",
					Title:     "Remittance Setup",
					TitleI18n: localize("Remittance Setup", "Setup Remittance"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("authorities", "Remittance Authorities", "Otoritas Remittance", "/ui/remittance/authorities", "Open remittance authorities.", "Buka otoritas remittance.", "remittance_authority.list"),
						adminConsoleLink("obligation_types", "Obligation Types", "Tipe Kewajiban", "/ui/remittance/obligation-types", "Open obligation types.", "Buka tipe kewajiban.", "remittance_obligation_type.list"),
						adminConsoleLink("schedule_rules", "Schedule Rules", "Aturan Jadwal", "/ui/remittance/schedule-rules", "Open remittance schedule rules.", "Buka aturan jadwal remittance.", "remittance_schedule_rule.list"),
						adminConsoleLink("profiles", "Remittance Profiles", "Profil Remittance", "/ui/remittance/profiles", "Open payroll remittance profiles.", "Buka profil remittance payroll.", "payroll_remittance_profile.list"),
					},
				},
				{
					Key:       "remittance_operations",
					Title:     "Remittance Operations",
					TitleI18n: localize("Remittance Operations", "Operasi Remittance"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("liabilities", "Remittance Liabilities", "Liabilitas Remittance", "/ui/remittance/liabilities", "Open payroll remittance liabilities.", "Buka liabilitas remittance payroll.", "document.list"),
						adminConsoleLink("adjustments", "Remittance Adjustments", "Penyesuaian Remittance", "/ui/remittance/adjustments", "Open remittance adjustments.", "Buka penyesuaian remittance.", "document.list"),
						adminConsoleLink("batches", "Remittance Batches", "Batch Remittance", "/ui/remittance/batches", "Open remittance batches.", "Buka batch remittance.", "document.list"),
						adminConsoleLink("payments", "Remittance Payments", "Pembayaran Remittance", "/ui/remittance/payments", "Open remittance payments.", "Buka pembayaran remittance.", "document.list"),
					},
				},
			},
		},
		Models: []model.Definition{
			remittanceModelDefinition("remittance_authority", "Remittance Authority", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "default_currency_code", Type: "string", DefaultValue: "IDR"},
				{Key: "default_treasury_account_id", Type: "string"},
				{Key: "payment_method_code", Type: "string", DefaultValue: "BANK"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			remittanceModelDefinition("remittance_obligation_type", "Remittance Obligation Type", []model.FieldDefinition{
				{Key: "remittance_authority_id", Type: "string", Required: true},
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "obligation_class", Type: "string", Required: true},
				{Key: "liability_account_code", Type: "string", Required: true},
				{Key: "clearing_account_code", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			remittanceModelDefinition("remittance_schedule_rule", "Remittance Schedule Rule", []model.FieldDefinition{
				{Key: "remittance_authority_id", Type: "string"},
				{Key: "remittance_obligation_type_id", Type: "string"},
				{Key: "due_days_after_period_end", Type: "number"},
				{Key: "due_day_of_month", Type: "number"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			remittanceModelDefinition("payroll_remittance_profile", "Payroll Remittance Profile", []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "payroll_tax_rule_id", Type: "string"},
				{Key: "payroll_contribution_rule_id", Type: "string"},
				{Key: "remittance_authority_id", Type: "string", Required: true},
				{Key: "withholding_obligation_type_id", Type: "string"},
				{Key: "employee_contribution_obligation_type_id", Type: "string"},
				{Key: "employer_contribution_obligation_type_id", Type: "string"},
				{Key: "default_treasury_account_id", Type: "string"},
				{Key: "payment_method_code", Type: "string", DefaultValue: "BANK"},
				{Key: "payroll_payable_account_code", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
		},
		Documents: []document.Definition{
			remittanceDocumentDefinition("payroll_remittance_liability", "Payroll Remittance Liability", "payroll_remittance_liability_flow", "payroll_remittance_liability_number"),
			remittanceDocumentDefinition("payroll_remittance_adjustment", "Payroll Remittance Adjustment", "payroll_remittance_adjustment_flow", "payroll_remittance_adjustment_number"),
			remittanceDocumentDefinition("payroll_remittance_batch", "Payroll Remittance Batch", "payroll_remittance_batch_flow", "payroll_remittance_batch_number"),
			remittanceDocumentDefinition("payroll_remittance_payment", "Payroll Remittance Payment", "payroll_remittance_payment_flow", "payroll_remittance_payment_number"),
		},
		Workflows: []workflow.Definition{
			commercialWorkflowDefinition("payroll_remittance_liability_flow", "paid", true),
			commercialWorkflowDefinition("payroll_remittance_adjustment_flow", "approved", true),
			commercialWorkflowDefinition("payroll_remittance_batch_flow", "approved", true),
			commercialWorkflowDefinition("payroll_remittance_payment_flow", "paid", true),
		},
		Datasets: []module.DatasetDefinition{
			{Key: "remittance.authority.summary", Title: "Remittance Authority Summary", TitleI18n: localize("Remittance Authority Summary", "Ringkasan Otoritas Remittance"), SourceKind: "model", ModelKey: "remittance_authority", Dimensions: []module.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localize("By Status", "Berdasarkan Status"), Path: "status"}}, Measures: []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}}},
			{Key: "remittance.profile.summary", Title: "Remittance Profile Summary", TitleI18n: localize("Remittance Profile Summary", "Ringkasan Profil Remittance"), SourceKind: "model", ModelKey: "payroll_remittance_profile", Dimensions: []module.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localize("By Status", "Berdasarkan Status"), Path: "status"}}, Measures: []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}}},
		},
		SearchIndexes: append([]search.IndexDefinition{
			commercialModelSearchIndex("remittance.authorities.search", "Remittance Authority Search", "remittance_authority", "remittance.authorities.list", []string{"code", "name", "status"}),
			commercialModelSearchIndex("remittance.obligation_types.search", "Remittance Obligation Type Search", "remittance_obligation_type", "remittance.obligation_types.list", []string{"code", "name", "obligation_class", "status"}),
			commercialModelSearchIndex("remittance.schedule_rules.search", "Remittance Schedule Rule Search", "remittance_schedule_rule", "remittance.schedule_rules.list", []string{"remittance_authority_id", "remittance_obligation_type_id", "status"}),
			commercialModelSearchIndex("remittance.profiles.search", "Payroll Remittance Profile Search", "payroll_remittance_profile", "remittance.profiles.list", []string{"remittance_authority_id", "payroll_tax_rule_id", "payroll_contribution_rule_id", "status"}),
		}, remittanceDocumentSearchIndexes()...),
		Security: module.SecurityDefinition{
			Permissions: append(
				append(
					append(
						commercialModelPermissions("remittance_authority", "Remittance Authority"),
						commercialModelPermissions("remittance_obligation_type", "Remittance Obligation Type")...,
					),
					append(
						commercialModelPermissions("remittance_schedule_rule", "Remittance Schedule Rule"),
						commercialModelPermissions("payroll_remittance_profile", "Payroll Remittance Profile")...,
					)...,
				),
				module.PermissionDefinition{Key: "remittance.read", Action: "read", Resource: "remittance", DisplayName: "Read Remittance", DisplayNameI18n: localize("Read Remittance", "Lihat Remittance")},
			),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:           "remittance_manager",
					Name:          "Remittance Manager",
					NameI18n:      localize("Remittance Manager", "Manajer Remittance"),
					AllowedScopes: []string{"deployment", "organization", "location"},
					PermissionKeys: []string{
						"remittance_authority.create", "remittance_authority.list", "remittance_authority.read", "remittance_authority.update",
						"remittance_obligation_type.create", "remittance_obligation_type.list", "remittance_obligation_type.read", "remittance_obligation_type.update",
						"remittance_schedule_rule.create", "remittance_schedule_rule.list", "remittance_schedule_rule.read", "remittance_schedule_rule.update",
						"payroll_remittance_profile.create", "payroll_remittance_profile.list", "payroll_remittance_profile.read", "payroll_remittance_profile.update",
						"remittance.read",
						"document.create", "document.list", "document.read", "document.update_draft", "document.submit", "document.approve", "document.reject", "document.reopen", "document.cancel",
					},
				},
				{
					Key:            "remittance_approver",
					Name:           "Remittance Approver",
					NameI18n:       localize("Remittance Approver", "Penyetuju Remittance"),
					AllowedScopes:  []string{"deployment", "organization", "location"},
					PermissionKeys: []string{"remittance.read", "document.list", "document.read", "document.approve", "document.reject"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "remittance.authorities", Label: "Remittance Authorities", LabelI18n: localize("Remittance Authorities", "Otoritas Remittance"), ActionKey: "remittance.authorities.list", Order: 41, RequiredPermissions: []string{"remittance_authority.list"}},
				{Key: "remittance.obligation_types", Label: "Obligation Types", LabelI18n: localize("Obligation Types", "Tipe Kewajiban"), ActionKey: "remittance.obligation_types.list", Order: 42, RequiredPermissions: []string{"remittance_obligation_type.list"}},
				{Key: "remittance.schedule_rules", Label: "Schedule Rules", LabelI18n: localize("Schedule Rules", "Aturan Jadwal"), ActionKey: "remittance.schedule_rules.list", Order: 43, RequiredPermissions: []string{"remittance_schedule_rule.list"}},
				{Key: "remittance.profiles", Label: "Remittance Profiles", LabelI18n: localize("Remittance Profiles", "Profil Remittance"), ActionKey: "remittance.profiles.list", Order: 44, RequiredPermissions: []string{"payroll_remittance_profile.list"}},
				{Key: "remittance.liabilities", Label: "Remittance Liabilities", LabelI18n: localize("Remittance Liabilities", "Liabilitas Remittance"), ActionKey: "remittance.liabilities.list", Order: 45, RequiredPermissions: []string{"document.list"}},
				{Key: "remittance.adjustments", Label: "Remittance Adjustments", LabelI18n: localize("Remittance Adjustments", "Penyesuaian Remittance"), ActionKey: "remittance.adjustments.list", Order: 46, RequiredPermissions: []string{"document.list"}},
				{Key: "remittance.batches", Label: "Remittance Batches", LabelI18n: localize("Remittance Batches", "Batch Remittance"), ActionKey: "remittance.batches.list", Order: 47, RequiredPermissions: []string{"document.list"}},
				{Key: "remittance.payments", Label: "Remittance Payments", LabelI18n: localize("Remittance Payments", "Pembayaran Remittance"), ActionKey: "remittance.payments.list", Order: 48, RequiredPermissions: []string{"document.list"}},
			},
			Actions: remittanceActions(),
			Views:   remittanceViews(),
		},
	}
}

func remittanceDocumentDefinition(documentType, displayName, workflowKey, numberingKey string) document.Definition {
	return document.Definition{
		Type:                   documentType,
		DisplayName:            displayName,
		SchemaVersion:          "v1",
		WorkflowKey:            workflowKey,
		NumberingKey:           numberingKey,
		OwnerModuleKey:         "payroll_remittance_core",
		AllowedLinkTypes:       []string{"related_to", "run_for", "batch_for", "payment_for", "adjustment_for"},
		AllowedAttachmentTypes: []string{"note", "image", "document"},
	}
}

func remittanceModelDefinition(key, displayName string, fields []model.FieldDefinition) model.Definition {
	return model.Definition{
		Key:                 key,
		DisplayName:         displayName,
		OwnerModuleKey:      "payroll_remittance_core",
		Version:             "v1",
		CreatePermissionKey: key + ".create",
		ListPermissionKey:   key + ".list",
		ReadPermissionKey:   key + ".read",
		UpdatePermissionKey: key + ".update",
		DefaultSort:         remittanceFieldSortKey(fields),
		Fields:              fields,
	}
}

func remittanceFieldSortKey(fields []model.FieldDefinition) string {
	for _, candidate := range []string{"code", "name", "remittance_authority_id"} {
		for _, field := range fields {
			if field.Key == candidate {
				return candidate
			}
		}
	}
	if len(fields) == 0 {
		return ""
	}
	return fields[0].Key
}

func remittanceDocumentSearchIndexes() []search.IndexDefinition {
	return []search.IndexDefinition{
		commercialDocumentSearchIndex("remittance.liabilities.search", "Remittance Liability Search", "payroll_remittance_liability", "remittance.liabilities.list", []search.IndexFieldDefinition{{Key: "source_payroll_run_id", Path: "body.payload.source_payroll_run_id", Type: "string", Searchable: true}, {Key: "remittance_authority_id", Path: "body.payload.remittance_authority_id", Type: "string", Searchable: true}, {Key: "due_date", Path: "body.payload.due_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
		commercialDocumentSearchIndex("remittance.adjustments.search", "Remittance Adjustment Search", "payroll_remittance_adjustment", "remittance.adjustments.list", []search.IndexFieldDefinition{{Key: "liability_id", Path: "body.payload.liability_id", Type: "string", Searchable: true}, {Key: "adjustment_date", Path: "body.payload.adjustment_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
		commercialDocumentSearchIndex("remittance.batches.search", "Remittance Batch Search", "payroll_remittance_batch", "remittance.batches.list", []search.IndexFieldDefinition{{Key: "remittance_authority_id", Path: "body.payload.remittance_authority_id", Type: "string", Searchable: true}, {Key: "payment_date", Path: "body.payload.payment_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
		commercialDocumentSearchIndex("remittance.payments.search", "Remittance Payment Search", "payroll_remittance_payment", "remittance.payments.list", []search.IndexFieldDefinition{{Key: "payroll_remittance_batch_id", Path: "body.payload.payroll_remittance_batch_id", Type: "string", Searchable: true}, {Key: "payment_date", Path: "body.payload.payment_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
	}
}

func remittanceModelActions(prefix, listLabel, detailLabel, formLabel, modelKey string) []module.ActionDefinition {
	return []module.ActionDefinition{
		{Key: "remittance." + prefix + ".list", Label: listLabel, LabelI18n: localize(listLabel, listLabel), Kind: "navigate", RoutePath: "/remittance/" + prefix, ViewKey: "remittance." + prefix + ".list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".list"}},
		{Key: "remittance." + prefix + ".detail", Label: detailLabel, LabelI18n: localize(detailLabel, detailLabel), Kind: "navigate", RoutePath: "/remittance/" + prefix + "/detail", ViewKey: "remittance." + prefix + ".detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".read"}},
		{Key: "remittance." + prefix + ".form", Label: formLabel, LabelI18n: localize(formLabel, formLabel), Kind: "navigate", RoutePath: "/remittance/" + prefix + "/form", ViewKey: "remittance." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".update"}},
	}
}

func remittanceModelViews(prefix, title, modelKey string, columns []module.ColumnDefinition, fields []module.FieldDefinition) []module.ViewDefinition {
	return []module.ViewDefinition{
		commercialModelListView("remittance."+prefix+".list", title, modelKey, columns, []string{"active", "inactive"}),
		commercialModelDetailView("remittance."+prefix+".detail", title+" Detail", modelKey, fields),
		commercialModelFormView("remittance."+prefix+".form", title+" Form", modelKey, fields),
	}
}

func remittanceActions() []module.ActionDefinition {
	actions := []module.ActionDefinition{}
	actions = append(actions, remittanceModelActions("authorities", "Remittance Authorities", "Remittance Authority Detail", "Remittance Authority Form", "remittance_authority")...)
	actions = append(actions, remittanceModelActions("obligation_types", "Obligation Types", "Obligation Type Detail", "Obligation Type Form", "remittance_obligation_type")...)
	actions = append(actions, remittanceModelActions("schedule_rules", "Schedule Rules", "Schedule Rule Detail", "Schedule Rule Form", "remittance_schedule_rule")...)
	actions = append(actions, remittanceModelActions("profiles", "Remittance Profiles", "Remittance Profile Detail", "Remittance Profile Form", "payroll_remittance_profile")...)
	actions = append(actions, commercialDocumentActions("remittance.liabilities", "payroll_remittance_liability", "Remittance Liabilities", "Remittance Liability Detail", "New Remittance Liability", "/remittance/liabilities")...)
	actions = append(actions, commercialDocumentActions("remittance.adjustments", "payroll_remittance_adjustment", "Remittance Adjustments", "Remittance Adjustment Detail", "New Remittance Adjustment", "/remittance/adjustments")...)
	actions = append(actions, commercialDocumentActions("remittance.batches", "payroll_remittance_batch", "Remittance Batches", "Remittance Batch Detail", "New Remittance Batch", "/remittance/batches")...)
	actions = append(actions, commercialDocumentActions("remittance.payments", "payroll_remittance_payment", "Remittance Payments", "Remittance Payment Detail", "New Remittance Payment", "/remittance/payments")...)
	return actions
}

func remittanceViews() []module.ViewDefinition {
	views := []module.ViewDefinition{}
	views = append(views, remittanceModelViews("authorities", "Remittance Authorities", "remittance_authority", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "status", Label: "Status", Path: "values.status"}}, remittanceSetupFields("remittance_authority"))...)
	views = append(views, remittanceModelViews("obligation_types", "Obligation Types", "remittance_obligation_type", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "obligation_class", Label: "Class", Path: "values.obligation_class"}, {Key: "status", Label: "Status", Path: "values.status"}}, remittanceSetupFields("remittance_obligation_type"))...)
	views = append(views, remittanceModelViews("schedule_rules", "Schedule Rules", "remittance_schedule_rule", []module.ColumnDefinition{{Key: "remittance_authority_id", Label: "Authority", Path: "values.remittance_authority_id"}, {Key: "remittance_obligation_type_id", Label: "Obligation", Path: "values.remittance_obligation_type_id"}, {Key: "due_days_after_period_end", Label: "Days After Period", Path: "values.due_days_after_period_end"}, {Key: "status", Label: "Status", Path: "values.status"}}, remittanceSetupFields("remittance_schedule_rule"))...)
	views = append(views, remittanceModelViews("profiles", "Remittance Profiles", "payroll_remittance_profile", []module.ColumnDefinition{{Key: "remittance_authority_id", Label: "Authority", Path: "values.remittance_authority_id"}, {Key: "payroll_tax_rule_id", Label: "Tax Rule", Path: "values.payroll_tax_rule_id"}, {Key: "payroll_contribution_rule_id", Label: "Contribution Rule", Path: "values.payroll_contribution_rule_id"}, {Key: "status", Label: "Status", Path: "values.status"}}, remittanceSetupFields("payroll_remittance_profile"))...)
	views = append(views, commercialDocumentViews("remittance.liabilities", "payroll_remittance_liability", "Remittance Liabilities", "Remittance Liability Detail", "Remittance Liability Form", remittanceDocumentColumns("payroll_remittance_liability"), []string{"open", "partially_paid", "paid", "cancelled"}, remittanceDocumentSections("payroll_remittance_liability"), remittanceDocumentFormSections("payroll_remittance_liability"))...)
	views = append(views, commercialDocumentViews("remittance.adjustments", "payroll_remittance_adjustment", "Remittance Adjustments", "Remittance Adjustment Detail", "Remittance Adjustment Form", remittanceDocumentColumns("payroll_remittance_adjustment"), []string{"draft", "submitted", "approved", "rejected", "cancelled"}, remittanceDocumentSections("payroll_remittance_adjustment"), remittanceDocumentFormSections("payroll_remittance_adjustment"))...)
	views = append(views, commercialDocumentViews("remittance.batches", "payroll_remittance_batch", "Remittance Batches", "Remittance Batch Detail", "Remittance Batch Form", remittanceDocumentColumns("payroll_remittance_batch"), []string{"draft", "submitted", "approved", "paid", "rejected", "cancelled"}, remittanceDocumentSections("payroll_remittance_batch"), remittanceDocumentFormSections("payroll_remittance_batch"))...)
	views = append(views, commercialDocumentViews("remittance.payments", "payroll_remittance_payment", "Remittance Payments", "Remittance Payment Detail", "Remittance Payment Form", remittanceDocumentColumns("payroll_remittance_payment"), []string{"draft", "submitted", "paid", "rejected", "cancelled"}, remittanceDocumentSections("payroll_remittance_payment"), remittanceDocumentFormSections("payroll_remittance_payment"))...)
	return views
}

func remittanceSetupFields(modelKey string) []module.FieldDefinition {
	switch modelKey {
	case "remittance_authority":
		return []module.FieldDefinition{{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: remittanceWidgetForForm(true, "text")}, {Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: remittanceWidgetForForm(true, "text")}, {Key: "default_treasury_account_id", Label: "Treasury Account", Path: "values.default_treasury_account_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "payment_method_code", Label: "Payment Method", Path: "values.payment_method_code", Type: "string", Widget: remittanceWidgetForForm(true, "text")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: remittanceWidgetForForm(true, "select")}}
	case "remittance_obligation_type":
		return []module.FieldDefinition{{Key: "remittance_authority_id", Label: "Authority", Path: "values.remittance_authority_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: remittanceWidgetForForm(true, "text")}, {Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: remittanceWidgetForForm(true, "text")}, {Key: "obligation_class", Label: "Class", Path: "values.obligation_class", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "liability_account_code", Label: "Liability Account", Path: "values.liability_account_code", Type: "string", Widget: remittanceWidgetForForm(true, "text")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: remittanceWidgetForForm(true, "select")}}
	case "remittance_schedule_rule":
		return []module.FieldDefinition{{Key: "remittance_authority_id", Label: "Authority", Path: "values.remittance_authority_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "remittance_obligation_type_id", Label: "Obligation", Path: "values.remittance_obligation_type_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "due_days_after_period_end", Label: "Days After Period End", Path: "values.due_days_after_period_end", Type: "number", Widget: remittanceWidgetForForm(true, "text")}, {Key: "due_day_of_month", Label: "Due Day Of Month", Path: "values.due_day_of_month", Type: "number", Widget: remittanceWidgetForForm(true, "text")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: remittanceWidgetForForm(true, "select")}}
	default:
		return []module.FieldDefinition{{Key: "remittance_authority_id", Label: "Authority", Path: "values.remittance_authority_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "payroll_tax_rule_id", Label: "Payroll Tax Rule", Path: "values.payroll_tax_rule_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "payroll_contribution_rule_id", Label: "Payroll Contribution Rule", Path: "values.payroll_contribution_rule_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "withholding_obligation_type_id", Label: "Withholding Obligation", Path: "values.withholding_obligation_type_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "employee_contribution_obligation_type_id", Label: "Employee Contribution Obligation", Path: "values.employee_contribution_obligation_type_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "employer_contribution_obligation_type_id", Label: "Employer Contribution Obligation", Path: "values.employer_contribution_obligation_type_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "default_treasury_account_id", Label: "Treasury Account", Path: "values.default_treasury_account_id", Type: "string", Widget: remittanceWidgetForForm(true, "select")}, {Key: "payment_method_code", Label: "Payment Method", Path: "values.payment_method_code", Type: "string", Widget: remittanceWidgetForForm(true, "text")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: remittanceWidgetForForm(true, "select")}}
	}
}

func remittanceDocumentColumns(documentType string) []module.ColumnDefinition {
	switch documentType {
	case "payroll_remittance_liability":
		return []module.ColumnDefinition{{Key: "number", Label: "Liability", Path: "header.number"}, {Key: "remittance_authority_id", Label: "Authority", Path: "body.payload.remittance_authority_id"}, {Key: "due_date", Label: "Due Date", Path: "body.payload.due_date"}, {Key: "outstanding_amount", Label: "Outstanding", Path: "body.payload.outstanding_amount"}, {Key: "status", Label: "Status", Path: "header.status"}}
	case "payroll_remittance_adjustment":
		return []module.ColumnDefinition{{Key: "number", Label: "Adjustment", Path: "header.number"}, {Key: "liability_id", Label: "Liability", Path: "body.payload.liability_id"}, {Key: "adjustment_date", Label: "Date", Path: "body.payload.adjustment_date"}, {Key: "amount", Label: "Amount", Path: "body.payload.amount"}, {Key: "status", Label: "Status", Path: "header.status"}}
	case "payroll_remittance_batch":
		return []module.ColumnDefinition{{Key: "number", Label: "Batch", Path: "header.number"}, {Key: "remittance_authority_id", Label: "Authority", Path: "body.payload.remittance_authority_id"}, {Key: "payment_date", Label: "Payment Date", Path: "body.payload.payment_date"}, {Key: "total_amount", Label: "Total", Path: "body.payload.total_amount"}, {Key: "status", Label: "Status", Path: "header.status"}}
	default:
		return []module.ColumnDefinition{{Key: "number", Label: "Payment", Path: "header.number"}, {Key: "payroll_remittance_batch_id", Label: "Batch", Path: "body.payload.payroll_remittance_batch_id"}, {Key: "payment_date", Label: "Payment Date", Path: "body.payload.payment_date"}, {Key: "amount_paid", Label: "Amount Paid", Path: "body.payload.amount_paid"}, {Key: "status", Label: "Status", Path: "header.status"}}
	}
}

func remittanceDocumentSections(documentType string) []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "summary", Title: "Summary", TitleI18n: localize("Summary", "Ringkasan"), Fields: remittanceDocumentFields(documentType, false)}}
}

func remittanceDocumentFormSections(documentType string) []module.SectionDefinition {
	return []module.SectionDefinition{{Key: "edit", Title: "Edit", TitleI18n: localize("Edit", "Ubah"), Fields: remittanceDocumentFields(documentType, true)}}
}

func remittanceDocumentFields(documentType string, form bool) []module.FieldDefinition {
	switch documentType {
	case "payroll_remittance_liability":
		return []module.FieldDefinition{
			{Key: "source_payroll_run_id", Label: "Payroll Run", Path: "body.payload.source_payroll_run_id", Type: "string", Widget: remittanceWidgetForForm(form, "select")},
			{Key: "remittance_authority_id", Label: "Authority", Path: "body.payload.remittance_authority_id", Type: "string", Widget: remittanceWidgetForForm(form, "select")},
			{Key: "remittance_obligation_type_id", Label: "Obligation Type", Path: "body.payload.remittance_obligation_type_id", Type: "string", Widget: remittanceWidgetForForm(form, "select")},
			{Key: "due_date", Label: "Due Date", Path: "body.payload.due_date", Type: "string", Widget: remittanceWidgetForForm(form, "text")},
			{Key: "outstanding_amount", Label: "Outstanding Amount", Path: "body.payload.outstanding_amount", Type: "number", Widget: remittanceWidgetForForm(form, "text")},
		}
	case "payroll_remittance_adjustment":
		return []module.FieldDefinition{
			{Key: "liability_id", Label: "Liability", Path: "body.payload.liability_id", Type: "string", Widget: remittanceWidgetForForm(form, "select")},
			{Key: "adjustment_date", Label: "Adjustment Date", Path: "body.payload.adjustment_date", Type: "string", Widget: remittanceWidgetForForm(form, "text")},
			{Key: "amount", Label: "Amount", Path: "body.payload.amount", Type: "number", Widget: remittanceWidgetForForm(form, "text")},
			{Key: "reason", Label: "Reason", Path: "body.payload.reason", Type: "string", Widget: remittanceWidgetForForm(form, "textarea")},
		}
	case "payroll_remittance_batch":
		return []module.FieldDefinition{
			{Key: "liability_ids", Label: "Liabilities", Path: "body.payload.liability_ids", Type: "object", Widget: remittanceWidgetForForm(form, "taglist")},
			{Key: "remittance_authority_id", Label: "Authority", Path: "body.payload.remittance_authority_id", Type: "string", Widget: remittanceWidgetForForm(form, "select")},
			{Key: "payment_date", Label: "Payment Date", Path: "body.payload.payment_date", Type: "string", Widget: remittanceWidgetForForm(form, "text")},
			{Key: "total_amount", Label: "Total Amount", Path: "body.payload.total_amount", Type: "number", Widget: remittanceWidgetForForm(form, "text")},
			{Key: "posted_ledger_id", Label: "Posted Ledger", Path: "body.payload.posted_ledger_id", Type: "string", Widget: remittanceWidgetForForm(form, "text")},
		}
	default:
		return []module.FieldDefinition{
			{Key: "payroll_remittance_batch_id", Label: "Batch", Path: "body.payload.payroll_remittance_batch_id", Type: "string", Widget: remittanceWidgetForForm(form, "select")},
			{Key: "payment_date", Label: "Payment Date", Path: "body.payload.payment_date", Type: "string", Widget: remittanceWidgetForForm(form, "text")},
			{Key: "amount_paid", Label: "Amount Paid", Path: "body.payload.amount_paid", Type: "number", Widget: remittanceWidgetForForm(form, "text")},
			{Key: "treasury_account_id", Label: "Treasury Account", Path: "body.payload.treasury_account_id", Type: "string", Widget: remittanceWidgetForForm(form, "select")},
			{Key: "posted_ledger_id", Label: "Posted Ledger", Path: "body.payload.posted_ledger_id", Type: "string", Widget: remittanceWidgetForForm(form, "text")},
		}
	}
}

func remittanceWidgetForForm(form bool, widget string) string {
	if !form {
		return ""
	}
	return widget
}
