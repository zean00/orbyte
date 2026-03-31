package app

import (
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

func organizationStructureKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                  "organization_structure",
		Name:                 "Organization Structure",
		NameI18n:             localize("Organization Structure", "Struktur Organisasi"),
		Version:              "1.0.0",
		DomainFamily:         "platform",
		Description:          "Shared organization-unit, department, and cost-center masters for cross-domain ownership and reporting.",
		DescriptionI18n:      localize("Shared organization-unit, department, and cost-center masters for cross-domain ownership and reporting.", "Master unit organisasi, departemen, dan pusat biaya bersama untuk kepemilikan dan pelaporan lintas domain."),
		BusinessCapabilities: []string{"organization hierarchy", "department master", "cost center master", "shared reporting structure"},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Organization Structure Console",
			TitleI18n:       localize("Organization Structure Console", "Konsol Struktur Organisasi"),
			Description:     "Shared hierarchy setup for organization units, departments, and cost centers.",
			DescriptionI18n: localize("Shared hierarchy setup for organization units, departments, and cost centers.", "Pengaturan hierarki bersama untuk unit organisasi, departemen, dan pusat biaya."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "organization_structure_operations",
					Title:     "Organization Structure Operations",
					TitleI18n: localize("Organization Structure Operations", "Operasi Struktur Organisasi"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("organization_units", "Organization Units", "Unit Organisasi", "/ui/organization/units", "Open organization-unit hierarchy records.", "Buka data hierarki unit organisasi.", "organization_structure.list"),
						adminConsoleLink("departments", "Departments", "Departemen", "/ui/organization/departments", "Open department masters.", "Buka data master departemen.", "organization_structure.list"),
						adminConsoleLink("cost_centers", "Cost Centers", "Pusat Biaya", "/ui/organization/cost_centers", "Open cost-center masters.", "Buka data master pusat biaya.", "organization_structure.list"),
					},
				},
			},
		},
		Models: []model.Definition{
			{
				Key:                 "organization_unit",
				DisplayName:         "Organization Unit",
				DisplayNameI18n:     localize("Organization Unit", "Unit Organisasi"),
				OwnerModuleKey:      "organization_structure",
				Version:             "v1",
				CreatePermissionKey: "organization_structure.create",
				ListPermissionKey:   "organization_structure.list",
				ReadPermissionKey:   "organization_structure.read",
				UpdatePermissionKey: "organization_structure.update",
				DefaultSort:         "code",
				Fields: []model.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
					{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Type: "string"},
					{Key: "parent_unit_id", Label: "Parent Unit", LabelI18n: localize("Parent Unit", "Unit Induk"), Type: "string"},
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "unit_type", Label: "Unit Type", LabelI18n: localize("Unit Type", "Tipe Unit"), Type: "string"},
					{Key: "manager_party_id", Label: "Manager Party", LabelI18n: localize("Manager Party", "Pihak Manajer"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			},
			{
				Key:                 "department",
				DisplayName:         "Department",
				DisplayNameI18n:     localize("Department", "Departemen"),
				OwnerModuleKey:      "organization_structure",
				Version:             "v1",
				CreatePermissionKey: "organization_structure.create",
				ListPermissionKey:   "organization_structure.list",
				ReadPermissionKey:   "organization_structure.read",
				UpdatePermissionKey: "organization_structure.update",
				DefaultSort:         "code",
				Fields: []model.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
					{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Type: "string"},
					{Key: "parent_department_id", Label: "Parent Department", LabelI18n: localize("Parent Department", "Departemen Induk"), Type: "string"},
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "manager_party_id", Label: "Manager Party", LabelI18n: localize("Manager Party", "Pihak Manajer"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			},
			{
				Key:                 "cost_center",
				DisplayName:         "Cost Center",
				DisplayNameI18n:     localize("Cost Center", "Pusat Biaya"),
				OwnerModuleKey:      "organization_structure",
				Version:             "v1",
				CreatePermissionKey: "organization_structure.create",
				ListPermissionKey:   "organization_structure.list",
				ReadPermissionKey:   "organization_structure.read",
				UpdatePermissionKey: "organization_structure.update",
				DefaultSort:         "code",
				Fields: []model.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
					{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Type: "string"},
					{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Type: "string"},
					{Key: "parent_cost_center_id", Label: "Parent Cost Center", LabelI18n: localize("Parent Cost Center", "Pusat Biaya Induk"), Type: "string"},
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			},
		},
		Datasets: []module.DatasetDefinition{{
			Key:        "organization.structure.summary",
			Title:      "Organization Structure Summary",
			TitleI18n:  localize("Organization Structure Summary", "Ringkasan Struktur Organisasi"),
			SourceKind: "model",
			ModelKey:   "organization_unit",
			Dimensions: []module.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localize("By Status", "Berdasarkan Status"), Path: "status"}},
			Measures:   []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}},
		}},
		SearchIndexes: []search.IndexDefinition{
			organizationStructureSearchIndex("organization.units.search", "Organization Unit Search", "organization_unit", "organization.units.list"),
			organizationStructureSearchIndex("organization.departments.search", "Department Search", "department", "organization.departments.list"),
			organizationStructureSearchIndex("organization.cost_centers.search", "Cost Center Search", "cost_center", "organization.cost_centers.list"),
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "organization_structure.create", Action: "create", Resource: "organization_structure", DisplayName: "Create Organization Structure", DisplayNameI18n: localize("Create Organization Structure", "Buat Struktur Organisasi")},
				{Key: "organization_structure.list", Action: "list", Resource: "organization_structure", DisplayName: "List Organization Structure", DisplayNameI18n: localize("List Organization Structure", "Daftar Struktur Organisasi")},
				{Key: "organization_structure.read", Action: "read", Resource: "organization_structure", DisplayName: "Read Organization Structure", DisplayNameI18n: localize("Read Organization Structure", "Lihat Struktur Organisasi")},
				{Key: "organization_structure.update", Action: "update", Resource: "organization_structure", DisplayName: "Update Organization Structure", DisplayNameI18n: localize("Update Organization Structure", "Perbarui Struktur Organisasi")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{{
				Key: "organization_structure_manager", Name: "Organization Structure Manager", NameI18n: localize("Organization Structure Manager", "Pengelola Struktur Organisasi"), AllowedScopes: []string{"deployment", "organization", "location"}, PermissionKeys: []string{"organization_structure.create", "organization_structure.list", "organization_structure.read", "organization_structure.update"},
			}},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "organization.units", Label: "Organization Units", LabelI18n: localize("Organization Units", "Unit Organisasi"), ActionKey: "organization.units.list", Order: 12, RequiredPermissions: []string{"organization_structure.list"}},
				{Key: "organization.departments", Label: "Departments", LabelI18n: localize("Departments", "Departemen"), ActionKey: "organization.departments.list", Order: 13, RequiredPermissions: []string{"organization_structure.list"}},
				{Key: "organization.cost_centers", Label: "Cost Centers", LabelI18n: localize("Cost Centers", "Pusat Biaya"), ActionKey: "organization.cost_centers.list", Order: 14, RequiredPermissions: []string{"organization_structure.list"}},
			},
			Actions: append(organizationStructureActions("units", "Organization Units", "Organization Unit Detail", "Organization Unit Form"),
				append(organizationStructureActions("departments", "Departments", "Department Detail", "Department Form"),
					organizationStructureActions("cost_centers", "Cost Centers", "Cost Center Detail", "Cost Center Form")...)...),
			Views: []module.ViewDefinition{
				commercialModelListView("organization.units.list", "Organization Units", "organization_unit", []module.ColumnDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id"},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id"},
					{Key: "unit_type", Label: "Type", LabelI18n: localize("Type", "Tipe"), Path: "values.unit_type"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("organization.units.detail", "Organization Unit Detail", "organization_unit", []module.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string"},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string"},
					{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Path: "values.operating_unit_id", Type: "string"},
					{Key: "parent_unit_id", Label: "Parent Unit", LabelI18n: localize("Parent Unit", "Unit Induk"), Path: "values.parent_unit_id", Type: "string"},
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
					{Key: "unit_type", Label: "Unit Type", LabelI18n: localize("Unit Type", "Tipe Unit"), Path: "values.unit_type", Type: "string"},
					{Key: "manager_party_id", Label: "Manager Party", LabelI18n: localize("Manager Party", "Pihak Manajer"), Path: "values.manager_party_id", Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
				}),
				commercialModelFormView("organization.units.form", "Organization Unit Form", "organization_unit", []module.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: "text", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: "text"},
					{Key: "operating_unit_id", Label: "Operating Unit", LabelI18n: localize("Operating Unit", "Unit Operasi"), Path: "values.operating_unit_id", Type: "string", Widget: "text"},
					{Key: "parent_unit_id", Label: "Parent Unit", LabelI18n: localize("Parent Unit", "Unit Induk"), Path: "values.parent_unit_id", Type: "string", Widget: "select"},
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
					{Key: "unit_type", Label: "Unit Type", LabelI18n: localize("Unit Type", "Tipe Unit"), Path: "values.unit_type", Type: "string", Widget: "text"},
					{Key: "manager_party_id", Label: "Manager Party", LabelI18n: localize("Manager Party", "Pihak Manajer"), Path: "values.manager_party_id", Type: "string", Widget: "select"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
				}),
				commercialModelListView("organization.departments.list", "Departments", "department", []module.ColumnDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
					{Key: "organization_unit_id", Label: "Unit", LabelI18n: localize("Unit", "Unit"), Path: "values.organization_unit_id"},
					{Key: "manager_party_id", Label: "Manager", LabelI18n: localize("Manager", "Manajer"), Path: "values.manager_party_id"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("organization.departments.detail", "Department Detail", "department", []module.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string"},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string"},
					{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Path: "values.organization_unit_id", Type: "string"},
					{Key: "parent_department_id", Label: "Parent Department", LabelI18n: localize("Parent Department", "Departemen Induk"), Path: "values.parent_department_id", Type: "string"},
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
					{Key: "manager_party_id", Label: "Manager Party", LabelI18n: localize("Manager Party", "Pihak Manajer"), Path: "values.manager_party_id", Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
				}),
				commercialModelFormView("organization.departments.form", "Department Form", "department", []module.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: "text", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: "text"},
					{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Path: "values.organization_unit_id", Type: "string", Widget: "select"},
					{Key: "parent_department_id", Label: "Parent Department", LabelI18n: localize("Parent Department", "Departemen Induk"), Path: "values.parent_department_id", Type: "string", Widget: "select"},
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
					{Key: "manager_party_id", Label: "Manager Party", LabelI18n: localize("Manager Party", "Pihak Manajer"), Path: "values.manager_party_id", Type: "string", Widget: "select"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
				}),
				commercialModelListView("organization.cost_centers.list", "Cost Centers", "cost_center", []module.ColumnDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
					{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Path: "values.department_id"},
					{Key: "organization_unit_id", Label: "Unit", LabelI18n: localize("Unit", "Unit"), Path: "values.organization_unit_id"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("organization.cost_centers.detail", "Cost Center Detail", "cost_center", []module.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string"},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string"},
					{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Path: "values.organization_unit_id", Type: "string"},
					{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Path: "values.department_id", Type: "string"},
					{Key: "parent_cost_center_id", Label: "Parent Cost Center", LabelI18n: localize("Parent Cost Center", "Pusat Biaya Induk"), Path: "values.parent_cost_center_id", Type: "string"},
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
				}),
				commercialModelFormView("organization.cost_centers.form", "Cost Center Form", "cost_center", []module.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: "text", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: "text"},
					{Key: "organization_unit_id", Label: "Organization Unit", LabelI18n: localize("Organization Unit", "Unit Organisasi"), Path: "values.organization_unit_id", Type: "string", Widget: "select"},
					{Key: "department_id", Label: "Department", LabelI18n: localize("Department", "Departemen"), Path: "values.department_id", Type: "string", Widget: "select"},
					{Key: "parent_cost_center_id", Label: "Parent Cost Center", LabelI18n: localize("Parent Cost Center", "Pusat Biaya Induk"), Path: "values.parent_cost_center_id", Type: "string", Widget: "select"},
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
				}),
			},
		},
		Offline: module.OfflineDefinition{
			Models: []module.OfflineModelDefinition{
				{ModelKey: "organization_unit", Title: "Organization Unit", TitleI18n: localize("Organization Unit", "Unit Organisasi"), CreatePermissionKey: "organization_structure.create", UpdatePermissionKey: "organization_structure.update", RequiredPermissions: []string{"organization_structure.read"}},
				{ModelKey: "department", Title: "Department", TitleI18n: localize("Department", "Departemen"), CreatePermissionKey: "organization_structure.create", UpdatePermissionKey: "organization_structure.update", RequiredPermissions: []string{"organization_structure.read"}},
				{ModelKey: "cost_center", Title: "Cost Center", TitleI18n: localize("Cost Center", "Pusat Biaya"), CreatePermissionKey: "organization_structure.create", UpdatePermissionKey: "organization_structure.update", RequiredPermissions: []string{"organization_structure.read"}},
			},
		},
	}
}

