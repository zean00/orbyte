package app

import (
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

func leavePolicyCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                  "leave_policy_core",
		Name:                 "Leave Policy Core",
		NameI18n:             localize("Leave Policy Core", "Inti Kebijakan Cuti"),
		Version:              "1.0.0",
		DomainFamily:         "business",
		Description:          "Leave policy master, entitlement rules, balance accounts, accrual runs, and employee self-service leave balance and request management.",
		DescriptionI18n:      localize("Leave policy master, entitlement rules, balance accounts, accrual runs, and employee self-service leave balance and request management.", "Master kebijakan cuti, aturan hak cuti, akun saldo, proses akrual, dan layanan mandiri saldo serta permintaan cuti karyawan."),
		BusinessCapabilities: []string{"leave policies", "leave entitlement rules", "leave balances", "leave accrual runs", "leave self-service"},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "employee_workforce", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "workforce_attendance", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "workflow_approval_policy", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "employee_payroll_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "organization_structure", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Leave Console",
			TitleI18n:       localize("Leave Console", "Konsol Cuti"),
			Description:     "Leave policy, entitlement, balance, and accrual setup.",
			DescriptionI18n: localize("Leave policy, entitlement, balance, and accrual setup.", "Setup kebijakan, hak cuti, saldo, dan akrual."),
			Sections: []module.AdminConsoleSectionDefinition{{
				Key:       "leave_policy_setup",
				Title:     "Leave Setup",
				TitleI18n: localize("Leave Setup", "Setup Cuti"),
				Kind:      module.AdminConsoleSectionResourceLinks,
				Links: []module.AdminConsoleLinkDefinition{
					adminConsoleLink("policies", "Leave Policies", "Kebijakan Cuti", "/ui/leave/policies", "Open leave policies.", "Buka kebijakan cuti.", "leave_policy.list"),
					adminConsoleLink("rules", "Entitlement Rules", "Aturan Hak Cuti", "/ui/leave/entitlement-rules", "Open leave entitlement rules.", "Buka aturan hak cuti.", "leave_entitlement_rule.list"),
					adminConsoleLink("profiles", "Employee Leave Profiles", "Profil Cuti Karyawan", "/ui/leave/profiles", "Open employee leave profiles.", "Buka profil cuti karyawan.", "employee_leave_profile.list"),
					adminConsoleLink("accounts", "Leave Balance Accounts", "Akun Saldo Cuti", "/ui/leave/balance-accounts", "Open leave balance accounts.", "Buka akun saldo cuti.", "leave_balance_account.list"),
					adminConsoleLink("entries", "Leave Balance Entries", "Entri Saldo Cuti", "/ui/leave/balance-entries", "Open leave balance entries.", "Buka entri saldo cuti.", "leave_balance_entry.list"),
					adminConsoleLink("accrual_runs", "Leave Accrual Runs", "Proses Akrual Cuti", "/ui/leave/accrual-runs", "Open leave accrual runs.", "Buka proses akrual cuti.", "leave_accrual_run.list"),
					adminConsoleLink("adjustments", "Leave Balance Adjustments", "Penyesuaian Saldo Cuti", "/ui/leave/balance-adjustments", "Open leave balance adjustments.", "Buka penyesuaian saldo cuti.", "leave_balance_adjustment.list"),
				},
			}},
		},
		Models: []model.Definition{
			leavePolicyModelDefinition("leave_policy", "Leave Policy", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "absence_code_id", Type: "string", Required: true},
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "paid_leave", Type: "bool", DefaultValue: true},
				{Key: "requires_balance", Type: "bool", DefaultValue: true},
				{Key: "allows_half_day", Type: "bool"},
				{Key: "notice_days", Type: "number"},
				{Key: "approval_policy_id", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			leavePolicyModelDefinition("leave_entitlement_rule", "Leave Entitlement Rule", []model.FieldDefinition{
				{Key: "leave_policy_id", Type: "string", Required: true},
				{Key: "grant_mode", Type: "string", DefaultValue: "annual_grant"},
				{Key: "annual_entitlement_days", Type: "number"},
				{Key: "monthly_accrual_days", Type: "number"},
				{Key: "carry_forward_cap_days", Type: "number"},
				{Key: "carry_forward_expiry_rule", Type: "string"},
				{Key: "prorate_on_join", Type: "bool"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			leavePolicyModelDefinition("employee_leave_profile", "Employee Leave Profile", []model.FieldDefinition{
				{Key: "employee_id", Type: "string", Required: true},
				{Key: "leave_policy_id", Type: "string", Required: true},
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "effective_from", Type: "string"},
				{Key: "effective_to", Type: "string"},
				{Key: "opening_balance_days", Type: "number"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			leavePolicyModelDefinition("leave_balance_account", "Leave Balance Account", []model.FieldDefinition{
				{Key: "employee_id", Type: "string", Required: true},
				{Key: "leave_policy_id", Type: "string", Required: true},
				{Key: "employee_leave_profile_id", Type: "string"},
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "current_balance_days", Type: "number"},
				{Key: "reserved_days", Type: "number"},
				{Key: "available_days", Type: "number"},
				{Key: "carry_forward_balance_days", Type: "number"},
				{Key: "last_accrual_date", Type: "string"},
				{Key: "carry_forward_expiry_date", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			leavePolicyModelDefinition("leave_balance_entry", "Leave Balance Entry", []model.FieldDefinition{
				{Key: "balance_account_id", Type: "string", Required: true},
				{Key: "employee_id", Type: "string"},
				{Key: "leave_policy_id", Type: "string"},
				{Key: "employee_leave_profile_id", Type: "string"},
				{Key: "leave_request_id", Type: "string"},
				{Key: "accrual_run_id", Type: "string"},
				{Key: "entry_type", Type: "string", Required: true},
				{Key: "days", Type: "number", Required: true},
				{Key: "carry_forward_days_delta", Type: "number"},
				{Key: "reversal_of_entry_id", Type: "string"},
				{Key: "effective_date", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			leavePolicyModelDefinition("leave_accrual_run", "Leave Accrual Run", []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "leave_policy_id", Type: "string"},
				{Key: "run_mode", Type: "string", DefaultValue: "annual_grant"},
				{Key: "effective_date", Type: "string", Required: true},
				{Key: "run_status", Type: "string", DefaultValue: "draft"},
				{Key: "processed_at", Type: "string"},
				{Key: "processed_by", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
			leavePolicyModelDefinition("leave_balance_adjustment", "Leave Balance Adjustment", []model.FieldDefinition{
				{Key: "balance_account_id", Type: "string", Required: true},
				{Key: "leave_policy_id", Type: "string"},
				{Key: "employee_id", Type: "string"},
				{Key: "days", Type: "number", Required: true},
				{Key: "reason_code", Type: "string"},
				{Key: "notes", Type: "string"},
				{Key: "status", Type: "string", DefaultValue: "active"},
			}),
		},
		Datasets: []module.DatasetDefinition{
			{Key: "leave.balance.summary", Title: "Leave Balance Summary", TitleI18n: localize("Leave Balance Summary", "Ringkasan Saldo Cuti"), SourceKind: "model", ModelKey: "leave_balance_account", Dimensions: []module.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localize("By Status", "Berdasarkan Status"), Path: "status"}}, Measures: []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}}},
			{Key: "leave.request.summary", Title: "Leave Request Summary", TitleI18n: localize("Leave Request Summary", "Ringkasan Permintaan Cuti"), SourceKind: "model", ModelKey: "leave_request", Dimensions: []module.DatasetDimension{{Key: "by_approval", Label: "By Approval", LabelI18n: localize("By Approval", "Berdasarkan Persetujuan"), Path: "approval_status"}}, Measures: []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}}},
		},
		SearchIndexes: []search.IndexDefinition{
			commercialModelSearchIndex("leave.policies.search", "Leave Policy Search", "leave_policy", "leave.policies.list", []string{"code", "name", "status"}),
			commercialModelSearchIndex("leave.entitlement_rules.search", "Leave Entitlement Rule Search", "leave_entitlement_rule", "leave.entitlement_rules.list", []string{"leave_policy_id", "grant_mode", "status"}),
			commercialModelSearchIndex("leave.profiles.search", "Employee Leave Profile Search", "employee_leave_profile", "leave.profiles.list", []string{"employee_id", "leave_policy_id", "status"}),
			commercialModelSearchIndex("leave.balance_accounts.search", "Leave Balance Account Search", "leave_balance_account", "leave.balance_accounts.list", []string{"employee_id", "leave_policy_id", "status"}),
			commercialModelSearchIndex("leave.balance_entries.search", "Leave Balance Entry Search", "leave_balance_entry", "leave.balance_entries.list", []string{"balance_account_id", "employee_id", "entry_type", "status"}),
			commercialModelSearchIndex("leave.accrual_runs.search", "Leave Accrual Run Search", "leave_accrual_run", "leave.accrual_runs.list", []string{"code", "name", "run_mode", "run_status", "status"}),
			commercialModelSearchIndex("leave.balance_adjustments.search", "Leave Balance Adjustment Search", "leave_balance_adjustment", "leave.balance_adjustments.list", []string{"balance_account_id", "employee_id", "reason_code", "status"}),
		},
		Security: module.SecurityDefinition{
			Permissions: append(
				append(
					append(
						append(
							leavePolicyPermissions("leave_policy", "Leave Policy"),
							leavePolicyPermissions("leave_entitlement_rule", "Leave Entitlement Rule")...,
						),
						append(
							leavePolicyPermissions("employee_leave_profile", "Employee Leave Profile"),
							leavePolicyPermissions("leave_balance_account", "Leave Balance Account")...,
						)...,
					),
					append(
						leavePolicyPermissions("leave_balance_entry", "Leave Balance Entry"),
						append(leavePolicyPermissions("leave_accrual_run", "Leave Accrual Run"), leavePolicyPermissions("leave_balance_adjustment", "Leave Balance Adjustment")...)...,
					)...,
				),
				[]module.PermissionDefinition{
					{Key: "leave.self_service.read", Action: "read", Resource: "leave_self_service", DisplayName: "Read Leave Self-Service", DisplayNameI18n: localize("Read Leave Self-Service", "Lihat Layanan Mandiri Cuti")},
					{Key: "leave.self_service.write", Action: "write", Resource: "leave_self_service", DisplayName: "Write Leave Self-Service", DisplayNameI18n: localize("Write Leave Self-Service", "Ubah Layanan Mandiri Cuti")},
				}...,
			),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:           "leave_policy_manager",
					Name:          "Leave Policy Manager",
					NameI18n:      localize("Leave Policy Manager", "Manajer Kebijakan Cuti"),
					AllowedScopes: []string{"deployment", "organization", "location"},
					PermissionKeys: []string{
						"leave_policy.create", "leave_policy.list", "leave_policy.read", "leave_policy.update",
						"leave_entitlement_rule.create", "leave_entitlement_rule.list", "leave_entitlement_rule.read", "leave_entitlement_rule.update",
						"employee_leave_profile.create", "employee_leave_profile.list", "employee_leave_profile.read", "employee_leave_profile.update",
						"leave_balance_account.create", "leave_balance_account.list", "leave_balance_account.read", "leave_balance_account.update",
						"leave_balance_entry.create", "leave_balance_entry.list", "leave_balance_entry.read", "leave_balance_entry.update",
						"leave_accrual_run.create", "leave_accrual_run.list", "leave_accrual_run.read", "leave_accrual_run.update",
						"leave_balance_adjustment.create", "leave_balance_adjustment.list", "leave_balance_adjustment.read", "leave_balance_adjustment.update",
						"attendance.read", "attendance.update", "attendance.cancel",
					},
				},
				{
					Key:            "leave_self_service_employee",
					Name:           "Leave Self-Service Employee",
					NameI18n:       localize("Leave Self-Service Employee", "Karyawan Layanan Mandiri Cuti"),
					AllowedScopes:  []string{"deployment", "organization", "location"},
					PermissionKeys: []string{"leave.self_service.read", "leave.self_service.write"},
				},
			},
		},
		SelfService: module.SelfServiceDefinition{
			APIs: []module.SelfServiceAPIDefinition{
				{Key: "leave.self_service.balances.list", Title: "List Leave Balances", TitleI18n: localize("List Leave Balances", "Daftar Saldo Cuti"), Method: "GET", RoutePath: "/ui/self-service/leave/balances", HandlerKind: "custom", RequiredPermissions: []string{"leave.self_service.read"}, AudienceKinds: []string{"employee"}, ResponseContractKey: "leave.self_service.balance.list"},
				{Key: "leave.self_service.requests.list", Title: "List My Leave Requests", TitleI18n: localize("List My Leave Requests", "Daftar Permintaan Cuti Saya"), Method: "GET", RoutePath: "/ui/self-service/leave/requests", HandlerKind: "custom", ModelKey: "leave_request", RequiredPermissions: []string{"leave.self_service.read"}, AudienceKinds: []string{"employee"}, ResponseContractKey: "leave.self_service.request.list"},
				{Key: "leave.self_service.requests.get", Title: "Get My Leave Request", TitleI18n: localize("Get My Leave Request", "Lihat Permintaan Cuti Saya"), Method: "GET", RoutePath: "/ui/self-service/leave/requests/{requestID}", HandlerKind: "custom", ModelKey: "leave_request", RequiredPermissions: []string{"leave.self_service.read"}, AudienceKinds: []string{"employee"}, ResponseContractKey: "leave.self_service.request.detail"},
				{Key: "leave.self_service.requests.create", Title: "Create My Leave Request", TitleI18n: localize("Create My Leave Request", "Buat Permintaan Cuti Saya"), Method: "POST", RoutePath: "/ui/self-service/leave/requests", HandlerKind: "custom", ModelKey: "leave_request", RequiredPermissions: []string{"leave.self_service.write"}, AudienceKinds: []string{"employee"}, RequestContractKey: "leave.self_service.request.create", ResponseContractKey: "leave.self_service.request.detail", Idempotent: true},
				{Key: "leave.self_service.requests.update", Title: "Update My Leave Request", TitleI18n: localize("Update My Leave Request", "Perbarui Permintaan Cuti Saya"), Method: "PUT", RoutePath: "/ui/self-service/leave/requests/{requestID}", HandlerKind: "custom", ModelKey: "leave_request", RequiredPermissions: []string{"leave.self_service.write"}, AudienceKinds: []string{"employee"}, RequestContractKey: "leave.self_service.request.update", ResponseContractKey: "leave.self_service.request.detail", Idempotent: true},
				{Key: "leave.self_service.requests.submit", Title: "Submit My Leave Request", TitleI18n: localize("Submit My Leave Request", "Ajukan Permintaan Cuti Saya"), Method: "POST", RoutePath: "/ui/self-service/leave/requests/{requestID}/submit", HandlerKind: "custom", ModelKey: "leave_request", RequiredPermissions: []string{"leave.self_service.write"}, AudienceKinds: []string{"employee"}, RequestContractKey: "leave.self_service.request.submit", ResponseContractKey: "leave.self_service.request.detail", Idempotent: true},
				{Key: "leave.self_service.requests.cancel", Title: "Cancel My Leave Request", TitleI18n: localize("Cancel My Leave Request", "Batalkan Permintaan Cuti Saya"), Method: "POST", RoutePath: "/ui/self-service/leave/requests/{requestID}/cancel", HandlerKind: "custom", ModelKey: "leave_request", RequiredPermissions: []string{"leave.self_service.write"}, AudienceKinds: []string{"employee"}, RequestContractKey: "leave.self_service.request.cancel", ResponseContractKey: "leave.self_service.request.detail", Idempotent: true},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "leave.policies", Label: "Leave Policies", LabelI18n: localize("Leave Policies", "Kebijakan Cuti"), ActionKey: "leave.policies.list", Order: 41, RequiredPermissions: []string{"leave_policy.list"}},
				{Key: "leave.entitlement_rules", Label: "Entitlement Rules", LabelI18n: localize("Entitlement Rules", "Aturan Hak Cuti"), ActionKey: "leave.entitlement_rules.list", Order: 42, RequiredPermissions: []string{"leave_entitlement_rule.list"}},
				{Key: "leave.profiles", Label: "Leave Profiles", LabelI18n: localize("Leave Profiles", "Profil Cuti"), ActionKey: "leave.profiles.list", Order: 43, RequiredPermissions: []string{"employee_leave_profile.list"}},
				{Key: "leave.balance_accounts", Label: "Leave Balance Accounts", LabelI18n: localize("Leave Balance Accounts", "Akun Saldo Cuti"), ActionKey: "leave.balance_accounts.list", Order: 44, RequiredPermissions: []string{"leave_balance_account.list"}},
				{Key: "leave.balance_entries", Label: "Leave Balance Entries", LabelI18n: localize("Leave Balance Entries", "Entri Saldo Cuti"), ActionKey: "leave.balance_entries.list", Order: 45, RequiredPermissions: []string{"leave_balance_entry.list"}},
				{Key: "leave.accrual_runs", Label: "Leave Accrual Runs", LabelI18n: localize("Leave Accrual Runs", "Proses Akrual Cuti"), ActionKey: "leave.accrual_runs.list", Order: 46, RequiredPermissions: []string{"leave_accrual_run.list"}},
				{Key: "leave.balance_adjustments", Label: "Leave Balance Adjustments", LabelI18n: localize("Leave Balance Adjustments", "Penyesuaian Saldo Cuti"), ActionKey: "leave.balance_adjustments.list", Order: 47, RequiredPermissions: []string{"leave_balance_adjustment.list"}},
			},
			Actions: leavePolicyActions(),
			Views:   leavePolicyViews(),
		},
	}
}

