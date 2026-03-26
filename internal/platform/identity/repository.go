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
	TOTPEnrollments() []TOTPEnrollment
	AuthChallenges() []AuthChallenge
	ServicePrincipals() []ServicePrincipal
	DelegationGrants() []DelegationGrant
	DeepLinkGrants() []DeepLinkGrant
	ReportingLines() []ReportingLine
	SaveUser(user User) error
	SaveRole(role Role) error
	SavePermission(permission Permission) error
	SaveRoleBinding(binding RoleBinding) error
	SaveRolePermission(grant RolePermission) error
	DeleteRolePermission(roleID, permissionKey string) error
	SaveServicePrincipal(principal ServicePrincipal) error
	SaveDelegationGrant(grant DelegationGrant) error
	SaveDeepLinkGrant(grant DeepLinkGrant) error
	SaveReportingLine(line ReportingLine) error
	FindUser(id string) (User, bool)
	FindUserByUsername(username string) (User, bool)
	FindUserByAuthenticationSubject(subject string) (User, bool)
	FindCredentialByUserID(userID string) (Credential, bool)
	FindSession(id string) (Session, bool)
	FindTOTPEnrollmentByUserID(userID string) (TOTPEnrollment, bool)
	FindAuthChallenge(id string) (AuthChallenge, bool)
	FindServicePrincipal(id string) (ServicePrincipal, bool)
	FindDelegationGrant(id string) (DelegationGrant, bool)
	FindDeepLinkGrant(id string) (DeepLinkGrant, bool)
	CountRecentLoginFailures(key string, since time.Time) int
	RecordLoginFailure(key string, attemptedAt time.Time) error
	ClearLoginFailures(key string) error
	CleanupLoginFailures(before time.Time) error
	SaveCredential(credential Credential) error
	SaveSession(session Session) error
	SaveTOTPEnrollment(enrollment TOTPEnrollment) error
	SaveAuthChallenge(challenge AuthChallenge) error
}
