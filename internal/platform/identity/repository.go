package identity

import "time"

type Repository interface {
	Users() []User
	Roles() []Role
	Permissions() []Permission
	RoleBindings() []RoleBinding
	RolePermissions() []RolePermission
	Credentials() []Credential
	Sessions() []Session
	ServicePrincipals() []ServicePrincipal
	DelegationGrants() []DelegationGrant
	ReportingLines() []ReportingLine
	SaveUser(user User) error
	SaveRole(role Role) error
	SavePermission(permission Permission) error
	SaveRoleBinding(binding RoleBinding) error
	SaveRolePermission(grant RolePermission) error
	SaveServicePrincipal(principal ServicePrincipal) error
	SaveDelegationGrant(grant DelegationGrant) error
	SaveReportingLine(line ReportingLine) error
	FindUser(id string) (User, bool)
	FindUserByUsername(username string) (User, bool)
	FindUserByAuthenticationSubject(subject string) (User, bool)
	FindCredentialByUserID(userID string) (Credential, bool)
	FindSession(id string) (Session, bool)
	FindServicePrincipal(id string) (ServicePrincipal, bool)
	FindDelegationGrant(id string) (DelegationGrant, bool)
	CountRecentLoginFailures(key string, since time.Time) int
	RecordLoginFailure(key string, attemptedAt time.Time) error
	ClearLoginFailures(key string) error
	CleanupLoginFailures(before time.Time) error
	SaveCredential(credential Credential) error
	SaveSession(session Session) error
}
