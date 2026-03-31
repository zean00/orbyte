package config

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/secretstore"
	"orbyte/internal/platform/shared"
)

type AuthPolicy struct {
	PasswordMinLength                    int
	PasswordRequireUppercase             bool
	PasswordRequireNumber                bool
	PasswordRequireSpecial               bool
	PasswordMaxAge                       time.Duration
	SessionTTL                           time.Duration
	SessionIdleTimeout                   time.Duration
	SessionRefreshWindow                 time.Duration
	LoginRateLimitAttempts               int
	LoginRateLimitWindow                 time.Duration
	TrustedOrigins                       []string
	PasswordEnabled                      bool
	TOTPEnabled                          bool
	TOTPEnrollmentAllowed                bool
	TOTPIssuer                           string
	TOTPLoginMode                        string
	TOTPApprovalMode                     string
	TOTPStepUpTTL                        time.Duration
	LoginTitle                           string
	LoginSubtitle                        string
	GoogleButtonLabel                    string
	GoogleEnabled                        bool
	GoogleAutoProvisionEnabled           bool
	GoogleAutoProvisionAllowedDomains    []string
	GoogleAutoProvisionRoleID            string
	GoogleAutoProvisionScopeType         string
	GoogleAutoProvisionScopeID           string
	GoogleAutoProvisionDefaultLocationID string
	GoogleClientID                       string
	GoogleClientSecret                   string
	GoogleRedirectURL                    string
	GoogleAuthURL                        string
	GoogleTokenURL                       string
	GoogleIssuer                         string
	GoogleJWKSURL                        string
	GoogleHostedDomain                   string
	GoogleTimeout                        time.Duration
}

type TypesensePolicy struct {
	Enabled        bool
	Endpoint       string
	APIKey         string
	TimeoutSeconds int
}

type NATSPolicy struct {
	Enabled        bool
	URL            string
	SinkName       string
	SubjectPrefix  string
	TimeoutSeconds int
}

type EmbeddingPolicy struct {
	Provider   string
	Dimensions int
}

const (
	defaultPasswordMinLength      = 8
	defaultSessionTTL             = 8 * time.Hour
	defaultSessionRefreshWindow   = time.Hour
	defaultLoginRateLimitAttempts = 5
	defaultLoginRateLimitWindow   = 5 * time.Minute
)

type Service struct {
	repo        Repository
	definitions map[string]Definition
	secrets     *secretstore.Service
}

func NewService() *Service {
	svc := NewServiceWithRepositoryAndSecrets(NewMemoryRepository(nil), secretstore.NewService())
	for _, def := range BuiltInDefinitions() {
		_ = svc.RegisterDefinition(def)
	}
	now := time.Now().UTC()
	for _, entry := range BuiltInEntries(now) {
		_ = svc.Save(entry)
	}
	return svc
}

func NewServiceWithRepository(repo Repository) *Service {
	return NewServiceWithRepositoryAndSecrets(repo, secretstore.NewService())
}

func NewServiceWithRepositoryAndSecrets(repo Repository, secrets *secretstore.Service) *Service {
	svc := &Service{repo: repo, definitions: map[string]Definition{}, secrets: secrets}
	for _, def := range BuiltInDefinitions() {
		_ = svc.RegisterDefinition(def)
	}
	return svc
}

