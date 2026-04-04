package app

import (
	"orbyte/internal/platform/httpx"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
)

func posCoreKernelPackManifests() []module.Manifest {
	return []module.Manifest{posCoreKernelPackManifest()}
}

func posCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "pos_core",
		Name:         "POS Core",
		NameI18n:     localize("POS Core", "Inti POS"),
		Version:      "1.0.0",
		DomainFamily: "business",
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "commercial_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "inventory_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "fulfillment_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "returns_core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:       "POS Console",
			TitleI18n:   localize("POS Console", "Konsol POS"),
			Description: "Register setup, tender setup, store configuration, and front-counter shortcuts.",
			DescriptionI18n: localize(
				"Register setup, tender setup, store configuration, and front-counter shortcuts.",
				"Pengaturan register, tender, konfigurasi toko, dan pintasan front-counter.",
			),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:       "pos_setup",
					Kind:      module.AdminConsoleSectionResourceLinks,
					Title:     "POS Setup",
					TitleI18n: localize("POS Setup", "Setup POS"),
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("pos_terminal", "POS Terminal", "Terminal POS", "/ui/pos/terminal", "Open the cashier terminal.", "Buka terminal kasir.", "pos_sale.create"),
						adminConsoleLink("pos_stores", "POS Stores", "Toko POS", "/ui/pos/stores", "Manage stores and warehouse defaults.", "Kelola toko dan default gudang.", "pos_store.list"),
						adminConsoleLink("pos_registers", "POS Registers", "Register POS", "/ui/pos/registers", "Manage cashier registers and settlement defaults.", "Kelola register kasir dan default settlement.", "pos_register.list"),
						adminConsoleLink("pos_tender_types", "POS Tender Types", "Jenis Tender POS", "/ui/pos/tender-types", "Manage tender methods and payment mappings.", "Kelola metode tender dan mapping pembayaran.", "pos_tender_type.list"),
					},
				},
				{
					Key:       "pos_operations",
					Kind:      module.AdminConsoleSectionResourceLinks,
					Title:     "POS Operations",
					TitleI18n: localize("POS Operations", "Operasi POS"),
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("pos_shifts", "POS Shifts", "Shift POS", "/ui/pos/shifts", "Review open and closed cashier shifts.", "Tinjau shift kasir yang buka dan tutup.", "pos_shift.list"),
						adminConsoleLink("pos_sales", "POS Sales", "Penjualan POS", "/ui/pos/sales", "Review cashier transactions.", "Tinjau transaksi kasir.", "pos_sale.list"),
					},
				},
			},
		},
		Models: []model.Definition{
			posModelDefinition("pos_store", "POS Store", "pos_store", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Type: "string", Required: true},
				{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Type: "string"},
				{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Type: "string"},
				{Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Type: "string"},
				{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Type: "string", DefaultValue: "IDR"},
				{Key: "checkout_mode", Label: "Checkout Mode", LabelI18n: localize("Checkout Mode", "Mode Checkout"), Type: "string", DefaultValue: "invoice_first"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			posModelDefinition("pos_register", "POS Register", "pos_register", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Type: "string", Required: true},
				{Key: "checkout_mode", Label: "Checkout Mode", LabelI18n: localize("Checkout Mode", "Mode Checkout"), Type: "string"},
				{Key: "cash_account_code", Label: "Cash Account", LabelI18n: localize("Cash Account", "Akun Kas"), Type: "string"},
				{Key: "card_account_code", Label: "Card Account", LabelI18n: localize("Card Account", "Akun Kartu"), Type: "string"},
				{Key: "hardware_profile", Label: "Hardware Profile", LabelI18n: localize("Hardware Profile", "Profil Perangkat"), Type: "string"},
				{Key: "receipt_template_key", Label: "Receipt Template", LabelI18n: localize("Receipt Template", "Template Struk"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			posModelDefinition("pos_tender_type", "POS Tender Type", "pos_tender_type", []model.FieldDefinition{
				{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
				{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
				{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Type: "string", Required: true},
				{Key: "payment_method_code", Label: "Payment Method", LabelI18n: localize("Payment Method", "Metode Pembayaran"), Type: "string", Required: true},
				{Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Type: "string"},
				{Key: "liability_account_code", Label: "Liability Account", LabelI18n: localize("Liability Account", "Akun Liabilitas"), Type: "string"},
				{Key: "requires_party", Label: "Requires Party", LabelI18n: localize("Requires Party", "Butuh Pihak"), Type: "bool"},
				{Key: "requires_reference", Label: "Requires Reference", LabelI18n: localize("Requires Reference", "Butuh Referensi"), Type: "bool"},
				{Key: "is_cash_like", Label: "Cash Like", LabelI18n: localize("Cash Like", "Seperti Tunai"), Type: "bool"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
			}),
			posModelDefinition("pos_shift", "POS Shift", "pos_shift", []model.FieldDefinition{
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "shift_number", Label: "Shift Number", LabelI18n: localize("Shift Number", "Nomor Shift"), Type: "string", Required: true},
				{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Type: "string", Required: true},
				{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Type: "string", Required: true},
				{Key: "cashier_user_id", Label: "Cashier", LabelI18n: localize("Cashier", "Kasir"), Type: "string", Required: true},
				{Key: "cashier_employee_id", Label: "Cashier Employee", LabelI18n: localize("Cashier Employee", "Karyawan Kasir"), Type: "string"},
				{Key: "roster_slot_id", Label: "Roster Slot", LabelI18n: localize("Roster Slot", "Slot Roster"), Type: "string"},
				{Key: "attendance_day_id", Label: "Attendance Day", LabelI18n: localize("Attendance Day", "Hari Kehadiran"), Type: "string"},
				{Key: "opened_at", Label: "Opened At", LabelI18n: localize("Opened At", "Dibuka Pada"), Type: "string"},
				{Key: "closed_at", Label: "Closed At", LabelI18n: localize("Closed At", "Ditutup Pada"), Type: "string"},
				{Key: "opening_cash_amount", Label: "Opening Cash", LabelI18n: localize("Opening Cash", "Kas Awal"), Type: "number"},
				{Key: "expected_cash_amount", Label: "Expected Cash", LabelI18n: localize("Expected Cash", "Kas Diharapkan"), Type: "number"},
				{Key: "actual_cash_amount", Label: "Actual Cash", LabelI18n: localize("Actual Cash", "Kas Aktual"), Type: "number"},
				{Key: "over_short_amount", Label: "Over / Short", LabelI18n: localize("Over / Short", "Lebih / Kurang"), Type: "number"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "draft"},
				{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
			}),
			posModelDefinition("pos_sale", "POS Sale", "pos_sale", []model.FieldDefinition{
				{Key: "organization_id", Label: "Organization", LabelI18n: localize("Organization", "Organisasi"), Type: "string"},
				{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Type: "string"},
				{Key: "sale_number", Label: "Sale Number", LabelI18n: localize("Sale Number", "Nomor Penjualan"), Type: "string", Required: true},
				{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Type: "string", Required: true},
				{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Type: "string", Required: true},
				{Key: "shift_id", Label: "Shift", LabelI18n: localize("Shift", "Shift"), Type: "string", Required: true},
				{Key: "cashier_user_id", Label: "Cashier", LabelI18n: localize("Cashier", "Kasir"), Type: "string", Required: true},
				{Key: "party_id", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Type: "string"},
				{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Type: "string"},
				{Key: "checkout_mode", Label: "Checkout Mode", LabelI18n: localize("Checkout Mode", "Mode Checkout"), Type: "string"},
				{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "held"},
				{Key: "reference", Label: "Reference", LabelI18n: localize("Reference", "Referensi"), Type: "string"},
				{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Type: "string"},
				{Key: "subtotal_amount", Label: "Subtotal", LabelI18n: localize("Subtotal", "Subtotal"), Type: "number"},
				{Key: "tax_amount", Label: "Tax", LabelI18n: localize("Tax", "Pajak"), Type: "number"},
				{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Type: "number"},
				{Key: "tendered_amount", Label: "Tendered", LabelI18n: localize("Tendered", "Dibayar"), Type: "number"},
				{Key: "change_due_amount", Label: "Change Due", LabelI18n: localize("Change Due", "Kembalian"), Type: "number"},
				{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Type: "string"},
				{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Type: "string"},
				{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Type: "string"},
				{Key: "promotion_codes_json", Label: "Promotion Codes JSON", LabelI18n: localize("Promotion Codes JSON", "JSON Kode Promosi"), Type: "string"},
				{Key: "lines_json", Label: "Lines JSON", LabelI18n: localize("Lines JSON", "JSON Baris"), Type: "string"},
				{Key: "tenders_json", Label: "Tenders JSON", LabelI18n: localize("Tenders JSON", "JSON Tender"), Type: "string"},
				{Key: "source_document_type", Label: "Source Type", LabelI18n: localize("Source Type", "Tipe Sumber"), Type: "string"},
				{Key: "source_document_id", Label: "Source Document", LabelI18n: localize("Source Document", "Dokumen Sumber"), Type: "string"},
				{Key: "order_id", Label: "Order", LabelI18n: localize("Order", "Order"), Type: "string"},
				{Key: "order_number", Label: "Order Number", LabelI18n: localize("Order Number", "Nomor Order"), Type: "string"},
				{Key: "invoice_id", Label: "Invoice", LabelI18n: localize("Invoice", "Invoice"), Type: "string"},
				{Key: "invoice_number", Label: "Invoice Number", LabelI18n: localize("Invoice Number", "Nomor Invoice"), Type: "string"},
				{Key: "fulfillment_id", Label: "Fulfillment", LabelI18n: localize("Fulfillment", "Fulfillment"), Type: "string"},
				{Key: "fulfillment_number", Label: "Fulfillment Number", LabelI18n: localize("Fulfillment Number", "Nomor Fulfillment"), Type: "string"},
				{Key: "payment_ids_json", Label: "Payment IDs JSON", LabelI18n: localize("Payment IDs JSON", "JSON ID Pembayaran"), Type: "string"},
				{Key: "device_id", Label: "Device", LabelI18n: localize("Device", "Perangkat"), Type: "string"},
				{Key: "offline_cached", Label: "Offline Cached", LabelI18n: localize("Offline Cached", "Cache Offline"), Type: "bool"},
				{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Type: "string"},
			}),
		},
		Security: module.SecurityDefinition{
			Permissions: append(
				append(
					append(
						append(
							commercialModelPermissions("pos_store", "POS Store"),
							commercialModelPermissions("pos_register", "POS Register")...,
						),
						commercialModelPermissions("pos_tender_type", "POS Tender Type")...,
					),
					commercialModelPermissions("pos_shift", "POS Shift")...,
				),
				commercialModelPermissions("pos_sale", "POS Sale")...,
			),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:           "pos_cashier",
					Name:          "POS Cashier",
					NameI18n:      localize("POS Cashier", "Kasir POS"),
					AllowedScopes: []string{"location"},
					PermissionKeys: []string{
						"pos_store.list", "pos_store.read",
						"pos_register.list", "pos_register.read",
						"pos_tender_type.list", "pos_tender_type.read",
						"pos_shift.create", "pos_shift.list", "pos_shift.read", "pos_shift.update",
						"pos_sale.create", "pos_sale.list", "pos_sale.read", "pos_sale.update",
						"item.list", "item.read",
						"party.list", "party.read",
						"payment_method.list", "payment_method.read",
						"document.create", "document.read", "document.list", "document.submit", "document.approve",
					},
				},
				{
					Key:           "pos_manager",
					Name:          "POS Manager",
					NameI18n:      localize("POS Manager", "Manajer POS"),
					AllowedScopes: []string{"location", "deployment"},
					PermissionKeys: []string{
						"pos_store.create", "pos_store.list", "pos_store.read", "pos_store.update",
						"pos_register.create", "pos_register.list", "pos_register.read", "pos_register.update",
						"pos_tender_type.create", "pos_tender_type.list", "pos_tender_type.read", "pos_tender_type.update",
						"pos_shift.create", "pos_shift.list", "pos_shift.read", "pos_shift.update",
						"pos_sale.create", "pos_sale.list", "pos_sale.read", "pos_sale.update",
						"item.list", "item.read",
						"party.list", "party.read",
						"payment_method.list", "payment_method.read",
						"document.create", "document.read", "document.list", "document.submit", "document.approve",
					},
				},
			},
		},
		SearchIndexes: []search.IndexDefinition{
			commercialModelSearchIndex("pos.stores.search", "POS Store Search", "pos_store", "pos.stores.list", []string{"code", "name", "warehouse_code", "status"}),
			commercialModelSearchIndex("pos.registers.search", "POS Register Search", "pos_register", "pos.registers.list", []string{"code", "name", "store_code", "status"}),
			commercialModelSearchIndex("pos.tender_types.search", "POS Tender Type Search", "pos_tender_type", "pos.tender_types.list", []string{"code", "name", "kind", "status"}),
			commercialModelSearchIndex("pos.shifts.search", "POS Shift Search", "pos_shift", "pos.shifts.list", []string{"shift_number", "register_code", "cashier_user_id", "cashier_employee_id", "status"}),
			commercialModelSearchIndex("pos.sales.search", "POS Sale Search", "pos_sale", "pos.sales.list", []string{"sale_number", "party_name", "invoice_number", "status"}),
		},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "pos.terminal", Label: "POS Terminal", LabelI18n: localize("POS Terminal", "Terminal POS"), ActionKey: "pos.terminal", Order: 80, Surface: module.UISurfacePOS, RequiredPermissions: []string{"pos_sale.create"}},
				{Key: "pos.stores", Label: "POS Stores", LabelI18n: localize("POS Stores", "Toko POS"), ActionKey: "pos.stores.list", Order: 81, RequiredPermissions: []string{"pos_store.list"}},
				{Key: "pos.registers", Label: "POS Registers", LabelI18n: localize("POS Registers", "Register POS"), ActionKey: "pos.registers.list", Order: 82, RequiredPermissions: []string{"pos_register.list"}},
				{Key: "pos.tender_types", Label: "POS Tender Types", LabelI18n: localize("POS Tender Types", "Jenis Tender POS"), ActionKey: "pos.tender_types.list", Order: 83, RequiredPermissions: []string{"pos_tender_type.list"}},
				{Key: "pos.shifts", Label: "POS Shifts", LabelI18n: localize("POS Shifts", "Shift POS"), ActionKey: "pos.shifts.list", Order: 84, RequiredPermissions: []string{"pos_shift.list"}},
				{Key: "pos.sales", Label: "POS Sales", LabelI18n: localize("POS Sales", "Penjualan POS"), ActionKey: "pos.sales.list", Order: 85, RequiredPermissions: []string{"pos_sale.list"}},
			},
			Actions: append([]module.ActionDefinition{
				{
					Key:                 "pos.terminal",
					Label:               "POS Terminal",
					LabelI18n:           localize("POS Terminal", "Terminal POS"),
					Kind:                "navigate",
					RoutePath:           "/pos/terminal",
					CustomEntryKey:      "pos.terminal",
					RenderMode:          module.RenderModeCustom,
					Surface:             module.UISurfacePOS,
					RequiredPermissions: []string{"pos_sale.create"},
				},
			}, posModelActions()...),
			Views: []module.ViewDefinition{
				posStoreListView(),
				posStoreDetailView(),
				posStoreFormView(),
				posRegisterListView(),
				posRegisterDetailView(),
				posRegisterFormView(),
				posTenderTypeListView(),
				posTenderTypeDetailView(),
				posTenderTypeFormView(),
				posShiftListView(),
				posShiftDetailView(),
				posShiftFormView(),
				posSaleListView(),
				posSaleDetailView(),
				posSaleFormView(),
			},
			CustomEntries: []module.CustomEntryDefinition{{
				Key:                 "pos.terminal",
				Title:               "",
				TitleI18n:           localize("", ""),
				RoutePath:           "/pos/terminal",
				BundleKey:           "pos-terminal",
				ComponentExport:     "render",
				Surface:             module.UISurfacePOS,
				RequiredPermissions: []string{"pos_sale.create"},
			}},
		},
		Bundles: []module.BundleDefinition{{
			Key:    "pos-terminal",
			Script: httpx.POSTerminalBundle(),
		}},
	}
}

