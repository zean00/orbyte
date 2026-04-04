package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"orbyte/internal/platform/store"
)

type PostgresRepository struct {
	db store.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return NewPostgresRepositoryWithDB(store.UninstrumentedDB(db))
}

func NewPostgresRepositoryWithDB(db store.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Users() []User {
	const query = `
		SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), COALESCE(preferred_user_route, ''), COALESCE(preferred_admin_route, ''), created_at, updated_at
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
		if err := rows.Scan(&item.ID, &item.Username, &item.AuthenticationSubject, &item.Status, &item.DefaultLocationID, &item.PreferredLocale, &item.PreferredUserRoute, &item.PreferredAdminRoute, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) Roles() []Role {
	const query = `
		SELECT role_id, role_key, name, scope_type, COALESCE(default_user_route, ''), COALESCE(default_admin_route, ''), created_at, updated_at
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
		if err := rows.Scan(&item.ID, &item.Key, &item.Name, &item.ScopeType, &item.DefaultUserRoute, &item.DefaultAdminRoute, &item.CreatedAt, &item.UpdatedAt); err != nil {
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
		SELECT role_binding_id, user_id, role_id, scope_type, COALESCE(scope_id, ''), COALESCE(priority, 0), effective_from, effective_to, status
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
		if err := rows.Scan(&item.ID, &item.UserID, &item.RoleID, &item.ScopeType, &item.ScopeID, &item.Priority, &item.EffectiveFrom, &effectiveTo, &item.Status); err != nil {
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

func (r *PostgresRepository) DeleteRolePermission(roleID, permissionKey string) error {
	_, err := r.db.ExecContext(context.Background(), `DELETE FROM role_permissions WHERE role_id = $1 AND permission_key = $2`, roleID, permissionKey)
	return err
}

func (r *PostgresRepository) Credentials() []Credential {
	const query = `
		SELECT user_id, password_hash, password_changed_at, COALESCE(cashier_pin_hash, ''), cashier_pin_changed_at, failed_attempt_count, locked_until, updated_at
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
			item                Credential
			cashierPINChangedAt sql.NullTime
			lockedUntil         sql.NullTime
		)
		if err := rows.Scan(&item.UserID, &item.PasswordHash, &item.PasswordChangedAt, &item.CashierPINHash, &cashierPINChangedAt, &item.FailedAttemptCount, &lockedUntil, &item.UpdatedAt); err != nil {
			continue
		}
		if cashierPINChangedAt.Valid {
			item.CashierPINChangedAt = cashierPINChangedAt.Time
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
		SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), COALESCE(preferred_user_route, ''), COALESCE(preferred_admin_route, ''), created_at, updated_at
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
		&item.PreferredUserRoute,
		&item.PreferredAdminRoute,
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
		SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), COALESCE(preferred_user_route, ''), COALESCE(preferred_admin_route, ''), created_at, updated_at
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
		&item.PreferredUserRoute,
		&item.PreferredAdminRoute,
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
		SELECT user_id, username, COALESCE(authentication_subject, ''), status, COALESCE(default_location_id, ''), COALESCE(preferred_locale, ''), COALESCE(preferred_user_route, ''), COALESCE(preferred_admin_route, ''), created_at, updated_at
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
		&item.PreferredUserRoute,
		&item.PreferredAdminRoute,
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
		SELECT user_id, password_hash, password_changed_at, COALESCE(cashier_pin_hash, ''), cashier_pin_changed_at, failed_attempt_count, locked_until, updated_at
		FROM user_credentials
		WHERE user_id = $1`

	var (
		item                Credential
		cashierPINChangedAt sql.NullTime
		lockedUntil         sql.NullTime
	)
	err := r.db.QueryRowContext(context.Background(), query, userID).Scan(
		&item.UserID,
		&item.PasswordHash,
		&item.PasswordChangedAt,
		&item.CashierPINHash,
		&cashierPINChangedAt,
		&item.FailedAttemptCount,
		&lockedUntil,
		&item.UpdatedAt,
	)
	if err != nil {
		return Credential{}, false
	}
	if cashierPINChangedAt.Valid {
		item.CashierPINChangedAt = cashierPINChangedAt.Time
	}
	if lockedUntil.Valid {
		item.LockedUntil = lockedUntil.Time
	}
	return item, true
}

func (r *PostgresRepository) Sessions() []Session {
	const query = `
		SELECT session_id, user_id, status, issued_at, expires_at, last_seen_at,
		       COALESCE(authentication_method, ''), COALESCE(current_location_scope, ''),
		       login_step_up_at, approval_step_up_at, approval_step_up_until, revoked_at,
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
			item                Session
			loginStepUpAt       sql.NullTime
			approvalStepUpAt    sql.NullTime
			approvalStepUpUntil sql.NullTime
			revokedAt           sql.NullTime
			metadataJSON        []byte
		)
		if err := rows.Scan(&item.ID, &item.UserID, &item.Status, &item.IssuedAt, &item.ExpiresAt, &item.LastSeenAt, &item.AuthenticationMethod, &item.CurrentLocationID, &loginStepUpAt, &approvalStepUpAt, &approvalStepUpUntil, &revokedAt, &metadataJSON); err != nil {
			continue
		}
		if loginStepUpAt.Valid {
			item.LoginStepUpAt = loginStepUpAt.Time
		}
		if approvalStepUpAt.Valid {
			item.ApprovalStepUpAt = approvalStepUpAt.Time
		}
		if approvalStepUpUntil.Valid {
			item.ApprovalStepUpUntil = approvalStepUpUntil.Time
		}
		if revokedAt.Valid {
			item.RevokedAt = revokedAt.Time
		}
		_ = json.Unmarshal(metadataJSON, &item.ClientMetadata)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) TOTPEnrollments() []TOTPEnrollment {
	const query = `
		SELECT user_id, secret, COALESCE(issuer, ''), COALESCE(account_name, ''), login_enabled, approval_enabled,
		       verified_at, disabled_at, created_at, updated_at
		FROM user_totp_enrollments
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]TOTPEnrollment, 0)
	for rows.Next() {
		var (
			item       TOTPEnrollment
			verifiedAt sql.NullTime
			disabledAt sql.NullTime
		)
		if err := rows.Scan(&item.UserID, &item.Secret, &item.Issuer, &item.AccountName, &item.LoginEnabled, &item.ApprovalEnabled, &verifiedAt, &disabledAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		if verifiedAt.Valid {
			item.VerifiedAt = verifiedAt.Time
		}
		if disabledAt.Valid {
			item.DisabledAt = disabledAt.Time
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) AuthChallenges() []AuthChallenge {
	const query = `
		SELECT challenge_id, user_id, username, auth_method, COALESCE(current_location_id, ''), status, purpose,
		       expires_at, created_at, consumed_at, COALESCE(client_metadata_json, '{}'::jsonb)
		FROM auth_challenges
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]AuthChallenge, 0)
	for rows.Next() {
		var (
			item         AuthChallenge
			consumedAt   sql.NullTime
			metadataJSON []byte
		)
		if err := rows.Scan(&item.ID, &item.UserID, &item.Username, &item.AuthMethod, &item.CurrentLocationID, &item.Status, &item.Purpose, &item.ExpiresAt, &item.CreatedAt, &consumedAt, &metadataJSON); err != nil {
			continue
		}
		if consumedAt.Valid {
			item.ConsumedAt = consumedAt.Time
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

func (r *PostgresRepository) DelegationGrants() []DelegationGrant {
	const query = `
		SELECT delegation_grant_id, grantor_user_id, COALESCE(delegate_kind, 'user'), COALESCE(delegate_id, delegate_user_id, ''), COALESCE(delegate_user_id, ''), status, location_id,
		       COALESCE(allowed_permission_keys_json, '[]'::jsonb),
		       COALESCE(allowed_document_types_json, '[]'::jsonb),
		       COALESCE(reason, ''), starts_at, expires_at, accepted_at,
		       COALESCE(accepted_by_kind, CASE WHEN accepted_by_user_id IS NOT NULL AND accepted_by_user_id <> '' THEN 'user' ELSE '' END),
		       COALESCE(accepted_by_id, accepted_by_user_id, ''),
		       COALESCE(accepted_by_user_id, ''), revoked_at, COALESCE(revoked_by_user_id, ''),
		       created_at, updated_at
		FROM delegation_grants
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]DelegationGrant, 0)
	for rows.Next() {
		var (
			item                 DelegationGrant
			allowedPermissions   []byte
			allowedDocumentTypes []byte
			acceptedAt           sql.NullTime
			revokedAt            sql.NullTime
		)
		if err := rows.Scan(
			&item.ID,
			&item.GrantorUserID,
			&item.DelegateKind,
			&item.DelegateID,
			&item.DelegateUserID,
			&item.Status,
			&item.LocationID,
			&allowedPermissions,
			&allowedDocumentTypes,
			&item.Reason,
			&item.StartsAt,
			&item.ExpiresAt,
			&acceptedAt,
			&item.AcceptedByKind,
			&item.AcceptedByID,
			&item.AcceptedByUserID,
			&revokedAt,
			&item.RevokedByUserID,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			continue
		}
		if acceptedAt.Valid {
			item.AcceptedAt = acceptedAt.Time
		}
		if revokedAt.Valid {
			item.RevokedAt = revokedAt.Time
		}
		_ = json.Unmarshal(allowedPermissions, &item.AllowedPermissionKeys)
		_ = json.Unmarshal(allowedDocumentTypes, &item.AllowedDocumentTypes)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) DeepLinkGrants() []DeepLinkGrant {
	const query = `
		SELECT deep_link_grant_id, grant_kind, user_id, status, target_type, target_id, COALESCE(location_id, ''),
		       COALESCE(allowed_permission_keys_json, '[]'::jsonb),
		       COALESCE(allowed_actions_json, '[]'::jsonb),
		       review_only, require_step_up, one_time, COALESCE(title, ''), COALESCE(message, ''),
		       starts_at, expires_at, activated_at, consumed_at, COALESCE(consumed_by_action, ''), revoked_at,
		       created_at, updated_at, COALESCE(metadata_json, '{}'::jsonb)
		FROM deep_link_grants
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]DeepLinkGrant, 0)
	for rows.Next() {
		var (
			item               DeepLinkGrant
			allowedPermissions []byte
			allowedActions     []byte
			metadataJSON       []byte
			activatedAt        sql.NullTime
			consumedAt         sql.NullTime
			revokedAt          sql.NullTime
		)
		if err := rows.Scan(
			&item.ID,
			&item.Kind,
			&item.UserID,
			&item.Status,
			&item.TargetType,
			&item.TargetID,
			&item.LocationID,
			&allowedPermissions,
			&allowedActions,
			&item.ReviewOnly,
			&item.RequireStepUp,
			&item.OneTime,
			&item.Title,
			&item.Message,
			&item.StartsAt,
			&item.ExpiresAt,
			&activatedAt,
			&consumedAt,
			&item.ConsumedByAction,
			&revokedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&metadataJSON,
		); err != nil {
			continue
		}
		if activatedAt.Valid {
			item.ActivatedAt = activatedAt.Time
		}
		if consumedAt.Valid {
			item.ConsumedAt = consumedAt.Time
		}
		if revokedAt.Valid {
			item.RevokedAt = revokedAt.Time
		}
		_ = json.Unmarshal(allowedPermissions, &item.AllowedPermissionKeys)
		_ = json.Unmarshal(allowedActions, &item.AllowedActions)
		_ = json.Unmarshal(metadataJSON, &item.Metadata)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) ReportingLines() []ReportingLine {
	const query = `
		SELECT reporting_line_id, subject_user_id, manager_user_id, relationship_type,
		       COALESCE(organization_id, ''), COALESCE(location_id, ''), COALESCE(operating_unit_id, ''),
		       status, COALESCE(priority, 0), effective_from, effective_to, created_at, updated_at
		FROM user_reporting_lines
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ReportingLine, 0)
	for rows.Next() {
		var item ReportingLine
		var effectiveTo sql.NullTime
		if err := rows.Scan(&item.ID, &item.SubjectUserID, &item.ManagerUserID, &item.RelationshipType, &item.OrganizationID, &item.LocationID, &item.OperatingUnitID, &item.Status, &item.Priority, &item.EffectiveFrom, &effectiveTo, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		if effectiveTo.Valid {
			item.EffectiveTo = effectiveTo.Time
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveUser(user User) error {
	const query = `
		INSERT INTO users (
			user_id, username, authentication_subject, status, default_location_id, preferred_locale, preferred_user_route, preferred_admin_route, created_at, updated_at
		) VALUES ($1, $2, NULLIF($3, ''), $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9, $10)
		ON CONFLICT (user_id) DO UPDATE SET
			username = EXCLUDED.username,
			authentication_subject = EXCLUDED.authentication_subject,
			status = EXCLUDED.status,
			default_location_id = EXCLUDED.default_location_id,
			preferred_locale = EXCLUDED.preferred_locale,
			preferred_user_route = EXCLUDED.preferred_user_route,
			preferred_admin_route = EXCLUDED.preferred_admin_route,
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
		user.PreferredUserRoute,
		user.PreferredAdminRoute,
		user.CreatedAt,
		user.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) SaveRole(role Role) error {
	const query = `
		INSERT INTO roles (
			role_id, role_key, name, scope_type, default_user_route, default_admin_route, created_at, updated_at
		) VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8)
		ON CONFLICT (role_id) DO UPDATE SET
			role_key = EXCLUDED.role_key,
			name = EXCLUDED.name,
			scope_type = EXCLUDED.scope_type,
			default_user_route = EXCLUDED.default_user_route,
			default_admin_route = EXCLUDED.default_admin_route,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(
		context.Background(),
		query,
		role.ID,
		role.Key,
		role.Name,
		role.ScopeType,
		role.DefaultUserRoute,
		role.DefaultAdminRoute,
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
			role_binding_id, user_id, role_id, scope_type, scope_id, priority, effective_from, effective_to, status
		) VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9)
		ON CONFLICT (role_binding_id) DO UPDATE SET
			role_id = EXCLUDED.role_id,
			scope_type = EXCLUDED.scope_type,
			scope_id = EXCLUDED.scope_id,
			priority = EXCLUDED.priority,
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
		binding.Priority,
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
		       COALESCE(authentication_method, ''), COALESCE(current_location_scope, ''),
		       login_step_up_at, approval_step_up_at, approval_step_up_until, revoked_at,
		       COALESCE(client_metadata_json, '{}'::jsonb)
		FROM sessions
		WHERE session_id = $1`
	var (
		item                Session
		loginStepUpAt       sql.NullTime
		approvalStepUpAt    sql.NullTime
		approvalStepUpUntil sql.NullTime
		revokedAt           sql.NullTime
		metadataJSON        []byte
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
		&loginStepUpAt,
		&approvalStepUpAt,
		&approvalStepUpUntil,
		&revokedAt,
		&metadataJSON,
	)
	if err != nil {
		return Session{}, false
	}
	if loginStepUpAt.Valid {
		item.LoginStepUpAt = loginStepUpAt.Time
	}
	if approvalStepUpAt.Valid {
		item.ApprovalStepUpAt = approvalStepUpAt.Time
	}
	if approvalStepUpUntil.Valid {
		item.ApprovalStepUpUntil = approvalStepUpUntil.Time
	}
	if revokedAt.Valid {
		item.RevokedAt = revokedAt.Time
	}
	_ = json.Unmarshal(metadataJSON, &item.ClientMetadata)
	return item, true
}

func (r *PostgresRepository) FindTOTPEnrollmentByUserID(userID string) (TOTPEnrollment, bool) {
	const query = `
		SELECT user_id, secret, COALESCE(issuer, ''), COALESCE(account_name, ''), login_enabled, approval_enabled,
		       verified_at, disabled_at, created_at, updated_at
		FROM user_totp_enrollments
		WHERE user_id = $1`
	var (
		item       TOTPEnrollment
		verifiedAt sql.NullTime
		disabledAt sql.NullTime
	)
	err := r.db.QueryRowContext(context.Background(), query, userID).Scan(
		&item.UserID,
		&item.Secret,
		&item.Issuer,
		&item.AccountName,
		&item.LoginEnabled,
		&item.ApprovalEnabled,
		&verifiedAt,
		&disabledAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return TOTPEnrollment{}, false
	}
	if verifiedAt.Valid {
		item.VerifiedAt = verifiedAt.Time
	}
	if disabledAt.Valid {
		item.DisabledAt = disabledAt.Time
	}
	return item, true
}

func (r *PostgresRepository) FindAuthChallenge(id string) (AuthChallenge, bool) {
	const query = `
		SELECT challenge_id, user_id, username, auth_method, COALESCE(current_location_id, ''), status, purpose,
		       expires_at, created_at, consumed_at, COALESCE(client_metadata_json, '{}'::jsonb)
		FROM auth_challenges
		WHERE challenge_id = $1`
	var (
		item         AuthChallenge
		consumedAt   sql.NullTime
		metadataJSON []byte
	)
	err := r.db.QueryRowContext(context.Background(), query, id).Scan(
		&item.ID,
		&item.UserID,
		&item.Username,
		&item.AuthMethod,
		&item.CurrentLocationID,
		&item.Status,
		&item.Purpose,
		&item.ExpiresAt,
		&item.CreatedAt,
		&consumedAt,
		&metadataJSON,
	)
	if err != nil {
		return AuthChallenge{}, false
	}
	if consumedAt.Valid {
		item.ConsumedAt = consumedAt.Time
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

func (r *PostgresRepository) FindDelegationGrant(id string) (DelegationGrant, bool) {
	const query = `
		SELECT delegation_grant_id, grantor_user_id, COALESCE(delegate_kind, 'user'), COALESCE(delegate_id, delegate_user_id, ''), COALESCE(delegate_user_id, ''), status, location_id,
		       COALESCE(allowed_permission_keys_json, '[]'::jsonb),
		       COALESCE(allowed_document_types_json, '[]'::jsonb),
		       COALESCE(reason, ''), starts_at, expires_at, accepted_at,
		       COALESCE(accepted_by_kind, CASE WHEN accepted_by_user_id IS NOT NULL AND accepted_by_user_id <> '' THEN 'user' ELSE '' END),
		       COALESCE(accepted_by_id, accepted_by_user_id, ''),
		       COALESCE(accepted_by_user_id, ''), revoked_at, COALESCE(revoked_by_user_id, ''),
		       created_at, updated_at
		FROM delegation_grants
		WHERE delegation_grant_id = $1`
	var (
		item                 DelegationGrant
		allowedPermissions   []byte
		allowedDocumentTypes []byte
		acceptedAt           sql.NullTime
		revokedAt            sql.NullTime
	)
	err := r.db.QueryRowContext(context.Background(), query, id).Scan(
		&item.ID,
		&item.GrantorUserID,
		&item.DelegateKind,
		&item.DelegateID,
		&item.DelegateUserID,
		&item.Status,
		&item.LocationID,
		&allowedPermissions,
		&allowedDocumentTypes,
		&item.Reason,
		&item.StartsAt,
		&item.ExpiresAt,
		&acceptedAt,
		&item.AcceptedByKind,
		&item.AcceptedByID,
		&item.AcceptedByUserID,
		&revokedAt,
		&item.RevokedByUserID,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return DelegationGrant{}, false
	}
	if acceptedAt.Valid {
		item.AcceptedAt = acceptedAt.Time
	}
	if revokedAt.Valid {
		item.RevokedAt = revokedAt.Time
	}
	_ = json.Unmarshal(allowedPermissions, &item.AllowedPermissionKeys)
	_ = json.Unmarshal(allowedDocumentTypes, &item.AllowedDocumentTypes)
	return item, true
}

func (r *PostgresRepository) FindDeepLinkGrant(id string) (DeepLinkGrant, bool) {
	const query = `
		SELECT deep_link_grant_id, grant_kind, user_id, status, target_type, target_id, COALESCE(location_id, ''),
		       COALESCE(allowed_permission_keys_json, '[]'::jsonb),
		       COALESCE(allowed_actions_json, '[]'::jsonb),
		       review_only, require_step_up, one_time, COALESCE(title, ''), COALESCE(message, ''),
		       starts_at, expires_at, activated_at, consumed_at, COALESCE(consumed_by_action, ''), revoked_at,
		       created_at, updated_at, COALESCE(metadata_json, '{}'::jsonb)
		FROM deep_link_grants
		WHERE deep_link_grant_id = $1`
	var (
		item               DeepLinkGrant
		allowedPermissions []byte
		allowedActions     []byte
		metadataJSON       []byte
		activatedAt        sql.NullTime
		consumedAt         sql.NullTime
		revokedAt          sql.NullTime
	)
	err := r.db.QueryRowContext(context.Background(), query, id).Scan(
		&item.ID,
		&item.Kind,
		&item.UserID,
		&item.Status,
		&item.TargetType,
		&item.TargetID,
		&item.LocationID,
		&allowedPermissions,
		&allowedActions,
		&item.ReviewOnly,
		&item.RequireStepUp,
		&item.OneTime,
		&item.Title,
		&item.Message,
		&item.StartsAt,
		&item.ExpiresAt,
		&activatedAt,
		&consumedAt,
		&item.ConsumedByAction,
		&revokedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&metadataJSON,
	)
	if err != nil {
		return DeepLinkGrant{}, false
	}
	if activatedAt.Valid {
		item.ActivatedAt = activatedAt.Time
	}
	if consumedAt.Valid {
		item.ConsumedAt = consumedAt.Time
	}
	if revokedAt.Valid {
		item.RevokedAt = revokedAt.Time
	}
	_ = json.Unmarshal(allowedPermissions, &item.AllowedPermissionKeys)
	_ = json.Unmarshal(allowedActions, &item.AllowedActions)
	_ = json.Unmarshal(metadataJSON, &item.Metadata)
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

func (r *PostgresRepository) SaveDelegationGrant(grant DelegationGrant) error {
	const query = `
		INSERT INTO delegation_grants (
			delegation_grant_id, grantor_user_id, delegate_kind, delegate_id, delegate_user_id, status, location_id,
			allowed_permission_keys_json, allowed_document_types_json, reason, starts_at, expires_at,
			accepted_at, accepted_by_kind, accepted_by_id, accepted_by_user_id, revoked_at, revoked_by_user_id, created_at, updated_at
		) VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8::jsonb, $9::jsonb, NULLIF($10, ''), $11, $12, $13, NULLIF($14, ''), NULLIF($15, ''), NULLIF($16, ''), $17, NULLIF($18, ''), $19, $20)
		ON CONFLICT (delegation_grant_id) DO UPDATE SET
			delegate_kind = EXCLUDED.delegate_kind,
			delegate_id = EXCLUDED.delegate_id,
			delegate_user_id = EXCLUDED.delegate_user_id,
			status = EXCLUDED.status,
			location_id = EXCLUDED.location_id,
			allowed_permission_keys_json = EXCLUDED.allowed_permission_keys_json,
			allowed_document_types_json = EXCLUDED.allowed_document_types_json,
			reason = EXCLUDED.reason,
			starts_at = EXCLUDED.starts_at,
			expires_at = EXCLUDED.expires_at,
			accepted_at = EXCLUDED.accepted_at,
			accepted_by_kind = EXCLUDED.accepted_by_kind,
			accepted_by_id = EXCLUDED.accepted_by_id,
			accepted_by_user_id = EXCLUDED.accepted_by_user_id,
			revoked_at = EXCLUDED.revoked_at,
			revoked_by_user_id = EXCLUDED.revoked_by_user_id,
			updated_at = EXCLUDED.updated_at`
	allowedPermissionsJSON, err := json.Marshal(grant.AllowedPermissionKeys)
	if err != nil {
		return err
	}
	allowedDocumentTypesJSON, err := json.Marshal(grant.AllowedDocumentTypes)
	if err != nil {
		return err
	}
	var acceptedAt any
	if !grant.AcceptedAt.IsZero() {
		acceptedAt = grant.AcceptedAt
	}
	var revokedAt any
	if !grant.RevokedAt.IsZero() {
		revokedAt = grant.RevokedAt
	}
	_, err = r.db.ExecContext(
		context.Background(),
		query,
		grant.ID,
		grant.GrantorUserID,
		grant.DelegateKind,
		grant.DelegateID,
		grant.DelegateUserID,
		grant.Status,
		grant.LocationID,
		string(allowedPermissionsJSON),
		string(allowedDocumentTypesJSON),
		grant.Reason,
		grant.StartsAt,
		grant.ExpiresAt,
		acceptedAt,
		grant.AcceptedByKind,
		grant.AcceptedByID,
		grant.AcceptedByUserID,
		revokedAt,
		grant.RevokedByUserID,
		grant.CreatedAt,
		grant.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) SaveDeepLinkGrant(grant DeepLinkGrant) error {
	const query = `
		INSERT INTO deep_link_grants (
			deep_link_grant_id, grant_kind, user_id, status, target_type, target_id, location_id,
			allowed_permission_keys_json, allowed_actions_json, review_only, require_step_up, one_time,
			title, message, starts_at, expires_at, activated_at, consumed_at, consumed_by_action, revoked_at,
			created_at, updated_at, metadata_json
		) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8::jsonb, $9::jsonb, $10, $11, $12, NULLIF($13, ''), NULLIF($14, ''), $15, $16, $17, $18, NULLIF($19, ''), $20, $21, $22, $23::jsonb)
		ON CONFLICT (deep_link_grant_id) DO UPDATE SET
			grant_kind = EXCLUDED.grant_kind,
			user_id = EXCLUDED.user_id,
			status = EXCLUDED.status,
			target_type = EXCLUDED.target_type,
			target_id = EXCLUDED.target_id,
			location_id = EXCLUDED.location_id,
			allowed_permission_keys_json = EXCLUDED.allowed_permission_keys_json,
			allowed_actions_json = EXCLUDED.allowed_actions_json,
			review_only = EXCLUDED.review_only,
			require_step_up = EXCLUDED.require_step_up,
			one_time = EXCLUDED.one_time,
			title = EXCLUDED.title,
			message = EXCLUDED.message,
			starts_at = EXCLUDED.starts_at,
			expires_at = EXCLUDED.expires_at,
			activated_at = EXCLUDED.activated_at,
			consumed_at = EXCLUDED.consumed_at,
			consumed_by_action = EXCLUDED.consumed_by_action,
			revoked_at = EXCLUDED.revoked_at,
			updated_at = EXCLUDED.updated_at,
			metadata_json = EXCLUDED.metadata_json`
	allowedPermissionsJSON, err := json.Marshal(grant.AllowedPermissionKeys)
	if err != nil {
		return err
	}
	allowedActionsJSON, err := json.Marshal(grant.AllowedActions)
	if err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(grant.Metadata)
	if err != nil {
		return err
	}
	var activatedAt any
	if !grant.ActivatedAt.IsZero() {
		activatedAt = grant.ActivatedAt
	}
	var consumedAt any
	if !grant.ConsumedAt.IsZero() {
		consumedAt = grant.ConsumedAt
	}
	var revokedAt any
	if !grant.RevokedAt.IsZero() {
		revokedAt = grant.RevokedAt
	}
	_, err = r.db.ExecContext(
		context.Background(),
		query,
		grant.ID,
		grant.Kind,
		grant.UserID,
		grant.Status,
		grant.TargetType,
		grant.TargetID,
		grant.LocationID,
		string(allowedPermissionsJSON),
		string(allowedActionsJSON),
		grant.ReviewOnly,
		grant.RequireStepUp,
		grant.OneTime,
		grant.Title,
		grant.Message,
		grant.StartsAt,
		grant.ExpiresAt,
		activatedAt,
		consumedAt,
		grant.ConsumedByAction,
		revokedAt,
		grant.CreatedAt,
		grant.UpdatedAt,
		string(metadataJSON),
	)
	return err
}

func (r *PostgresRepository) SaveReportingLine(line ReportingLine) error {
	const query = `
		INSERT INTO user_reporting_lines (
			reporting_line_id, subject_user_id, manager_user_id, relationship_type, organization_id, location_id,
			operating_unit_id, status, priority, effective_from, effective_to, created_at, updated_at
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),$8,$9,$10,$11,$12,$13)
		ON CONFLICT (reporting_line_id) DO UPDATE SET
			subject_user_id = EXCLUDED.subject_user_id,
			manager_user_id = EXCLUDED.manager_user_id,
			relationship_type = EXCLUDED.relationship_type,
			organization_id = EXCLUDED.organization_id,
			location_id = EXCLUDED.location_id,
			operating_unit_id = EXCLUDED.operating_unit_id,
			status = EXCLUDED.status,
			priority = EXCLUDED.priority,
			effective_from = EXCLUDED.effective_from,
			effective_to = EXCLUDED.effective_to,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, line.ID, line.SubjectUserID, line.ManagerUserID, line.RelationshipType, line.OrganizationID, line.LocationID, line.OperatingUnitID, line.Status, line.Priority, line.EffectiveFrom, nullableTime(line.EffectiveTo), line.CreatedAt, line.UpdatedAt)
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
			authentication_method, client_metadata_json, current_location_scope,
			login_step_up_at, approval_step_up_at, approval_step_up_until, revoked_at
		) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8::jsonb, NULLIF($9, ''), $10, $11, $12, $13)
		ON CONFLICT (session_id) DO UPDATE SET
			status = EXCLUDED.status,
			expires_at = EXCLUDED.expires_at,
			last_seen_at = EXCLUDED.last_seen_at,
			authentication_method = EXCLUDED.authentication_method,
			client_metadata_json = EXCLUDED.client_metadata_json,
			current_location_scope = EXCLUDED.current_location_scope,
			login_step_up_at = EXCLUDED.login_step_up_at,
			approval_step_up_at = EXCLUDED.approval_step_up_at,
			approval_step_up_until = EXCLUDED.approval_step_up_until,
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
		nullableTime(session.LoginStepUpAt),
		nullableTime(session.ApprovalStepUpAt),
		nullableTime(session.ApprovalStepUpUntil),
		revokedAt,
	)
	return err
}

