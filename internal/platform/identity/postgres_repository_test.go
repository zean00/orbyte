package identity

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresIdentityRepository(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	userRows := sqlmock.NewRows([]string{"user_id", "username", "authentication_subject", "status", "default_location_id", "preferred_locale", "preferred_user_route", "preferred_admin_route", "created_at", "updated_at"}).AddRow("u1", "admin", "", "active", "loc1", "id", "/documents", "/admin/modules", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), COALESCE(preferred_user_route, ''), COALESCE(preferred_admin_route, ''), created_at, updated_at")).WillReturnRows(userRows)
	users := repo.Users()
	if len(users) != 1 {
		t.Fatal("expected users")
	}
	if users[0].PreferredLocale != "id" {
		t.Fatalf("expected preferred locale to load, got %+v", users[0])
	}
	if users[0].PreferredUserRoute != "/documents" || users[0].PreferredAdminRoute != "/admin/modules" {
		t.Fatalf("expected preferred routes to load, got %+v", users[0])
	}
	roleRows := sqlmock.NewRows([]string{"role_id", "role_key", "name", "scope_type", "default_user_route", "default_admin_route", "created_at", "updated_at"}).AddRow("r1", "admin", "Admin", "deployment", "/documents", "/admin/modules", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT role_id, role_key, name, scope_type, COALESCE(default_user_route, ''), COALESCE(default_admin_route, ''), created_at, updated_at")).WillReturnRows(roleRows)
	if len(repo.Roles()) != 1 {
		t.Fatal("expected roles")
	}
	permRows := sqlmock.NewRows([]string{"permission_key", "module_key", "action_kind", "resource_kind", "description"}).AddRow("p1", "document", "create", "document", "x")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT permission_key, module_key, action_kind, resource_kind, COALESCE(description, '')")).WillReturnRows(permRows)
	if len(repo.Permissions()) != 1 {
		t.Fatal("expected perms")
	}
	bindingRows := sqlmock.NewRows([]string{"role_binding_id", "user_id", "role_id", "scope_type", "scope_id", "priority", "effective_from", "effective_to", "status"}).AddRow("rb1", "u1", "r1", "deployment", "", 5, now, nil, "active")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT role_binding_id, user_id, role_id, scope_type, COALESCE(scope_id, ''), COALESCE(priority, 0), effective_from, effective_to, status")).WillReturnRows(bindingRows)
	if len(repo.RoleBindings()) != 1 {
		t.Fatal("expected bindings")
	}
	grantRows := sqlmock.NewRows([]string{"role_id", "permission_key"}).AddRow("r1", "p1")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT role_id, permission_key")).WillReturnRows(grantRows)
	if len(repo.RolePermissions()) != 1 {
		t.Fatal("expected role permissions")
	}
	credentialRows := sqlmock.NewRows([]string{"user_id", "password_hash", "password_changed_at", "failed_attempt_count", "locked_until", "updated_at"}).
		AddRow("u1", "$argon2id$example", now, 0, nil, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, password_hash, password_changed_at, failed_attempt_count, locked_until, updated_at")).WillReturnRows(credentialRows)
	if len(repo.Credentials()) != 1 {
		t.Fatal("expected credentials")
	}
	sessionRows := sqlmock.NewRows([]string{"session_id", "user_id", "status", "issued_at", "expires_at", "last_seen_at", "authentication_method", "current_location_scope", "revoked_at", "client_metadata_json"}).
		AddRow("s1", "u1", "active", now, now.Add(time.Hour), now, "password", "loc1", nil, []byte(`{"ip":"127.0.0.1"}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT session_id, user_id, status, issued_at, expires_at, last_seen_at,")).WillReturnRows(sessionRows)
	if len(repo.Sessions()) != 1 {
		t.Fatal("expected sessions")
	}
	principalRows := sqlmock.NewRows([]string{"service_principal_id", "principal_key", "status", "allowed_operation_types_json", "credential_ref", "created_at", "updated_at"}).
		AddRow("sp1", "worker", "active", []byte(`["projection.refresh"]`), "vault://worker", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT service_principal_id, principal_key, status,")).WillReturnRows(principalRows)
	if len(repo.ServicePrincipals()) != 1 {
		t.Fatal("expected service principals")
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO users (")).
		WithArgs("u2", "clerk", "", "active", "loc1", "id", "/documents", "/admin/modules", now, now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveUser(User{ID: "u2", Username: "clerk", Status: "active", DefaultLocationID: "loc1", PreferredLocale: "id", PreferredUserRoute: "/documents", PreferredAdminRoute: "/admin/modules", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("expected save user: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO role_bindings (")).
		WithArgs("rb2", "u2", "r1", "deployment", "", 3, now, nil, "active").
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveRoleBinding(RoleBinding{ID: "rb2", UserID: "u2", RoleID: "r1", ScopeType: "deployment", Priority: 3, EffectiveFrom: now, Status: "active"}); err != nil {
		t.Fatalf("expected save role binding: %v", err)
	}
	findRows := sqlmock.NewRows([]string{"user_id", "username", "authentication_subject", "status", "default_location_id", "preferred_locale", "preferred_user_route", "preferred_admin_route", "created_at", "updated_at"}).AddRow("u1", "admin", "", "active", "loc1", "id", "/documents", "/admin/modules", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), COALESCE(preferred_user_route, ''), COALESCE(preferred_admin_route, ''), created_at, updated_at")).WithArgs("u1").WillReturnRows(findRows)
	if _, ok := repo.FindUser("u1"); !ok {
		t.Fatal("expected find user")
	}
	findUserByNameRows := sqlmock.NewRows([]string{"user_id", "username", "authentication_subject", "status", "default_location_id", "preferred_locale", "preferred_user_route", "preferred_admin_route", "created_at", "updated_at"}).AddRow("u1", "admin", "", "active", "loc1", "id", "/documents", "/admin/modules", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), COALESCE(preferred_user_route, ''), COALESCE(preferred_admin_route, ''), created_at, updated_at")).WithArgs("admin").WillReturnRows(findUserByNameRows)
	if _, ok := repo.FindUserByUsername("admin"); !ok {
		t.Fatal("expected find user by username")
	}
	findUserBySubjectRows := sqlmock.NewRows([]string{"user_id", "username", "authentication_subject", "status", "default_location_id", "preferred_locale", "preferred_user_route", "preferred_admin_route", "created_at", "updated_at"}).AddRow("u1", "admin", "google:sub-1", "active", "loc1", "id", "/documents", "/admin/modules", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), COALESCE(preferred_user_route, ''), COALESCE(preferred_admin_route, ''), created_at, updated_at")).WithArgs("google:sub-1").WillReturnRows(findUserBySubjectRows)
	if _, ok := repo.FindUserByAuthenticationSubject("google:sub-1"); !ok {
		t.Fatal("expected find user by authentication subject")
	}
	findCredentialRows := sqlmock.NewRows([]string{"user_id", "password_hash", "password_changed_at", "failed_attempt_count", "locked_until", "updated_at"}).
		AddRow("u1", "$argon2id$example", now, 0, nil, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id, password_hash, password_changed_at, failed_attempt_count, locked_until, updated_at")).WithArgs("u1").WillReturnRows(findCredentialRows)
	if _, ok := repo.FindCredentialByUserID("u1"); !ok {
		t.Fatal("expected find credential by user id")
	}
	findSessionRows := sqlmock.NewRows([]string{"session_id", "user_id", "status", "issued_at", "expires_at", "last_seen_at", "authentication_method", "current_location_scope", "revoked_at", "client_metadata_json"}).
		AddRow("s1", "u1", "active", now, now.Add(time.Hour), now, "password", "loc1", nil, []byte(`{"ip":"127.0.0.1"}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT session_id, user_id, status, issued_at, expires_at, last_seen_at,")).WithArgs("s1").WillReturnRows(findSessionRows)
	if _, ok := repo.FindSession("s1"); !ok {
		t.Fatal("expected find session")
	}
	findPrincipalRows := sqlmock.NewRows([]string{"service_principal_id", "principal_key", "status", "allowed_operation_types_json", "credential_ref", "created_at", "updated_at"}).
		AddRow("sp1", "worker", "active", []byte(`["projection.refresh"]`), "vault://worker", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT service_principal_id, principal_key, status,")).WithArgs("sp1").WillReturnRows(findPrincipalRows)
	if _, ok := repo.FindServicePrincipal("sp1"); !ok {
		t.Fatal("expected find service principal")
	}
	countRows := sqlmock.NewRows([]string{"count"}).AddRow(2)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).WithArgs("admin|192.0.2.10", now.Add(-time.Minute)).WillReturnRows(countRows)
	if count := repo.CountRecentLoginFailures("admin|192.0.2.10", now.Add(-time.Minute)); count != 2 {
		t.Fatalf("expected login failure count, got %d", count)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO auth_login_failures")).WithArgs("login-failure:"+now.Format("20060102150405.000000000"), "admin|192.0.2.10", now).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.RecordLoginFailure("admin|192.0.2.10", now); err != nil {
		t.Fatalf("expected record login failure: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM auth_login_failures")).WithArgs("admin|192.0.2.10").WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.ClearLoginFailures("admin|192.0.2.10"); err != nil {
		t.Fatalf("expected clear login failures: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM auth_login_failures")).WithArgs(now.Add(-time.Minute)).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.CleanupLoginFailures(now.Add(-time.Minute)); err != nil {
		t.Fatalf("expected cleanup login failures: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO user_credentials (")).
		WithArgs("u1", "$argon2id$example", now, 0, nil, now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveCredential(Credential{
		UserID:            "u1",
		PasswordHash:      "$argon2id$example",
		PasswordChangedAt: now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("expected save credential: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO sessions (")).
		WithArgs("s1", "u1", "active", now, now.Add(time.Hour), now, "password", `{"ip":"127.0.0.1"}`, "loc1", nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveSession(Session{
		ID:                   "s1",
		UserID:               "u1",
		Status:               "active",
		IssuedAt:             now,
		ExpiresAt:            now.Add(time.Hour),
		LastSeenAt:           now,
		AuthenticationMethod: "password",
		CurrentLocationID:    "loc1",
		ClientMetadata:       map[string]any{"ip": "127.0.0.1"},
	}); err != nil {
		t.Fatalf("expected save session: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
