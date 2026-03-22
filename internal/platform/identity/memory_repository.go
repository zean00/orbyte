package identity

import "time"

type MemoryRepository struct {
	users             []User
	roles             []Role
	permissions       []Permission
	bindings          []RoleBinding
	grants            []RolePermission
	credentials       []Credential
	sessions          []Session
	servicePrincipals []ServicePrincipal
	delegationGrants  []DelegationGrant
	reportingLines    []ReportingLine
	loginFailures     map[string][]time.Time
}

func NewMemoryRepository(users []User, roles []Role, permissions []Permission, bindings []RoleBinding, grants []RolePermission, credentials []Credential, sessions []Session, servicePrincipals []ServicePrincipal) *MemoryRepository {
	return &MemoryRepository{
		users:             append([]User(nil), users...),
		roles:             append([]Role(nil), roles...),
		permissions:       append([]Permission(nil), permissions...),
		bindings:          append([]RoleBinding(nil), bindings...),
		grants:            append([]RolePermission(nil), grants...),
		credentials:       append([]Credential(nil), credentials...),
		sessions:          append([]Session(nil), sessions...),
		servicePrincipals: append([]ServicePrincipal(nil), servicePrincipals...),
		delegationGrants:  []DelegationGrant{},
		reportingLines:    []ReportingLine{},
		loginFailures:     make(map[string][]time.Time),
	}
}

func (r *MemoryRepository) Users() []User {
	return append([]User(nil), r.users...)
}

func (r *MemoryRepository) Roles() []Role {
	return append([]Role(nil), r.roles...)
}

func (r *MemoryRepository) Permissions() []Permission {
	return append([]Permission(nil), r.permissions...)
}

func (r *MemoryRepository) RoleBindings() []RoleBinding {
	return append([]RoleBinding(nil), r.bindings...)
}

func (r *MemoryRepository) RolePermissions() []RolePermission {
	return append([]RolePermission(nil), r.grants...)
}

func (r *MemoryRepository) Credentials() []Credential {
	return append([]Credential(nil), r.credentials...)
}

func (r *MemoryRepository) Sessions() []Session {
	return append([]Session(nil), r.sessions...)
}

func (r *MemoryRepository) ServicePrincipals() []ServicePrincipal {
	return append([]ServicePrincipal(nil), r.servicePrincipals...)
}

func (r *MemoryRepository) DelegationGrants() []DelegationGrant {
	return append([]DelegationGrant(nil), r.delegationGrants...)
}

func (r *MemoryRepository) ReportingLines() []ReportingLine {
	return append([]ReportingLine(nil), r.reportingLines...)
}

func (r *MemoryRepository) SaveUser(user User) error {
	for i, current := range r.users {
		if current.ID == user.ID {
			r.users[i] = user
			return nil
		}
	}
	r.users = append(r.users, user)
	return nil
}

func (r *MemoryRepository) SaveRole(role Role) error {
	for i, current := range r.roles {
		if current.ID == role.ID {
			r.roles[i] = role
			return nil
		}
	}
	r.roles = append(r.roles, role)
	return nil
}

func (r *MemoryRepository) SavePermission(permission Permission) error {
	for i, current := range r.permissions {
		if current.Key == permission.Key {
			r.permissions[i] = permission
			return nil
		}
	}
	r.permissions = append(r.permissions, permission)
	return nil
}

func (r *MemoryRepository) SaveRoleBinding(binding RoleBinding) error {
	for i, current := range r.bindings {
		if current.ID == binding.ID {
			r.bindings[i] = binding
			return nil
		}
	}
	r.bindings = append(r.bindings, binding)
	return nil
}

func (r *MemoryRepository) SaveRolePermission(grant RolePermission) error {
	for i, current := range r.grants {
		if current.RoleID == grant.RoleID && current.PermissionKey == grant.PermissionKey {
			r.grants[i] = grant
			return nil
		}
	}
	r.grants = append(r.grants, grant)
	return nil
}

func (r *MemoryRepository) DeleteRolePermission(roleID, permissionKey string) error {
	filtered := r.grants[:0]
	for _, grant := range r.grants {
		if grant.RoleID == roleID && grant.PermissionKey == permissionKey {
			continue
		}
		filtered = append(filtered, grant)
	}
	r.grants = filtered
	return nil
}

