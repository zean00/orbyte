package config

import (
	"testing"
	"time"
)

func TestServiceKeysAndGet(t *testing.T) {
	svc := NewService()
	keys := svc.Keys()
	if len(keys) == 0 {
		t.Fatal("expected config keys")
	}
	entry, ok := svc.Get("platform.http")
	if !ok {
		t.Fatal("expected platform.http entry")
	}
	if entry.Category != "platform" {
		t.Fatalf("unexpected category: %s", entry.Category)
	}
}

func TestMemoryRepositoryListSorted(t *testing.T) {
	repo := NewMemoryRepository([]Entry{{Key: "b", Scope: "deployment"}, {Key: "a", Scope: "deployment"}})
	items := repo.List()
	if items[0].Key != "a" {
		t.Fatalf("expected sorted keys, got %s", items[0].Key)
	}
}

func TestAuthPolicyDefaultsAndOverrides(t *testing.T) {
	svc := NewService()
	policy := svc.AuthPolicy()
	if policy.PasswordMinLength != 8 || policy.SessionTTL <= 0 || policy.LoginRateLimitAttempts != 5 {
		t.Fatalf("unexpected default auth policy: %+v", policy)
	}
	if policy.PasswordRequireUppercase || policy.PasswordRequireNumber || policy.PasswordRequireSpecial || policy.PasswordMaxAge != 0 || policy.SessionIdleTimeout != 0 {
		t.Fatalf("unexpected default password/session policy extensions: %+v", policy)
	}
	if !policy.PasswordEnabled || policy.LoginTitle != "Platform Access" || policy.LoginSubtitle != "Sign in to continue." || policy.GoogleButtonLabel != "Continue with Google" || policy.GoogleAutoProvisionEnabled || policy.GoogleAutoProvisionScopeType != "deployment" || policy.GoogleEnabled || policy.GoogleClientID != "" || policy.GoogleJWKSURL == "" {
		t.Fatalf("unexpected default google auth policy: %+v", policy)
	}

	override := NewServiceWithRepository(NewMemoryRepository([]Entry{{
		Key:      "identity.auth",
		Category: "identity",
		Scope:    "deployment",
		Value: map[string]any{
			"password_min_length":                       12,
			"password_require_uppercase":                true,
			"password_require_number":                   true,
			"password_require_special":                  true,
			"password_max_age_days":                     30,
			"session_ttl_minutes":                       30,
			"session_idle_timeout_minutes":              15,
			"session_refresh_window_minutes":            10,
			"login_rate_limit_attempts":                 2,
			"login_rate_limit_window_seconds":           45,
			"trusted_origins":                           []any{"https://app.example.com"},
			"password_enabled":                          false,
			"login_title":                               "Welcome to Orbyte",
			"login_subtitle":                            "Use your company account.",
			"google_button_label":                       "Sign in with Google Workspace",
			"google_enabled":                            true,
			"google_auto_provision_enabled":             true,
			"google_auto_provision_allowed_domains":     []any{"example.com", "example.org"},
			"google_auto_provision_role_id":             "role_admin",
			"google_auto_provision_scope_type":          "deployment",
			"google_auto_provision_scope_id":            "",
			"google_auto_provision_default_location_id": "loc_hq",
			"google_client_id":                          "client-123",
			"google_client_secret":                      "secret-123",
			"google_redirect_url":                       "https://app.example.com/auth/google/callback",
			"google_auth_url":                           "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":                          "https://oauth2.googleapis.com/token",
			"google_issuer":                             "https://accounts.google.com",
			"google_jwks_url":                           "https://example.test/jwks",
			"google_hosted_domain":                      "example.com",
			"google_timeout_seconds":                    9,
		},
	}}))
	policy = override.AuthPolicy()
	if policy.PasswordMinLength != 12 || !policy.PasswordRequireUppercase || !policy.PasswordRequireNumber || !policy.PasswordRequireSpecial || policy.PasswordMaxAge != 30*24*time.Hour || policy.SessionTTL != 30*time.Minute || policy.SessionIdleTimeout != 15*time.Minute || policy.SessionRefreshWindow != 10*time.Minute || policy.LoginRateLimitAttempts != 2 || policy.LoginRateLimitWindow != 45*time.Second || len(policy.TrustedOrigins) != 1 || policy.TrustedOrigins[0] != "https://app.example.com" || policy.PasswordEnabled || policy.LoginTitle != "Welcome to Orbyte" || policy.LoginSubtitle != "Use your company account." || policy.GoogleButtonLabel != "Sign in with Google Workspace" || !policy.GoogleEnabled || !policy.GoogleAutoProvisionEnabled || len(policy.GoogleAutoProvisionAllowedDomains) != 2 || policy.GoogleAutoProvisionAllowedDomains[0] != "example.com" || policy.GoogleAutoProvisionRoleID != "role_admin" || policy.GoogleAutoProvisionScopeType != "deployment" || policy.GoogleAutoProvisionDefaultLocationID != "loc_hq" || policy.GoogleClientID != "client-123" || policy.GoogleClientSecret != "secret-123" || policy.GoogleRedirectURL != "https://app.example.com/auth/google/callback" || policy.GoogleAuthURL != "https://accounts.google.com/o/oauth2/v2/auth" || policy.GoogleTokenURL != "https://oauth2.googleapis.com/token" || policy.GoogleHostedDomain != "example.com" || policy.GoogleTimeout != 9*time.Second {
		t.Fatalf("unexpected overridden auth policy: %+v", policy)
	}
}