func BuiltInDefinitions() []Definition {
	return []Definition{{
		Key:             "platform.http",
		ModuleKey:       "platform.core",
		Category:        "platform",
		DisplayName:     "HTTP Settings",
		DisplayNameI18n: i18n.LocalizedText{"en": "HTTP Settings", "id": "Pengaturan HTTP"},
		Description:     "Platform HTTP listener settings.",
		DescriptionI18n: i18n.LocalizedText{"en": "Platform HTTP listener settings.", "id": "Pengaturan listener HTTP platform."},
		AllowedScopes:   []string{"deployment"},
		DefaultValue:    map[string]any{"address": ":8080"},
		Fields: []FieldDefinition{{
			Key: "address", Label: "Address", LabelI18n: i18n.LocalizedText{"en": "Address", "id": "Alamat"}, Type: "string", Required: true, Description: "HTTP bind address.", DescriptionI18n: i18n.LocalizedText{"en": "HTTP bind address.", "id": "Alamat bind HTTP."},
		}},
	}, {
		Key:             "platform.acp",
		ModuleKey:       "platform.core",
		Category:        "platform",
		DisplayName:     "ACP Integration",
		DisplayNameI18n: i18n.LocalizedText{"en": "ACP Integration", "id": "Integrasi ACP"},
		Description:     "Deployment-scoped ACP provider configuration for shell-native agent sessions.",
		DescriptionI18n: i18n.LocalizedText{"en": "Deployment-scoped ACP provider configuration for shell-native agent sessions.", "id": "Konfigurasi penyedia ACP pada scope deployment untuk sesi agen native shell."},
		AllowedScopes:   []string{"deployment"},
		DefaultValue: map[string]any{
			"enabled":        false,
			"providers_json": "[]",
		},
		Fields: []FieldDefinition{
			{Key: "enabled", Label: "Enabled", LabelI18n: i18n.LocalizedText{"en": "Enabled", "id": "Aktif"}, Type: "bool"},
			{Key: "providers_json", Label: "Providers JSON", LabelI18n: i18n.LocalizedText{"en": "Providers JSON", "id": "JSON Penyedia"}, Type: "string"},
		},
	}, {
		Key:             "platform.mcp",
		ModuleKey:       "platform.core",
		Category:        "platform",
		DisplayName:     "MCP Runtime",
		DisplayNameI18n: i18n.LocalizedText{"en": "MCP Runtime", "id": "Runtime MCP"},
		Description:     "Deployment-scoped MCP runtime and tool exposure settings.",
		DescriptionI18n: i18n.LocalizedText{"en": "Deployment-scoped MCP runtime and tool exposure settings.", "id": "Pengaturan runtime MCP dan eksposur tool pada scope deployment."},
		AllowedScopes:   []string{"deployment"},
		DefaultValue: map[string]any{
			"enabled":                            true,
			"governance_enabled":                 true,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
		},
		Fields: []FieldDefinition{
			{Key: "enabled", Label: "Enabled", LabelI18n: i18n.LocalizedText{"en": "Enabled", "id": "Aktif"}, Type: "bool"},
			{Key: "governance_enabled", Label: "Governance Enabled", LabelI18n: i18n.LocalizedText{"en": "Governance Enabled", "id": "Governance Aktif"}, Type: "bool"},
			{Key: "default_action_mode", Label: "Default Action Mode", LabelI18n: i18n.LocalizedText{"en": "Default Action Mode", "id": "Mode Aksi Default"}, Type: "string"},
			{Key: "tool_states_json", Label: "Tool States JSON", LabelI18n: i18n.LocalizedText{"en": "Tool States JSON", "id": "JSON Status Tool"}, Type: "string"},
			{Key: "blocked_action_classes_json", Label: "Blocked Action Classes JSON", LabelI18n: i18n.LocalizedText{"en": "Blocked Action Classes JSON", "id": "JSON Kelas Aksi Diblokir"}, Type: "string"},
			{Key: "blocked_tool_keys_json", Label: "Blocked Tool Keys JSON", LabelI18n: i18n.LocalizedText{"en": "Blocked Tool Keys JSON", "id": "JSON Kunci Tool Diblokir"}, Type: "string"},
			{Key: "blocked_document_types_json", Label: "Blocked Document Types JSON", LabelI18n: i18n.LocalizedText{"en": "Blocked Document Types JSON", "id": "JSON Tipe Dokumen Diblokir"}, Type: "string"},
			{Key: "allowed_submit_document_types_json", Label: "Allowed Submit Document Types JSON", LabelI18n: i18n.LocalizedText{"en": "Allowed Submit Document Types JSON", "id": "JSON Tipe Dokumen Submit Diizinkan"}, Type: "string"},
			{Key: "domain_policy_overrides_json", Label: "Domain Policy Overrides JSON", LabelI18n: i18n.LocalizedText{"en": "Domain Policy Overrides JSON", "id": "JSON Override Kebijakan Domain"}, Type: "string"},
		},
	}, {
		Key:             "platform.db",
		ModuleKey:       "platform.core",
		Category:        "platform",
		DisplayName:     "Database Instrumentation",
		DisplayNameI18n: i18n.LocalizedText{"en": "Database Instrumentation", "id": "Instrumentasi Database"},
		Description:     "Database observability and named read-strategy settings.",
		DescriptionI18n: i18n.LocalizedText{"en": "Database observability and named read-strategy settings.", "id": "Pengaturan observabilitas database dan strategi baca bernama."},
		AllowedScopes:   []string{"deployment"},
		DefaultValue: map[string]any{
			"slow_query_threshold_ms": 250,
			"top_operations_limit":    20,
			"slow_queries_limit":      50,
			"read_strategies_json":    "{}",
		},
		Fields: []FieldDefinition{
			{Key: "slow_query_threshold_ms", Label: "Slow Query Threshold (ms)", LabelI18n: i18n.LocalizedText{"en": "Slow Query Threshold (ms)", "id": "Ambang Query Lambat (ms)"}, Type: "int"},
			{Key: "top_operations_limit", Label: "Top Operations Limit", LabelI18n: i18n.LocalizedText{"en": "Top Operations Limit", "id": "Batas Operasi Teratas"}, Type: "int"},
			{Key: "slow_queries_limit", Label: "Slow Queries Limit", LabelI18n: i18n.LocalizedText{"en": "Slow Queries Limit", "id": "Batas Query Lambat"}, Type: "int"},
			{Key: "read_strategies_json", Label: "Read Strategies JSON", LabelI18n: i18n.LocalizedText{"en": "Read Strategies JSON", "id": "JSON Strategi Baca"}, Type: "string"},
		},
	}, {
		Key:             "search.typesense",
		ModuleKey:       "platform.core",
		Category:        "search",
		DisplayName:     "Typesense Search",
		DisplayNameI18n: i18n.LocalizedText{"en": "Typesense Search", "id": "Pencarian Typesense"},
		Description:     "Typesense endpoint and runtime settings.",
		DescriptionI18n: i18n.LocalizedText{"en": "Typesense endpoint and runtime settings.", "id": "Pengaturan endpoint dan runtime Typesense."},
		AllowedScopes:   []string{"deployment"},
		DefaultValue: map[string]any{
			"enabled":         false,
			"endpoint":        "",
			"api_key":         "",
			"timeout_seconds": 5,
		},
		Fields: []FieldDefinition{
			{Key: "enabled", Label: "Enabled", LabelI18n: i18n.LocalizedText{"en": "Enabled", "id": "Aktif"}, Type: "bool"},
			{Key: "endpoint", Label: "Endpoint", LabelI18n: i18n.LocalizedText{"en": "Endpoint", "id": "Endpoint"}, Type: "string"},
			{Key: "api_key", Label: "API Key", LabelI18n: i18n.LocalizedText{"en": "API Key", "id": "Kunci API"}, Type: "string", Sensitive: true},
			{Key: "timeout_seconds", Label: "Timeout Seconds", LabelI18n: i18n.LocalizedText{"en": "Timeout Seconds", "id": "Detik Timeout"}, Type: "int"},
		},
	}, {
		Key:             "eventing.nats",
		ModuleKey:       "platform.core",
		Category:        "eventing",
		DisplayName:     "NATS Eventing",
		DisplayNameI18n: i18n.LocalizedText{"en": "NATS Eventing", "id": "Eventing NATS"},
		Description:     "NATS broker settings for external event publication.",
		DescriptionI18n: i18n.LocalizedText{"en": "NATS broker settings for external event publication.", "id": "Pengaturan broker NATS untuk publikasi event eksternal."},
		AllowedScopes:   []string{"deployment"},
		DefaultValue: map[string]any{
			"enabled":         false,
			"url":             "",
			"sink_name":       "nats",
			"subject_prefix":  "",
			"timeout_seconds": 5,
		},
		Fields: []FieldDefinition{
			{Key: "enabled", Label: "Enabled", LabelI18n: i18n.LocalizedText{"en": "Enabled", "id": "Aktif"}, Type: "bool"},
			{Key: "url", Label: "URL", LabelI18n: i18n.LocalizedText{"en": "URL", "id": "URL"}, Type: "string"},
			{Key: "sink_name", Label: "Sink Name", LabelI18n: i18n.LocalizedText{"en": "Sink Name", "id": "Nama Sink"}, Type: "string"},
			{Key: "subject_prefix", Label: "Subject Prefix", LabelI18n: i18n.LocalizedText{"en": "Subject Prefix", "id": "Prefix Subject"}, Type: "string"},
			{Key: "timeout_seconds", Label: "Timeout Seconds", LabelI18n: i18n.LocalizedText{"en": "Timeout Seconds", "id": "Detik Timeout"}, Type: "int"},
		},
	}, {
		Key:             "search.embedding",
		ModuleKey:       "platform.core",
		Category:        "search",
		DisplayName:     "Embedding Settings",
		DisplayNameI18n: i18n.LocalizedText{"en": "Embedding Settings", "id": "Pengaturan Embedding"},
		Description:     "External embedding provider defaults.",
		DescriptionI18n: i18n.LocalizedText{"en": "External embedding provider defaults.", "id": "Nilai baku penyedia embedding eksternal."},
		AllowedScopes:   []string{"deployment"},
		DefaultValue: map[string]any{
			"provider":   "hash",
			"dimensions": 8,
		},
		Fields: []FieldDefinition{
			{Key: "provider", Label: "Provider", LabelI18n: i18n.LocalizedText{"en": "Provider", "id": "Penyedia"}, Type: "string"},
			{Key: "dimensions", Label: "Dimensions", LabelI18n: i18n.LocalizedText{"en": "Dimensions", "id": "Dimensi"}, Type: "int"},
		},
	}, {
		Key:             "commercial.posting",
		ModuleKey:       "commercial_core",
		Category:        "finance",
		DisplayName:     "Commercial Posting Defaults",
		DisplayNameI18n: i18n.LocalizedText{"en": "Commercial Posting Defaults", "id": "Default Posting Komersial"},
		Description:     "Fallback account configuration for commercial invoice and payment postings.",
		DescriptionI18n: i18n.LocalizedText{"en": "Fallback account configuration for commercial invoice and payment postings.", "id": "Konfigurasi akun fallback untuk posting faktur dan pembayaran komersial."},
		AllowedScopes:   []string{"deployment", "organization", "location"},
		DefaultValue: map[string]any{
			"invoice_issue_receivable_account_code":   "1100-AR",
			"invoice_issue_revenue_account_code":      "4000-REV",
			"invoice_issue_tax_account_code":          "2100-TAX",
			"payment_receipt_clearing_account_code":   "1000-CASH",
			"payment_receipt_receivable_account_code": "1100-AR",
		},
		Fields: []FieldDefinition{
			{Key: "invoice_issue_receivable_account_code", Label: "Invoice Issue Receivable Account", Type: "string"},
			{Key: "invoice_issue_revenue_account_code", Label: "Invoice Issue Revenue Account", Type: "string"},
			{Key: "invoice_issue_tax_account_code", Label: "Invoice Issue Tax Account", Type: "string"},
			{Key: "payment_receipt_clearing_account_code", Label: "Payment Receipt Clearing Account", Type: "string"},
			{Key: "payment_receipt_receivable_account_code", Label: "Payment Receipt Receivable Account", Type: "string"},
		},
	}, {
		Key:             "procurement.posting",
		ModuleKey:       "procurement_core",
		Category:        "finance",
		DisplayName:     "Procurement Posting Defaults",
		DisplayNameI18n: i18n.LocalizedText{"en": "Procurement Posting Defaults", "id": "Default Posting Pengadaan"},
		Description:     "Fallback account configuration for vendor bill, payment out, and vendor credit postings.",
		DescriptionI18n: i18n.LocalizedText{"en": "Fallback account configuration for vendor bill, payment out, and vendor credit postings.", "id": "Konfigurasi akun fallback untuk posting tagihan vendor, pembayaran keluar, dan nota kredit vendor."},
		AllowedScopes:   []string{"deployment", "organization", "location"},
		DefaultValue: map[string]any{
			"vendor_bill_payable_account_code":    "2000-AP",
			"vendor_bill_expense_account_code":    "5000-EXP",
			"vendor_bill_tax_account_code":        "2100-TAX",
			"payment_out_clearing_account_code":   "1000-CASH",
			"payment_out_payable_account_code":    "2000-AP",
			"vendor_credit_payable_account_code":  "2000-AP",
			"vendor_credit_expense_account_code":  "5000-EXP",
			"vendor_credit_tax_account_code":      "2100-TAX",
			"goods_receipt_clearing_account_code": "2050-GRIR",
		},
		Fields: []FieldDefinition{
			{Key: "vendor_bill_payable_account_code", Label: "Vendor Bill Payable Account", Type: "string"},
			{Key: "vendor_bill_expense_account_code", Label: "Vendor Bill Expense Account", Type: "string"},
			{Key: "vendor_bill_tax_account_code", Label: "Vendor Bill Tax Account", Type: "string"},
			{Key: "payment_out_clearing_account_code", Label: "Payment Out Clearing Account", Type: "string"},
			{Key: "payment_out_payable_account_code", Label: "Payment Out Payable Account", Type: "string"},
			{Key: "vendor_credit_payable_account_code", Label: "Vendor Credit Payable Account", Type: "string"},
			{Key: "vendor_credit_expense_account_code", Label: "Vendor Credit Expense Account", Type: "string"},
			{Key: "vendor_credit_tax_account_code", Label: "Vendor Credit Tax Account", Type: "string"},
			{Key: "goods_receipt_clearing_account_code", Label: "Goods Receipt Clearing Account", Type: "string"},
		},
	}, {
		Key:             "discount.policy",
		ModuleKey:       "discount_core",
		Category:        "commercial",
		DisplayName:     "Discount Policy",
		DisplayNameI18n: i18n.LocalizedText{"en": "Discount Policy", "id": "Kebijakan Diskon"},
		Description:     "Global discount stacking and evaluation defaults.",
		DescriptionI18n: i18n.LocalizedText{"en": "Global discount stacking and evaluation defaults.", "id": "Nilai baku global untuk stacking dan evaluasi diskon."},
		AllowedScopes:   []string{"deployment", "organization", "location"},
		DefaultValue: map[string]any{
			"stacking_mode": "best_one_only",
			"time_zone":     "Asia/Jakarta",
		},
		Fields: []FieldDefinition{
			{Key: "stacking_mode", Label: "Stacking Mode", Type: "string", Required: true},
			{Key: "time_zone", Label: "Time Zone", Type: "string"},
		},
	}, {
		Key:             "finance_asset.posting",
		ModuleKey:       "finance_asset_core",
		Category:        "finance",
		DisplayName:     "Finance Asset Posting Defaults",
		DisplayNameI18n: i18n.LocalizedText{"en": "Finance Asset Posting Defaults", "id": "Default Posting Aset Keuangan"},
		Description:     "Fallback account configuration for fixed assets and prepaid schedules.",
		DescriptionI18n: i18n.LocalizedText{"en": "Fallback account configuration for fixed assets and prepaid schedules.", "id": "Konfigurasi akun fallback untuk aset tetap dan jadwal biaya dibayar dimuka."},
		AllowedScopes:   []string{"deployment", "organization", "location"},
		DefaultValue: map[string]any{
			"fixed_asset_default_asset_account_code":                    "1500-FA",
			"fixed_asset_default_accumulated_depreciation_account_code": "1590-ACC-DEPR",
			"fixed_asset_default_depreciation_expense_account_code":     "6100-DEPR",
			"fixed_asset_default_accumulated_impairment_account_code":   "1595-ACC-IMP",
			"fixed_asset_default_impairment_expense_account_code":       "6210-IMP",
			"fixed_asset_default_revaluation_reserve_account_code":      "3105-FA-REVAL",
			"fixed_asset_default_revaluation_loss_account_code":         "6215-FA-REVAL-LOSS",
			"fixed_asset_disposal_gain_account_code":                    "7200-FA-GAIN",
			"fixed_asset_disposal_loss_account_code":                    "6205-FA-LOSS",
			"fixed_asset_sale_proceeds_account_code":                    "1000-CASH",
			"prepaid_default_asset_account_code":                        "1600-PREPAID",
			"prepaid_default_expense_account_code":                      "6200-AMORT",
		},
		Fields: []FieldDefinition{
			{Key: "fixed_asset_default_asset_account_code", Label: "Fixed Asset Account", Type: "string"},
			{Key: "fixed_asset_default_accumulated_depreciation_account_code", Label: "Accumulated Depreciation Account", Type: "string"},
			{Key: "fixed_asset_default_depreciation_expense_account_code", Label: "Depreciation Expense Account", Type: "string"},
			{Key: "fixed_asset_default_accumulated_impairment_account_code", Label: "Accumulated Impairment Account", Type: "string"},
			{Key: "fixed_asset_default_impairment_expense_account_code", Label: "Impairment Expense Account", Type: "string"},
			{Key: "fixed_asset_default_revaluation_reserve_account_code", Label: "Revaluation Reserve Account", Type: "string"},
			{Key: "fixed_asset_default_revaluation_loss_account_code", Label: "Revaluation Loss Account", Type: "string"},
			{Key: "fixed_asset_disposal_gain_account_code", Label: "Disposal Gain Account", Type: "string"},
			{Key: "fixed_asset_disposal_loss_account_code", Label: "Disposal Loss Account", Type: "string"},
			{Key: "fixed_asset_sale_proceeds_account_code", Label: "Sale Proceeds Account", Type: "string"},
			{Key: "prepaid_default_asset_account_code", Label: "Prepaid Asset Account", Type: "string"},
			{Key: "prepaid_default_expense_account_code", Label: "Prepaid Expense Account", Type: "string"},
		},
	}, {
		Key:             "retail_finance.posting",
		ModuleKey:       "retail_finance_core",
		Category:        "finance",
		DisplayName:     "Retail Finance Posting Defaults",
		DisplayNameI18n: i18n.LocalizedText{"en": "Retail Finance Posting Defaults", "id": "Default Posting Keuangan Retail"},
		Description:     "Fallback account configuration for POS over/short and stored-value liability postings.",
		DescriptionI18n: i18n.LocalizedText{"en": "Fallback account configuration for POS over/short and stored-value liability postings.", "id": "Konfigurasi akun fallback untuk posting lebih/kurang kas POS dan liabilitas stored-value."},
		AllowedScopes:   []string{"deployment", "organization", "location"},
		DefaultValue: map[string]any{
			"cash_over_gain_account_code":         "4890-CASH-OVER",
			"cash_over_short_loss_account_code":   "5890-CASH-SHORT",
			"gift_card_liability_account_code":    "2250-GIFT-CARD",
			"store_credit_liability_account_code": "2260-STORE-CREDIT",
		},
		Fields: []FieldDefinition{
			{Key: "cash_over_gain_account_code", Label: "Cash Over Gain Account", Type: "string"},
			{Key: "cash_over_short_loss_account_code", Label: "Cash Short Loss Account", Type: "string"},
			{Key: "gift_card_liability_account_code", Label: "Gift Card Liability Account", Type: "string"},
			{Key: "store_credit_liability_account_code", Label: "Store Credit Liability Account", Type: "string"},
		},
	}, {
		Key:             "treasury.posting",
		ModuleKey:       "treasury_core",
		Category:        "finance",
		DisplayName:     "Treasury Posting Defaults",
		DisplayNameI18n: i18n.LocalizedText{"en": "Treasury Posting Defaults", "id": "Default Posting Treasury"},
		Description:     "Fallback account configuration for treasury transfers, bank fees, interest, and suspense handling.",
		DescriptionI18n: i18n.LocalizedText{"en": "Fallback account configuration for treasury transfers, bank fees, interest, and suspense handling.", "id": "Konfigurasi akun fallback untuk transfer treasury, biaya bank, bunga, dan suspense."},
		AllowedScopes:   []string{"deployment", "organization", "location"},
		DefaultValue: map[string]any{
			"bank_transfer_clearing_account_code": "1095-TRANSFER-CLEARING",
			"bank_fee_expense_account_code":       "6300-BANK-FEE",
			"bank_interest_income_account_code":   "4800-BANK-INT",
			"treasury_suspense_account_code":      "1099-TREASURY-SUSPENSE",
		},
		Fields: []FieldDefinition{
			{Key: "bank_transfer_clearing_account_code", Label: "Transfer Clearing Account", Type: "string"},
			{Key: "bank_fee_expense_account_code", Label: "Bank Fee Expense Account", Type: "string"},
			{Key: "bank_interest_income_account_code", Label: "Bank Interest Income Account", Type: "string"},
			{Key: "treasury_suspense_account_code", Label: "Treasury Suspense Account", Type: "string"},
		},
	}, {
		Key:             "inventory.policy",
		ModuleKey:       "inventory_core",
		Category:        "inventory",
		DisplayName:     "Inventory Policy",
		DisplayNameI18n: i18n.LocalizedText{"en": "Inventory Policy", "id": "Kebijakan Inventori"},
		Description:     "Inventory control defaults for near-expiry and batch operations.",
		DescriptionI18n: i18n.LocalizedText{"en": "Inventory control defaults for near-expiry and batch operations.", "id": "Nilai baku kontrol inventori untuk hampir kedaluwarsa dan operasi batch."},
		AllowedScopes:   []string{"deployment", "organization", "location"},
		DefaultValue: map[string]any{
			"near_expiry_days": 30,
		},
		Fields: []FieldDefinition{
			{Key: "near_expiry_days", Label: "Near Expiry Days", Type: "int", Required: true},
		},
	}, {
		Key:             "identity.auth",
		ModuleKey:       "identity",
		Category:        "security",
		DisplayName:     "Authentication Policy",
		DisplayNameI18n: i18n.LocalizedText{"en": "Authentication Policy", "id": "Kebijakan Autentikasi"},
		Description:     "Authentication, session, and login throttling policy.",
		DescriptionI18n: i18n.LocalizedText{"en": "Authentication, session, and login throttling policy.", "id": "Kebijakan autentikasi, sesi, dan pembatasan login."},
		AllowedScopes:   []string{"deployment", "organization", "location"},
		DefaultValue: map[string]any{
			"password_min_length":                       defaultPasswordMinLength,
			"password_require_uppercase":                false,
			"password_require_number":                   false,
			"password_require_special":                  false,
			"password_max_age_days":                     0,
			"session_ttl_minutes":                       int(defaultSessionTTL / time.Minute),
			"session_idle_timeout_minutes":              0,
			"session_refresh_window_minutes":            int(defaultSessionRefreshWindow / time.Minute),
			"login_rate_limit_attempts":                 defaultLoginRateLimitAttempts,
			"login_rate_limit_window_seconds":           int(defaultLoginRateLimitWindow / time.Second),
			"trusted_origins":                           []string{},
			"password_enabled":                          true,
			"totp_enabled":                              false,
			"totp_enrollment_allowed":                   true,
			"totp_issuer":                               "Orbyte",
			"totp_login_mode":                           "optional",
			"totp_approval_mode":                        "optional",
			"totp_step_up_ttl_minutes":                  10,
			"login_title":                               "Platform Access",
			"login_subtitle":                            "Sign in to continue.",
			"google_button_label":                       "Continue with Google",
			"google_enabled":                            false,
			"google_auto_provision_enabled":             false,
			"google_auto_provision_allowed_domains":     []string{},
			"google_auto_provision_role_id":             "",
			"google_auto_provision_scope_type":          "deployment",
			"google_auto_provision_scope_id":            "",
			"google_auto_provision_default_location_id": "",
			"google_client_id":                          "",
			"google_client_secret":                      "",
			"google_redirect_url":                       "",
			"google_auth_url":                           "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                          "https://oauth2.googleapis.com/token",
			"google_issuer":                             "https://accounts.google.com",
			"google_jwks_url":                           "https://www.googleapis.com/oauth2/v3/certs",
			"google_hosted_domain":                      "",
			"google_timeout_seconds":                    5,
		},
		Fields: []FieldDefinition{
			{Key: "password_min_length", Label: "Password Min Length", Type: "int", Required: true},
			{Key: "password_require_uppercase", Label: "Password Requires Uppercase", Type: "bool"},
			{Key: "password_require_number", Label: "Password Requires Number", Type: "bool"},
			{Key: "password_require_special", Label: "Password Requires Special Character", Type: "bool"},
			{Key: "password_max_age_days", Label: "Password Max Age Days", Type: "int", Required: true},
			{Key: "session_ttl_minutes", Label: "Session TTL Minutes", Type: "int", Required: true},
			{Key: "session_idle_timeout_minutes", Label: "Session Idle Timeout Minutes", Type: "int", Required: true},
			{Key: "session_refresh_window_minutes", Label: "Session Refresh Window Minutes", Type: "int", Required: true},
			{Key: "login_rate_limit_attempts", Label: "Login Rate Limit Attempts", Type: "int", Required: true},
			{Key: "login_rate_limit_window_seconds", Label: "Login Rate Limit Window Seconds", Type: "int", Required: true},
			{Key: "trusted_origins", Label: "Trusted Origins", Type: "string_list"},
			{Key: "password_enabled", Label: "Password Enabled", Type: "bool"},
			{Key: "totp_enabled", Label: "2FA Enabled", Type: "bool"},
			{Key: "totp_enrollment_allowed", Label: "2FA Enrollment Allowed", Type: "bool"},
			{Key: "totp_issuer", Label: "2FA Issuer", Type: "string"},
			{Key: "totp_login_mode", Label: "2FA Login Mode", Type: "string", Enum: []string{"disabled", "optional", "required"}},
			{Key: "totp_approval_mode", Label: "2FA Approval Mode", Type: "string", Enum: []string{"disabled", "optional", "required"}},
			{Key: "totp_step_up_ttl_minutes", Label: "2FA Approval Step-Up TTL Minutes", Type: "int", Required: true},
			{Key: "login_title", Label: "Login Title", Type: "string"},
			{Key: "login_subtitle", Label: "Login Subtitle", Type: "string"},
			{Key: "google_button_label", Label: "Google Button Label", Type: "string"},
			{Key: "google_enabled", Label: "Google Enabled", Type: "bool"},
			{Key: "google_auto_provision_enabled", Label: "Google Auto Provision Enabled", Type: "bool"},
			{Key: "google_auto_provision_allowed_domains", Label: "Google Auto Provision Allowed Domains", Type: "string_list"},
			{Key: "google_auto_provision_role_id", Label: "Google Auto Provision Role ID", Type: "string"},
			{Key: "google_auto_provision_scope_type", Label: "Google Auto Provision Scope Type", Type: "string"},
			{Key: "google_auto_provision_scope_id", Label: "Google Auto Provision Scope ID", Type: "string"},
			{Key: "google_auto_provision_default_location_id", Label: "Google Auto Provision Default Location ID", Type: "string"},
			{Key: "google_client_id", Label: "Google Client ID", Type: "string"},
			{Key: "google_client_secret", Label: "Google Client Secret", Type: "string", Sensitive: true},
			{Key: "google_redirect_url", Label: "Google Redirect URL", Type: "string"},
			{Key: "google_auth_url", Label: "Google Auth URL", Type: "string"},
			{Key: "google_token_url", Label: "Google Token URL", Type: "string"},
			{Key: "google_issuer", Label: "Google Issuer", Type: "string"},
			{Key: "google_jwks_url", Label: "Google JWKS URL", Type: "string"},
			{Key: "google_hosted_domain", Label: "Google Hosted Domain", Type: "string"},
			{Key: "google_timeout_seconds", Label: "Google Timeout Seconds", Type: "int"},
		},
	}}
}

