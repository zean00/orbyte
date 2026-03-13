package identity

import "time"

type User struct {
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	AuthenticationSubject string `json:"authentication_subject,omitempty"`
	Status            string    `json:"status"`
	DefaultLocationID string    `json:"default_location_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Role struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	ScopeType string    `json:"scope_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Permission struct {
	Key         string `json:"key"`
	Module      string `json:"module"`
	Action      string `json:"action"`
	Resource    string `json:"resource"`
	Description string `json:"description,omitempty"`
}

type RoleBinding struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	RoleID        string    `json:"role_id"`
	ScopeType     string    `json:"scope_type"`
	ScopeID       string    `json:"scope_id,omitempty"`
	EffectiveFrom time.Time `json:"effective_from"`
	EffectiveTo   time.Time `json:"effective_to,omitempty"`
	Status        string    `json:"status"`
}

type RolePermission struct {
	RoleID        string `json:"role_id"`
	PermissionKey string `json:"permission_key"`
}

type Credential struct {
	UserID             string    `json:"user_id"`
	PasswordHash       string    `json:"-"`
	PasswordChangedAt  time.Time `json:"password_changed_at"`
	FailedAttemptCount int       `json:"failed_attempt_count"`
	LockedUntil        time.Time `json:"locked_until,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Session struct {
	ID                   string         `json:"id"`
	UserID               string         `json:"user_id"`
	Status               string         `json:"status"`
	IssuedAt             time.Time      `json:"issued_at"`
	ExpiresAt            time.Time      `json:"expires_at"`
	LastSeenAt           time.Time      `json:"last_seen_at"`
	AuthenticationMethod string         `json:"authentication_method,omitempty"`
	CurrentLocationID    string         `json:"current_location_id,omitempty"`
	RevokedAt            time.Time      `json:"revoked_at,omitempty"`
	ClientMetadata       map[string]any `json:"client_metadata,omitempty"`
}

type SessionReview struct {
	ConcurrentActiveSessions int      `json:"concurrent_active_sessions"`
	Flags                    []string `json:"flags,omitempty"`
}

type ServicePrincipal struct {
	ID                    string    `json:"id"`
	Key                   string    `json:"key"`
	Status                string    `json:"status"`
	AllowedOperationTypes []string  `json:"allowed_operation_types,omitempty"`
	CredentialRef         string    `json:"credential_ref,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type Decision struct {
	Allowed     bool     `json:"allowed"`
	Constraints []string `json:"constraints,omitempty"`
	Reason      string   `json:"reason,omitempty"`
}