func TestResolveScopedConfiguration(t *testing.T) {
	svc := NewServiceWithRepository(NewMemoryRepository([]Entry{{
		Key:       "identity.auth",
		ModuleKey: "identity",
		Category:  "security",
		Scope:     "deployment",
		Value:     map[string]any{"session_ttl_minutes": 90, "password_min_length": 10},
	}, {
		Key:       "identity.auth",
		ModuleKey: "identity",
		Category:  "security",
		Scope:     "organization",
		ScopeID:   "org_default",
		Value:     map[string]any{"session_ttl_minutes": 45},
	}, {
		Key:       "identity.auth",
		ModuleKey: "identity",
		Category:  "security",
		Scope:     "location",
		ScopeID:   "loc_hq",
		Value:     map[string]any{"login_rate_limit_attempts": 2},
	}}))

	effective, ok := svc.Resolve("identity.auth", "org_default", "loc_hq")
	if !ok {
		t.Fatal("expected effective config")
	}
	if effective.SourceScope != "location" || intValue(effective.Value["session_ttl_minutes"]) != 45 || intValue(effective.Value["login_rate_limit_attempts"]) != 2 || intValue(effective.Value["password_min_length"]) != 10 {
		t.Fatalf("unexpected effective config: %+v", effective)
	}
}

func TestDefinitionsEntriesResolveAllAndSaveValidation(t *testing.T) {
	svc := NewService()
	if len(svc.Definitions()) == 0 {
		t.Fatal("expected config definitions")
	}
	if len(svc.Entries()) == 0 {
		t.Fatal("expected config entries")
	}
	if len(svc.ResolveAll("org_default", "loc_hq")) == 0 {
		t.Fatal("expected resolved effective entries")
	}

	if err := svc.Save(Entry{Key: "unknown.key", Scope: "deployment", Value: map[string]any{}}); err == nil {
		t.Fatal("expected unknown key save failure")
	}
	if err := svc.Save(Entry{Key: "platform.http", Scope: "location", ScopeID: "loc_hq", Value: map[string]any{"address": ":8081"}}); err == nil {
		t.Fatal("expected disallowed scope failure")
	}
	if err := svc.Save(Entry{Key: "platform.http", Scope: "deployment", Value: map[string]any{"address": 123}}); err == nil {
		t.Fatal("expected invalid string field failure")
	}
	if err := svc.Save(Entry{Key: "identity.auth", Scope: "deployment", Value: map[string]any{"trusted_origins": "bad"}}); err == nil {
		t.Fatal("expected invalid string_list field failure")
	}
	if err := svc.Save(Entry{Key: "identity.auth", Scope: "deployment", Value: map[string]any{"password_min_length": "bad"}}); err == nil {
		t.Fatal("expected invalid int field failure")
	}
	invalid := NewServiceWithRepository(NewMemoryRepository([]Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value:    map[string]any{"google_enabled": true},
	}}))
	if report := invalid.ValidateAll("", ""); report.Valid {
		t.Fatal("expected google auth validation to fail without client id")
	}
	invalidProvision := NewServiceWithRepository(NewMemoryRepository([]Entry{{
		Key:      "identity.auth",
		Category: "security",
		Scope:    "deployment",
		Value: map[string]any{
			"google_enabled":                true,
			"google_client_id":              "client-123",
			"google_client_secret":          "secret-123",
			"google_redirect_url":           "https://app.example.com/auth/google/callback",
			"google_auth_url":               "https://accounts.google.com/o/oauth2/v2/auth",
			"google_token_url":              "https://oauth2.googleapis.com/token",
			"google_jwks_url":               "https://example.test/jwks",
			"google_auto_provision_enabled": true,
		},
	}}))
	if report := invalidProvision.ValidateAll("", ""); report.Valid {
		t.Fatal("expected google auto provision validation to fail without role id")
	}
}