func BuiltInEntries(now time.Time) []Entry {
	return []Entry{{
		Key:       "platform.http",
		ModuleKey: "platform.core",
		Category:  "platform",
		Scope:     "deployment",
		ScopeID:   "",
		Value:     map[string]any{"address": ":8080"},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "platform.db",
		ModuleKey: "platform.core",
		Category:  "platform",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"slow_query_threshold_ms": 250,
			"top_operations_limit":    20,
			"slow_queries_limit":      50,
			"read_strategies_json":    "{}",
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "search.typesense",
		ModuleKey: "platform.core",
		Category:  "search",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"enabled":         false,
			"endpoint":        "",
			"api_key":         "",
			"timeout_seconds": 5,
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "eventing.nats",
		ModuleKey: "platform.core",
		Category:  "eventing",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"enabled":         false,
			"url":             "",
			"sink_name":       "nats",
			"subject_prefix":  "",
			"timeout_seconds": 5,
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "search.embedding",
		ModuleKey: "platform.core",
		Category:  "search",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"provider":   "hash",
			"dimensions": 8,
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "commercial.posting",
		ModuleKey: "commercial_core",
		Category:  "finance",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"invoice_issue_receivable_account_code":   "1100-AR",
			"invoice_issue_revenue_account_code":      "4000-REV",
			"invoice_issue_tax_account_code":          "2100-TAX",
			"payment_receipt_clearing_account_code":   "1000-CASH",
			"payment_receipt_receivable_account_code": "1100-AR",
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "procurement.posting",
		ModuleKey: "procurement_core",
		Category:  "finance",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"vendor_bill_payable_account_code":    "2000-AP",
			"vendor_bill_expense_account_code":    "5000-EXP",
			"vendor_bill_tax_account_code":        "2100-TAX",
			"payment_out_clearing_account_code":   "1000-CASH",
			"payment_out_payable_account_code":    "2000-AP",
			"vendor_credit_payable_account_code":  "2000-AP",
			"vendor_credit_expense_account_code":  "5000-EXP",
			"vendor_credit_tax_account_code":      "2100-TAX",
			"goods_receipt_clearing_account_code": "2050-GRIR",
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "inventory.policy",
		ModuleKey: "inventory_core",
		Category:  "inventory",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"near_expiry_days": 30,
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "treasury.posting",
		ModuleKey: "treasury_core",
		Category:  "finance",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"bank_transfer_clearing_account_code": "1095-TRANSFER-CLEARING",
			"bank_fee_expense_account_code":       "6300-BANK-FEE",
			"bank_interest_income_account_code":   "4800-BANK-INT",
			"treasury_suspense_account_code":      "1099-TREASURY-SUSPENSE",
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "retail_finance.posting",
		ModuleKey: "retail_finance_core",
		Category:  "finance",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"cash_over_gain_account_code":         "4890-CASH-OVER",
			"cash_over_short_loss_account_code":   "5890-CASH-SHORT",
			"gift_card_liability_account_code":    "2250-GIFT-CARD",
			"store_credit_liability_account_code": "2260-STORE-CREDIT",
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "finance_asset.posting",
		ModuleKey: "finance_asset_core",
		Category:  "finance",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"fixed_asset_default_asset_account_code":                    "1500-FA",
			"fixed_asset_default_accumulated_depreciation_account_code": "1590-ACC-DEPR",
			"fixed_asset_default_depreciation_expense_account_code":     "6100-DEPR",
			"fixed_asset_default_accumulated_impairment_account_code":   "1595-ACC-IMP",
			"fixed_asset_default_impairment_expense_account_code":       "6210-IMP",
			"fixed_asset_default_revaluation_reserve_account_code":      "3105-FA-REVAL",
			"fixed_asset_default_revaluation_loss_account_code":         "6215-FA-REVAL-LOSS",
			"fixed_asset_disposal_gain_account_code":                    "7200-FA-GAIN",
			"fixed_asset_disposal_loss_account_code":                    "6205-FA-LOSS",
			"fixed_asset_sale_proceeds_account_code":                    "1000-CASH",
			"prepaid_default_asset_account_code":                        "1600-PREPAID",
			"prepaid_default_expense_account_code":                      "6200-AMORT",
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}, {
		Key:       "identity.auth",
		ModuleKey: "identity",
		Category:  "security",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"password_min_length":                       defaultPasswordMinLength,
			"password_require_uppercase":                false,
			"password_require_number":                   false,
			"password_require_special":                  false,
			"password_max_age_days":                     0,
			"session_ttl_minutes":                       int(defaultSessionTTL / time.Minute),
			"session_idle_timeout_minutes":              0,
			"session_refresh_window_minutes":            int(defaultSessionRefreshWindow / time.Minute),
			"login_rate_limit_attempts":                 defaultLoginRateLimitAttempts,
			"login_rate_limit_window_seconds":           int(defaultLoginRateLimitWindow / time.Second),
			"trusted_origins":                           []string{},
			"password_enabled":                          true,
			"login_title":                               "Platform Access",
			"login_subtitle":                            "Sign in to continue.",
			"google_button_label":                       "Continue with Google",
			"google_enabled":                            false,
			"google_auto_provision_enabled":             false,
			"google_auto_provision_allowed_domains":     []string{},
			"google_auto_provision_role_id":             "",
			"google_auto_provision_scope_type":          "deployment",
			"google_auto_provision_scope_id":            "",
			"google_auto_provision_default_location_id": "",
			"google_client_id":                          "",
			"google_client_secret":                      "",
			"google_redirect_url":                       "",
			"google_auth_url":                           "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                          "https://oauth2.googleapis.com/token",
			"google_issuer":                             "https://accounts.google.com",
			"google_jwks_url":                           "https://www.googleapis.com/oauth2/v3/certs",
			"google_hosted_domain":                      "",
			"google_timeout_seconds":                    5,
		},
		UpdatedAt: now,
		UpdatedBy: "system",
	}}
}

