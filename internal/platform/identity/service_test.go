package identity

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/organization"
)

func TestServiceDecide(t *testing.T) {
	svc := NewService(organization.NewService())
	decision := svc.Decide("user_admin", "document.create", "loc_hq")
	if !decision.Allowed {
		t.Fatalf("expected allowed decision: %+v", decision)
	}
}

func TestServiceDecideRejectsMissingUser(t *testing.T) {
	svc := NewService(organization.NewService())
	decision := svc.Decide("", "document.create", "")
	if decision.Allowed {
		t.Fatal("expected deny for missing user")
	}
}

func TestMemoryRepositoryFindUser(t *testing.T) {
	now := time.Now().UTC()
	repo := NewMemoryRepository(
		[]User{{ID: "u1", Username: "admin", Status: "active"}},
		[]Role{{ID: "r1"}},
		[]Permission{{Key: "p1"}},
		[]RoleBinding{{ID: "rb1", UserID: "u1", RoleID: "r1", ScopeType: "deployment", EffectiveFrom: now, Status: "active"}},
		[]RolePermission{{RoleID: "r1", PermissionKey: "p1"}},
		nil,
		nil,
		nil,
	)
	if _, ok := repo.FindUser("u1"); !ok {
		t.Fatal("expected user lookup to succeed")
	}
	if _, ok := repo.FindUserByUsername("admin"); !ok {
		t.Fatal("expected username lookup to succeed")
	}
	if len(repo.Users()) != 1 || len(repo.Roles()) != 1 || len(repo.Permissions()) != 1 || len(repo.RoleBindings()) != 1 || len(repo.RolePermissions()) != 1 {
		t.Fatal("expected repository lists")
	}
	if err := repo.RecordLoginFailure("admin|192.0.2.10", now); err != nil {
		t.Fatalf("expected login failure record: %v", err)
	}
	if count := repo.CountRecentLoginFailures("admin|192.0.2.10", now.Add(-time.Minute)); count != 1 {
		t.Fatalf("expected login failure count, got %d", count)
	}
	if err := repo.ClearLoginFailures("admin|192.0.2.10"); err != nil {
		t.Fatalf("expected clear login failures: %v", err)
	}
	if count := repo.CountRecentLoginFailures("admin|192.0.2.10", now.Add(-time.Minute)); count != 0 {
		t.Fatalf("expected cleared login failures, got %d", count)
	}
	_ = repo.RecordLoginFailure("admin|192.0.2.10", now.Add(-2*time.Minute))
	_ = repo.RecordLoginFailure("admin|192.0.2.10", now)
	if err := repo.CleanupLoginFailures(now.Add(-time.Minute)); err != nil {
		t.Fatalf("expected cleanup login failures: %v", err)
	}
	if count := repo.CountRecentLoginFailures("admin|192.0.2.10", now.Add(-10*time.Minute)); count != 1 {
		t.Fatalf("expected one recent login failure after cleanup, got %d", count)
	}
}

func TestServiceDecideRejectsWithoutPermissionGrant(t *testing.T) {
	now := time.Now().UTC()
	repo := NewMemoryRepository(
		[]User{{ID: "u1", Status: "active"}},
		[]Role{{ID: "r1"}},
		[]Permission{{Key: "document.create"}},
		[]RoleBinding{{ID: "rb1", UserID: "u1", RoleID: "r1", ScopeType: "deployment", EffectiveFrom: now, Status: "active"}},
		nil,
		nil,
		nil,
		nil,
	)
	svc := NewServiceWithRepository(organization.NewService(), repo)
	decision := svc.Decide("u1", "document.create", "")
	if decision.Allowed {
		t.Fatal("expected deny without grant")
	}
}

func TestServiceDecideSession(t *testing.T) {
	svc := NewService(organization.NewService())
	decision := svc.DecideSession("sess_admin", "document.create", "")
	if !decision.Allowed {
		t.Fatalf("expected active session to authorize: %+v", decision)
	}
}

func TestServiceDecideSessionRejectsExpired(t *testing.T) {
	now := time.Now().UTC()
	repo := NewMemoryRepository(
		[]User{{ID: "u1", Status: "active"}},
		[]Role{{ID: "r1"}},
		[]Permission{{Key: "document.create"}},
		[]RoleBinding{{ID: "rb1", UserID: "u1", RoleID: "r1", ScopeType: "deployment", EffectiveFrom: now, Status: "active"}},
		[]RolePermission{{RoleID: "r1", PermissionKey: "document.create"}},
		nil,
		[]Session{{ID: "s1", UserID: "u1", Status: "active", IssuedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour), LastSeenAt: now.Add(-90 * time.Minute)}},
		nil,
	)
	svc := NewServiceWithRepository(organization.NewService(), repo)
	decision := svc.DecideSession("s1", "document.create", "")
	if decision.Allowed {
		t.Fatal("expected expired session to be denied")
	}
}

