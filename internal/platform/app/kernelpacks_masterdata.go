package app

import (
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

func masterdataKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "masterdata",
		Name:         "Master Data",
		NameI18n:     localize("Master Data", "Data Master"),
		Version:      "1.0.0",
		DomainFamily: "platform",
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "reference_masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:           "Master Data Console",
			TitleI18n:       localize("Master Data Console", "Konsol Data Master"),
			Description:     "Master data operations and related commercial/customer entry points.",
			DescriptionI18n: localize("Master data operations and related commercial/customer entry points.", "Operasi data master dan pintu masuk pelanggan/komersial terkait."),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "masterdata_operations",
					Title:     "Master Data Operations",
					TitleI18n: localize("Master Data Operations", "Operasi Data Master"),
					Kind:      module.AdminConsoleSectionResourceLinks,
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("parties", "Parties", "Pihak", "/ui/masterdata/parties", "Open party master records.", "Buka data master pihak.", "party.list"),
						adminConsoleLink("catalog", "Catalog", "Katalog", "/ui/commercial/catalog", "Open the sellable catalog linked to parties.", "Buka katalog jual terkait pihak.", "item.list"),
						adminConsoleLink("receivables", "Receivables", "Piutang", "/ui/commercial/receivables", "Open receivables tied to parties.", "Buka piutang yang terkait pihak.", "document.list"),
					},
				},
			},
		},
		Models: []model.Definition{{
			Key:                 "party",
			DisplayName:         "Party",
			DisplayNameI18n:     localize("Party", "Pihak"),
			OwnerModuleKey:      "masterdata",
			Version:             "v1",
			CreatePermissionKey: "party.create",
			ListPermissionKey:   "party.list",
			ReadPermissionKey:   "party.read",
			UpdatePermissionKey: "party.update",
			DefaultSort:         "name",
			Fields: []model.FieldDefinition{
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "display_name", Label: "Display Name", LabelI18n: localize("Display Name", "Nama Tampil"), Type: "string", ReadOnly: true, ComputeRuleKey: "party.display_name.compute"},
				{Key: "email", Label: "Email", LabelI18n: localize("Email", "Email"), Type: "string"},
				{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Type: "string"},
				{Key: "customer_type", Label: "Customer Type", LabelI18n: localize("Customer Type", "Tipe Pelanggan"), Type: "string"},
				{Key: "member_status", Label: "Member Status", LabelI18n: localize("Member Status", "Status Member"), Type: "string"},
				{Key: "member_tier", Label: "Member Tier", LabelI18n: localize("Member Tier", "Tier Member"), Type: "string"},
				{Key: "member_valid_from", Label: "Member Valid From", LabelI18n: localize("Member Valid From", "Member Berlaku Dari"), Type: "string"},
				{Key: "member_valid_to", Label: "Member Valid To", LabelI18n: localize("Member Valid To", "Member Berlaku Sampai"), Type: "string"},
				{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Type: "string"},
				{Key: "default_price_list_code", Label: "Default Price List", LabelI18n: localize("Default Price List", "Daftar Harga Default"), Type: "string"},
				{Key: "payment_term_days", Label: "Payment Term Days", LabelI18n: localize("Payment Term Days", "Hari Termin Pembayaran"), Type: "number"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultRuleKey: "party.status.default", ConstraintRuleKeys: []string{"party.status.allowed"}},
			},
			Relations: []model.RelationDefinition{
				{Key: "contacts", Type: "has_many", TargetModelKey: "party_contact", ForeignKey: "party_id"},
			},
		}, {
			Key:                 "party_contact",
			DisplayName:         "Party Contact",
			DisplayNameI18n:     localize("Party Contact", "Kontak Pihak"),
			OwnerModuleKey:      "masterdata",
			Version:             "v1",
			CreatePermissionKey: "party.update",
			ListPermissionKey:   "party.read",
			ReadPermissionKey:   "party.read",
			UpdatePermissionKey: "party.update",
			DefaultSort:         "name",
			Fields: []model.FieldDefinition{
				{Key: "party_id", Label: "Party ID", LabelI18n: localize("Party ID", "ID Pihak"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "phone", Label: "Phone", LabelI18n: localize("Phone", "Telepon"), Type: "string"},
				{Key: "role", Label: "Role", LabelI18n: localize("Role", "Peran"), Type: "string"},
			},
		}},
		Datasets: []module.DatasetDefinition{{
			Key:        "masterdata.party.summary",
			Title:      "Party Summary",
			TitleI18n:  localize("Party Summary", "Ringkasan Pihak"),
			SourceKind: "model",
			ModelKey:   "party",
			Dimensions: []module.DatasetDimension{{Key: "by_status", Label: "By Status", LabelI18n: localize("By Status", "Berdasarkan Status"), Path: "status"}},
			Measures:   []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}},
		}},
		SearchIndexes: []search.IndexDefinition{{
			Key:                 "masterdata.party.search",
			Title:               "Party Search",
			SourceKind:          "model",
			ModelKey:            "party",
			ViewKey:             "masterdata.parties.list",
			Modes:               []string{"keyword", "vector", "hybrid"},
			OrganizationSplit:   true,
			RequiredPermissions: []string{"party.list"},
			QueryFilterFields:   []string{"status", "location_id"},
			QuerySortFields:     []string{"name", "updated_at"},
			Fields: []search.IndexFieldDefinition{
				{Key: "name", Path: "name", Type: "string", Searchable: true, Sort: true},
				{Key: "email", Path: "email", Type: "string", Searchable: true},
				{Key: "status", Path: "status", Type: "string", Facet: true, Sort: true},
			},
			VectorFields: []search.VectorFieldDefinition{{
				Key: "semantic", SourcePaths: []string{"name", "email"}, EmbeddingMode: "external", Dimensions: 8, DistanceMetric: "cosine",
			}},
		}},
		Security: module.SecurityDefinition{
			Permissions: []module.PermissionDefinition{
				{Key: "party.create", Action: "create", Resource: "party", DisplayName: "Create Parties", DisplayNameI18n: localize("Create Parties", "Buat Pihak")},
				{Key: "party.list", Action: "list", Resource: "party", DisplayName: "List Parties", DisplayNameI18n: localize("List Parties", "Daftar Pihak")},
				{Key: "party.read", Action: "read", Resource: "party", DisplayName: "Read Parties", DisplayNameI18n: localize("Read Parties", "Lihat Pihak")},
				{Key: "party.update", Action: "update", Resource: "party", DisplayName: "Update Parties", DisplayNameI18n: localize("Update Parties", "Perbarui Pihak")},
			},
			RoleTemplates: []module.RoleTemplateDefinition{{
				Key: "party_manager", Name: "Party Manager", NameI18n: localize("Party Manager", "Pengelola Pihak"), AllowedScopes: []string{"deployment", "location"}, PermissionKeys: []string{"party.create", "party.list", "party.read", "party.update"},
			}},
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{{
				Key:                 "masterdata.parties",
				Label:               "Parties",
				LabelI18n:           localize("Parties", "Pihak"),
				ActionKey:           "masterdata.parties.list",
				Order:               5,
				RequiredPermissions: []string{"party.list"},
			}},
			Actions: []module.ActionDefinition{
				{Key: "masterdata.parties.list", Label: "Parties", LabelI18n: localize("Parties", "Pihak"), Kind: "navigate", RoutePath: "/masterdata/parties", ViewKey: "masterdata.parties.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"party.list"}},
				{Key: "masterdata.parties.detail", Label: "Party Detail", LabelI18n: localize("Party Detail", "Detail Pihak"), Kind: "navigate", RoutePath: "/masterdata/parties/detail", ViewKey: "masterdata.parties.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"party.read"}},
				{Key: "masterdata.parties.form", Label: "Party Form", LabelI18n: localize("Party Form", "Form Pihak"), Kind: "navigate", RoutePath: "/masterdata/parties/form", ViewKey: "masterdata.parties.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"party.update"}},
			},
			Views: []module.ViewDefinition{
				{
					Key: "masterdata.parties.list", Title: "Parties", TitleI18n: localize("Parties", "Pihak"), Kind: "list", ModelKey: "party", RequiredPermissions: []string{"party.list"},
					Columns: []module.ColumnDefinition{
						{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
						{Key: "email", Label: "Email", LabelI18n: localize("Email", "Email"), Path: "values.email"},
						{Key: "customer_type", Label: "Customer Type", LabelI18n: localize("Customer Type", "Tipe Pelanggan"), Path: "values.customer_type"},
						{Key: "member_status", Label: "Member Status", LabelI18n: localize("Member Status", "Status Member"), Path: "values.member_status"},
						{Key: "member_tier", Label: "Member Tier", LabelI18n: localize("Member Tier", "Tier Member"), Path: "values.member_tier"},
						{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "values.tax_profile_code"},
						{Key: "default_price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Path: "values.default_price_list_code"},
						{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
					},
					Filters:         []module.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: []string{"active", "inactive", "blocked"}}},
					DefaultPageSize: 10,
					EmptyState:      "No parties registered yet.",
					EmptyStateI18n:  localize("No parties registered yet.", "Belum ada pihak terdaftar."),
				},
				{
					Key: "masterdata.parties.detail", Title: "Party Detail", TitleI18n: localize("Party Detail", "Detail Pihak"), Kind: "detail", ModelKey: "party", RequiredPermissions: []string{"party.read"},
					Tabs: []module.TabDefinition{{
						Key: "summary", Title: "Summary", TitleI18n: localize("Summary", "Ringkasan"), Sections: []module.SectionDefinition{{
							Key: "core", Title: "Core Fields", TitleI18n: localize("Core Fields", "Field Utama"), Fields: []module.FieldDefinition{
								{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
								{Key: "display_name", Label: "Display Name", LabelI18n: localize("Display Name", "Nama Tampil"), Path: "values.display_name", Type: "string"},
								{Key: "email", Label: "Email", LabelI18n: localize("Email", "Email"), Path: "values.email", Type: "string"},
								{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string"},
								{Key: "customer_type", Label: "Customer Type", LabelI18n: localize("Customer Type", "Tipe Pelanggan"), Path: "values.customer_type", Type: "string"},
								{Key: "member_status", Label: "Member Status", LabelI18n: localize("Member Status", "Status Member"), Path: "values.member_status", Type: "string"},
								{Key: "member_tier", Label: "Member Tier", LabelI18n: localize("Member Tier", "Tier Member"), Path: "values.member_tier", Type: "string"},
								{Key: "member_valid_from", Label: "Member Valid From", LabelI18n: localize("Member Valid From", "Member Berlaku Dari"), Path: "values.member_valid_from", Type: "string"},
								{Key: "member_valid_to", Label: "Member Valid To", LabelI18n: localize("Member Valid To", "Member Berlaku Sampai"), Path: "values.member_valid_to", Type: "string"},
								{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "values.tax_profile_code", Type: "string"},
								{Key: "default_price_list_code", Label: "Default Price List", LabelI18n: localize("Default Price List", "Daftar Harga Default"), Path: "values.default_price_list_code", Type: "string"},
								{Key: "payment_term_days", Label: "Payment Term Days", LabelI18n: localize("Payment Term Days", "Hari Termin Pembayaran"), Path: "values.payment_term_days", Type: "number"},
								{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
							},
						}},
					}},
					RelatedViews: []module.RelatedViewDefinition{
						{Key: "timeline", Title: "Timeline", TitleI18n: localize("Timeline", "Linimasa"), Source: "timeline", EmptyState: "No activity yet", EmptyStateI18n: localize("No activity yet", "Belum ada aktivitas")},
						{Key: "contacts", Title: "Contacts", TitleI18n: localize("Contacts", "Kontak"), Source: "contacts", EmptyState: "No related contacts", EmptyStateI18n: localize("No related contacts", "Belum ada kontak terkait")},
					},
				},
				{
					Key: "masterdata.parties.form", Title: "Party Form", TitleI18n: localize("Party Form", "Form Pihak"), Kind: "form", ModelKey: "party", RequiredPermissions: []string{"party.update"},
					Sections: []module.SectionDefinition{{
						Key: "edit", Title: "Edit Party", TitleI18n: localize("Edit Party", "Ubah Pihak"), Fields: []module.FieldDefinition{
							{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Placeholder: "Party name", PlaceholderI18n: localize("Party name", "Nama pihak")},
							{Key: "email", Label: "Email", LabelI18n: localize("Email", "Email"), Path: "values.email", Type: "string", Widget: "text", Placeholder: "Email address", PlaceholderI18n: localize("Email address", "Alamat email")},
							{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string", Widget: "text", Placeholder: "Default currency", PlaceholderI18n: localize("Default currency", "Mata uang default")},
							{Key: "customer_type", Label: "Customer Type", LabelI18n: localize("Customer Type", "Tipe Pelanggan"), Path: "values.customer_type", Type: "string", Widget: "select", Options: []string{"retail", "member", "vip", "corporate"}},
							{Key: "member_status", Label: "Member Status", LabelI18n: localize("Member Status", "Status Member"), Path: "values.member_status", Type: "string", Widget: "select", Options: []string{"inactive", "active", "expired"}},
							{Key: "member_tier", Label: "Member Tier", LabelI18n: localize("Member Tier", "Tier Member"), Path: "values.member_tier", Type: "string", Widget: "text", Placeholder: "Member tier", PlaceholderI18n: localize("Member tier", "Tier member")},
							{Key: "member_valid_from", Label: "Member Valid From", LabelI18n: localize("Member Valid From", "Member Berlaku Dari"), Path: "values.member_valid_from", Type: "string", Widget: "text", Placeholder: "YYYY-MM-DD", PlaceholderI18n: localize("YYYY-MM-DD", "YYYY-MM-DD")},
							{Key: "member_valid_to", Label: "Member Valid To", LabelI18n: localize("Member Valid To", "Member Berlaku Sampai"), Path: "values.member_valid_to", Type: "string", Widget: "text", Placeholder: "YYYY-MM-DD", PlaceholderI18n: localize("YYYY-MM-DD", "YYYY-MM-DD")},
							{Key: "tax_profile_code", Label: "Tax Profile Code", LabelI18n: localize("Tax Profile Code", "Kode Profil Pajak"), Path: "values.tax_profile_code", Type: "string", Widget: "select", Placeholder: "Commercial tax profile", PlaceholderI18n: localize("Commercial tax profile", "Profil pajak komersial")},
							{Key: "default_price_list_code", Label: "Default Price List", LabelI18n: localize("Default Price List", "Daftar Harga Default"), Path: "values.default_price_list_code", Type: "string", Widget: "select", Placeholder: "Commercial price list", PlaceholderI18n: localize("Commercial price list", "Daftar harga komersial")},
							{Key: "payment_term_days", Label: "Payment Term Days", LabelI18n: localize("Payment Term Days", "Hari Termin Pembayaran"), Path: "values.payment_term_days", Type: "number"},
							{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive", "blocked"}},
						},
					}},
					RelatedViews: []module.RelatedViewDefinition{
						{Key: "contacts", Title: "Contacts", TitleI18n: localize("Contacts", "Kontak"), Source: "contacts", EmptyState: "No related contacts", EmptyStateI18n: localize("No related contacts", "Belum ada kontak terkait")},
					},
				},
			},
		},
		Offline: module.OfflineDefinition{
			Projections: []module.OfflineProjectionDefinition{{
				IndexKey:             "masterdata.party.search",
				Title:                "Party Search",
				TitleI18n:            localize("Party Search", "Pencarian Pihak"),
				RequiredPermissions:  []string{"party.list"},
				DefaultIncludeFields: []string{"name", "email", "status"},
			}},
			Models: []module.OfflineModelDefinition{{
				ModelKey:            "party",
				Title:               "Party",
				TitleI18n:           localize("Party", "Pihak"),
				CreatePermissionKey: "party.create",
				UpdatePermissionKey: "party.update",
				RequiredPermissions: []string{"party.read"},
			}},
		},
		Templates: []module.TemplateDefinition{{
			Key:                 "documents.generic_request.default",
			Title:               "Generic Request Print",
			TitleI18n:           localize("Generic Request Print", "Cetak Permintaan Generik"),
			Description:         "Default printable request template.",
			DescriptionI18n:     localize("Default printable request template.", "Template cetak default untuk permintaan."),
			TargetKind:          "document",
			TargetKey:           "generic_request",
			RendererKind:        "html",
			DefaultFormat:       "html",
			Formats:             []string{"html", "pdf"},
			Purpose:             "official",
			Channel:             "print",
			AllowedScopes:       []string{"deployment", "organization", "location"},
			RequiredPermissions: []string{"template.read", "template.render"},
			DefaultBody: `<article class="print-sheet">
  <header>
    <h1>Generic Request</h1>
    <p>{{ .document.Header.Number }}</p>
  </header>
  <dl>
    <dt>Status</dt><dd>{{ .document.Header.Status }}</dd>
    <dt>Title</dt><dd>{{ index .document.Body.Payload "title" }}</dd>
    <dt>Organization</dt><dd>{{ .document.Header.OrganizationID }}</dd>
    <dt>Location</dt><dd>{{ .document.Header.LocationID }}</dd>
  </dl>
</article>`,
			DefaultStyle: `.print-sheet{font-family:Arial,sans-serif;padding:24px;color:#0f172a}.print-sheet h1{margin:0 0 8px}.print-sheet dl{display:grid;grid-template-columns:160px 1fr;gap:8px 12px}`,
		}},
	}
}