func (s *Service) RegisterDefinition(def Definition) error {
	if strings.TrimSpace(def.Key) == "" {
		return shared.Validation("configuration key is required")
	}
	if strings.TrimSpace(def.ModuleKey) == "" {
		return shared.Validation("module_key is required")
	}
	if len(def.AllowedScopes) == 0 {
		def.AllowedScopes = []string{"deployment"}
	}
	if def.DefaultValue == nil {
		def.DefaultValue = map[string]any{}
	}
	s.definitions[def.Key] = def
	return nil
}

func (s *Service) Definition(key string) (Definition, bool) {
	def, ok := s.definitions[key]
	return def, ok
}

func (s *Service) Definitions() []Definition {
	items := make([]Definition, 0, len(s.definitions))
	for _, def := range s.definitions {
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) Get(key string) (Entry, bool) {
	return s.repo.Get(key, "deployment", "")
}

func (s *Service) Keys() []string {
	keys := make([]string, 0, len(s.definitions))
	for key := range s.definitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *Service) Entries() []Entry {
	return s.repo.List()
}

func (s *Service) Save(entry Entry) error {
	def, ok := s.Definition(entry.Key)
	if !ok {
		return shared.Validation("configuration key is not registered")
	}
	if entry.Scope == "" {
		entry.Scope = "deployment"
	}
	if !containsScope(def.AllowedScopes, entry.Scope) {
		return shared.Validation("configuration scope is not allowed")
	}
	if err := validateValue(entry.Value, def.Fields); err != nil {
		return err
	}
	if entry.ModuleKey == "" {
		entry.ModuleKey = def.ModuleKey
	}
	if entry.Category == "" {
		entry.Category = def.Category
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = time.Now().UTC()
	}
	if entry.UpdatedBy == "" {
		entry.UpdatedBy = "system"
	}
	if entry.Value == nil {
		entry.Value = map[string]any{}
	}
	entry.Value = s.prepareStoredValue(def, entry)
	return s.repo.Save(entry)
}

func (s *Service) SaveStored(entry Entry) error {
	def, ok := s.Definition(entry.Key)
	if !ok {
		return shared.Validation("configuration key is not registered")
	}
	if entry.Scope == "" {
		entry.Scope = "deployment"
	}
	if !containsScope(def.AllowedScopes, entry.Scope) {
		return shared.Validation("configuration scope is not allowed")
	}
	if entry.ModuleKey == "" {
		entry.ModuleKey = def.ModuleKey
	}
	if entry.Category == "" {
		entry.Category = def.Category
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = time.Now().UTC()
	}
	if entry.UpdatedBy == "" {
		entry.UpdatedBy = "system"
	}
	if entry.Value == nil {
		entry.Value = map[string]any{}
	}
	return s.repo.Save(entry)
}

func (s *Service) Resolve(key, organizationID, locationID string) (EffectiveValue, bool) {
	def, ok := s.Definition(key)
	if !ok {
		return EffectiveValue{}, false
	}
	resolved := cloneMap(def.DefaultValue)
	sourceScope := "default"
	sourceScopeID := ""
	for _, candidate := range []struct {
		scope   string
		scopeID string
	}{
		{scope: "deployment", scopeID: ""},
		{scope: "organization", scopeID: organizationID},
		{scope: "location", scopeID: locationID},
	} {
		if candidate.scopeID == "" && candidate.scope != "deployment" {
			continue
		}
		if !containsScope(def.AllowedScopes, candidate.scope) {
			continue
		}
		entry, ok := s.repo.Get(key, candidate.scope, candidate.scopeID)
		if !ok {
			continue
		}
		mergeMap(resolved, s.resolveSecrets(def, entry.Value))
		sourceScope = candidate.scope
		sourceScopeID = candidate.scopeID
	}
	return EffectiveValue{
		Key:           key,
		ModuleKey:     def.ModuleKey,
		Scope:         "effective",
		Value:         resolved,
		SourceScope:   sourceScope,
		SourceScopeID: sourceScopeID,
		ResolvedAt:    time.Now().UTC(),
	}, true
}

func (s *Service) prepareStoredValue(def Definition, entry Entry) map[string]any {
	stored := cloneMap(entry.Value)
	for _, field := range def.Fields {
		if !field.Sensitive {
			continue
		}
		raw, ok := stored[field.Key]
		if !ok {
			continue
		}
		if ref := secretRefFromValue(raw); ref != "" {
			stored[field.Key] = map[string]any{"secret_ref": ref}
			continue
		}
		text, ok := raw.(string)
		if !ok {
			continue
		}
		name := def.Key + ":" + field.Key + ":" + entry.Scope + ":" + entry.ScopeID
		secret, err := s.secrets.Upsert(name, "", text)
		if err != nil {
			continue
		}
		stored[field.Key] = map[string]any{"secret_ref": secret.Ref}
	}
	return stored
}

func (s *Service) resolveSecrets(def Definition, value map[string]any) map[string]any {
	resolved := cloneMap(value)
	for _, field := range def.Fields {
		if !field.Sensitive {
			continue
		}
		raw, ok := resolved[field.Key]
		if !ok {
			continue
		}
		ref := secretRefFromValue(raw)
		if ref == "" {
			continue
		}
		if secret, ok := s.secrets.Resolve(ref); ok {
			resolved[field.Key] = secret
		}
	}
	return resolved
}

func secretRefFromValue(raw any) string {
	switch value := raw.(type) {
	case map[string]any:
		if ref, _ := value["secret_ref"].(string); strings.TrimSpace(ref) != "" {
			return strings.TrimSpace(ref)
		}
	}
	return ""
}

func (s *Service) ResolveAll(organizationID, locationID string) []EffectiveValue {
	keys := s.Keys()
	items := make([]EffectiveValue, 0, len(keys))
	for _, key := range keys {
		if value, ok := s.Resolve(key, organizationID, locationID); ok {
			items = append(items, value)
		}
	}
	return items
}

func (s *Service) ValidateAll(organizationID, locationID string) ValidationReport {
	report := ValidationReport{Valid: true}
	for _, def := range s.Definitions() {
		value, ok := s.Resolve(def.Key, organizationID, locationID)
		if !ok {
			report.Valid = false
			report.Issues = append(report.Issues, ValidationIssue{
				Key:      def.Key,
				Severity: "error",
				Message:  "configuration definition cannot be resolved",
			})
			continue
		}
		if err := validateValue(value.Value, def.Fields); err != nil {
			report.Valid = false
			report.Issues = append(report.Issues, ValidationIssue{
				Key:      def.Key,
				Severity: "error",
				Message:  err.Error(),
			})
		}
		for _, field := range def.Fields {
			if !field.Required {
				continue
			}
			current, ok := value.Value[field.Key]
			if !ok || isZeroFieldValue(current) {
				report.Valid = false
				report.Issues = append(report.Issues, ValidationIssue{
					Key:      def.Key,
					Field:    field.Key,
					Severity: "error",
					Message:  "required field is missing or empty",
				})
			}
		}
		if def.Key == "identity.auth" && boolFromValue(value.Value["google_enabled"]) {
			if strings.TrimSpace(stringFromValue(value.Value["google_client_id"])) == "" {
				report.Valid = false
				report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_client_id", Severity: "error", Message: "google client id is required when google auth is enabled"})
			}
			if boolFromValue(value.Value["google_auto_provision_enabled"]) && strings.TrimSpace(stringFromValue(value.Value["google_auto_provision_role_id"])) == "" {
				report.Valid = false
				report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_auto_provision_role_id", Severity: "error", Message: "google auto provision role id is required when google auto provision is enabled"})
			}
			if strings.TrimSpace(stringFromValue(value.Value["google_client_secret"])) == "" {
				report.Valid = false
				report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_client_secret", Severity: "error", Message: "google client secret is required when google auth is enabled"})
			}
			if strings.TrimSpace(stringFromValue(value.Value["google_redirect_url"])) == "" {
				report.Valid = false
				report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_redirect_url", Severity: "error", Message: "google redirect url is required when google auth is enabled"})
			}
			if strings.TrimSpace(stringFromValue(value.Value["google_auth_url"])) == "" {
				report.Valid = false
				report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_auth_url", Severity: "error", Message: "google auth url is required when google auth is enabled"})
			}
			if strings.TrimSpace(stringFromValue(value.Value["google_token_url"])) == "" {
				report.Valid = false
				report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_token_url", Severity: "error", Message: "google token url is required when google auth is enabled"})
			}
			if strings.TrimSpace(stringFromValue(value.Value["google_jwks_url"])) == "" {
				report.Valid = false
				report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_jwks_url", Severity: "error", Message: "google jwks url is required when google auth is enabled"})
			}
		}
	}
	return report
}

func (s *Service) ValidateEntry(entry Entry) ValidationReport {
	report := ValidationReport{Valid: true}
	def, ok := s.Definition(entry.Key)
	if !ok {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{
			Key:      entry.Key,
			Severity: "error",
			Message:  "configuration key is not registered",
		})
		return report
	}
	if entry.Scope == "" {
		entry.Scope = "deployment"
	}
	if !containsScope(def.AllowedScopes, entry.Scope) {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{
			Key:      entry.Key,
			Severity: "error",
			Message:  "configuration scope is not allowed",
		})
		return report
	}
	entry.ModuleKey = def.ModuleKey
	entry.Category = def.Category
	value := cloneMap(def.DefaultValue)
	mergeMap(value, entry.Value)
	if err := validateValue(value, def.Fields); err != nil {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{
			Key:      def.Key,
			Severity: "error",
			Message:  err.Error(),
		})
	}
	appendRequiredFieldIssues(&report, def, value)
	appendSpecialValidationIssues(&report, def, value)
	return report
}

