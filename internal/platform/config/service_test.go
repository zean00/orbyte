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

	override := NewServiceWithRepository(NewMemoryRepository([]Entry{{
		Key:      "identity.auth",
		Category: "identity",
		Scope:    "deployment",
		Value: map[string]any{
			"password_min_length":             12,
			"session_ttl_minutes":             30,
			"session_refresh_window_minutes":  10,
			"login_rate_limit_attempts":       2,
			"login_rate_limit_window_seconds": 45,
			"trusted_origins":                 []any{"https://app.example.com"},
		},
	}}))
	policy = override.AuthPolicy()
	if policy.PasswordMinLength != 12 || policy.SessionTTL != 30*time.Minute || policy.SessionRefreshWindow != 10*time.Minute || policy.LoginRateLimitAttempts != 2 || policy.LoginRateLimitWindow != 45*time.Second || len(policy.TrustedOrigins) != 1 || policy.TrustedOrigins[0] != "https://app.example.com" {
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
