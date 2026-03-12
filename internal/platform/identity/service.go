package identity

import (
	"fmt"
	"os"
	"strings"
	"time"

	"clinic/internal/platform/organization"
	"clinic/internal/platform/shared"
)

type Service struct {
	organization *organization.Service
	repo         Repository
}

type bootstrapData struct {
	roles             []Role
	permissions       []Permission
	users             []User
	credentials       []Credential
	sessions          []Session
	servicePrincipals []ServicePrincipal
	bindings          []RoleBinding
	grants            []RolePermission
}

const (
	defaultSessionTTL        = 8 * time.Hour
	maxFailedPasswordAttempt = 5
	passwordLockoutWindow    = 15 * time.Minute
	minPasswordLength        = 8
)

func NewService(org *organization.Service) *Service {
	now := time.Now().UTC()
	data := defaultBootstrapData(now, defaultBootstrapAdminPassword())
	return NewServiceWithRepository(org, NewMemoryRepository(data.users, data.roles, data.permissions, data.bindings, data.grants, data.credentials, data.sessions, data.servicePrincipals))
}

func defaultBootstrapData(now time.Time, bootstrapPassword string) bootstrapData {
	roles := []Role{{
		ID:        "role_admin",
		Key:       "platform_admin",
		Name:      "Platform Administrator",
		ScopeType: "deployment",
		CreatedAt: now,
		UpdatedAt: now,
	}}
	permissions := []Permission{{
		Key:      "document.create",
		Module:   "document",
		Action:   "create",
		Resource: "document",
	}, {
		Key:      "document.submit",
		Module:   "document",
		Action:   "submit",
		Resource: "document",
	}, {
		Key:      "document.approve",
		Module:   "document",
		Action:   "approve",
		Resource: "document",
	}, {
		Key:      "document.reject",
		Module:   "document",
		Action:   "reject",
		Resource: "document",
	}, {
		Key:      "document.reopen",
		Module:   "document",
		Action:   "reopen",
		Resource: "document",
	}, {
		Key:      "document.cancel",
		Module:   "document",
		Action:   "cancel",
		Resource: "document",
	}, {
		Key:      "document.read",
		Module:   "document",
		Action:   "read",
		Resource: "document",
	}, {
		Key:      "document.list",
		Module:   "document",
		Action:   "list",
		Resource: "document",
	}, {
		Key:      "document.update_draft",
		Module:   "document",
		Action:   "update_draft",
		Resource: "document",
	}, {
		Key:      "platform.context.read",
		Module:   "platform",
		Action:   "read",
		Resource: "context",
	}, {
		Key:      "audit.read",
		Module:   "audit",
		Action:   "read",
		Resource: "audit_event",
	}, {
		Key:      "event.read",
		Module:   "eventing",
		Action:   "read",
		Resource: "domain_event",
	}, {
		Key:      "outbox.read",
		Module:   "eventing",
		Action:   "read",
		Resource: "outbox",
	}, {
		Key:      "outbox.dispatch",
		Module:   "eventing",
		Action:   "dispatch",
		Resource: "outbox",
	}, {
		Key:      "deadletter.read",
		Module:   "eventing",
		Action:   "read",
		Resource: "dead_letter",
	}, {
		Key:      "metrics.read",
		Module:   "monitoring",
		Action:   "read",
		Resource: "metrics",
	}, {
		Key:      "monitoring.read",
		Module:   "monitoring",
		Action:   "read",
		Resource: "dashboard",
	}, {
		Key:      "analytics.read",
		Module:   "analytics",
		Action:   "read",
		Resource: "analytics",
	}, {
		Key:      "analytics.manage_reports",
		Module:   "analytics",
		Action:   "manage_reports",
		Resource: "analytics_report",
	}, {
		Key:      "analytics.deliver_reports",
		Module:   "analytics",
		Action:   "deliver_reports",
		Resource: "analytics_report",
	}, {
		Key:      "module.read",
		Module:   "module",
		Action:   "read",
		Resource: "module",
	}, {
		Key:      "module.manage",
		Module:   "module",
		Action:   "manage",
		Resource: "module",
	}, {
		Key:      "configuration.read",
		Module:   "configuration",
		Action:   "read",
		Resource: "configuration",
	}, {
		Key:      "configuration.manage",
		Module:   "configuration",
		Action:   "manage",
		Resource: "configuration",
	}, {
		Key:      "identity.manage_sessions",
		Module:   "identity",
		Action:   "manage",
		Resource: "session",
	}, {
		Key:      "identity.manage_users",
		Module:   "identity",
		Action:   "manage",
		Resource: "user",
	}}
	users := []User{{
		ID:                "user_admin",
		Username:          "admin",
		Status:            "active",
		DefaultLocationID: "loc_hq",
		CreatedAt:         now,
		UpdatedAt:         now,
	}}
	credentials := []Credential(nil)
	if bootstrapPassword != "" {
		adminPasswordHash, err := HashPassword(bootstrapPassword)
		if err != nil {
			panic(err)
		}
		credentials = append(credentials, Credential{
			UserID:            "user_admin",
			PasswordHash:      adminPasswordHash,
			PasswordChangedAt: now,
			UpdatedAt:         now,
		})
	}
	sessions := []Session{{
		ID:                   "sess_admin",
		UserID:               "user_admin",
		Status:               "active",
		IssuedAt:             now,
		ExpiresAt:            now.Add(8 * time.Hour),
		LastSeenAt:           now,
		AuthenticationMethod: "bootstrap",
		CurrentLocationID:    "loc_hq",
		ClientMetadata:       map[string]any{"source": "bootstrap"},
	}}
	servicePrincipals := []ServicePrincipal{{
		ID:                    "sp_projection_worker",
		Key:                   "projection_worker",
		Status:                "active",
		AllowedOperationTypes: []string{"projection.refresh", "outbox.dispatch"},
		CredentialRef:         "bootstrap://projection-worker",
		CreatedAt:             now,
		UpdatedAt:             now,
	}}
	bindings := []RoleBinding{{
		ID:            "rb_admin",
		UserID:        "user_admin",
		RoleID:        "role_admin",
		ScopeType:     "deployment",
		EffectiveFrom: now,
		Status:        "active",
	}}
	grants := []RolePermission{{
		RoleID:        "role_admin",
		PermissionKey: "document.create",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "document.submit",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "document.approve",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "document.reject",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "document.reopen",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "document.cancel",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "document.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "document.list",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "document.update_draft",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "platform.context.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "audit.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "event.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "outbox.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "outbox.dispatch",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "deadletter.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "metrics.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "monitoring.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "analytics.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "analytics.manage_reports",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "analytics.deliver_reports",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "module.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "module.manage",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "configuration.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "configuration.manage",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "identity.manage_sessions",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "identity.manage_users",
	}}
	return bootstrapData{
		roles:             roles,
		permissions:       permissions,
		users:             users,
		credentials:       credentials,
		sessions:          sessions,
		servicePrincipals: servicePrincipals,
		bindings:          bindings,
		grants:            grants,
	}
}

