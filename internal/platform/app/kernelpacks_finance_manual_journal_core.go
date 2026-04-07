package app

import "orbyte/internal/platform/module"

func financeManualJournalCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{financeManualJournalCoreKernelPackManifest()}
}

func financeManualJournalCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:                    "finance_manual_journal_core",
		Name:                   "Finance Manual Journal Core",
		NameI18n:               localize("Finance Manual Journal Core", "Inti Jurnal Manual Keuangan"),
		Version:                "1.0.0",
		DomainFamily:           "business",
		DependencyRequirements: requiredModuleDependencies("platform.core", "commercial_core", "finance_reporting_core"),
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "finance.manual_journals", Label: "Manual Journals", LabelI18n: localize("Manual Journals", "Jurnal Manual"), ActionKey: "finance.manual_journals.list", Order: 102, RequiredPermissions: []string{"finance.journal.read"}},
				{Key: "finance.manual_journal_approvals", Label: "Journal Approvals", LabelI18n: localize("Journal Approvals", "Persetujuan Jurnal"), ActionKey: "finance.manual_journals.pending", Order: 103, RequiredPermissions: []string{"finance.journal.approve"}},
			},
			Actions: []module.ActionDefinition{
				{Key: "finance.manual_journals.list", Label: "Manual Journals", LabelI18n: localize("Manual Journals", "Jurnal Manual"), Kind: "navigate", RoutePath: "/finance/manual-journals", ViewKey: "finance.manual_journals.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"finance.journal.read"}},
				{Key: "finance.manual_journals.pending", Label: "Journal Approvals", LabelI18n: localize("Journal Approvals", "Persetujuan Jurnal"), Kind: "navigate", RoutePath: "/finance/manual-journals/pending", ViewKey: "finance.manual_journals.pending", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"finance.journal.approve"}},
				{Key: "finance.manual_journals.detail", Label: "Manual Journal Detail", LabelI18n: localize("Manual Journal Detail", "Detail Jurnal Manual"), Kind: "navigate", RoutePath: "/finance/manual-journals/detail", ViewKey: "finance.manual_journals.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"finance.journal.read"}},
				{Key: "finance.manual_journals.form", Label: "Manual Journal Form", LabelI18n: localize("Manual Journal Form", "Form Jurnal Manual"), Kind: "navigate", RoutePath: "/finance/manual-journals/form", ViewKey: "finance.manual_journals.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"finance.journal.create"}},
				{Key: "finance.manual_journals.new", Label: "New Manual Journal", LabelI18n: localize("New Manual Journal", "Jurnal Manual Baru"), Kind: "navigate", RoutePath: "/finance/manual-journals/new", ViewKey: "finance.manual_journals.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"finance.journal.create"}},
			},
			Views: []module.ViewDefinition{
				{
					Key:                 "finance.manual_journals.list",
					Title:               "Manual Journals",
					TitleI18n:           localize("Manual Journals", "Jurnal Manual"),
					Kind:                "list",
					DocumentType:        "ledger_posting",
					ProjectionKey:       "document_summary",
					RequiredPermissions: []string{"finance.journal.read"},
					Columns:             ledgerColumns(),
					Filters: []module.FilterDefinition{
						{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: []string{"draft", "submitted", "posted", "rejected", "cancelled"}},
						{Key: "journal_source_kind", Label: "Source Kind", LabelI18n: localize("Source Kind", "Jenis Sumber"), Type: "enum", Options: []string{"manual"}},
						{Key: "manual_journal_type", Label: "Journal Type", LabelI18n: localize("Journal Type", "Tipe Jurnal"), Type: "enum", Options: []string{"adjusting", "opening_balance", "reclass", "correction", "other"}},
					},
					DefaultPageSize: 10,
				},
				{
					Key:                 "finance.manual_journals.pending",
					Title:               "Journal Approvals",
					TitleI18n:           localize("Journal Approvals", "Persetujuan Jurnal"),
					Kind:                "list",
					DocumentType:        "ledger_posting",
					ProjectionKey:       "document_summary",
					RequiredPermissions: []string{"finance.journal.approve"},
					Columns:             ledgerColumns(),
					Filters: []module.FilterDefinition{
						{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: []string{"submitted"}},
						{Key: "journal_source_kind", Label: "Source Kind", LabelI18n: localize("Source Kind", "Jenis Sumber"), Type: "enum", Options: []string{"manual"}},
					},
					DefaultPageSize: 10,
				},
				{
					Key:                 "finance.manual_journals.detail",
					Title:               "Manual Journal Detail",
					TitleI18n:           localize("Manual Journal Detail", "Detail Jurnal Manual"),
					Kind:                "detail",
					DocumentType:        "ledger_posting",
					Printable:           true,
					PrintPurpose:        "official",
					RequiredPermissions: []string{"finance.journal.read"},
					Sections:            ledgerSections(),
				},
				{
					Key:                 "finance.manual_journals.form",
					Title:               "Manual Journal Form",
					TitleI18n:           localize("Manual Journal Form", "Form Jurnal Manual"),
					Kind:                "form",
					DocumentType:        "ledger_posting",
					RequiredPermissions: []string{"finance.journal.create"},
					Sections:            ledgerFormSections(),
				},
			},
		},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "finance.journal.create", Action: "create", Resource: "finance_manual_journal", DisplayName: "Create Manual Journals", DisplayNameI18n: localize("Create Manual Journals", "Buat Jurnal Manual")},
				{Key: "finance.journal.read", Action: "read", Resource: "finance_manual_journal", DisplayName: "Read Manual Journals", DisplayNameI18n: localize("Read Manual Journals", "Lihat Jurnal Manual")},
				{Key: "finance.journal.submit", Action: "submit", Resource: "finance_manual_journal", DisplayName: "Submit Manual Journals", DisplayNameI18n: localize("Submit Manual Journals", "Ajukan Jurnal Manual")},
				{Key: "finance.journal.approve", Action: "approve", Resource: "finance_manual_journal", DisplayName: "Approve Manual Journals", DisplayNameI18n: localize("Approve Manual Journals", "Setujui Jurnal Manual")},
				{Key: "finance.journal.reject", Action: "reject", Resource: "finance_manual_journal", DisplayName: "Reject Manual Journals", DisplayNameI18n: localize("Reject Manual Journals", "Tolak Jurnal Manual")},
				{Key: "finance.journal.cancel", Action: "cancel", Resource: "finance_manual_journal", DisplayName: "Cancel Manual Journals", DisplayNameI18n: localize("Cancel Manual Journals", "Batalkan Jurnal Manual")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "finance_journal_manager",
					Name:           "Finance Journal Manager",
					NameI18n:       localize("Finance Journal Manager", "Manajer Jurnal Keuangan"),
					AllowedScopes:  []string{"deployment", "organization", "location"},
					PermissionKeys: []string{"finance.journal.create", "finance.journal.read", "finance.journal.submit", "finance.journal.approve", "finance.journal.reject", "finance.journal.cancel"},
				},
				{
					Key:            "finance_journal_approver",
					Name:           "Finance Journal Approver",
					NameI18n:       localize("Finance Journal Approver", "Penyetuju Jurnal Keuangan"),
					AllowedScopes:  []string{"deployment", "organization", "location"},
					PermissionKeys: []string{"finance.journal.read", "finance.journal.approve", "finance.journal.reject"},
				},
			},
		},
	}
}
