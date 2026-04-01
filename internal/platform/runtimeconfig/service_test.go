package runtimeconfig

import (
	"testing"
	"time"
)

func TestServiceEnvironmentAndModeHelpers(t *testing.T) {
	svc := NewService()
	if Current() == nil {
		t.Fatal("expected default service")
	}

	t.Setenv("APP_DOMAIN_PROFILE", " clinic ")
	t.Setenv("APP_ADDRESS", " :9090 ")
	t.Setenv("APP_ENV", " DEV ")
	t.Setenv("APP_AUTH_DEV_MODE", "yes")
	t.Setenv("APP_AUTH_DEV_BYPASS", "true")
	t.Setenv("APP_JWT_SECRET", " secret ")
	t.Setenv("APP_JWT_ISSUER", " issuer ")
	t.Setenv("APP_BOOTSTRAP_ADMIN_PASSWORD", " adminpw ")
	t.Setenv("DATABASE_URL", " postgres://db ")
	t.Setenv("WORKFLOW_EMAIL_AUTO_DISPATCH", "1")

	if got := svc.DomainProfile(); got != "clinic" {
		t.Fatalf("unexpected domain profile: %q", got)
	}
	if got := svc.Address(); got != ":9090" {
		t.Fatalf("unexpected address: %q", got)
	}
	if got := svc.Environment(); got != "dev" {
		t.Fatalf("unexpected environment: %q", got)
	}
	if !svc.IsDevelopmentLike() || !svc.AuthDevMode() || !svc.AuthDevBypass() || !svc.DocsEnabled() {
		t.Fatal("expected development-like/auth-dev/docs-enabled state")
	}
	if svc.CookieSecure() {
		t.Fatal("expected non-secure cookies in development-like mode")
	}
	if got := svc.JWTSecret(); got != "secret" {
		t.Fatalf("unexpected jwt secret: %q", got)
	}
	if got := svc.JWTIssuer(); got != "issuer" {
		t.Fatalf("unexpected jwt issuer: %q", got)
	}
	if got := svc.BootstrapAdminPassword(); got != "adminpw" {
		t.Fatalf("unexpected bootstrap admin password: %q", got)
	}
	if got := svc.DatabaseURL(); got != "postgres://db" {
		t.Fatalf("unexpected database url: %q", got)
	}
	if !svc.WorkflowEmailAutoDispatch() {
		t.Fatal("expected workflow email auto dispatch to be enabled")
	}
}

func TestServiceDefaultsAndFallbackParsers(t *testing.T) {
	svc := NewService()

	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_AUTH_DEV_MODE", "false")
	t.Setenv("APP_AUTH_DEV_BYPASS", "0")
	t.Setenv("APP_JWT_ISSUER", "")
	t.Setenv("APP_ADDRESS", "")
	t.Setenv("APP_HTTP_READ_TIMEOUT_SECONDS", "2m")
	t.Setenv("APP_HTTP_WRITE_TIMEOUT_SECONDS", "45")
	t.Setenv("APP_HTTP_IDLE_TIMEOUT_SECONDS", "broken")
	t.Setenv("APP_DB_MAX_OPEN_CONNS", "17")
	t.Setenv("APP_DB_MAX_IDLE_CONNS", "-2")
	t.Setenv("APP_DB_CONN_MAX_LIFETIME_SECONDS", "600")
	t.Setenv("APP_DB_CONN_MAX_IDLE_TIME_SECONDS", "oops")
	t.Setenv("APP_INTEGRATION_HTTP_TIMEOUT_SECONDS", "12")

	if svc.IsDevelopmentLike() || svc.DocsEnabled() {
		t.Fatal("expected production mode to disable development/docs defaults")
	}
	if !svc.CookieSecure() {
		t.Fatal("expected secure cookies outside development-like mode")
	}
	if got := svc.JWTIssuer(); got != "orbyte" {
		t.Fatalf("unexpected default jwt issuer: %q", got)
	}
	if got := svc.Address(); got != ":8080" {
		t.Fatalf("unexpected default address: %q", got)
	}
	if got := svc.HTTPReadTimeout(); got != 2*time.Minute {
		t.Fatalf("unexpected read timeout: %v", got)
	}
	if got := svc.HTTPWriteTimeout(); got != 45*time.Second {
		t.Fatalf("unexpected write timeout: %v", got)
	}
	if got := svc.HTTPIdleTimeout(); got != 60*time.Second {
		t.Fatalf("unexpected idle timeout fallback: %v", got)
	}
	if got := svc.DBMaxOpenConns(); got != 17 {
		t.Fatalf("unexpected max open conns: %d", got)
	}
	if got := svc.DBMaxIdleConns(); got != 25 {
		t.Fatalf("unexpected fallback max idle conns: %d", got)
	}
	if got := svc.DBConnMaxLifetime(); got != 600*time.Second {
		t.Fatalf("unexpected conn max lifetime: %v", got)
	}
	if got := svc.DBConnMaxIdleTime(); got != 15*time.Minute {
		t.Fatalf("unexpected fallback conn max idle time: %v", got)
	}
	if got := svc.IntegrationHTTPTimeout(); got != 12*time.Second {
		t.Fatalf("unexpected integration timeout: %v", got)
	}
}