func NewServiceWithRepository(org *organization.Service, repo Repository) *Service {
	return &Service{organization: org, repo: repo}
}

func (s *Service) SeedBootstrapData(password string) error {
	now := time.Now().UTC()
	data := defaultBootstrapData(now, strings.TrimSpace(password))
	for _, role := range data.roles {
		if err := s.repo.SaveRole(role); err != nil {
			return err
		}
	}
	for _, permission := range data.permissions {
		if err := s.repo.SavePermission(permission); err != nil {
			return err
		}
	}
	for _, user := range data.users {
		if err := s.repo.SaveUser(user); err != nil {
			return err
		}
	}
	for _, binding := range data.bindings {
		if err := s.repo.SaveRoleBinding(binding); err != nil {
			return err
		}
	}
	for _, grant := range data.grants {
		if err := s.repo.SaveRolePermission(grant); err != nil {
			return err
		}
	}
	for _, credential := range data.credentials {
		if err := s.repo.SaveCredential(credential); err != nil {
			return err
		}
	}
	for _, session := range data.sessions {
		if err := s.repo.SaveSession(session); err != nil {
			return err
		}
	}
	for _, principal := range data.servicePrincipals {
		if err := s.repo.SaveServicePrincipal(principal); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Roles() []Role {
	return s.repo.Roles()
}

func (s *Service) Permissions() []Permission {
	return s.repo.Permissions()
}

func (s *Service) RolePermissions() []RolePermission {
	return s.repo.RolePermissions()
}

func (s *Service) Users() []User {
	return s.repo.Users()
}

func (s *Service) FindUser(id string) (User, bool) {
	return s.repo.FindUser(id)
}

func (s *Service) Bindings() []RoleBinding {
	return s.repo.RoleBindings()
}

func (s *Service) Sessions() []Session {
	return s.repo.Sessions()
}

func (s *Service) Credentials() []Credential {
	return s.repo.Credentials()
}

func (s *Service) ServicePrincipals() []ServicePrincipal {
	return s.repo.ServicePrincipals()
}

func (s *Service) FindSession(id string) (Session, bool) {
	return s.repo.FindSession(id)
}

func (s *Service) FindUserByUsername(username string) (User, bool) {
	return s.repo.FindUserByUsername(username)
}

func (s *Service) FindCredentialByUserID(userID string) (Credential, bool) {
	return s.repo.FindCredentialByUserID(userID)
}

func (s *Service) FindServicePrincipal(id string) (ServicePrincipal, bool) {
	return s.repo.FindServicePrincipal(id)
}

func (s *Service) CountRecentLoginFailures(key string, since time.Time) int {
	return s.repo.CountRecentLoginFailures(key, since)
}

func (s *Service) RecordLoginFailure(key string, attemptedAt time.Time) error {
	return s.repo.RecordLoginFailure(key, attemptedAt)
}

func (s *Service) ClearLoginFailures(key string) error {
	return s.repo.ClearLoginFailures(key)
}

func (s *Service) CleanupLoginFailures(before time.Time) error {
	return s.repo.CleanupLoginFailures(before)
}

func (s *Service) CreateUser(username, password, defaultLocationID, roleID, scopeType, scopeID string) (User, error) {
	return s.CreateUserWithPasswordPolicy(username, password, defaultLocationID, roleID, scopeType, scopeID, minPasswordLength)
}

func (s *Service) CreateUserWithPasswordPolicy(username, password, defaultLocationID, roleID, scopeType, scopeID string, passwordMinLength int) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return User{}, shared.Validation("username is required")
	}
	if _, ok := s.repo.FindUserByUsername(username); ok {
		return User{}, shared.Conflict("username already exists")
	}
	if err := validateNewPassword(password, passwordMinLength); err != nil {
		return User{}, err
	}
	if roleID == "" {
		roleID = "role_admin"
	}
	if !s.roleExists(roleID) {
		return User{}, shared.Validation("role_id is invalid")
	}
	if scopeType == "" {
		scopeType = "deployment"
	}
	now := time.Now().UTC()
	user := User{
		ID:                fmt.Sprintf("user:%d", now.UnixNano()),
		Username:          username,
		Status:            "active",
		DefaultLocationID: defaultLocationID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.SaveUser(user); err != nil {
		return User{}, err
	}
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	if err := s.repo.SaveCredential(Credential{
		UserID:            user.ID,
		PasswordHash:      hashedPassword,
		PasswordChangedAt: now,
		UpdatedAt:         now,
	}); err != nil {
		return User{}, err
	}
	if err := s.repo.SaveRoleBinding(RoleBinding{
		ID:            fmt.Sprintf("rb:%d", now.UnixNano()),
		UserID:        user.ID,
		RoleID:        roleID,
		ScopeType:     scopeType,
		ScopeID:       scopeID,
		EffectiveFrom: now,
		Status:        "active",
	}); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Service) AuthenticatePassword(username, password, locationID string, clientMetadata map[string]any, ttl time.Duration) (Session, error) {
	user, ok := s.repo.FindUserByUsername(username)
	if !ok {
		return Session{}, shared.Unauthorized("invalid credentials")
	}
	if user.Status != "active" {
		return Session{}, shared.Forbidden("user not active")
	}
	credential, ok := s.repo.FindCredentialByUserID(user.ID)
	if !ok {
		return Session{}, shared.Unauthorized("invalid credentials")
	}
	now := time.Now().UTC()
	if !credential.LockedUntil.IsZero() && credential.LockedUntil.After(now) {
		return Session{}, shared.Forbidden("account temporarily locked")
	}
	verified, err := VerifyPassword(credential.PasswordHash, password)
	if err != nil {
		return Session{}, err
	}
	if !verified {
		credential.FailedAttemptCount++
		if credential.FailedAttemptCount >= maxFailedPasswordAttempt {
			credential.LockedUntil = now.Add(passwordLockoutWindow)
			credential.FailedAttemptCount = 0
		}
		credential.UpdatedAt = now
		if saveErr := s.repo.SaveCredential(credential); saveErr != nil {
			return Session{}, saveErr
		}
		return Session{}, shared.Unauthorized("invalid credentials")
	}
	credential.FailedAttemptCount = 0
	credential.LockedUntil = time.Time{}
	credential.UpdatedAt = now
	if err := s.repo.SaveCredential(credential); err != nil {
		return Session{}, err
	}
	return s.StartSession(username, locationID, "password", clientMetadata, ttl)
}

func (s *Service) StartSession(username, locationID, authenticationMethod string, clientMetadata map[string]any, ttl time.Duration) (Session, error) {
	user, ok := s.repo.FindUserByUsername(username)
	if !ok {
		return Session{}, shared.Unauthorized("invalid credentials")
	}
	if user.Status != "active" {
		return Session{}, shared.Forbidden("user not active")
	}
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	if locationID == "" {
		locationID = user.DefaultLocationID
	}
	now := time.Now().UTC()
	session := Session{
		ID:                   fmt.Sprintf("sess:%d", now.UnixNano()),
		UserID:               user.ID,
		Status:               "active",
		IssuedAt:             now,
		ExpiresAt:            now.Add(ttl),
		LastSeenAt:           now,
		AuthenticationMethod: authenticationMethod,
		CurrentLocationID:    locationID,
		ClientMetadata:       cloneMetadata(clientMetadata),
	}
	if err := s.repo.SaveSession(session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) ChangePassword(userID, currentPassword, newPassword string) error {
	return s.ChangePasswordWithPolicy(userID, currentPassword, newPassword, minPasswordLength)
}

func (s *Service) ChangePasswordWithPolicy(userID, currentPassword, newPassword string, passwordMinLength int) error {
	if err := validateNewPassword(newPassword, passwordMinLength); err != nil {
		return err
	}
	user, ok := s.repo.FindUser(userID)
	if !ok {
		return shared.NotFound("user not found")
	}
	credential, ok := s.repo.FindCredentialByUserID(user.ID)
	if !ok {
		return shared.NotFound("credential not found")
	}
	verified, err := VerifyPassword(credential.PasswordHash, currentPassword)
	if err != nil {
		return err
	}
	if !verified {
		return shared.Unauthorized("current password is invalid")
	}
	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	credential.PasswordHash = hashedPassword
	credential.PasswordChangedAt = now
	credential.FailedAttemptCount = 0
	credential.LockedUntil = time.Time{}
	credential.UpdatedAt = now
	if err := s.repo.SaveCredential(credential); err != nil {
		return err
	}
	return s.revokeUserSessions(user.ID, now)
}

func (s *Service) ResetPassword(userID, newPassword string) error {
	return s.ResetPasswordWithPolicy(userID, newPassword, minPasswordLength)
}

func (s *Service) ResetPasswordWithPolicy(userID, newPassword string, passwordMinLength int) error {
	if err := validateNewPassword(newPassword, passwordMinLength); err != nil {
		return err
	}
	user, ok := s.repo.FindUser(userID)
	if !ok {
		return shared.NotFound("user not found")
	}
	credential, ok := s.repo.FindCredentialByUserID(user.ID)
	if !ok {
		return shared.NotFound("credential not found")
	}
	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	credential.PasswordHash = hashedPassword
	credential.PasswordChangedAt = now
	credential.FailedAttemptCount = 0
	credential.LockedUntil = time.Time{}
	credential.UpdatedAt = now
	if err := s.repo.SaveCredential(credential); err != nil {
		return err
	}
	return s.revokeUserSessions(user.ID, now)
}

func (s *Service) EnsureBootstrapAdminCredential(password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return nil
	}
	user, ok := s.repo.FindUserByUsername("admin")
	if !ok {
		return shared.NotFound("bootstrap admin user not found")
	}
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	credential := Credential{
		UserID:            user.ID,
		PasswordHash:      hashedPassword,
		PasswordChangedAt: now,
		UpdatedAt:         now,
	}
	if existing, ok := s.repo.FindCredentialByUserID(user.ID); ok {
		credential.FailedAttemptCount = 0
		credential.LockedUntil = time.Time{}
		credential.UpdatedAt = now
		_ = existing
	}
	return s.repo.SaveCredential(credential)
}

func (s *Service) UpsertRole(role Role) error {
	if strings.TrimSpace(role.ID) == "" || strings.TrimSpace(role.Key) == "" || strings.TrimSpace(role.Name) == "" {
		return shared.Validation("role id, key, and name are required")
	}
	now := time.Now().UTC()
	if role.CreatedAt.IsZero() {
		role.CreatedAt = now
	}
	role.UpdatedAt = now
	return s.repo.SaveRole(role)
}

func (s *Service) UpsertPermission(permission Permission) error {
	if strings.TrimSpace(permission.Key) == "" || strings.TrimSpace(permission.Action) == "" || strings.TrimSpace(permission.Resource) == "" {
		return shared.Validation("permission key, action, and resource are required")
	}
	return s.repo.SavePermission(permission)
}

func (s *Service) GrantRolePermission(grant RolePermission) error {
	if strings.TrimSpace(grant.RoleID) == "" || strings.TrimSpace(grant.PermissionKey) == "" {
		return shared.Validation("role permission grant is invalid")
	}
	return s.repo.SaveRolePermission(grant)
}

func (s *Service) RefreshSession(sessionID string, ttl time.Duration) (Session, error) {
	return s.RotateSession(sessionID, ttl)
}

func (s *Service) RotateSession(sessionID string, ttl time.Duration) (Session, error) {
	session, ok := s.repo.FindSession(sessionID)
	if !ok {
		return Session{}, shared.NotFound("session not found")
	}
	if session.Status != "active" || !session.RevokedAt.IsZero() {
		return Session{}, shared.Unauthorized("session is not refreshable")
	}
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	now := time.Now().UTC()
	session.Status = "revoked"
	session.RevokedAt = now
	session.LastSeenAt = now
	if err := s.repo.SaveSession(session); err != nil {
		return Session{}, err
	}
	rotated := Session{
		ID:                   fmt.Sprintf("sess:%d", now.UnixNano()),
		UserID:               session.UserID,
		Status:               "active",
		IssuedAt:             now,
		ExpiresAt:            now.Add(ttl),
		LastSeenAt:           now,
		AuthenticationMethod: session.AuthenticationMethod,
		CurrentLocationID:    session.CurrentLocationID,
		ClientMetadata:       cloneMetadata(session.ClientMetadata),
	}
	if err := s.repo.SaveSession(rotated); err != nil {
		return Session{}, err
	}
	return rotated, nil
}

func (s *Service) SetUserStatus(userID, status string) (User, error) {
	user, ok := s.repo.FindUser(userID)
	if !ok {
		return User{}, shared.NotFound("user not found")
	}
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case "active", "disabled":
	default:
		return User{}, shared.Validation("status must be active or disabled")
	}
	now := time.Now().UTC()
	user.Status = status
	user.UpdatedAt = now
	if err := s.repo.SaveUser(user); err != nil {
		return User{}, err
	}
	if status != "active" {
		if err := s.revokeUserSessions(user.ID, now); err != nil {
			return User{}, err
		}
	}
	return user, nil
}

