package runtimeconfig

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Service struct{}

type EmailSettings struct {
	From      string
	Host      string
	Port      string
	Username  string
	Password  string
	UseTLS    bool
	OutboxDir string
}

type ObjectStoreSettings struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

var defaultService = &Service{}

func Current() *Service {
	return defaultService
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) DomainProfile() string {
	return strings.TrimSpace(os.Getenv("APP_DOMAIN_PROFILE"))
}

func (s *Service) Address() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("APP_ADDRESS")), ":8080")
}

func (s *Service) Environment() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
}

func (s *Service) IsDevelopmentLike() bool {
	switch s.Environment() {
	case "", "development", "dev", "test":
		return true
	default:
		return false
	}
}

func (s *Service) AuthDevMode() bool {
	return s.bool("APP_AUTH_DEV_MODE")
}

func (s *Service) AuthDevBypass() bool {
	return s.bool("APP_AUTH_DEV_BYPASS")
}

func (s *Service) JWTSecret() string {
	return strings.TrimSpace(os.Getenv("APP_JWT_SECRET"))
}

func (s *Service) JWTIssuer() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("APP_JWT_ISSUER")), "orbyte")
}

func (s *Service) BootstrapAdminPassword() string {
	return strings.TrimSpace(os.Getenv("APP_BOOTSTRAP_ADMIN_PASSWORD"))
}

func (s *Service) DatabaseURL() string {
	return strings.TrimSpace(os.Getenv("DATABASE_URL"))
}

func (s *Service) HTTPReadTimeout() time.Duration {
	return s.duration("APP_HTTP_READ_TIMEOUT_SECONDS", 15*time.Second)
}

func (s *Service) HTTPWriteTimeout() time.Duration {
	return s.duration("APP_HTTP_WRITE_TIMEOUT_SECONDS", 30*time.Second)
}

func (s *Service) HTTPIdleTimeout() time.Duration {
	return s.duration("APP_HTTP_IDLE_TIMEOUT_SECONDS", 60*time.Second)
}

func (s *Service) DBMaxOpenConns() int {
	return s.int("APP_DB_MAX_OPEN_CONNS", 25)
}

func (s *Service) DBMaxIdleConns() int {
	return s.int("APP_DB_MAX_IDLE_CONNS", 25)
}

func (s *Service) DBConnMaxLifetime() time.Duration {
	return s.durationSecondsOnly("APP_DB_CONN_MAX_LIFETIME_SECONDS", time.Hour)
}

func (s *Service) DBConnMaxIdleTime() time.Duration {
	return s.durationSecondsOnly("APP_DB_CONN_MAX_IDLE_TIME_SECONDS", 15*time.Minute)
}

func (s *Service) IntegrationHTTPTimeout() time.Duration {
	return s.durationSecondsOnly("APP_INTEGRATION_HTTP_TIMEOUT_SECONDS", 15*time.Second)
}

func (s *Service) DocsEnabled() bool {
	if s.AuthDevMode() {
		return true
	}
	switch s.Environment() {
	case "development", "dev", "test":
		return true
	default:
		return false
	}
}

func (s *Service) CookieSecure() bool {
	return !s.IsDevelopmentLike()
}

func (s *Service) WorkflowEmailAutoDispatch() bool {
	return s.bool("WORKFLOW_EMAIL_AUTO_DISPATCH")
}

func (s *Service) EmailSettings() EmailSettings {
	return EmailSettings{
		From:      firstNonEmpty(strings.TrimSpace(os.Getenv("SMTP_FROM")), "workflow@orbyte.local"),
		Host:      strings.TrimSpace(os.Getenv("SMTP_HOST")),
		Port:      firstNonEmpty(strings.TrimSpace(os.Getenv("SMTP_PORT")), "587"),
		Username:  strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		Password:  strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
		UseTLS:    s.bool("SMTP_TLS"),
		OutboxDir: strings.TrimSpace(os.Getenv("WORKFLOW_EMAIL_OUTBOX_DIR")),
	}
}

func (s *Service) ObjectStoreSettings() ObjectStoreSettings {
	return ObjectStoreSettings{
		Endpoint:  strings.TrimSpace(os.Getenv("OBJECT_STORE_ENDPOINT")),
		AccessKey: strings.TrimSpace(os.Getenv("OBJECT_STORE_ACCESS_KEY")),
		SecretKey: strings.TrimSpace(os.Getenv("OBJECT_STORE_SECRET_KEY")),
		UseSSL:    s.bool("OBJECT_STORE_SSL"),
	}
}

func (s *Service) bool(key string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes"
}

func (s *Service) int(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func (s *Service) duration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err == nil {
		return parsed
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func DurationFromEnv(key string, fallback time.Duration) time.Duration {
	return Current().duration(key, fallback)
}

func DurationSecondsFromEnv(key string, fallback time.Duration) time.Duration {
	return Current().durationSecondsOnly(key, fallback)
}

func IntFromEnv(key string, fallback int) int {
	return Current().int(key, fallback)
}

func (s *Service) durationSecondsOnly(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
