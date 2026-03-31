package app

import (
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func employeePayrollCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                  "employee_payroll_core",
		Name:                 "Employee Payroll Core",
		NameI18n:             localize("Employee Payroll Core", "Inti Payroll Karyawan"),
		Version:              "1.0.0",
		DomainFamily:         "business",
		Description:          "Compensation structures, payroll periods, payroll runs, and treasury-settled payroll payments.",
		DescriptionI18n:      localize("Compensation structures, payroll periods, payroll runs, and treasury-settled payroll payments.", "Struktur kompensasi, periode payroll, payroll run, dan pembayaran payroll melalui treasury."),
		BusinessCapabilities: []string{"pay components", "salary structures", "payroll periods", "payroll runs", "payroll payments"},
		OwnedDocumentTypes:   []string{"payroll_run", "payroll_adjustment", "payroll_payment_batch", "payroll_payment"},
		OwnedWorkflowKeys:    []string{"payroll_run_flow", "payroll_adjustment_flow", "payroll_payment_batch_flow", "payroll_payment_flow"},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "employee_workforce", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "workforce_attendance", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "employee_spend_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "workflow_approval_policy", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "treasury_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "finance_manual_journal_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "organization_structure", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Payroll Console",
			TitleI18n:       localize("Payroll Console", "Konsol Payroll"),
			Description:     "Compensation setup, payroll periods, payroll runs, and payroll payment operations.",
			DescriptionI18n: localize("Compensation setup, payroll periods, payroll runs, and payroll payment operations.", "Setup kompensasi, periode payroll, payroll run, dan operasi pembayaran payroll."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "payroll_setup",
					Title:     "Payroll Setup",
					TitleI18n: localize("Payroll Setup", "Setup Payroll"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("components", "Pay Components", "Komponen Gaji", "/ui/payroll/components", "Open pay components.", "Buka komponen gaji.", "pay_component.list"),
						adminConsoleLink("structures", "Salary Structures", "Struktur Gaji", "/ui/payroll/structures", "Open salary structures.", "Buka struktur gaji.", "salary_structure.list"),
						adminConsoleLink("structure_lines", "Structure Lines", "Baris Struktur", "/ui/payroll/structure-lines", "Open salary structure lines.", "Buka baris struktur gaji.", "salary_structure_line.list"),
						adminConsoleLink("profiles", "Employee Payroll Profiles", "Profil Payroll Karyawan", "/ui/payroll/profiles", "Open employee payroll profiles.", "Buka profil payroll karyawan.", "employee_payroll_profile.list"),
						adminConsoleLink("periods", "Payroll Periods", "Periode Payroll", "/ui/payroll/periods", "Open payroll periods.", "Buka periode payroll.", "payroll_period.list"),
						adminConsoleLink("tax_rules", "Tax Rules", "Aturan Pajak", "/ui/payroll/tax-rules", "Open payroll tax rules.", "Buka aturan pajak payroll.", "payroll_tax_rule.list"),
						adminConsoleLink("contribution_rules", "Contribution Rules", "Aturan Kontribusi", "/ui/payroll/contribution-rules", "Open payroll contribution rules.", "Buka aturan kontribusi payroll.", "payroll_contribution_rule.list"),
					},
				},
				{
					Key:       "payroll_operations",
					Title:     "Payroll Operations",
					TitleI18n: localize("Payroll Operations", "Operasi Payroll"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("runs", "Payroll Runs", "Payroll Run", "/ui/payroll/runs", "Open payroll runs.", "Buka payroll run.", "document.list"),
						adminConsoleLink("adjustments", "Payroll Adjustments", "Penyesuaian Payroll", "/ui/payroll/adjustments", "Open payroll adjustments.", "Buka penyesuaian payroll.", "document.list"),
						adminConsoleLink("batches", "Payroll Payment Batches", "Batch Pembayaran Payroll", "/ui/payroll/payment-batches", "Open payroll payment batches.", "Buka batch pembayaran payroll.", "document.list"),
						adminConsoleLink("payments", "Payroll Payments", "Pembayaran Payroll", "/ui/payroll/payments", "Open payroll payments.", "Buka pembayaran payroll.", "document.list"),
					},
				},
			},
		},
		Models: []model.Definition{
			payrollModelDefinition("pay_component", "Pay Component", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "component_class", Type: "string", DefaultValue: "earning"},
				{Key: "taxable", Type: "bool"},
				{Key: "contribution_applicable", Type: "bool"},
				{Key: "include_in_net_pay", Type: "bool", DefaultValue: true},
				{Key: "default_account_code", Type: "string"},
				{Key: "payable_account_code", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			payrollModelDefinition("salary_structure", "Salary Structure", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "currency_code", Type: "string", DefaultValue: "IDR"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			payrollModelDefinition("salary_structure_line", "Salary Structure Line", []model.FieldDefinition{
				{Key: "salary_structure_id", Type: "string", Required: true},
				{Key: "component_code", Type: "string", Required: true},
				{Key: "sequence", Type: "number"},
				{Key: "formula_key", Type: "string", DefaultValue: "fixed_amount"},
				{Key: "fixed_amount", Type: "number"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			payrollModelDefinition("employee_payroll_profile", "Employee Payroll Profile", []model.FieldDefinition{
				{Key: "employee_id", Type: "string", Required: true},
				{Key: "salary_structure_id", Type: "string"},
				{Key: "currency_code", Type: "string", DefaultValue: "IDR"},
				{Key: "payment_method_code", Type: "string", DefaultValue: "BANK"},
				{Key: "treasury_account_id", Type: "string"},
				{Key: "payroll_party_id", Type: "string"},
				{Key: "tax_rule_id", Type: "string"},
				{Key: "contribution_rule_id", Type: "string"},
				{Key: "leave_deduction_daily_rate", Type: "number"},
				{Key: "reimbursement_in_payroll", Type: "bool"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			payrollModelDefinition("payroll_period", "Payroll Period", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "organization_id", Type: "string", Required: true},
				{Key: "location_id", Type: "string"},
				{Key: "start_date", Type: "string", Required: true},
				{Key: "end_date", Type: "string", Required: true},
				{Key: "pay_date", Type: "string", Required: true},
				{Key: "status", Type: "string", DefaultValue: "open"},
			}),
			payrollModelDefinition("payroll_tax_rule", "Payroll Tax Rule", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "employee_rate_percent", Type: "number"},
				{Key: "employer_rate_percent", Type: "number"},
				{Key: "fixed_amount", Type: "number"},
				{Key: "threshold_amount", Type: "number"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			payrollModelDefinition("payroll_contribution_rule", "Payroll Contribution Rule", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "employee_rate_percent", Type: "number"},
				{Key: "employee_fixed_amount", Type: "number"},
				{Key: "employer_rate_percent", Type: "number"},
				{Key: "employer_fixed_amount", Type: "number"},
				{Key: "threshold_amount", Type: "number"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
		},
		Documents: []document.Definition{
			payrollDocumentDefinition("payroll_run", "Payroll Run", "payroll_run_flow", "payroll_run_number"),
			payrollDocumentDefinition("payroll_adjustment", "Payroll Adjustment", "payroll_adjustment_flow", "payroll_adjustment_number"),
			payrollDocumentDefinition("payroll_payment_batch", "Payroll Payment Batch", "payroll_payment_batch_flow", "payroll_payment_batch_number"),
			payrollDocumentDefinition("payroll_payment", "Payroll Payment", "payroll_payment_flow", "payroll_payment_number"),
		},
		Workflows: []workflow.Definition{
			commercialWorkflowDefinition("payroll_run_flow", "processed", true),
			commercialWorkflowDefinition("payroll_adjustment_flow", "approved", true),
			commercialWorkflowDefinition("payroll_payment_batch_flow", "paid", true),
			commercialWorkflowDefinition("payroll_payment_flow", "paid", true),
		},
		Datasets: []module.DatasetDefinition{
			{Key: "payroll.period.summary", Title: "Payroll Period Summary", TitleI18n: localize("Payroll Period Summary", "Ringkasan Periode Payroll"), SourceKind: "model", ModelKey: "payroll_period", Dimensions: []module.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localize("By Status", "Berdasarkan Status"), Path: "status"}}, Measures: []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}}},
			{Key: "payroll.profile.summary", Title: "Payroll Profile Summary", TitleI18n: localize("Payroll Profile Summary", "Ringkasan Profil Payroll"), SourceKind: "model", ModelKey: "employee_payroll_profile", Dimensions: []module.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localize("By Status", "Berdasarkan Status"), Path: "status"}}, Measures: []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}}},
		},
		SearchIndexes: append([]search.IndexDefinition{
			commercialModelSearchIndex("payroll.components.search", "Pay Component Search", "pay_component", "payroll.components.list", []string{"code", "name", "component_class", "status"}),
			commercialModelSearchIndex("payroll.structures.search", "Salary Structure Search", "salary_structure", "payroll.structures.list", []string{"code", "name", "status"}),
			commercialModelSearchIndex("payroll.structure_lines.search", "Salary Structure Line Search", "salary_structure_line", "payroll.structure_lines.list", []string{"salary_structure_id", "component_code", "formula_key", "status"}),
			commercialModelSearchIndex("payroll.profiles.search", "Employee Payroll Profile Search", "employee_payroll_profile", "payroll.profiles.list", []string{"employee_id", "salary_structure_id", "status"}),
			commercialModelSearchIndex("payroll.periods.search", "Payroll Period Search", "payroll_period", "payroll.periods.list", []string{"code", "name", "start_date", "end_date", "status"}),
			commercialModelSearchIndex("payroll.tax_rules.search", "Payroll Tax Rule Search", "payroll_tax_rule", "payroll.tax_rules.list", []string{"code", "name", "status"}),
			commercialModelSearchIndex("payroll.contribution_rules.search", "Payroll Contribution Rule Search", "payroll_contribution_rule", "payroll.contribution_rules.list", []string{"code", "name", "status"}),
		}, payrollDocumentSearchIndexes()...),
		Security: module.SecurityDefinition{
			Permissions: append(
				append(
					append(
						commercialModelPermissions("pay_component", "Pay Component"),
						commercialModelPermissions("salary_structure", "Salary Structure")...,
					),
					append(
						commercialModelPermissions("salary_structure_line", "Salary Structure Line"),
						append(
							commercialModelPermissions("employee_payroll_profile", "Employee Payroll Profile"),
							append(commercialModelPermissions("payroll_period", "Payroll Period"),
								append(commercialModelPermissions("payroll_tax_rule", "Payroll Tax Rule"), commercialModelPermissions("payroll_contribution_rule", "Payroll Contribution Rule")...)...)...,
						)...,
					)...,
				),
				module.PermissionDefinition{Key: "payroll.read", Action: "read", Resource: "payroll", DisplayName: "Read Payroll", DisplayNameI18n: localize("Read Payroll", "Lihat Payroll")},
			),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:           "payroll_manager",
					Name:          "Payroll Manager",
					NameI18n:      localize("Payroll Manager", "Manajer Payroll"),
					AllowedScopes: []string{"deployment", "organization", "location"},
					PermissionKeys: []string{
						"pay_component.create", "pay_component.list", "pay_component.read", "pay_component.update",
						"salary_structure.create", "salary_structure.list", "salary_structure.read", "salary_structure.update",
						"salary_structure_line.create", "salary_structure_line.list", "salary_structure_line.read", "salary_structure_line.update",
						"employee_payroll_profile.create", "employee_payroll_profile.list", "employee_payroll_profile.read", "employee_payroll_profile.update",
						"payroll_period.create", "payroll_period.list", "payroll_period.read", "payroll_period.update",
						"payroll_tax_rule.create", "payroll_tax_rule.list", "payroll_tax_rule.read", "payroll_tax_rule.update",
						"payroll_contribution_rule.create", "payroll_contribution_rule.list", "payroll_contribution_rule.read", "payroll_contribution_rule.update",
						"payroll.read",
						"document.create", "document.list", "document.read", "document.update_draft", "document.submit", "document.approve", "document.reject", "document.reopen", "document.cancel",
					},
				},
				{
					Key:            "payroll_approver",
					Name:           "Payroll Approver",
					NameI18n:       localize("Payroll Approver", "Penyetuju Payroll"),
					AllowedScopes:  []string{"deployment", "organization", "location"},
					PermissionKeys: []string{"payroll.read", "document.list", "document.read", "document.approve", "document.reject"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "payroll.components", Label: "Pay Components", LabelI18n: localize("Pay Components", "Komponen Gaji"), ActionKey: "payroll.components.list", Order: 30, RequiredPermissions: []string{"pay_component.list"}},
				{Key: "payroll.structures", Label: "Salary Structures", LabelI18n: localize("Salary Structures", "Struktur Gaji"), ActionKey: "payroll.structures.list", Order: 31, RequiredPermissions: []string{"salary_structure.list"}},
				{Key: "payroll.structure_lines", Label: "Structure Lines", LabelI18n: localize("Structure Lines", "Baris Struktur"), ActionKey: "payroll.structure_lines.list", Order: 32, RequiredPermissions: []string{"salary_structure_line.list"}},
				{Key: "payroll.profiles", Label: "Payroll Profiles", LabelI18n: localize("Payroll Profiles", "Profil Payroll"), ActionKey: "payroll.profiles.list", Order: 33, RequiredPermissions: []string{"employee_payroll_profile.list"}},
				{Key: "payroll.periods", Label: "Payroll Periods", LabelI18n: localize("Payroll Periods", "Periode Payroll"), ActionKey: "payroll.periods.list", Order: 34, RequiredPermissions: []string{"payroll_period.list"}},
				{Key: "payroll.tax_rules", Label: "Tax Rules", LabelI18n: localize("Tax Rules", "Aturan Pajak"), ActionKey: "payroll.tax_rules.list", Order: 35, RequiredPermissions: []string{"payroll_tax_rule.list"}},
				{Key: "payroll.contribution_rules", Label: "Contribution Rules", LabelI18n: localize("Contribution Rules", "Aturan Kontribusi"), ActionKey: "payroll.contribution_rules.list", Order: 36, RequiredPermissions: []string{"payroll_contribution_rule.list"}},
				{Key: "payroll.runs", Label: "Payroll Runs", LabelI18n: localize("Payroll Runs", "Payroll Run"), ActionKey: "payroll.runs.list", Order: 37, RequiredPermissions: []string{"document.list"}},
				{Key: "payroll.adjustments", Label: "Payroll Adjustments", LabelI18n: localize("Payroll Adjustments", "Penyesuaian Payroll"), ActionKey: "payroll.adjustments.list", Order: 38, RequiredPermissions: []string{"document.list"}},
				{Key: "payroll.payment_batches", Label: "Payroll Payment Batches", LabelI18n: localize("Payroll Payment Batches", "Batch Pembayaran Payroll"), ActionKey: "payroll.payment_batches.list", Order: 39, RequiredPermissions: []string{"document.list"}},
				{Key: "payroll.payments", Label: "Payroll Payments", LabelI18n: localize("Payroll Payments", "Pembayaran Payroll"), ActionKey: "payroll.payments.list", Order: 40, RequiredPermissions: []string{"document.list"}},
			},
			Actions: payrollActions(),
			Views:   payrollViews(),
		},
	}
}

