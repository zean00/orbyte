package app

import (
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
)

func inventoryFinanceCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{inventoryFinanceCoreKernelPackManifest()}
}

func inventoryFinanceCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "inventory_finance_core",
		Name:         "Inventory Finance Core",
		NameI18n:     localize("Inventory Finance Core", "Inti Keuangan Inventori"),
		Version:      "1.0.0",
		DomainFamily: "business",
		Dependencies: []string{"inventory_core", "finance_reporting_core"},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Inventory Finance Console",
			TitleI18n:       localize("Inventory Finance Console", "Konsol Keuangan Inventori"),
			Description:     "Inventory valuation, stock-to-GL reconciliation, count sessions, and adjustment review.",
			DescriptionI18n: localize("Inventory valuation, stock-to-GL reconciliation, count sessions, and adjustment review.", "Penilaian inventori, rekonsiliasi stok ke GL, sesi hitung, dan tinjauan penyesuaian."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "finance_controls",
					Title:     "Finance Controls",
					TitleI18n: localize("Finance Controls", "Kontrol Keuangan"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("inventory_valuation", "Inventory Valuation", "Penilaian Inventori", "/ui/finance/inventory-valuation", "Open current inventory valuation.", "Buka penilaian inventori saat ini.", "finance.read"),
						adminConsoleLink("inventory_valuation_as_of", "Inventory Valuation As Of", "Penilaian Inventori Per Tanggal", "/ui/finance/inventory-valuation-as-of", "Open valuation as of a selected date.", "Buka penilaian inventori per tanggal tertentu.", "finance.read"),
						adminConsoleLink("inventory_gl_reconciliation", "Inventory GL Reconciliation", "Rekonsiliasi GL Inventori", "/ui/finance/inventory-gl-reconciliation", "Open stock-to-ledger reconciliation.", "Buka rekonsiliasi stok ke ledger.", "finance.read"),
						adminConsoleLink("inventory_adjustment_review", "Adjustment Review", "Tinjauan Penyesuaian", "/ui/finance/inventory-adjustment-review", "Review pending stock adjustments with financial impact.", "Tinjau penyesuaian stok dengan dampak keuangan.", "inventory.finance.review"),
						adminConsoleLink("count_sessions", "Count Sessions", "Sesi Hitung", "/ui/inventory/count-sessions", "Manage cycle count sessions.", "Kelola sesi hitung stok.", "inventory_count_session.list"),
						adminConsoleLink("reconciliation_cases", "Reconciliation Cases", "Kasus Rekonsiliasi", "/ui/finance/inventory-reconciliation-cases", "Review inventory reconciliation cases.", "Tinjau kasus rekonsiliasi inventori.", "inventory_reconciliation_case.list"),
					},
				},
			},
		},
		Models: []model.Definition{
			{
				Key:                 "inventory_count_session",
				DisplayName:         "Inventory Count Session",
				DisplayNameI18n:     localize("Inventory Count Session", "Sesi Hitung Inventori"),
				OwnerModuleKey:      "inventory_finance_core",
				Version:             "v1",
				CreatePermissionKey: "inventory_count_session.create",
				ListPermissionKey:   "inventory_count_session.list",
				ReadPermissionKey:   "inventory_count_session.read",
				UpdatePermissionKey: "inventory_count_session.update",
				DefaultSort:         "count_date",
				Fields: []model.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
					{Key: "session_code", Label: "Session Code", LabelI18n: localize("Session Code", "Kode Sesi"), Type: "string"},
					{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Type: "string", Required: true},
					{Key: "count_date", Label: "Count Date", LabelI18n: localize("Count Date", "Tanggal Hitung"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "open"},
					{Key: "adjustment_reason", Label: "Adjustment Reason", LabelI18n: localize("Adjustment Reason", "Alasan Penyesuaian"), Type: "string", DefaultValue: "cycle_count"},
					{Key: "adjustment_account_code", Label: "Adjustment Account", LabelI18n: localize("Adjustment Account", "Akun Penyesuaian"), Type: "string", DefaultValue: "5800-INV-ADJ"},
					{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Type: "object"},
					{Key: "generated_adjustment_id", Label: "Generated Adjustment ID", LabelI18n: localize("Generated Adjustment ID", "ID Penyesuaian Hasil Generate"), Type: "string"},
					{Key: "generated_adjustment_number", Label: "Generated Adjustment", LabelI18n: localize("Generated Adjustment", "Penyesuaian Hasil Generate"), Type: "string"},
					{Key: "note", Label: "Note", LabelI18n: localize("Note", "Catatan"), Type: "string"},
				},
			},
			{
				Key:                 "inventory_reconciliation_case",
				DisplayName:         "Inventory Reconciliation Case",
				DisplayNameI18n:     localize("Inventory Reconciliation Case", "Kasus Rekonsiliasi Inventori"),
				OwnerModuleKey:      "inventory_finance_core",
				Version:             "v1",
				CreatePermissionKey: "inventory_reconciliation_case.create",
				ListPermissionKey:   "inventory_reconciliation_case.list",
				ReadPermissionKey:   "inventory_reconciliation_case.read",
				UpdatePermissionKey: "inventory_reconciliation_case.update",
				DefaultSort:         "as_of_date",
				Fields: []model.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
					{Key: "as_of_date", Label: "As Of", LabelI18n: localize("As Of", "Per Tanggal"), Type: "string"},
					{Key: "account_code", Label: "Account", LabelI18n: localize("Account", "Akun"), Type: "string"},
					{Key: "account_name", Label: "Account Name", LabelI18n: localize("Account Name", "Nama Akun"), Type: "string"},
					{Key: "mismatch_type", Label: "Mismatch Type", LabelI18n: localize("Mismatch Type", "Tipe Selisih"), Type: "string", DefaultValue: "inventory_gl"},
					{Key: "inventory_value", Label: "Inventory Value", LabelI18n: localize("Inventory Value", "Nilai Inventori"), Type: "number"},
					{Key: "gl_value", Label: "GL Value", LabelI18n: localize("GL Value", "Nilai GL"), Type: "number"},
					{Key: "difference", Label: "Difference", LabelI18n: localize("Difference", "Selisih"), Type: "number"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "open"},
					{Key: "assignee_user_id", Label: "Assignee", LabelI18n: localize("Assignee", "Penanggung Jawab"), Type: "string"},
					{Key: "linked_document_id", Label: "Linked Document", LabelI18n: localize("Linked Document", "Dokumen Terkait"), Type: "string"},
					{Key: "linked_posting_id", Label: "Linked Posting", LabelI18n: localize("Linked Posting", "Posting Terkait"), Type: "string"},
					{Key: "note", Label: "Note", LabelI18n: localize("Note", "Catatan"), Type: "string"},
				},
			},
		},
		Security: module.SecurityDefinition{
			Permissions: append(
				append(
					commercialModelPermissions("inventory_count_session", "Inventory Count Session"),
					commercialModelPermissions("inventory_reconciliation_case", "Inventory Reconciliation Case")...,
				),
				module.PermissionDefinition{
					Key:             "inventory.finance.review",
					Action:          "review",
					Resource:        "inventory_finance",
					DisplayName:     "Review Inventory Finance Controls",
					DisplayNameI18n: localize("Review Inventory Finance Controls", "Tinjau Kontrol Keuangan Inventori"),
				},
			),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:           "inventory_finance_manager",
					Name:          "Inventory Finance Manager",
					NameI18n:      localize("Inventory Finance Manager", "Manajer Keuangan Inventori"),
					AllowedScopes: []string{"deployment", "organization", "location"},
					PermissionKeys: []string{
						"inventory_count_session.create", "inventory_count_session.list", "inventory_count_session.read", "inventory_count_session.update",
						"inventory_reconciliation_case.create", "inventory_reconciliation_case.list", "inventory_reconciliation_case.read", "inventory_reconciliation_case.update",
						"inventory.finance.review", "finance.read", "document.read", "document.create", "document.update_draft",
					},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "inventory.count_sessions", Label: "Count Sessions", LabelI18n: localize("Count Sessions", "Sesi Hitung"), ActionKey: "inventory.count_sessions.list", Order: 58, RequiredPermissions: []string{"inventory_count_session.list"}},
				{Key: "finance.inventory_valuation", Label: "Inventory Valuation", LabelI18n: localize("Inventory Valuation", "Penilaian Inventori"), ActionKey: "finance.inventory_valuation", Order: 107, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.inventory_valuation_as_of", Label: "Inventory Valuation As Of", LabelI18n: localize("Inventory Valuation As Of", "Penilaian Inventori Per Tanggal"), ActionKey: "finance.inventory_valuation_as_of", Order: 108, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.inventory_gl_reconciliation", Label: "Inventory GL Reconciliation", LabelI18n: localize("Inventory GL Reconciliation", "Rekonsiliasi GL Inventori"), ActionKey: "finance.inventory_gl_reconciliation", Order: 109, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.inventory_adjustment_review", Label: "Inventory Adjustment Review", LabelI18n: localize("Inventory Adjustment Review", "Tinjauan Penyesuaian Inventori"), ActionKey: "finance.inventory_adjustment_review", Order: 110, RequiredPermissions: []string{"inventory.finance.review"}},
				{Key: "finance.inventory_reconciliation_cases", Label: "Inventory Reconciliation Cases", LabelI18n: localize("Inventory Reconciliation Cases", "Kasus Rekonsiliasi Inventori"), ActionKey: "finance.inventory_reconciliation_cases.list", Order: 111, RequiredPermissions: []string{"inventory_reconciliation_case.list"}},
			},
			Actions: []module.ActionDefinition{
				{Key: "inventory.count_sessions.list", Label: "Count Sessions", LabelI18n: localize("Count Sessions", "Sesi Hitung"), Kind: "navigate", RoutePath: "/inventory/count-sessions", ViewKey: "inventory.count_sessions.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"inventory_count_session.list"}},
				{Key: "inventory.count_sessions.detail", Label: "Count Session Detail", LabelI18n: localize("Count Session Detail", "Detail Sesi Hitung"), Kind: "navigate", RoutePath: "/inventory/count-sessions/detail", ViewKey: "inventory.count_sessions.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"inventory_count_session.read"}},
				{Key: "inventory.count_sessions.form", Label: "Count Session Form", LabelI18n: localize("Count Session Form", "Form Sesi Hitung"), Kind: "navigate", RoutePath: "/inventory/count-sessions/form", ViewKey: "inventory.count_sessions.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"inventory_count_session.update"}},
				{Key: "finance.inventory_reconciliation_cases.list", Label: "Inventory Reconciliation Cases", LabelI18n: localize("Inventory Reconciliation Cases", "Kasus Rekonsiliasi Inventori"), Kind: "navigate", RoutePath: "/finance/inventory-reconciliation-cases", ViewKey: "finance.inventory_reconciliation_cases.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"inventory_reconciliation_case.list"}},
				{Key: "finance.inventory_reconciliation_cases.detail", Label: "Inventory Reconciliation Case Detail", LabelI18n: localize("Inventory Reconciliation Case Detail", "Detail Kasus Rekonsiliasi Inventori"), Kind: "navigate", RoutePath: "/finance/inventory-reconciliation-cases/detail", ViewKey: "finance.inventory_reconciliation_cases.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"inventory_reconciliation_case.read"}},
				{Key: "finance.inventory_reconciliation_cases.form", Label: "Inventory Reconciliation Case Form", LabelI18n: localize("Inventory Reconciliation Case Form", "Form Kasus Rekonsiliasi Inventori"), Kind: "navigate", RoutePath: "/finance/inventory-reconciliation-cases/form", ViewKey: "finance.inventory_reconciliation_cases.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"inventory_reconciliation_case.update"}},
				{Key: "finance.inventory_valuation", Label: "Inventory Valuation", LabelI18n: localize("Inventory Valuation", "Penilaian Inventori"), Kind: "navigate", RoutePath: "/finance/inventory-valuation", CustomEntryKey: "finance.inventory_valuation", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.inventory_valuation_as_of", Label: "Inventory Valuation As Of", LabelI18n: localize("Inventory Valuation As Of", "Penilaian Inventori Per Tanggal"), Kind: "navigate", RoutePath: "/finance/inventory-valuation-as-of", CustomEntryKey: "finance.inventory_valuation_as_of", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.inventory_gl_reconciliation", Label: "Inventory GL Reconciliation", LabelI18n: localize("Inventory GL Reconciliation", "Rekonsiliasi GL Inventori"), Kind: "navigate", RoutePath: "/finance/inventory-gl-reconciliation", CustomEntryKey: "finance.inventory_gl_reconciliation", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.inventory_adjustment_review", Label: "Inventory Adjustment Review", LabelI18n: localize("Inventory Adjustment Review", "Tinjauan Penyesuaian Inventori"), Kind: "navigate", RoutePath: "/finance/inventory-adjustment-review", CustomEntryKey: "finance.inventory_adjustment_review", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"inventory.finance.review"}},
			},
			Views: []module.ViewDefinition{
				commercialModelListView("inventory.count_sessions.list", "Count Sessions", "inventory_count_session", []module.ColumnDefinition{
					{Key: "session_code", Label: "Session", LabelI18n: localize("Session", "Sesi"), Path: "values.session_code"},
					{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code"},
					{Key: "count_date", Label: "Count Date", LabelI18n: localize("Count Date", "Tanggal Hitung"), Path: "values.count_date"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
					{Key: "generated_adjustment_number", Label: "Adjustment", LabelI18n: localize("Adjustment", "Penyesuaian"), Path: "values.generated_adjustment_number"},
				}, []string{"draft", "open", "adjustment_generated", "closed"}),
				commercialModelDetailView("inventory.count_sessions.detail", "Count Session Detail", "inventory_count_session", []module.FieldDefinition{
					{Key: "session_code", Label: "Session", LabelI18n: localize("Session", "Sesi"), Path: "values.session_code", Type: "string"},
					{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code", Type: "string"},
					{Key: "count_date", Label: "Count Date", LabelI18n: localize("Count Date", "Tanggal Hitung"), Path: "values.count_date", Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
					{Key: "adjustment_account_code", Label: "Adjustment Account", LabelI18n: localize("Adjustment Account", "Akun Penyesuaian"), Path: "values.adjustment_account_code", Type: "string"},
					{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "values.lines", Type: "object"},
					{Key: "generated_adjustment_number", Label: "Generated Adjustment", LabelI18n: localize("Generated Adjustment", "Penyesuaian Hasil Generate"), Path: "values.generated_adjustment_number", Type: "string"},
					{Key: "note", Label: "Note", LabelI18n: localize("Note", "Catatan"), Path: "values.note", Type: "string"},
				}),
				commercialModelFormView("inventory.count_sessions.form", "Count Session Form", "inventory_count_session", []module.FieldDefinition{
					{Key: "session_code", Label: "Session Code", LabelI18n: localize("Session Code", "Kode Sesi"), Path: "values.session_code", Type: "string", Widget: "text"},
					{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code", Type: "string", Widget: "text"},
					{Key: "count_date", Label: "Count Date", LabelI18n: localize("Count Date", "Tanggal Hitung"), Path: "values.count_date", Type: "string", Widget: "text"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"draft", "open", "adjustment_generated", "closed"}},
					{Key: "adjustment_account_code", Label: "Adjustment Account", LabelI18n: localize("Adjustment Account", "Akun Penyesuaian"), Path: "values.adjustment_account_code", Type: "string", Widget: "text"},
					{Key: "lines", Label: "Lines", LabelI18n: localize("Lines", "Baris"), Path: "values.lines", Type: "object", Widget: "json"},
					{Key: "note", Label: "Note", LabelI18n: localize("Note", "Catatan"), Path: "values.note", Type: "string", Widget: "textarea"},
				}),
				commercialModelListView("finance.inventory_reconciliation_cases.list", "Inventory Reconciliation Cases", "inventory_reconciliation_case", []module.ColumnDefinition{
					{Key: "as_of_date", Label: "As Of", LabelI18n: localize("As Of", "Per Tanggal"), Path: "values.as_of_date"},
					{Key: "account_code", Label: "Account", LabelI18n: localize("Account", "Akun"), Path: "values.account_code"},
					{Key: "difference", Label: "Difference", LabelI18n: localize("Difference", "Selisih"), Path: "values.difference"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
					{Key: "assignee_user_id", Label: "Assignee", LabelI18n: localize("Assignee", "Penanggung Jawab"), Path: "values.assignee_user_id"},
				}, []string{"open", "investigating", "corrected", "explained", "closed"}),
				commercialModelDetailView("finance.inventory_reconciliation_cases.detail", "Inventory Reconciliation Case Detail", "inventory_reconciliation_case", []module.FieldDefinition{
					{Key: "as_of_date", Label: "As Of", LabelI18n: localize("As Of", "Per Tanggal"), Path: "values.as_of_date", Type: "string"},
					{Key: "account_code", Label: "Account", LabelI18n: localize("Account", "Akun"), Path: "values.account_code", Type: "string"},
					{Key: "account_name", Label: "Account Name", LabelI18n: localize("Account Name", "Nama Akun"), Path: "values.account_name", Type: "string"},
					{Key: "inventory_value", Label: "Inventory Value", LabelI18n: localize("Inventory Value", "Nilai Inventori"), Path: "values.inventory_value", Type: "number"},
					{Key: "gl_value", Label: "GL Value", LabelI18n: localize("GL Value", "Nilai GL"), Path: "values.gl_value", Type: "number"},
					{Key: "difference", Label: "Difference", LabelI18n: localize("Difference", "Selisih"), Path: "values.difference", Type: "number"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
					{Key: "assignee_user_id", Label: "Assignee", LabelI18n: localize("Assignee", "Penanggung Jawab"), Path: "values.assignee_user_id", Type: "string"},
					{Key: "note", Label: "Note", LabelI18n: localize("Note", "Catatan"), Path: "values.note", Type: "string"},
				}),
				commercialModelFormView("finance.inventory_reconciliation_cases.form", "Inventory Reconciliation Case Form", "inventory_reconciliation_case", []module.FieldDefinition{
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"open", "investigating", "corrected", "explained", "closed"}},
					{Key: "assignee_user_id", Label: "Assignee", LabelI18n: localize("Assignee", "Penanggung Jawab"), Path: "values.assignee_user_id", Type: "string", Widget: "text"},
					{Key: "linked_document_id", Label: "Linked Document", LabelI18n: localize("Linked Document", "Dokumen Terkait"), Path: "values.linked_document_id", Type: "string", Widget: "text"},
					{Key: "linked_posting_id", Label: "Linked Posting", LabelI18n: localize("Linked Posting", "Posting Terkait"), Path: "values.linked_posting_id", Type: "string", Widget: "text"},
					{Key: "note", Label: "Note", LabelI18n: localize("Note", "Catatan"), Path: "values.note", Type: "string", Widget: "textarea"},
				}),
			},
			CustomEntries: []module.CustomEntryDefinition{
				{Key: "finance.inventory_valuation", Title: "Inventory Valuation", TitleI18n: localize("Inventory Valuation", "Penilaian Inventori"), RoutePath: "/finance/inventory-valuation", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.inventory_valuation_as_of", Title: "Inventory Valuation As Of", TitleI18n: localize("Inventory Valuation As Of", "Penilaian Inventori Per Tanggal"), RoutePath: "/finance/inventory-valuation-as-of", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.inventory_gl_reconciliation", Title: "Inventory GL Reconciliation", TitleI18n: localize("Inventory GL Reconciliation", "Rekonsiliasi GL Inventori"), RoutePath: "/finance/inventory-gl-reconciliation", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.inventory_adjustment_review", Title: "Inventory Adjustment Review", TitleI18n: localize("Inventory Adjustment Review", "Tinjauan Penyesuaian Inventori"), RoutePath: "/finance/inventory-adjustment-review", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"inventory.finance.review"}},
			},
		},
	}
}
