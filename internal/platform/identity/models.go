package identity

import "time"

type User struct {
	ID                    string    `json:"id"`
	Username              string    `json:"username"`
	AuthenticationSubject string    `json:"authentication_subject,omitempty"`
	Status                string    `json:"status"`
	DefaultLocationID     string    `json:"default_location_id,omitempty"`
	PreferredLocale       string    `json:"preferred_locale,omitempty"`
	PreferredUserRoute    string    `json:"preferred_user_route,omitempty"`
	PreferredAdminRoute   string    `json:"preferred_admin_route,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type Role struct {
	ID                string    `json:"id"`
	Key               string    `json:"key"`
	Name              string    `json:"name"`
	ScopeType         string    `json:"scope_type"`
	DefaultUserRoute  string    `json:"default_user_route,omitempty"`
	DefaultAdminRoute string    `json:"default_admin_route,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
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
	Priority      int       `json:"priority,omitempty"`
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

type DelegationGrant struct {
	ID                    string    `json:"id"`
	GrantorUserID         string    `json:"grantor_user_id"`
	DelegateKind          string    `json:"delegate_kind,omitempty"`
	DelegateID            string    `json:"delegate_id,omitempty"`
	DelegateUserID        string    `json:"delegate_user_id"`
	Status                string    `json:"status"`
	LocationID            string    `json:"location_id"`
	AllowedPermissionKeys []string  `json:"allowed_permission_keys,omitempty"`
	AllowedDocumentTypes  []string  `json:"allowed_document_types,omitempty"`
	Reason                string    `json:"reason,omitempty"`
	StartsAt              time.Time `json:"starts_at"`
	ExpiresAt             time.Time `json:"expires_at"`
	AcceptedAt            time.Time `json:"accepted_at,omitempty"`
	AcceptedByKind        string    `json:"accepted_by_kind,omitempty"`
	AcceptedByID          string    `json:"accepted_by_id,omitempty"`
	AcceptedByUserID      string    `json:"accepted_by_user_id,omitempty"`
	RevokedAt             time.Time `json:"revoked_at,omitempty"`
	RevokedByUserID       string    `json:"revoked_by_user_id,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type DeepLinkGrant struct {
	ID                    string         `json:"id"`
	Kind                  string         `json:"kind"`
	UserID                string         `json:"user_id"`
	Status                string         `json:"status"`
	TargetType            string         `json:"target_type"`
	TargetID              string         `json:"target_id"`
	LocationID            string         `json:"location_id,omitempty"`
	AllowedPermissionKeys []string       `json:"allowed_permission_keys,omitempty"`
	AllowedActions        []string       `json:"allowed_actions,omitempty"`
	ReviewOnly            bool           `json:"review_only,omitempty"`
	RequireStepUp         bool           `json:"require_step_up,omitempty"`
	OneTime               bool           `json:"one_time,omitempty"`
	Title                 string         `json:"title,omitempty"`
	Message               string         `json:"message,omitempty"`
	StartsAt              time.Time      `json:"starts_at"`
	ExpiresAt             time.Time      `json:"expires_at"`
	ActivatedAt           time.Time      `json:"activated_at,omitempty"`
	ConsumedAt            time.Time      `json:"consumed_at,omitempty"`
	ConsumedByAction      string         `json:"consumed_by_action,omitempty"`
	RevokedAt             time.Time      `json:"revoked_at,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	Metadata              map[string]any `json:"metadata,omitempty"`
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

type ReportingLine struct {
	ID               string    `json:"id"`
	SubjectUserID    string    `json:"subject_user_id"`
	ManagerUserID    string    `json:"manager_user_id"`
	RelationshipType string    `json:"relationship_type"`
	OrganizationID   string    `json:"organization_id,omitempty"`
	LocationID       string    `json:"location_id,omitempty"`
	OperatingUnitID  string    `json:"operating_unit_id,omitempty"`
	Status           string    `json:"status"`
	Priority         int       `json:"priority,omitempty"`
	EffectiveFrom    time.Time `json:"effective_from"`
	EffectiveTo      time.Time `json:"effective_to,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ManagerResolution struct {
	Line    ReportingLine `json:"line"`
	Manager User          `json:"manager"`
	Via     string        `json:"via"`
}