func payrollDocumentDefinition(documentType, displayName, workflowKey, numberingKey string) document.Definition {
	return document.Definition{
		Type:                   documentType,
		DisplayName:            displayName,
		SchemaVersion:          "v1",
		WorkflowKey:            workflowKey,
		NumberingKey:           numberingKey,
		OwnerModuleKey:         "employee_payroll_core",
		AllowedLinkTypes:       []string{"related_to", "posting_for", "payment_for", "run_for", "batch_for"},
		AllowedAttachmentTypes: []string{"note", "image", "document"},
	}
}

func payrollModelDefinition(key, displayName string, fields []model.FieldDefinition) model.Definition {
	return model.Definition{
		Key:                 key,
		DisplayName:         displayName,
		OwnerModuleKey:      "employee_payroll_core",
		Version:             "v1",
		CreatePermissionKey: key + ".create",
		ListPermissionKey:   key + ".list",
		ReadPermissionKey:   key + ".read",
		UpdatePermissionKey: key + ".update",
		DefaultSort:         payrollFieldSortKey(fields),
		Fields:              fields,
	}
}

func payrollFieldSortKey(fields []model.FieldDefinition) string {
	for _, candidate := range []string{"code", "name", "employee_id", "start_date"} {
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

func payrollDocumentSearchIndexes() []search.IndexDefinition {
	return []search.IndexDefinition{
		commercialDocumentSearchIndex("payroll.runs.search", "Payroll Run Search", "payroll_run", "payroll.runs.list", []search.IndexFieldDefinition{{Key: "payroll_period_id", Path: "body.payload.payroll_period_id", Type: "string", Searchable: true}, {Key: "pay_date", Path: "body.payload.pay_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
		commercialDocumentSearchIndex("payroll.adjustments.search", "Payroll Adjustment Search", "payroll_adjustment", "payroll.adjustments.list", []search.IndexFieldDefinition{{Key: "employee_id", Path: "body.payload.employee_id", Type: "string", Searchable: true}, {Key: "adjustment_date", Path: "body.payload.adjustment_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
		commercialDocumentSearchIndex("payroll.payment_batches.search", "Payroll Payment Batch Search", "payroll_payment_batch", "payroll.payment_batches.list", []search.IndexFieldDefinition{{Key: "payroll_run_id", Path: "body.payload.payroll_run_id", Type: "string", Searchable: true}, {Key: "payment_date", Path: "body.payload.payment_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
		commercialDocumentSearchIndex("payroll.payments.search", "Payroll Payment Search", "payroll_payment", "payroll.payments.list", []search.IndexFieldDefinition{{Key: "employee_id", Path: "body.payload.employee_id", Type: "string", Searchable: true}, {Key: "payment_date", Path: "body.payload.payment_date", Type: "string", Searchable: true}, {Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true}}),
	}
}

func payrollModelActions(prefix, listLabel, detailLabel, formLabel, modelKey string) []module.ActionDefinition {
	return []module.ActionDefinition{
		{Key: "payroll." + prefix + ".list", Label: listLabel, LabelI18n: localize(listLabel, listLabel), Kind: "navigate", RoutePath: "/payroll/" + prefix, ViewKey: "payroll." + prefix + ".list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".list"}},
		{Key: "payroll." + prefix + ".detail", Label: detailLabel, LabelI18n: localize(detailLabel, detailLabel), Kind: "navigate", RoutePath: "/payroll/" + prefix + "/detail", ViewKey: "payroll." + prefix + ".detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".read"}},
		{Key: "payroll." + prefix + ".form", Label: formLabel, LabelI18n: localize(formLabel, formLabel), Kind: "navigate", RoutePath: "/payroll/" + prefix + "/form", ViewKey: "payroll." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".update"}},
	}
}

func payrollModelViews(prefix, title, modelKey string, columns []module.ColumnDefinition, fields []module.FieldDefinition) []module.ViewDefinition {
	return []module.ViewDefinition{
		commercialModelListView("payroll."+prefix+".list", title, modelKey, columns, []string{"active", "inactive", "open", "locked", "processed"}),
		commercialModelDetailView("payroll."+prefix+".detail", title+" Detail", modelKey, fields),
		commercialModelFormView("payroll."+prefix+".form", title+" Form", modelKey, fields),
	}
}

func payrollActions() []module.ActionDefinition {
	actions := []module.ActionDefinition{}
	actions = append(actions, payrollModelActions("components", "Pay Components", "Pay Component Detail", "Pay Component Form", "pay_component")...)
	actions = append(actions, payrollModelActions("structures", "Salary Structures", "Salary Structure Detail", "Salary Structure Form", "salary_structure")...)
	actions = append(actions, payrollModelActions("structure_lines", "Structure Lines", "Structure Line Detail", "Structure Line Form", "salary_structure_line")...)
	actions = append(actions, payrollModelActions("profiles", "Payroll Profiles", "Payroll Profile Detail", "Payroll Profile Form", "employee_payroll_profile")...)
	actions = append(actions, payrollModelActions("periods", "Payroll Periods", "Payroll Period Detail", "Payroll Period Form", "payroll_period")...)
	actions = append(actions, payrollModelActions("tax_rules", "Tax Rules", "Tax Rule Detail", "Tax Rule Form", "payroll_tax_rule")...)
	actions = append(actions, payrollModelActions("contribution_rules", "Contribution Rules", "Contribution Rule Detail", "Contribution Rule Form", "payroll_contribution_rule")...)
	actions = append(actions, commercialDocumentActions("payroll.runs", "payroll_run", "Payroll Runs", "Payroll Run Detail", "New Payroll Run", "/payroll/runs")...)
	actions = append(actions, commercialDocumentActions("payroll.adjustments", "payroll_adjustment", "Payroll Adjustments", "Payroll Adjustment Detail", "New Payroll Adjustment", "/payroll/adjustments")...)
	actions = append(actions, commercialDocumentActions("payroll.payment_batches", "payroll_payment_batch", "Payroll Payment Batches", "Payroll Payment Batch Detail", "New Payroll Payment Batch", "/payroll/payment-batches")...)
	actions = append(actions, commercialDocumentActions("payroll.payments", "payroll_payment", "Payroll Payments", "Payroll Payment Detail", "New Payroll Payment", "/payroll/payments")...)
	return actions
}

func payrollViews() []module.ViewDefinition {
	views := []module.ViewDefinition{}
	views = append(views, payrollModelViews("components", "Pay Components", "pay_component", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "component_class", Label: "Class", Path: "values.component_class"}, {Key: "status", Label: "Status", Path: "values.status"}}, payrollSetupFields("pay_component"))...)
	views = append(views, payrollModelViews("structures", "Salary Structures", "salary_structure", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "currency_code", Label: "Currency", Path: "values.currency_code"}, {Key: "status", Label: "Status", Path: "values.status"}}, payrollSetupFields("salary_structure"))...)
	views = append(views, payrollModelViews("structure_lines", "Structure Lines", "salary_structure_line", []module.ColumnDefinition{{Key: "salary_structure_id", Label: "Structure", Path: "values.salary_structure_id"}, {Key: "component_code", Label: "Component", Path: "values.component_code"}, {Key: "formula_key", Label: "Formula", Path: "values.formula_key"}, {Key: "status", Label: "Status", Path: "values.status"}}, payrollSetupFields("salary_structure_line"))...)
	views = append(views, payrollModelViews("profiles", "Payroll Profiles", "employee_payroll_profile", []module.ColumnDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id"}, {Key: "salary_structure_id", Label: "Structure", Path: "values.salary_structure_id"}, {Key: "payment_method_code", Label: "Method", Path: "values.payment_method_code"}, {Key: "status", Label: "Status", Path: "values.status"}}, payrollSetupFields("employee_payroll_profile"))...)
	views = append(views, payrollModelViews("periods", "Payroll Periods", "payroll_period", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "start_date", Label: "Start", Path: "values.start_date"}, {Key: "end_date", Label: "End", Path: "values.end_date"}, {Key: "status", Label: "Status", Path: "values.status"}}, payrollSetupFields("payroll_period"))...)
	views = append(views, payrollModelViews("tax_rules", "Tax Rules", "payroll_tax_rule", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "employee_rate_percent", Label: "Employee Rate", Path: "values.employee_rate_percent"}, {Key: "status", Label: "Status", Path: "values.status"}}, payrollSetupFields("payroll_tax_rule"))...)
	views = append(views, payrollModelViews("contribution_rules", "Contribution Rules", "payroll_contribution_rule", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "employee_rate_percent", Label: "Employee Rate", Path: "values.employee_rate_percent"}, {Key: "status", Label: "Status", Path: "values.status"}}, payrollSetupFields("payroll_contribution_rule"))...)
	views = append(views, commercialDocumentViews("payroll.runs", "payroll_run", "Payroll Runs", "Payroll Run Detail", "Payroll Run Form", payrollDocumentColumns("payroll_run"), []string{"draft", "submitted", "processed", "rejected", "cancelled"}, payrollDocumentSections("payroll_run"), payrollDocumentFormSections("payroll_run"))...)
	views = append(views, commercialDocumentViews("payroll.adjustments", "payroll_adjustment", "Payroll Adjustments", "Payroll Adjustment Detail", "Payroll Adjustment Form", payrollDocumentColumns("payroll_adjustment"), []string{"draft", "submitted", "approved", "rejected", "cancelled"}, payrollDocumentSections("payroll_adjustment"), payrollDocumentFormSections("payroll_adjustment"))...)
	views = append(views, commercialDocumentViews("payroll.payment_batches", "payroll_payment_batch", "Payroll Payment Batches", "Payroll Payment Batch Detail", "Payroll Payment Batch Form", payrollDocumentColumns("payroll_payment_batch"), []string{"draft", "submitted", "paid", "rejected", "cancelled"}, payrollDocumentSections("payroll_payment_batch"), payrollDocumentFormSections("payroll_payment_batch"))...)
	views = append(views, commercialDocumentViews("payroll.payments", "payroll_payment", "Payroll Payments", "Payroll Payment Detail", "Payroll Payment Form", payrollDocumentColumns("payroll_payment"), []string{"draft", "submitted", "paid", "rejected", "cancelled"}, payrollDocumentSections("payroll_payment"), payrollDocumentFormSections("payroll_payment"))...)
	return views
}

func payrollSetupFields(modelKey string) []module.FieldDefinition {
	switch modelKey {
	case "pay_component":
		return []module.FieldDefinition{{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "component_class", Label: "Class", Path: "values.component_class", Type: "string", Widget: payrollWidgetForForm(true, "select")}, {Key: "default_account_code", Label: "Default Account", Path: "values.default_account_code", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: payrollWidgetForForm(true, "select")}}
	case "salary_structure":
		return []module.FieldDefinition{{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "currency_code", Label: "Currency", Path: "values.currency_code", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: payrollWidgetForForm(true, "select")}}
	case "salary_structure_line":
		return []module.FieldDefinition{{Key: "salary_structure_id", Label: "Salary Structure", Path: "values.salary_structure_id", Type: "string", Widget: payrollWidgetForForm(true, "select")}, {Key: "component_code", Label: "Component", Path: "values.component_code", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "sequence", Label: "Sequence", Path: "values.sequence", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "formula_key", Label: "Formula", Path: "values.formula_key", Type: "string", Widget: payrollWidgetForForm(true, "select")}, {Key: "fixed_amount", Label: "Fixed Amount", Path: "values.fixed_amount", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: payrollWidgetForForm(true, "select")}}
	case "employee_payroll_profile":
		return []module.FieldDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id", Type: "string", Widget: payrollWidgetForForm(true, "select")}, {Key: "salary_structure_id", Label: "Salary Structure", Path: "values.salary_structure_id", Type: "string", Widget: payrollWidgetForForm(true, "select")}, {Key: "payment_method_code", Label: "Payment Method", Path: "values.payment_method_code", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "treasury_account_id", Label: "Treasury Account", Path: "values.treasury_account_id", Type: "string", Widget: payrollWidgetForForm(true, "select")}, {Key: "tax_rule_id", Label: "Tax Rule", Path: "values.tax_rule_id", Type: "string", Widget: payrollWidgetForForm(true, "select")}, {Key: "contribution_rule_id", Label: "Contribution Rule", Path: "values.contribution_rule_id", Type: "string", Widget: payrollWidgetForForm(true, "select")}, {Key: "leave_deduction_daily_rate", Label: "Leave Deduction Daily Rate", Path: "values.leave_deduction_daily_rate", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "reimbursement_in_payroll", Label: "Reimbursement In Payroll", Path: "values.reimbursement_in_payroll", Type: "bool", Widget: payrollWidgetForForm(true, "checkbox")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: payrollWidgetForForm(true, "select")}}
	case "payroll_period":
		return []module.FieldDefinition{{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "organization_id", Label: "Organization", Path: "values.organization_id", Type: "string", Widget: payrollWidgetForForm(true, "select")}, {Key: "location_id", Label: "Location", Path: "values.location_id", Type: "string", Widget: payrollWidgetForForm(true, "select")}, {Key: "start_date", Label: "Start Date", Path: "values.start_date", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "end_date", Label: "End Date", Path: "values.end_date", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "pay_date", Label: "Pay Date", Path: "values.pay_date", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: payrollWidgetForForm(true, "select")}}
	case "payroll_tax_rule":
		return []module.FieldDefinition{{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "employee_rate_percent", Label: "Employee Rate", Path: "values.employee_rate_percent", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "employer_rate_percent", Label: "Employer Rate", Path: "values.employer_rate_percent", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "fixed_amount", Label: "Fixed Amount", Path: "values.fixed_amount", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "threshold_amount", Label: "Threshold", Path: "values.threshold_amount", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: payrollWidgetForForm(true, "select")}}
	case "payroll_contribution_rule":
		return []module.FieldDefinition{{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: payrollWidgetForForm(true, "text")}, {Key: "employee_rate_percent", Label: "Employee Rate", Path: "values.employee_rate_percent", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "employee_fixed_amount", Label: "Employee Fixed Amount", Path: "values.employee_fixed_amount", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "employer_rate_percent", Label: "Employer Rate", Path: "values.employer_rate_percent", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "employer_fixed_amount", Label: "Employer Fixed Amount", Path: "values.employer_fixed_amount", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "threshold_amount", Label: "Threshold", Path: "values.threshold_amount", Type: "number", Widget: payrollWidgetForForm(true, "text")}, {Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: payrollWidgetForForm(true, "select")}}
	default:
		return nil
	}
}

func payrollDocumentColumns(documentType string) []module.ColumnDefinition {
	switch documentType {
	case "payroll_run":
		return []module.ColumnDefinition{{Key: "number", Label: "Run", Path: "header.number"}, {Key: "payroll_period_id", Label: "Period", Path: "body.payload.payroll_period_id"}, {Key: "pay_date", Label: "Pay Date", Path: "body.payload.pay_date"}, {Key: "employee_count", Label: "Employees", Path: "body.payload.employee_count"}, {Key: "net_pay_total", Label: "Net Pay", Path: "body.payload.net_pay_total"}, {Key: "status", Label: "Status", Path: "header.status"}}
	case "payroll_adjustment":
		return []module.ColumnDefinition{{Key: "number", Label: "Adjustment", Path: "header.number"}, {Key: "employee_id", Label: "Employee", Path: "body.payload.employee_id"}, {Key: "adjustment_date", Label: "Date", Path: "body.payload.adjustment_date"}, {Key: "amount", Label: "Amount", Path: "body.payload.amount"}, {Key: "status", Label: "Status", Path: "header.status"}}
	case "payroll_payment_batch":
		return []module.ColumnDefinition{{Key: "number", Label: "Batch", Path: "header.number"}, {Key: "payroll_run_id", Label: "Run", Path: "body.payload.payroll_run_id"}, {Key: "payment_date", Label: "Payment Date", Path: "body.payload.payment_date"}, {Key: "employee_count", Label: "Employees", Path: "body.payload.employee_count"}, {Key: "total_amount", Label: "Total", Path: "body.payload.total_amount"}, {Key: "status", Label: "Status", Path: "header.status"}}
	default:
		return []module.ColumnDefinition{{Key: "number", Label: "Payment", Path: "header.number"}, {Key: "employee_id", Label: "Employee", Path: "body.payload.employee_id"}, {Key: "payment_date", Label: "Payment Date", Path: "body.payload.payment_date"}, {Key: "net_pay", Label: "Net Pay", Path: "body.payload.net_pay"}, {Key: "status", Label: "Status", Path: "header.status"}}
	}
}

func payrollDocumentSections(documentType string) []module.SectionDefinition {
	return []module.SectionDefinition{{
		Key:       "summary",
		Title:     "Summary",
		TitleI18n: localize("Summary", "Ringkasan"),
		Fields:    payrollDocumentFields(documentType, false),
	}}
}

func payrollDocumentFormSections(documentType string) []module.SectionDefinition {
	return []module.SectionDefinition{{
		Key:       "edit",
		Title:     "Edit",
		TitleI18n: localize("Edit", "Ubah"),
		Fields:    payrollDocumentFields(documentType, true),
	}}
}

func payrollDocumentFields(documentType string, form bool) []module.FieldDefinition {
	switch documentType {
	case "payroll_run":
		return []module.FieldDefinition{
			{Key: "payroll_period_id", Label: "Payroll Period", Path: "body.payload.payroll_period_id", Type: "string", Widget: payrollWidgetForForm(form, "select")},
			{Key: "period_start_date", Label: "Start Date", Path: "body.payload.period_start_date", Type: "string", Widget: payrollWidgetForForm(form, "text")},
			{Key: "period_end_date", Label: "End Date", Path: "body.payload.period_end_date", Type: "string", Widget: payrollWidgetForForm(form, "text")},
			{Key: "pay_date", Label: "Pay Date", Path: "body.payload.pay_date", Type: "string", Widget: payrollWidgetForForm(form, "text")},
			{Key: "employee_ids", Label: "Employees", Path: "body.payload.employee_ids", Type: "object", Widget: payrollWidgetForForm(form, "taglist")},
			{Key: "net_pay_total", Label: "Net Pay Total", Path: "body.payload.net_pay_total", Type: "number", Widget: payrollWidgetForForm(form, "text")},
			{Key: "payroll_lines", Label: "Payroll Lines", Path: "body.payload.payroll_lines", Type: "object", Widget: "json"},
		}
	case "payroll_adjustment":
		return []module.FieldDefinition{
			{Key: "employee_id", Label: "Employee", Path: "body.payload.employee_id", Type: "string", Widget: payrollWidgetForForm(form, "select")},
			{Key: "adjustment_date", Label: "Adjustment Date", Path: "body.payload.adjustment_date", Type: "string", Widget: payrollWidgetForForm(form, "text")},
			{Key: "amount", Label: "Amount", Path: "body.payload.amount", Type: "number", Widget: payrollWidgetForForm(form, "text")},
			{Key: "reason", Label: "Reason", Path: "body.payload.reason", Type: "string", Widget: payrollWidgetForForm(form, "textarea")},
		}
	case "payroll_payment_batch":
		return []module.FieldDefinition{
			{Key: "payroll_run_id", Label: "Payroll Run", Path: "body.payload.payroll_run_id", Type: "string", Widget: payrollWidgetForForm(form, "select")},
			{Key: "payment_date", Label: "Payment Date", Path: "body.payload.payment_date", Type: "string", Widget: payrollWidgetForForm(form, "text")},
			{Key: "employee_count", Label: "Employee Count", Path: "body.payload.employee_count", Type: "number", Widget: payrollWidgetForForm(form, "text")},
			{Key: "total_amount", Label: "Total Amount", Path: "body.payload.total_amount", Type: "number", Widget: payrollWidgetForForm(form, "text")},
			{Key: "ledger_posting_id", Label: "Ledger Posting", Path: "body.payload.ledger_posting_id", Type: "string", Widget: payrollWidgetForForm(form, "text")},
		}
	default:
		return []module.FieldDefinition{
			{Key: "payroll_run_id", Label: "Payroll Run", Path: "body.payload.payroll_run_id", Type: "string", Widget: payrollWidgetForForm(form, "select")},
			{Key: "payroll_payment_batch_id", Label: "Payment Batch", Path: "body.payload.payroll_payment_batch_id", Type: "string", Widget: payrollWidgetForForm(form, "select")},
			{Key: "employee_id", Label: "Employee", Path: "body.payload.employee_id", Type: "string", Widget: payrollWidgetForForm(form, "select")},
			{Key: "payment_date", Label: "Payment Date", Path: "body.payload.payment_date", Type: "string", Widget: payrollWidgetForForm(form, "text")},
			{Key: "net_pay", Label: "Net Pay", Path: "body.payload.net_pay", Type: "number", Widget: payrollWidgetForForm(form, "text")},
			{Key: "treasury_account_id", Label: "Treasury Account", Path: "body.payload.treasury_account_id", Type: "string", Widget: payrollWidgetForForm(form, "select")},
		}
	}
}

func payrollWidgetForForm(form bool, widget string) string {
	if !form {
		return ""
	}
	return widget
}