func TestSensitiveValuesStoredAsSecretRefs(t *testing.T) {
	svc := NewService()
	if err := svc.Save(Entry{
		Key:      "search.typesense",
		Category: "search",
		Scope:    "deployment",
		Value: map[string]any{
			"enabled":  true,
			"endpoint": "http://typesense.local",
			"api_key":  "secret-key-123",
		},
	}); err != nil {
		t.Fatalf("save sensitive config: %v", err)
	}

	entry, ok := svc.Get("search.typesense")
	if !ok {
		t.Fatal("expected saved entry")
	}
	field, ok := entry.Value["api_key"].(map[string]any)
	if !ok {
		t.Fatalf("expected stored secret ref, got %#v", entry.Value["api_key"])
	}
	if _, ok := field["secret_ref"].(string); !ok {
		t.Fatalf("expected secret ref payload, got %#v", field)
	}

	effective, ok := svc.Resolve("search.typesense", "", "")
	if !ok {
		t.Fatal("expected effective value")
	}
	if effective.Value["api_key"] != "secret-key-123" {
		t.Fatalf("expected resolved secret value, got %#v", effective.Value["api_key"])
	}
}

func TestNATSPolicyDefaultsAndOverrides(t *testing.T) {
	svc := NewService()
	policy := svc.NATSPolicy()
	if policy.Enabled || policy.SinkName != "nats" || policy.TimeoutSeconds != 5 {
		t.Fatalf("unexpected default nats policy: %+v", policy)
	}

	override := NewServiceWithRepository(NewMemoryRepository([]Entry{{
		Key:      "eventing.nats",
		Category: "eventing",
		Scope:    "deployment",
		Value: map[string]any{
			"enabled":         true,
			"url":             "nats://127.0.0.1:4222",
			"sink_name":       "broker",
			"subject_prefix":  "clinic",
			"timeout_seconds": 9,
		},
	}}))
	policy = override.NATSPolicy()
	if !policy.Enabled || policy.URL != "nats://127.0.0.1:4222" || policy.SinkName != "broker" || policy.SubjectPrefix != "clinic" || policy.TimeoutSeconds != 9 {
		t.Fatalf("unexpected overridden nats policy: %+v", policy)
	}
}

func TestValidateEntryAndCompareContexts(t *testing.T) {
	svc := NewService()
	report := svc.ValidateEntry(Entry{
		Key:      "identity.auth",
		Scope:    "deployment",
		Value:    map[string]any{"google_enabled": true},
		Category: "security",
	})
	if report.Valid {
		t.Fatalf("expected validation failure, got %+v", report)
	}

	override := NewServiceWithRepository(NewMemoryRepository([]Entry{
		{Key: "identity.auth", ModuleKey: "identity", Category: "security", Scope: "deployment", Value: map[string]any{"session_ttl_minutes": 90, "password_min_length": 10}},
		{Key: "identity.auth", ModuleKey: "identity", Category: "security", Scope: "location", ScopeID: "loc_hq", Value: map[string]any{"login_rate_limit_attempts": 2}},
	}))
	items := override.CompareContexts(
		CompareContext{Label: "left"},
		CompareContext{Label: "right", LocationID: "loc_hq"},
	)
	if len(items) == 0 {
		t.Fatal("expected comparison items")
	}
	var found bool
	for _, item := range items {
		if item.Key != "identity.auth" {
			continue
		}
		found = true
		if item.Status != "drifted" {
			t.Fatalf("expected drifted config status, got %+v", item)
		}
		if len(item.ChangedFields) == 0 {
			t.Fatalf("expected changed fields, got %+v", item)
		}
	}
	if !found {
		t.Fatal("expected identity.auth comparison item")
	}
}
