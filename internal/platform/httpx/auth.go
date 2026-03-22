package httpx

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/shared"
)

const sessionCookieName = "orbyte_session"
const csrfCookieName = "orbyte_csrf"
const delegationCookieName = "orbyte_delegation"

type principalKind string

const (
	userPrincipal    principalKind = "user"
	servicePrincipal principalKind = "service"
)

type principal struct {
	kind              principalKind
	userID            string
	effectiveUserID   string
	sessionID         string
	currentLocationID string
	serviceID         string
	authMethod        string
	stepUpVerified    bool
	delegationGrantID string
	onBehalfOfUserID  string
	delegation        *identity.DelegationGrant
}

type authContextKey string

const (
	principalContextKey authContextKey = "principal"
	authErrorContextKey authContextKey = "auth_error"
)

func withAuthentication(next http.Handler, ident *identity.Service) http.Handler {
	tokenManager := identity.NewTokenManagerFromEnv()
	devBypass := os.Getenv("APP_AUTH_DEV_BYPASS") == "true"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, authErr := authenticateRequest(r, w, ident, tokenManager, devBypass)
		ctx := r.Context()
		if p != nil {
			if p.kind == userPrincipal && p.sessionID != "" {
				if _, err := ident.TouchSession(p.sessionID, time.Now().UTC()); err != nil {
					authErr = shared.Unauthorized(err.Error())
					p = nil
				}
			}
		}
		if p != nil {
			ctx = context.WithValue(ctx, principalContextKey, *p)
		}
		if authErr != nil {
			ctx = context.WithValue(ctx, authErrorContextKey, authErr)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func authenticateRequest(r *http.Request, w http.ResponseWriter, ident *identity.Service, tokenManager *identity.TokenManager, devBypass bool) (*principal, error) {
	if devBypass {
		if userID := strings.TrimSpace(r.Header.Get("X-Dev-User-ID")); userID != "" {
			for _, session := range ident.Sessions() {
				if session.UserID == userID && session.Status == "active" {
					return &principal{
						kind:              userPrincipal,
						userID:            userID,
						effectiveUserID:   userID,
						sessionID:         session.ID,
						currentLocationID: session.CurrentLocationID,
						authMethod:        "dev_bypass",
						stepUpVerified:    true,
					}, nil
				}
			}
			return nil, shared.Unauthorized("development user has no active session")
		}
	}

	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		claims, parseErr := tokenManager.Parse(cookie.Value)
		if parseErr != nil {
			return nil, shared.Unauthorized(parseErr.Error())
		}
		if claims.Kind != "session" || claims.SessionID == "" {
			return nil, shared.Unauthorized("invalid session token")
		}
		session, ok := ident.FindSession(claims.SessionID)
		if !ok || session.UserID != claims.Subject {
			return nil, shared.Unauthorized("session not found")
		}
		if err := validateAuthenticatedSession(session); err != nil {
			return nil, err
		}
		p := &principal{
			kind:              userPrincipal,
			userID:            session.UserID,
			effectiveUserID:   session.UserID,
			sessionID:         session.ID,
			currentLocationID: session.CurrentLocationID,
			authMethod:        "cookie",
			stepUpVerified:    stepUpVerified(r),
		}
		return resolveDelegationPrincipal(r, w, ident, tokenManager, session, p)
	}

	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authz, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
		claims, parseErr := tokenManager.Parse(token)
		if parseErr != nil {
			return nil, shared.Unauthorized(parseErr.Error())
		}
		switch claims.Kind {
		case "session":
			if claims.SessionID == "" {
				return nil, shared.Unauthorized("invalid session token")
			}
			session, ok := ident.FindSession(claims.SessionID)
			if !ok || session.UserID != claims.Subject {
				return nil, shared.Unauthorized("session not found")
			}
			if err := validateAuthenticatedSession(session); err != nil {
				return nil, err
			}
			p := &principal{
				kind:              userPrincipal,
				userID:            session.UserID,
				effectiveUserID:   session.UserID,
				sessionID:         session.ID,
				currentLocationID: session.CurrentLocationID,
				authMethod:        "bearer",
				stepUpVerified:    stepUpVerified(r),
			}
			return resolveDelegationPrincipal(r, w, ident, tokenManager, session, p)
		case "service":
			if claims.ServicePrincipal == "" {
				return nil, shared.Unauthorized("invalid service principal token")
			}
			serviceAccount, ok := ident.FindServicePrincipal(claims.ServicePrincipal)
			if !ok {
				return nil, shared.Unauthorized("service principal not found")
			}
			if serviceAccount.Status != "active" {
				return nil, shared.Unauthorized("service principal not active")
			}
			return &principal{
				kind:       servicePrincipal,
				serviceID:  claims.ServicePrincipal,
				authMethod: "bearer",
			}, nil
		default:
			return nil, shared.Unauthorized("unknown token kind")
		}
	}
	return nil, nil
}

func validateAuthenticatedSession(session identity.Session) error {
	if session.Status != "active" {
		return shared.Unauthorized("session not active")
	}
	if !session.RevokedAt.IsZero() {
		return shared.Unauthorized("session revoked")
	}
	if !session.ExpiresAt.IsZero() && session.ExpiresAt.Before(time.Now().UTC()) {
		return shared.Unauthorized("session expired")
	}
	return nil
}

func currentPrincipal(r *http.Request) (principal, bool) {
	value := r.Context().Value(principalContextKey)
	if value == nil {
		return principal{}, false
	}
	p, ok := value.(principal)
	return p, ok
}

