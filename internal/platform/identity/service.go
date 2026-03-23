package identity

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/shared"
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
		ID:                "role_admin",
		Key:               "platform_admin",
		Name:              "Platform Administrator",
		ScopeType:         "deployment",
		DefaultAdminRoute: "/admin/modules",
		CreatedAt:         now,
		UpdatedAt:         now,
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
		Key:      "analytics.author",
		Module:   "analytics",
		Action:   "author",
		Resource: "analytics_runtime",
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
		Key:      "template.read",
		Module:   "platform",
		Action:   "read",
		Resource: "template",
	}, {
		Key:      "template.manage",
		Module:   "platform",
		Action:   "manage",
		Resource: "template",
	}, {
		Key:      "template.publish",
		Module:   "platform",
		Action:   "publish",
		Resource: "template",
	}, {
		Key:      "template.bind",
		Module:   "platform",
		Action:   "bind",
		Resource: "template",
	}, {
		Key:      "template.render",
		Module:   "platform",
		Action:   "render",
		Resource: "template_output",
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
	}, {
		Key:      "identity.manage_service_principals",
		Module:   "identity",
		Action:   "manage",
		Resource: "service_principal",
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
		if err == nil {
			credentials = append(credentials, Credential{
				UserID:            "user_admin",
				PasswordHash:      adminPasswordHash,
				PasswordChangedAt: now,
				UpdatedAt:         now,
			})
		}
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
		Priority:      100,
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
		PermissionKey: "analytics.author",
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
		PermissionKey: "template.read",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "template.manage",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "template.publish",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "template.bind",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "template.render",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "identity.manage_sessions",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "identity.manage_users",
	}, {
		RoleID:        "role_admin",
		PermissionKey: "identity.manage_service_principals",
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

func (s *Service) PreferredLocale(userID string) string {
	user, ok := s.repo.FindUser(userID)
	if !ok {
		return ""
	}
	if strings.TrimSpace(user.PreferredLocale) == "" {
		return ""
	}
	return i18n.NormalizeLocale(user.PreferredLocale)
}

func (s *Service) PreferredRoute(userID, surface string) string {
	user, ok := s.repo.FindUser(userID)
	if !ok {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(surface)) {
	case "admin":
		return normalizeRoutePreference(user.PreferredAdminRoute)
	default:
		return normalizeRoutePreference(user.PreferredUserRoute)
	}
}

func (s *Service) DefaultRoute(userID, surface string) string {
	now := time.Now().UTC()
	type rankedRole struct {
		role     Role
		priority int
		from     time.Time
	}
	ranked := make([]rankedRole, 0)
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
		role, ok := s.findRole(binding.RoleID)
		if !ok {
			continue
		}
		ranked = append(ranked, rankedRole{role: role, priority: binding.Priority, from: binding.EffectiveFrom})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].priority == ranked[j].priority {
			if ranked[i].from.Equal(ranked[j].from) {
				return ranked[i].role.ID < ranked[j].role.ID
			}
			return ranked[i].from.After(ranked[j].from)
		}
		return ranked[i].priority > ranked[j].priority
	})
	for _, item := range ranked {
		switch strings.ToLower(strings.TrimSpace(surface)) {
		case "admin":
			if route := normalizeRoutePreference(item.role.DefaultAdminRoute); route != "" {
				return route
			}
		default:
			if route := normalizeRoutePreference(item.role.DefaultUserRoute); route != "" {
				return route
			}
		}
	}
	return ""
}

func (s *Service) Bindings() []RoleBinding {
	return s.repo.RoleBindings()
}

func (s *Service) ReportingLines() []ReportingLine {
	items := append([]ReportingLine(nil), s.repo.ReportingLines()...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].SubjectUserID == items[j].SubjectUserID {
			if items[i].Priority == items[j].Priority {
				return items[i].ID < items[j].ID
			}
			return items[i].Priority > items[j].Priority
		}
		return items[i].SubjectUserID < items[j].SubjectUserID
	})
	return items
}