func TestServiceDecideServicePrincipal(t *testing.T) {
	svc := NewService(organization.NewService())
	decision := svc.DecideServicePrincipal("sp_projection_worker", "projection.refresh")
	if !decision.Allowed {
		t.Fatalf("expected service principal decision to allow: %+v", decision)
	}
}

func TestServiceDecideServicePrincipalRejectsUnknownOperation(t *testing.T) {
	svc := NewService(organization.NewService())
	decision := svc.DecideServicePrincipal("sp_projection_worker", "integration.submit")
	if decision.Allowed {
		t.Fatal("expected service principal operation denial")
	}
}

func TestServiceStartTouchAndRevokeSession(t *testing.T) {
	svc := NewService(organization.NewService())
	session, err := svc.StartSession("admin", "loc_hq", "username_bootstrap", map[string]any{"source": "test"}, time.Hour)
	if err != nil {
		t.Fatalf("expected session start to succeed: %v", err)
	}
	if session.UserID != "user_admin" {
		t.Fatalf("expected admin session user, got %s", session.UserID)
	}
	touchedAt := time.Now().UTC().Add(2 * time.Minute)
	session, err = svc.TouchSession(session.ID, touchedAt)
	if err != nil {
		t.Fatalf("expected touch session to succeed: %v", err)
	}
	if !session.LastSeenAt.Equal(touchedAt) {
		t.Fatalf("expected updated last_seen_at, got %s", session.LastSeenAt)
	}
	session, err = svc.RevokeSession(session.ID, touchedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("expected revoke session to succeed: %v", err)
	}
	if session.Status != "revoked" || session.RevokedAt.IsZero() {
		t.Fatalf("expected revoked session, got %+v", session)
	}
}

func TestSetUserPreferredLocale(t *testing.T) {
	svc := NewService(organization.NewService())
	user, err := svc.SetUserPreferredLocale("user_admin", "id-ID")
	if err != nil {
		t.Fatalf("expected preferred locale update to succeed: %v", err)
	}
	if user.PreferredLocale != "id" {
		t.Fatalf("expected normalized locale on user, got %+v", user)
	}
	if got := svc.PreferredLocale("user_admin"); got != "id" {
		t.Fatalf("expected preferred locale lookup to return id, got %q", got)
	}
}

func TestDefaultRoutePrefersUserOverrideThenHighestPriorityRole(t *testing.T) {
	svc := NewService(organization.NewService())
	if err := svc.UpsertRole(Role{ID: "role_ops", Key: "ops", Name: "Ops", ScopeType: "deployment", DefaultUserRoute: "/monitoring", DefaultAdminRoute: "/admin/observability"}); err != nil {
		t.Fatalf("upsert role failed: %v", err)
	}
	now := time.Now().UTC()
	repo := svc.repo
	if err := repo.SaveRoleBinding(RoleBinding{ID: "rb_ops", UserID: "user_admin", RoleID: "role_ops", ScopeType: "deployment", Priority: 10, EffectiveFrom: now, Status: "active"}); err != nil {
		t.Fatalf("save role binding failed: %v", err)
	}
	if got := svc.DefaultRoute("user_admin", "admin"); got != "/admin/modules" {
		t.Fatalf("expected bootstrap admin role default route first, got %q", got)
	}
	if _, err := svc.SetRoleBindingPriority("rb_ops", 200); err != nil {
		t.Fatalf("set role binding priority failed: %v", err)
	}
	if got := svc.DefaultRoute("user_admin", "user"); got != "/monitoring" {
		t.Fatalf("expected highest-priority role user route, got %q", got)
	}
	if got := svc.DefaultRoute("user_admin", "admin"); got != "/admin/observability" {
		t.Fatalf("expected highest-priority role admin route, got %q", got)
	}
	if _, err := svc.SetUserPreferredRoutes("user_admin", "/documents", "/admin/security"); err != nil {
		t.Fatalf("set user preferred routes failed: %v", err)
	}
	if got := svc.PreferredRoute("user_admin", "user"); got != "/documents" {
		t.Fatalf("expected preferred user route, got %q", got)
	}
	if got := svc.PreferredRoute("user_admin", "admin"); got != "/admin/security" {
		t.Fatalf("expected preferred admin route, got %q", got)
	}
}