func posModelDefinition(key, singular, permissionPrefix string, fields []model.FieldDefinition) model.Definition {
	return model.Definition{
		Key:                 key,
		DisplayName:         singular,
		DisplayNameI18n:     localize(singular, singular),
		OwnerModuleKey:      "pos_core",
		Version:             "v1",
		CreatePermissionKey: permissionPrefix + ".create",
		ListPermissionKey:   permissionPrefix + ".list",
		ReadPermissionKey:   permissionPrefix + ".read",
		UpdatePermissionKey: permissionPrefix + ".update",
		DefaultSort:         fields[0].Key,
		Fields:              fields,
	}
}

func posModelActions() []module.ActionDefinition {
	return []module.ActionDefinition{
		{Key: "pos.stores.list", Label: "POS Stores", LabelI18n: localize("POS Stores", "Toko POS"), Kind: "navigate", RoutePath: "/pos/stores", ViewKey: "pos.stores.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_store.list"}},
		{Key: "pos.stores.detail", Label: "POS Store Detail", LabelI18n: localize("POS Store Detail", "Detail Toko POS"), Kind: "navigate", RoutePath: "/pos/stores/detail", ViewKey: "pos.stores.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_store.read"}},
		{Key: "pos.stores.form", Label: "POS Store Form", LabelI18n: localize("POS Store Form", "Form Toko POS"), Kind: "navigate", RoutePath: "/pos/stores/form", ViewKey: "pos.stores.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_store.update"}},
		{Key: "pos.registers.list", Label: "POS Registers", LabelI18n: localize("POS Registers", "Register POS"), Kind: "navigate", RoutePath: "/pos/registers", ViewKey: "pos.registers.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_register.list"}},
		{Key: "pos.registers.detail", Label: "POS Register Detail", LabelI18n: localize("POS Register Detail", "Detail Register POS"), Kind: "navigate", RoutePath: "/pos/registers/detail", ViewKey: "pos.registers.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_register.read"}},
		{Key: "pos.registers.form", Label: "POS Register Form", LabelI18n: localize("POS Register Form", "Form Register POS"), Kind: "navigate", RoutePath: "/pos/registers/form", ViewKey: "pos.registers.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_register.update"}},
		{Key: "pos.tender_types.list", Label: "POS Tender Types", LabelI18n: localize("POS Tender Types", "Jenis Tender POS"), Kind: "navigate", RoutePath: "/pos/tender-types", ViewKey: "pos.tender_types.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_tender_type.list"}},
		{Key: "pos.tender_types.detail", Label: "POS Tender Type Detail", LabelI18n: localize("POS Tender Type Detail", "Detail Jenis Tender POS"), Kind: "navigate", RoutePath: "/pos/tender-types/detail", ViewKey: "pos.tender_types.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_tender_type.read"}},
		{Key: "pos.tender_types.form", Label: "POS Tender Type Form", LabelI18n: localize("POS Tender Type Form", "Form Jenis Tender POS"), Kind: "navigate", RoutePath: "/pos/tender-types/form", ViewKey: "pos.tender_types.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_tender_type.update"}},
		{Key: "pos.shifts.list", Label: "POS Shifts", LabelI18n: localize("POS Shifts", "Shift POS"), Kind: "navigate", RoutePath: "/pos/shifts", ViewKey: "pos.shifts.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_shift.list"}},
		{Key: "pos.shifts.detail", Label: "POS Shift Detail", LabelI18n: localize("POS Shift Detail", "Detail Shift POS"), Kind: "navigate", RoutePath: "/pos/shifts/detail", ViewKey: "pos.shifts.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_shift.read"}},
		{Key: "pos.shifts.form", Label: "POS Shift Form", LabelI18n: localize("POS Shift Form", "Form Shift POS"), Kind: "navigate", RoutePath: "/pos/shifts/form", ViewKey: "pos.shifts.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_shift.update"}},
		{Key: "pos.sales.list", Label: "POS Sales", LabelI18n: localize("POS Sales", "Penjualan POS"), Kind: "navigate", RoutePath: "/pos/sales", ViewKey: "pos.sales.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_sale.list"}},
		{Key: "pos.sales.detail", Label: "POS Sale Detail", LabelI18n: localize("POS Sale Detail", "Detail Penjualan POS"), Kind: "navigate", RoutePath: "/pos/sales/detail", ViewKey: "pos.sales.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_sale.read"}},
		{Key: "pos.sales.form", Label: "POS Sale Form", LabelI18n: localize("POS Sale Form", "Form Penjualan POS"), Kind: "navigate", RoutePath: "/pos/sales/form", ViewKey: "pos.sales.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"pos_sale.update"}},
	}
}