func (s *Service) UpsertReportingLine(line ReportingLine) (ReportingLine, error) {
	now := time.Now().UTC()
	line.ID = strings.TrimSpace(line.ID)
	line.SubjectUserID = strings.TrimSpace(line.SubjectUserID)
	line.ManagerUserID = strings.TrimSpace(line.ManagerUserID)
	line.RelationshipType = strings.ToLower(strings.TrimSpace(line.RelationshipType))
	line.OrganizationID = strings.TrimSpace(line.OrganizationID)
	line.LocationID = strings.TrimSpace(line.LocationID)
	line.OperatingUnitID = strings.TrimSpace(line.OperatingUnitID)
	line.Status = strings.ToLower(strings.TrimSpace(line.Status))
	if line.ID == "" {
		line.ID = fmt.Sprintf("reporting_line:%d", now.UnixNano())
	}
	if line.SubjectUserID == "" || line.ManagerUserID == "" {
		return ReportingLine{}, shared.Validation("subject_user_id and manager_user_id are required")
	}
	if line.SubjectUserID == line.ManagerUserID {
		return ReportingLine{}, shared.Validation("subject_user_id and manager_user_id must differ")
	}
	if _, ok := s.repo.FindUser(line.SubjectUserID); !ok {
		return ReportingLine{}, shared.NotFound("subject user not found")
	}
	if _, ok := s.repo.FindUser(line.ManagerUserID); !ok {
		return ReportingLine{}, shared.NotFound("manager user not found")
	}
	switch line.RelationshipType {
	case "", "primary_manager":
		line.RelationshipType = "primary_manager"
	case "acting_manager":
	default:
		return ReportingLine{}, shared.Validation("relationship_type must be primary_manager or acting_manager")
	}
	if line.Status == "" {
		line.Status = "active"
	}
	if line.Status != "active" && line.Status != "inactive" {
		return ReportingLine{}, shared.Validation("status must be active or inactive")
	}
	if line.EffectiveFrom.IsZero() {
		line.EffectiveFrom = now
	}
	if !line.EffectiveTo.IsZero() && line.EffectiveTo.Before(line.EffectiveFrom) {
		return ReportingLine{}, shared.Validation("effective_to must be after effective_from")
	}
	if line.CreatedAt.IsZero() {
		line.CreatedAt = now
	}
	line.UpdatedAt = now
	if err := s.repo.SaveReportingLine(line); err != nil {
		return ReportingLine{}, err
	}
	return line, nil
}

func (s *Service) ResolveManager(subjectUserID, organizationID, locationID, operatingUnitID string, at time.Time) (ManagerResolution, bool) {
	lines := activeReportingLinesForUser(s.repo.ReportingLines(), strings.TrimSpace(subjectUserID), organizationID, locationID, operatingUnitID, resolveTime(at))
	if len(lines) == 0 {
		return ManagerResolution{}, false
	}
	sort.SliceStable(lines, func(i, j int) bool {
		left := reportingLineRank(lines[i].RelationshipType)
		right := reportingLineRank(lines[j].RelationshipType)
		if left == right {
			if lines[i].Priority == lines[j].Priority {
				return lines[i].ID < lines[j].ID
			}
			return lines[i].Priority > lines[j].Priority
		}
		return left < right
	})
	for _, line := range lines {
		manager, ok := s.repo.FindUser(line.ManagerUserID)
		if !ok || manager.Status != "active" {
			continue
		}
		return ManagerResolution{Line: line, Manager: manager, Via: line.RelationshipType}, true
	}
	return ManagerResolution{}, false
}

func (s *Service) ResolveRoleCandidates(roleKey, organizationID, locationID, operatingUnitID string, at time.Time) []User {
	roleKey = strings.TrimSpace(roleKey)
	if roleKey == "" {
		return nil
	}
	role, ok := s.findRoleByKey(roleKey)
	if !ok {
		return nil
	}
	now := resolveTime(at)
	candidateIDs := map[string]struct{}{}
	for _, binding := range s.repo.RoleBindings() {
		if binding.RoleID != role.ID || binding.Status != "active" {
			continue
		}
		if binding.EffectiveFrom.After(now) {
			continue
		}
		if !binding.EffectiveTo.IsZero() && binding.EffectiveTo.Before(now) {
			continue
		}
		if !bindingMatchesScope(binding, organizationID, locationID, operatingUnitID) {
			continue
		}
		user, ok := s.repo.FindUser(binding.UserID)
		if !ok || user.Status != "active" {
			continue
		}
		candidateIDs[user.ID] = struct{}{}
	}
	if len(candidateIDs) == 0 {
		return nil
	}
	users := make([]User, 0, len(candidateIDs))
	for _, user := range s.repo.Users() {
		if _, ok := candidateIDs[user.ID]; ok {
			users = append(users, user)
		}
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].Username == users[j].Username {
			return users[i].ID < users[j].ID
		}
		return users[i].Username < users[j].Username
	})
	return users
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