func leavePolicyModelDefinition(key, singular string, fields []model.FieldDefinition) model.Definition {
	return model.Definition{
		Key:                 key,
		DisplayName:         singular,
		DisplayNameI18n:     localize(singular, singular),
		OwnerModuleKey:      "leave_policy_core",
		Version:             "v1",
		CreatePermissionKey: key + ".create",
		ListPermissionKey:   key + ".list",
		ReadPermissionKey:   key + ".read",
		UpdatePermissionKey: key + ".update",
		DefaultSort:         leavePolicySortKey(fields),
		Fields:              fields,
	}
}

func leavePolicyPermissions(key, singular string) []module.PermissionDefinition {
	return []module.PermissionDefinition{
		{Key: key + ".create", Action: "create", Resource: key, DisplayName: "Create " + singular, DisplayNameI18n: localize("Create "+singular, "Buat "+singular)},
		{Key: key + ".list", Action: "list", Resource: key, DisplayName: "List " + singular, DisplayNameI18n: localize("List "+singular, "Daftar "+singular)},
		{Key: key + ".read", Action: "read", Resource: key, DisplayName: "Read " + singular, DisplayNameI18n: localize("Read "+singular, "Lihat "+singular)},
		{Key: key + ".update", Action: "update", Resource: key, DisplayName: "Update " + singular, DisplayNameI18n: localize("Update "+singular, "Perbarui "+singular)},
	}
}