func posStoreListView() module.ViewDefinition {
	return commercialModelListView("pos.stores.list", "POS Stores", "pos_store", []module.ColumnDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code"},
		{Key: "checkout_mode", Label: "Checkout", LabelI18n: localize("Checkout", "Checkout"), Path: "values.checkout_mode"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []string{"active", "inactive"})
}

func posStoreDetailView() module.ViewDefinition {
	return commercialModelDetailView("pos.stores.detail", "POS Store Detail", "pos_store", []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code", Type: "string"},
		{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Path: "values.price_list_code", Type: "string"},
		{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "values.tax_profile_code", Type: "string"},
		{Key: "checkout_mode", Label: "Checkout Mode", LabelI18n: localize("Checkout Mode", "Mode Checkout"), Path: "values.checkout_mode", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
	})
}

func posStoreFormView() module.ViewDefinition {
	return commercialModelFormView("pos.stores.form", "POS Store Form", "pos_store", []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
		{Key: "location_id", Label: "Location", LabelI18n: localize("Location", "Lokasi"), Path: "values.location_id", Type: "string", Widget: "text"},
		{Key: "warehouse_code", Label: "Warehouse", LabelI18n: localize("Warehouse", "Gudang"), Path: "values.warehouse_code", Type: "string", Widget: "select", Required: true},
		{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Path: "values.price_list_code", Type: "string", Widget: "select"},
		{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "values.tax_profile_code", Type: "string", Widget: "select"},
		{Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "values.default_tax_code", Type: "string", Widget: "select"},
		{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string", Widget: "text"},
		{Key: "checkout_mode", Label: "Checkout Mode", LabelI18n: localize("Checkout Mode", "Mode Checkout"), Path: "values.checkout_mode", Type: "string", Widget: "select", Options: []string{"invoice_first", "sales_order_first"}},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
	})
}

func posRegisterListView() module.ViewDefinition {
	return commercialModelListView("pos.registers.list", "POS Registers", "pos_register", []module.ColumnDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code"},
		{Key: "checkout_mode", Label: "Checkout", LabelI18n: localize("Checkout", "Checkout"), Path: "values.checkout_mode"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []string{"active", "inactive"})
}

func posRegisterDetailView() module.ViewDefinition {
	return commercialModelDetailView("pos.registers.detail", "POS Register Detail", "pos_register", []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code", Type: "string"},
		{Key: "checkout_mode", Label: "Checkout Mode", LabelI18n: localize("Checkout Mode", "Mode Checkout"), Path: "values.checkout_mode", Type: "string"},
		{Key: "cash_account_code", Label: "Cash Account", LabelI18n: localize("Cash Account", "Akun Kas"), Path: "values.cash_account_code", Type: "string"},
		{Key: "card_account_code", Label: "Card Account", LabelI18n: localize("Card Account", "Akun Kartu"), Path: "values.card_account_code", Type: "string"},
		{Key: "hardware_profile", Label: "Hardware Profile", LabelI18n: localize("Hardware Profile", "Profil Perangkat"), Path: "values.hardware_profile", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
	})
}

func posRegisterFormView() module.ViewDefinition {
	return commercialModelFormView("pos.registers.form", "POS Register Form", "pos_register", []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code", Type: "string", Widget: "select", Required: true},
		{Key: "checkout_mode", Label: "Checkout Mode", LabelI18n: localize("Checkout Mode", "Mode Checkout"), Path: "values.checkout_mode", Type: "string", Widget: "select", Options: []string{"invoice_first", "sales_order_first"}},
		{Key: "cash_account_code", Label: "Cash Account", LabelI18n: localize("Cash Account", "Akun Kas"), Path: "values.cash_account_code", Type: "string", Widget: "text"},
		{Key: "card_account_code", Label: "Card Account", LabelI18n: localize("Card Account", "Akun Kartu"), Path: "values.card_account_code", Type: "string", Widget: "text"},
		{Key: "hardware_profile", Label: "Hardware Profile", LabelI18n: localize("Hardware Profile", "Profil Perangkat"), Path: "values.hardware_profile", Type: "string", Widget: "text"},
		{Key: "receipt_template_key", Label: "Receipt Template", LabelI18n: localize("Receipt Template", "Template Struk"), Path: "values.receipt_template_key", Type: "string", Widget: "text"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
	})
}

func posTenderTypeListView() module.ViewDefinition {
	return commercialModelListView("pos.tender_types.list", "POS Tender Types", "pos_tender_type", []module.ColumnDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
		{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind"},
		{Key: "payment_method_code", Label: "Payment Method", LabelI18n: localize("Payment Method", "Metode Pembayaran"), Path: "values.payment_method_code"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []string{"active", "inactive"})
}

func posTenderTypeDetailView() module.ViewDefinition {
	return commercialModelDetailView("pos.tender_types.detail", "POS Tender Type Detail", "pos_tender_type", []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
		{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind", Type: "string"},
		{Key: "payment_method_code", Label: "Payment Method", LabelI18n: localize("Payment Method", "Metode Pembayaran"), Path: "values.payment_method_code", Type: "string"},
		{Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Path: "values.clearing_account_code", Type: "string"},
		{Key: "requires_reference", Label: "Requires Reference", LabelI18n: localize("Requires Reference", "Butuh Referensi"), Path: "values.requires_reference", Type: "bool"},
		{Key: "is_cash_like", Label: "Cash Like", LabelI18n: localize("Cash Like", "Seperti Tunai"), Path: "values.is_cash_like", Type: "bool"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
	})
}

func posTenderTypeFormView() module.ViewDefinition {
	return commercialModelFormView("pos.tender_types.form", "POS Tender Type Form", "pos_tender_type", []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
		{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind", Type: "string", Widget: "select", Options: []string{"cash", "card", "bank_transfer", "voucher", "gift_card", "store_credit", "other"}, Required: true},
		{Key: "payment_method_code", Label: "Payment Method", LabelI18n: localize("Payment Method", "Metode Pembayaran"), Path: "values.payment_method_code", Type: "string", Widget: "select", Required: true},
		{Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Path: "values.clearing_account_code", Type: "string", Widget: "text"},
		{Key: "requires_reference", Label: "Requires Reference", LabelI18n: localize("Requires Reference", "Butuh Referensi"), Path: "values.requires_reference", Type: "bool"},
		{Key: "is_cash_like", Label: "Cash Like", LabelI18n: localize("Cash Like", "Seperti Tunai"), Path: "values.is_cash_like", Type: "bool"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
	})
}

func posShiftListView() module.ViewDefinition {
	return commercialModelListView("pos.shifts.list", "POS Shifts", "pos_shift", []module.ColumnDefinition{
		{Key: "shift_number", Label: "Shift", LabelI18n: localize("Shift", "Shift"), Path: "values.shift_number"},
		{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Path: "values.register_code"},
		{Key: "cashier_user_id", Label: "Cashier", LabelI18n: localize("Cashier", "Kasir"), Path: "values.cashier_user_id"},
		{Key: "cashier_employee_id", Label: "Employee", LabelI18n: localize("Employee", "Karyawan"), Path: "values.cashier_employee_id"},
		{Key: "roster_slot_id", Label: "Roster Slot", LabelI18n: localize("Roster Slot", "Slot Roster"), Path: "values.roster_slot_id"},
		{Key: "opened_at", Label: "Opened", LabelI18n: localize("Opened", "Dibuka"), Path: "values.opened_at"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []string{"draft", "opened", "closed", "cancelled"})
}

func posShiftDetailView() module.ViewDefinition {
	return commercialModelDetailView("pos.shifts.detail", "POS Shift Detail", "pos_shift", []module.FieldDefinition{
		{Key: "shift_number", Label: "Shift Number", LabelI18n: localize("Shift Number", "Nomor Shift"), Path: "values.shift_number", Type: "string"},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code", Type: "string"},
		{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Path: "values.register_code", Type: "string"},
		{Key: "cashier_user_id", Label: "Cashier", LabelI18n: localize("Cashier", "Kasir"), Path: "values.cashier_user_id", Type: "string"},
		{Key: "cashier_employee_id", Label: "Cashier Employee", LabelI18n: localize("Cashier Employee", "Karyawan Kasir"), Path: "values.cashier_employee_id", Type: "string"},
		{Key: "roster_slot_id", Label: "Roster Slot", LabelI18n: localize("Roster Slot", "Slot Roster"), Path: "values.roster_slot_id", Type: "string"},
		{Key: "attendance_day_id", Label: "Attendance Day", LabelI18n: localize("Attendance Day", "Hari Kehadiran"), Path: "values.attendance_day_id", Type: "string"},
		{Key: "opened_at", Label: "Opened At", LabelI18n: localize("Opened At", "Dibuka Pada"), Path: "values.opened_at", Type: "string"},
		{Key: "closed_at", Label: "Closed At", LabelI18n: localize("Closed At", "Ditutup Pada"), Path: "values.closed_at", Type: "string"},
		{Key: "opening_cash_amount", Label: "Opening Cash", LabelI18n: localize("Opening Cash", "Kas Awal"), Path: "values.opening_cash_amount", Type: "number"},
		{Key: "expected_cash_amount", Label: "Expected Cash", LabelI18n: localize("Expected Cash", "Kas Diharapkan"), Path: "values.expected_cash_amount", Type: "number"},
		{Key: "actual_cash_amount", Label: "Actual Cash", LabelI18n: localize("Actual Cash", "Kas Aktual"), Path: "values.actual_cash_amount", Type: "number"},
		{Key: "over_short_amount", Label: "Over / Short", LabelI18n: localize("Over / Short", "Lebih / Kurang"), Path: "values.over_short_amount", Type: "number"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string"},
	})
}

func posShiftFormView() module.ViewDefinition {
	return commercialModelFormView("pos.shifts.form", "POS Shift Form", "pos_shift", []module.FieldDefinition{
		{Key: "shift_number", Label: "Shift Number", LabelI18n: localize("Shift Number", "Nomor Shift"), Path: "values.shift_number", Type: "string", Widget: "text", Required: true},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code", Type: "string", Widget: "select", Required: true},
		{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Path: "values.register_code", Type: "string", Widget: "select", Required: true},
		{Key: "cashier_user_id", Label: "Cashier", LabelI18n: localize("Cashier", "Kasir"), Path: "values.cashier_user_id", Type: "string", Widget: "text", Required: true},
		{Key: "cashier_employee_id", Label: "Cashier Employee", LabelI18n: localize("Cashier Employee", "Karyawan Kasir"), Path: "values.cashier_employee_id", Type: "string", Widget: "select"},
		{Key: "roster_slot_id", Label: "Roster Slot", LabelI18n: localize("Roster Slot", "Slot Roster"), Path: "values.roster_slot_id", Type: "string", Widget: "select"},
		{Key: "attendance_day_id", Label: "Attendance Day", LabelI18n: localize("Attendance Day", "Hari Kehadiran"), Path: "values.attendance_day_id", Type: "string", Widget: "select"},
		{Key: "opening_cash_amount", Label: "Opening Cash", LabelI18n: localize("Opening Cash", "Kas Awal"), Path: "values.opening_cash_amount", Type: "number"},
		{Key: "expected_cash_amount", Label: "Expected Cash", LabelI18n: localize("Expected Cash", "Kas Diharapkan"), Path: "values.expected_cash_amount", Type: "number"},
		{Key: "actual_cash_amount", Label: "Actual Cash", LabelI18n: localize("Actual Cash", "Kas Aktual"), Path: "values.actual_cash_amount", Type: "number"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"draft", "opened", "closed", "cancelled"}},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string", Widget: "textarea"},
	})
}

func posSaleListView() module.ViewDefinition {
	return commercialModelListView("pos.sales.list", "POS Sales", "pos_sale", []module.ColumnDefinition{
		{Key: "sale_number", Label: "Sale", LabelI18n: localize("Sale", "Penjualan"), Path: "values.sale_number"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "values.party_name"},
		{Key: "invoice_number", Label: "Invoice", LabelI18n: localize("Invoice", "Invoice"), Path: "values.invoice_number"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "values.total_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []string{"held", "completed", "voided"})
}

func posSaleDetailView() module.ViewDefinition {
	return commercialModelDetailView("pos.sales.detail", "POS Sale Detail", "pos_sale", []module.FieldDefinition{
		{Key: "sale_number", Label: "Sale Number", LabelI18n: localize("Sale Number", "Nomor Penjualan"), Path: "values.sale_number", Type: "string"},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code", Type: "string"},
		{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Path: "values.register_code", Type: "string"},
		{Key: "shift_id", Label: "Shift", LabelI18n: localize("Shift", "Shift"), Path: "values.shift_id", Type: "string"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "values.party_name", Type: "string"},
		{Key: "checkout_mode", Label: "Checkout Mode", LabelI18n: localize("Checkout Mode", "Mode Checkout"), Path: "values.checkout_mode", Type: "string"},
		{Key: "promotion_codes_json", Label: "Promotion Codes", LabelI18n: localize("Promotion Codes", "Kode Promosi"), Path: "values.promotion_codes_json", Type: "string"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "values.total_amount", Type: "number"},
		{Key: "tendered_amount", Label: "Tendered", LabelI18n: localize("Tendered", "Dibayar"), Path: "values.tendered_amount", Type: "number"},
		{Key: "change_due_amount", Label: "Change Due", LabelI18n: localize("Change Due", "Kembalian"), Path: "values.change_due_amount", Type: "number"},
		{Key: "invoice_number", Label: "Invoice", LabelI18n: localize("Invoice", "Invoice"), Path: "values.invoice_number", Type: "string"},
		{Key: "fulfillment_number", Label: "Fulfillment", LabelI18n: localize("Fulfillment", "Fulfillment"), Path: "values.fulfillment_number", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string"},
	})
}

func posSaleFormView() module.ViewDefinition {
	return commercialModelFormView("pos.sales.form", "POS Sale Form", "pos_sale", []module.FieldDefinition{
		{Key: "sale_number", Label: "Sale Number", LabelI18n: localize("Sale Number", "Nomor Penjualan"), Path: "values.sale_number", Type: "string", Widget: "text", Required: true},
		{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "values.store_code", Type: "string", Widget: "select", Required: true},
		{Key: "register_code", Label: "Register", LabelI18n: localize("Register", "Register"), Path: "values.register_code", Type: "string", Widget: "select", Required: true},
		{Key: "shift_id", Label: "Shift", LabelI18n: localize("Shift", "Shift"), Path: "values.shift_id", Type: "string", Widget: "text", Required: true},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "values.party_name", Type: "string", Widget: "text"},
		{Key: "checkout_mode", Label: "Checkout Mode", LabelI18n: localize("Checkout Mode", "Mode Checkout"), Path: "values.checkout_mode", Type: "string", Widget: "select", Options: []string{"invoice_first", "sales_order_first"}},
		{Key: "promotion_codes_json", Label: "Promotion Codes", LabelI18n: localize("Promotion Codes", "Kode Promosi"), Path: "values.promotion_codes_json", Type: "string", Widget: "textarea"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"held", "completed", "voided"}},
		{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "values.notes", Type: "string", Widget: "textarea"},
	})
}