func (s *Service) DelegationGrants() []DelegationGrant {
	items := append([]DelegationGrant(nil), s.repo.DelegationGrants()...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}

func (s *Service) FindSession(id string) (Session, bool) {
	return s.repo.FindSession(id)
}

func (s *Service) FindUserByUsername(username string) (User, bool) {
	return s.repo.FindUserByUsername(username)
}

func (s *Service) SetUserPreferredLocale(userID, locale string) (User, error) {
	user, ok := s.repo.FindUser(userID)
	if !ok {
		return User{}, shared.NotFound("user not found")
	}
	user.PreferredLocale = i18n.NormalizeLocale(locale)
	user.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveUser(user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Service) SetUserPreferredRoutes(userID, userRoute, adminRoute string) (User, error) {
	user, ok := s.repo.FindUser(userID)
	if !ok {
		return User{}, shared.NotFound("user not found")
	}
	user.PreferredUserRoute = normalizeRoutePreference(userRoute)
	user.PreferredAdminRoute = normalizeRoutePreference(adminRoute)
	user.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveUser(user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Service) FindCredentialByUserID(userID string) (Credential, bool) {
	return s.repo.FindCredentialByUserID(userID)
}

func (s *Service) FindServicePrincipal(id string) (ServicePrincipal, bool) {
	return s.repo.FindServicePrincipal(id)
}

func (s *Service) FindDelegationGrant(id string) (DelegationGrant, bool) {
	return s.repo.FindDelegationGrant(id)
}

func (s *Service) ListOutgoingDelegationGrants(userID string) []DelegationGrant {
	items := make([]DelegationGrant, 0)
	for _, item := range s.DelegationGrants() {
		if item.GrantorUserID == userID {
			items = append(items, s.normalizeDelegationGrant(item))
		}
	}
	return items
}

func (s *Service) ListIncomingDelegationGrants(userID string) []DelegationGrant {
	items := make([]DelegationGrant, 0)
	for _, item := range s.DelegationGrants() {
		normalized := s.normalizeDelegationGrant(item)
		if normalized.DelegateKind == "user" && normalized.DelegateID == userID {
			items = append(items, normalized)
		}
	}
	return items
}

func (s *Service) ListIncomingAgentDelegationGrants(servicePrincipalID string) []DelegationGrant {
	items := make([]DelegationGrant, 0)
	for _, item := range s.DelegationGrants() {
		normalized := s.normalizeDelegationGrant(item)
		if normalized.DelegateKind == "agent" && normalized.DelegateID == servicePrincipalID {
			items = append(items, normalized)
		}
	}
	return items
}

func (s *Service) UpsertServicePrincipal(principal ServicePrincipal) (ServicePrincipal, error) {
	if strings.TrimSpace(principal.Key) == "" {
		return ServicePrincipal{}, shared.Validation("service principal key is required")
	}
	status := strings.TrimSpace(strings.ToLower(principal.Status))
	if status == "" {
		status = "active"
	}
	switch status {
	case "active", "disabled":
	default:
		return ServicePrincipal{}, shared.Validation("service principal status must be active or disabled")
	}
	now := time.Now().UTC()
	if strings.TrimSpace(principal.ID) == "" {
		principal.ID = fmt.Sprintf("sp:%d", now.UnixNano())
	}
	if existing, ok := s.repo.FindServicePrincipal(principal.ID); ok {
		principal.CreatedAt = existing.CreatedAt
		if principal.CredentialRef == "" {
			principal.CredentialRef = existing.CredentialRef
		}
	}
	if principal.CreatedAt.IsZero() {
		principal.CreatedAt = now
	}
	principal.Key = strings.TrimSpace(principal.Key)
	principal.Status = status
	principal.UpdatedAt = now
	if principal.CredentialRef == "" {
		principal.CredentialRef = "managed://" + principal.Key
	}
	filtered := make([]string, 0, len(principal.AllowedOperationTypes))
	seen := map[string]bool{}
	for _, item := range principal.AllowedOperationTypes {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		filtered = append(filtered, item)
	}
	principal.AllowedOperationTypes = filtered
	return principal, s.repo.SaveServicePrincipal(principal)
}

func (s *Service) SetServicePrincipalStatus(id, status string) (ServicePrincipal, error) {
	principal, ok := s.repo.FindServicePrincipal(strings.TrimSpace(id))
	if !ok {
		return ServicePrincipal{}, shared.NotFound("service principal not found")
	}
	principal.Status = status
	return s.UpsertServicePrincipal(principal)
}

func (s *Service) createDelegationGrant(grantorUserID, delegateKind, delegateID, locationID string, allowedPermissionKeys, allowedDocumentTypes []string, startsAt, expiresAt time.Time, reason string) (DelegationGrant, error) {
	grantorUserID = strings.TrimSpace(grantorUserID)
	delegateKind = strings.ToLower(strings.TrimSpace(delegateKind))
	delegateID = strings.TrimSpace(delegateID)
	locationID = strings.TrimSpace(locationID)
	reason = strings.TrimSpace(reason)
	if grantorUserID == "" || delegateID == "" {
		return DelegationGrant{}, shared.Validation("grantor and delegate are required")
	}
	switch delegateKind {
	case "", "user":
		delegateKind = "user"
	case "agent":
	default:
		return DelegationGrant{}, shared.Validation("delegate_kind must be user or agent")
	}
	if delegateKind == "user" && grantorUserID == delegateID {
		return DelegationGrant{}, shared.Validation("delegate user must be different from grantor")
	}
	if locationID == "" {
		return DelegationGrant{}, shared.Validation("location_id is required")
	}
	if ctx := s.organization.Resolve(locationID); ctx.LocationID == "" {
		return DelegationGrant{}, shared.Validation("location_id is invalid")
	}
	if _, ok := s.repo.FindUser(grantorUserID); !ok {
		return DelegationGrant{}, shared.NotFound("grantor user not found")
	}
	switch delegateKind {
	case "user":
		if _, ok := s.repo.FindUser(delegateID); !ok {
			return DelegationGrant{}, shared.NotFound("delegate user not found")
		}
	case "agent":
		principal, ok := s.repo.FindServicePrincipal(delegateID)
		if !ok {
			return DelegationGrant{}, shared.NotFound("delegate service principal not found")
		}
		if principal.Status != "active" {
			return DelegationGrant{}, shared.Forbidden("delegate service principal is not active")
		}
	}
	allowedPermissionKeys = normalizeStringList(allowedPermissionKeys)
	if len(allowedPermissionKeys) == 0 {
		return DelegationGrant{}, shared.Validation("allowed_permission_keys is required")
	}
	for _, permissionKey := range allowedPermissionKeys {
		if !s.permissionExists(permissionKey) {
			return DelegationGrant{}, shared.Validation("unknown permission key: " + permissionKey)
		}
		if !s.Decide(grantorUserID, permissionKey, locationID).Allowed {
			return DelegationGrant{}, shared.Forbidden("grantor is not allowed to delegate permission: " + permissionKey)
		}
	}
	allowedDocumentTypes = normalizeStringList(allowedDocumentTypes)
	now := time.Now().UTC()
	if startsAt.IsZero() {
		startsAt = now
	} else {
		startsAt = startsAt.UTC()
	}
	if expiresAt.IsZero() {
		return DelegationGrant{}, shared.Validation("expires_at is required")
	}
	expiresAt = expiresAt.UTC()
	if !expiresAt.After(startsAt) {
		return DelegationGrant{}, shared.Validation("expires_at must be after starts_at")
	}
	grant := DelegationGrant{
		ID:                    fmt.Sprintf("dlg:%d", now.UnixNano()),
		GrantorUserID:         grantorUserID,
		DelegateKind:          delegateKind,
		DelegateID:            delegateID,
		Status:                "pending",
		LocationID:            locationID,
		AllowedPermissionKeys: allowedPermissionKeys,
		AllowedDocumentTypes:  allowedDocumentTypes,
		Reason:                reason,
		StartsAt:              startsAt,
		ExpiresAt:             expiresAt,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if delegateKind == "user" {
		grant.DelegateUserID = delegateID
	}
	if err := s.repo.SaveDelegationGrant(grant); err != nil {
		return DelegationGrant{}, err
	}
	return grant, nil
}

func (s *Service) CreateDelegationGrant(grantorUserID, delegateUserID, locationID string, allowedPermissionKeys, allowedDocumentTypes []string, startsAt, expiresAt time.Time, reason string) (DelegationGrant, error) {
	return s.createDelegationGrant(grantorUserID, "user", delegateUserID, locationID, allowedPermissionKeys, allowedDocumentTypes, startsAt, expiresAt, reason)
}

func (s *Service) AcceptDelegationGrant(grantID, delegateUserID string) (DelegationGrant, error) {
	return s.acceptDelegationGrant(grantID, "user", delegateUserID)
}

func (s *Service) RejectDelegationGrant(grantID, delegateUserID string) (DelegationGrant, error) {
	return s.rejectDelegationGrant(grantID, "user", delegateUserID)
}

func (s *Service) CreateAgentDelegationGrant(grantorUserID, servicePrincipalID, locationID string, allowedPermissionKeys, allowedDocumentTypes []string, startsAt, expiresAt time.Time, reason string) (DelegationGrant, error) {
	return s.createDelegationGrant(grantorUserID, "agent", servicePrincipalID, locationID, allowedPermissionKeys, allowedDocumentTypes, startsAt, expiresAt, reason)
}

func (s *Service) AcceptAgentDelegationGrant(grantID, servicePrincipalID string) (DelegationGrant, error) {
	return s.acceptDelegationGrant(grantID, "agent", servicePrincipalID)
}

func (s *Service) RejectAgentDelegationGrant(grantID, servicePrincipalID string) (DelegationGrant, error) {
	return s.rejectDelegationGrant(grantID, "agent", servicePrincipalID)
}

func (s *Service) acceptDelegationGrant(grantID, delegateKind, delegateID string) (DelegationGrant, error) {
	grant, err := s.requireDelegationGrant(grantID)
	if err != nil {
		return DelegationGrant{}, err
	}
	grant = s.normalizeDelegationGrant(grant)
	if grant.DelegateKind != strings.TrimSpace(delegateKind) || grant.DelegateID != strings.TrimSpace(delegateID) {
		return DelegationGrant{}, shared.Forbidden("delegation grant is not assigned to the current user")
	}
	if grant.Status != "pending" {
		return DelegationGrant{}, shared.Conflict("delegation grant is not pending")
	}
	now := time.Now().UTC()
	grant.Status = "accepted"
	grant.AcceptedAt = now
	grant.AcceptedByKind = delegateKind
	grant.AcceptedByID = strings.TrimSpace(delegateID)
	if grant.AcceptedByKind == "user" {
		grant.AcceptedByUserID = grant.AcceptedByID
	} else {
		grant.AcceptedByUserID = ""
	}
	grant.UpdatedAt = now
	if err := s.repo.SaveDelegationGrant(grant); err != nil {
		return DelegationGrant{}, err
	}
	return grant, nil
}

func (s *Service) rejectDelegationGrant(grantID, delegateKind, delegateID string) (DelegationGrant, error) {
	grant, err := s.requireDelegationGrant(grantID)
	if err != nil {
		return DelegationGrant{}, err
	}
	grant = s.normalizeDelegationGrant(grant)
	if grant.DelegateKind != strings.TrimSpace(delegateKind) || grant.DelegateID != strings.TrimSpace(delegateID) {
		return DelegationGrant{}, shared.Forbidden("delegation grant is not assigned to the current user")
	}
	if grant.Status != "pending" {
		return DelegationGrant{}, shared.Conflict("delegation grant is not pending")
	}
	grant.Status = "rejected"
	grant.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveDelegationGrant(grant); err != nil {
		return DelegationGrant{}, err
	}
	return grant, nil
}

func (s *Service) RevokeDelegationGrant(grantID, grantorUserID string) (DelegationGrant, error) {
	grant, err := s.requireDelegationGrant(grantID)
	if err != nil {
		return DelegationGrant{}, err
	}
	if grant.GrantorUserID != strings.TrimSpace(grantorUserID) {
		return DelegationGrant{}, shared.Forbidden("delegation grant is not owned by the current user")
	}
	grant = s.normalizeDelegationGrant(grant)
	switch grant.Status {
	case "pending", "accepted":
	default:
		return DelegationGrant{}, shared.Conflict("delegation grant cannot be revoked")
	}
	now := time.Now().UTC()
	grant.Status = "revoked"
	grant.RevokedAt = now
	grant.RevokedByUserID = strings.TrimSpace(grantorUserID)
	grant.UpdatedAt = now
	if err := s.repo.SaveDelegationGrant(grant); err != nil {
		return DelegationGrant{}, err
	}
	return grant, nil
}

func (s *Service) ResolveDelegationGrantForActivation(grantID, delegateUserID, locationID string, now time.Time) (DelegationGrant, error) {
	grant, err := s.requireDelegationGrant(grantID)
	if err != nil {
		return DelegationGrant{}, err
	}
	grant = s.normalizeDelegationGrantAt(grant, now)
	if grant.DelegateKind != "user" || grant.DelegateID != strings.TrimSpace(delegateUserID) {
		return DelegationGrant{}, shared.Forbidden("delegation grant is not assigned to the current user")
	}
	if grant.Status != "accepted" {
		return DelegationGrant{}, shared.Forbidden("delegation grant is not active")
	}
	if locationID != "" && grant.LocationID != strings.TrimSpace(locationID) {
		return DelegationGrant{}, shared.Forbidden("delegation grant is not valid for the current location")
	}
	return grant, nil
}

func (s *Service) ResolveAgentDelegationGrantForActivation(grantID, servicePrincipalID, locationID string, now time.Time) (DelegationGrant, error) {
	grant, err := s.requireDelegationGrant(grantID)
	if err != nil {
		return DelegationGrant{}, err
	}
	grant = s.normalizeDelegationGrantAt(grant, now)
	if grant.DelegateKind != "agent" || grant.DelegateID != strings.TrimSpace(servicePrincipalID) {
		return DelegationGrant{}, shared.Forbidden("delegation grant is not assigned to the current service principal")
	}
	if grant.Status != "accepted" {
		return DelegationGrant{}, shared.Forbidden("delegation grant is not active")
	}
	if locationID != "" && grant.LocationID != strings.TrimSpace(locationID) {
		return DelegationGrant{}, shared.Forbidden("delegation grant is not valid for the current location")
	}
	return grant, nil
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
		Priority:      0,
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

type GoogleProvisioningPolicy struct {
	Enabled           bool
	AllowedDomains    []string
	RoleID            string
	ScopeType         string
	ScopeID           string
	DefaultLocationID string
}

func (s *Service) StartSession(username, locationID, authenticationMethod string, clientMetadata map[string]any, ttl time.Duration) (Session, error) {
	user, ok := s.repo.FindUserByUsername(username)
	if !ok {
		return Session{}, shared.Unauthorized("invalid credentials")
	}
	return s.startSessionForUser(user, locationID, authenticationMethod, clientMetadata, ttl)
}

func (s *Service) AuthenticateGoogle(identity GoogleIdentity, locationID string, clientMetadata map[string]any, ttl time.Duration, provisioning GoogleProvisioningPolicy) (Session, error) {
	subject := "google:" + strings.TrimSpace(identity.Subject)
	if subject == "google:" {
		return Session{}, shared.Validation("google subject is required")
	}
	user, ok := s.repo.FindUserByAuthenticationSubject(subject)
	if !ok {
		user, ok = s.repo.FindUserByUsername(strings.ToLower(strings.TrimSpace(identity.Email)))
		if !ok {
			provisioned, err := s.provisionGoogleUser(identity, subject, provisioning)
			if err != nil {
				return Session{}, err
			}
			user = provisioned
		}
	}
	if user.AuthenticationSubject == "" {
		user.AuthenticationSubject = subject
		user.UpdatedAt = time.Now().UTC()
		if err := s.repo.SaveUser(user); err != nil {
			return Session{}, err
		}
	} else if user.AuthenticationSubject != subject {
		return Session{}, shared.Forbidden("google account does not match linked platform user")
	}
	metadata := cloneMetadata(clientMetadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["google_subject"] = identity.Subject
	metadata["google_email"] = identity.Email
	if identity.HostedDomain != "" {
		metadata["google_hosted_domain"] = identity.HostedDomain
	}
	if identity.Name != "" {
		metadata["google_name"] = identity.Name
	}
	return s.startSessionForUser(user, locationID, "google", metadata, ttl)
}

func (s *Service) provisionGoogleUser(identity GoogleIdentity, subject string, provisioning GoogleProvisioningPolicy) (User, error) {
	if !provisioning.Enabled {
		return User{}, shared.Forbidden("google account is not linked to a platform user")
	}
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	if email == "" {
		return User{}, shared.Forbidden("google account email is required")
	}
	if len(provisioning.AllowedDomains) > 0 {
		domainAllowed := false
		parts := strings.SplitN(email, "@", 2)
		if len(parts) == 2 {
			emailDomain := strings.ToLower(strings.TrimSpace(parts[1]))
			for _, allowed := range provisioning.AllowedDomains {
				if strings.EqualFold(strings.TrimSpace(allowed), emailDomain) {
					domainAllowed = true
					break
				}
			}
		}
		if !domainAllowed {
			return User{}, shared.Forbidden("google account domain is not allowed for auto provisioning")
		}
	}
	roleID := strings.TrimSpace(provisioning.RoleID)
	if roleID == "" {
		return User{}, shared.Validation("google auto provision role id is required")
	}
	if !s.roleExists(roleID) {
		return User{}, shared.Validation("google auto provision role id is invalid")
	}
	scopeType := strings.TrimSpace(provisioning.ScopeType)
	if scopeType == "" {
		scopeType = "deployment"
	}
	now := time.Now().UTC()
	user := User{
		ID:                    fmt.Sprintf("user:%d", now.UnixNano()),
		Username:              email,
		AuthenticationSubject: subject,
		Status:                "active",
		DefaultLocationID:     strings.TrimSpace(provisioning.DefaultLocationID),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := s.repo.SaveUser(user); err != nil {
		return User{}, err
	}
	if err := s.repo.SaveRoleBinding(RoleBinding{
		ID:            fmt.Sprintf("rb:%d", now.UnixNano()),
		UserID:        user.ID,
		RoleID:        roleID,
		ScopeType:     scopeType,
		ScopeID:       strings.TrimSpace(provisioning.ScopeID),
		Priority:      0,
		EffectiveFrom: now,
		Status:        "active",
	}); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Service) startSessionForUser(user User, locationID, authenticationMethod string, clientMetadata map[string]any, ttl time.Duration) (Session, error) {
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
	role.DefaultUserRoute = normalizeRoutePreference(role.DefaultUserRoute)
	role.DefaultAdminRoute = normalizeRoutePreference(role.DefaultAdminRoute)
	now := time.Now().UTC()
	if role.CreatedAt.IsZero() {
		role.CreatedAt = now
	}
	role.UpdatedAt = now
	return s.repo.SaveRole(role)
}

func (s *Service) UpsertUser(user User) error {
	if strings.TrimSpace(user.ID) == "" || strings.TrimSpace(user.Username) == "" {
		return shared.Validation("user id and username are required")
	}
	status := strings.TrimSpace(strings.ToLower(user.Status))
	if status == "" {
		status = "active"
	}
	switch status {
	case "active", "disabled":
	default:
		return shared.Validation("user status must be active or disabled")
	}
	user.Status = status
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	return s.repo.SaveUser(user)
}

func (s *Service) SetRoleDefaultRoutes(roleID, userRoute, adminRoute string) (Role, error) {
	role, ok := s.findRole(roleID)
	if !ok {
		return Role{}, shared.NotFound("role not found")
	}
	role.DefaultUserRoute = normalizeRoutePreference(userRoute)
	role.DefaultAdminRoute = normalizeRoutePreference(adminRoute)
	role.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveRole(role); err != nil {
		return Role{}, err
	}
	return role, nil
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

func (s *Service) UpsertRoleBinding(binding RoleBinding) error {
	if strings.TrimSpace(binding.ID) == "" || strings.TrimSpace(binding.UserID) == "" || strings.TrimSpace(binding.RoleID) == "" {
		return shared.Validation("role binding id, user_id, and role_id are required")
	}
	if _, ok := s.repo.FindUser(binding.UserID); !ok {
		return shared.NotFound("user not found")
	}
	if !s.roleExists(binding.RoleID) {
		return shared.NotFound("role not found")
	}
	scopeType := strings.TrimSpace(binding.ScopeType)
	switch scopeType {
	case "", "deployment":
		scopeType = "deployment"
	case "organization", "location", "operating_unit":
	default:
		return shared.Validation("role binding scope_type is invalid")
	}
	binding.ScopeType = scopeType
	if scopeType != "deployment" && strings.TrimSpace(binding.ScopeID) == "" {
		return shared.Validation("role binding scope_id is required")
	}
	status := strings.TrimSpace(strings.ToLower(binding.Status))
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "inactive" {
		return shared.Validation("role binding status must be active or inactive")
	}
	binding.Status = status
	if binding.EffectiveFrom.IsZero() {
		binding.EffectiveFrom = time.Now().UTC()
	}
	if !binding.EffectiveTo.IsZero() && binding.EffectiveTo.Before(binding.EffectiveFrom) {
		return shared.Validation("role binding effective_to must be after effective_from")
	}
	return s.repo.SaveRoleBinding(binding)
}

func (s *Service) RevokeRolePermission(roleID, permissionKey string) error {
	roleID = strings.TrimSpace(roleID)
	permissionKey = strings.TrimSpace(permissionKey)
	if roleID == "" || permissionKey == "" {
		return shared.Validation("role permission revoke is invalid")
	}
	if !s.roleExists(roleID) {
		return shared.NotFound("role not found")
	}
	if !s.permissionExists(permissionKey) {
		return shared.NotFound("permission not found")
	}
	return s.repo.DeleteRolePermission(roleID, permissionKey)
}

func (s *Service) SetRoleBindingPriority(bindingID string, priority int) (RoleBinding, error) {
	if priority < 0 {
		return RoleBinding{}, shared.Validation("priority must be zero or greater")
	}
	for _, binding := range s.repo.RoleBindings() {
		if binding.ID != bindingID {
			continue
		}
		binding.Priority = priority
		if err := s.repo.SaveRoleBinding(binding); err != nil {
			return RoleBinding{}, err
		}
		return binding, nil
	}
	return RoleBinding{}, shared.NotFound("role binding not found")
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
	return s.DecideActingSession(sessionID, "", permissionKey, locationID, nil)
}

func (s *Service) DecideActingSession(sessionID, effectiveUserID, permissionKey, locationID string, grant *DelegationGrant) Decision {
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
	actingUserID := strings.TrimSpace(effectiveUserID)
	if actingUserID == "" {
		actingUserID = session.UserID
	}
	if grant != nil {
		normalized := s.normalizeDelegationGrant(*grant)
		grant = &normalized
		if strings.TrimSpace(grant.Status) != "accepted" {
			return Decision{Allowed: false, Reason: "delegation grant is not active"}
		}
		if grant.DelegateKind != "user" || grant.DelegateID != session.UserID || grant.GrantorUserID != actingUserID {
			return Decision{Allowed: false, Reason: "delegation grant does not match session"}
		}
		if grant.LocationID != "" && locationID != "" && grant.LocationID != locationID {
			return Decision{Allowed: false, Reason: "delegation grant location mismatch"}
		}
		if !grant.StartsAt.IsZero() && time.Now().UTC().Before(grant.StartsAt) {
			return Decision{Allowed: false, Reason: "delegation grant not active yet"}
		}
		if !grant.ExpiresAt.IsZero() && !time.Now().UTC().Before(grant.ExpiresAt) {
			return Decision{Allowed: false, Reason: "delegation grant expired"}
		}
		if !containsString(grant.AllowedPermissionKeys, permissionKey) {
			return Decision{Allowed: false, Reason: "delegation grant does not allow permission"}
		}
	}
	return s.Decide(actingUserID, permissionKey, locationID)
}

func (s *Service) DecideActingServicePrincipal(principalID, effectiveUserID, permissionKey, locationID string, grant *DelegationGrant) Decision {
	if principalID == "" {
		return Decision{Allowed: false, Reason: "missing service principal"}
	}
	principal, ok := s.repo.FindServicePrincipal(principalID)
	if !ok {
		return Decision{Allowed: false, Reason: "service principal not found"}
	}
	if principal.Status != "active" {
		return Decision{Allowed: false, Reason: "service principal not active"}
	}
	if grant == nil {
		return Decision{Allowed: false, Reason: "delegation grant is required"}
	}
	normalized := s.normalizeDelegationGrant(*grant)
	grant = &normalized
	if strings.TrimSpace(grant.Status) != "accepted" {
		return Decision{Allowed: false, Reason: "delegation grant is not active"}
	}
	actingUserID := strings.TrimSpace(effectiveUserID)
	if actingUserID == "" {
		actingUserID = grant.GrantorUserID
	}
	if grant.DelegateKind != "agent" || grant.DelegateID != principalID || grant.GrantorUserID != actingUserID {
		return Decision{Allowed: false, Reason: "delegation grant does not match service principal"}
	}
	if grant.LocationID != "" && locationID != "" && grant.LocationID != locationID {
		return Decision{Allowed: false, Reason: "delegation grant location mismatch"}
	}
	if !grant.StartsAt.IsZero() && time.Now().UTC().Before(grant.StartsAt) {
		return Decision{Allowed: false, Reason: "delegation grant not active yet"}
	}
	if !grant.ExpiresAt.IsZero() && !time.Now().UTC().Before(grant.ExpiresAt) {
		return Decision{Allowed: false, Reason: "delegation grant expired"}
	}
	if !containsString(grant.AllowedPermissionKeys, permissionKey) {
		return Decision{Allowed: false, Reason: "delegation grant does not allow permission"}
	}
	return s.Decide(actingUserID, permissionKey, grant.LocationID)
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

func normalizeStringList(items []string) []string {
	filtered := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		filtered = append(filtered, item)
	}
	sort.Strings(filtered)
	return filtered
}

func containsString(items []string, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}

func (s *Service) permissionExists(permissionKey string) bool {
	for _, permission := range s.repo.Permissions() {
		if permission.Key == permissionKey {
			return true
		}
	}
	return false
}

func (s *Service) requireDelegationGrant(grantID string) (DelegationGrant, error) {
	grant, ok := s.repo.FindDelegationGrant(strings.TrimSpace(grantID))
	if !ok {
		return DelegationGrant{}, shared.NotFound("delegation grant not found")
	}
	return grant, nil
}

func (s *Service) normalizeDelegationGrant(grant DelegationGrant) DelegationGrant {
	return s.normalizeDelegationGrantAt(grant, time.Now().UTC())
}

func (s *Service) normalizeDelegationGrantAt(grant DelegationGrant, now time.Time) DelegationGrant {
	grant.DelegateKind = strings.ToLower(strings.TrimSpace(grant.DelegateKind))
	if grant.DelegateKind == "" {
		grant.DelegateKind = "user"
	}
	grant.DelegateID = strings.TrimSpace(grant.DelegateID)
	grant.DelegateUserID = strings.TrimSpace(grant.DelegateUserID)
	if grant.DelegateID == "" {
		grant.DelegateID = grant.DelegateUserID
	}
	if grant.DelegateKind == "user" && grant.DelegateUserID == "" {
		grant.DelegateUserID = grant.DelegateID
	}
	grant.AcceptedByKind = strings.ToLower(strings.TrimSpace(grant.AcceptedByKind))
	grant.AcceptedByID = strings.TrimSpace(grant.AcceptedByID)
	grant.AcceptedByUserID = strings.TrimSpace(grant.AcceptedByUserID)
	if grant.AcceptedByKind == "" && grant.AcceptedByUserID != "" {
		grant.AcceptedByKind = "user"
	}
	if grant.AcceptedByID == "" {
		grant.AcceptedByID = grant.AcceptedByUserID
	}
	if grant.AcceptedByKind == "user" && grant.AcceptedByUserID == "" {
		grant.AcceptedByUserID = grant.AcceptedByID
	}
	switch grant.Status {
	case "pending", "accepted":
		if !grant.ExpiresAt.IsZero() && !now.Before(grant.ExpiresAt) {
			grant.Status = "expired"
		}
	}
	return grant
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

func (s *Service) findRole(roleID string) (Role, bool) {
	for _, role := range s.repo.Roles() {
		if role.ID == roleID {
			return role, true
		}
	}
	return Role{}, false
}

func (s *Service) findRoleByKey(roleKey string) (Role, bool) {
	roleKey = strings.TrimSpace(roleKey)
	for _, role := range s.repo.Roles() {
		if role.Key == roleKey {
			return role, true
		}
	}
	return Role{}, false
}

func normalizeRoutePreference(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return ""
	}
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	return route
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

func resolveTime(at time.Time) time.Time {
	if at.IsZero() {
		return time.Now().UTC()
	}
	return at.UTC()
}

func reportingLineRank(kind string) int {
	switch strings.TrimSpace(kind) {
	case "acting_manager":
		return 0
	case "primary_manager":
		return 1
	default:
		return 2
	}
}

func activeReportingLinesForUser(lines []ReportingLine, subjectUserID, organizationID, locationID, operatingUnitID string, at time.Time) []ReportingLine {
	items := make([]ReportingLine, 0)
	for _, line := range lines {
		if line.SubjectUserID != subjectUserID || line.Status != "active" {
			continue
		}
		if line.EffectiveFrom.After(at) {
			continue
		}
		if !line.EffectiveTo.IsZero() && line.EffectiveTo.Before(at) {
			continue
		}
		if !reportingLineMatchesScope(line, organizationID, locationID, operatingUnitID) {
			continue
		}
		items = append(items, line)
	}
	return items
}

func reportingLineMatchesScope(line ReportingLine, organizationID, locationID, operatingUnitID string) bool {
	if line.OrganizationID != "" && organizationID != "" && line.OrganizationID != organizationID {
		return false
	}
	if line.LocationID != "" && locationID != "" && line.LocationID != locationID {
		return false
	}
	if line.OperatingUnitID != "" && operatingUnitID != "" && line.OperatingUnitID != operatingUnitID {
		return false
	}
	return true
}

func bindingMatchesScope(binding RoleBinding, organizationID, locationID, operatingUnitID string) bool {
	switch binding.ScopeType {
	case "", "deployment":
		return true
	case "organization":
		return binding.ScopeID == "" || organizationID == "" || binding.ScopeID == organizationID
	case "location":
		return binding.ScopeID == "" || locationID == "" || binding.ScopeID == locationID
	case "operating_unit":
		return binding.ScopeID == "" || operatingUnitID == "" || binding.ScopeID == operatingUnitID
	default:
		return true
	}
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