func (r *PostgresRepository) SaveCredential(credential Credential) error {
	const query = `
		INSERT INTO user_credentials (
			user_id, password_hash, password_changed_at, cashier_pin_hash, cashier_pin_changed_at, failed_attempt_count, locked_until, updated_at
		) VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			password_changed_at = EXCLUDED.password_changed_at,
			cashier_pin_hash = EXCLUDED.cashier_pin_hash,
			cashier_pin_changed_at = EXCLUDED.cashier_pin_changed_at,
			failed_attempt_count = EXCLUDED.failed_attempt_count,
			locked_until = EXCLUDED.locked_until,
			updated_at = EXCLUDED.updated_at`
	var cashierPINChangedAt any
	if !credential.CashierPINChangedAt.IsZero() {
		cashierPINChangedAt = credential.CashierPINChangedAt
	}
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
		credential.CashierPINHash,
		cashierPINChangedAt,
		credential.FailedAttemptCount,
		lockedUntil,
		credential.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) SaveTOTPEnrollment(enrollment TOTPEnrollment) error {
	const query = `
		INSERT INTO user_totp_enrollments (
			user_id, secret, issuer, account_name, login_enabled, approval_enabled,
			verified_at, disabled_at, created_at, updated_at
		) VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id) DO UPDATE SET
			secret = EXCLUDED.secret,
			issuer = EXCLUDED.issuer,
			account_name = EXCLUDED.account_name,
			login_enabled = EXCLUDED.login_enabled,
			approval_enabled = EXCLUDED.approval_enabled,
			verified_at = EXCLUDED.verified_at,
			disabled_at = EXCLUDED.disabled_at,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(
		context.Background(),
		query,
		enrollment.UserID,
		enrollment.Secret,
		enrollment.Issuer,
		enrollment.AccountName,
		enrollment.LoginEnabled,
		enrollment.ApprovalEnabled,
		nullableTime(enrollment.VerifiedAt),
		nullableTime(enrollment.DisabledAt),
		enrollment.CreatedAt,
		enrollment.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) SaveAuthChallenge(challenge AuthChallenge) error {
	const query = `
		INSERT INTO auth_challenges (
			challenge_id, user_id, username, auth_method, current_location_id, status, purpose,
			expires_at, created_at, consumed_at, client_metadata_json
		) VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8, $9, $10, $11::jsonb)
		ON CONFLICT (challenge_id) DO UPDATE SET
			status = EXCLUDED.status,
			expires_at = EXCLUDED.expires_at,
			consumed_at = EXCLUDED.consumed_at,
			client_metadata_json = EXCLUDED.client_metadata_json`
	metadataJSON, err := json.Marshal(challenge.ClientMetadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(
		context.Background(),
		query,
		challenge.ID,
		challenge.UserID,
		challenge.Username,
		challenge.AuthMethod,
		challenge.CurrentLocationID,
		challenge.Status,
		challenge.Purpose,
		challenge.ExpiresAt,
		challenge.CreatedAt,
		nullableTime(challenge.ConsumedAt),
		string(metadataJSON),
	)
	return err
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