func TestAuthenticatePasswordAndChangePassword(t *testing.T) {
	svc := NewService(organization.NewService())
	session, err := svc.AuthenticatePassword("admin", "admin123!", "loc_hq", map[string]any{"source": "test"}, time.Hour)
	if err != nil {
		t.Fatalf("expected password auth to succeed: %v", err)
	}
	if session.AuthenticationMethod != "password" {
		t.Fatalf("expected password session, got %+v", session)
	}
	if err := svc.ChangePassword("user_admin", "admin123!", "better-admin-123"); err != nil {
		t.Fatalf("expected password change to succeed: %v", err)
	}
	revokedSession, ok := svc.FindSession(session.ID)
	if !ok || revokedSession.Status != "revoked" {
		t.Fatalf("expected existing session to be revoked after password change, got %+v", revokedSession)
	}
	if _, err := svc.AuthenticatePassword("admin", "admin123!", "loc_hq", nil, time.Hour); err == nil {
		t.Fatal("expected old password to stop working")
	}
	if _, err := svc.AuthenticatePassword("admin", "better-admin-123", "loc_hq", nil, time.Hour); err != nil {
		t.Fatalf("expected new password to work: %v", err)
	}
}

func TestAuthenticateGoogleLinksExistingUserByEmail(t *testing.T) {
	svc := NewService(organization.NewService())
	user, err := svc.CreateUser("user@example.com", "example-pass-123", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	session, err := svc.AuthenticateGoogle(GoogleIdentity{
		Subject:       "sub-123",
		Email:         "user@example.com",
		EmailVerified: true,
		Name:          "Example User",
	}, "loc_hq", map[string]any{"source": "test"}, time.Hour, GoogleProvisioningPolicy{})
	if err != nil {
		t.Fatalf("google auth failed: %v", err)
	}
	if session.AuthenticationMethod != "google" {
		t.Fatalf("expected google session, got %+v", session)
	}
	linked, ok := svc.FindUser(user.ID)
	if !ok || linked.AuthenticationSubject != "google:sub-123" {
		t.Fatalf("expected user to be linked to google subject, got %+v", linked)
	}
}

func TestAuthenticateGoogleAutoProvisionsUser(t *testing.T) {
	svc := NewService(organization.NewService())
	session, err := svc.AuthenticateGoogle(GoogleIdentity{
		Subject:       "sub-456",
		Email:         "newuser@example.com",
		EmailVerified: true,
		Name:          "New User",
	}, "", map[string]any{"source": "test"}, time.Hour, GoogleProvisioningPolicy{
		Enabled:           true,
		AllowedDomains:    []string{"example.com"},
		RoleID:            "role_admin",
		ScopeType:         "deployment",
		DefaultLocationID: "loc_hq",
	})
	if err != nil {
		t.Fatalf("expected google auto provisioning to succeed: %v", err)
	}
	if session.AuthenticationMethod != "google" || session.CurrentLocationID != "loc_hq" {
		t.Fatalf("unexpected provisioned session: %+v", session)
	}
	user, ok := svc.FindUserByUsername("newuser@example.com")
	if !ok {
		t.Fatal("expected auto provisioned user")
	}
	if user.AuthenticationSubject != "google:sub-456" || user.DefaultLocationID != "loc_hq" {
		t.Fatalf("unexpected auto provisioned user: %+v", user)
	}
	foundBinding := false
	for _, binding := range svc.Bindings() {
		if binding.UserID == user.ID && binding.RoleID == "role_admin" {
			foundBinding = true
			break
		}
	}
	if !foundBinding {
		t.Fatal("expected role binding for auto provisioned user")
	}
}

func TestAuthenticateGoogleAutoProvisionRejectsDisallowedDomain(t *testing.T) {
	svc := NewService(organization.NewService())
	_, err := svc.AuthenticateGoogle(GoogleIdentity{
		Subject:       "sub-789",
		Email:         "newuser@blocked.example",
		EmailVerified: true,
		Name:          "Blocked User",
	}, "", nil, time.Hour, GoogleProvisioningPolicy{
		Enabled:        true,
		AllowedDomains: []string{"example.com"},
		RoleID:         "role_admin",
		ScopeType:      "deployment",
	})
	if err == nil {
		t.Fatal("expected disallowed domain to fail auto provisioning")
	}
}

func TestAuthenticatePasswordLocksAfterRepeatedFailures(t *testing.T) {
	svc := NewService(organization.NewService())
	for range 5 {
		if _, err := svc.AuthenticatePassword("admin", "wrong-password", "loc_hq", nil, time.Hour); err == nil {
			t.Fatal("expected invalid password to fail")
		}
	}
	credential, ok := svc.FindCredentialByUserID("user_admin")
	if !ok {
		t.Fatal("expected admin credential")
	}
	if credential.LockedUntil.IsZero() {
		t.Fatal("expected credential lockout after repeated failures")
	}
	if _, err := svc.AuthenticatePassword("admin", "admin123!", "loc_hq", nil, time.Hour); err == nil {
		t.Fatal("expected locked account to reject authentication")
	}
}

func TestCreateUserAndAdminResetPassword(t *testing.T) {
	svc := NewService(organization.NewService())
	user, err := svc.CreateUser("clerk", "clerk-pass-123", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("expected create user to succeed: %v", err)
	}
	if user.Username != "clerk" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if _, err := svc.AuthenticatePassword("clerk", "clerk-pass-123", "loc_hq", nil, time.Hour); err != nil {
		t.Fatalf("expected created user to authenticate: %v", err)
	}
	session, err := svc.AuthenticatePassword("clerk", "clerk-pass-123", "loc_hq", nil, time.Hour)
	if err != nil {
		t.Fatalf("expected created user to authenticate again: %v", err)
	}
	if err := svc.ResetPassword(user.ID, "clerk-pass-456"); err != nil {
		t.Fatalf("expected admin reset password to succeed: %v", err)
	}
	revokedSession, ok := svc.FindSession(session.ID)
	if !ok || revokedSession.Status != "revoked" {
		t.Fatalf("expected reset password to revoke sessions, got %+v", revokedSession)
	}
	if _, err := svc.AuthenticatePassword("clerk", "clerk-pass-123", "loc_hq", nil, time.Hour); err == nil {
		t.Fatal("expected old password to stop working after reset")
	}
	if _, err := svc.AuthenticatePassword("clerk", "clerk-pass-456", "loc_hq", nil, time.Hour); err != nil {
		t.Fatalf("expected reset password to work: %v", err)
	}
}

func TestDisableAndEnableUser(t *testing.T) {
	svc := NewService(organization.NewService())
	user, err := svc.CreateUser("clerk", "clerk-pass-123", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("expected create user to succeed: %v", err)
	}
	session, err := svc.AuthenticatePassword("clerk", "clerk-pass-123", "loc_hq", nil, time.Hour)
	if err != nil {
		t.Fatalf("expected authentication to succeed: %v", err)
	}
	updated, err := svc.SetUserStatus(user.ID, "disabled")
	if err != nil {
		t.Fatalf("expected disable user to succeed: %v", err)
	}
	if updated.Status != "disabled" {
		t.Fatalf("expected disabled status, got %+v", updated)
	}
	revokedSession, ok := svc.FindSession(session.ID)
	if !ok || revokedSession.Status != "revoked" {
		t.Fatalf("expected user disable to revoke sessions, got %+v", revokedSession)
	}
	if _, err := svc.AuthenticatePassword("clerk", "clerk-pass-123", "loc_hq", nil, time.Hour); err == nil {
		t.Fatal("expected disabled user to fail authentication")
	}
	updated, err = svc.SetUserStatus(user.ID, "active")
	if err != nil {
		t.Fatalf("expected enable user to succeed: %v", err)
	}
	if updated.Status != "active" {
		t.Fatalf("expected active status, got %+v", updated)
	}
	if _, err := svc.AuthenticatePassword("clerk", "clerk-pass-123", "loc_hq", nil, time.Hour); err != nil {
		t.Fatalf("expected enabled user to authenticate: %v", err)
	}
}

func TestServiceAccessorsAndLoginFailureHelpers(t *testing.T) {
	now := time.Now().UTC()
	repo := NewMemoryRepository(
		[]User{{ID: "u1", Username: "user1", Status: "active"}},
		[]Role{{ID: "r1", Name: "Role 1"}},
		[]Permission{{Key: "document.read"}},
		[]RoleBinding{{ID: "rb1", UserID: "u1", RoleID: "r1", ScopeType: "deployment", EffectiveFrom: now, Status: "active"}},
		[]RolePermission{{RoleID: "r1", PermissionKey: "document.read"}},
		[]Credential{{UserID: "u1", PasswordHash: "hash"}},
		[]Session{{ID: "s1", UserID: "u1", Status: "active", IssuedAt: now, ExpiresAt: now.Add(time.Hour), LastSeenAt: now}},
		[]ServicePrincipal{{ID: "sp1", Status: "active", AllowedOperationTypes: []string{"projection.refresh"}}},
	)
	svc := NewServiceWithRepository(organization.NewService(), repo)

	if len(svc.Roles()) != 1 || len(svc.Users()) != 1 || len(svc.Bindings()) != 1 {
		t.Fatal("expected accessor lists to return data")
	}
	if len(svc.Sessions()) != 1 || len(svc.Credentials()) != 1 || len(svc.ServicePrincipals()) != 1 {
		t.Fatal("expected session, credential, and principal accessors to return data")
	}
	if _, ok := svc.FindUser("u1"); !ok {
		t.Fatal("expected FindUser to succeed")
	}
	if _, ok := svc.FindUserByUsername("user1"); !ok {
		t.Fatal("expected FindUserByUsername to succeed")
	}
	if _, ok := svc.FindServicePrincipal("sp1"); !ok {
		t.Fatal("expected FindServicePrincipal to succeed")
	}

	if err := svc.RecordLoginFailure("user1|127.0.0.1", now); err != nil {
		t.Fatalf("RecordLoginFailure failed: %v", err)
	}
	if count := svc.CountRecentLoginFailures("user1|127.0.0.1", now.Add(-time.Minute)); count != 1 {
		t.Fatalf("unexpected login failure count: %d", count)
	}
	if err := svc.CleanupLoginFailures(now.Add(-time.Second)); err != nil {
		t.Fatalf("CleanupLoginFailures failed: %v", err)
	}
	if err := svc.ClearLoginFailures("user1|127.0.0.1"); err != nil {
		t.Fatalf("ClearLoginFailures failed: %v", err)
	}
	if count := svc.CountRecentLoginFailures("user1|127.0.0.1", now.Add(-time.Minute)); count != 0 {
		t.Fatalf("expected cleared login failures, got %d", count)
	}
}

func TestEnsureBootstrapAdminCredentialRefreshRotateAndReviewSession(t *testing.T) {
	svc := NewService(organization.NewService())

	if err := svc.EnsureBootstrapAdminCredential("bootstrap-123!"); err != nil {
		t.Fatalf("EnsureBootstrapAdminCredential failed: %v", err)
	}
	if _, err := svc.AuthenticatePassword("admin", "bootstrap-123!", "loc_hq", map[string]any{
		"remote_addr": "192.0.2.1",
		"user_agent":  "ua-1",
	}, time.Hour); err != nil {
		t.Fatalf("expected bootstrap credential to authenticate: %v", err)
	}

	original, err := svc.StartSession("admin", "loc_hq", "password", map[string]any{
		"remote_addr": "192.0.2.1",
		"user_agent":  "ua-1",
	}, time.Hour)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	_, err = svc.StartSession("admin", "loc_hq", "password", map[string]any{
		"remote_addr": "198.51.100.8",
		"user_agent":  "ua-2",
	}, time.Hour)
	if err != nil {
		t.Fatalf("second StartSession failed: %v", err)
	}

	refreshed, err := svc.RefreshSession(original.ID, 2*time.Hour)
	if err != nil {
		t.Fatalf("RefreshSession failed: %v", err)
	}
	if refreshed.ID == original.ID || refreshed.Status != "active" {
		t.Fatalf("expected rotated active session, got %+v", refreshed)
	}
	revoked, ok := svc.FindSession(original.ID)
	if !ok || revoked.Status != "revoked" {
		t.Fatalf("expected original session revoked after refresh, got %+v", revoked)
	}

	review, ok := svc.ReviewSession(refreshed.ID)
	if !ok {
		t.Fatal("expected ReviewSession to succeed")
	}
	if review.ConcurrentActiveSessions == 0 {
		t.Fatalf("expected concurrent session review, got %+v", review)
	}
	if len(review.Flags) == 0 {
		t.Fatalf("expected anomaly flags, got %+v", review)
	}
}

func TestNewTokenManagerFromEnv(t *testing.T) {
	oldSecret := os.Getenv("APP_JWT_SECRET")
	oldIssuer := os.Getenv("APP_JWT_ISSUER")
	defer func() {
		_ = os.Setenv("APP_JWT_SECRET", oldSecret)
		_ = os.Setenv("APP_JWT_ISSUER", oldIssuer)
	}()

	_ = os.Setenv("APP_JWT_SECRET", "secret-1")
	_ = os.Setenv("APP_JWT_ISSUER", "issuer-1")

	manager := NewTokenManagerFromEnv()
	if string(manager.secret) != "secret-1" || manager.issuer != "issuer-1" {
		t.Fatalf("unexpected token manager env config: %+v", manager)
	}
}

func TestAppendIfMissing(t *testing.T) {
	flags := appendIfMissing([]string{"a"}, "b")
	flags = appendIfMissing(flags, "b")
	if len(flags) != 2 {
		t.Fatalf("expected unique append behavior, got %+v", flags)
	}
}