func (r *MemoryRepository) FindUser(id string) (User, bool) {
	for _, user := range r.users {
		if user.ID == id {
			return user, true
		}
	}
	return User{}, false
}

func (r *MemoryRepository) FindUserByUsername(username string) (User, bool) {
	for _, user := range r.users {
		if user.Username == username {
			return user, true
		}
	}
	return User{}, false
}

func (r *MemoryRepository) FindUserByAuthenticationSubject(subject string) (User, bool) {
	for _, user := range r.users {
		if user.AuthenticationSubject == subject {
			return user, true
		}
	}
	return User{}, false
}

func (r *MemoryRepository) FindSession(id string) (Session, bool) {
	for _, session := range r.sessions {
		if session.ID == id {
			return session, true
		}
	}
	return Session{}, false
}

func (r *MemoryRepository) FindCredentialByUserID(userID string) (Credential, bool) {
	for _, credential := range r.credentials {
		if credential.UserID == userID {
			return credential, true
		}
	}
	return Credential{}, false
}

func (r *MemoryRepository) FindServicePrincipal(id string) (ServicePrincipal, bool) {
	for _, principal := range r.servicePrincipals {
		if principal.ID == id {
			return principal, true
		}
	}
	return ServicePrincipal{}, false
}

func (r *MemoryRepository) FindDelegationGrant(id string) (DelegationGrant, bool) {
	for _, grant := range r.delegationGrants {
		if grant.ID == id {
			return grant, true
		}
	}
	return DelegationGrant{}, false
}

func (r *MemoryRepository) SaveServicePrincipal(principal ServicePrincipal) error {
	for i, current := range r.servicePrincipals {
		if current.ID == principal.ID {
			r.servicePrincipals[i] = principal
			return nil
		}
	}
	r.servicePrincipals = append(r.servicePrincipals, principal)
	return nil
}

func (r *MemoryRepository) SaveDelegationGrant(grant DelegationGrant) error {
	for i, current := range r.delegationGrants {
		if current.ID == grant.ID {
			r.delegationGrants[i] = grant
			return nil
		}
	}
	r.delegationGrants = append(r.delegationGrants, grant)
	return nil
}

func (r *MemoryRepository) SaveReportingLine(line ReportingLine) error {
	for i, current := range r.reportingLines {
		if current.ID == line.ID {
			r.reportingLines[i] = line
			return nil
		}
	}
	r.reportingLines = append(r.reportingLines, line)
	return nil
}

func (r *MemoryRepository) SaveCredential(credential Credential) error {
	for i, current := range r.credentials {
		if current.UserID == credential.UserID {
			r.credentials[i] = credential
			return nil
		}
	}
	r.credentials = append(r.credentials, credential)
	return nil
}

func (r *MemoryRepository) SaveSession(session Session) error {
	for i, current := range r.sessions {
		if current.ID == session.ID {
			r.sessions[i] = session
			return nil
		}
	}
	r.sessions = append(r.sessions, session)
	return nil
}

func (r *MemoryRepository) CountRecentLoginFailures(key string, since time.Time) int {
	failures := r.loginFailures[key]
	if len(failures) == 0 {
		return 0
	}
	filtered := failures[:0]
	for _, attemptedAt := range failures {
		if !attemptedAt.Before(since) {
			filtered = append(filtered, attemptedAt)
		}
	}
	if len(filtered) == 0 {
		delete(r.loginFailures, key)
		return 0
	}
	r.loginFailures[key] = filtered
	return len(filtered)
}

func (r *MemoryRepository) RecordLoginFailure(key string, attemptedAt time.Time) error {
	r.loginFailures[key] = append(r.loginFailures[key], attemptedAt)
	return nil
}

func (r *MemoryRepository) ClearLoginFailures(key string) error {
	delete(r.loginFailures, key)
	return nil
}

func (r *MemoryRepository) CleanupLoginFailures(before time.Time) error {
	for key, failures := range r.loginFailures {
		filtered := failures[:0]
		for _, attemptedAt := range failures {
			if !attemptedAt.Before(before) {
				filtered = append(filtered, attemptedAt)
			}
		}
		if len(filtered) == 0 {
			delete(r.loginFailures, key)
			continue
		}
		r.loginFailures[key] = filtered
	}
	return nil
}