func organizationStructureActions(prefix, listLabel, detailLabel, formLabel string) []module.ActionDefinition {
	base := "/organization/" + prefix
	return []module.ActionDefinition{
		{Key: "organization." + prefix + ".list", Label: listLabel, LabelI18n: localize(listLabel, listLabel), Kind: "navigate", RoutePath: base, ViewKey: "organization." + prefix + ".list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"organization_structure.list"}},
		{Key: "organization." + prefix + ".detail", Label: detailLabel, LabelI18n: localize(detailLabel, detailLabel), Kind: "navigate", RoutePath: base + "/detail", ViewKey: "organization." + prefix + ".detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"organization_structure.read"}},
		{Key: "organization." + prefix + ".form", Label: formLabel, LabelI18n: localize(formLabel, formLabel), Kind: "navigate", RoutePath: base + "/form", ViewKey: "organization." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"organization_structure.update"}},
		{Key: "organization." + prefix + ".new", Label: "New " + listLabel, LabelI18n: localize("New "+listLabel, "Baru"), Kind: "navigate", RoutePath: base + "/new", ViewKey: "organization." + prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"organization_structure.update"}},
	}
}

func organizationStructureSearchIndex(key, title, modelKey, viewKey string) search.IndexDefinition {
	return search.IndexDefinition{
		Key:                 key,
		Title:               title,
		SourceKind:          "model",
		ModelKey:            modelKey,
		ViewKey:             viewKey,
		Modes:               []string{"keyword", "hybrid"},
		OrganizationSplit:   true,
		RequiredPermissions: []string{"organization_structure.list"},
		QueryFilterFields:   []string{"organization_id", "location_id", "status"},
		QuerySortFields:     []string{"code", "name", "updated_at"},
		Fields: []search.IndexFieldDefinition{
			{Key: "organization_id", Path: "organization_id", Type: "string", Facet: true},
			{Key: "location_id", Path: "location_id", Type: "string", Facet: true},
			{Key: "code", Path: "code", Type: "string", Searchable: true, Sort: true},
			{Key: "name", Path: "name", Type: "string", Searchable: true, Sort: true},
			{Key: "status", Path: "status", Type: "string", Facet: true, Sort: true},
		},
	}
}
