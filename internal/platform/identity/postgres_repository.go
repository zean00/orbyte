package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Users() []User {
	const query = `
		SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), created_at, updated_at
		FROM users
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]User, 0)
	for rows.Next() {
		var item User
		if err := rows.Scan(&item.ID, &item.Username, &item.AuthenticationSubject, &item.Status, &item.DefaultLocationID, &item.PreferredLocale, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) Roles() []Role {
	const query = `
		SELECT role_id, role_key, name, scope_type, created_at, updated_at
		FROM roles
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]Role, 0)
	for rows.Next() {
		var item Role
		if err := rows.Scan(&item.ID, &item.Key, &item.Name, &item.ScopeType, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) Permissions() []Permission {
	const query = `
		SELECT permission_key, module_key, action_kind, resource_kind, COALESCE(description, '')
		FROM permissions
		ORDER BY permission_key ASC`

	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]Permission, 0)
	for rows.Next() {
		var item Permission
		if err := rows.Scan(&item.Key, &item.Module, &item.Action, &item.Resource, &item.Description); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) RoleBindings() []RoleBinding {
	const query = `
		SELECT role_binding_id, user_id, role_id, scope_type, COALESCE(scope_id, ''), effective_from, effective_to, status
		FROM role_bindings
		ORDER BY effective_from ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]RoleBinding, 0)
	for rows.Next() {
		var item RoleBinding
		var effectiveTo sql.NullTime
		if err := rows.Scan(&item.ID, &item.UserID, &item.RoleID, &item.ScopeType, &item.ScopeID, &item.EffectiveFrom, &effectiveTo, &item.Status); err != nil {
			continue
		}
		if effectiveTo.Valid {
			item.EffectiveTo = effectiveTo.Time
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) RolePermissions() []RolePermission {
	const query = `
		SELECT role_id, permission_key
		FROM role_permissions
		ORDER BY role_id, permission_key`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]RolePermission, 0)
	for rows.Next() {
		var item RolePermission
		if err := rows.Scan(&item.RoleID, &item.PermissionKey); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) Credentials() []Credential {
	const query = `
		SELECT user_id, password_hash, password_changed_at, failed_attempt_count, locked_until, updated_at
		FROM user_credentials
		ORDER BY updated_at ASC`

	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]Credential, 0)
	for rows.Next() {
		var (
			item        Credential
			lockedUntil sql.NullTime
		)
		if err := rows.Scan(&item.UserID, &item.PasswordHash, &item.PasswordChangedAt, &item.FailedAttemptCount, &lockedUntil, &item.UpdatedAt); err != nil {
			continue
		}
		if lockedUntil.Valid {
			item.LockedUntil = lockedUntil.Time
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) FindUser(id string) (User, bool) {
	const query = `
		SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), created_at, updated_at
		FROM users
		WHERE user_id = $1`

	var item User
	err := r.db.QueryRowContext(context.Background(), query, id).Scan(
		&item.ID,
		&item.Username,
		&item.AuthenticationSubject,
		&item.Status,
		&item.DefaultLocationID,
		&item.PreferredLocale,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return User{}, false
	}
	return item, true
}

func (r *PostgresRepository) FindUserByUsername(username string) (User, bool) {
	const query = `
		SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), created_at, updated_at
		FROM users
		WHERE username = $1`

	var item User
	err := r.db.QueryRowContext(context.Background(), query, username).Scan(
		&item.ID,
		&item.Username,
		&item.AuthenticationSubject,
		&item.Status,
		&item.DefaultLocationID,
		&item.PreferredLocale,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return User{}, false
	}
	return item, true
}

func (r *PostgresRepository) FindUserByAuthenticationSubject(subject string) (User, bool) {
	const query = `
		SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), created_at, updated_at
		FROM users
		WHERE authentication_subject = $1`

	var item User
	err := r.db.QueryRowContext(context.Background(), query, subject).Scan(
		&item.ID,
		&item.Username,
		&item.AuthenticationSubject,
		&item.Status,
		&item.DefaultLocationID,
		&item.PreferredLocale,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return User{}, false
	}
	return item, true
}

func (r *PostgresRepository) FindCredentialByUserID(userID string) (Credential, bool) {
	const query = `
		SELECT user_id, password_hash, password_changed_at, failed_attempt_count, locked_until, updated_at
		FROM user_credentials
		WHERE user_id = $1`

	var (
		item        Credential
		lockedUntil sql.NullTime
	)
	err := r.db.QueryRowContext(context.Background(), query, userID).Scan(
		&item.UserID,
		&item.PasswordHash,
		&item.PasswordChangedAt,
		&item.FailedAttemptCount,
		&lockedUntil,
		&item.UpdatedAt,
	)
	if err != nil {
		return Credential{}, false
	}
	if lockedUntil.Valid {
		item.LockedUntil = lockedUntil.Time
	}
	return item, true
}

func (r *PostgresRepository) Sessions() []Session {
	const query = `
		SELECT session_id, user_id, status, issued_at, expires_at, last_seen_at,
		       COALESCE(authentication_method, ''), COALESCE(current_location_scope, ''), revoked_at,
		       COALESCE(client_metadata_json, '{}'::jsonb)
		FROM sessions
		ORDER BY issued_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]Session, 0)
	for rows.Next() {
		var (
			item         Session
			revokedAt    sql.NullTime
			metadataJSON []byte
		)
		if err := rows.Scan(&item.ID, &item.UserID, &item.Status, &item.IssuedAt, &item.ExpiresAt, &item.LastSeenAt, &item.AuthenticationMethod, &item.CurrentLocationID, &revokedAt, &metadataJSON); err != nil {
			continue
		}
		if revokedAt.Valid {
			item.RevokedAt = revokedAt.Time
		}
		_ = json.Unmarshal(metadataJSON, &item.ClientMetadata)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) ServicePrincipals() []ServicePrincipal {
	const query = `
		SELECT service_principal_id, principal_key, status,
		       COALESCE(allowed_operation_types_json, '[]'::jsonb), COALESCE(credential_ref, ''),
		       created_at, updated_at
		FROM service_principals
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]ServicePrincipal, 0)
	for rows.Next() {
		var (
			item       ServicePrincipal
			allowedOps []byte
		)
		if err := rows.Scan(&item.ID, &item.Key, &item.Status, &allowedOps, &item.CredentialRef, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(allowedOps, &item.AllowedOperationTypes)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveUser(user User) error {
	const query = `
		INSERT INTO users (
			user_id, username, authentication_subject, status, default_location_id, preferred_locale, created_at, updated_at
		) VALUES ($1, $2, NULLIF($3, ''), $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			username = EXCLUDED.username,
			authentication_subject = EXCLUDED.authentication_subject,
			status = EXCLUDED.status,
			default_location_id = EXCLUDED.default_location_id,
			preferred_locale = EXCLUDED.preferred_locale,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(
		context.Background(),
		query,
		user.ID,
		user.Username,
		user.AuthenticationSubject,
		user.Status,
		user.DefaultLocationID,
		user.PreferredLocale,
		user.CreatedAt,
		user.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) SaveRole(role Role) error {
	const query = `
		INSERT INTO roles (
			role_id, role_key, name, scope_type, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (role_id) DO UPDATE SET
			role_key = EXCLUDED.role_key,
			name = EXCLUDED.name,
			scope_type = EXCLUDED.scope_type,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(
		context.Background(),
		query,
		role.ID,
		role.Key,
		role.Name,
		role.ScopeType,
		role.CreatedAt,
		role.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) SavePermission(permission Permission) error {
	const query = `
		INSERT INTO permissions (
			permission_key, module_key, action_kind, resource_kind, description
		) VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		ON CONFLICT (permission_key) DO UPDATE SET
			module_key = EXCLUDED.module_key,
			action_kind = EXCLUDED.action_kind,
			resource_kind = EXCLUDED.resource_kind,
			description = EXCLUDED.description`
	_, err := r.db.ExecContext(
		context.Background(),
		query,
		permission.Key,
		permission.Module,
		permission.Action,
		permission.Resource,
		permission.Description,
	)
	return err
}

func (r *PostgresRepository) SaveRoleBinding(binding RoleBinding) error {
	const query = `
		INSERT INTO role_bindings (
			role_binding_id, user_id, role_id, scope_type, scope_id, effective_from, effective_to, status
		) VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8)
		ON CONFLICT (role_binding_id) DO UPDATE SET
			role_id = EXCLUDED.role_id,
			scope_type = EXCLUDED.scope_type,
			scope_id = EXCLUDED.scope_id,
			effective_from = EXCLUDED.effective_from,
			effective_to = EXCLUDED.effective_to,
			status = EXCLUDED.status`
	var effectiveTo any
	if !binding.EffectiveTo.IsZero() {
		effectiveTo = binding.EffectiveTo
	}
	_, err := r.db.ExecContext(
		context.Background(),
		query,
		binding.ID,
		binding.UserID,
		binding.RoleID,
		binding.ScopeType,
		binding.ScopeID,
		binding.EffectiveFrom,
		effectiveTo,
		binding.Status,
	)
	return err
}

func (r *PostgresRepository) SaveRolePermission(grant RolePermission) error {
	const query = `
		INSERT INTO role_permissions (role_id, permission_key)
		VALUES ($1, $2)
		ON CONFLICT (role_id, permission_key) DO NOTHING`
	_, err := r.db.ExecContext(context.Background(), query, grant.RoleID, grant.PermissionKey)
	return err
}

func (r *PostgresRepository) FindSession(id string) (Session, bool) {
	const query = `
		SELECT session_id, user_id, status, issued_at, expires_at, last_seen_at,
		       COALESCE(authentication_method, ''), COALESCE(current_location_scope, ''), revoked_at,
		       COALESCE(client_metadata_json, '{}'::jsonb)
		FROM sessions
		WHERE session_id = $1`
	var (
		item         Session
		revokedAt    sql.NullTime
		metadataJSON []byte
	)
	err := r.db.QueryRowContext(context.Background(), query, id).Scan(
		&item.ID,
		&item.UserID,
		&item.Status,
		&item.IssuedAt,
		&item.ExpiresAt,
		&item.LastSeenAt,
		&item.AuthenticationMethod,
		&item.CurrentLocationID,
		&revokedAt,
		&metadataJSON,
	)
	if err != nil {
		return Session{}, false
	}
	if revokedAt.Valid {
		item.RevokedAt = revokedAt.Time
	}
	_ = json.Unmarshal(metadataJSON, &item.ClientMetadata)
	return item, true
}

func (r *PostgresRepository) FindServicePrincipal(id string) (ServicePrincipal, bool) {
	const query = `
		SELECT service_principal_id, principal_key, status,
		       COALESCE(allowed_operation_types_json, '[]'::jsonb), COALESCE(credential_ref, ''),
		       created_at, updated_at
		FROM service_principals
		WHERE service_principal_id = $1`
	var (
		item       ServicePrincipal
		allowedOps []byte
	)
	err := r.db.QueryRowContext(context.Background(), query, id).Scan(
		&item.ID,
		&item.Key,
		&item.Status,
		&allowedOps,
		&item.CredentialRef,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return ServicePrincipal{}, false
	}
	_ = json.Unmarshal(allowedOps, &item.AllowedOperationTypes)
	return item, true
}

func (r *PostgresRepository) SaveServicePrincipal(principal ServicePrincipal) error {
	const query = `
		INSERT INTO service_principals (
			service_principal_id, principal_key, status, allowed_operation_types_json, credential_ref, created_at, updated_at
		) VALUES ($1, $2, $3, $4::jsonb, NULLIF($5, ''), $6, $7)
		ON CONFLICT (service_principal_id) DO UPDATE SET
			principal_key = EXCLUDED.principal_key,
			status = EXCLUDED.status,
			allowed_operation_types_json = EXCLUDED.allowed_operation_types_json,
			credential_ref = EXCLUDED.credential_ref,
			updated_at = EXCLUDED.updated_at`
	allowedOpsJSON, err := json.Marshal(principal.AllowedOperationTypes)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(
		context.Background(),
		query,
		principal.ID,
		principal.Key,
		principal.Status,
		string(allowedOpsJSON),
		principal.CredentialRef,
		principal.CreatedAt,
		principal.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) CountRecentLoginFailures(key string, since time.Time) int {
	const query = `
		SELECT COUNT(*)
		FROM auth_login_failures
		WHERE throttle_key = $1 AND attempted_at >= $2`
	var count int
	if err := r.db.QueryRowContext(context.Background(), query, key, since).Scan(&count); err != nil {
		return 0
	}
	return count
}

func (r *PostgresRepository) RecordLoginFailure(key string, attemptedAt time.Time) error {
	const query = `
		INSERT INTO auth_login_failures (failure_id, throttle_key, attempted_at)
		VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(context.Background(), query, "login-failure:"+attemptedAt.Format("20060102150405.000000000"), key, attemptedAt)
	return err
}

func (r *PostgresRepository) ClearLoginFailures(key string) error {
	const query = `
		DELETE FROM auth_login_failures
		WHERE throttle_key = $1`
	_, err := r.db.ExecContext(context.Background(), query, key)
	return err
}

func (r *PostgresRepository) CleanupLoginFailures(before time.Time) error {
	const query = `
		DELETE FROM auth_login_failures
		WHERE attempted_at < $1`
	_, err := r.db.ExecContext(context.Background(), query, before)
	return err
}

func (r *PostgresRepository) SaveSession(session Session) error {
	const query = `
		INSERT INTO sessions (
			session_id, user_id, status, issued_at, expires_at, last_seen_at,
			authentication_method, client_metadata_json, current_location_scope, revoked_at
		) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8::jsonb, NULLIF($9, ''), $10)
		ON CONFLICT (session_id) DO UPDATE SET
			status = EXCLUDED.status,
			expires_at = EXCLUDED.expires_at,
			last_seen_at = EXCLUDED.last_seen_at,
			authentication_method = EXCLUDED.authentication_method,
			client_metadata_json = EXCLUDED.client_metadata_json,
			current_location_scope = EXCLUDED.current_location_scope,
			revoked_at = EXCLUDED.revoked_at`
	metadataJSON, err := json.Marshal(session.ClientMetadata)
	if err != nil {
		return err
	}
	var revokedAt any
	if !session.RevokedAt.IsZero() {
		revokedAt = session.RevokedAt
	}
	_, err = r.db.ExecContext(
		context.Background(),
		query,
		session.ID,
		session.UserID,
		session.Status,
		session.IssuedAt,
		session.ExpiresAt,
		session.LastSeenAt,
		session.AuthenticationMethod,
		string(metadataJSON),
		session.CurrentLocationID,
		revokedAt,
	)
	return err
}

func (r *PostgresRepository) SaveCredential(credential Credential) error {
	const query = `
		INSERT INTO user_credentials (
			user_id, password_hash, password_changed_at, failed_attempt_count, locked_until, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			password_changed_at = EXCLUDED.password_changed_at,
			failed_attempt_count = EXCLUDED.failed_attempt_count,
			locked_until = EXCLUDED.locked_until,
			updated_at = EXCLUDED.updated_at`
	var lockedUntil any
	if !credential.LockedUntil.IsZero() {
		lockedUntil = credential.LockedUntil
	}
	_, err := r.db.ExecContext(
		context.Background(),
		query,
		credential.UserID,
		credential.PasswordHash,
		credential.PasswordChangedAt,
		credential.FailedAttemptCount,
		lockedUntil,
		credential.UpdatedAt,
	)
	return err
}
