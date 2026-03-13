package httpx

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

const sessionCookieName = "orbyte_session"
const csrfCookieName = "orbyte_csrf"

type principalKind string

const (
	userPrincipal    principalKind = "user"
	servicePrincipal principalKind = "service"
)

type principal struct {
	kind              principalKind
	userID            string
	sessionID         string
	currentLocationID string
	serviceID         string
	authMethod        string
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
		p, authErr := authenticateRequest(r, ident, tokenManager, devBypass)
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

func authenticateRequest(r *http.Request, ident *identity.Service, tokenManager *identity.TokenManager, devBypass bool) (*principal, error) {
	if devBypass {
		if userID := strings.TrimSpace(r.Header.Get("X-Dev-User-ID")); userID != "" {
			for _, session := range ident.Sessions() {
				if session.UserID == userID && session.Status == "active" {
					return &principal{
						kind:              userPrincipal,
						userID:            userID,
						sessionID:         session.ID,
						currentLocationID: session.CurrentLocationID,
						authMethod:        "dev_bypass",
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
		return &principal{
			kind:              userPrincipal,
			userID:            session.UserID,
			sessionID:         session.ID,
			currentLocationID: session.CurrentLocationID,
			authMethod:        "cookie",
		}, nil
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
			return &principal{
				kind:              userPrincipal,
				userID:            session.UserID,
				sessionID:         session.ID,
				currentLocationID: session.CurrentLocationID,
				authMethod:        "bearer",
			}, nil
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
		decision := ident.DecideSession(p.sessionID, userPermission, locationID)
		if !decision.Allowed {
			respondError(w, shared.Forbidden(decision.Reason))
			return principal{}, false
		}
		return p, true
	case servicePrincipal:
		if serviceOperation == "" {
			respondError(w, shared.Forbidden("service principal access is not allowed for this route"))
			return principal{}, false
		}
		decision := ident.DecideServicePrincipal(p.serviceID, serviceOperation)
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