func authError(r *http.Request) error {
	value := r.Context().Value(authErrorContextKey)
	if value == nil {
		return nil
	}
	err, _ := value.(error)
	return err
}

func requireAuthorization(w http.ResponseWriter, r *http.Request, ident *identity.Service, userPermission, locationID, serviceOperation string) (principal, bool) {
	return requireAuthorizationWithOptions(w, r, ident, authorizationOptions{
		UserPermission:   userPermission,
		LocationID:       locationID,
		ServiceOperation: serviceOperation,
	})
}

type authorizationOptions struct {
	UserPermission   string
	LocationID       string
	ServiceOperation string
	RequireStepUp    bool
}

func requireAuthorizationWithOptions(w http.ResponseWriter, r *http.Request, ident *identity.Service, opts authorizationOptions) (principal, bool) {
	if err := authError(r); err != nil {
		respondError(w, err)
		return principal{}, false
	}
	p, ok := currentPrincipal(r)
	if !ok {
		respondError(w, shared.Unauthorized("authentication required"))
		return principal{}, false
	}
	switch p.kind {
	case userPrincipal:
		locationID := strings.TrimSpace(opts.LocationID)
		if locationID == "" {
			locationID = p.currentLocationID
		}
		decision := ident.DecideActingSession(p.sessionID, principalEffectiveUserID(p), opts.UserPermission, locationID, p.delegation)
		if !decision.Allowed {
			respondError(w, shared.Forbidden(decision.Reason))
			return principal{}, false
		}
		if opts.RequireStepUp && !p.stepUpVerified {
			respondError(w, shared.Forbidden("step-up verification required"))
			return principal{}, false
		}
		return p, true
	case servicePrincipal:
		decision := ident.DecideServicePrincipal(p.serviceID, opts.ServiceOperation)
		if !decision.Allowed {
			respondError(w, shared.Forbidden(decision.Reason))
			return principal{}, false
		}
		return p, true
	default:
		respondError(w, shared.Unauthorized("authentication required"))
		return principal{}, false
	}
}

func stepUpVerified(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Step-Up-Verified")), "true")
}

func principalEffectiveUserID(p principal) string {
	if strings.TrimSpace(p.effectiveUserID) != "" {
		return p.effectiveUserID
	}
	return p.userID
}

func principalHasDelegation(p principal) bool {
	return p.kind == userPrincipal && p.delegation != nil && p.delegationGrantID != ""
}

func principalOnBehalfOfUserID(p principal) string {
	if principalHasDelegation(p) {
		return principalEffectiveUserID(p)
	}
	return ""
}

func principalDelegationGrantID(p principal) string {
	return strings.TrimSpace(p.delegationGrantID)
}

func principalActingContext(p principal) application.ActingContext {
	return application.ActingContext{
		ActorID:           principalActorID(p),
		ActorKind:         principalActorKind(p),
		EffectiveUserID:   principalEffectiveUserID(p),
		OnBehalfOfUserID:  principalOnBehalfOfUserID(p),
		DelegationGrantID: principalDelegationGrantID(p),
	}
}

func requestActingContext(r *http.Request, p principal) application.ActingContext {
	acting := principalActingContext(p)
	if r != nil {
		acting.CorrelationID = strings.TrimSpace(logging.CorrelationID(r.Context()))
	}
	return acting
}

func principalAllowsPermission(ident *identity.Service, p principal, permissionKey, locationID string) bool {
	if strings.TrimSpace(permissionKey) == "" {
		return true
	}
	locationID = strings.TrimSpace(locationID)
	if locationID == "" {
		locationID = p.currentLocationID
	}
	switch p.kind {
	case userPrincipal:
		return ident.DecideActingSession(p.sessionID, principalEffectiveUserID(p), permissionKey, locationID, p.delegation).Allowed
	case servicePrincipal:
		return ident.DecideServicePrincipal(p.serviceID, permissionKey).Allowed
	default:
		return false
	}
}

func principalAllowsDocumentType(p principal, documentType string) bool {
	if !principalHasDelegation(p) {
		return true
	}
	allowed := p.delegation.AllowedDocumentTypes
	if len(allowed) == 0 {
		return true
	}
	documentType = strings.TrimSpace(documentType)
	for _, item := range allowed {
		if item == documentType {
			return true
		}
	}
	return false
}

func resolveDelegationPrincipal(r *http.Request, w http.ResponseWriter, ident *identity.Service, tokenManager *identity.TokenManager, session identity.Session, base *principal) (*principal, error) {
	if base == nil {
		return nil, nil
	}
	cookie, err := r.Cookie(delegationCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return base, nil
	}
	claims, err := tokenManager.Parse(cookie.Value)
	if err != nil || claims.Kind != "delegation" || claims.DelegationGrantID == "" {
		http.SetCookie(w, clearedDelegationCookie())
		return base, nil
	}
	grant, err := ident.ResolveDelegationGrantForActivation(claims.DelegationGrantID, session.UserID, session.CurrentLocationID, time.Now().UTC())
	if err != nil {
		http.SetCookie(w, clearedDelegationCookie())
		return base, nil
	}
	if claims.EffectiveUserID != "" && claims.EffectiveUserID != grant.GrantorUserID {
		http.SetCookie(w, clearedDelegationCookie())
		return base, nil
	}
	base.effectiveUserID = grant.GrantorUserID
	base.currentLocationID = grant.LocationID
	base.delegationGrantID = grant.ID
	base.onBehalfOfUserID = grant.GrantorUserID
	base.delegation = &grant
	return base, nil
}