func TestServiceSettingsAndExportedHelpers(t *testing.T) {
	svc := NewService()

	t.Setenv("SMTP_FROM", " sender@example.com ")
	t.Setenv("SMTP_HOST", " smtp.example.com ")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USERNAME", " user ")
	t.Setenv("SMTP_PASSWORD", " pass ")
	t.Setenv("SMTP_TLS", "TRUE")
	t.Setenv("WORKFLOW_EMAIL_OUTBOX_DIR", " /tmp/outbox ")
	t.Setenv("OBJECT_STORE_ENDPOINT", " https://obj.example.com ")
	t.Setenv("OBJECT_STORE_ACCESS_KEY", " access ")
	t.Setenv("OBJECT_STORE_SECRET_KEY", " secret ")
	t.Setenv("OBJECT_STORE_SSL", "yes")
	t.Setenv("CUSTOM_DURATION", "90")
	t.Setenv("CUSTOM_SECONDS", "bad")
	t.Setenv("CUSTOM_INT", "7")
	t.Setenv("CUSTOM_EMPTY", "")

	email := svc.EmailSettings()
	if email.From != "sender@example.com" || email.Host != "smtp.example.com" || email.Port != "587" || email.Username != "user" || email.Password != "pass" || !email.UseTLS || email.OutboxDir != "/tmp/outbox" {
		t.Fatalf("unexpected email settings: %+v", email)
	}

	store := svc.ObjectStoreSettings()
	if store.Endpoint != "https://obj.example.com" || store.AccessKey != "access" || store.SecretKey != "secret" || !store.UseSSL {
		t.Fatalf("unexpected object store settings: %+v", store)
	}

	if got := DurationFromEnv("CUSTOM_DURATION", 5*time.Second); got != 90*time.Second {
		t.Fatalf("unexpected duration helper result: %v", got)
	}
	if got := DurationSecondsFromEnv("CUSTOM_SECONDS", 11*time.Second); got != 11*time.Second {
		t.Fatalf("unexpected duration seconds fallback: %v", got)
	}
	if got := IntFromEnv("CUSTOM_INT", 3); got != 7 {
		t.Fatalf("unexpected int helper result: %d", got)
	}
	if got := firstNonEmpty("", "  ", " value "); got != "value" {
		t.Fatalf("unexpected firstNonEmpty result: %q", got)
	}
	if got := firstNonEmpty("", " "); got != "" {
		t.Fatalf("unexpected empty firstNonEmpty result: %q", got)
	}
}
