package httpx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/runtimeconfig"
	"orbyte/internal/platform/shared"
)

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	LocationID string `json:"location_id"`
}

type googleLoginRequest struct {
	IDToken    string `json:"id_token"`
	LocationID string `json:"location_id"`
}

type googleTokenResponse struct {
	IDToken string `json:"id_token"`
}

type authOptionsResponse struct {
	PasswordEnabled   bool   `json:"password_enabled"`
	GoogleEnabled     bool   `json:"google_enabled"`
	TOTPEnabled       bool   `json:"totp_enabled"`
	LoginTitle        string `json:"login_title"`
	LoginSubtitle     string `json:"login_subtitle"`
	GoogleButtonLabel string `json:"google_button_label"`
}

const (
	googleOAuthStateCookieName = "orbyte_google_state"
	googleOAuthNextCookieName  = "orbyte_google_next"
	googleOAuthCookieTTL       = 10 * time.Minute
)

type changePasswordRequest struct {
	Username        string `json:"username,omitempty"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type createUserRequest struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	DefaultLocationID string `json:"default_location_id"`
	RoleID            string `json:"role_id"`
	ScopeType         string `json:"scope_type"`
	ScopeID           string `json:"scope_id"`
}

type resetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

type userStatusRequest struct {
	Status string `json:"status"`
}

type userNavigationPreferencesRequest struct {
	PreferredUserRoute  string `json:"preferred_user_route"`
	PreferredAdminRoute string `json:"preferred_admin_route"`
}

type uiPreferencesRequest struct {
	Surface     string         `json:"surface"`
	RoutePath   string         `json:"route_path"`
	ViewKey     string         `json:"view_key"`
	Filters     map[string]any `json:"filters"`
	Columns     []string       `json:"columns"`
	ColumnOrder []string       `json:"column_order"`
	Density     string         `json:"density"`
}

type roleNavigationDefaultsRequest struct {
	DefaultUserRoute  string `json:"default_user_route"`
	DefaultAdminRoute string `json:"default_admin_route"`
}

type roleBindingPriorityRequest struct {
	Priority int `json:"priority"`
}

type servicePrincipalRequest struct {
	ID                    string   `json:"id,omitempty"`
	Key                   string   `json:"key"`
	Status                string   `json:"status,omitempty"`
	AllowedOperationTypes []string `json:"allowed_operation_types,omitempty"`
	CredentialRef         string   `json:"credential_ref,omitempty"`
}

type servicePrincipalStatusRequest struct {
	Status string `json:"status"`
}

type servicePrincipalTokenRequest struct {
	TTLSeconds int `json:"ttl_seconds,omitempty"`
}

type delegationGrantRequest struct {
	DelegateKind          string    `json:"delegate_kind,omitempty"`
	DelegateID            string    `json:"delegate_id,omitempty"`
	DelegateUserID        string    `json:"delegate_user_id"`
	LocationID            string    `json:"location_id"`
	AllowedPermissionKeys []string  `json:"allowed_permission_keys"`
	AllowedDocumentTypes  []string  `json:"allowed_document_types,omitempty"`
	StartsAt              time.Time `json:"starts_at,omitempty"`
	ExpiresAt             time.Time `json:"expires_at"`
	Reason                string    `json:"reason,omitempty"`
}

type delegationActivateRequest struct {
	GrantID string `json:"grant_id"`
}

func registerAuthRoutes(mux *http.ServeMux, cfg *config.Service, ident *identity.Service, auditSvc *audit.Service, uiPrefs *UIPreferencesService) {
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": ident.Users()})
	})

	mux.HandleFunc("GET /users/", func(w http.ResponseWriter, r *http.Request) {
		if userID, ok := userIDPath(r.URL.Path); ok {
			if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
				return
			}
			user, found := ident.FindUser(userID)
			if !found {
				respondError(w, shared.NotFound("user not found"))
				return
			}
			payload := map[string]any{
				"user": user,
			}
			if credential, ok := ident.FindCredentialByUserID(userID); ok {
				payload["credential"] = map[string]any{
					"password_changed_at":  credential.PasswordChangedAt,
					"failed_attempt_count": credential.FailedAttemptCount,
					"locked_until":         credential.LockedUntil,
					"updated_at":           credential.UpdatedAt,
				}
			}
			bindings := make([]identity.RoleBinding, 0)
			for _, binding := range ident.Bindings() {
				if binding.UserID == userID {
					bindings = append(bindings, binding)
				}
			}
			payload["role_bindings"] = bindings
			respondJSON(w, http.StatusOK, payload)
			return
		}
	})

	mux.HandleFunc("GET /sessions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_sessions", "", "identity.manage_sessions"); !ok {
			return
		}
		type sessionItem struct {
			Session identity.Session       `json:"session"`
			Review  identity.SessionReview `json:"review"`
		}
		sessions := ident.Sessions()
		items := make([]sessionItem, 0, len(sessions))
		for _, item := range sessions {
			review, _ := ident.ReviewSession(item.ID)
			items = append(items, sessionItem{Session: item, Review: review})
		}
		if userID := strings.TrimSpace(r.URL.Query().Get("user_id")); userID != "" {
			filtered := make([]sessionItem, 0, len(items))
			for _, item := range items {
				if item.Session.UserID == userID {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("GET /roles", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": ident.Roles()})
	})

	mux.HandleFunc("GET /role-bindings", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": ident.Bindings()})
	})

	mux.HandleFunc("GET /service-principals", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_service_principals", "", "identity.manage_service_principals"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": ident.ServicePrincipals()})
	})

	mux.HandleFunc("GET /service-principals/", func(w http.ResponseWriter, r *http.Request) {
		principalID, action, ok := servicePrincipalPath(r.URL.Path)
		if !ok || action != "" {
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_service_principals", "", "identity.manage_service_principals"); !ok {
			return
		}
		principal, found := ident.FindServicePrincipal(principalID)
		if !found {
			respondError(w, shared.NotFound("service principal not found"))
			return
		}
		respondJSON(w, http.StatusOK, principal)
	})

	mux.HandleFunc("POST /service-principals", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "identity.manage_service_principals", "", "identity.manage_service_principals")
		if !ok {
			return
		}
		var req servicePrincipalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid service principal payload"))
			return
		}
		principal, err := ident.UpsertServicePrincipal(identity.ServicePrincipal{
			ID:                    req.ID,
			Key:                   req.Key,
			Status:                req.Status,
			AllowedOperationTypes: req.AllowedOperationTypes,
			CredentialRef:         req.CredentialRef,
		})
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:identity:service_principal:create:" + principal.ID,
			Action:        "identity.service_principal.create",
			TargetType:    "service_principal",
			TargetID:      principal.ID,
			ActorID:       principalActorID(p),
			ActorKind:     principalActorKind(p),
			OccurredAt:    time.Now().UTC(),
			ChangeSummary: map[string]any{"fields": []string{"key", "status", "allowed_operation_types"}},
			Metadata:      map[string]any{"key": principal.Key, "status": principal.Status},
		})
		respondJSON(w, http.StatusCreated, principal)
	})

	mux.HandleFunc("PUT /service-principals/", func(w http.ResponseWriter, r *http.Request) {
		principalID, action, ok := servicePrincipalPath(r.URL.Path)
		if !ok || action != "status" {
			return
		}
		p, ok := requireAuthorization(w, r, ident, "identity.manage_service_principals", "", "identity.manage_service_principals")
		if !ok {
			return
		}
		var req servicePrincipalStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid service principal status payload"))
			return
		}
		principal, err := ident.SetServicePrincipalStatus(principalID, req.Status)
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:identity:service_principal:status:" + principal.ID + ":" + strings.ToLower(principal.Status),
			Action:        "identity.service_principal.status",
			TargetType:    "service_principal",
			TargetID:      principal.ID,
			ActorID:       principalActorID(p),
			ActorKind:     principalActorKind(p),
			OccurredAt:    time.Now().UTC(),
			ChangeSummary: map[string]any{"fields": []string{"status"}},
			Metadata:      map[string]any{"status": principal.Status},
		})
		respondJSON(w, http.StatusOK, principal)
	})

	mux.HandleFunc("POST /service-principals/", func(w http.ResponseWriter, r *http.Request) {
		principalID, action, ok := servicePrincipalPath(r.URL.Path)
		if !ok || action != "tokens" {
			return
		}
		p, ok := requireAuthorization(w, r, ident, "identity.manage_service_principals", "", "identity.manage_service_principals")
		if !ok {
			return
		}
		var req servicePrincipalTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			respondError(w, shared.Validation("invalid service principal token payload"))
			return
		}
		principal, found := ident.FindServicePrincipal(principalID)
		if !found {
			respondError(w, shared.NotFound("service principal not found"))
			return
		}
		token, err := identity.NewTokenManagerFromEnv().IssueServicePrincipalToken(principal, time.Duration(req.TTLSeconds)*time.Second)
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:         "audit:identity:service_principal:token:" + principal.ID + ":" + time.Now().UTC().Format("20060102150405.000000000"),
			Action:     "identity.service_principal.token.issue",
			TargetType: "service_principal",
			TargetID:   principal.ID,
			ActorID:    principalActorID(p),
			ActorKind:  principalActorKind(p),
			OccurredAt: time.Now().UTC(),
			Metadata:   map[string]any{"ttl_seconds": req.TTLSeconds},
		})
		respondJSON(w, http.StatusOK, map[string]any{"token": token, "service_principal": principal})
	})

	mux.HandleFunc("GET /auth/options", func(w http.ResponseWriter, r *http.Request) {
		policy := cfg.AuthPolicy()
		respondJSON(w, http.StatusOK, authOptionsResponse{
			PasswordEnabled:   policy.PasswordEnabled,
			GoogleEnabled:     policy.GoogleEnabled,
			TOTPEnabled:       policy.TOTPEnabled,
			LoginTitle:        policy.LoginTitle,
			LoginSubtitle:     policy.LoginSubtitle,
			GoogleButtonLabel: policy.GoogleButtonLabel,
		})
	})

	mux.HandleFunc("GET /auth/session", func(w http.ResponseWriter, r *http.Request) {
		p, ok := currentPrincipal(r)
		err := authError(r)
		payload := map[string]any{
			"authenticated": ok && err == nil,
		}
		if err != nil {
			payload["auth_error"] = err.Error()
		}
		if ok && err == nil {
			payload["principal_kind"] = string(p.kind)
			payload["auth_method"] = p.authMethod
			payload["current_location_id"] = p.currentLocationID
			payload["login_step_up_verified"] = p.loginStepUpVerified
			payload["approval_step_up_verified"] = p.stepUpVerified
			payload["approval_step_up_until"] = p.approvalStepUpUntil
			switch p.kind {
			case userPrincipal:
				payload["user_id"] = p.userID
				payload["effective_user_id"] = p.effectiveUserID
			case servicePrincipal:
				payload["service_id"] = p.serviceID
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Vary", "Cookie, Authorization")
		respondJSON(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /auth/login", func(w http.ResponseWriter, r *http.Request) {
		policy := cfg.AuthPolicy()
		if !policy.PasswordEnabled {
			respondError(w, shared.Forbidden("password authentication is disabled"))
			return
		}
		limiter := newLoginRateLimiter(ident, policy.LoginRateLimitAttempts, policy.LoginRateLimitWindow)
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid login payload"))
			return
		}
		req.Username = strings.TrimSpace(req.Username)
		req.Password = strings.TrimSpace(req.Password)
		req.LocationID = strings.TrimSpace(req.LocationID)
		if req.Username == "" {
			respondError(w, shared.Validation("username is required"))
			return
		}
		if req.Password == "" {
			respondError(w, shared.Validation("password is required"))
			return
		}
		limiterKey := loginLimitKey(r, req.Username)
		if !limiter.Allow(limiterKey) {
			respondError(w, shared.Forbidden("login rate limit exceeded"))
			return
		}

		user, err := ident.AuthenticatePasswordPrimary(req.Username, req.Password)
		if err != nil {
			limiter.AddFailure(limiterKey)
			recordAudit(auditSvc, audit.Event{
				ID:            "audit:auth:login:failed:" + time.Now().UTC().Format("20060102150405.000000000"),
				Action:        "auth.login.failed",
				TargetType:    "session",
				TargetID:      "",
				ActorID:       req.Username,
				OccurredAt:    time.Now().UTC(),
				Metadata:      map[string]any{"username": req.Username, "reason": err.Error()},
				CorrelationID: logging.CorrelationID(r.Context()),
			})
			respondError(w, err)
			return
		}
		limiter.Reset(limiterKey)
		passwordExpired, err := ident.PasswordExpired(user.ID, policy.PasswordMaxAge, time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		if passwordExpired {
			respondJSON(w, http.StatusForbidden, map[string]any{
				"status":   "password_change_required",
				"username": user.Username,
				"password_policy": map[string]any{
					"min_length":        policy.PasswordMinLength,
					"require_uppercase": policy.PasswordRequireUppercase,
					"require_number":    policy.PasswordRequireNumber,
					"require_special":   policy.PasswordRequireSpecial,
					"max_age_days":      int(policy.PasswordMaxAge / (24 * time.Hour)),
				},
			})
			return
		}
		loginResult, err := beginInteractiveLogin(w, ident, policy, user, "password", req.LocationID, clientMetadataFromRequest(r))
		if err != nil {
			respondError(w, err)
			return
		}
		if loginResult.RequiresChallenge {
			payload := buildChallengePayload(loginResult.Challenge, loginResult.Enrollment, loginResult.QRURI)
			payload["status"] = "2fa_required"
			if loginResult.Challenge.Purpose == identity.AuthChallengePurposeTOTPEnroll {
				payload["status"] = "2fa_enrollment_required"
			}
			respondJSON(w, http.StatusAccepted, payload)
			return
		}
		session, err := ident.StartSession(user.Username, req.LocationID, "password", clientMetadataFromRequest(r), policy.SessionTTL)
		if err != nil {
			respondError(w, err)
			return
		}
		session.LoginStepUpAt = time.Now().UTC()
		if err := issueAuthenticatedSession(w, ident, session); err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:auth:login:" + session.ID,
			Action:        "auth.login",
			TargetType:    "session",
			TargetID:      session.ID,
			ActorID:       session.UserID,
			OccurredAt:    time.Now().UTC(),
			Metadata:      map[string]any{"authentication_method": session.AuthenticationMethod, "location_id": session.CurrentLocationID},
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{
			"session": map[string]any{
				"id":                  session.ID,
				"user_id":             session.UserID,
				"status":              session.Status,
				"issued_at":           session.IssuedAt,
				"expires_at":          session.ExpiresAt,
				"last_seen_at":        session.LastSeenAt,
				"current_location_id": session.CurrentLocationID,
			},
		})
	})

	mux.HandleFunc("POST /auth/google", func(w http.ResponseWriter, r *http.Request) {
		policy := cfg.AuthPolicy()
		googleVerifier := googleVerifierForPolicy(policy)
		var req googleLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid google login payload"))
			return
		}
		verified, err := googleVerifier.VerifyIDToken(r.Context(), strings.TrimSpace(req.IDToken), identity.GoogleAuthSettings{
			Enabled:      policy.GoogleEnabled,
			ClientID:     policy.GoogleClientID,
			Issuer:       policy.GoogleIssuer,
			JWKSURL:      policy.GoogleJWKSURL,
			HostedDomain: policy.GoogleHostedDomain,
			Timeout:      policy.GoogleTimeout,
		})
		if err != nil {
			recordAudit(auditSvc, audit.Event{
				ID:            "audit:auth:google:failed:" + time.Now().UTC().Format("20060102150405.000000000"),
				Action:        "auth.google.failed",
				TargetType:    "session",
				ActorID:       verified.Email,
				OccurredAt:    time.Now().UTC(),
				Metadata:      map[string]any{"reason": err.Error()},
				CorrelationID: logging.CorrelationID(r.Context()),
			})
			respondError(w, err)
			return
		}
		user, metadata, err := ident.AuthenticateGooglePrimary(verified, clientMetadataFromRequest(r), googleProvisioningPolicy(policy))
		if err != nil {
			recordAudit(auditSvc, audit.Event{
				ID:            "audit:auth:google:failed:" + time.Now().UTC().Format("20060102150405.000000000"),
				Action:        "auth.google.failed",
				TargetType:    "session",
				ActorID:       verified.Email,
				OccurredAt:    time.Now().UTC(),
				Metadata:      map[string]any{"email": verified.Email, "reason": err.Error()},
				CorrelationID: logging.CorrelationID(r.Context()),
			})
			respondError(w, err)
			return
		}
		loginResult, err := beginInteractiveLogin(w, ident, policy, user, "google", strings.TrimSpace(req.LocationID), metadata)
		if err != nil {
			respondError(w, err)
			return
		}
		if loginResult.RequiresChallenge {
			payload := buildChallengePayload(loginResult.Challenge, loginResult.Enrollment, loginResult.QRURI)
			payload["status"] = "2fa_required"
			if loginResult.Challenge.Purpose == identity.AuthChallengePurposeTOTPEnroll {
				payload["status"] = "2fa_enrollment_required"
			}
			respondJSON(w, http.StatusAccepted, payload)
			return
		}
		session, err := ident.StartSession(user.Username, strings.TrimSpace(req.LocationID), "google", metadata, policy.SessionTTL)
		if err != nil {
			respondError(w, err)
			return
		}
		session.LoginStepUpAt = time.Now().UTC()
		if err := issueAuthenticatedSession(w, ident, session); err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:auth:google:" + session.ID,
			Action:        "auth.google",
			TargetType:    "session",
			TargetID:      session.ID,
			ActorID:       session.UserID,
			OccurredAt:    time.Now().UTC(),
			Metadata:      map[string]any{"authentication_method": session.AuthenticationMethod, "location_id": session.CurrentLocationID},
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{"session": map[string]any{
			"id":                  session.ID,
			"user_id":             session.UserID,
			"status":              session.Status,
			"issued_at":           session.IssuedAt,
			"expires_at":          session.ExpiresAt,
			"last_seen_at":        session.LastSeenAt,
			"current_location_id": session.CurrentLocationID,
		}})
	})

	mux.HandleFunc("GET /auth/google/start", func(w http.ResponseWriter, r *http.Request) {
		policy := cfg.AuthPolicy()
		if err := validateGoogleOAuthPolicy(policy); err != nil {
			respondError(w, err)
			return
		}
		state, err := randomOAuthToken(32)
		if err != nil {
			respondError(w, err)
			return
		}
		nextPath := sanitizeRelativeRedirectPath(r.URL.Query().Get("next"))
		expiresAt := time.Now().UTC().Add(googleOAuthCookieTTL)
		http.SetCookie(w, buildGoogleOAuthCookie(googleOAuthStateCookieName, state, expiresAt))
		http.SetCookie(w, buildGoogleOAuthCookie(googleOAuthNextCookieName, nextPath, expiresAt))
		redirectURL, err := buildGoogleAuthorizationURL(policy, state)
		if err != nil {
			respondError(w, err)
			return
		}
		http.Redirect(w, r, redirectURL, http.StatusFound)
	})

	mux.HandleFunc("GET /auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		policy := cfg.AuthPolicy()
		googleVerifier := googleVerifierForPolicy(policy)
		fallbackPath := "/ui?auth_error=google_login_failed"
		nextPath := sanitizeRelativeRedirectPath(cookieValue(r, googleOAuthNextCookieName))
		if nextPath == "" {
			nextPath = "/ui"
		}
		clearGoogleOAuthCookies(w)
		if err := validateGoogleOAuthPolicy(policy); err != nil {
			redirectGoogleOAuthError(w, r, fallbackPath)
			return
		}
		if strings.TrimSpace(r.URL.Query().Get("error")) != "" {
			redirectGoogleOAuthError(w, r, fallbackPath)
			return
		}
		state := strings.TrimSpace(r.URL.Query().Get("state"))
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if state == "" || code == "" || state != cookieValue(r, googleOAuthStateCookieName) {
			redirectGoogleOAuthError(w, r, fallbackPath)
			return
		}
		idToken, err := exchangeGoogleAuthorizationCode(r.Context(), policy, code)
		if err != nil {
			recordAudit(auditSvc, audit.Event{
				ID:            "audit:auth:google:callback_failed:" + time.Now().UTC().Format("20060102150405.000000000"),
				Action:        "auth.google.callback.failed",
				TargetType:    "session",
				OccurredAt:    time.Now().UTC(),
				Metadata:      map[string]any{"reason": err.Error()},
				CorrelationID: logging.CorrelationID(r.Context()),
			})
			redirectGoogleOAuthError(w, r, fallbackPath)
			return
		}
		verified, err := googleVerifier.VerifyIDToken(r.Context(), idToken, identity.GoogleAuthSettings{
			Enabled:      policy.GoogleEnabled,
			ClientID:     policy.GoogleClientID,
			Issuer:       policy.GoogleIssuer,
			JWKSURL:      policy.GoogleJWKSURL,
			HostedDomain: policy.GoogleHostedDomain,
			Timeout:      policy.GoogleTimeout,
		})
		if err != nil {
			recordAudit(auditSvc, audit.Event{
				ID:            "audit:auth:google:callback_failed:" + time.Now().UTC().Format("20060102150405.000000000"),
				Action:        "auth.google.callback.failed",
				TargetType:    "session",
				ActorID:       verified.Email,
				OccurredAt:    time.Now().UTC(),
				Metadata:      map[string]any{"reason": err.Error()},
				CorrelationID: logging.CorrelationID(r.Context()),
			})
			redirectGoogleOAuthError(w, r, fallbackPath)
			return
		}
		user, metadata, err := ident.AuthenticateGooglePrimary(verified, clientMetadataFromRequest(r), googleProvisioningPolicy(policy))
		if err != nil {
			recordAudit(auditSvc, audit.Event{
				ID:            "audit:auth:google:callback_failed:" + time.Now().UTC().Format("20060102150405.000000000"),
				Action:        "auth.google.callback.failed",
				TargetType:    "session",
				ActorID:       verified.Email,
				OccurredAt:    time.Now().UTC(),
				Metadata:      map[string]any{"email": verified.Email, "reason": err.Error()},
				CorrelationID: logging.CorrelationID(r.Context()),
			})
			redirectGoogleOAuthError(w, r, fallbackPath)
			return
		}
		loginResult, err := beginInteractiveLogin(w, ident, policy, user, "google", "", metadata)
		if err != nil {
			redirectGoogleOAuthError(w, r, fallbackPath)
			return
		}
		if loginResult.RequiresChallenge {
			http.Redirect(w, r, "/ui/login?next="+url.QueryEscape(nextPath), http.StatusFound)
			return
		}
		session, err := ident.StartSession(user.Username, "", "google", metadata, policy.SessionTTL)
		if err != nil {
			redirectGoogleOAuthError(w, r, fallbackPath)
			return
		}
		session.LoginStepUpAt = time.Now().UTC()
		if err := issueAuthenticatedSession(w, ident, session); err != nil {
			redirectGoogleOAuthError(w, r, fallbackPath)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:auth:google:callback:" + session.ID,
			Action:        "auth.google.callback",
			TargetType:    "session",
			TargetID:      session.ID,
			ActorID:       session.UserID,
			OccurredAt:    time.Now().UTC(),
			Metadata:      map[string]any{"authentication_method": session.AuthenticationMethod, "location_id": session.CurrentLocationID},
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		http.Redirect(w, r, nextPath, http.StatusFound)
	})

	mux.HandleFunc("POST /auth/password/change", func(w http.ResponseWriter, r *http.Request) {
		policy := cfg.AuthPolicy()
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if ok && p.kind != userPrincipal {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid password change payload"))
			return
		}
		passwordPolicy := identity.PasswordPolicy{
			MinLength:        policy.PasswordMinLength,
			RequireUppercase: policy.PasswordRequireUppercase,
			RequireNumber:    policy.PasswordRequireNumber,
			RequireSpecial:   policy.PasswordRequireSpecial,
		}
		targetUserID := p.userID
		actorID := p.userID
		limiter := newLoginRateLimiter(ident, policy.LoginRateLimitAttempts, policy.LoginRateLimitWindow)
		limiterKey := ""
		if targetUserID == "" {
			username := strings.TrimSpace(req.Username)
			if username == "" {
				respondError(w, shared.Unauthorized("authentication required"))
				return
			}
			limiterKey = loginLimitKey(r, username)
			if !limiter.Allow(limiterKey) {
				recordAudit(auditSvc, audit.Event{
					ID:            "audit:auth:password_change:rate_limited:" + time.Now().UTC().Format("20060102150405.000000000"),
					Action:        "auth.password.change.rate_limited",
					TargetType:    "session",
					ActorID:       username,
					OccurredAt:    time.Now().UTC(),
					Metadata:      map[string]any{"username": username},
					CorrelationID: logging.CorrelationID(r.Context()),
				})
				respondError(w, shared.Forbidden("login rate limit exceeded"))
				return
			}
			user, ok := ident.FindUserByUsername(username)
			if !ok {
				limiter.AddFailure(limiterKey)
				recordAudit(auditSvc, audit.Event{
					ID:            "audit:auth:password_change:failed:" + time.Now().UTC().Format("20060102150405.000000000"),
					Action:        "auth.password.change.failed",
					TargetType:    "session",
					ActorID:       username,
					OccurredAt:    time.Now().UTC(),
					Metadata:      map[string]any{"username": username, "reason": "invalid credentials"},
					CorrelationID: logging.CorrelationID(r.Context()),
				})
				respondError(w, shared.Unauthorized("invalid credentials"))
				return
			}
			targetUserID = user.ID
			actorID = user.ID
		}
		if err := ident.ChangePasswordUsingPolicy(targetUserID, req.CurrentPassword, req.NewPassword, passwordPolicy); err != nil {
			if limiterKey != "" {
				limiter.AddFailure(limiterKey)
				recordAudit(auditSvc, audit.Event{
					ID:            "audit:auth:password_change:failed:" + time.Now().UTC().Format("20060102150405.000000000"),
					Action:        "auth.password.change.failed",
					TargetType:    "session",
					ActorID:       strings.TrimSpace(req.Username),
					OccurredAt:    time.Now().UTC(),
					Metadata:      map[string]any{"username": strings.TrimSpace(req.Username), "reason": err.Error()},
					CorrelationID: logging.CorrelationID(r.Context()),
				})
			}
			respondError(w, err)
			return
		}
		if limiterKey != "" {
			limiter.Reset(limiterKey)
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:auth:password_change:" + targetUserID,
			Action:        "auth.password.change",
			TargetType:    "user",
			TargetID:      targetUserID,
			ActorID:       actorID,
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{"status": "password_changed"})
	})

	mux.HandleFunc("POST /auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		policy := cfg.AuthPolicy()
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.sessionID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		currentSession, ok := ident.FindSession(p.sessionID)
		if !ok {
			respondError(w, shared.NotFound("session not found"))
			return
		}
		if policy.SessionRefreshWindow > 0 && time.Until(currentSession.ExpiresAt) > policy.SessionRefreshWindow {
			respondError(w, shared.Conflict("session is not within refresh window"))
			return
		}
		session, err := ident.RefreshSession(p.sessionID, policy.SessionTTL)
		if err != nil {
			respondError(w, err)
			return
		}
		tokenManager := identity.NewTokenManagerFromEnv()
		token, err := tokenManager.IssueSessionToken(session)
		if err != nil {
			respondError(w, err)
			return
		}
		csrfCookie, err := buildCSRFCookie(session.ID)
		if err != nil {
			respondError(w, err)
			return
		}
		http.SetCookie(w, buildSessionCookie(token, session.ExpiresAt))
		http.SetCookie(w, csrfCookie)
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:auth:refresh:" + session.ID,
			Action:        "auth.refresh",
			TargetType:    "session",
			TargetID:      session.ID,
			ActorID:       session.UserID,
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{"session": session})
	})

	mux.HandleFunc("POST /auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.sessionID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		session, err := ident.RevokeSession(p.sessionID, time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		http.SetCookie(w, clearedSessionCookie())
		http.SetCookie(w, clearedCSRFCookie())
		http.SetCookie(w, clearedDelegationCookie())
		http.SetCookie(w, clearedDeepLinkCookie())
		http.SetCookie(w, clearedDeepLinkStepUpCookie())
		http.SetCookie(w, clearedAuthChallengeCookie())
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:auth:logout:" + session.ID,
			Action:        "auth.logout",
			TargetType:    "session",
			TargetID:      session.ID,
			ActorID:       session.UserID,
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{"revoked_session_id": session.ID})
	})

	mux.HandleFunc("GET /auth/context", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		payload := map[string]any{
			"actor_user_id":       p.userID,
			"effective_user_id":   principalEffectiveUserID(p),
			"location_id":         p.currentLocationID,
			"delegation_active":   principalHasDelegation(p),
			"delegation_grant_id": principalDelegationGrantID(p),
		}
		if principalHasDelegation(p) {
			payload["delegation"] = p.delegation
		}
		respondJSON(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /auth/delegation/activate", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" || p.sessionID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req delegationActivateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid delegation activate payload"))
			return
		}
		grant, err := ident.ResolveDelegationGrantForActivation(req.GrantID, p.userID, p.currentLocationID, time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		token, err := identity.NewTokenManagerFromEnv().IssueDelegationToken(grant)
		if err != nil {
			respondError(w, err)
			return
		}
		http.SetCookie(w, buildDelegationCookie(token, grant.ExpiresAt))
		recordAudit(auditSvc, audit.Event{
			ID:                "audit:delegation:activate:" + grant.ID + ":" + p.userID,
			Action:            "delegation.activate",
			TargetType:        "delegation_grant",
			TargetID:          grant.ID,
			ActorID:           p.userID,
			ActorKind:         "user",
			OnBehalfOfUserID:  grant.GrantorUserID,
			DelegationGrantID: grant.ID,
			LocationID:        grant.LocationID,
			OccurredAt:        time.Now().UTC(),
			CorrelationID:     logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{"grant": grant, "delegation_active": true})
	})

	mux.HandleFunc("POST /auth/delegation/exit", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		http.SetCookie(w, clearedDelegationCookie())
		recordAudit(auditSvc, audit.Event{
			ID:                "audit:delegation:exit:" + principalDelegationGrantID(p) + ":" + p.userID + ":" + time.Now().UTC().Format("20060102150405.000000000"),
			Action:            "delegation.exit",
			TargetType:        "delegation_grant",
			TargetID:          principalDelegationGrantID(p),
			ActorID:           p.userID,
			ActorKind:         "user",
			OnBehalfOfUserID:  principalOnBehalfOfUserID(p),
			DelegationGrantID: principalDelegationGrantID(p),
			OccurredAt:        time.Now().UTC(),
			CorrelationID:     logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{"delegation_active": false})
	})

	mux.HandleFunc("GET /me/delegations/outgoing", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": ident.ListOutgoingDelegationGrants(p.userID)})
	})

	mux.HandleFunc("POST /me/delegations/outgoing", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req delegationGrantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid delegation grant payload"))
			return
		}
		delegateKind := strings.ToLower(strings.TrimSpace(req.DelegateKind))
		delegateID := strings.TrimSpace(req.DelegateID)
		if delegateKind == "" && strings.TrimSpace(req.DelegateUserID) != "" {
			delegateKind = "user"
			delegateID = strings.TrimSpace(req.DelegateUserID)
		}
		var (
			grant identity.DelegationGrant
			err   error
		)
		switch delegateKind {
		case "", "user":
			if delegateID == "" {
				delegateID = strings.TrimSpace(req.DelegateUserID)
			}
			grant, err = ident.CreateDelegationGrant(p.userID, delegateID, req.LocationID, req.AllowedPermissionKeys, req.AllowedDocumentTypes, req.StartsAt, req.ExpiresAt, req.Reason)
		case "agent":
			grant, err = ident.CreateAgentDelegationGrant(p.userID, delegateID, req.LocationID, req.AllowedPermissionKeys, req.AllowedDocumentTypes, req.StartsAt, req.ExpiresAt, req.Reason)
		default:
			respondError(w, shared.Validation("delegate_kind must be user or agent"))
			return
		}
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:delegation:create:" + grant.ID,
			Action:        "delegation.grant.create",
			TargetType:    "delegation_grant",
			TargetID:      grant.ID,
			ActorID:       p.userID,
			ActorKind:     "user",
			LocationID:    grant.LocationID,
			OccurredAt:    time.Now().UTC(),
			ChangeSummary: map[string]any{"fields": []string{"delegate_kind", "delegate_id", "allowed_permission_keys", "allowed_document_types", "starts_at", "expires_at"}},
			Metadata:      map[string]any{"delegate_kind": grant.DelegateKind, "delegate_id": grant.DelegateID, "delegate_user_id": grant.DelegateUserID, "grantor_user_id": grant.GrantorUserID},
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusCreated, grant)
	})

	mux.HandleFunc("POST /me/delegations/outgoing/", func(w http.ResponseWriter, r *http.Request) {
		grantID, action, ok := delegationOutgoingActionPath(r.URL.Path)
		if !ok || action != "revoke" {
			http.NotFound(w, r)
			return
		}
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		grant, err := ident.RevokeDelegationGrant(grantID, p.userID)
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:delegation:revoke:" + grant.ID,
			Action:        "delegation.grant.revoke",
			TargetType:    "delegation_grant",
			TargetID:      grant.ID,
			ActorID:       p.userID,
			ActorKind:     "user",
			LocationID:    grant.LocationID,
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, grant)
	})

	mux.HandleFunc("GET /me/delegations/incoming", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": ident.ListIncomingDelegationGrants(p.userID)})
	})

	mux.HandleFunc("GET /service-principals/me/delegations/incoming", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != servicePrincipal || p.serviceID == "" {
			respondError(w, shared.Unauthorized("service principal authentication required"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": ident.ListIncomingAgentDelegationGrants(p.serviceID)})
	})

	mux.HandleFunc("POST /me/delegations/incoming/", func(w http.ResponseWriter, r *http.Request) {
		grantID, action, ok := delegationIncomingActionPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var (
			grant identity.DelegationGrant
			err   error
		)
		switch action {
		case "accept":
			grant, err = ident.AcceptDelegationGrant(grantID, p.userID)
		case "reject":
			grant, err = ident.RejectDelegationGrant(grantID, p.userID)
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:delegation:" + action + ":" + grant.ID,
			Action:        "delegation.grant." + action,
			TargetType:    "delegation_grant",
			TargetID:      grant.ID,
			ActorID:       p.userID,
			ActorKind:     "user",
			LocationID:    grant.LocationID,
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, grant)
	})

	mux.HandleFunc("POST /service-principals/me/delegations/incoming/", func(w http.ResponseWriter, r *http.Request) {
		grantID, action, ok := delegationServicePrincipalIncomingActionPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != servicePrincipal || p.serviceID == "" {
			respondError(w, shared.Unauthorized("service principal authentication required"))
			return
		}
		var (
			grant identity.DelegationGrant
			err   error
		)
		switch action {
		case "accept":
			grant, err = ident.AcceptAgentDelegationGrant(grantID, p.serviceID)
		case "reject":
			grant, err = ident.RejectAgentDelegationGrant(grantID, p.serviceID)
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:delegation:" + action + ":" + grant.ID + ":" + p.serviceID,
			Action:        "delegation.grant." + action,
			TargetType:    "delegation_grant",
			TargetID:      grant.ID,
			ActorID:       p.serviceID,
			ActorKind:     "service",
			LocationID:    grant.LocationID,
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, grant)
	})

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		policy := cfg.AuthPolicy()
		p, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users")
		if !ok {
			return
		}
		var req createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid user create payload"))
			return
		}
		user, err := ident.CreateUserWithPolicy(req.Username, req.Password, strings.TrimSpace(req.DefaultLocationID), strings.TrimSpace(req.RoleID), strings.TrimSpace(req.ScopeType), strings.TrimSpace(req.ScopeID), identity.PasswordPolicy{
			MinLength:        policy.PasswordMinLength,
			RequireUppercase: policy.PasswordRequireUppercase,
			RequireNumber:    policy.PasswordRequireNumber,
			RequireSpecial:   policy.PasswordRequireSpecial,
		})
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:identity:user_create:" + user.ID,
			Action:        "identity.user.create",
			TargetType:    "user",
			TargetID:      user.ID,
			ActorID:       principalActorID(p),
			OccurredAt:    time.Now().UTC(),
			Metadata:      map[string]any{"username": user.Username, "default_location_id": user.DefaultLocationID},
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusCreated, user)
	})

	mux.HandleFunc("GET /me/preferences/navigation", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		user, found := ident.FindUser(p.userID)
		if !found {
			respondError(w, shared.NotFound("user not found"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"preferred_user_route":  user.PreferredUserRoute,
			"preferred_admin_route": user.PreferredAdminRoute,
		})
	})

	mux.HandleFunc("PUT /me/preferences/navigation", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req userNavigationPreferencesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid navigation preference payload"))
			return
		}
		user, err := ident.SetUserPreferredRoutes(p.userID, req.PreferredUserRoute, req.PreferredAdminRoute)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, user)
	})

	mux.HandleFunc("GET /me/preferences/ui", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		surface := strings.TrimSpace(r.URL.Query().Get("surface"))
		routePath := normalizeUIRoutePath(r.URL.Query().Get("route_path"))
		if surface == "" || routePath == "" {
			respondError(w, shared.Validation("surface and route_path are required"))
			return
		}
		respondJSON(w, http.StatusOK, uiPrefs.Get(p.userID, surface, routePath))
	})

	mux.HandleFunc("PUT /me/preferences/ui", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req uiPreferencesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid ui preference payload"))
			return
		}
		if strings.TrimSpace(req.Surface) == "" || normalizeUIRoutePath(req.RoutePath) == "" {
			respondError(w, shared.Validation("surface and route_path are required"))
			return
		}
		respondJSON(w, http.StatusOK, uiPrefs.Put(p.userID, UIPreferences{
			Surface:     req.Surface,
			RoutePath:   req.RoutePath,
			ViewKey:     req.ViewKey,
			Filters:     req.Filters,
			Columns:     req.Columns,
			ColumnOrder: req.ColumnOrder,
			Density:     req.Density,
		}))
	})

	mux.HandleFunc("POST /sessions/", func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := sessionRevokePath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		p, ok := requireAuthorization(w, r, ident, "identity.manage_sessions", "", "identity.manage_sessions")
		if !ok {
			return
		}
		session, err := ident.RevokeSession(sessionID, time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:auth:revoke:" + session.ID,
			Action:        "auth.session.revoke",
			TargetType:    "session",
			TargetID:      session.ID,
			ActorID:       principalActorID(p),
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{"session": session})
	})

	mux.HandleFunc("POST /users/", func(w http.ResponseWriter, r *http.Request) {
		if userID, ok := userSetStatusPath(r.URL.Path); ok {
			p, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users")
			if !ok {
				return
			}
			var req userStatusRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid user status payload"))
				return
			}
			user, err := ident.SetUserStatus(userID, req.Status)
			if err != nil {
				respondError(w, err)
				return
			}
			recordAudit(auditSvc, audit.Event{
				ID:            "audit:identity:user_status:" + user.ID + ":" + strings.ToLower(req.Status),
				Action:        "identity.user.status",
				TargetType:    "user",
				TargetID:      user.ID,
				ActorID:       principalActorID(p),
				OccurredAt:    time.Now().UTC(),
				Metadata:      map[string]any{"status": user.Status},
				CorrelationID: logging.CorrelationID(r.Context()),
			})
			respondJSON(w, http.StatusOK, user)
			return
		}
		if userID, ok := userResetTwoFactorPath(r.URL.Path); ok {
			p, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users")
			if !ok {
				return
			}
			if err := ident.DisableTOTP(userID, time.Now().UTC()); err != nil {
				respondError(w, err)
				return
			}
			recordAudit(auditSvc, audit.Event{
				ID:            "audit:identity:user_reset_2fa:" + userID,
				Action:        "identity.user.reset_2fa",
				TargetType:    "user",
				TargetID:      userID,
				ActorID:       principalActorID(p),
				OccurredAt:    time.Now().UTC(),
				CorrelationID: logging.CorrelationID(r.Context()),
			})
			respondJSON(w, http.StatusOK, map[string]any{"status": "2fa_reset"})
			return
		}
		userID, ok := userResetPasswordPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		policy := cfg.AuthPolicy()
		p, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users")
		if !ok {
			return
		}
		var req resetPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid password reset payload"))
			return
		}
		if err := ident.ResetPasswordUsingPolicy(userID, req.NewPassword, identity.PasswordPolicy{
			MinLength:        policy.PasswordMinLength,
			RequireUppercase: policy.PasswordRequireUppercase,
			RequireNumber:    policy.PasswordRequireNumber,
			RequireSpecial:   policy.PasswordRequireSpecial,
		}); err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:identity:user_reset_password:" + userID,
			Action:        "identity.user.reset_password",
			TargetType:    "user",
			TargetID:      userID,
			ActorID:       principalActorID(p),
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{"status": "password_reset"})
	})

	mux.HandleFunc("PUT /users/", func(w http.ResponseWriter, r *http.Request) {
		if userID, ok := userNavigationPreferencesPath(r.URL.Path); ok {
			if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
				return
			}
			var req userNavigationPreferencesRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid navigation preference payload"))
				return
			}
			user, err := ident.SetUserPreferredRoutes(userID, req.PreferredUserRoute, req.PreferredAdminRoute)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, user)
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("PUT /roles/", func(w http.ResponseWriter, r *http.Request) {
		if roleID, ok := roleNavigationDefaultsPath(r.URL.Path); ok {
			if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
				return
			}
			var req roleNavigationDefaultsRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid role navigation defaults payload"))
				return
			}
			role, err := ident.SetRoleDefaultRoutes(roleID, req.DefaultUserRoute, req.DefaultAdminRoute)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, role)
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("PUT /role-bindings/", func(w http.ResponseWriter, r *http.Request) {
		bindingID, ok := roleBindingPriorityPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		var req roleBindingPriorityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid role binding priority payload"))
			return
		}
		binding, err := ident.SetRoleBindingPriority(bindingID, req.Priority)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, binding)
	})

	mux.HandleFunc("GET /sessions/", func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := sessionIDPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_sessions", "", "identity.manage_sessions"); !ok {
			return
		}
		session, found := ident.FindSession(sessionID)
		if !found {
			respondError(w, shared.NotFound("session not found"))
			return
		}
		review, _ := ident.ReviewSession(sessionID)
		respondJSON(w, http.StatusOK, map[string]any{"session": session, "review": review})
	})

	registerTwoFactorAuthRoutes(mux, cfg, ident, auditSvc)
}

func servicePrincipalPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 2 && parts[0] == "service-principals" {
		return strings.TrimSpace(parts[1]), "", parts[1] != ""
	}
	if len(parts) == 3 && parts[0] == "service-principals" {
		return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]), parts[1] != "" && parts[2] != ""
	}
	return "", "", false
}

func userNavigationPreferencesPath(path string) (string, bool) {
	if !strings.HasPrefix(path, "/users/") || !strings.HasSuffix(path, "/preferences/navigation") {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/preferences/navigation")
	id = strings.Trim(id, "/")
	return id, id != ""
}

func userResetTwoFactorPath(path string) (string, bool) {
	if !strings.HasPrefix(path, "/users/") || !strings.HasSuffix(path, "/actions/reset-2fa") {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/actions/reset-2fa")
	id = strings.Trim(id, "/")
	return id, id != ""
}

func roleNavigationDefaultsPath(path string) (string, bool) {
	if !strings.HasPrefix(path, "/roles/") || !strings.HasSuffix(path, "/defaults/navigation") {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/roles/"), "/defaults/navigation")
	id = strings.Trim(id, "/")
	return id, id != ""
}

func roleBindingPriorityPath(path string) (string, bool) {
	if !strings.HasPrefix(path, "/role-bindings/") || !strings.HasSuffix(path, "/priority") {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/role-bindings/"), "/priority")
	id = strings.Trim(id, "/")
	return id, id != ""
}

func validateGoogleOAuthPolicy(policy config.AuthPolicy) error {
	if !policy.GoogleEnabled {
		return shared.Forbidden("google authentication is not enabled")
	}
	if strings.TrimSpace(policy.GoogleClientID) == "" {
		return shared.Validation("google client id is not configured")
	}
	if strings.TrimSpace(policy.GoogleClientSecret) == "" {
		return shared.Validation("google client secret is not configured")
	}
	if strings.TrimSpace(policy.GoogleRedirectURL) == "" {
		return shared.Validation("google redirect url is not configured")
	}
	if strings.TrimSpace(policy.GoogleAuthURL) == "" {
		return shared.Validation("google auth url is not configured")
	}
	if strings.TrimSpace(policy.GoogleTokenURL) == "" {
		return shared.Validation("google token url is not configured")
	}
	if strings.TrimSpace(policy.GoogleJWKSURL) == "" {
		return shared.Validation("google jwks url is not configured")
	}
	return nil
}

func googleVerifierForPolicy(policy config.AuthPolicy) identity.GoogleVerifier {
	return identity.OIDCGoogleVerifier{HTTPClient: &http.Client{Timeout: policy.GoogleTimeout}}
}

func googleProvisioningPolicy(policy config.AuthPolicy) identity.GoogleProvisioningPolicy {
	return identity.GoogleProvisioningPolicy{
		Enabled:           policy.GoogleAutoProvisionEnabled,
		AllowedDomains:    policy.GoogleAutoProvisionAllowedDomains,
		RoleID:            policy.GoogleAutoProvisionRoleID,
		ScopeType:         policy.GoogleAutoProvisionScopeType,
		ScopeID:           policy.GoogleAutoProvisionScopeID,
		DefaultLocationID: policy.GoogleAutoProvisionDefaultLocationID,
	}
}

func buildGoogleAuthorizationURL(policy config.AuthPolicy, state string) (string, error) {
	authURL, err := url.Parse(policy.GoogleAuthURL)
	if err != nil {
		return "", shared.Validation("google auth url is invalid")
	}
	query := authURL.Query()
	query.Set("client_id", policy.GoogleClientID)
	query.Set("redirect_uri", policy.GoogleRedirectURL)
	query.Set("response_type", "code")
	query.Set("scope", "openid email profile")
	query.Set("state", state)
	query.Set("access_type", "offline")
	query.Set("prompt", "select_account")
	if strings.TrimSpace(policy.GoogleHostedDomain) != "" {
		query.Set("hd", policy.GoogleHostedDomain)
	}
	authURL.RawQuery = query.Encode()
	return authURL.String(), nil
}

func exchangeGoogleAuthorizationCode(ctx context.Context, policy config.AuthPolicy, code string) (string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", policy.GoogleClientID)
	form.Set("client_secret", policy.GoogleClientSecret)
	form.Set("redirect_uri", policy.GoogleRedirectURL)
	form.Set("grant_type", "authorization_code")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, policy.GoogleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: policy.GoogleTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", shared.Unauthorized("google token exchange failed: " + strings.TrimSpace(string(body)))
	}
	var payload googleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.IDToken) == "" {
		return "", shared.Unauthorized("google token exchange did not return an id token")
	}
	return payload.IDToken, nil
}

func randomOAuthToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func buildGoogleOAuthCookie(name, value string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   runtimeconfig.Current().CookieSecure(),
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	}
}

func clearGoogleOAuthCookies(w http.ResponseWriter) {
	secure := runtimeconfig.Current().CookieSecure()
	http.SetCookie(w, &http.Cookie{Name: googleOAuthStateCookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure, Expires: time.Unix(0, 0), MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: googleOAuthNextCookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure, Expires: time.Unix(0, 0), MaxAge: -1})
}

func cookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func sanitizeRelativeRedirectPath(next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return "/ui"
	}
	parsed, err := url.Parse(next)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/ui"
	}
	return next
}

func redirectGoogleOAuthError(w http.ResponseWriter, r *http.Request, location string) {
	clearGoogleOAuthCookies(w)
	http.Redirect(w, r, location, http.StatusFound)
}

func buildSessionCookie(token string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   runtimeconfig.Current().CookieSecure(),
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt.UTC(),
	}
}

func buildDelegationCookie(token string, expiresAt time.Time) *http.Cookie {
	secure := buildSessionCookie("", time.Now().UTC()).Secure
	return &http.Cookie{
		Name:     delegationCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt.UTC(),
	}
}

func buildDeepLinkCookie(token string, expiresAt time.Time) *http.Cookie {
	secure := buildSessionCookie("", time.Now().UTC()).Secure
	return &http.Cookie{
		Name:     deepLinkCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt.UTC(),
	}
}

func buildDeepLinkStepUpCookie(token string, expiresAt time.Time) *http.Cookie {
	secure := buildSessionCookie("", time.Now().UTC()).Secure
	return &http.Cookie{
		Name:     deepLinkStepUpCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt.UTC(),
	}
}

func clearedSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   buildSessionCookie("", time.Now().UTC()).Secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
	}
}

func clearedDelegationCookie() *http.Cookie {
	return &http.Cookie{
		Name:     delegationCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   buildSessionCookie("", time.Now().UTC()).Secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
	}
}

func clearedDeepLinkCookie() *http.Cookie {
	return &http.Cookie{
		Name:     deepLinkCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   buildSessionCookie("", time.Now().UTC()).Secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
	}
}

func clearedDeepLinkStepUpCookie() *http.Cookie {
	return &http.Cookie{
		Name:     deepLinkStepUpCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   buildSessionCookie("", time.Now().UTC()).Secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
	}
}

func sessionRevokePath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "sessions" || parts[2] != "actions" || parts[3] != "revoke" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), parts[1] != ""
}

func userResetPasswordPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "users" || parts[2] != "actions" || parts[3] != "reset-password" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), parts[1] != ""
}

func userSetStatusPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "users" || parts[2] != "actions" || parts[3] != "set-status" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), parts[1] != ""
}

func userIDPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] != "users" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), parts[1] != ""
}

func sessionIDPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] != "sessions" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), parts[1] != ""
}

func delegationOutgoingActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "me" || parts[1] != "delegations" || parts[2] != "outgoing" {
		return "", "", false
	}
	return strings.TrimSpace(parts[3]), strings.TrimSpace(parts[4]), parts[3] != "" && parts[4] != ""
}

func delegationIncomingActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "me" || parts[1] != "delegations" || parts[2] != "incoming" {
		return "", "", false
	}
	return strings.TrimSpace(parts[3]), strings.TrimSpace(parts[4]), parts[3] != "" && parts[4] != ""
}

func delegationServicePrincipalIncomingActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "service-principals" || parts[1] != "me" || parts[2] != "delegations" || parts[3] != "incoming" {
		return "", "", false
	}
	return strings.TrimSpace(parts[4]), strings.TrimSpace(parts[5]), parts[4] != "" && parts[5] != ""
}

func clientMetadataFromRequest(r *http.Request) map[string]any {
	metadata := map[string]any{}
	if userAgent := strings.TrimSpace(r.UserAgent()); userAgent != "" {
		metadata["user_agent"] = userAgent
	}
	if remoteAddr := strings.TrimSpace(r.RemoteAddr); remoteAddr != "" {
		metadata["remote_addr"] = remoteAddr
	}
	return metadata
}

func principalActorID(p principal) string {
	if p.kind == servicePrincipal {
		return p.serviceID
	}
	return p.userID
}

func principalActorKind(p principal) string {
	if p.kind == servicePrincipal {
		return "service"
	}
	return "user"
}

func principalAuditEvent(p principal, event audit.Event) audit.Event {
	event.ActorID = principalActorID(p)
	if event.ActorKind == "" {
		event.ActorKind = principalActorKind(p)
	}
	if event.OnBehalfOfUserID == "" {
		event.OnBehalfOfUserID = principalOnBehalfOfUserID(p)
	}
	if event.DelegationGrantID == "" {
		event.DelegationGrantID = principalDelegationGrantID(p)
	}
	return event
}

func loginLimitKey(r *http.Request, username string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "|" + clientIP(r)
}

func clientIP(r *http.Request) string {
	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func recordAudit(auditSvc *audit.Service, event audit.Event) {
	if auditSvc == nil {
		return
	}
	if event.ActorKind == "" {
		switch {
		case strings.HasPrefix(event.ActorID, "sp"), strings.HasPrefix(event.ActorID, "projection_worker"):
			event.ActorKind = "service"
		default:
			event.ActorKind = "user"
		}
	}
	_ = auditSvc.Record(event)
}
