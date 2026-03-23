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
	SessionTTL                           time.Duration
	SessionRefreshWindow                 time.Duration
	LoginRateLimitAttempts               int
	LoginRateLimitWindow                 time.Duration
	TrustedOrigins                       []string
	PasswordEnabled                      bool
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
			"session_ttl_minutes":                       int(defaultSessionTTL / time.Minute),
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
		Fields: []FieldDefinition{
			{Key: "password_min_length", Label: "Password Min Length", Type: "int", Required: true},
			{Key: "session_ttl_minutes", Label: "Session TTL Minutes", Type: "int", Required: true},
			{Key: "session_refresh_window_minutes", Label: "Session Refresh Window Minutes", Type: "int", Required: true},
			{Key: "login_rate_limit_attempts", Label: "Login Rate Limit Attempts", Type: "int", Required: true},
			{Key: "login_rate_limit_window_seconds", Label: "Login Rate Limit Window Seconds", Type: "int", Required: true},
			{Key: "trusted_origins", Label: "Trusted Origins", Type: "string_list"},
			{Key: "password_enabled", Label: "Password Enabled", Type: "bool"},
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
		Key:       "identity.auth",
		ModuleKey: "identity",
		Category:  "security",
		Scope:     "deployment",
		ScopeID:   "",
		Value: map[string]any{
			"password_min_length":                       defaultPasswordMinLength,
			"session_ttl_minutes":                       int(defaultSessionTTL / time.Minute),
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
		SessionTTL:                   defaultSessionTTL,
		SessionRefreshWindow:         defaultSessionRefreshWindow,
		LoginRateLimitAttempts:       defaultLoginRateLimitAttempts,
		LoginRateLimitWindow:         defaultLoginRateLimitWindow,
		PasswordEnabled:              true,
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
	if raw := intValue(value.Value["session_ttl_minutes"]); raw > 0 {
		policy.SessionTTL = time.Duration(raw) * time.Minute
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