func leavePolicySortKey(fields []model.FieldDefinition) string {
	for _, candidate := range []string{"code", "name", "employee_id", "effective_date", "updated_at"} {
		for _, field := range fields {
			if field.Key == candidate {
				return candidate
			}
		}
	}
	return "updated_at"
}

func leavePolicyModelActions(prefix, title, modelKey string) []module.ActionDefinition {
	return []module.ActionDefinition{
		{Key: "leave." + prefix + ".list", Label: title, LabelI18n: localize(title, title), Kind: "navigate", RoutePath: "/leave/" + prefix, ViewKey: "leave." + prefix + ".list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".list"}},
		{Key: "leave." + prefix + ".detail", Label: title + " Detail", LabelI18n: localize(title+" Detail", title+" Detail"), Kind: "navigate", RoutePath: "/leave/" + prefix + "/detail", ViewKey: "leave." + prefix + ".detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".read"}},
		{Key: "leave." + prefix + ".form", Label: title + " Form", LabelI18n: localize(title+" Form", title+" Form"), Kind: "navigate", RoutePath: "/leave/" + prefix + "/form", ViewKey: "leave." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{modelKey + ".update"}},
	}
}

func leavePolicyViews() []module.ViewDefinition {
	return []module.ViewDefinition{
		commercialModelListView("leave.policies.list", "Leave Policies", "leave_policy", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "name", Label: "Name", Path: "values.name"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("leave.policies.detail", "Leave Policy Detail", "leave_policy", leavePolicyFields("leave_policy", false)),
		commercialModelFormView("leave.policies.form", "Leave Policy Form", "leave_policy", leavePolicyFields("leave_policy", true)),
		commercialModelListView("leave.entitlement_rules.list", "Entitlement Rules", "leave_entitlement_rule", []module.ColumnDefinition{{Key: "leave_policy_id", Label: "Policy", Path: "values.leave_policy_id"}, {Key: "grant_mode", Label: "Mode", Path: "values.grant_mode"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("leave.entitlement_rules.detail", "Entitlement Rule Detail", "leave_entitlement_rule", leavePolicyFields("leave_entitlement_rule", false)),
		commercialModelFormView("leave.entitlement_rules.form", "Entitlement Rule Form", "leave_entitlement_rule", leavePolicyFields("leave_entitlement_rule", true)),
		commercialModelListView("leave.profiles.list", "Leave Profiles", "employee_leave_profile", []module.ColumnDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id"}, {Key: "leave_policy_id", Label: "Policy", Path: "values.leave_policy_id"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("leave.profiles.detail", "Leave Profile Detail", "employee_leave_profile", leavePolicyFields("employee_leave_profile", false)),
		commercialModelFormView("leave.profiles.form", "Leave Profile Form", "employee_leave_profile", leavePolicyFields("employee_leave_profile", true)),
		commercialModelListView("leave.balance_accounts.list", "Leave Balance Accounts", "leave_balance_account", []module.ColumnDefinition{{Key: "employee_id", Label: "Employee", Path: "values.employee_id"}, {Key: "leave_policy_id", Label: "Policy", Path: "values.leave_policy_id"}, {Key: "available_days", Label: "Available", Path: "values.available_days"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("leave.balance_accounts.detail", "Leave Balance Account Detail", "leave_balance_account", leavePolicyFields("leave_balance_account", false)),
		commercialModelFormView("leave.balance_accounts.form", "Leave Balance Account Form", "leave_balance_account", leavePolicyFields("leave_balance_account", true)),
		commercialModelListView("leave.balance_entries.list", "Leave Balance Entries", "leave_balance_entry", []module.ColumnDefinition{{Key: "balance_account_id", Label: "Balance Account", Path: "values.balance_account_id"}, {Key: "entry_type", Label: "Type", Path: "values.entry_type"}, {Key: "days", Label: "Days", Path: "values.days"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("leave.balance_entries.detail", "Leave Balance Entry Detail", "leave_balance_entry", leavePolicyFields("leave_balance_entry", false)),
		commercialModelFormView("leave.balance_entries.form", "Leave Balance Entry Form", "leave_balance_entry", leavePolicyFields("leave_balance_entry", true)),
		commercialModelListView("leave.accrual_runs.list", "Leave Accrual Runs", "leave_accrual_run", []module.ColumnDefinition{{Key: "code", Label: "Code", Path: "values.code"}, {Key: "run_mode", Label: "Mode", Path: "values.run_mode"}, {Key: "run_status", Label: "Run Status", Path: "values.run_status"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"draft", "processed", "active", "inactive"}),
		commercialModelDetailView("leave.accrual_runs.detail", "Leave Accrual Run Detail", "leave_accrual_run", leavePolicyFields("leave_accrual_run", false)),
		commercialModelFormView("leave.accrual_runs.form", "Leave Accrual Run Form", "leave_accrual_run", leavePolicyFields("leave_accrual_run", true)),
		commercialModelListView("leave.balance_adjustments.list", "Leave Balance Adjustments", "leave_balance_adjustment", []module.ColumnDefinition{{Key: "balance_account_id", Label: "Balance Account", Path: "values.balance_account_id"}, {Key: "days", Label: "Days", Path: "values.days"}, {Key: "reason_code", Label: "Reason", Path: "values.reason_code"}, {Key: "status", Label: "Status", Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("leave.balance_adjustments.detail", "Leave Balance Adjustment Detail", "leave_balance_adjustment", leavePolicyFields("leave_balance_adjustment", false)),
		commercialModelFormView("leave.balance_adjustments.form", "Leave Balance Adjustment Form", "leave_balance_adjustment", leavePolicyFields("leave_balance_adjustment", true)),
	}
}

func leavePolicyActions() []module.ActionDefinition {
	actions := []module.ActionDefinition{}
	actions = append(actions, leavePolicyModelActions("policies", "Leave Policies", "leave_policy")...)
	actions = append(actions, leavePolicyModelActions("entitlement_rules", "Entitlement Rules", "leave_entitlement_rule")...)
	actions = append(actions, leavePolicyModelActions("profiles", "Leave Profiles", "employee_leave_profile")...)
	actions = append(actions, leavePolicyModelActions("balance_accounts", "Leave Balance Accounts", "leave_balance_account")...)
	actions = append(actions, leavePolicyModelActions("balance_entries", "Leave Balance Entries", "leave_balance_entry")...)
	actions = append(actions, leavePolicyModelActions("accrual_runs", "Leave Accrual Runs", "leave_accrual_run")...)
	actions = append(actions, leavePolicyModelActions("balance_adjustments", "Leave Balance Adjustments", "leave_balance_adjustment")...)
	return actions
}

func leavePolicyFields(modelKey string, form bool) []module.FieldDefinition {
	defs := map[string][]module.FieldDefinition{
		"leave_policy": {
			{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
			{Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
			{Key: "absence_code_id", Label: "Absence Code", Path: "values.absence_code_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
			{Key: "paid_leave", Label: "Paid Leave", Path: "values.paid_leave", Type: "bool", Widget: widgetForForm(form, "checkbox")},
			{Key: "requires_balance", Label: "Requires Balance", Path: "values.requires_balance", Type: "bool", Widget: widgetForForm(form, "checkbox")},
			{Key: "allows_half_day", Label: "Allows Half Day", Path: "values.allows_half_day", Type: "bool", Widget: widgetForForm(form, "checkbox")},
			{Key: "notice_days", Label: "Notice Days", Path: "values.notice_days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "approval_policy_id", Label: "Approval Policy", Path: "values.approval_policy_id", Type: "string", Widget: widgetForForm(form, "select")},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
		},
		"leave_entitlement_rule": {
			{Key: "leave_policy_id", Label: "Leave Policy", Path: "values.leave_policy_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
			{Key: "grant_mode", Label: "Grant Mode", Path: "values.grant_mode", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"annual_grant", "monthly_accrual"}},
			{Key: "annual_entitlement_days", Label: "Annual Entitlement Days", Path: "values.annual_entitlement_days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "monthly_accrual_days", Label: "Monthly Accrual Days", Path: "values.monthly_accrual_days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "carry_forward_cap_days", Label: "Carry Forward Cap Days", Path: "values.carry_forward_cap_days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "carry_forward_expiry_rule", Label: "Carry Forward Expiry Rule", Path: "values.carry_forward_expiry_rule", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"", "year_end", "q1_end", "month_end"}},
			{Key: "prorate_on_join", Label: "Prorate On Join", Path: "values.prorate_on_join", Type: "bool", Widget: widgetForForm(form, "checkbox")},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
		},
		"employee_leave_profile": {
			{Key: "employee_id", Label: "Employee", Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
			{Key: "leave_policy_id", Label: "Leave Policy", Path: "values.leave_policy_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
			{Key: "organization_id", Label: "Organization", Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text")},
			{Key: "location_id", Label: "Location", Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
			{Key: "effective_from", Label: "Effective From", Path: "values.effective_from", Type: "string", Widget: widgetForForm(form, "date")},
			{Key: "effective_to", Label: "Effective To", Path: "values.effective_to", Type: "string", Widget: widgetForForm(form, "date")},
			{Key: "opening_balance_days", Label: "Opening Balance Days", Path: "values.opening_balance_days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
		},
		"leave_balance_account": {
			{Key: "employee_id", Label: "Employee", Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select")},
			{Key: "leave_policy_id", Label: "Leave Policy", Path: "values.leave_policy_id", Type: "string", Widget: widgetForForm(form, "select")},
			{Key: "current_balance_days", Label: "Current Balance Days", Path: "values.current_balance_days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "reserved_days", Label: "Reserved Days", Path: "values.reserved_days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "available_days", Label: "Available Days", Path: "values.available_days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "carry_forward_balance_days", Label: "Carry Forward Balance Days", Path: "values.carry_forward_balance_days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "last_accrual_date", Label: "Last Accrual Date", Path: "values.last_accrual_date", Type: "string", Widget: widgetForForm(form, "date")},
			{Key: "carry_forward_expiry_date", Label: "Carry Forward Expiry Date", Path: "values.carry_forward_expiry_date", Type: "string", Widget: widgetForForm(form, "date")},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
		},
		"leave_balance_entry": {
			{Key: "balance_account_id", Label: "Balance Account", Path: "values.balance_account_id", Type: "string", Widget: widgetForForm(form, "select")},
			{Key: "employee_id", Label: "Employee", Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "text")},
			{Key: "leave_policy_id", Label: "Leave Policy", Path: "values.leave_policy_id", Type: "string", Widget: widgetForForm(form, "text")},
			{Key: "leave_request_id", Label: "Leave Request", Path: "values.leave_request_id", Type: "string", Widget: widgetForForm(form, "text")},
			{Key: "entry_type", Label: "Entry Type", Path: "values.entry_type", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"opening", "annual_grant", "monthly_accrual", "carry_forward", "expiry", "reservation", "release", "consumption", "adjustment", "reversal"}},
			{Key: "days", Label: "Days", Path: "values.days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "carry_forward_days_delta", Label: "Carry Forward Days Delta", Path: "values.carry_forward_days_delta", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "reversal_of_entry_id", Label: "Reversal Of Entry", Path: "values.reversal_of_entry_id", Type: "string", Widget: widgetForForm(form, "text")},
			{Key: "effective_date", Label: "Effective Date", Path: "values.effective_date", Type: "string", Widget: widgetForForm(form, "date")},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
		},
		"leave_accrual_run": {
			{Key: "code", Label: "Code", Path: "values.code", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
			{Key: "name", Label: "Name", Path: "values.name", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
			{Key: "leave_policy_id", Label: "Leave Policy", Path: "values.leave_policy_id", Type: "string", Widget: widgetForForm(form, "select")},
			{Key: "run_mode", Label: "Run Mode", Path: "values.run_mode", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"annual_grant", "monthly_accrual"}},
			{Key: "effective_date", Label: "Effective Date", Path: "values.effective_date", Type: "string", Widget: widgetForForm(form, "date"), Required: true},
			{Key: "run_status", Label: "Run Status", Path: "values.run_status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"draft", "processed"}},
			{Key: "processed_at", Label: "Processed At", Path: "values.processed_at", Type: "string", Widget: widgetForForm(form, "datetime")},
			{Key: "processed_by", Label: "Processed By", Path: "values.processed_by", Type: "string", Widget: widgetForForm(form, "text")},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
		},
		"leave_balance_adjustment": {
			{Key: "balance_account_id", Label: "Balance Account", Path: "values.balance_account_id", Type: "string", Widget: widgetForForm(form, "select")},
			{Key: "employee_id", Label: "Employee", Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "text")},
			{Key: "leave_policy_id", Label: "Leave Policy", Path: "values.leave_policy_id", Type: "string", Widget: widgetForForm(form, "text")},
			{Key: "days", Label: "Days", Path: "values.days", Type: "number", Widget: widgetForForm(form, "text")},
			{Key: "reason_code", Label: "Reason Code", Path: "values.reason_code", Type: "string", Widget: widgetForForm(form, "text")},
			{Key: "notes", Label: "Notes", Path: "values.notes", Type: "string", Widget: widgetForForm(form, "textarea")},
			{Key: "status", Label: "Status", Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
		},
	}
	return defs[modelKey]
}
