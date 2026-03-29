package app

import (
	"orbyte/internal/platform/httpx"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
)

func financeReportingCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "finance_reporting_core",
		Name:         "Finance Reporting Core",
		NameI18n:     localize("Finance Reporting Core", "Inti Pelaporan Keuangan"),
		Version:      "1.0.0",
		DomainFamily: "business",
		Dependencies: []string{"platform.core", "commercial_core", "procurement_core", "inventory_core", "production_core"},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Finance Console",
			TitleI18n:       localize("Finance Console", "Konsol Keuangan"),
			Description:     "Financial statements, tax summary, journal review, and accounting period close.",
			DescriptionI18n: localize("Financial statements, tax summary, journal review, and accounting period close.", "Laporan keuangan, ringkasan pajak, tinjauan jurnal, dan tutup periode akuntansi."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "reports",
					Title:     "Finance Reports",
					TitleI18n: localize("Finance Reports", "Laporan Keuangan"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("trial_balance", "Trial Balance", "Neraca Saldo", "/ui/finance/trial-balance", "Open trial balance.", "Buka neraca saldo.", "finance.read"),
						adminConsoleLink("profit_loss", "Profit and Loss", "Laba Rugi", "/ui/finance/profit-and-loss", "Open profit and loss.", "Buka laporan laba rugi.", "finance.read"),
						adminConsoleLink("balance_sheet", "Balance Sheet", "Neraca", "/ui/finance/balance-sheet", "Open balance sheet.", "Buka neraca.", "finance.read"),
						adminConsoleLink("tax_summary", "Tax Summary", "Ringkasan Pajak", "/ui/finance/tax-summary", "Open tax summary.", "Buka ringkasan pajak.", "finance.read"),
						{
							Key:                 "journal_ledger",
							Label:               "Journal Ledger",
							LabelI18n:           localize("Journal Ledger", "Buku Jurnal"),
							RoutePath:           "/ui/finance/journal-ledger",
							Description:         "Open journal ledger.",
							DescriptionI18n:     localize("Open journal ledger.", "Buka buku jurnal."),
							RequiredPermissions: []string{"finance.read", "document.read"},
						},
					},
				},
				{
					Key:       "masters",
					Title:     "Accounting Setup",
					TitleI18n: localize("Accounting Setup", "Pengaturan Akuntansi"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("finance_accounts", "Finance Accounts", "Akun Keuangan", "/ui/finance/accounts", "Manage finance accounts and report grouping.", "Kelola akun keuangan dan grup laporan.", "finance_account.list"),
						adminConsoleLink("accounting_periods", "Accounting Periods", "Periode Akuntansi", "/ui/finance/periods", "Manage open and closed accounting periods.", "Kelola periode akuntansi terbuka dan tertutup.", "accounting_period.list"),
					},
				},
			},
		},
		Models: []model.Definition{
			{
				Key:                 "finance_account",
				DisplayName:         "Finance Account",
				DisplayNameI18n:     localize("Finance Account", "Akun Keuangan"),
				OwnerModuleKey:      "finance_reporting_core",
				Version:             "v1",
				CreatePermissionKey: "finance_account.create",
				ListPermissionKey:   "finance_account.list",
				ReadPermissionKey:   "finance_account.read",
				UpdatePermissionKey: "finance_account.update",
				DefaultSort:         "code",
				Fields: []model.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "account_type", Label: "Account Type", LabelI18n: localize("Account Type", "Tipe Akun"), Type: "string", Required: true, DefaultValue: "expense"},
					{Key: "report_group", Label: "Report Group", LabelI18n: localize("Report Group", "Grup Laporan"), Type: "string"},
					{Key: "normal_balance", Label: "Normal Balance", LabelI18n: localize("Normal Balance", "Saldo Normal"), Type: "string", DefaultValue: "debit"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			},
			{
				Key:                 "accounting_period",
				DisplayName:         "Accounting Period",
				DisplayNameI18n:     localize("Accounting Period", "Periode Akuntansi"),
				OwnerModuleKey:      "finance_reporting_core",
				Version:             "v1",
				CreatePermissionKey: "accounting_period.create",
				ListPermissionKey:   "accounting_period.list",
				ReadPermissionKey:   "accounting_period.read",
				UpdatePermissionKey: "accounting_period.update",
				DefaultSort:         "start_date",
				Fields: []model.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
					{Key: "period_key", Label: "Period Key", LabelI18n: localize("Period Key", "Kunci Periode"), Type: "string", Required: true},
					{Key: "start_date", Label: "Start Date", LabelI18n: localize("Start Date", "Tanggal Mulai"), Type: "string", Required: true},
					{Key: "end_date", Label: "End Date", LabelI18n: localize("End Date", "Tanggal Selesai"), Type: "string", Required: true, ConstraintRuleKeys: []string{"accounting_period.date_range"}},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "open"},
					{Key: "closed_at", Label: "Closed At", LabelI18n: localize("Closed At", "Ditutup Pada"), Type: "string"},
					{Key: "closed_by", Label: "Closed By", LabelI18n: localize("Closed By", "Ditutup Oleh"), Type: "string"},
					{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
				},
			},
		},
		Security: module.SecurityDefinition{
			Permissions: append(
				append(
					commercialModelPermissions("finance_account", "Finance Account"),
					commercialModelPermissions("accounting_period", "Accounting Period")...,
				),
				[]module.PermissionDefinition{
					{Key: "finance.read", Action: "read", Resource: "finance", DisplayName: "Read Finance Reports", DisplayNameI18n: localize("Read Finance Reports", "Lihat Laporan Keuangan")},
					{Key: "finance.close", Action: "close", Resource: "finance", DisplayName: "Close Finance Periods", DisplayNameI18n: localize("Close Finance Periods", "Tutup Periode Keuangan")},
				}...,
			),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "finance_manager",
					Name:           "Finance Manager",
					NameI18n:       localize("Finance Manager", "Manajer Keuangan"),
					AllowedScopes:  []string{"deployment", "organization", "location"},
					PermissionKeys: []string{"finance_account.create", "finance_account.list", "finance_account.read", "finance_account.update", "accounting_period.create", "accounting_period.list", "accounting_period.read", "accounting_period.update", "finance.read", "finance.close", "document.list", "document.read"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "finance.accounts", Label: "Finance Accounts", LabelI18n: localize("Finance Accounts", "Akun Keuangan"), ActionKey: "finance.accounts.list", Order: 88, RequiredPermissions: []string{"finance_account.list"}},
				{Key: "finance.periods", Label: "Accounting Periods", LabelI18n: localize("Accounting Periods", "Periode Akuntansi"), ActionKey: "finance.periods.list", Order: 89, RequiredPermissions: []string{"accounting_period.list"}},
				{Key: "finance.trial_balance", Label: "Trial Balance", LabelI18n: localize("Trial Balance", "Neraca Saldo"), ActionKey: "finance.trial_balance", Order: 90, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.profit_loss", Label: "Profit and Loss", LabelI18n: localize("Profit and Loss", "Laba Rugi"), ActionKey: "finance.profit_and_loss", Order: 91, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.balance_sheet", Label: "Balance Sheet", LabelI18n: localize("Balance Sheet", "Neraca"), ActionKey: "finance.balance_sheet", Order: 92, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.tax_summary", Label: "Tax Summary", LabelI18n: localize("Tax Summary", "Ringkasan Pajak"), ActionKey: "finance.tax_summary", Order: 93, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.journal_ledger", Label: "Journal Ledger", LabelI18n: localize("Journal Ledger", "Buku Jurnal"), ActionKey: "finance.journal_ledger", Order: 94, RequiredPermissions: []string{"finance.read", "document.read"}},
			},
			Actions: []module.ActionDefinition{
				{Key: "finance.accounts.list", Label: "Finance Accounts", LabelI18n: localize("Finance Accounts", "Akun Keuangan"), Kind: "navigate", RoutePath: "/finance/accounts", ViewKey: "finance.accounts.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"finance_account.list"}},
				{Key: "finance.accounts.detail", Label: "Finance Account Detail", LabelI18n: localize("Finance Account Detail", "Detail Akun Keuangan"), Kind: "navigate", RoutePath: "/finance/accounts/detail", ViewKey: "finance.accounts.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"finance_account.read"}},
				{Key: "finance.accounts.form", Label: "Finance Account Form", LabelI18n: localize("Finance Account Form", "Form Akun Keuangan"), Kind: "navigate", RoutePath: "/finance/accounts/form", ViewKey: "finance.accounts.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"finance_account.update"}},
				{Key: "finance.periods.list", Label: "Accounting Periods", LabelI18n: localize("Accounting Periods", "Periode Akuntansi"), Kind: "navigate", RoutePath: "/finance/periods", ViewKey: "finance.periods.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"accounting_period.list"}},
				{Key: "finance.periods.detail", Label: "Accounting Period Detail", LabelI18n: localize("Accounting Period Detail", "Detail Periode Akuntansi"), Kind: "navigate", RoutePath: "/finance/periods/detail", ViewKey: "finance.periods.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"accounting_period.read"}},
				{Key: "finance.periods.form", Label: "Accounting Period Form", LabelI18n: localize("Accounting Period Form", "Form Periode Akuntansi"), Kind: "navigate", RoutePath: "/finance/periods/form", ViewKey: "finance.periods.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"accounting_period.update"}},
				{Key: "finance.trial_balance", Label: "Trial Balance", LabelI18n: localize("Trial Balance", "Neraca Saldo"), Kind: "navigate", RoutePath: "/finance/trial-balance", CustomEntryKey: "finance.trial_balance", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.profit_and_loss", Label: "Profit and Loss", LabelI18n: localize("Profit and Loss", "Laba Rugi"), Kind: "navigate", RoutePath: "/finance/profit-and-loss", CustomEntryKey: "finance.profit_and_loss", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.balance_sheet", Label: "Balance Sheet", LabelI18n: localize("Balance Sheet", "Neraca"), Kind: "navigate", RoutePath: "/finance/balance-sheet", CustomEntryKey: "finance.balance_sheet", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.tax_summary", Label: "Tax Summary", LabelI18n: localize("Tax Summary", "Ringkasan Pajak"), Kind: "navigate", RoutePath: "/finance/tax-summary", CustomEntryKey: "finance.tax_summary", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.journal_ledger", Label: "Journal Ledger", LabelI18n: localize("Journal Ledger", "Buku Jurnal"), Kind: "navigate", RoutePath: "/finance/journal-ledger", CustomEntryKey: "finance.journal_ledger", RenderMode: module.RenderModeCustom, RequiredPermissions: []string{"finance.read", "document.read"}},
			},
			Views: []module.ViewDefinition{
				commercialModelListView("finance.accounts.list", "Finance Accounts", "finance_account", []module.ColumnDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
					{Key: "account_type", Label: "Type", LabelI18n: localize("Type", "Tipe"), Path: "values.account_type"},
					{Key: "report_group", Label: "Group", LabelI18n: localize("Group", "Grup"), Path: "values.report_group"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"active", "inactive"}),
				commercialModelDetailView("finance.accounts.detail", "Finance Account Detail", "finance_account", []module.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
					{Key: "account_type", Label: "Account Type", LabelI18n: localize("Account Type", "Tipe Akun"), Path: "values.account_type", Type: "string"},
					{Key: "report_group", Label: "Report Group", LabelI18n: localize("Report Group", "Grup Laporan"), Path: "values.report_group", Type: "string"},
					{Key: "normal_balance", Label: "Normal Balance", LabelI18n: localize("Normal Balance", "Saldo Normal"), Path: "values.normal_balance", Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
				}),
				commercialModelFormView("finance.accounts.form", "Finance Account Form", "finance_account", []module.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
					{Key: "account_type", Label: "Account Type", LabelI18n: localize("Account Type", "Tipe Akun"), Path: "values.account_type", Type: "string", Widget: "select", Options: []string{"asset", "liability", "equity", "revenue", "expense"}},
					{Key: "report_group", Label: "Report Group", LabelI18n: localize("Report Group", "Grup Laporan"), Path: "values.report_group", Type: "string", Widget: "text"},
					{Key: "normal_balance", Label: "Normal Balance", LabelI18n: localize("Normal Balance", "Saldo Normal"), Path: "values.normal_balance", Type: "string", Widget: "select", Options: []string{"debit", "credit"}},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
				}),
				commercialModelListView("finance.periods.list", "Accounting Periods", "accounting_period", []module.ColumnDefinition{
					{Key: "period_key", Label: "Period", LabelI18n: localize("Period", "Periode"), Path: "values.period_key"},
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id"},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id"},
					{Key: "start_date", Label: "Start", LabelI18n: localize("Start", "Mulai"), Path: "values.start_date"},
					{Key: "end_date", Label: "End", LabelI18n: localize("End", "Selesai"), Path: "values.end_date"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
				}, []string{"open", "closed"}),
				commercialModelDetailView("finance.periods.detail", "Accounting Period Detail", "accounting_period", []module.FieldDefinition{
					{Key: "period_key", Label: "Period", LabelI18n: localize("Period", "Periode"), Path: "values.period_key", Type: "string"},
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string"},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string"},
					{Key: "start_date", Label: "Start Date", LabelI18n: localize("Start Date", "Tanggal Mulai"), Path: "values.start_date", Type: "string"},
					{Key: "end_date", Label: "End Date", LabelI18n: localize("End Date", "Tanggal Selesai"), Path: "values.end_date", Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
					{Key: "closed_at", Label: "Closed At", LabelI18n: localize("Closed At", "Ditutup Pada"), Path: "values.closed_at", Type: "string"},
					{Key: "closed_by", Label: "Closed By", LabelI18n: localize("Closed By", "Ditutup Oleh"), Path: "values.closed_by", Type: "string"},
					{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string"},
				}),
				commercialModelFormView("finance.periods.form", "Accounting Period Form", "accounting_period", []module.FieldDefinition{
					{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Path: "values.organization_id", Type: "string", Widget: "text", Required: true},
					{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: "text"},
					{Key: "period_key", Label: "Period Key", LabelI18n: localize("Period Key", "Kunci Periode"), Path: "values.period_key", Type: "string", Widget: "text", Required: true},
					{Key: "start_date", Label: "Start Date", LabelI18n: localize("Start Date", "Tanggal Mulai"), Path: "values.start_date", Type: "string", Widget: "text", Required: true},
					{Key: "end_date", Label: "End Date", LabelI18n: localize("End Date", "Tanggal Selesai"), Path: "values.end_date", Type: "string", Widget: "text", Required: true},
					{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string", Widget: "textarea"},
				}),
			},
			CustomEntries: []module.CustomEntryDefinition{
				{Key: "finance.trial_balance", Title: "Trial Balance", TitleI18n: localize("Trial Balance", "Neraca Saldo"), RoutePath: "/finance/trial-balance", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.profit_and_loss", Title: "Profit and Loss", TitleI18n: localize("Profit and Loss", "Laba Rugi"), RoutePath: "/finance/profit-and-loss", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.balance_sheet", Title: "Balance Sheet", TitleI18n: localize("Balance Sheet", "Neraca"), RoutePath: "/finance/balance-sheet", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.tax_summary", Title: "Tax Summary", TitleI18n: localize("Tax Summary", "Ringkasan Pajak"), RoutePath: "/finance/tax-summary", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"finance.read"}},
				{Key: "finance.journal_ledger", Title: "Journal Ledger", TitleI18n: localize("Journal Ledger", "Buku Jurnal"), RoutePath: "/finance/journal-ledger", BundleKey: "finance-reports", ComponentExport: "render", RequiredPermissions: []string{"finance.read", "document.read"}},
			},
		},
		Bundles: []module.BundleDefinition{{
			Key:    "finance-reports",
			Script: httpx.FinanceReportsBundle(),
		}},
	}
}

func financeReportingCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{financeReportingCoreKernelPackManifest()}
}
