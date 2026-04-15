package app

import (
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

func employeeWorkforceKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                  "employee_workforce",
		Name:                 "Employee Workforce",
		NameI18n:             localize("Employee Workforce", "Tenaga Kerja Karyawan"),
		Version:              "1.0.0",
		DomainFamily:         "platform",
		Description:          "Shared employee master, assignments, role eligibility, and labor cost hooks used across business domains.",
		DescriptionI18n:      localize("Shared employee master, assignments, role eligibility, and labor cost hooks used across business domains.", "Master karyawan, penugasan, kelayakan peran, dan kait biaya tenaga kerja bersama yang digunakan lintas domain bisnis."),
		BusinessCapabilities: []string{"employee master", "workforce assignment", "operational eligibility", "labor cost hooks", "shared workforce accountability"},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "organization_structure", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Workforce Console",
			TitleI18n:       localize("Workforce Console", "Konsol Tenaga Kerja"),
			Description:     "Shared employee setup, assignments, eligibility, and compensation hooks for accountability across business operations.",
			DescriptionI18n: localize("Shared employee setup, assignments, eligibility, and compensation hooks for accountability across business operations.", "Pengaturan karyawan, penugasan, kelayakan, dan kait kompensasi bersama untuk akuntabilitas lintas operasi bisnis."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "employee_workforce_operations",
					Title:     "Workforce Operations",
					TitleI18n: localize("Workforce Operations", "Operasi Tenaga Kerja"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("employees", "Employees", "Karyawan", "/ui/workforce/employees", "Open employee master records.", "Buka data master karyawan.", "employee.list"),
						adminConsoleLink("assignments", "Employee Assignments", "Penugasan Karyawan", "/ui/workforce/assignments", "Open effective-dated workforce assignments.", "Buka penugasan tenaga kerja efektif.", "employee.read"),
						adminConsoleLink("eligibility", "Role Eligibility", "Kelayakan Peran", "/ui/workforce/eligibility", "Open operational role eligibility records.", "Buka data kelayakan peran operasi.", "employee.read"),
						adminConsoleLink("compensation", "Compensation Profiles", "Profil Kompensasi", "/ui/workforce/compensation", "Open labor cost and payroll hook profiles.", "Buka profil biaya tenaga kerja dan kait payroll.", "employee.read"),
					},
				},
			},
		},
		Models: []model.Definition{
			{
				Key:                 "employee_profile",
				DisplayName:         "Employee Profile",
				DisplayNameI18n:     localize("Employee Profile", "Profil Karyawan"),
				OwnerModuleKey:      "employee_workforce",
				Version:             "v1",
				CreatePermissionKey: "employee.create",
				ListPermissionKey:   "employee.list",
				ReadPermissionKey:   "employee.read",
				UpdatePermissionKey: "employee.update",
				DefaultSort:         "employee_code",
				Fields: []model.FieldDefinition{
					{Key: "party_id", Label: "Party", LabelI18n: localize("Party", "Pihak"), Type: "string", Required: true},
					{Key: "user_id", Label: "User", LabelI18n: localize("User", "Pengguna"), Type: "string"},
					{Key: "employee_code", Label: "Employee Code", LabelI18n: localize("Employee Code", "Kode Karyawan"), Type: "string", Required: true},
					{Key: "employment_type", Label: "Employment Type", LabelI18n: localize("Employment Type", "Tipe Karyawan"), Type: "string"},
					{Key: "employment_status", Label: "Employment Status", LabelI18n: localize("Employment Status", "Status Karyawan"), Type: "string", DefaultValue: "active"},
					{Key: "hire_date", Label: "Hire Date", LabelI18n: localize("Hire Date", "Tanggal Masuk"), Type: "string"},
					{Key: "termination_date", Label: "Termination Date", LabelI18n: localize("Termination Date", "Tanggal Berhenti"), Type: "string"},
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
					{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Type: "string"},
					{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Type: "string"},
					{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Type: "string"},
					{Key: "cost_center_id", Label: "Cost Center", LabelI18n: localize("Cost Center", "Pusat Biaya"), Type: "string"},
					{Key: "manager_user_id", Label: "Manager User", LabelI18n: localize("Manager User", "Pengguna Manajer"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
				Relations: []model.RelationDefinition{
					{Key: "assignments", Type: "has_many", TargetModelKey: "employee_assignment", ForeignKey: "employee_id"},
					{Key: "eligibility", Type: "has_many", TargetModelKey: "employee_role_eligibility", ForeignKey: "employee_id"},
					{Key: "compensation_profiles", Type: "has_many", TargetModelKey: "employee_compensation_profile", ForeignKey: "employee_id"},
				},
			},
			{
				Key:                 "employee_assignment",
				DisplayName:         "Employee Assignment",
				DisplayNameI18n:     localize("Employee Assignment", "Penugasan Karyawan"),
				OwnerModuleKey:      "employee_workforce",
				Version:             "v1",
				CreatePermissionKey: "employee.create",
				ListPermissionKey:   "employee.read",
				ReadPermissionKey:   "employee.read",
				UpdatePermissionKey: "employee.update",
				DefaultSort:         "effective_from",
				Fields: []model.FieldDefinition{
					{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Type: "string", Required: true},
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
					{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Type: "string"},
					{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Type: "string"},
					{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Type: "string"},
					{Key: "cost_center_id", Label: "Cost Center", LabelI18n: localize("Cost Center", "Pusat Biaya"), Type: "string"},
					{Key: "manager_user_id", Label: "Manager User", LabelI18n: localize("Manager User", "Pengguna Manajer"), Type: "string"},
					{Key: "manager_employee_id", Label: "Manager Employee", LabelI18n: localize("Manager Employee", "Karyawan Manajer"), Type: "string"},
					{Key: "assignment_type", Label: "Assignment Type", LabelI18n: localize("Assignment Type", "Tipe Penugasan"), Type: "string", DefaultValue: "primary"},
					{Key: "effective_from", Label: "Effective From", LabelI18n: localize("Effective From", "Berlaku Dari"), Type: "string", Required: true},
					{Key: "effective_to", Label: "Effective To", LabelI18n: localize("Effective To", "Berlaku Sampai"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			},
			{
				Key:                 "employee_role_eligibility",
				DisplayName:         "Employee Role Eligibility",
				DisplayNameI18n:     localize("Employee Role Eligibility", "Kelayakan Peran Karyawan"),
				OwnerModuleKey:      "employee_workforce",
				Version:             "v1",
				CreatePermissionKey: "employee.create",
				ListPermissionKey:   "employee.read",
				ReadPermissionKey:   "employee.read",
				UpdatePermissionKey: "employee.update",
				DefaultSort:         "eligibility_type",
				Fields: []model.FieldDefinition{
					{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Type: "string", Required: true},
					{Key: "eligibility_type", Label: "Eligibility Type", LabelI18n: localize("Eligibility Type", "Tipe Kelayakan"), Type: "string", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
					{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Type: "string"},
					{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Type: "string"},
					{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Type: "string"},
					{Key: "work_center_code", Label: "Work Center", LabelI18n: localize("Work Center", "Pusat Kerja"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			},
			{
				Key:                 "employee_compensation_profile",
				DisplayName:         "Employee Compensation Profile",
				DisplayNameI18n:     localize("Employee Compensation Profile", "Profil Kompensasi Karyawan"),
				OwnerModuleKey:      "employee_workforce",
				Version:             "v1",
				CreatePermissionKey: "employee.create",
				ListPermissionKey:   "employee.read",
				ReadPermissionKey:   "employee.read",
				UpdatePermissionKey: "employee.update",
				DefaultSort:         "employee_id",
				Fields: []model.FieldDefinition{
					{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Type: "string", Required: true},
					{Key: "pay_basis", Label: "Pay Basis", LabelI18n: localize("Pay Basis", "Basis Bayar"), Type: "string", DefaultValue: "hourly"},
					{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Type: "string", DefaultValue: "IDR"},
					{Key: "standard_hourly_rate", Label: "Standard Hourly Rate", LabelI18n: localize("Standard Hourly Rate", "Tarif Jam Standar"), Type: "number"},
					{Key: "overtime_hourly_rate", Label: "Overtime Hourly Rate", LabelI18n: localize("Overtime Hourly Rate", "Tarif Jam Lembur"), Type: "number"},
					{Key: "labor_cost_center_id", Label: "Labor Cost Center", LabelI18n: localize("Labor Cost Center", "Pusat Biaya Tenaga Kerja"), Type: "string"},
					{Key: "payable_party_id", Label: "Payable Party", LabelI18n: localize("Payable Party", "Pihak Pembayaran"), Type: "string"},
					{Key: "external_payroll_reference", Label: "Payroll Reference", LabelI18n: localize("Payroll Reference", "Referensi Payroll"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			},
		},
		Datasets: []module.DatasetDefinition{
			{
				Key:        "workforce.employee.summary",
				Title:      "Employee Summary",
				TitleI18n:  localize("Employee Summary", "Ringkasan Karyawan"),
				SourceKind: "model",
				ModelKey:   "employee_profile",
				Dimensions: []module.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localize("By Status", "Berdasarkan Status"), Path: "status"}},
				Measures:   []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}},
			},
			{
				Key:        "workforce.assignment.summary",
				Title:      "Employee Assignment Summary",
				TitleI18n:  localize("Employee Assignment Summary", "Ringkasan Penugasan Karyawan"),
				SourceKind: "model",
				ModelKey:   "employee_assignment",
				Dimensions: []module.DatasetDimension{{Key: "by_type", Label: "By Assignment Type", LabelI18n: localize("By Assignment Type", "Berdasarkan Tipe Penugasan"), Path: "assignment_type"}},
				Measures:   []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}},
			},
			{
				Key:        "workforce.eligibility.summary",
				Title:      "Eligibility Summary",
				TitleI18n:  localize("Eligibility Summary", "Ringkasan Kelayakan"),
				SourceKind: "model",
				ModelKey:   "employee_role_eligibility",
				Dimensions: []module.DatasetDimension{{Key: "by_type", Label: "By Eligibility Type", LabelI18n: localize("By Eligibility Type", "Berdasarkan Tipe Kelayakan"), Path: "eligibility_type"}},
				Measures:   []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}},
			},
		},
		SearchIndexes: []search.IndexDefinition{
			workforceSearchIndex("workforce.employees.search", "Employee Search", "employee_profile", "workforce.employees.list", []string{"employee_code", "party_id", "user_id", "employment_status", "status"}),
			workforceSearchIndex("workforce.assignments.search", "Employee Assignment Search", "employee_assignment", "workforce.assignments.list", []string{"employee_id", "assignment_type", "organization_id", "location_id", "status"}),
			workforceSearchIndex("workforce.eligibility.search", "Eligibility Search", "employee_role_eligibility", "workforce.eligibility.list", []string{"employee_id", "eligibility_type", "store_code", "work_center_code", "status"}),
			workforceSearchIndex("workforce.compensation.search", "Compensation Profile Search", "employee_compensation_profile", "workforce.compensation.list", []string{"employee_id", "pay_basis", "currency_code", "status"}),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "employee.create", Action: "create", Resource: "employee", DisplayName: "Create Employees", DisplayNameI18n: localize("Create Employees", "Buat Karyawan")},
				{Key: "employee.list", Action: "list", Resource: "employee", DisplayName: "List Employees", DisplayNameI18n: localize("List Employees", "Daftar Karyawan")},
				{Key: "employee.read", Action: "read", Resource: "employee", DisplayName: "Read Employees", DisplayNameI18n: localize("Read Employees", "Lihat Karyawan")},
				{Key: "employee.update", Action: "update", Resource: "employee", DisplayName: "Update Employees", DisplayNameI18n: localize("Update Employees", "Perbarui Karyawan")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{{
				Key: "employee_manager", Name: "Employee Manager", NameI18n: localize("Employee Manager", "Pengelola Karyawan"), AllowedScopes: []string{"deployment", "organization", "location"}, PermissionKeys: []string{"employee.create", "employee.list", "employee.read", "employee.update", "party.read", "organization_structure.read"},
			}},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "workforce.employees", Label: "Employees", LabelI18n: localize("Employees", "Karyawan"), ActionKey: "workforce.employees.list", Order: 15, RequiredPermissions: []string{"employee.list"}},
				{Key: "workforce.assignments", Label: "Employee Assignments", LabelI18n: localize("Employee Assignments", "Penugasan Karyawan"), ActionKey: "workforce.assignments.list", Order: 16, RequiredPermissions: []string{"employee.read"}},
				{Key: "workforce.eligibility", Label: "Role Eligibility", LabelI18n: localize("Role Eligibility", "Kelayakan Peran"), ActionKey: "workforce.eligibility.list", Order: 17, RequiredPermissions: []string{"employee.read"}},
				{Key: "workforce.compensation", Label: "Compensation Profiles", LabelI18n: localize("Compensation Profiles", "Profil Kompensasi"), ActionKey: "workforce.compensation.list", Order: 18, RequiredPermissions: []string{"employee.read"}},
			},
			Actions: append(workforceActions("employees", "Employees", "Employee Detail", "Employee Form"),
				append(workforceActions("assignments", "Employee Assignments", "Employee Assignment Detail", "Employee Assignment Form"),
					append(workforceActions("eligibility", "Role Eligibility", "Eligibility Detail", "Eligibility Form"),
						workforceActions("compensation", "Compensation Profiles", "Compensation Profile Detail", "Compensation Profile Form")...)...)...),
			Views: []module.ViewDefinition{
				commercialModelListView("workforce.employees.list", "Employees", "employee_profile", []module.ColumnDefinition{
					{Key: "employee_code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.employee_code"},
					{Key: "party_id", Label: "Party", LabelI18n: localize("Party", "Pihak"), Path: "values.party_id"},
					{Key: "user_id", Label: "User", LabelI18n: localize("User", "Pengguna"), Path: "values.user_id"},
					{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Path: "values.department_id"},
					{Key: "employment_status", Label: "Employment", LabelI18n: localize("Employment", "Ketenagakerjaan"), Path: "values.employment_status"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("workforce.employees.detail", "Employee Detail", "employee_profile", workforceEmployeeFields(false)),
				commercialModelFormView("workforce.employees.form", "Employee Form", "employee_profile", workforceEmployeeFields(true)),
				commercialModelListView("workforce.assignments.list", "Employee Assignments", "employee_assignment", []module.ColumnDefinition{
					{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id"},
					{Key: "assignment_type", Label: "Type", LabelI18n: localize("Type", "Tipe"), Path: "values.assignment_type"},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id"},
					{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Path: "values.department_id"},
					{Key: "effective_from", Label: "Effective From", LabelI18n: localize("Effective From", "Berlaku Dari"), Path: "values.effective_from"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("workforce.assignments.detail", "Employee Assignment Detail", "employee_assignment", workforceAssignmentFields(false)),
				commercialModelFormView("workforce.assignments.form", "Employee Assignment Form", "employee_assignment", workforceAssignmentFields(true)),
				commercialModelListView("workforce.eligibility.list", "Role Eligibility", "employee_role_eligibility", []module.ColumnDefinition{
					{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id"},
					{Key: "eligibility_type", Label: "Eligibility", LabelI18n: localize("Eligibility", "Kelayakan"), Path: "values.eligibility_type"},
					{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code"},
					{Key: "work_center_code", Label: "Work Center", LabelI18n: localize("Work Center", "Pusat Kerja"), Path: "values.work_center_code"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("workforce.eligibility.detail", "Eligibility Detail", "employee_role_eligibility", workforceEligibilityFields(false)),
				commercialModelFormView("workforce.eligibility.form", "Eligibility Form", "employee_role_eligibility", workforceEligibilityFields(true)),
				commercialModelListView("workforce.compensation.list", "Compensation Profiles", "employee_compensation_profile", []module.ColumnDefinition{
					{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id"},
					{Key: "pay_basis", Label: "Pay Basis", LabelI18n: localize("Pay Basis", "Basis Bayar"), Path: "values.pay_basis"},
					{Key: "standard_hourly_rate", Label: "Standard Rate", LabelI18n: localize("Standard Rate", "Tarif Standar"), Path: "values.standard_hourly_rate"},
					{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("workforce.compensation.detail", "Compensation Profile Detail", "employee_compensation_profile", workforceCompensationFields(false)),
				commercialModelFormView("workforce.compensation.form", "Compensation Profile Form", "employee_compensation_profile", workforceCompensationFields(true)),
			},
		},
		Offline: module.OfflineDefinition{
			Models: []module.OfflineModelDefinition{
				{ModelKey: "employee_profile", Title: "Employee Profile", TitleI18n: localize("Employee Profile", "Profil Karyawan"), CreatePermissionKey: "employee.create", UpdatePermissionKey: "employee.update", RequiredPermissions: []string{"employee.read"}},
				{ModelKey: "employee_assignment", Title: "Employee Assignment", TitleI18n: localize("Employee Assignment", "Penugasan Karyawan"), CreatePermissionKey: "employee.create", UpdatePermissionKey: "employee.update", RequiredPermissions: []string{"employee.read"}},
				{ModelKey: "employee_role_eligibility", Title: "Employee Role Eligibility", TitleI18n: localize("Employee Role Eligibility", "Kelayakan Peran Karyawan"), CreatePermissionKey: "employee.create", UpdatePermissionKey: "employee.update", RequiredPermissions: []string{"employee.read"}},
				{ModelKey: "employee_compensation_profile", Title: "Employee Compensation Profile", TitleI18n: localize("Employee Compensation Profile", "Profil Kompensasi Karyawan"), CreatePermissionKey: "employee.create", UpdatePermissionKey: "employee.update", RequiredPermissions: []string{"employee.read"}},
			},
		},
	}
}

func workforceActions(prefix, listLabel, detailLabel, formLabel string) []module.ActionDefinition {
	base := "/workforce/" + prefix
	listPermission := "employee.read"
	if prefix == "employees" {
		listPermission = "employee.list"
	}
	return []module.ActionDefinition{
		{Key: "workforce." + prefix + ".list", Label: listLabel, LabelI18n: localize(listLabel, listLabel), Kind: "navigate", RoutePath: base, ViewKey: "workforce." + prefix + ".list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{listPermission}},
		{Key: "workforce." + prefix + ".detail", Label: detailLabel, LabelI18n: localize(detailLabel, detailLabel), Kind: "navigate", RoutePath: base + "/detail", ViewKey: "workforce." + prefix + ".detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"employee.read"}},
		{Key: "workforce." + prefix + ".form", Label: formLabel, LabelI18n: localize(formLabel, formLabel), Kind: "navigate", RoutePath: base + "/form", ViewKey: "workforce." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"employee.update"}},
		{Key: "workforce." + prefix + ".new", Label: "New " + listLabel, LabelI18n: localize("New "+listLabel, "Baru"), Kind: "navigate", RoutePath: base + "/new", ViewKey: "workforce." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"employee.update"}},
	}
}

func workforceSearchIndex(key, title, modelKey, viewKey string, fieldKeys []string) search.IndexDefinition {
	fields := make([]search.IndexFieldDefinition, 0, len(fieldKeys))
	for _, fieldKey := range fieldKeys {
		fields = append(fields, search.IndexFieldDefinition{Key: fieldKey, Path: fieldKey, Type: "string", Searchable: true, Facet: fieldKey == "status"})
	}
	requiredPermissions := []string{"employee.read"}
	if modelKey == "employee_profile" {
		requiredPermissions = []string{"employee.list"}
	}
	return search.IndexDefinition{
		Key:                 key,
		Title:               title,
		SourceKind:          "model",
		ModelKey:            modelKey,
		ViewKey:             viewKey,
		Modes:               []string{"keyword", "hybrid"},
		OrganizationSplit:   true,
		RequiredPermissions: requiredPermissions,
		QueryFilterFields:   []string{"status", "location_id"},
		QuerySortFields:     fieldKeys,
		Fields:              fields,
	}
}

func workforceEmployeeFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "party_id", Label: "Party", LabelI18n: localize("Party", "Pihak"), Path: "values.party_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "user_id", Label: "User", LabelI18n: localize("User", "Pengguna"), Path: "values.user_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "employee_code", Label: "Employee Code", LabelI18n: localize("Employee Code", "Kode Karyawan"), Path: "values.employee_code", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "employment_type", Label: "Employment Type", LabelI18n: localize("Employment Type", "Tipe Karyawan"), Path: "values.employment_type", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"permanent", "contract", "part_time", "temporary"}},
		{Key: "employment_status", Label: "Employment Status", LabelI18n: localize("Employment Status", "Status Karyawan"), Path: "values.employment_status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive", "leave", "terminated"}},
		{Key: "hire_date", Label: "Hire Date", LabelI18n: localize("Hire Date", "Tanggal Masuk"), Path: "values.hire_date", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "termination_date", Label: "Termination Date", LabelI18n: localize("Termination Date", "Tanggal Berhenti"), Path: "values.termination_date", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Path: "values.operating_unit_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Path: "values.organization_unit_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Path: "values.department_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "cost_center_id", Label: "Cost Center", LabelI18n: localize("Cost Center", "Pusat Biaya"), Path: "values.cost_center_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "manager_user_id", Label: "Manager User", LabelI18n: localize("Manager User", "Pengguna Manajer"), Path: "values.manager_user_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func workforceAssignmentFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Path: "values.operating_unit_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Path: "values.organization_unit_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Path: "values.department_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "cost_center_id", Label: "Cost Center", LabelI18n: localize("Cost Center", "Pusat Biaya"), Path: "values.cost_center_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "manager_user_id", Label: "Manager User", LabelI18n: localize("Manager User", "Pengguna Manajer"), Path: "values.manager_user_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "manager_employee_id", Label: "Manager Employee", LabelI18n: localize("Manager Employee", "Karyawan Manajer"), Path: "values.manager_employee_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "assignment_type", Label: "Assignment Type", LabelI18n: localize("Assignment Type", "Tipe Penugasan"), Path: "values.assignment_type", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"primary", "secondary", "temporary"}},
		{Key: "effective_from", Label: "Effective From", LabelI18n: localize("Effective From", "Berlaku Dari"), Path: "values.effective_from", Type: "string", Widget: widgetForForm(form, "text"), Required: true},
		{Key: "effective_to", Label: "Effective To", LabelI18n: localize("Effective To", "Berlaku Sampai"), Path: "values.effective_to", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func workforceEligibilityFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "eligibility_type", Label: "Eligibility Type", LabelI18n: localize("Eligibility Type", "Tipe Kelayakan"), Path: "values.eligibility_type", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"cashier", "store_supervisor", "warehouse_operator", "production_operator", "approver_eligible", "asset_custodian"}, Required: true},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Path: "values.register_code", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "work_center_code", Label: "Work Center", LabelI18n: localize("Work Center", "Pusat Kerja"), Path: "values.work_center_code", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func workforceCompensationFields(form bool) []module.FieldDefinition {
	return []module.FieldDefinition{
		{Key: "employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.employee_id", Type: "string", Widget: widgetForForm(form, "select"), Required: true},
		{Key: "pay_basis", Label: "Pay Basis", LabelI18n: localize("Pay Basis", "Basis Bayar"), Path: "values.pay_basis", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"hourly", "monthly", "daily"}},
		{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "standard_hourly_rate", Label: "Standard Hourly Rate", LabelI18n: localize("Standard Hourly Rate", "Tarif Jam Standar"), Path: "values.standard_hourly_rate", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "overtime_hourly_rate", Label: "Overtime Hourly Rate", LabelI18n: localize("Overtime Hourly Rate", "Tarif Jam Lembur"), Path: "values.overtime_hourly_rate", Type: "number", Widget: widgetForForm(form, "text")},
		{Key: "labor_cost_center_id", Label: "Labor Cost Center", LabelI18n: localize("Labor Cost Center", "Pusat Biaya Tenaga Kerja"), Path: "values.labor_cost_center_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "payable_party_id", Label: "Payable Party", LabelI18n: localize("Payable Party", "Pihak Pembayaran"), Path: "values.payable_party_id", Type: "string", Widget: widgetForForm(form, "select")},
		{Key: "external_payroll_reference", Label: "Payroll Reference", LabelI18n: localize("Payroll Reference", "Referensi Payroll"), Path: "values.external_payroll_reference", Type: "string", Widget: widgetForForm(form, "text")},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: widgetForForm(form, "select"), Options: []string{"active", "inactive"}},
	}
}

func widgetForForm(form bool, widget string) string {
	if form {
		return widget
	}
	return ""
}
