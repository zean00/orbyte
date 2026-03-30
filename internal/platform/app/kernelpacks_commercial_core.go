package app

import (
	"fmt"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func commercialCoreKernelPackManifest() module.Manifest {
	return module.Manifest{
		Key:          "commercial_core",
		Name:         "Commercial Core",
		NameI18n:     localize("Commercial Core", "Inti Komersial"),
		Version:      "1.0.0",
		DomainFamily: "business",
		OwnedDocumentTypes: []string{
			"sales_order",
			"invoice",
			"credit_note",
			"payment_receipt",
			"payment_refund",
			"ledger_posting",
		},
		OwnedWorkflowKeys: []string{
			"sales_order_flow",
			"invoice_flow",
			"credit_note_flow",
			"payment_receipt_flow",
			"payment_refund_flow",
			"ledger_posting_flow",
		},
		OwnedTemplateKeys: []string{
			"commercial.sales_order.print.default",
			"commercial.invoice.print.default",
			"commercial.credit_note.print.default",
			"commercial.payment_receipt.print.default",
			"commercial.payment_refund.print.default",
			"commercial.ledger_posting.print.default",
		},
		DependencyRequirements: []module.DependencyRequirement{
			{ModuleKey: "platform.core", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "masterdata", VersionRange: ">=1.0.0,<2.0.0", Kind: module.DependencyKindRequired},
			{ModuleKey: "documents", VersionRange: ">=1.1.0,<2.0.0", Kind: module.DependencyKindRequired},
		},
		AdminConsole: module.AdminConsoleDefinition{
			Title:       "Commercial Console",
			TitleI18n:   localize("Commercial Console", "Konsol Komersial"),
			Description: "Commercial setup, posting defaults, and shortcuts to catalog, operations, workflows, and templates.",
			DescriptionI18n: localize(
				"Commercial setup, posting defaults, and shortcuts to catalog, operations, workflows, and templates.",
				"Pengaturan komersial, default posting, dan pintasan ke katalog, operasi, workflow, dan template.",
			),
			Sections: []module.AdminConsoleSectionDefinition{
				{
					Key:         "posting_defaults",
					Kind:        module.AdminConsoleSectionSettingsForm,
					Title:       "Posting Defaults",
					TitleI18n:   localize("Posting Defaults", "Default Posting"),
					Description: "Default receivable, revenue, tax, and clearing accounts used by commercial postings.",
					DescriptionI18n: localize(
						"Default receivable, revenue, tax, and clearing accounts used by commercial postings.",
						"Default akun piutang, pendapatan, pajak, dan clearing yang dipakai oleh posting komersial.",
					),
					ConfigKey:           "commercial.posting",
					RequiredPermissions: []string{"configuration.read"},
				},
				{
					Key:       "catalog_setup",
					Kind:      module.AdminConsoleSectionResourceLinks,
					Title:     "Catalog Setup",
					TitleI18n: localize("Catalog Setup", "Setup Katalog"),
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("catalog", "Catalog", "Katalog", "/ui/commercial/catalog", "Open the primary sellable catalog.", "Buka katalog jual utama.", "item.list"),
						adminConsoleLink("products", "Products", "Produk", "/ui/commercial/products", "Manage parent products and their variant setup.", "Kelola produk induk dan pengaturan variannya.", "product.list"),
						adminConsoleLink("items", "Items", "Item", "/ui/commercial/items", "Manage products and services.", "Kelola produk dan layanan.", "item.list"),
						adminConsoleLink("variant_dimensions", "Variant Dimensions", "Dimensi Varian", "/ui/commercial/variant-dimensions", "Maintain reusable variant dimensions such as color or size.", "Pelihara dimensi varian seperti warna atau ukuran.", "variant_dimension.list"),
						adminConsoleLink("variant_values", "Variant Values", "Nilai Varian", "/ui/commercial/variant-values", "Maintain allowed values for each variant dimension.", "Pelihara nilai yang diizinkan untuk setiap dimensi varian.", "variant_value.list"),
						adminConsoleLink("item_categories", "Item Categories", "Kategori Item", "/ui/commercial/item-categories", "Maintain commercial item groupings.", "Pelihara pengelompokan item komersial.", "item_category.list"),
						adminConsoleLink("uoms", "Units", "Satuan", "/ui/commercial/uoms", "Manage units of measure.", "Kelola satuan ukur.", "uom.list"),
						adminConsoleLink("tax_codes", "Tax Codes", "Kode Pajak", "/ui/commercial/tax-codes", "Manage tax calculation rules.", "Kelola aturan perhitungan pajak.", "tax_code.list"),
						adminConsoleLink("tax_profiles", "Tax Profiles", "Profil Pajak", "/ui/commercial/tax-profiles", "Manage commercial tax defaults.", "Kelola default pajak komersial.", "tax_profile.list"),
						adminConsoleLink("price_lists", "Price Lists", "Daftar Harga", "/ui/commercial/price-lists", "Manage commercial pricing matrices.", "Kelola matriks harga komersial.", "price_list.list"),
						adminConsoleLink("price_list_items", "Price List Items", "Item Daftar Harga", "/ui/commercial/price-list-items", "Maintain per-item pricing entries.", "Pelihara entri harga per item.", "price_list_item.list"),
						adminConsoleLink("accounts", "Accounts", "Akun", "/ui/commercial/accounts", "Manage posting accounts.", "Kelola akun posting.", "account.list"),
						adminConsoleLink("payment_methods", "Payment Methods", "Metode Pembayaran", "/ui/commercial/payment-methods", "Manage payment and clearing defaults.", "Kelola default pembayaran dan clearing.", "payment_method.list"),
					},
				},
				{
					Key:       "operations",
					Kind:      module.AdminConsoleSectionResourceLinks,
					Title:     "Commercial Operations",
					TitleI18n: localize("Commercial Operations", "Operasi Komersial"),
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("orders", "Orders", "Order", "/ui/commercial/orders", "Open sales orders.", "Buka order penjualan.", "document.list"),
						adminConsoleLink("fulfillments", "Fulfillments", "Fulfillment", "/ui/fulfillment/fulfillments", "Open sales fulfillments.", "Buka fulfillment penjualan.", "document.list"),
						adminConsoleLink("invoices", "Invoices", "Invoice", "/ui/commercial/invoices", "Open invoices.", "Buka invoice.", "document.list"),
						adminConsoleLink("credit_notes", "Credit Notes", "Credit Note", "/ui/commercial/credit-notes", "Open credit notes.", "Buka credit note.", "document.list"),
						adminConsoleLink("receivables", "Receivables", "Piutang", "/ui/commercial/receivables", "Open the receivables dashboard.", "Buka dashboard piutang.", "document.list"),
						adminConsoleLink("payments", "Payments", "Pembayaran", "/ui/commercial/payments", "Open payment receipts.", "Buka penerimaan pembayaran.", "document.list"),
						adminConsoleLink("refunds", "Refunds", "Refund", "/ui/commercial/refunds", "Open payment refunds.", "Buka refund pembayaran.", "document.list"),
						adminConsoleLink("ledger", "Ledger", "Buku Besar", "/ui/commercial/ledger", "Open generated ledger postings.", "Buka posting ledger yang dihasilkan.", "document.list"),
					},
				},
				{
					Key:       "workflow_links",
					Kind:      module.AdminConsoleSectionWorkflowLinks,
					Title:     "Commercial Workflows",
					TitleI18n: localize("Commercial Workflows", "Workflow Komersial"),
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("sales_order_flow", "Sales Order Workflow", "Workflow Order Penjualan", "/admin/workflows/designer?key=sales_order_flow", "Open the sales order workflow designer.", "Buka desainer workflow order penjualan.", "configuration.read"),
						adminConsoleLink("invoice_flow", "Invoice Workflow", "Workflow Invoice", "/admin/workflows/designer?key=invoice_flow", "Open the invoice workflow designer.", "Buka desainer workflow invoice.", "configuration.read"),
						adminConsoleLink("credit_note_flow", "Credit Note Workflow", "Workflow Credit Note", "/admin/workflows/designer?key=credit_note_flow", "Open the credit note workflow designer.", "Buka desainer workflow credit note.", "configuration.read"),
						adminConsoleLink("payment_receipt_flow", "Payment Receipt Workflow", "Workflow Penerimaan Pembayaran", "/admin/workflows/designer?key=payment_receipt_flow", "Open the payment receipt workflow designer.", "Buka desainer workflow penerimaan pembayaran.", "configuration.read"),
						adminConsoleLink("payment_refund_flow", "Payment Refund Workflow", "Workflow Refund Pembayaran", "/admin/workflows/designer?key=payment_refund_flow", "Open the payment refund workflow designer.", "Buka desainer workflow refund pembayaran.", "configuration.read"),
						adminConsoleLink("ledger_posting_flow", "Ledger Posting Workflow", "Workflow Posting Ledger", "/admin/workflows/designer?key=ledger_posting_flow", "Open the ledger posting workflow designer.", "Buka desainer workflow posting ledger.", "configuration.read"),
					},
				},
				{
					Key:       "template_links",
					Kind:      module.AdminConsoleSectionTemplateLinks,
					Title:     "Commercial Templates",
					TitleI18n: localize("Commercial Templates", "Template Komersial"),
					Links: []module.AdminConsoleLinkDefinition{
						adminConsoleLink("sales_order_template", "Sales Order Print", "Cetak Order Penjualan", "/admin/templates/designer?key=commercial.sales_order.print.default", "Open the sales order print template.", "Buka template cetak order penjualan.", "configuration.read"),
						adminConsoleLink("invoice_template", "Invoice Print", "Cetak Invoice", "/admin/templates/designer?key=commercial.invoice.print.default", "Open the invoice print template.", "Buka template cetak invoice.", "configuration.read"),
						adminConsoleLink("credit_note_template", "Credit Note Print", "Cetak Credit Note", "/admin/templates/designer?key=commercial.credit_note.print.default", "Open the credit note print template.", "Buka template cetak credit note.", "configuration.read"),
						adminConsoleLink("payment_receipt_template", "Payment Receipt Print", "Cetak Penerimaan Pembayaran", "/admin/templates/designer?key=commercial.payment_receipt.print.default", "Open the payment receipt print template.", "Buka template cetak penerimaan pembayaran.", "configuration.read"),
						adminConsoleLink("payment_refund_template", "Payment Refund Print", "Cetak Refund Pembayaran", "/admin/templates/designer?key=commercial.payment_refund.print.default", "Open the payment refund print template.", "Buka template cetak refund pembayaran.", "configuration.read"),
						adminConsoleLink("ledger_posting_template", "Ledger Posting Print", "Cetak Posting Ledger", "/admin/templates/designer?key=commercial.ledger_posting.print.default", "Open the ledger posting print template.", "Buka template cetak posting ledger.", "configuration.read"),
					},
				},
			},
		},
		Models: []model.Definition{
			commercialCatalogModelDefinition(
				"commercial_product",
				"Commercial Product",
				"Commercial Products",
				"product",
				[]model.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Type: "string"},
					{Key: "brand", Label: "Brand", LabelI18n: localize("Brand", "Merek"), Type: "string"},
					{Key: "item_type", Label: "Item Type", LabelI18n: localize("Item Type", "Tipe Item"), Type: "string", Required: true, DefaultValue: "product"},
					{Key: "category_code", Label: "Category", LabelI18n: localize("Category", "Kategori"), Type: "string"},
					{Key: "tags", Label: "Tags", LabelI18n: localize("Tags", "Tag"), Type: "string"},
					{Key: "variant_dimension_codes", Label: "Variant Dimensions", LabelI18n: localize("Variant Dimensions", "Dimensi Varian"), Type: "string"},
					{Key: "uom_code", Label: "UOM", LabelI18n: localize("UOM", "Satuan"), Type: "string"},
					{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Type: "string"},
					{Key: "tax_code", Label: "Tax Code", LabelI18n: localize("Tax Code", "Kode Pajak"), Type: "string"},
					{Key: "revenue_account_code", Label: "Revenue Account", LabelI18n: localize("Revenue Account", "Akun Pendapatan"), Type: "string"},
					{Key: "inventory_asset_account_code", Label: "Inventory Asset Account", LabelI18n: localize("Inventory Asset Account", "Akun Aset Inventori"), Type: "string"},
					{Key: "cogs_account_code", Label: "COGS Account", LabelI18n: localize("COGS Account", "Akun HPP"), Type: "string"},
					{Key: "wip_account_code", Label: "WIP Account", LabelI18n: localize("WIP Account", "Akun BDP"), Type: "string"},
					{Key: "inventory_enabled", Label: "Inventory Enabled", LabelI18n: localize("Inventory Enabled", "Inventori Aktif"), Type: "bool"},
					{Key: "inventory_tracking_mode", Label: "Inventory Tracking", LabelI18n: localize("Inventory Tracking", "Pelacakan Inventori"), Type: "string", DefaultValue: "none"},
					{Key: "expiry_tracking_enabled", Label: "Expiry Tracking", LabelI18n: localize("Expiry Tracking", "Pelacakan Kedaluwarsa"), Type: "bool"},
					{Key: "allow_negative_stock", Label: "Allow Negative Stock", LabelI18n: localize("Allow Negative Stock", "Izinkan Stok Negatif"), Type: "bool"},
					{Key: "default_issue_strategy", Label: "Issue Strategy", LabelI18n: localize("Issue Strategy", "Strategi Pengeluaran"), Type: "string", DefaultValue: "manual"},
					{Key: "replenishment_enabled", Label: "Replenishment Enabled", LabelI18n: localize("Replenishment Enabled", "Replenishment Aktif"), Type: "bool"},
					{Key: "replenishment_mode", Label: "Replenishment Mode", LabelI18n: localize("Replenishment Mode", "Mode Replenishment"), Type: "string", DefaultValue: "manual"},
					{Key: "reorder_point_quantity", Label: "Reorder Point", LabelI18n: localize("Reorder Point", "Titik Pemesanan Ulang"), Type: "number"},
					{Key: "target_stock_quantity", Label: "Target Stock", LabelI18n: localize("Target Stock", "Target Stok"), Type: "number"},
					{Key: "default_replenishment_warehouse_code", Label: "Default Replenishment Warehouse", LabelI18n: localize("Default Replenishment Warehouse", "Gudang Replenishment Default"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"commercial_variant_dimension",
				"Variant Dimension",
				"Variant Dimensions",
				"variant_dimension",
				[]model.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"commercial_variant_value",
				"Variant Value",
				"Variant Values",
				"variant_value",
				[]model.FieldDefinition{
					{Key: "dimension_code", Label: "Dimension", LabelI18n: localize("Dimension", "Dimensi"), Type: "string", Required: true},
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "sort_order", Label: "Sort Order", LabelI18n: localize("Sort Order", "Urutan"), Type: "number"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"commercial_item",
				"Commercial Item",
				"Commercial Items",
				"item",
				[]model.FieldDefinition{
					{Key: "sku", Label: "SKU", LabelI18n: localize("SKU", "SKU"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Type: "string"},
					{Key: "product_code", Label: "Product", LabelI18n: localize("Product", "Produk"), Type: "string"},
					{Key: "is_variant", Label: "Variant SKU", LabelI18n: localize("Variant SKU", "SKU Varian"), Type: "bool"},
					{Key: "variant_signature", Label: "Variant Signature", LabelI18n: localize("Variant Signature", "Signature Varian"), Type: "string"},
					{Key: "variant_label", Label: "Variant Label", LabelI18n: localize("Variant Label", "Label Varian"), Type: "string"},
					{Key: "variant_values", Label: "Variant Values", LabelI18n: localize("Variant Values", "Nilai Varian"), Type: "string"},
					{Key: "item_type", Label: "Item Type", LabelI18n: localize("Item Type", "Tipe Item"), Type: "string", Required: true, DefaultValue: "service"},
					{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Type: "string", Required: true},
					{Key: "category_code", Label: "Category", LabelI18n: localize("Category", "Kategori"), Type: "string"},
					{Key: "tags", Label: "Tags", LabelI18n: localize("Tags", "Tag"), Type: "string"},
					{Key: "is_sellable", Label: "Sellable", LabelI18n: localize("Sellable", "Dapat Dijual"), Type: "bool", DefaultValue: true},
					{Key: "uom_code", Label: "UOM", LabelI18n: localize("UOM", "Satuan"), Type: "string"},
					{Key: "base_price", Label: "Base Price", LabelI18n: localize("Base Price", "Harga Dasar"), Type: "number"},
					{Key: "unit_price", Label: "Legacy Unit Price", LabelI18n: localize("Legacy Unit Price", "Harga Satuan Lama"), Type: "number"},
					{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Type: "string"},
					{Key: "tax_code", Label: "Tax Code", LabelI18n: localize("Tax Code", "Kode Pajak"), Type: "string"},
					{Key: "revenue_account_code", Label: "Revenue Account", LabelI18n: localize("Revenue Account", "Akun Pendapatan"), Type: "string"},
					{Key: "inventory_asset_account_code", Label: "Inventory Asset Account", LabelI18n: localize("Inventory Asset Account", "Akun Aset Inventori"), Type: "string"},
					{Key: "cogs_account_code", Label: "COGS Account", LabelI18n: localize("COGS Account", "Akun HPP"), Type: "string"},
					{Key: "wip_account_code", Label: "WIP Account", LabelI18n: localize("WIP Account", "Akun BDP"), Type: "string"},
					{Key: "service_unit", Label: "Service Unit", LabelI18n: localize("Service Unit", "Unit Layanan"), Type: "string"},
					{Key: "standard_duration_minutes", Label: "Standard Duration Minutes", LabelI18n: localize("Standard Duration Minutes", "Durasi Standar Menit"), Type: "number"},
					{Key: "product_unit", Label: "Product Unit", LabelI18n: localize("Product Unit", "Unit Produk"), Type: "string"},
					{Key: "fulfillment_mode", Label: "Fulfillment Mode", LabelI18n: localize("Fulfillment Mode", "Mode Pemenuhan"), Type: "string"},
					{Key: "inventory_enabled", Label: "Inventory Enabled", LabelI18n: localize("Inventory Enabled", "Inventori Aktif"), Type: "bool"},
					{Key: "inventory_tracking_mode", Label: "Inventory Tracking", LabelI18n: localize("Inventory Tracking", "Pelacakan Inventori"), Type: "string", DefaultValue: "none"},
					{Key: "expiry_tracking_enabled", Label: "Expiry Tracking", LabelI18n: localize("Expiry Tracking", "Pelacakan Kedaluwarsa"), Type: "bool"},
					{Key: "allow_negative_stock", Label: "Allow Negative Stock", LabelI18n: localize("Allow Negative Stock", "Izinkan Stok Negatif"), Type: "bool"},
					{Key: "default_issue_strategy", Label: "Issue Strategy", LabelI18n: localize("Issue Strategy", "Strategi Pengeluaran"), Type: "string", DefaultValue: "manual"},
					{Key: "replenishment_enabled", Label: "Replenishment Enabled", LabelI18n: localize("Replenishment Enabled", "Replenishment Aktif"), Type: "bool"},
					{Key: "replenishment_mode", Label: "Replenishment Mode", LabelI18n: localize("Replenishment Mode", "Mode Replenishment"), Type: "string", DefaultValue: "manual"},
					{Key: "reorder_point_quantity", Label: "Reorder Point", LabelI18n: localize("Reorder Point", "Titik Pemesanan Ulang"), Type: "number"},
					{Key: "target_stock_quantity", Label: "Target Stock", LabelI18n: localize("Target Stock", "Target Stok"), Type: "number"},
					{Key: "default_replenishment_warehouse_code", Label: "Default Replenishment Warehouse", LabelI18n: localize("Default Replenishment Warehouse", "Gudang Replenishment Default"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"commercial_item_category",
				"Item Category",
				"Item Categories",
				"item_category",
				[]model.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"commercial_uom",
				"Unit of Measure",
				"Units of Measure",
				"uom",
				[]model.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "symbol", Label: "Symbol", LabelI18n: localize("Symbol", "Simbol"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"commercial_tax_code",
				"Tax Code",
				"Tax Codes",
				"tax_code",
				[]model.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "mode", Label: "Mode", LabelI18n: localize("Mode", "Mode"), Type: "string", DefaultValue: "exclusive"},
					{Key: "rate_percent", Label: "Rate Percent", LabelI18n: localize("Rate Percent", "Persen Tarif"), Type: "number"},
					{Key: "tax_account_code", Label: "Tax Account", LabelI18n: localize("Tax Account", "Akun Pajak"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"commercial_tax_profile",
				"Tax Profile",
				"Tax Profiles",
				"tax_profile",
				[]model.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Type: "string"},
					{Key: "payment_term_days", Label: "Payment Term Days", LabelI18n: localize("Payment Term Days", "Hari Termin Pembayaran"), Type: "number"},
					{Key: "price_tax_mode", Label: "Price Tax Mode", LabelI18n: localize("Price Tax Mode", "Mode Harga Pajak"), Type: "string", DefaultValue: "exclusive"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"commercial_price_list",
				"Price List",
				"Price Lists",
				"price_list",
				[]model.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Type: "string"},
					{Key: "effective_from", Label: "Effective From", LabelI18n: localize("Effective From", "Berlaku Mulai"), Type: "string"},
					{Key: "effective_to", Label: "Effective To", LabelI18n: localize("Effective To", "Berlaku Sampai"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"commercial_price_list_item",
				"Price List Item",
				"Price List Items",
				"price_list_item",
				[]model.FieldDefinition{
					{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Type: "string", Required: true},
					{Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Type: "string", Required: true},
					{Key: "unit_price", Label: "Unit Price", LabelI18n: localize("Unit Price", "Harga Satuan"), Type: "number", Required: true},
					{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Type: "string"},
					{Key: "tax_code", Label: "Tax Code", LabelI18n: localize("Tax Code", "Kode Pajak"), Type: "string"},
					{Key: "revenue_account_code", Label: "Revenue Account", LabelI18n: localize("Revenue Account", "Akun Pendapatan"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"commercial_account",
				"Commercial Account",
				"Commercial Accounts",
				"account",
				[]model.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "category", Label: "Category", LabelI18n: localize("Category", "Kategori"), Type: "string", Required: true},
					{Key: "normal_balance", Label: "Normal Balance", LabelI18n: localize("Normal Balance", "Saldo Normal"), Type: "string", DefaultValue: "debit"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
			commercialCatalogModelDefinition(
				"payment_method",
				"Payment Method",
				"Payment Methods",
				"payment_method",
				[]model.FieldDefinition{
					{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Type: "string", Required: true},
					{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Type: "string", Required: true},
					{Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Type: "string", Required: true},
					{Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Type: "string"},
					{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "string", DefaultValue: "active"},
				},
			),
		},
		Documents: []document.Definition{
			commercialDocumentDefinition("sales_order", "Sales Order", "sales_order_flow", "sales_order_number"),
			commercialDocumentDefinition("invoice", "Invoice", "invoice_flow", "invoice_number"),
			commercialDocumentDefinition("credit_note", "Credit Note", "credit_note_flow", "credit_note_number"),
			commercialDocumentDefinition("payment_receipt", "Payment Receipt", "payment_receipt_flow", "payment_receipt_number"),
			commercialDocumentDefinition("payment_refund", "Payment Refund", "payment_refund_flow", "payment_refund_number"),
			commercialDocumentDefinition("ledger_posting", "Ledger Posting", "ledger_posting_flow", "ledger_posting_number"),
		},
		Workflows: []workflow.Definition{
			commercialWorkflowDefinition("sales_order_flow", "confirmed", false),
			commercialWorkflowDefinition("invoice_flow", "issued", true),
			commercialWorkflowDefinition("credit_note_flow", "issued", false),
			commercialWorkflowDefinition("payment_receipt_flow", "received", true),
			commercialWorkflowDefinition("payment_refund_flow", "refunded", true),
			commercialWorkflowDefinition("ledger_posting_flow", "posted", false),
		},
		Datasets: []module.DatasetDefinition{
			{Key: "commercial.product.summary", Title: "Product Summary", TitleI18n: localize("Product Summary", "Ringkasan Produk"), SourceKind: "model", ModelKey: "commercial_product", Dimensions: []module.DatasetDimension{{Key: "by_category", Label: "By Category", LabelI18n: localize("By Category", "Berdasarkan Kategori"), Path: "category_code"}}, Measures: []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}}},
			{Key: "commercial.item.summary", Title: "Item Summary", TitleI18n: localize("Item Summary", "Ringkasan Item"), SourceKind: "model", ModelKey: "commercial_item", Dimensions: []module.DatasetDimension{{Key: "by_type", Label: "By Type", LabelI18n: localize("By Type", "Berdasarkan Tipe"), Path: "item_type"}, {Key: "by_category", Label: "By Category", LabelI18n: localize("By Category", "Berdasarkan Kategori"), Path: "category_code"}}, Measures: []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}}},
			{Key: "commercial.account.summary", Title: "Account Summary", TitleI18n: localize("Account Summary", "Ringkasan Akun"), SourceKind: "model", ModelKey: "commercial_account", Dimensions: []module.DatasetDimension{{Key: "by_category", Label: "By Category", LabelI18n: localize("By Category", "Berdasarkan Kategori"), Path: "category"}}, Measures: []module.DatasetMeasure{{Key: "total", Label: "Total", LabelI18n: localize("Total", "Total"), Kind: "count"}}},
		},
		SearchIndexes: append([]search.IndexDefinition{
			commercialModelSearchIndex("commercial.products.search", "Product Search", "commercial_product", "commercial.products.list", []string{"code", "name", "category_code", "status"}),
			commercialModelSearchIndex("commercial.variant_dimensions.search", "Variant Dimension Search", "commercial_variant_dimension", "commercial.variant_dimensions.list", []string{"code", "name", "status"}),
			commercialModelSearchIndex("commercial.variant_values.search", "Variant Value Search", "commercial_variant_value", "commercial.variant_values.list", []string{"dimension_code", "code", "name", "status"}),
			commercialModelSearchIndex("commercial.items.search", "Item Search", "commercial_item", "commercial.items.list", []string{"sku", "name", "item_type", "category_code", "status"}),
			commercialModelSearchIndex("commercial.item_categories.search", "Item Category Search", "commercial_item_category", "commercial.item_categories.list", []string{"code", "name", "status"}),
			commercialModelSearchIndex("commercial.uoms.search", "UOM Search", "commercial_uom", "commercial.uoms.list", []string{"code", "name", "symbol", "status"}),
			commercialModelSearchIndex("commercial.tax_codes.search", "Tax Code Search", "commercial_tax_code", "commercial.tax_codes.list", []string{"code", "name", "mode", "status"}),
			commercialModelSearchIndex("commercial.tax_profiles.search", "Tax Profile Search", "commercial_tax_profile", "commercial.tax_profiles.list", []string{"code", "name", "default_tax_code", "status"}),
			commercialModelSearchIndex("commercial.price_lists.search", "Price List Search", "commercial_price_list", "commercial.price_lists.list", []string{"code", "name", "currency_code", "status"}),
			commercialModelSearchIndex("commercial.price_list_items.search", "Price List Item Search", "commercial_price_list_item", "commercial.price_list_items.list", []string{"price_list_code", "item_code", "status"}),
			commercialModelSearchIndex("commercial.accounts.search", "Account Search", "commercial_account", "commercial.accounts.list", []string{"code", "name", "category", "status"}),
			commercialModelSearchIndex("commercial.payment_methods.search", "Payment Method Search", "payment_method", "commercial.payment_methods.list", []string{"code", "name", "kind", "status"}),
		}, commercialDocumentSearchIndexes()...),
		Security: module.SecurityDefinition{
			Permissions: append(
				append(
					append(
						append(
							append(
								append(
									append(
										append(
											append(
												append(
													append([]module.PermissionDefinition{},
														commercialModelPermissions("product", "Product")...,
													),
													commercialModelPermissions("variant_dimension", "Variant Dimension")...,
												),
												commercialModelPermissions("variant_value", "Variant Value")...,
											),
											commercialModelPermissions("item", "Item")...,
										),
										commercialModelPermissions("item_category", "Item Category")...,
									),
									commercialModelPermissions("uom", "Unit of Measure")...,
								),
								commercialModelPermissions("tax_code", "Tax Code")...,
							),
							commercialModelPermissions("tax_profile", "Tax Profile")...,
						),
						commercialModelPermissions("price_list", "Price List")...,
					),
					commercialModelPermissions("price_list_item", "Price List Item")...,
				),
				append(
					commercialModelPermissions("account", "Account"),
					commercialModelPermissions("payment_method", "Payment Method")...,
				)...,
			),
			RoleTemplates: []module.RoleTemplateDefinition{
				{
					Key:            "commercial_catalog_manager",
					Name:           "Commercial Catalog Manager",
					NameI18n:       localize("Commercial Catalog Manager", "Pengelola Katalog Komersial"),
					AllowedScopes:  []string{"deployment", "location"},
					PermissionKeys: []string{"product.create", "product.list", "product.read", "product.update", "variant_dimension.create", "variant_dimension.list", "variant_dimension.read", "variant_dimension.update", "variant_value.create", "variant_value.list", "variant_value.read", "variant_value.update", "item.create", "item.list", "item.read", "item.update", "item_category.create", "item_category.list", "item_category.read", "item_category.update", "uom.create", "uom.list", "uom.read", "uom.update", "tax_code.create", "tax_code.list", "tax_code.read", "tax_code.update", "tax_profile.create", "tax_profile.list", "tax_profile.read", "tax_profile.update", "price_list.create", "price_list.list", "price_list.read", "price_list.update", "price_list_item.create", "price_list_item.list", "price_list_item.read", "price_list_item.update", "account.create", "account.list", "account.read", "account.update", "payment_method.create", "payment_method.list", "payment_method.read", "payment_method.update"},
				},
			},
		},
		Frontend: module.FrontendDefinition{
			Menus: append(
				append(commercialModelMenus(),
					module.MenuDefinition{Key: "commercial.orders", Label: "Orders", LabelI18n: localize("Orders", "Pesanan"), ActionKey: "commercial.orders.list", Order: 30, RequiredPermissions: []string{"document.list"}},
					module.MenuDefinition{Key: "commercial.invoices", Label: "Invoices", LabelI18n: localize("Invoices", "Faktur"), ActionKey: "commercial.invoices.list", Order: 31, RequiredPermissions: []string{"document.list"}},
					module.MenuDefinition{Key: "commercial.credit_notes", Label: "Credit Notes", LabelI18n: localize("Credit Notes", "Nota Kredit"), ActionKey: "commercial.credit_notes.list", Order: 32, RequiredPermissions: []string{"document.list"}},
					module.MenuDefinition{Key: "commercial.receivables", Label: "Receivables", LabelI18n: localize("Receivables", "Piutang"), ActionKey: "commercial.receivables.dashboard", Order: 33, RequiredPermissions: []string{"document.list"}},
					module.MenuDefinition{Key: "commercial.payments", Label: "Payments", LabelI18n: localize("Payments", "Pembayaran"), ActionKey: "commercial.payments.list", Order: 34, RequiredPermissions: []string{"document.list"}},
					module.MenuDefinition{Key: "commercial.refunds", Label: "Refunds", LabelI18n: localize("Refunds", "Pengembalian Dana"), ActionKey: "commercial.refunds.list", Order: 35, RequiredPermissions: []string{"document.list"}},
					module.MenuDefinition{Key: "commercial.ledger", Label: "Ledger", LabelI18n: localize("Ledger", "Buku Besar"), ActionKey: "commercial.ledger.list", Order: 36, RequiredPermissions: []string{"document.list"}},
				),
			),
			Actions: append(
				append(commercialModelActions(),
					commercialDocumentActions("commercial.orders", "sales_order", "Orders", "Order", "New Order", "/commercial/orders")...,
				),
				append(
					append(
						commercialDocumentActions("commercial.invoices", "invoice", "Invoices", "Invoice", "New Invoice", "/commercial/invoices"),
						commercialDocumentActions("commercial.credit_notes", "credit_note", "Credit Notes", "Credit Note", "New Credit Note", "/commercial/credit-notes")...,
					),
					append(
						append(
							append([]module.ActionDefinition{{
								Key:                 "commercial.receivables.dashboard",
								Label:               "Receivables",
								LabelI18n:           localize("Receivables", "Piutang"),
								Kind:                "navigate",
								RoutePath:           "/commercial/receivables",
								ViewKey:             "commercial.receivables.dashboard",
								RenderMode:          module.RenderModeGeneric,
								RequiredPermissions: []string{"document.list"},
							}, {
								Key:                 "commercial.party_statement.dashboard",
								Label:               "Customer Statement",
								LabelI18n:           localize("Customer Statement", "Laporan Pelanggan"),
								Kind:                "navigate",
								RoutePath:           "/commercial/party-statement",
								ViewKey:             "commercial.party_statement.dashboard",
								RenderMode:          module.RenderModeGeneric,
								RequiredPermissions: []string{"party.read", "document.list"},
							}},
								commercialDocumentActions("commercial.payments", "payment_receipt", "Payments", "Payment Receipt", "New Payment", "/commercial/payments")...,
							),
							commercialDocumentActions("commercial.refunds", "payment_refund", "Refunds", "Payment Refund", "New Refund", "/commercial/refunds")...,
						),
						commercialDocumentActions("commercial.ledger", "ledger_posting", "Ledger Postings", "Ledger Posting", "New Posting", "/commercial/ledger")...,
					)...,
				)...,
			),
			Views: append(
				append(
					commercialModelViews(),
					commercialDocumentViews("commercial.orders", "sales_order", "Orders", "Order Detail", "Order Draft", salesOrderColumns(), []string{"draft", "submitted", "confirmed", "rejected", "cancelled"}, salesOrderSections(), salesOrderFormSections())...,
				),
				append(
					append(
						append(
							append(
								append([]module.ViewDefinition{},
									commercialDocumentViews("commercial.invoices", "invoice", "Invoices", "Invoice Detail", "Invoice Draft", invoiceColumns(), []string{"draft", "submitted", "issued", "partially_paid", "paid", "refunded", "rejected", "cancelled"}, invoiceSections(), invoiceFormSections())...,
								),
								commercialReceivablesDashboardView(),
								commercialPartyStatementDashboardView(),
							),
							commercialDocumentViews("commercial.credit_notes", "credit_note", "Credit Notes", "Credit Note Detail", "Credit Note Draft", creditNoteColumns(), []string{"draft", "submitted", "issued", "rejected", "cancelled"}, creditNoteSections(), creditNoteFormSections())...,
						),
						commercialDocumentViews("commercial.payments", "payment_receipt", "Payments", "Payment Detail", "Payment Draft", paymentColumns(), []string{"draft", "submitted", "received", "rejected", "cancelled"}, paymentSections(), paymentFormSections())...,
					),
					append(
						append([]module.ViewDefinition{},
							commercialDocumentViews("commercial.refunds", "payment_refund", "Refunds", "Refund Detail", "Refund Draft", refundColumns(), []string{"draft", "submitted", "refunded", "rejected", "cancelled"}, refundSections(), refundFormSections())...,
						),
						commercialDocumentViews("commercial.ledger", "ledger_posting", "Ledger Postings", "Posting Detail", "Posting Draft", ledgerColumns(), []string{"draft", "submitted", "posted", "rejected", "cancelled"}, ledgerSections(), ledgerFormSections())...,
					)...,
				)...,
			),
		},
		Templates: []module.TemplateDefinition{
			commercialTemplateDefinition("commercial.sales_order.print.default", "Sales Order Print", "sales_order", "Sales Order", []string{"party_name", "order_date", "currency_code", "total_amount", "lines"}),
			commercialTemplateDefinition("commercial.invoice.print.default", "Invoice Print", "invoice", "Invoice", []string{"party_name", "invoice_date", "due_date", "currency_code", "total_amount", "lines"}),
			commercialTemplateDefinition("commercial.credit_note.print.default", "Credit Note Print", "credit_note", "Credit Note", []string{"party_name", "credit_date", "source_invoice_number", "currency_code", "total_amount", "lines"}),
			commercialTemplateDefinition("commercial.payment_receipt.print.default", "Payment Receipt Print", "payment_receipt", "Payment Receipt", []string{"party_name", "receipt_date", "payment_method_code", "amount_received", "allocations"}),
			commercialTemplateDefinition("commercial.payment_refund.print.default", "Payment Refund Print", "payment_refund", "Payment Refund", []string{"party_name", "refund_date", "payment_method_code", "amount_refunded", "source_credit_note_number", "refund_allocations"}),
			commercialTemplateDefinition("commercial.ledger_posting.print.default", "Ledger Posting Print", "ledger_posting", "Ledger Posting", []string{"source_document_type", "source_document_id", "posting_date", "posting_rule_key", "journal_lines"}),
		},
	}
}

func commercialCatalogModelDefinition(key, singular, plural, permissionPrefix string, fields []model.FieldDefinition) model.Definition {
	return model.Definition{
		Key:                 key,
		DisplayName:         singular,
		DisplayNameI18n:     localize(singular, singular),
		OwnerModuleKey:      "commercial_core",
		Version:             "v1",
		CreatePermissionKey: permissionPrefix + ".create",
		ListPermissionKey:   permissionPrefix + ".list",
		ReadPermissionKey:   permissionPrefix + ".read",
		UpdatePermissionKey: permissionPrefix + ".update",
		DefaultSort:         fields[0].Key,
		Fields:              fields,
	}
}

func commercialDocumentDefinition(documentType, displayName, workflowKey, numberingKey string) document.Definition {
	return document.Definition{
		Type:                   documentType,
		DisplayName:            displayName,
		SchemaVersion:          "v1",
		WorkflowKey:            workflowKey,
		NumberingKey:           numberingKey,
		OwnerModuleKey:         "commercial_core",
		AllowedLinkTypes:       []string{"related_to", "source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for", "delivery_for", "return_for", "exchange_for", "production_for"},
		AllowedAttachmentTypes: []string{"note", "image", "document"},
	}
}

func commercialWorkflowDefinition(key, approvedState string, allowCancelApproved bool) workflow.Definition {
	states := []string{"draft", "submitted", approvedState, "rejected", "cancelled"}
	actions := []workflow.ActionRule{
		{Action: "submit", FromState: "draft", ToState: "submitted", PermissionKey: "document.submit"},
		{Action: "approve", FromState: "submitted", ToState: approvedState, PermissionKey: "document.approve"},
		{Action: "reject", FromState: "submitted", ToState: "rejected", PermissionKey: "document.reject"},
		{Action: "reopen", FromState: "rejected", ToState: "draft", PermissionKey: "document.reopen"},
		{Action: "reopen", FromState: approvedState, ToState: "draft", PermissionKey: "document.reopen"},
		{Action: "cancel", FromState: "draft", ToState: "cancelled", PermissionKey: "document.cancel"},
		{Action: "cancel", FromState: "submitted", ToState: "cancelled", PermissionKey: "document.cancel"},
	}
	if allowCancelApproved {
		actions = append(actions, workflow.ActionRule{Action: "cancel", FromState: approvedState, ToState: "cancelled", PermissionKey: "document.cancel"})
	}
	return workflow.Definition{
		Key:     key,
		States:  states,
		Actions: actions,
	}
}

func commercialModelPermissions(resource, display string) []module.PermissionDefinition {
	return []module.PermissionDefinition{
		{Key: resource + ".create", Action: "create", Resource: resource, DisplayName: "Create " + display + "s", DisplayNameI18n: localize("Create "+display+"s", "Buat "+display)},
		{Key: resource + ".list", Action: "list", Resource: resource, DisplayName: "List " + display + "s", DisplayNameI18n: localize("List "+display+"s", "Daftar "+display)},
		{Key: resource + ".read", Action: "read", Resource: resource, DisplayName: "Read " + display + "s", DisplayNameI18n: localize("Read "+display+"s", "Lihat "+display)},
		{Key: resource + ".update", Action: "update", Resource: resource, DisplayName: "Update " + display + "s", DisplayNameI18n: localize("Update "+display+"s", "Perbarui "+display)},
	}
}

func commercialModelMenus() []module.MenuDefinition {
	return []module.MenuDefinition{
		{Key: "commercial.catalog", Label: "Catalog", LabelI18n: localize("Catalog", "Katalog"), ActionKey: "commercial.catalog.list", Order: 19, RequiredPermissions: []string{"item.list"}},
		{Key: "commercial.products", Label: "Products", LabelI18n: localize("Products", "Produk"), ActionKey: "commercial.products.list", Order: 20, RequiredPermissions: []string{"product.list"}},
		{Key: "commercial.items", Label: "Items", LabelI18n: localize("Items", "Item"), ActionKey: "commercial.items.list", Order: 21, RequiredPermissions: []string{"item.list"}},
		{Key: "commercial.variant_dimensions", Label: "Variant Dimensions", LabelI18n: localize("Variant Dimensions", "Dimensi Varian"), ActionKey: "commercial.variant_dimensions.list", Order: 22, RequiredPermissions: []string{"variant_dimension.list"}},
		{Key: "commercial.variant_values", Label: "Variant Values", LabelI18n: localize("Variant Values", "Nilai Varian"), ActionKey: "commercial.variant_values.list", Order: 23, RequiredPermissions: []string{"variant_value.list"}},
		{Key: "commercial.item_categories", Label: "Item Categories", LabelI18n: localize("Item Categories", "Kategori Item"), ActionKey: "commercial.item_categories.list", Order: 24, RequiredPermissions: []string{"item_category.list"}},
		{Key: "commercial.uoms", Label: "Units", LabelI18n: localize("Units", "Satuan"), ActionKey: "commercial.uoms.list", Order: 25, RequiredPermissions: []string{"uom.list"}},
		{Key: "commercial.tax_codes", Label: "Tax Codes", LabelI18n: localize("Tax Codes", "Kode Pajak"), ActionKey: "commercial.tax_codes.list", Order: 26, RequiredPermissions: []string{"tax_code.list"}},
		{Key: "commercial.tax_profiles", Label: "Tax Profiles", LabelI18n: localize("Tax Profiles", "Profil Pajak"), ActionKey: "commercial.tax_profiles.list", Order: 27, RequiredPermissions: []string{"tax_profile.list"}},
		{Key: "commercial.price_lists", Label: "Price Lists", LabelI18n: localize("Price Lists", "Daftar Harga"), ActionKey: "commercial.price_lists.list", Order: 28, RequiredPermissions: []string{"price_list.list"}},
		{Key: "commercial.price_list_items", Label: "Price List Items", LabelI18n: localize("Price List Items", "Item Daftar Harga"), ActionKey: "commercial.price_list_items.list", Order: 29, RequiredPermissions: []string{"price_list_item.list"}},
		{Key: "commercial.accounts", Label: "Accounts", LabelI18n: localize("Accounts", "Akun"), ActionKey: "commercial.accounts.list", Order: 30, RequiredPermissions: []string{"account.list"}},
		{Key: "commercial.payment_methods", Label: "Payment Methods", LabelI18n: localize("Payment Methods", "Metode Pembayaran"), ActionKey: "commercial.payment_methods.list", Order: 31, RequiredPermissions: []string{"payment_method.list"}},
	}
}

func commercialModelActions() []module.ActionDefinition {
	return []module.ActionDefinition{
		{Key: "commercial.catalog.list", Label: "Catalog", LabelI18n: localize("Catalog", "Katalog"), Kind: "navigate", RoutePath: "/commercial/catalog", ViewKey: "commercial.items.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"item.list"}},
		{Key: "commercial.catalog.detail", Label: "Catalog Detail", LabelI18n: localize("Catalog Detail", "Detail Katalog"), Kind: "navigate", RoutePath: "/commercial/catalog/detail", ViewKey: "commercial.items.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"item.read"}},
		{Key: "commercial.catalog.form", Label: "Catalog Form", LabelI18n: localize("Catalog Form", "Form Katalog"), Kind: "navigate", RoutePath: "/commercial/catalog/form", ViewKey: "commercial.items.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"item.update"}},
		{Key: "commercial.products.list", Label: "Products", LabelI18n: localize("Products", "Produk"), Kind: "navigate", RoutePath: "/commercial/products", ViewKey: "commercial.products.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"product.list"}},
		{Key: "commercial.products.detail", Label: "Product Detail", LabelI18n: localize("Product Detail", "Detail Produk"), Kind: "navigate", RoutePath: "/commercial/products/detail", ViewKey: "commercial.products.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"product.read"}},
		{Key: "commercial.products.form", Label: "Product Form", LabelI18n: localize("Product Form", "Form Produk"), Kind: "navigate", RoutePath: "/commercial/products/form", ViewKey: "commercial.products.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"product.update"}},
		{Key: "commercial.items.list", Label: "Items", LabelI18n: localize("Items", "Item"), Kind: "navigate", RoutePath: "/commercial/items", ViewKey: "commercial.items.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"item.list"}},
		{Key: "commercial.items.detail", Label: "Item Detail", LabelI18n: localize("Item Detail", "Detail Item"), Kind: "navigate", RoutePath: "/commercial/items/detail", ViewKey: "commercial.items.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"item.read"}},
		{Key: "commercial.items.form", Label: "Item Form", LabelI18n: localize("Item Form", "Form Item"), Kind: "navigate", RoutePath: "/commercial/items/form", ViewKey: "commercial.items.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"item.update"}},
		{Key: "commercial.variant_dimensions.list", Label: "Variant Dimensions", LabelI18n: localize("Variant Dimensions", "Dimensi Varian"), Kind: "navigate", RoutePath: "/commercial/variant-dimensions", ViewKey: "commercial.variant_dimensions.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"variant_dimension.list"}},
		{Key: "commercial.variant_dimensions.detail", Label: "Variant Dimension Detail", LabelI18n: localize("Variant Dimension Detail", "Detail Dimensi Varian"), Kind: "navigate", RoutePath: "/commercial/variant-dimensions/detail", ViewKey: "commercial.variant_dimensions.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"variant_dimension.read"}},
		{Key: "commercial.variant_dimensions.form", Label: "Variant Dimension Form", LabelI18n: localize("Variant Dimension Form", "Form Dimensi Varian"), Kind: "navigate", RoutePath: "/commercial/variant-dimensions/form", ViewKey: "commercial.variant_dimensions.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"variant_dimension.update"}},
		{Key: "commercial.variant_values.list", Label: "Variant Values", LabelI18n: localize("Variant Values", "Nilai Varian"), Kind: "navigate", RoutePath: "/commercial/variant-values", ViewKey: "commercial.variant_values.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"variant_value.list"}},
		{Key: "commercial.variant_values.detail", Label: "Variant Value Detail", LabelI18n: localize("Variant Value Detail", "Detail Nilai Varian"), Kind: "navigate", RoutePath: "/commercial/variant-values/detail", ViewKey: "commercial.variant_values.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"variant_value.read"}},
		{Key: "commercial.variant_values.form", Label: "Variant Value Form", LabelI18n: localize("Variant Value Form", "Form Nilai Varian"), Kind: "navigate", RoutePath: "/commercial/variant-values/form", ViewKey: "commercial.variant_values.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"variant_value.update"}},
		{Key: "commercial.item_categories.list", Label: "Item Categories", LabelI18n: localize("Item Categories", "Kategori Item"), Kind: "navigate", RoutePath: "/commercial/item-categories", ViewKey: "commercial.item_categories.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"item_category.list"}},
		{Key: "commercial.item_categories.detail", Label: "Item Category Detail", LabelI18n: localize("Item Category Detail", "Detail Kategori Item"), Kind: "navigate", RoutePath: "/commercial/item-categories/detail", ViewKey: "commercial.item_categories.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"item_category.read"}},
		{Key: "commercial.item_categories.form", Label: "Item Category Form", LabelI18n: localize("Item Category Form", "Form Kategori Item"), Kind: "navigate", RoutePath: "/commercial/item-categories/form", ViewKey: "commercial.item_categories.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"item_category.update"}},
		{Key: "commercial.uoms.list", Label: "Units", LabelI18n: localize("Units", "Satuan"), Kind: "navigate", RoutePath: "/commercial/uoms", ViewKey: "commercial.uoms.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"uom.list"}},
		{Key: "commercial.uoms.detail", Label: "Unit Detail", LabelI18n: localize("Unit Detail", "Detail Satuan"), Kind: "navigate", RoutePath: "/commercial/uoms/detail", ViewKey: "commercial.uoms.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"uom.read"}},
		{Key: "commercial.uoms.form", Label: "Unit Form", LabelI18n: localize("Unit Form", "Form Satuan"), Kind: "navigate", RoutePath: "/commercial/uoms/form", ViewKey: "commercial.uoms.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"uom.update"}},
		{Key: "commercial.tax_codes.list", Label: "Tax Codes", LabelI18n: localize("Tax Codes", "Kode Pajak"), Kind: "navigate", RoutePath: "/commercial/tax-codes", ViewKey: "commercial.tax_codes.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"tax_code.list"}},
		{Key: "commercial.tax_codes.detail", Label: "Tax Code Detail", LabelI18n: localize("Tax Code Detail", "Detail Kode Pajak"), Kind: "navigate", RoutePath: "/commercial/tax-codes/detail", ViewKey: "commercial.tax_codes.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"tax_code.read"}},
		{Key: "commercial.tax_codes.form", Label: "Tax Code Form", LabelI18n: localize("Tax Code Form", "Form Kode Pajak"), Kind: "navigate", RoutePath: "/commercial/tax-codes/form", ViewKey: "commercial.tax_codes.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"tax_code.update"}},
		{Key: "commercial.tax_profiles.list", Label: "Tax Profiles", LabelI18n: localize("Tax Profiles", "Profil Pajak"), Kind: "navigate", RoutePath: "/commercial/tax-profiles", ViewKey: "commercial.tax_profiles.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"tax_profile.list"}},
		{Key: "commercial.tax_profiles.detail", Label: "Tax Profile Detail", LabelI18n: localize("Tax Profile Detail", "Detail Profil Pajak"), Kind: "navigate", RoutePath: "/commercial/tax-profiles/detail", ViewKey: "commercial.tax_profiles.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"tax_profile.read"}},
		{Key: "commercial.tax_profiles.form", Label: "Tax Profile Form", LabelI18n: localize("Tax Profile Form", "Form Profil Pajak"), Kind: "navigate", RoutePath: "/commercial/tax-profiles/form", ViewKey: "commercial.tax_profiles.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"tax_profile.update"}},
		{Key: "commercial.price_lists.list", Label: "Price Lists", LabelI18n: localize("Price Lists", "Daftar Harga"), Kind: "navigate", RoutePath: "/commercial/price-lists", ViewKey: "commercial.price_lists.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"price_list.list"}},
		{Key: "commercial.price_lists.detail", Label: "Price List Detail", LabelI18n: localize("Price List Detail", "Detail Daftar Harga"), Kind: "navigate", RoutePath: "/commercial/price-lists/detail", ViewKey: "commercial.price_lists.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"price_list.read"}},
		{Key: "commercial.price_lists.form", Label: "Price List Form", LabelI18n: localize("Price List Form", "Form Daftar Harga"), Kind: "navigate", RoutePath: "/commercial/price-lists/form", ViewKey: "commercial.price_lists.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"price_list.update"}},
		{Key: "commercial.price_list_items.list", Label: "Price List Items", LabelI18n: localize("Price List Items", "Item Daftar Harga"), Kind: "navigate", RoutePath: "/commercial/price-list-items", ViewKey: "commercial.price_list_items.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"price_list_item.list"}},
		{Key: "commercial.price_list_items.detail", Label: "Price List Item Detail", LabelI18n: localize("Price List Item Detail", "Detail Item Daftar Harga"), Kind: "navigate", RoutePath: "/commercial/price-list-items/detail", ViewKey: "commercial.price_list_items.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"price_list_item.read"}},
		{Key: "commercial.price_list_items.form", Label: "Price List Item Form", LabelI18n: localize("Price List Item Form", "Form Item Daftar Harga"), Kind: "navigate", RoutePath: "/commercial/price-list-items/form", ViewKey: "commercial.price_list_items.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"price_list_item.update"}},
		{Key: "commercial.accounts.list", Label: "Accounts", LabelI18n: localize("Accounts", "Akun"), Kind: "navigate", RoutePath: "/commercial/accounts", ViewKey: "commercial.accounts.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"account.list"}},
		{Key: "commercial.accounts.detail", Label: "Account Detail", LabelI18n: localize("Account Detail", "Detail Akun"), Kind: "navigate", RoutePath: "/commercial/accounts/detail", ViewKey: "commercial.accounts.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"account.read"}},
		{Key: "commercial.accounts.form", Label: "Account Form", LabelI18n: localize("Account Form", "Form Akun"), Kind: "navigate", RoutePath: "/commercial/accounts/form", ViewKey: "commercial.accounts.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"account.update"}},
		{Key: "commercial.payment_methods.list", Label: "Payment Methods", LabelI18n: localize("Payment Methods", "Metode Pembayaran"), Kind: "navigate", RoutePath: "/commercial/payment-methods", ViewKey: "commercial.payment_methods.list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"payment_method.list"}},
		{Key: "commercial.payment_methods.detail", Label: "Payment Method Detail", LabelI18n: localize("Payment Method Detail", "Detail Metode Pembayaran"), Kind: "navigate", RoutePath: "/commercial/payment-methods/detail", ViewKey: "commercial.payment_methods.detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"payment_method.read"}},
		{Key: "commercial.payment_methods.form", Label: "Payment Method Form", LabelI18n: localize("Payment Method Form", "Form Metode Pembayaran"), Kind: "navigate", RoutePath: "/commercial/payment-methods/form", ViewKey: "commercial.payment_methods.form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"payment_method.update"}},
	}
}

func commercialModelViews() []module.ViewDefinition {
	return []module.ViewDefinition{
		commercialProductListView(),
		commercialProductDetailView(),
		commercialProductFormView(),
		commercialModelListView("commercial.variant_dimensions.list", "Variant Dimensions", "commercial_variant_dimension", []module.ColumnDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("commercial.variant_dimensions.detail", "Variant Dimension Detail", "commercial_variant_dimension", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"}, {Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"}}),
		commercialModelFormView("commercial.variant_dimensions.form", "Variant Dimension Form", "commercial_variant_dimension", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true}, {Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string", Widget: "textarea"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}}}),
		commercialModelListView("commercial.variant_values.list", "Variant Values", "commercial_variant_value", []module.ColumnDefinition{{Key: "dimension_code", Label: "Dimension", LabelI18n: localize("Dimension", "Dimensi"), Path: "values.dimension_code"}, {Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("commercial.variant_values.detail", "Variant Value Detail", "commercial_variant_value", []module.FieldDefinition{{Key: "dimension_code", Label: "Dimension", LabelI18n: localize("Dimension", "Dimensi"), Path: "values.dimension_code", Type: "string"}, {Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"}, {Key: "sort_order", Label: "Sort Order", LabelI18n: localize("Sort Order", "Urutan"), Path: "values.sort_order", Type: "number"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"}}),
		commercialModelFormView("commercial.variant_values.form", "Variant Value Form", "commercial_variant_value", []module.FieldDefinition{{Key: "dimension_code", Label: "Dimension", LabelI18n: localize("Dimension", "Dimensi"), Path: "values.dimension_code", Type: "string", Widget: "select", Required: true}, {Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true}, {Key: "sort_order", Label: "Sort Order", LabelI18n: localize("Sort Order", "Urutan"), Path: "values.sort_order", Type: "number"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}}}),
		commercialItemListView(),
		commercialItemDetailView(),
		commercialItemFormView(),
		commercialModelListView("commercial.item_categories.list", "Item Categories", "commercial_item_category", []module.ColumnDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("commercial.item_categories.detail", "Item Category Detail", "commercial_item_category", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"}, {Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"}}),
		commercialModelFormView("commercial.item_categories.form", "Item Category Form", "commercial_item_category", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true}, {Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string", Widget: "textarea"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}}}),
		commercialModelListView("commercial.uoms.list", "Units", "commercial_uom", []module.ColumnDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"}, {Key: "symbol", Label: "Symbol", LabelI18n: localize("Symbol", "Simbol"), Path: "values.symbol"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("commercial.uoms.detail", "Unit Detail", "commercial_uom", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"}, {Key: "symbol", Label: "Symbol", LabelI18n: localize("Symbol", "Simbol"), Path: "values.symbol", Type: "string"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"}}),
		commercialModelFormView("commercial.uoms.form", "Unit Form", "commercial_uom", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true}, {Key: "symbol", Label: "Symbol", LabelI18n: localize("Symbol", "Simbol"), Path: "values.symbol", Type: "string", Widget: "text"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}}}),
		commercialModelListView("commercial.tax_codes.list", "Tax Codes", "commercial_tax_code", []module.ColumnDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"}, {Key: "mode", Label: "Mode", LabelI18n: localize("Mode", "Mode"), Path: "values.mode"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("commercial.tax_codes.detail", "Tax Code Detail", "commercial_tax_code", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"}, {Key: "mode", Label: "Mode", LabelI18n: localize("Mode", "Mode"), Path: "values.mode", Type: "string"}, {Key: "rate_percent", Label: "Rate Percent", LabelI18n: localize("Rate Percent", "Persen Tarif"), Path: "values.rate_percent", Type: "number"}, {Key: "tax_account_code", Label: "Tax Account", LabelI18n: localize("Tax Account", "Akun Pajak"), Path: "values.tax_account_code", Type: "string"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"}}),
		commercialModelFormView("commercial.tax_codes.form", "Tax Code Form", "commercial_tax_code", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true}, {Key: "mode", Label: "Mode", LabelI18n: localize("Mode", "Mode"), Path: "values.mode", Type: "string", Widget: "select", Options: []string{"exclusive", "inclusive", "exempt"}}, {Key: "rate_percent", Label: "Rate Percent", LabelI18n: localize("Rate Percent", "Persen Tarif"), Path: "values.rate_percent", Type: "number"}, {Key: "tax_account_code", Label: "Tax Account", LabelI18n: localize("Tax Account", "Akun Pajak"), Path: "values.tax_account_code", Type: "string", Widget: "text"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}}}),
		commercialModelListView("commercial.tax_profiles.list", "Tax Profiles", "commercial_tax_profile", []module.ColumnDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"}, {Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "values.default_tax_code"}, {Key: "payment_term_days", Label: "Payment Terms", LabelI18n: localize("Payment Terms", "Termin Pembayaran"), Path: "values.payment_term_days"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("commercial.tax_profiles.detail", "Tax Profile Detail", "commercial_tax_profile", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"}, {Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "values.default_tax_code", Type: "string"}, {Key: "payment_term_days", Label: "Payment Term Days", LabelI18n: localize("Payment Term Days", "Hari Termin Pembayaran"), Path: "values.payment_term_days", Type: "number"}, {Key: "price_tax_mode", Label: "Price Tax Mode", LabelI18n: localize("Price Tax Mode", "Mode Harga Pajak"), Path: "values.price_tax_mode", Type: "string"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"}}),
		commercialModelFormView("commercial.tax_profiles.form", "Tax Profile Form", "commercial_tax_profile", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true}, {Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "values.default_tax_code", Type: "string", Widget: "select"}, {Key: "payment_term_days", Label: "Payment Term Days", LabelI18n: localize("Payment Term Days", "Hari Termin Pembayaran"), Path: "values.payment_term_days", Type: "number"}, {Key: "price_tax_mode", Label: "Price Tax Mode", LabelI18n: localize("Price Tax Mode", "Mode Harga Pajak"), Path: "values.price_tax_mode", Type: "string", Widget: "select", Options: []string{"exclusive", "inclusive", "exempt"}}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}}}),
		commercialModelListView("commercial.price_lists.list", "Price Lists", "commercial_price_list", []module.ColumnDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"}, {Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("commercial.price_lists.detail", "Price List Detail", "commercial_price_list", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"}, {Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string"}, {Key: "effective_from", Label: "Effective From", LabelI18n: localize("Effective From", "Berlaku Mulai"), Path: "values.effective_from", Type: "string"}, {Key: "effective_to", Label: "Effective To", LabelI18n: localize("Effective To", "Berlaku Sampai"), Path: "values.effective_to", Type: "string"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"}}),
		commercialModelFormView("commercial.price_lists.form", "Price List Form", "commercial_price_list", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true}, {Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string", Widget: "text"}, {Key: "effective_from", Label: "Effective From", LabelI18n: localize("Effective From", "Berlaku Mulai"), Path: "values.effective_from", Type: "string", Widget: "text"}, {Key: "effective_to", Label: "Effective To", LabelI18n: localize("Effective To", "Berlaku Sampai"), Path: "values.effective_to", Type: "string", Widget: "text"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}}}),
		commercialModelListView("commercial.price_list_items.list", "Price List Items", "commercial_price_list_item", []module.ColumnDefinition{{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Path: "values.price_list_code"}, {Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "values.item_code"}, {Key: "unit_price", Label: "Unit Price", LabelI18n: localize("Unit Price", "Harga Satuan"), Path: "values.unit_price"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("commercial.price_list_items.detail", "Price List Item Detail", "commercial_price_list_item", []module.FieldDefinition{{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Path: "values.price_list_code", Type: "string"}, {Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "values.item_code", Type: "string"}, {Key: "unit_price", Label: "Unit Price", LabelI18n: localize("Unit Price", "Harga Satuan"), Path: "values.unit_price", Type: "number"}, {Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string"}, {Key: "tax_code", Label: "Tax Code", LabelI18n: localize("Tax Code", "Kode Pajak"), Path: "values.tax_code", Type: "string"}, {Key: "revenue_account_code", Label: "Revenue Account", LabelI18n: localize("Revenue Account", "Akun Pendapatan"), Path: "values.revenue_account_code", Type: "string"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"}}),
		commercialModelFormView("commercial.price_list_items.form", "Price List Item Form", "commercial_price_list_item", []module.FieldDefinition{{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Path: "values.price_list_code", Type: "string", Widget: "select", Required: true}, {Key: "item_code", Label: "Item", LabelI18n: localize("Item", "Item"), Path: "values.item_code", Type: "string", Widget: "select", Required: true}, {Key: "unit_price", Label: "Unit Price", LabelI18n: localize("Unit Price", "Harga Satuan"), Path: "values.unit_price", Type: "number", Required: true}, {Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string", Widget: "text"}, {Key: "tax_code", Label: "Tax Code", LabelI18n: localize("Tax Code", "Kode Pajak"), Path: "values.tax_code", Type: "string", Widget: "select"}, {Key: "revenue_account_code", Label: "Revenue Account", LabelI18n: localize("Revenue Account", "Akun Pendapatan"), Path: "values.revenue_account_code", Type: "string", Widget: "text"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}}}),
		commercialModelListView("commercial.accounts.list", "Accounts", "commercial_account", []module.ColumnDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"}, {Key: "category", Label: "Category", LabelI18n: localize("Category", "Kategori"), Path: "values.category"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("commercial.accounts.detail", "Account Detail", "commercial_account", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"}, {Key: "category", Label: "Category", LabelI18n: localize("Category", "Kategori"), Path: "values.category", Type: "string"}, {Key: "normal_balance", Label: "Normal Balance", LabelI18n: localize("Normal Balance", "Saldo Normal"), Path: "values.normal_balance", Type: "string"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"}}),
		commercialModelFormView("commercial.accounts.form", "Account Form", "commercial_account", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true}, {Key: "category", Label: "Category", LabelI18n: localize("Category", "Kategori"), Path: "values.category", Type: "string", Widget: "select", Options: []string{"asset", "liability", "equity", "revenue", "expense"}, Required: true}, {Key: "normal_balance", Label: "Normal Balance", LabelI18n: localize("Normal Balance", "Saldo Normal"), Path: "values.normal_balance", Type: "string", Widget: "select", Options: []string{"debit", "credit"}}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}}}),
		commercialModelListView("commercial.payment_methods.list", "Payment Methods", "payment_method", []module.ColumnDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"}, {Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"}}, []string{"active", "inactive"}),
		commercialModelDetailView("commercial.payment_methods.detail", "Payment Method Detail", "payment_method", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"}, {Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind", Type: "string"}, {Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Path: "values.clearing_account_code", Type: "string"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"}}),
		commercialModelFormView("commercial.payment_methods.form", "Payment Method Form", "payment_method", []module.FieldDefinition{{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true}, {Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true}, {Key: "kind", Label: "Kind", LabelI18n: localize("Kind", "Jenis"), Path: "values.kind", Type: "string", Widget: "select", Options: []string{"cash", "bank_transfer", "card", "e_wallet", "cheque", "other"}, Required: true}, {Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Path: "values.clearing_account_code", Type: "string", Widget: "text"}, {Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}}}),
	}
}

func commercialModelListView(key, title, modelKey string, columns []module.ColumnDefinition, statusOptions []string) module.ViewDefinition {
	filters := []module.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: statusOptions}}
	if modelKey == "commercial_product" {
		filters = append(filters,
			module.FilterDefinition{Key: "category_code", Label: "Category", LabelI18n: localize("Category", "Kategori"), Type: "string"},
			module.FilterDefinition{Key: "item_type", Label: "Item Type", LabelI18n: localize("Item Type", "Tipe Item"), Type: "enum", Options: []string{"product", "service", "fee"}},
		)
	}
	if modelKey == "commercial_item" {
		filters = append(filters,
			module.FilterDefinition{Key: "product_code", Label: "Product", LabelI18n: localize("Product", "Produk"), Type: "string"},
			module.FilterDefinition{Key: "item_type", Label: "Item Type", LabelI18n: localize("Item Type", "Tipe Item"), Type: "enum", Options: []string{"product", "service", "fee"}},
			module.FilterDefinition{Key: "category_code", Label: "Category", LabelI18n: localize("Category", "Kategori"), Type: "string"},
			module.FilterDefinition{Key: "is_variant", Label: "Variant SKU", LabelI18n: localize("Variant SKU", "SKU Varian"), Type: "enum", Options: []string{"true", "false"}},
			module.FilterDefinition{Key: "is_sellable", Label: "Sellable", LabelI18n: localize("Sellable", "Dapat Dijual"), Type: "enum", Options: []string{"true", "false"}},
			module.FilterDefinition{Key: "inventory_enabled", Label: "Inventory", LabelI18n: localize("Inventory", "Inventori"), Type: "enum", Options: []string{"true", "false"}},
			module.FilterDefinition{Key: "inventory_tracking_mode", Label: "Tracking", LabelI18n: localize("Tracking", "Pelacakan"), Type: "enum", Options: []string{"none", "quantity", "batch"}},
			module.FilterDefinition{Key: "replenishment_enabled", Label: "Replenishment", LabelI18n: localize("Replenishment", "Replenishment"), Type: "enum", Options: []string{"true", "false"}},
			module.FilterDefinition{Key: "replenishment_mode", Label: "Replenishment Mode", LabelI18n: localize("Replenishment Mode", "Mode Replenishment"), Type: "enum", Options: []string{"manual", "reorder_point_target"}},
		)
	}
	return module.ViewDefinition{
		Key:                 key,
		Title:               title,
		TitleI18n:           localize(title, title),
		Kind:                "list",
		ModelKey:            modelKey,
		RequiredPermissions: []string{modelPermissionPrefix(modelKey) + ".list"},
		Columns:             columns,
		Filters:             filters,
		DefaultPageSize:     10,
		EmptyState:          "No records available yet.",
		EmptyStateI18n:      localize("No records available yet.", "Belum ada data."),
	}
}

func commercialItemListView() module.ViewDefinition {
	return commercialModelListView("commercial.items.list", "Items", "commercial_item", []module.ColumnDefinition{
		{Key: "product_code", Label: "Product", LabelI18n: localize("Product", "Produk"), Path: "values.product_code"},
		{Key: "sku", Label: "SKU", LabelI18n: localize("SKU", "SKU"), Path: "values.sku"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
		{Key: "variant_label", Label: "Variant", LabelI18n: localize("Variant", "Varian"), Path: "values.variant_label"},
		{Key: "item_type", Label: "Type", LabelI18n: localize("Type", "Tipe"), Path: "values.item_type"},
		{Key: "category_code", Label: "Category", LabelI18n: localize("Category", "Kategori"), Path: "values.category_code"},
		{Key: "base_price", Label: "Base Price", LabelI18n: localize("Base Price", "Harga Dasar"), Path: "values.base_price"},
		{Key: "inventory_tracking_mode", Label: "Tracking", LabelI18n: localize("Tracking", "Pelacakan"), Path: "values.inventory_tracking_mode"},
		{Key: "replenishment_enabled", Label: "Replenishment", LabelI18n: localize("Replenishment", "Replenishment"), Path: "values.replenishment_enabled"},
		{Key: "target_stock_quantity", Label: "Target Stock", LabelI18n: localize("Target Stock", "Target Stok"), Path: "values.target_stock_quantity"},
		{Key: "uom_code", Label: "UOM", LabelI18n: localize("UOM", "Satuan"), Path: "values.uom_code"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []string{"active", "inactive"})
}

func commercialItemDetailView() module.ViewDefinition {
	return commercialModelDetailView("commercial.items.detail", "Item Detail", "commercial_item", []module.FieldDefinition{
		{Key: "product_code", Label: "Product", LabelI18n: localize("Product", "Produk"), Path: "values.product_code", Type: "string"},
		{Key: "sku", Label: "SKU", LabelI18n: localize("SKU", "SKU"), Path: "values.sku", Type: "string"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
		{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string"},
		{Key: "is_variant", Label: "Variant SKU", LabelI18n: localize("Variant SKU", "SKU Varian"), Path: "values.is_variant", Type: "bool"},
		{Key: "variant_signature", Label: "Variant Signature", LabelI18n: localize("Variant Signature", "Signature Varian"), Path: "values.variant_signature", Type: "string"},
		{Key: "variant_label", Label: "Variant Label", LabelI18n: localize("Variant Label", "Label Varian"), Path: "values.variant_label", Type: "string"},
		{Key: "variant_values", Label: "Variant Values", LabelI18n: localize("Variant Values", "Nilai Varian"), Path: "values.variant_values", Type: "string"},
		{Key: "item_type", Label: "Item Type", LabelI18n: localize("Item Type", "Tipe Item"), Path: "values.item_type", Type: "string"},
		{Key: "category_code", Label: "Category", LabelI18n: localize("Category", "Kategori"), Path: "values.category_code", Type: "string"},
		{Key: "is_sellable", Label: "Sellable", LabelI18n: localize("Sellable", "Dapat Dijual"), Path: "values.is_sellable", Type: "bool"},
		{Key: "uom_code", Label: "UOM", LabelI18n: localize("UOM", "Satuan"), Path: "values.uom_code", Type: "string"},
		{Key: "base_price", Label: "Base Price", LabelI18n: localize("Base Price", "Harga Dasar"), Path: "values.base_price", Type: "number"},
		{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string"},
		{Key: "tax_code", Label: "Tax Code", LabelI18n: localize("Tax Code", "Kode Pajak"), Path: "values.tax_code", Type: "string"},
		{Key: "revenue_account_code", Label: "Revenue Account", LabelI18n: localize("Revenue Account", "Akun Pendapatan"), Path: "values.revenue_account_code", Type: "string"},
		{Key: "inventory_asset_account_code", Label: "Inventory Asset Account", LabelI18n: localize("Inventory Asset Account", "Akun Aset Inventori"), Path: "values.inventory_asset_account_code", Type: "string"},
		{Key: "cogs_account_code", Label: "COGS Account", LabelI18n: localize("COGS Account", "Akun HPP"), Path: "values.cogs_account_code", Type: "string"},
		{Key: "wip_account_code", Label: "WIP Account", LabelI18n: localize("WIP Account", "Akun BDP"), Path: "values.wip_account_code", Type: "string"},
		{Key: "inventory_enabled", Label: "Inventory Enabled", LabelI18n: localize("Inventory Enabled", "Inventori Aktif"), Path: "values.inventory_enabled", Type: "bool"},
		{Key: "inventory_tracking_mode", Label: "Inventory Tracking", LabelI18n: localize("Inventory Tracking", "Pelacakan Inventori"), Path: "values.inventory_tracking_mode", Type: "string"},
		{Key: "expiry_tracking_enabled", Label: "Expiry Tracking", LabelI18n: localize("Expiry Tracking", "Pelacakan Kedaluwarsa"), Path: "values.expiry_tracking_enabled", Type: "bool"},
		{Key: "allow_negative_stock", Label: "Allow Negative Stock", LabelI18n: localize("Allow Negative Stock", "Izinkan Stok Negatif"), Path: "values.allow_negative_stock", Type: "bool"},
		{Key: "default_issue_strategy", Label: "Issue Strategy", LabelI18n: localize("Issue Strategy", "Strategi Pengeluaran"), Path: "values.default_issue_strategy", Type: "string"},
		{Key: "replenishment_enabled", Label: "Replenishment Enabled", LabelI18n: localize("Replenishment Enabled", "Replenishment Aktif"), Path: "values.replenishment_enabled", Type: "bool"},
		{Key: "replenishment_mode", Label: "Replenishment Mode", LabelI18n: localize("Replenishment Mode", "Mode Replenishment"), Path: "values.replenishment_mode", Type: "string"},
		{Key: "reorder_point_quantity", Label: "Reorder Point", LabelI18n: localize("Reorder Point", "Titik Pemesanan Ulang"), Path: "values.reorder_point_quantity", Type: "number"},
		{Key: "target_stock_quantity", Label: "Target Stock", LabelI18n: localize("Target Stock", "Target Stok"), Path: "values.target_stock_quantity", Type: "number"},
		{Key: "default_replenishment_warehouse_code", Label: "Default Replenishment Warehouse", LabelI18n: localize("Default Replenishment Warehouse", "Gudang Replenishment Default"), Path: "values.default_replenishment_warehouse_code", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
	})
}

func commercialItemFormView() module.ViewDefinition {
	return commercialModelFormView("commercial.items.form", "Item Form", "commercial_item", []module.FieldDefinition{
		{Key: "product_code", Label: "Product", LabelI18n: localize("Product", "Produk"), Path: "values.product_code", Type: "string", Widget: "select"},
		{Key: "sku", Label: "SKU", LabelI18n: localize("SKU", "SKU"), Path: "values.sku", Type: "string", Widget: "text", Required: true},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
		{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string", Widget: "textarea"},
		{Key: "is_variant", Label: "Variant SKU", LabelI18n: localize("Variant SKU", "SKU Varian"), Path: "values.is_variant", Type: "bool"},
		{Key: "variant_signature", Label: "Variant Signature", LabelI18n: localize("Variant Signature", "Signature Varian"), Path: "values.variant_signature", Type: "string", Widget: "text"},
		{Key: "variant_label", Label: "Variant Label", LabelI18n: localize("Variant Label", "Label Varian"), Path: "values.variant_label", Type: "string", Widget: "text"},
		{Key: "variant_values", Label: "Variant Values", LabelI18n: localize("Variant Values", "Nilai Varian"), Path: "values.variant_values", Type: "string", Widget: "textarea"},
		{Key: "item_type", Label: "Item Type", LabelI18n: localize("Item Type", "Tipe Item"), Path: "values.item_type", Type: "string", Widget: "select", Options: []string{"product", "service", "fee"}, Required: true},
		{Key: "category_code", Label: "Category", LabelI18n: localize("Category", "Kategori"), Path: "values.category_code", Type: "string", Widget: "select"},
		{Key: "tags", Label: "Tags", LabelI18n: localize("Tags", "Tag"), Path: "values.tags", Type: "string", Widget: "text"},
		{Key: "is_sellable", Label: "Sellable", LabelI18n: localize("Sellable", "Dapat Dijual"), Path: "values.is_sellable", Type: "bool"},
		{Key: "uom_code", Label: "UOM", LabelI18n: localize("UOM", "Satuan"), Path: "values.uom_code", Type: "string", Widget: "select"},
		{Key: "base_price", Label: "Base Price", LabelI18n: localize("Base Price", "Harga Dasar"), Path: "values.base_price", Type: "number"},
		{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string", Widget: "text"},
		{Key: "tax_code", Label: "Tax Code", LabelI18n: localize("Tax Code", "Kode Pajak"), Path: "values.tax_code", Type: "string", Widget: "select"},
		{Key: "revenue_account_code", Label: "Revenue Account", LabelI18n: localize("Revenue Account", "Akun Pendapatan"), Path: "values.revenue_account_code", Type: "string", Widget: "text"},
		{Key: "inventory_asset_account_code", Label: "Inventory Asset Account", LabelI18n: localize("Inventory Asset Account", "Akun Aset Inventori"), Path: "values.inventory_asset_account_code", Type: "string", Widget: "text"},
		{Key: "cogs_account_code", Label: "COGS Account", LabelI18n: localize("COGS Account", "Akun HPP"), Path: "values.cogs_account_code", Type: "string", Widget: "text"},
		{Key: "wip_account_code", Label: "WIP Account", LabelI18n: localize("WIP Account", "Akun BDP"), Path: "values.wip_account_code", Type: "string", Widget: "text"},
		{Key: "service_unit", Label: "Service Unit", LabelI18n: localize("Service Unit", "Unit Layanan"), Path: "values.service_unit", Type: "string", Widget: "text"},
		{Key: "standard_duration_minutes", Label: "Standard Duration Minutes", LabelI18n: localize("Standard Duration Minutes", "Durasi Standar Menit"), Path: "values.standard_duration_minutes", Type: "number"},
		{Key: "product_unit", Label: "Product Unit", LabelI18n: localize("Product Unit", "Unit Produk"), Path: "values.product_unit", Type: "string", Widget: "text"},
		{Key: "fulfillment_mode", Label: "Fulfillment Mode", LabelI18n: localize("Fulfillment Mode", "Mode Pemenuhan"), Path: "values.fulfillment_mode", Type: "string", Widget: "select", Options: []string{"manual", "delivery", "digital", "pickup"}},
		{Key: "inventory_enabled", Label: "Inventory Enabled", LabelI18n: localize("Inventory Enabled", "Inventori Aktif"), Path: "values.inventory_enabled", Type: "bool"},
		{Key: "inventory_tracking_mode", Label: "Inventory Tracking", LabelI18n: localize("Inventory Tracking", "Pelacakan Inventori"), Path: "values.inventory_tracking_mode", Type: "string", Widget: "select", Options: []string{"none", "quantity", "batch"}},
		{Key: "expiry_tracking_enabled", Label: "Expiry Tracking", LabelI18n: localize("Expiry Tracking", "Pelacakan Kedaluwarsa"), Path: "values.expiry_tracking_enabled", Type: "bool"},
		{Key: "allow_negative_stock", Label: "Allow Negative Stock", LabelI18n: localize("Allow Negative Stock", "Izinkan Stok Negatif"), Path: "values.allow_negative_stock", Type: "bool"},
		{Key: "default_issue_strategy", Label: "Issue Strategy", LabelI18n: localize("Issue Strategy", "Strategi Pengeluaran"), Path: "values.default_issue_strategy", Type: "string", Widget: "select", Options: []string{"manual", "fifo", "fefo"}},
		{Key: "replenishment_enabled", Label: "Replenishment Enabled", LabelI18n: localize("Replenishment Enabled", "Replenishment Aktif"), Path: "values.replenishment_enabled", Type: "bool"},
		{Key: "replenishment_mode", Label: "Replenishment Mode", LabelI18n: localize("Replenishment Mode", "Mode Replenishment"), Path: "values.replenishment_mode", Type: "string", Widget: "select", Options: []string{"manual", "reorder_point_target"}},
		{Key: "reorder_point_quantity", Label: "Reorder Point", LabelI18n: localize("Reorder Point", "Titik Pemesanan Ulang"), Path: "values.reorder_point_quantity", Type: "number"},
		{Key: "target_stock_quantity", Label: "Target Stock", LabelI18n: localize("Target Stock", "Target Stok"), Path: "values.target_stock_quantity", Type: "number"},
		{Key: "default_replenishment_warehouse_code", Label: "Default Replenishment Warehouse", LabelI18n: localize("Default Replenishment Warehouse", "Gudang Replenishment Default"), Path: "values.default_replenishment_warehouse_code", Type: "string", Widget: "select"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
	})
}

func commercialProductListView() module.ViewDefinition {
	return commercialModelListView("commercial.products.list", "Products", "commercial_product", []module.ColumnDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name"},
		{Key: "brand", Label: "Brand", LabelI18n: localize("Brand", "Merek"), Path: "values.brand"},
		{Key: "item_type", Label: "Type", LabelI18n: localize("Type", "Tipe"), Path: "values.item_type"},
		{Key: "category_code", Label: "Category", LabelI18n: localize("Category", "Kategori"), Path: "values.category_code"},
		{Key: "variant_dimension_codes", Label: "Variant Dimensions", LabelI18n: localize("Variant Dimensions", "Dimensi Varian"), Path: "values.variant_dimension_codes"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status"},
	}, []string{"active", "inactive"})
}

func commercialProductDetailView() module.ViewDefinition {
	return commercialModelDetailView("commercial.products.detail", "Product Detail", "commercial_product", []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string"},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string"},
		{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string"},
		{Key: "brand", Label: "Brand", LabelI18n: localize("Brand", "Merek"), Path: "values.brand", Type: "string"},
		{Key: "item_type", Label: "Item Type", LabelI18n: localize("Item Type", "Tipe Item"), Path: "values.item_type", Type: "string"},
		{Key: "category_code", Label: "Category", LabelI18n: localize("Category", "Kategori"), Path: "values.category_code", Type: "string"},
		{Key: "variant_dimension_codes", Label: "Variant Dimensions", LabelI18n: localize("Variant Dimensions", "Dimensi Varian"), Path: "values.variant_dimension_codes", Type: "string"},
		{Key: "uom_code", Label: "UOM", LabelI18n: localize("UOM", "Satuan"), Path: "values.uom_code", Type: "string"},
		{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string"},
		{Key: "tax_code", Label: "Tax Code", LabelI18n: localize("Tax Code", "Kode Pajak"), Path: "values.tax_code", Type: "string"},
		{Key: "revenue_account_code", Label: "Revenue Account", LabelI18n: localize("Revenue Account", "Akun Pendapatan"), Path: "values.revenue_account_code", Type: "string"},
		{Key: "inventory_asset_account_code", Label: "Inventory Asset Account", LabelI18n: localize("Inventory Asset Account", "Akun Aset Inventori"), Path: "values.inventory_asset_account_code", Type: "string"},
		{Key: "cogs_account_code", Label: "COGS Account", LabelI18n: localize("COGS Account", "Akun HPP"), Path: "values.cogs_account_code", Type: "string"},
		{Key: "wip_account_code", Label: "WIP Account", LabelI18n: localize("WIP Account", "Akun BDP"), Path: "values.wip_account_code", Type: "string"},
		{Key: "inventory_enabled", Label: "Inventory Enabled", LabelI18n: localize("Inventory Enabled", "Inventori Aktif"), Path: "values.inventory_enabled", Type: "bool"},
		{Key: "inventory_tracking_mode", Label: "Inventory Tracking", LabelI18n: localize("Inventory Tracking", "Pelacakan Inventori"), Path: "values.inventory_tracking_mode", Type: "string"},
		{Key: "expiry_tracking_enabled", Label: "Expiry Tracking", LabelI18n: localize("Expiry Tracking", "Pelacakan Kedaluwarsa"), Path: "values.expiry_tracking_enabled", Type: "bool"},
		{Key: "allow_negative_stock", Label: "Allow Negative Stock", LabelI18n: localize("Allow Negative Stock", "Izinkan Stok Negatif"), Path: "values.allow_negative_stock", Type: "bool"},
		{Key: "default_issue_strategy", Label: "Issue Strategy", LabelI18n: localize("Issue Strategy", "Strategi Pengeluaran"), Path: "values.default_issue_strategy", Type: "string"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string"},
	})
}

func commercialProductFormView() module.ViewDefinition {
	return commercialModelFormView("commercial.products.form", "Product Form", "commercial_product", []module.FieldDefinition{
		{Key: "code", Label: "Code", LabelI18n: localize("Code", "Kode"), Path: "values.code", Type: "string", Widget: "text", Required: true},
		{Key: "name", Label: "Name", LabelI18n: localize("Name", "Nama"), Path: "values.name", Type: "string", Widget: "text", Required: true},
		{Key: "description", Label: "Description", LabelI18n: localize("Description", "Deskripsi"), Path: "values.description", Type: "string", Widget: "textarea"},
		{Key: "brand", Label: "Brand", LabelI18n: localize("Brand", "Merek"), Path: "values.brand", Type: "string", Widget: "text"},
		{Key: "item_type", Label: "Item Type", LabelI18n: localize("Item Type", "Tipe Item"), Path: "values.item_type", Type: "string", Widget: "select", Options: []string{"product", "service", "fee"}, Required: true},
		{Key: "category_code", Label: "Category", LabelI18n: localize("Category", "Kategori"), Path: "values.category_code", Type: "string", Widget: "select"},
		{Key: "tags", Label: "Tags", LabelI18n: localize("Tags", "Tag"), Path: "values.tags", Type: "string", Widget: "text"},
		{Key: "variant_dimension_codes", Label: "Variant Dimensions", LabelI18n: localize("Variant Dimensions", "Dimensi Varian"), Path: "values.variant_dimension_codes", Type: "string", Widget: "text", HelpText: "Comma-separated dimension codes such as color,size."},
		{Key: "uom_code", Label: "UOM", LabelI18n: localize("UOM", "Satuan"), Path: "values.uom_code", Type: "string", Widget: "select"},
		{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "values.currency_code", Type: "string", Widget: "text"},
		{Key: "tax_code", Label: "Tax Code", LabelI18n: localize("Tax Code", "Kode Pajak"), Path: "values.tax_code", Type: "string", Widget: "select"},
		{Key: "revenue_account_code", Label: "Revenue Account", LabelI18n: localize("Revenue Account", "Akun Pendapatan"), Path: "values.revenue_account_code", Type: "string", Widget: "text"},
		{Key: "inventory_asset_account_code", Label: "Inventory Asset Account", LabelI18n: localize("Inventory Asset Account", "Akun Aset Inventori"), Path: "values.inventory_asset_account_code", Type: "string", Widget: "text"},
		{Key: "cogs_account_code", Label: "COGS Account", LabelI18n: localize("COGS Account", "Akun HPP"), Path: "values.cogs_account_code", Type: "string", Widget: "text"},
		{Key: "wip_account_code", Label: "WIP Account", LabelI18n: localize("WIP Account", "Akun BDP"), Path: "values.wip_account_code", Type: "string", Widget: "text"},
		{Key: "inventory_enabled", Label: "Inventory Enabled", LabelI18n: localize("Inventory Enabled", "Inventori Aktif"), Path: "values.inventory_enabled", Type: "bool"},
		{Key: "inventory_tracking_mode", Label: "Inventory Tracking", LabelI18n: localize("Inventory Tracking", "Pelacakan Inventori"), Path: "values.inventory_tracking_mode", Type: "string", Widget: "select", Options: []string{"none", "quantity", "batch"}},
		{Key: "expiry_tracking_enabled", Label: "Expiry Tracking", LabelI18n: localize("Expiry Tracking", "Pelacakan Kedaluwarsa"), Path: "values.expiry_tracking_enabled", Type: "bool"},
		{Key: "allow_negative_stock", Label: "Allow Negative Stock", LabelI18n: localize("Allow Negative Stock", "Izinkan Stok Negatif"), Path: "values.allow_negative_stock", Type: "bool"},
		{Key: "default_issue_strategy", Label: "Issue Strategy", LabelI18n: localize("Issue Strategy", "Strategi Pengeluaran"), Path: "values.default_issue_strategy", Type: "string", Widget: "select", Options: []string{"manual", "fifo", "fefo"}},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "values.status", Type: "string", Widget: "select", Options: []string{"active", "inactive"}},
	})
}

func commercialModelDetailView(key, title, modelKey string, fields []module.FieldDefinition) module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 key,
		Title:               title,
		TitleI18n:           localize(title, title),
		Kind:                "detail",
		ModelKey:            modelKey,
		RequiredPermissions: []string{modelPermissionPrefix(modelKey) + ".read"},
		Sections: []module.SectionDefinition{{
			Key: "summary", Title: "Summary", TitleI18n: localize("Summary", "Ringkasan"), Fields: fields,
		}},
	}
}

func commercialModelFormView(key, title, modelKey string, fields []module.FieldDefinition) module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 key,
		Title:               title,
		TitleI18n:           localize(title, title),
		Kind:                "form",
		ModelKey:            modelKey,
		RequiredPermissions: []string{modelPermissionPrefix(modelKey) + ".update"},
		Sections: []module.SectionDefinition{{
			Key: "edit", Title: title, TitleI18n: localize(title, title), Fields: fields,
		}},
	}
}

func commercialDocumentActions(prefix, documentType, listLabel, detailLabel, newLabel, basePath string) []module.ActionDefinition {
	_ = documentType
	return []module.ActionDefinition{
		{Key: prefix + ".list", Label: listLabel, LabelI18n: localize(listLabel, listLabel), Kind: "navigate", RoutePath: basePath, ViewKey: prefix + ".list", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"document.list"}},
		{Key: prefix + ".new", Label: newLabel, LabelI18n: localize(newLabel, newLabel), Kind: "navigate", RoutePath: basePath + "/new", ViewKey: prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"document.create"}},
		{Key: prefix + ".detail", Label: detailLabel, LabelI18n: localize(detailLabel, detailLabel), Kind: "navigate", RoutePath: basePath + "/detail", ViewKey: prefix + ".detail", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"document.read"}},
		{Key: prefix + ".form", Label: detailLabel + " Form", LabelI18n: localize(detailLabel+" Form", detailLabel+" Form"), Kind: "navigate", RoutePath: basePath + "/form", ViewKey: prefix + ".form", RenderMode: module.RenderModeGeneric, RequiredPermissions: []string{"document.update_draft"}},
	}
}

func commercialDocumentViews(prefix, documentType, listTitle, detailTitle, formTitle string, columns []module.ColumnDefinition, statusOptions []string, detailSections, formSections []module.SectionDefinition) []module.ViewDefinition {
	listFilters := []module.FilterDefinition{{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: statusOptions}}
	if documentType == "invoice" {
		listFilters = invoiceFilters()
	}
	return []module.ViewDefinition{
		{
			Key:                 prefix + ".list",
			Title:               listTitle,
			TitleI18n:           localize(listTitle, listTitle),
			Kind:                "list",
			DocumentType:        documentType,
			ProjectionKey:       "document_summary",
			RequiredPermissions: []string{"document.list"},
			Columns:             columns,
			Filters:             listFilters,
			DefaultPageSize:     10,
			EmptyState:          fmt.Sprintf("No %s exist yet.", listTitle),
			EmptyStateI18n:      localize(fmt.Sprintf("No %s exist yet.", listTitle), fmt.Sprintf("Belum ada %s.", listTitle)),
		},
		{
			Key:                 prefix + ".detail",
			Title:               detailTitle,
			TitleI18n:           localize(detailTitle, detailTitle),
			Kind:                "detail",
			DocumentType:        documentType,
			Printable:           true,
			PrintPurpose:        "official",
			PrintChannel:        "print",
			RequiredPermissions: []string{"document.read"},
			AllowedActions:      commercialDetailActions(documentType),
			Sections:            detailSections,
		},
		{
			Key:                 prefix + ".form",
			Title:               formTitle,
			TitleI18n:           localize(formTitle, formTitle),
			Kind:                "form",
			DocumentType:        documentType,
			RequiredPermissions: []string{"document.update_draft"},
			Sections:            formSections,
		},
	}
}

func commercialDetailActions(documentType string) []string {
	actions := []string{"submit", "approve", "reject", "reopen", "cancel"}
	switch documentType {
	case "sales_order":
		return append(actions, "generate_fulfillment", "generate_invoice", "generate_production_order")
	case "invoice":
		return append(actions, "register_payment", "issue_credit_note")
	case "credit_note":
		return append(actions, "register_refund")
	case "purchase_request":
		return append(actions, "generate_purchase_order")
	case "purchase_order":
		return append(actions, "register_receipt", "register_vendor_bill")
	case "goods_receipt":
		return append(actions, "register_vendor_bill", "register_supplier_return")
	case "vendor_bill":
		return append(actions, "register_payment_out", "issue_vendor_credit_note", "register_supplier_return")
	case "sales_return":
		return append(actions, "register_return_receipt", "issue_credit_note", "register_refund")
	case "production_order":
		return append(actions, "register_production_issue", "register_production_output")
	case "supplier_return":
		return append(actions, "issue_vendor_credit_note")
	default:
		return actions
	}
}

func commercialDocumentSearchIndexes() []search.IndexDefinition {
	return []search.IndexDefinition{
		commercialDocumentSearchIndex("commercial.orders.search", "Order Search", "sales_order", "commercial.orders.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "party_name", Path: "body.payload.party_name", Type: "string", Searchable: true},
			{Key: "reference", Path: "body.payload.reference", Type: "string", Searchable: true},
			{Key: "order_date", Path: "body.payload.order_date", Type: "string", Sort: true},
			{Key: "total_amount", Path: "body.payload.total_amount", Type: "number", Sort: true},
		}),
		commercialDocumentSearchIndex("commercial.invoices.search", "Invoice Search", "invoice", "commercial.invoices.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "party_name", Path: "body.payload.party_name", Type: "string", Searchable: true},
			{Key: "invoice_date", Path: "body.payload.invoice_date", Type: "string", Sort: true},
			{Key: "due_date", Path: "body.payload.due_date", Type: "string", Sort: true},
			{Key: "total_amount", Path: "body.payload.total_amount", Type: "number", Sort: true},
		}),
		commercialDocumentSearchIndex("commercial.credit_notes.search", "Credit Note Search", "credit_note", "commercial.credit_notes.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "party_name", Path: "body.payload.party_name", Type: "string", Searchable: true},
			{Key: "credit_date", Path: "body.payload.credit_date", Type: "string", Sort: true},
			{Key: "source_invoice_number", Path: "body.payload.source_invoice_number", Type: "string", Searchable: true},
			{Key: "total_amount", Path: "body.payload.total_amount", Type: "number", Sort: true},
		}),
		commercialDocumentSearchIndex("commercial.payments.search", "Payment Search", "payment_receipt", "commercial.payments.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "party_name", Path: "body.payload.party_name", Type: "string", Searchable: true},
			{Key: "receipt_date", Path: "body.payload.receipt_date", Type: "string", Sort: true},
			{Key: "payment_method_code", Path: "body.payload.payment_method_code", Type: "string", Searchable: true},
			{Key: "amount_received", Path: "body.payload.amount_received", Type: "number", Sort: true},
		}),
		commercialDocumentSearchIndex("commercial.refunds.search", "Refund Search", "payment_refund", "commercial.refunds.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "party_name", Path: "body.payload.party_name", Type: "string", Searchable: true},
			{Key: "refund_date", Path: "body.payload.refund_date", Type: "string", Sort: true},
			{Key: "payment_method_code", Path: "body.payload.payment_method_code", Type: "string", Searchable: true},
			{Key: "amount_refunded", Path: "body.payload.amount_refunded", Type: "number", Sort: true},
		}),
		commercialDocumentSearchIndex("commercial.ledger.search", "Ledger Search", "ledger_posting", "commercial.ledger.list", []search.IndexFieldDefinition{
			{Key: "number", Path: "header.number", Type: "string", Searchable: true},
			{Key: "status", Path: "header.status", Type: "string", Facet: true, Sort: true},
			{Key: "posting_date", Path: "body.payload.posting_date", Type: "string", Sort: true},
			{Key: "source_document_type", Path: "body.payload.source_document_type", Type: "string", Searchable: true},
			{Key: "source_document_id", Path: "body.payload.source_document_id", Type: "string", Searchable: true},
			{Key: "total_amount", Path: "body.payload.total_amount", Type: "number", Sort: true},
		}),
	}
}

func commercialDocumentSearchIndex(key, title, documentType, viewKey string, extraFields []search.IndexFieldDefinition) search.IndexDefinition {
	fields := []search.IndexFieldDefinition{
		{Key: "document_id", Path: "header.id", Type: "string", Searchable: true},
		{Key: "document_type", Path: "header.type", Type: "string", Facet: true},
	}
	fields = append(fields, extraFields...)
	return search.IndexDefinition{
		Key:                 key,
		Title:               title,
		SourceKind:          "document",
		DocumentType:        documentType,
		ViewKey:             viewKey,
		Modes:               []string{"keyword", "vector", "hybrid"},
		OrganizationSplit:   true,
		RequiredPermissions: []string{"document.list"},
		QueryFilterFields:   []string{"document_type", "status"},
		QuerySortFields:     []string{"status"},
		Fields:              fields,
		VectorFields: []search.VectorFieldDefinition{{
			Key: "semantic", SourcePaths: []string{"body.payload.party_name", "body.payload.reference"}, EmbeddingMode: "external", Dimensions: 8, DistanceMetric: "cosine",
		}},
	}
}

func commercialModelSearchIndex(key, title, modelKey, viewKey string, fieldKeys []string) search.IndexDefinition {
	fields := make([]search.IndexFieldDefinition, 0, len(fieldKeys))
	for _, fieldKey := range fieldKeys {
		item := search.IndexFieldDefinition{
			Key:        fieldKey,
			Path:       fieldKey,
			Type:       "string",
			Searchable: true,
		}
		if fieldKey == "status" {
			item.Facet = true
			item.Sort = true
		}
		fields = append(fields, item)
	}
	sortFields := []string{"status"}
	if len(fieldKeys) > 0 {
		sortFields = append(sortFields, fieldKeys[0])
	}
	return search.IndexDefinition{
		Key:                 key,
		Title:               title,
		SourceKind:          "model",
		ModelKey:            modelKey,
		ViewKey:             viewKey,
		Modes:               []string{"keyword", "vector", "hybrid"},
		OrganizationSplit:   true,
		RequiredPermissions: []string{modelPermissionPrefix(modelKey) + ".list"},
		QueryFilterFields:   []string{"status"},
		QuerySortFields:     sortFields,
		Fields:              fields,
		VectorFields: []search.VectorFieldDefinition{{
			Key: "semantic", SourcePaths: fieldKeys[:minInt(2, len(fieldKeys))], EmbeddingMode: "external", Dimensions: 8, DistanceMetric: "cosine",
		}},
	}
}

func commercialTemplateDefinition(key, title, targetKey, heading string, payloadKeys []string) module.TemplateDefinition {
	body := `<article class="print-sheet"><header><h1>` + heading + `</h1><p>{{ .document.Header.Number }}</p></header><dl>`
	for _, payloadKey := range payloadKeys {
		switch payloadKey {
		case "lines":
			body += `</dl><section><h2>Lines</h2><table class="print-table"><thead><tr><th>Item</th><th>Description</th><th>Qty</th><th>Unit Price</th><th>Total</th></tr></thead><tbody>{{ range (index .document.Body.Payload "lines") }}<tr><td>{{ index . "item_code" }}</td><td>{{ index . "description" }}</td><td>{{ index . "quantity" }}</td><td>{{ index . "unit_price" }}</td><td>{{ index . "line_total" }}</td></tr>{{ end }}</tbody></table></section><dl>`
		case "allocations":
			body += `</dl><section><h2>Allocations</h2><table class="print-table"><thead><tr><th>Invoice</th><th>Invoice ID</th><th>Amount</th><th>Note</th></tr></thead><tbody>{{ range (index .document.Body.Payload "allocations") }}<tr><td>{{ index . "invoice_number" }}</td><td>{{ index . "invoice_id" }}</td><td>{{ index . "amount" }}</td><td>{{ index . "note" }}</td></tr>{{ end }}</tbody></table></section><dl>`
		case "refund_allocations":
			body += `</dl><section><h2>Refund Allocations</h2><table class="print-table"><thead><tr><th>Receipt</th><th>Receipt ID</th><th>Amount</th><th>Note</th></tr></thead><tbody>{{ range (index .document.Body.Payload "refund_allocations") }}<tr><td>{{ index . "payment_number" }}</td><td>{{ index . "payment_id" }}</td><td>{{ index . "amount" }}</td><td>{{ index . "note" }}</td></tr>{{ end }}</tbody></table></section><dl>`
		case "journal_lines":
			body += `</dl><section><h2>Journal Lines</h2><table class="print-table"><thead><tr><th>Account</th><th>Description</th><th>Debit</th><th>Credit</th></tr></thead><tbody>{{ range (index .document.Body.Payload "journal_lines") }}<tr><td>{{ index . "account_code" }}</td><td>{{ index . "description" }}</td><td>{{ index . "debit" }}</td><td>{{ index . "credit" }}</td></tr>{{ end }}</tbody></table></section><dl>`
		default:
			body += `<dt>` + humanizeTitle(payloadKey) + `</dt><dd>{{ index .document.Body.Payload "` + payloadKey + `" }}</dd>`
		}
	}
	body += `</dl></article>`
	return module.TemplateDefinition{
		Key:                 key,
		Title:               title,
		TitleI18n:           localize(title, title),
		Description:         "Default printable commercial document template.",
		DescriptionI18n:     localize("Default printable commercial document template.", "Template cetak dokumen komersial default."),
		TargetKind:          "document",
		TargetKey:           targetKey,
		RendererKind:        "html",
		DefaultFormat:       "html",
		Formats:             []string{"html", "pdf"},
		Purpose:             "official",
		Channel:             "print",
		AllowedScopes:       []string{"deployment", "organization", "location"},
		RequiredPermissions: []string{"template.read", "template.render"},
		DefaultBody:         body,
		DefaultStyle:        `.print-sheet{font-family:Arial,sans-serif;padding:24px;color:#0f172a}.print-sheet h1{margin:0 0 8px}.print-sheet dl{display:grid;grid-template-columns:180px 1fr;gap:8px 12px;white-space:pre-wrap}.print-sheet section{margin-top:20px}.print-table{width:100%;border-collapse:collapse}.print-table th,.print-table td{border:1px solid #cbd5e1;padding:6px 8px;text-align:left}`,
	}
}

func salesOrderColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Order", LabelI18n: localize("Order", "Pesanan"), Path: "header.number"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name"},
		{Key: "order_date", Label: "Order Date", LabelI18n: localize("Order Date", "Tanggal Pesanan"), Path: "body.payload.order_date"},
		{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func invoiceColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Invoice", LabelI18n: localize("Invoice", "Faktur"), Path: "header.number"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name"},
		{Key: "invoice_date", Label: "Invoice Date", LabelI18n: localize("Invoice Date", "Tanggal Faktur"), Path: "body.payload.invoice_date"},
		{Key: "due_date", Label: "Due Date", LabelI18n: localize("Due Date", "Jatuh Tempo"), Path: "body.payload.due_date"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func invoiceFilters() []module.FilterDefinition {
	return []module.FilterDefinition{
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Type: "enum", Options: []string{"draft", "submitted", "issued", "partially_paid", "paid", "refunded", "rejected", "cancelled"}},
		{Key: "receivable_state", Label: "Receivable State", LabelI18n: localize("Receivable State", "Status Piutang"), Type: "enum", Options: []string{"open", "due_today", "overdue", "current", "paid"}},
	}
}

func paymentColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Receipt", LabelI18n: localize("Receipt", "Tanda Terima"), Path: "header.number"},
		{Key: "party_name", Label: "Payer", LabelI18n: localize("Payer", "Pembayar"), Path: "body.payload.party_name"},
		{Key: "receipt_date", Label: "Receipt Date", LabelI18n: localize("Receipt Date", "Tanggal Penerimaan"), Path: "body.payload.receipt_date"},
		{Key: "payment_method_code", Label: "Method", LabelI18n: localize("Method", "Metode"), Path: "body.payload.payment_method_code"},
		{Key: "amount_received", Label: "Amount", LabelI18n: localize("Amount", "Jumlah"), Path: "body.payload.amount_received"},
		{Key: "refunded_amount", Label: "Refunded", LabelI18n: localize("Refunded", "Direfund"), Path: "body.payload.refunded_amount"},
		{Key: "unapplied_amount", Label: "Unapplied", LabelI18n: localize("Unapplied", "Belum Dialokasikan"), Path: "body.payload.unapplied_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func refundColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Refund", LabelI18n: localize("Refund", "Refund"), Path: "header.number"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name"},
		{Key: "refund_date", Label: "Refund Date", LabelI18n: localize("Refund Date", "Tanggal Refund"), Path: "body.payload.refund_date"},
		{Key: "source_invoice_number", Label: "Source Invoice", LabelI18n: localize("Source Invoice", "Faktur Sumber"), Path: "body.payload.source_invoice_number"},
		{Key: "payment_method_code", Label: "Method", LabelI18n: localize("Method", "Metode"), Path: "body.payload.payment_method_code"},
		{Key: "amount_refunded", Label: "Amount", LabelI18n: localize("Amount", "Jumlah"), Path: "body.payload.amount_refunded"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func creditNoteColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Credit Note", LabelI18n: localize("Credit Note", "Nota Kredit"), Path: "header.number"},
		{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name"},
		{Key: "credit_date", Label: "Credit Date", LabelI18n: localize("Credit Date", "Tanggal Kredit"), Path: "body.payload.credit_date"},
		{Key: "source_invoice_number", Label: "Source Invoice", LabelI18n: localize("Source Invoice", "Faktur Sumber"), Path: "body.payload.source_invoice_number"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func ledgerColumns() []module.ColumnDefinition {
	return []module.ColumnDefinition{
		{Key: "number", Label: "Posting", LabelI18n: localize("Posting", "Posting"), Path: "header.number"},
		{Key: "source_document_type", Label: "Source Type", LabelI18n: localize("Source Type", "Tipe Sumber"), Path: "body.payload.source_document_type"},
		{Key: "source_document_id", Label: "Source ID", LabelI18n: localize("Source ID", "ID Sumber"), Path: "body.payload.source_document_id"},
		{Key: "posting_rule_key", Label: "Posting Rule", LabelI18n: localize("Posting Rule", "Aturan Posting"), Path: "body.payload.posting_rule_key"},
		{Key: "posting_date", Label: "Posting Date", LabelI18n: localize("Posting Date", "Tanggal Posting"), Path: "body.payload.posting_date"},
		{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount"},
		{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status"},
	}
}

func commercialReceivablesDashboardView() module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 "commercial.receivables.dashboard",
		Title:               "Receivables",
		TitleI18n:           localize("Receivables", "Piutang"),
		Kind:                "dashboard",
		ProjectionKey:       "commercial.receivables.summary",
		RequiredPermissions: []string{"document.list"},
		Cards: []module.CardDefinition{
			{Key: "open_invoice_count", Label: "Open Invoices", LabelI18n: localize("Open Invoices", "Faktur Terbuka"), Path: "open_invoice_count", ActionKey: "commercial.invoices.list"},
			{Key: "open_balance_total", Label: "Open Balance", LabelI18n: localize("Open Balance", "Saldo Terbuka"), Path: "open_balance_total", ActionKey: "commercial.invoices.list"},
			{Key: "overdue_invoice_count", Label: "Overdue Invoices", LabelI18n: localize("Overdue Invoices", "Faktur Jatuh Tempo"), Path: "overdue_invoice_count", ActionKey: "commercial.invoices.list"},
			{Key: "overdue_balance_total", Label: "Overdue Balance", LabelI18n: localize("Overdue Balance", "Saldo Jatuh Tempo"), Path: "overdue_balance_total", ActionKey: "commercial.invoices.list"},
			{Key: "due_today_total", Label: "Due Today", LabelI18n: localize("Due Today", "Jatuh Tempo Hari Ini"), Path: "due_today_total", ActionKey: "commercial.invoices.list"},
			{Key: "current_balance_total", Label: "Current Balance", LabelI18n: localize("Current Balance", "Saldo Lancar"), Path: "current_balance_total", ActionKey: "commercial.invoices.list"},
			{Key: "paid_amount_total", Label: "Collected", LabelI18n: localize("Collected", "Tertagih"), Path: "paid_amount_total", ActionKey: "commercial.payments.list"},
			{Key: "refunded_amount_total", Label: "Refunded", LabelI18n: localize("Refunded", "Direfund"), Path: "refunded_amount_total", ActionKey: "commercial.refunds.list"},
		},
	}
}

func commercialPartyStatementDashboardView() module.ViewDefinition {
	return module.ViewDefinition{
		Key:                 "commercial.party_statement.dashboard",
		Title:               "Customer Statement",
		TitleI18n:           localize("Customer Statement", "Laporan Pelanggan"),
		Kind:                "dashboard",
		ProjectionKey:       "commercial.party_statement",
		RequiredPermissions: []string{"party.read", "document.list"},
		Cards: []module.CardDefinition{
			{Key: "open_invoice_count", Label: "Open Invoices", LabelI18n: localize("Open Invoices", "Faktur Terbuka"), Path: "open_invoice_count", ActionKey: "commercial.invoices.list"},
			{Key: "open_balance_total", Label: "Open Balance", LabelI18n: localize("Open Balance", "Saldo Terbuka"), Path: "open_balance_total", ActionKey: "commercial.invoices.list"},
			{Key: "paid_amount_total", Label: "Collected", LabelI18n: localize("Collected", "Tertagih"), Path: "paid_amount_total", ActionKey: "commercial.payments.list"},
			{Key: "credited_amount_total", Label: "Credited", LabelI18n: localize("Credited", "Dikreditkan"), Path: "credited_amount_total", ActionKey: "commercial.credit_notes.list"},
			{Key: "refunded_amount_total", Label: "Refunded", LabelI18n: localize("Refunded", "Direfund"), Path: "refunded_amount_total", ActionKey: "commercial.refunds.list"},
		},
	}
}

func salesOrderSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "summary", Title: "Order Summary", TitleI18n: localize("Order Summary", "Ringkasan Pesanan"), Fields: []module.FieldDefinition{
			{Key: "number", Label: "Order Number", LabelI18n: localize("Order Number", "Nomor Pesanan"), Path: "header.number", Type: "string", ReadOnly: true},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
			{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string"},
			{Key: "order_date", Label: "Order Date", LabelI18n: localize("Order Date", "Tanggal Pesanan"), Path: "body.payload.order_date", Type: "string"},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string"},
			{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Path: "body.payload.price_list_code", Type: "string"},
			{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "body.payload.tax_profile_code", Type: "string"},
			{Key: "sales_channel", Label: "Sales Channel", LabelI18n: localize("Sales Channel", "Kanal Penjualan"), Path: "body.payload.sales_channel", Type: "string"},
			{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "body.payload.store_code", Type: "string"},
			{Key: "promotion_codes", Label: "Promotion Codes", LabelI18n: localize("Promotion Codes", "Kode Promosi"), Path: "body.payload.promotion_codes", Type: "string"},
			{Key: "payment_term_days", Label: "Payment Terms", LabelI18n: localize("Payment Terms", "Termin Pembayaran"), Path: "body.payload.payment_term_days", Type: "number"},
			{Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "body.payload.default_tax_code", Type: "string"},
			{Key: "discount_amount_total", Label: "Discount", LabelI18n: localize("Discount", "Diskon"), Path: "body.payload.discount_amount_total", Type: "number"},
			{Key: "promotion_breakdown", Label: "Promotions", LabelI18n: localize("Promotions", "Promosi"), Path: "body.payload.promotion_breakdown", Type: "object"},
			{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
			{Key: "reference", Label: "Reference", LabelI18n: localize("Reference", "Referensi"), Path: "body.payload.reference", Type: "string"},
			{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "commercial_lines"},
		}},
	}
}

func invoiceSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "summary", Title: "Invoice Summary", TitleI18n: localize("Invoice Summary", "Ringkasan Faktur"), Fields: []module.FieldDefinition{
			{Key: "number", Label: "Invoice Number", LabelI18n: localize("Invoice Number", "Nomor Faktur"), Path: "header.number", Type: "string", ReadOnly: true},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
			{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string"},
			{Key: "invoice_date", Label: "Invoice Date", LabelI18n: localize("Invoice Date", "Tanggal Faktur"), Path: "body.payload.invoice_date", Type: "string"},
			{Key: "due_date", Label: "Due Date", LabelI18n: localize("Due Date", "Jatuh Tempo"), Path: "body.payload.due_date", Type: "string"},
			{Key: "source_order_id", Label: "Source Order", LabelI18n: localize("Source Order", "Pesanan Sumber"), Path: "body.payload.source_order_id", Type: "string"},
			{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Path: "body.payload.price_list_code", Type: "string"},
			{Key: "tax_profile_code", Label: "Tax Profile", LabelI18n: localize("Tax Profile", "Profil Pajak"), Path: "body.payload.tax_profile_code", Type: "string"},
			{Key: "sales_channel", Label: "Sales Channel", LabelI18n: localize("Sales Channel", "Kanal Penjualan"), Path: "body.payload.sales_channel", Type: "string"},
			{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "body.payload.store_code", Type: "string"},
			{Key: "promotion_codes", Label: "Promotion Codes", LabelI18n: localize("Promotion Codes", "Kode Promosi"), Path: "body.payload.promotion_codes", Type: "string"},
			{Key: "payment_term_days", Label: "Payment Terms", LabelI18n: localize("Payment Terms", "Termin Pembayaran"), Path: "body.payload.payment_term_days", Type: "number"},
			{Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "body.payload.default_tax_code", Type: "string"},
			{Key: "discount_amount_total", Label: "Discount", LabelI18n: localize("Discount", "Diskon"), Path: "body.payload.discount_amount_total", Type: "number"},
			{Key: "promotion_breakdown", Label: "Promotions", LabelI18n: localize("Promotions", "Promosi"), Path: "body.payload.promotion_breakdown", Type: "object"},
			{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
			{Key: "paid_amount", Label: "Paid", LabelI18n: localize("Paid", "Terbayar"), Path: "body.payload.paid_amount", Type: "number"},
			{Key: "refunded_amount", Label: "Refunded", LabelI18n: localize("Refunded", "Direfund"), Path: "body.payload.refunded_amount", Type: "number"},
			{Key: "balance_due_amount", Label: "Balance Due", LabelI18n: localize("Balance Due", "Sisa Tagihan"), Path: "body.payload.balance_due_amount", Type: "number"},
			{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "commercial_lines"},
		}},
	}
}

func paymentSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "summary", Title: "Payment Summary", TitleI18n: localize("Payment Summary", "Ringkasan Pembayaran"), Fields: []module.FieldDefinition{
			{Key: "number", Label: "Receipt Number", LabelI18n: localize("Receipt Number", "Nomor Tanda Terima"), Path: "header.number", Type: "string", ReadOnly: true},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
			{Key: "party_name", Label: "Payer", LabelI18n: localize("Payer", "Pembayar"), Path: "body.payload.party_name", Type: "string"},
			{Key: "receipt_date", Label: "Receipt Date", LabelI18n: localize("Receipt Date", "Tanggal Penerimaan"), Path: "body.payload.receipt_date", Type: "string"},
			{Key: "payment_method_code", Label: "Payment Method", LabelI18n: localize("Payment Method", "Metode Pembayaran"), Path: "body.payload.payment_method_code", Type: "string"},
			{Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Path: "body.payload.clearing_account_code", Type: "string"},
			{Key: "amount_received", Label: "Amount Received", LabelI18n: localize("Amount Received", "Jumlah Diterima"), Path: "body.payload.amount_received", Type: "number"},
			{Key: "refunded_amount", Label: "Refunded", LabelI18n: localize("Refunded", "Direfund"), Path: "body.payload.refunded_amount", Type: "number"},
			{Key: "unapplied_amount", Label: "Unapplied Amount", LabelI18n: localize("Unapplied Amount", "Jumlah Belum Dialokasikan"), Path: "body.payload.unapplied_amount", Type: "number"},
			{Key: "allocations", Label: "Allocations", LabelI18n: localize("Allocations", "Alokasi"), Path: "body.payload.allocations", Type: "object", Widget: "commercial_allocations"},
		}},
	}
}

func refundSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "summary", Title: "Refund Summary", TitleI18n: localize("Refund Summary", "Ringkasan Refund"), Fields: []module.FieldDefinition{
			{Key: "number", Label: "Refund Number", LabelI18n: localize("Refund Number", "Nomor Refund"), Path: "header.number", Type: "string", ReadOnly: true},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
			{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string"},
			{Key: "refund_date", Label: "Refund Date", LabelI18n: localize("Refund Date", "Tanggal Refund"), Path: "body.payload.refund_date", Type: "string"},
			{Key: "payment_method_code", Label: "Payment Method", LabelI18n: localize("Payment Method", "Metode Pembayaran"), Path: "body.payload.payment_method_code", Type: "string"},
			{Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Path: "body.payload.clearing_account_code", Type: "string"},
			{Key: "refund_reference", Label: "Reference", LabelI18n: localize("Reference", "Referensi"), Path: "body.payload.refund_reference", Type: "string"},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string"},
			{Key: "amount_refunded", Label: "Amount Refunded", LabelI18n: localize("Amount Refunded", "Jumlah Refund"), Path: "body.payload.amount_refunded", Type: "number"},
			{Key: "source_credit_note_number", Label: "Source Credit Note", LabelI18n: localize("Source Credit Note", "Nota Kredit Sumber"), Path: "body.payload.source_credit_note_number", Type: "string"},
			{Key: "source_invoice_number", Label: "Source Invoice", LabelI18n: localize("Source Invoice", "Faktur Sumber"), Path: "body.payload.source_invoice_number", Type: "string"},
			{Key: "source_payment_number", Label: "Source Payment", LabelI18n: localize("Source Payment", "Pembayaran Sumber"), Path: "body.payload.source_payment_number", Type: "string"},
			{Key: "refund_allocations", Label: "Refund Allocations", LabelI18n: localize("Refund Allocations", "Alokasi Refund"), Path: "body.payload.refund_allocations", Type: "object", Widget: "commercial_refund_allocations"},
			{Key: "reason", Label: "Reason", LabelI18n: localize("Reason", "Alasan"), Path: "body.payload.reason", Type: "string"},
		}},
	}
}

func creditNoteSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "summary", Title: "Credit Note Summary", TitleI18n: localize("Credit Note Summary", "Ringkasan Nota Kredit"), Fields: []module.FieldDefinition{
			{Key: "number", Label: "Credit Note Number", LabelI18n: localize("Credit Note Number", "Nomor Nota Kredit"), Path: "header.number", Type: "string", ReadOnly: true},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
			{Key: "party_name", Label: "Customer", LabelI18n: localize("Customer", "Pelanggan"), Path: "body.payload.party_name", Type: "string"},
			{Key: "credit_date", Label: "Credit Date", LabelI18n: localize("Credit Date", "Tanggal Kredit"), Path: "body.payload.credit_date", Type: "string"},
			{Key: "source_invoice_id", Label: "Source Invoice ID", LabelI18n: localize("Source Invoice ID", "ID Faktur Sumber"), Path: "body.payload.source_invoice_id", Type: "string"},
			{Key: "source_invoice_number", Label: "Source Invoice", LabelI18n: localize("Source Invoice", "Faktur Sumber"), Path: "body.payload.source_invoice_number", Type: "string"},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string"},
			{Key: "receivable_account_code", Label: "Receivable Account", LabelI18n: localize("Receivable Account", "Akun Piutang"), Path: "body.payload.receivable_account_code", Type: "string"},
			{Key: "subtotal_amount", Label: "Subtotal", LabelI18n: localize("Subtotal", "Subtotal"), Path: "body.payload.subtotal_amount", Type: "number"},
			{Key: "discount_amount_total", Label: "Discount", LabelI18n: localize("Discount", "Diskon"), Path: "body.payload.discount_amount_total", Type: "number"},
			{Key: "tax_amount", Label: "Tax Amount", LabelI18n: localize("Tax Amount", "Jumlah Pajak"), Path: "body.payload.tax_amount", Type: "number"},
			{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
			{Key: "refunded_amount", Label: "Refunded", LabelI18n: localize("Refunded", "Direfund"), Path: "body.payload.refunded_amount", Type: "number"},
			{Key: "reason", Label: "Reason", LabelI18n: localize("Reason", "Alasan"), Path: "body.payload.reason", Type: "string"},
			{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "commercial_lines"},
		}},
	}
}

func ledgerSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "summary", Title: "Posting Summary", TitleI18n: localize("Posting Summary", "Ringkasan Posting"), Fields: []module.FieldDefinition{
			{Key: "number", Label: "Posting Number", LabelI18n: localize("Posting Number", "Nomor Posting"), Path: "header.number", Type: "string", ReadOnly: true},
			{Key: "status", Label: "Status", LabelI18n: localize("Status", "Status"), Path: "header.status", Type: "string", ReadOnly: true},
			{Key: "source_document_type", Label: "Source Type", LabelI18n: localize("Source Type", "Tipe Sumber"), Path: "body.payload.source_document_type", Type: "string"},
			{Key: "source_document_id", Label: "Source ID", LabelI18n: localize("Source ID", "ID Sumber"), Path: "body.payload.source_document_id", Type: "string"},
			{Key: "posting_date", Label: "Posting Date", LabelI18n: localize("Posting Date", "Tanggal Posting"), Path: "body.payload.posting_date", Type: "string"},
			{Key: "posting_rule_key", Label: "Posting Rule", LabelI18n: localize("Posting Rule", "Aturan Posting"), Path: "body.payload.posting_rule_key", Type: "string"},
			{Key: "journal_source_kind", Label: "Journal Source Kind", LabelI18n: localize("Journal Source Kind", "Jenis Sumber Jurnal"), Path: "body.payload.journal_source_kind", Type: "string"},
			{Key: "journal_template_id", Label: "Journal Template ID", LabelI18n: localize("Journal Template ID", "ID Template Jurnal"), Path: "body.payload.journal_template_id", Type: "string"},
			{Key: "journal_run_id", Label: "Journal Run ID", LabelI18n: localize("Journal Run ID", "ID Run Jurnal"), Path: "body.payload.journal_run_id", Type: "string"},
			{Key: "accounting_period_id", Label: "Accounting Period ID", LabelI18n: localize("Accounting Period ID", "ID Periode Akuntansi"), Path: "body.payload.accounting_period_id", Type: "string"},
			{Key: "reversal_of_posting_id", Label: "Reversal Of Posting ID", LabelI18n: localize("Reversal Of Posting ID", "ID Posting Asal Reversal"), Path: "body.payload.reversal_of_posting_id", Type: "string"},
			{Key: "reversed_by_posting_id", Label: "Reversed By Posting ID", LabelI18n: localize("Reversed By Posting ID", "ID Posting Reversal"), Path: "body.payload.reversed_by_posting_id", Type: "string"},
			{Key: "reversal_status", Label: "Reversal Status", LabelI18n: localize("Reversal Status", "Status Reversal"), Path: "body.payload.reversal_status", Type: "string"},
			{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
			{Key: "journal_lines", Label: "Journal Lines", LabelI18n: localize("Journal Lines", "Baris Jurnal"), Path: "body.payload.journal_lines", Type: "object", Widget: "commercial_journal_lines"},
		}},
	}
}

func salesOrderFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "commercial", Title: "Commercial Terms", TitleI18n: localize("Commercial Terms", "Ketentuan Komersial"), Fields: []module.FieldDefinition{
			{Key: "party_id", Label: "Customer ID", LabelI18n: localize("Customer ID", "ID Pelanggan"), Path: "body.payload.party_id", Type: "string", Widget: "select", Required: true},
			{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Path: "body.payload.party_name", Type: "string", Widget: "text", Required: true},
			{Key: "order_date", Label: "Order Date", LabelI18n: localize("Order Date", "Tanggal Pesanan"), Path: "body.payload.order_date", Type: "string", Widget: "text", Required: true},
			{Key: "requested_delivery_date", Label: "Requested Delivery", LabelI18n: localize("Requested Delivery", "Tanggal Kirim Diminta"), Path: "body.payload.requested_delivery_date", Type: "string", Widget: "text"},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string", Widget: "text", Required: true},
			{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Path: "body.payload.price_list_code", Type: "string", Widget: "select"},
			{Key: "tax_profile_code", Label: "Tax Profile Code", LabelI18n: localize("Tax Profile Code", "Kode Profil Pajak"), Path: "body.payload.tax_profile_code", Type: "string", Widget: "select"},
			{Key: "sales_channel", Label: "Sales Channel", LabelI18n: localize("Sales Channel", "Kanal Penjualan"), Path: "body.payload.sales_channel", Type: "string", Widget: "text"},
			{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "body.payload.store_code", Type: "string", Widget: "text"},
			{Key: "promotion_codes", Label: "Promotion Codes", LabelI18n: localize("Promotion Codes", "Kode Promosi"), Path: "body.payload.promotion_codes", Type: "string", Widget: "textarea"},
			{Key: "payment_term_days", Label: "Payment Term Days", LabelI18n: localize("Payment Term Days", "Hari Termin Pembayaran"), Path: "body.payload.payment_term_days", Type: "number"},
			{Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "body.payload.default_tax_code", Type: "string", Widget: "select"},
			{Key: "reference", Label: "Reference", LabelI18n: localize("Reference", "Referensi"), Path: "body.payload.reference", Type: "string", Widget: "text"},
			{Key: "subtotal_amount", Label: "Subtotal", LabelI18n: localize("Subtotal", "Subtotal"), Path: "body.payload.subtotal_amount", Type: "number"},
			{Key: "discount_amount_total", Label: "Discount", LabelI18n: localize("Discount", "Diskon"), Path: "body.payload.discount_amount_total", Type: "number"},
			{Key: "promotion_breakdown", Label: "Promotions", LabelI18n: localize("Promotions", "Promosi"), Path: "body.payload.promotion_breakdown", Type: "object"},
			{Key: "tax_amount", Label: "Tax Amount", LabelI18n: localize("Tax Amount", "Jumlah Pajak"), Path: "body.payload.tax_amount", Type: "number"},
			{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
			{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "commercial_lines", HelpText: "Add line items directly. Totals are derived from the rows below."},
			{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
		}},
	}
}

func invoiceFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "billing", Title: "Billing Terms", TitleI18n: localize("Billing Terms", "Ketentuan Penagihan"), Fields: []module.FieldDefinition{
			{Key: "party_id", Label: "Customer ID", LabelI18n: localize("Customer ID", "ID Pelanggan"), Path: "body.payload.party_id", Type: "string", Widget: "select", Required: true},
			{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Path: "body.payload.party_name", Type: "string", Widget: "text", Required: true},
			{Key: "invoice_date", Label: "Invoice Date", LabelI18n: localize("Invoice Date", "Tanggal Faktur"), Path: "body.payload.invoice_date", Type: "string", Widget: "text", Required: true},
			{Key: "due_date", Label: "Due Date", LabelI18n: localize("Due Date", "Jatuh Tempo"), Path: "body.payload.due_date", Type: "string", Widget: "text", Required: true},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string", Widget: "text", Required: true},
			{Key: "source_order_id", Label: "Source Order ID", LabelI18n: localize("Source Order ID", "ID Pesanan Sumber"), Path: "body.payload.source_order_id", Type: "string", Widget: "text"},
			{Key: "price_list_code", Label: "Price List", LabelI18n: localize("Price List", "Daftar Harga"), Path: "body.payload.price_list_code", Type: "string", Widget: "select"},
			{Key: "tax_profile_code", Label: "Tax Profile Code", LabelI18n: localize("Tax Profile Code", "Kode Profil Pajak"), Path: "body.payload.tax_profile_code", Type: "string", Widget: "select"},
			{Key: "sales_channel", Label: "Sales Channel", LabelI18n: localize("Sales Channel", "Kanal Penjualan"), Path: "body.payload.sales_channel", Type: "string", Widget: "text"},
			{Key: "store_code", Label: "Store", LabelI18n: localize("Store", "Toko"), Path: "body.payload.store_code", Type: "string", Widget: "text"},
			{Key: "promotion_codes", Label: "Promotion Codes", LabelI18n: localize("Promotion Codes", "Kode Promosi"), Path: "body.payload.promotion_codes", Type: "string", Widget: "textarea"},
			{Key: "payment_term_days", Label: "Payment Term Days", LabelI18n: localize("Payment Term Days", "Hari Termin Pembayaran"), Path: "body.payload.payment_term_days", Type: "number"},
			{Key: "default_tax_code", Label: "Default Tax Code", LabelI18n: localize("Default Tax Code", "Kode Pajak Default"), Path: "body.payload.default_tax_code", Type: "string", Widget: "select"},
			{Key: "receivable_account_code", Label: "Receivable Account", LabelI18n: localize("Receivable Account", "Akun Piutang"), Path: "body.payload.receivable_account_code", Type: "string", Widget: "text"},
			{Key: "subtotal_amount", Label: "Subtotal", LabelI18n: localize("Subtotal", "Subtotal"), Path: "body.payload.subtotal_amount", Type: "number"},
			{Key: "tax_amount", Label: "Tax Amount", LabelI18n: localize("Tax Amount", "Jumlah Pajak"), Path: "body.payload.tax_amount", Type: "number"},
			{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
			{Key: "paid_amount", Label: "Paid", LabelI18n: localize("Paid", "Terbayar"), Path: "body.payload.paid_amount", Type: "number"},
			{Key: "refunded_amount", Label: "Refunded", LabelI18n: localize("Refunded", "Direfund"), Path: "body.payload.refunded_amount", Type: "number"},
			{Key: "balance_due_amount", Label: "Balance Due", LabelI18n: localize("Balance Due", "Sisa Tagihan"), Path: "body.payload.balance_due_amount", Type: "number"},
			{Key: "promotion_breakdown", Label: "Promotions", LabelI18n: localize("Promotions", "Promosi"), Path: "body.payload.promotion_breakdown", Type: "object"},
			{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "commercial_lines", HelpText: "Add invoice line items directly. Totals are derived from the rows below."},
			{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
		}},
	}
}

func creditNoteFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "credit", Title: "Credit Terms", TitleI18n: localize("Credit Terms", "Ketentuan Kredit"), Fields: []module.FieldDefinition{
			{Key: "party_id", Label: "Customer ID", LabelI18n: localize("Customer ID", "ID Pelanggan"), Path: "body.payload.party_id", Type: "string", Widget: "text", Required: true},
			{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Path: "body.payload.party_name", Type: "string", Widget: "text", Required: true},
			{Key: "credit_date", Label: "Credit Date", LabelI18n: localize("Credit Date", "Tanggal Kredit"), Path: "body.payload.credit_date", Type: "string", Widget: "text", Required: true},
			{Key: "source_invoice_id", Label: "Source Invoice ID", LabelI18n: localize("Source Invoice ID", "ID Faktur Sumber"), Path: "body.payload.source_invoice_id", Type: "string", Widget: "text", Required: true},
			{Key: "source_invoice_number", Label: "Source Invoice Number", LabelI18n: localize("Source Invoice Number", "Nomor Faktur Sumber"), Path: "body.payload.source_invoice_number", Type: "string", Widget: "text"},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string", Widget: "text", Required: true},
			{Key: "receivable_account_code", Label: "Receivable Account", LabelI18n: localize("Receivable Account", "Akun Piutang"), Path: "body.payload.receivable_account_code", Type: "string", Widget: "text"},
			{Key: "subtotal_amount", Label: "Subtotal", LabelI18n: localize("Subtotal", "Subtotal"), Path: "body.payload.subtotal_amount", Type: "number"},
			{Key: "tax_amount", Label: "Tax Amount", LabelI18n: localize("Tax Amount", "Jumlah Pajak"), Path: "body.payload.tax_amount", Type: "number"},
			{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
			{Key: "refunded_amount", Label: "Refunded", LabelI18n: localize("Refunded", "Direfund"), Path: "body.payload.refunded_amount", Type: "number"},
			{Key: "reason", Label: "Reason", LabelI18n: localize("Reason", "Alasan"), Path: "body.payload.reason", Type: "string", Widget: "textarea"},
			{Key: "lines", Label: "Line Items", LabelI18n: localize("Line Items", "Baris"), Path: "body.payload.lines", Type: "object", Widget: "commercial_lines", HelpText: "Credit note lines reverse the original invoice amount."},
		}},
	}
}

func paymentFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "payment", Title: "Payment Details", TitleI18n: localize("Payment Details", "Detail Pembayaran"), Fields: []module.FieldDefinition{
			{Key: "party_id", Label: "Payer ID", LabelI18n: localize("Payer ID", "ID Pembayar"), Path: "body.payload.party_id", Type: "string", Widget: "text", Required: true},
			{Key: "party_name", Label: "Payer Name", LabelI18n: localize("Payer Name", "Nama Pembayar"), Path: "body.payload.party_name", Type: "string", Widget: "text", Required: true},
			{Key: "receipt_date", Label: "Receipt Date", LabelI18n: localize("Receipt Date", "Tanggal Penerimaan"), Path: "body.payload.receipt_date", Type: "string", Widget: "text", Required: true},
			{Key: "payment_method_code", Label: "Payment Method", LabelI18n: localize("Payment Method", "Metode Pembayaran"), Path: "body.payload.payment_method_code", Type: "string", Widget: "text", Required: true},
			{Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Path: "body.payload.clearing_account_code", Type: "string", Widget: "text"},
			{Key: "payment_reference", Label: "Reference", LabelI18n: localize("Reference", "Referensi"), Path: "body.payload.payment_reference", Type: "string", Widget: "text"},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string", Widget: "text", Required: true},
			{Key: "amount_received", Label: "Amount Received", LabelI18n: localize("Amount Received", "Jumlah Diterima"), Path: "body.payload.amount_received", Type: "number", Required: true},
			{Key: "unapplied_amount", Label: "Unapplied Amount", LabelI18n: localize("Unapplied Amount", "Jumlah Belum Dialokasikan"), Path: "body.payload.unapplied_amount", Type: "number"},
			{Key: "allocations", Label: "Allocations", LabelI18n: localize("Allocations", "Alokasi"), Path: "body.payload.allocations", Type: "object", Widget: "commercial_allocations", HelpText: "Allocate the receipt across one or more invoices."},
			{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
		}},
	}
}

func refundFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "refund", Title: "Refund Details", TitleI18n: localize("Refund Details", "Detail Refund"), Fields: []module.FieldDefinition{
			{Key: "party_id", Label: "Customer ID", LabelI18n: localize("Customer ID", "ID Pelanggan"), Path: "body.payload.party_id", Type: "string", Widget: "text", Required: true},
			{Key: "party_name", Label: "Customer Name", LabelI18n: localize("Customer Name", "Nama Pelanggan"), Path: "body.payload.party_name", Type: "string", Widget: "text", Required: true},
			{Key: "refund_date", Label: "Refund Date", LabelI18n: localize("Refund Date", "Tanggal Refund"), Path: "body.payload.refund_date", Type: "string", Widget: "text", Required: true},
			{Key: "payment_method_code", Label: "Payment Method", LabelI18n: localize("Payment Method", "Metode Pembayaran"), Path: "body.payload.payment_method_code", Type: "string", Widget: "text", Required: true},
			{Key: "clearing_account_code", Label: "Clearing Account", LabelI18n: localize("Clearing Account", "Akun Clearing"), Path: "body.payload.clearing_account_code", Type: "string", Widget: "text"},
			{Key: "refund_reference", Label: "Reference", LabelI18n: localize("Reference", "Referensi"), Path: "body.payload.refund_reference", Type: "string", Widget: "text"},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string", Widget: "text", Required: true},
			{Key: "amount_refunded", Label: "Amount Refunded", LabelI18n: localize("Amount Refunded", "Jumlah Refund"), Path: "body.payload.amount_refunded", Type: "number", Required: true},
			{Key: "source_credit_note_id", Label: "Source Credit Note ID", LabelI18n: localize("Source Credit Note ID", "ID Nota Kredit Sumber"), Path: "body.payload.source_credit_note_id", Type: "string", Widget: "text", Required: true},
			{Key: "source_credit_note_number", Label: "Source Credit Note", LabelI18n: localize("Source Credit Note", "Nota Kredit Sumber"), Path: "body.payload.source_credit_note_number", Type: "string", Widget: "text"},
			{Key: "source_invoice_id", Label: "Source Invoice ID", LabelI18n: localize("Source Invoice ID", "ID Faktur Sumber"), Path: "body.payload.source_invoice_id", Type: "string", Widget: "text", Required: true},
			{Key: "source_invoice_number", Label: "Source Invoice", LabelI18n: localize("Source Invoice", "Faktur Sumber"), Path: "body.payload.source_invoice_number", Type: "string", Widget: "text"},
			{Key: "refund_allocations", Label: "Refund Allocations", LabelI18n: localize("Refund Allocations", "Alokasi Refund"), Path: "body.payload.refund_allocations", Type: "object", Widget: "commercial_refund_allocations", HelpText: "Split the refund across one or more original receipts."},
			{Key: "receivable_account_code", Label: "Receivable Account", LabelI18n: localize("Receivable Account", "Akun Piutang"), Path: "body.payload.receivable_account_code", Type: "string", Widget: "text"},
			{Key: "reason", Label: "Reason", LabelI18n: localize("Reason", "Alasan"), Path: "body.payload.reason", Type: "string", Widget: "textarea"},
			{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
		}},
	}
}

func ledgerFormSections() []module.SectionDefinition {
	return []module.SectionDefinition{
		{Key: "posting", Title: "Posting Details", TitleI18n: localize("Posting Details", "Detail Posting"), Fields: []module.FieldDefinition{
			{Key: "source_document_type", Label: "Source Document Type", LabelI18n: localize("Source Document Type", "Tipe Dokumen Sumber"), Path: "body.payload.source_document_type", Type: "string", Widget: "text", Required: true},
			{Key: "source_document_id", Label: "Source Document ID", LabelI18n: localize("Source Document ID", "ID Dokumen Sumber"), Path: "body.payload.source_document_id", Type: "string", Widget: "text", Required: true},
			{Key: "posting_date", Label: "Posting Date", LabelI18n: localize("Posting Date", "Tanggal Posting"), Path: "body.payload.posting_date", Type: "string", Widget: "text", Required: true},
			{Key: "currency_code", Label: "Currency", LabelI18n: localize("Currency", "Mata Uang"), Path: "body.payload.currency_code", Type: "string", Widget: "text", Required: true},
			{Key: "posting_rule_key", Label: "Posting Rule", LabelI18n: localize("Posting Rule", "Aturan Posting"), Path: "body.payload.posting_rule_key", Type: "string", Widget: "text"},
			{Key: "journal_source_kind", Label: "Journal Source Kind", LabelI18n: localize("Journal Source Kind", "Jenis Sumber Jurnal"), Path: "body.payload.journal_source_kind", Type: "string", Widget: "select", Options: []string{"manual", "recurring", "accrual", "reversal"}},
			{Key: "journal_template_id", Label: "Journal Template ID", LabelI18n: localize("Journal Template ID", "ID Template Jurnal"), Path: "body.payload.journal_template_id", Type: "string", Widget: "text"},
			{Key: "journal_run_id", Label: "Journal Run ID", LabelI18n: localize("Journal Run ID", "ID Run Jurnal"), Path: "body.payload.journal_run_id", Type: "string", Widget: "text"},
			{Key: "accounting_period_id", Label: "Accounting Period ID", LabelI18n: localize("Accounting Period ID", "ID Periode Akuntansi"), Path: "body.payload.accounting_period_id", Type: "string", Widget: "text"},
			{Key: "reversal_of_posting_id", Label: "Reversal Of Posting ID", LabelI18n: localize("Reversal Of Posting ID", "ID Posting Asal Reversal"), Path: "body.payload.reversal_of_posting_id", Type: "string", Widget: "text"},
			{Key: "reversed_by_posting_id", Label: "Reversed By Posting ID", LabelI18n: localize("Reversed By Posting ID", "ID Posting Reversal"), Path: "body.payload.reversed_by_posting_id", Type: "string", Widget: "text"},
			{Key: "reversal_status", Label: "Reversal Status", LabelI18n: localize("Reversal Status", "Status Reversal"), Path: "body.payload.reversal_status", Type: "string", Widget: "text"},
			{Key: "total_amount", Label: "Total", LabelI18n: localize("Total", "Total"), Path: "body.payload.total_amount", Type: "number"},
			{Key: "journal_lines", Label: "Journal Lines", LabelI18n: localize("Journal Lines", "Baris Jurnal"), Path: "body.payload.journal_lines", Type: "object", Widget: "commercial_journal_lines", HelpText: "Enter debit and credit lines directly."},
			{Key: "notes", Label: "Notes", LabelI18n: localize("Notes", "Catatan"), Path: "body.payload.notes", Type: "string", Widget: "textarea"},
		}},
	}
}

func modelPermissionPrefix(modelKey string) string {
	switch modelKey {
	case "commercial_product":
		return "product"
	case "commercial_variant_dimension":
		return "variant_dimension"
	case "commercial_variant_value":
		return "variant_value"
	case "commercial_item":
		return "item"
	case "commercial_item_category":
		return "item_category"
	case "commercial_uom":
		return "uom"
	case "commercial_tax_code":
		return "tax_code"
	case "commercial_tax_profile":
		return "tax_profile"
	case "commercial_price_list":
		return "price_list"
	case "commercial_price_list_item":
		return "price_list_item"
	case "commercial_account":
		return "account"
	case "payment_method":
		return "payment_method"
	case "vendor_profile":
		return "vendor"
	default:
		return modelKey
	}
}

func humanizeTitle(key string) string {
	switch key {
	case "party_name":
		return "Customer"
	case "payment_method_code":
		return "Payment Method"
	case "tax_profile_code":
		return "Tax Profile"
	case "payment_term_days":
		return "Payment Terms"
	case "source_document_type":
		return "Source Type"
	case "source_document_id":
		return "Source ID"
	case "posting_rule_key":
		return "Posting Rule"
	case "lines":
		return "Line Items"
	case "allocations":
		return "Allocations"
	case "tax_summary":
		return "Tax Summary"
	case "journal_lines":
		return "Journal Lines"
	default:
		return key
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