func (s *Service) CompareContexts(left, right CompareContext) []ComparisonItem {
	items := make([]ComparisonItem, 0, len(s.Definitions()))
	for _, def := range s.Definitions() {
		leftValue, _ := s.Resolve(def.Key, left.OrganizationID, left.LocationID)
		rightValue, _ := s.Resolve(def.Key, right.OrganizationID, right.LocationID)
		item := ComparisonItem{
			Key:       def.Key,
			ModuleKey: def.ModuleKey,
			Left:      leftValue,
			Right:     rightValue,
			Status:    "same",
		}
		if leftValue.SourceScope != rightValue.SourceScope || leftValue.SourceScopeID != rightValue.SourceScopeID {
			item.Status = "overridden"
		}
		item.ChangedFields = changedValueFields(leftValue.Value, rightValue.Value)
		if len(item.ChangedFields) > 0 {
			item.Status = "drifted"
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) AuthPolicy() AuthPolicy {
	policy := AuthPolicy{
		PasswordMinLength:            defaultPasswordMinLength,
		PasswordRequireUppercase:     false,
		PasswordRequireNumber:        false,
		PasswordRequireSpecial:       false,
		PasswordMaxAge:               0,
		SessionTTL:                   defaultSessionTTL,
		SessionIdleTimeout:           0,
		SessionRefreshWindow:         defaultSessionRefreshWindow,
		LoginRateLimitAttempts:       defaultLoginRateLimitAttempts,
		LoginRateLimitWindow:         defaultLoginRateLimitWindow,
		PasswordEnabled:              true,
		TOTPEnrollmentAllowed:        true,
		TOTPIssuer:                   "Orbyte",
		TOTPLoginMode:                "optional",
		TOTPApprovalMode:             "optional",
		TOTPStepUpTTL:                10 * time.Minute,
		LoginTitle:                   "Platform Access",
		LoginSubtitle:                "Sign in to continue.",
		GoogleButtonLabel:            "Continue with Google",
		GoogleAutoProvisionScopeType: "deployment",
		GoogleAuthURL:                "https://accounts.google.com/o/oauth2/v2/auth",
		GoogleTokenURL:               "https://oauth2.googleapis.com/token",
		GoogleIssuer:                 "https://accounts.google.com",
		GoogleJWKSURL:                "https://www.googleapis.com/oauth2/v3/certs",
		GoogleTimeout:                5 * time.Second,
	}
	value, ok := s.Resolve("identity.auth", "", "")
	if !ok {
		return policy
	}
	if raw := intValue(value.Value["password_min_length"]); raw > 0 {
		policy.PasswordMinLength = raw
	}
	policy.PasswordRequireUppercase = boolFromValue(value.Value["password_require_uppercase"])
	policy.PasswordRequireNumber = boolFromValue(value.Value["password_require_number"])
	policy.PasswordRequireSpecial = boolFromValue(value.Value["password_require_special"])
	if raw := intValue(value.Value["password_max_age_days"]); raw > 0 {
		policy.PasswordMaxAge = time.Duration(raw) * 24 * time.Hour
	}
	if raw := intValue(value.Value["session_ttl_minutes"]); raw > 0 {
		policy.SessionTTL = time.Duration(raw) * time.Minute
	}
	if raw := intValue(value.Value["session_idle_timeout_minutes"]); raw > 0 {
		policy.SessionIdleTimeout = time.Duration(raw) * time.Minute
	}
	if raw := intValue(value.Value["session_refresh_window_minutes"]); raw > 0 {
		policy.SessionRefreshWindow = time.Duration(raw) * time.Minute
	}
	if raw := intValue(value.Value["login_rate_limit_attempts"]); raw > 0 {
		policy.LoginRateLimitAttempts = raw
	}
	if raw := intValue(value.Value["login_rate_limit_window_seconds"]); raw > 0 {
		policy.LoginRateLimitWindow = time.Duration(raw) * time.Second
	}
	policy.TrustedOrigins = stringSliceValue(value.Value["trusted_origins"])
	policy.PasswordEnabled = boolFromValue(value.Value["password_enabled"])
	policy.TOTPEnabled = boolFromValue(value.Value["totp_enabled"])
	policy.TOTPEnrollmentAllowed = boolFromValue(value.Value["totp_enrollment_allowed"])
	if issuer := strings.TrimSpace(stringFromValue(value.Value["totp_issuer"])); issuer != "" {
		policy.TOTPIssuer = issuer
	}
	if mode := normalizeAuthMode(stringFromValue(value.Value["totp_login_mode"])); mode != "" {
		policy.TOTPLoginMode = mode
	}
	if mode := normalizeAuthMode(stringFromValue(value.Value["totp_approval_mode"])); mode != "" {
		policy.TOTPApprovalMode = mode
	}
	if raw := intValue(value.Value["totp_step_up_ttl_minutes"]); raw > 0 {
		policy.TOTPStepUpTTL = time.Duration(raw) * time.Minute
	}
	if title := strings.TrimSpace(stringFromValue(value.Value["login_title"])); title != "" {
		policy.LoginTitle = title
	}
	if subtitle := strings.TrimSpace(stringFromValue(value.Value["login_subtitle"])); subtitle != "" {
		policy.LoginSubtitle = subtitle
	}
	if label := strings.TrimSpace(stringFromValue(value.Value["google_button_label"])); label != "" {
		policy.GoogleButtonLabel = label
	}
	policy.GoogleEnabled = boolFromValue(value.Value["google_enabled"])
	policy.GoogleAutoProvisionEnabled = boolFromValue(value.Value["google_auto_provision_enabled"])
	policy.GoogleAutoProvisionAllowedDomains = stringSliceValue(value.Value["google_auto_provision_allowed_domains"])
	policy.GoogleAutoProvisionRoleID = strings.TrimSpace(stringFromValue(value.Value["google_auto_provision_role_id"]))
	if scopeType := strings.TrimSpace(stringFromValue(value.Value["google_auto_provision_scope_type"])); scopeType != "" {
		policy.GoogleAutoProvisionScopeType = scopeType
	}
	policy.GoogleAutoProvisionScopeID = strings.TrimSpace(stringFromValue(value.Value["google_auto_provision_scope_id"]))
	policy.GoogleAutoProvisionDefaultLocationID = strings.TrimSpace(stringFromValue(value.Value["google_auto_provision_default_location_id"]))
	policy.GoogleClientID = strings.TrimSpace(stringFromValue(value.Value["google_client_id"]))
	policy.GoogleClientSecret = strings.TrimSpace(stringFromValue(value.Value["google_client_secret"]))
	policy.GoogleRedirectURL = strings.TrimSpace(stringFromValue(value.Value["google_redirect_url"]))
	policy.GoogleAuthURL = strings.TrimSpace(stringFromValue(value.Value["google_auth_url"]))
	policy.GoogleTokenURL = strings.TrimSpace(stringFromValue(value.Value["google_token_url"]))
	policy.GoogleIssuer = strings.TrimSpace(stringFromValue(value.Value["google_issuer"]))
	policy.GoogleJWKSURL = strings.TrimSpace(stringFromValue(value.Value["google_jwks_url"]))
	policy.GoogleHostedDomain = strings.TrimSpace(stringFromValue(value.Value["google_hosted_domain"]))
	if raw := intValue(value.Value["google_timeout_seconds"]); raw > 0 {
		policy.GoogleTimeout = time.Duration(raw) * time.Second
	}
	return policy
}

func normalizeAuthMode(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "disabled", "optional", "required":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return ""
	}
}

func (s *Service) TypesensePolicy() TypesensePolicy {
	policy := TypesensePolicy{Enabled: false, TimeoutSeconds: 5}
	if value, ok := s.Resolve("search.typesense", "", ""); ok {
		policy.Enabled = boolFromValue(value.Value["enabled"])
		policy.Endpoint = strings.TrimSpace(stringFromValue(value.Value["endpoint"]))
		policy.APIKey = strings.TrimSpace(stringFromValue(value.Value["api_key"]))
		if timeout := intFromValue(value.Value["timeout_seconds"]); timeout > 0 {
			policy.TimeoutSeconds = timeout
		}
	}
	return policy
}

func (s *Service) NATSPolicy() NATSPolicy {
	policy := NATSPolicy{Enabled: false, SinkName: "nats", TimeoutSeconds: 5}
	if value, ok := s.Resolve("eventing.nats", "", ""); ok {
		policy.Enabled = boolFromValue(value.Value["enabled"])
		policy.URL = strings.TrimSpace(stringFromValue(value.Value["url"]))
		if sinkName := strings.TrimSpace(stringFromValue(value.Value["sink_name"])); sinkName != "" {
			policy.SinkName = sinkName
		}
		policy.SubjectPrefix = strings.TrimSpace(stringFromValue(value.Value["subject_prefix"]))
		if timeout := intFromValue(value.Value["timeout_seconds"]); timeout > 0 {
			policy.TimeoutSeconds = timeout
		}
	}
	return policy
}

func (s *Service) EmbeddingPolicy() EmbeddingPolicy {
	policy := EmbeddingPolicy{Provider: "hash", Dimensions: 8}
	if value, ok := s.Resolve("search.embedding", "", ""); ok {
		if provider := strings.TrimSpace(stringFromValue(value.Value["provider"])); provider != "" {
			policy.Provider = provider
		}
		if dimensions := intFromValue(value.Value["dimensions"]); dimensions > 0 {
			policy.Dimensions = dimensions
		}
	}
	return policy
}

func validateValue(value map[string]any, fields []FieldDefinition) error {
	for _, field := range fields {
		current, ok := value[field.Key]
		if field.Required && !ok {
			continue
		}
		if !ok {
			continue
		}
		switch field.Type {
		case "int":
			if intValue(current) == 0 && current != 0 && current != int32(0) && current != int64(0) && current != float64(0) {
				return shared.Validation(fmt.Sprintf("%s must be an integer", field.Key))
			}
		case "bool":
			if _, ok := current.(bool); !ok {
				return shared.Validation(fmt.Sprintf("%s must be a boolean", field.Key))
			}
		case "string":
			if _, ok := current.(string); !ok {
				return shared.Validation(fmt.Sprintf("%s must be a string", field.Key))
			}
		case "string_list":
			if strings := stringSliceValue(current); len(strings) == 0 && current != nil {
				switch typed := current.(type) {
				case []string:
					_ = typed
				case []any:
					_ = typed
				default:
					return shared.Validation(fmt.Sprintf("%s must be a list of strings", field.Key))
				}
			}
		}
		if len(field.Enum) > 0 {
			text, _ := current.(string)
			if text != "" && !containsString(field.Enum, text) {
				return shared.Validation(fmt.Sprintf("%s must be one of %s", field.Key, strings.Join(field.Enum, ", ")))
			}
		}
	}
	return nil
}

func appendRequiredFieldIssues(report *ValidationReport, def Definition, value map[string]any) {
	for _, field := range def.Fields {
		if !field.Required {
			continue
		}
		current, ok := value[field.Key]
		if !ok || isZeroFieldValue(current) {
			report.Valid = false
			report.Issues = append(report.Issues, ValidationIssue{
				Key:      def.Key,
				Field:    field.Key,
				Severity: "error",
				Message:  "required field is missing or empty",
			})
		}
	}
}

func appendSpecialValidationIssues(report *ValidationReport, def Definition, value map[string]any) {
	if def.Key == "identity.auth" {
		if intValue(value["password_max_age_days"]) < 0 {
			report.Valid = false
			report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "password_max_age_days", Severity: "error", Message: "password max age days must be zero or greater"})
		}
		if intValue(value["session_idle_timeout_minutes"]) < 0 {
			report.Valid = false
			report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "session_idle_timeout_minutes", Severity: "error", Message: "session idle timeout minutes must be zero or greater"})
		}
	}
	if def.Key != "identity.auth" || !boolFromValue(value["google_enabled"]) {
		return
	}
	if strings.TrimSpace(stringFromValue(value["google_client_id"])) == "" {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_client_id", Severity: "error", Message: "google client id is required when google auth is enabled"})
	}
	if boolFromValue(value["google_auto_provision_enabled"]) && strings.TrimSpace(stringFromValue(value["google_auto_provision_role_id"])) == "" {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_auto_provision_role_id", Severity: "error", Message: "google auto provision role id is required when google auto provision is enabled"})
	}
	if strings.TrimSpace(stringFromValue(value["google_client_secret"])) == "" {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_client_secret", Severity: "error", Message: "google client secret is required when google auth is enabled"})
	}
	if strings.TrimSpace(stringFromValue(value["google_redirect_url"])) == "" {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_redirect_url", Severity: "error", Message: "google redirect url is required when google auth is enabled"})
	}
	if strings.TrimSpace(stringFromValue(value["google_auth_url"])) == "" {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_auth_url", Severity: "error", Message: "google auth url is required when google auth is enabled"})
	}
	if strings.TrimSpace(stringFromValue(value["google_token_url"])) == "" {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_token_url", Severity: "error", Message: "google token url is required when google auth is enabled"})
	}
	if strings.TrimSpace(stringFromValue(value["google_jwks_url"])) == "" {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{Key: def.Key, Field: "google_jwks_url", Severity: "error", Message: "google jwks url is required when google auth is enabled"})
	}
}

func changedValueFields(left, right map[string]any) []string {
	seen := map[string]struct{}{}
	fields := make([]string, 0)
	for key := range left {
		seen[key] = struct{}{}
	}
	for key := range right {
		seen[key] = struct{}{}
	}
	for key := range seen {
		if stringifyComparable(left[key]) != stringifyComparable(right[key]) {
			fields = append(fields, key)
		}
	}
	sort.Strings(fields)
	return fields
}

func stringifyComparable(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(raw)
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		return 0
	}
}

func intFromValue(value any) int {
	return intValue(value)
}

func boolFromValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func stringFromValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func isZeroFieldValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []string:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && text != "" {
				items = append(items, text)
			}
		}
		return items
	default:
		return nil
	}
}

func containsScope(scopes []string, scope string) bool {
	return containsString(scopes, scope)
}

func containsString(items []string, candidate string) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func mergeMap(target, source map[string]any) {
	for key, value := range source {
		target[key] = value
	}
}