func (s *Service) TouchSession(sessionID string, seenAt time.Time) (Session, error) {
	session, ok := s.repo.FindSession(sessionID)
	if !ok {
		return Session{}, shared.NotFound("session not found")
	}
	if seenAt.IsZero() {
		seenAt = time.Now().UTC()
	}
	session.LastSeenAt = seenAt.UTC()
	if err := s.repo.SaveSession(session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) RevokeSession(sessionID string, revokedAt time.Time) (Session, error) {
	session, ok := s.repo.FindSession(sessionID)
	if !ok {
		return Session{}, shared.NotFound("session not found")
	}
	if revokedAt.IsZero() {
		revokedAt = time.Now().UTC()
	}
	session.Status = "revoked"
	session.RevokedAt = revokedAt.UTC()
	session.LastSeenAt = revokedAt.UTC()
	if err := s.repo.SaveSession(session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) Decide(userID, permissionKey, locationID string) Decision {
	if userID == "" {
		return Decision{Allowed: false, Reason: "missing user"}
	}
	if permissionKey == "" {
		return Decision{Allowed: false, Reason: "missing permission"}
	}
	user, ok := s.repo.FindUser(userID)
	if !ok || user.Status != "active" {
		return Decision{Allowed: false, Reason: "user not active"}
	}
	if !s.userHasPermission(userID, permissionKey, locationID) {
		return Decision{Allowed: false, Reason: "permission denied"}
	}
	ctx := s.organization.Resolve(locationID)
	decision := Decision{Allowed: true}
	if ctx.LocationID == "" && locationID != "" {
		decision.Constraints = append(decision.Constraints, "requested_location_not_found")
	}
	return decision
}

func (s *Service) DecideSession(sessionID, permissionKey, locationID string) Decision {
	if sessionID == "" {
		return Decision{Allowed: false, Reason: "missing session"}
	}
	session, ok := s.repo.FindSession(sessionID)
	if !ok {
		return Decision{Allowed: false, Reason: "session not found"}
	}
	if session.Status != "active" {
		return Decision{Allowed: false, Reason: "session not active"}
	}
	if !session.RevokedAt.IsZero() {
		return Decision{Allowed: false, Reason: "session revoked"}
	}
	if !session.ExpiresAt.IsZero() && session.ExpiresAt.Before(time.Now().UTC()) {
		return Decision{Allowed: false, Reason: "session expired"}
	}
	if locationID == "" {
		locationID = session.CurrentLocationID
	}
	return s.Decide(session.UserID, permissionKey, locationID)
}

func (s *Service) DecideServicePrincipal(principalID, operationType string) Decision {
	if principalID == "" {
		return Decision{Allowed: false, Reason: "missing service principal"}
	}
	if operationType == "" {
		return Decision{Allowed: false, Reason: "missing operation type"}
	}
	principal, ok := s.repo.FindServicePrincipal(principalID)
	if !ok {
		return Decision{Allowed: false, Reason: "service principal not found"}
	}
	if principal.Status != "active" {
		return Decision{Allowed: false, Reason: "service principal not active"}
	}
	for _, allowed := range principal.AllowedOperationTypes {
		if allowed == operationType {
			return Decision{Allowed: true}
		}
	}
	return Decision{Allowed: false, Reason: "operation not allowed"}
}

func (s *Service) userHasPermission(userID, permissionKey, locationID string) bool {
	now := time.Now().UTC()
	allowedRoleIDs := make(map[string]struct{})
	for _, binding := range s.repo.RoleBindings() {
		if binding.UserID != userID || binding.Status != "active" {
			continue
		}
		if binding.EffectiveFrom.After(now) {
			continue
		}
		if !binding.EffectiveTo.IsZero() && binding.EffectiveTo.Before(now) {
			continue
		}
		if binding.ScopeType == "location" && binding.ScopeID != "" && locationID != "" && binding.ScopeID != locationID {
			continue
		}
		allowedRoleIDs[binding.RoleID] = struct{}{}
	}
	if len(allowedRoleIDs) == 0 {
		return false
	}
	for _, grant := range s.repo.RolePermissions() {
		if grant.PermissionKey != permissionKey {
			continue
		}
		if _, ok := allowedRoleIDs[grant.RoleID]; ok {
			return true
		}
	}
	return false
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func defaultBootstrapAdminPassword() string {
	password := strings.TrimSpace(os.Getenv("APP_BOOTSTRAP_ADMIN_PASSWORD"))
	if password == "" && allowDefaultBootstrapPassword() {
		password = "admin123!"
	}
	return password
}

func allowDefaultBootstrapPassword() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) {
	case "", "development", "dev", "test":
		return true
	default:
		return false
	}
}

func validateNewPassword(password string, minLength int) error {
	if strings.TrimSpace(password) == "" {
		return shared.Validation("new_password is required")
	}
	if minLength <= 0 {
		minLength = minPasswordLength
	}
	if len(password) < minLength {
		return shared.Validation(fmt.Sprintf("new_password must be at least %d characters", minLength))
	}
	return nil
}

func (s *Service) roleExists(roleID string) bool {
	for _, role := range s.repo.Roles() {
		if role.ID == roleID {
			return true
		}
	}
	return false
}

func (s *Service) revokeUserSessions(userID string, revokedAt time.Time) error {
	for _, session := range s.repo.Sessions() {
		if session.UserID != userID || session.Status != "active" || !session.RevokedAt.IsZero() {
			continue
		}
		session.Status = "revoked"
		session.RevokedAt = revokedAt
		session.LastSeenAt = revokedAt
		if err := s.repo.SaveSession(session); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ReviewSession(sessionID string) (SessionReview, bool) {
	session, ok := s.repo.FindSession(sessionID)
	if !ok {
		return SessionReview{}, false
	}
	review := SessionReview{}
	remoteAddr, _ := session.ClientMetadata["remote_addr"].(string)
	userAgent, _ := session.ClientMetadata["user_agent"].(string)
	for _, candidate := range s.repo.Sessions() {
		if candidate.UserID != session.UserID || candidate.ID == session.ID {
			continue
		}
		if candidate.Status == "active" && candidate.RevokedAt.IsZero() {
			review.ConcurrentActiveSessions++
		}
		if remoteAddr != "" {
			if otherRemote, _ := candidate.ClientMetadata["remote_addr"].(string); otherRemote != "" && otherRemote != remoteAddr {
				review.Flags = appendIfMissing(review.Flags, "new_remote_addr")
			}
		}
		if userAgent != "" {
			if otherAgent, _ := candidate.ClientMetadata["user_agent"].(string); otherAgent != "" && otherAgent != userAgent {
				review.Flags = appendIfMissing(review.Flags, "new_user_agent")
			}
		}
	}
	if review.ConcurrentActiveSessions > 0 {
		review.Flags = appendIfMissing(review.Flags, "concurrent_active_sessions")
	}
	return review, true
}

func appendIfMissing(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
