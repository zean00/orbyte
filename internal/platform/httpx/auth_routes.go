package httpx

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/shared"
)

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	LocationID string `json:"location_id"`
}

type changePasswordRequest struct {
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

func registerAuthRoutes(mux *http.ServeMux, cfg *config.Service, ident *identity.Service, auditSvc *audit.Service) {
	policy := cfg.AuthPolicy()
	limiter := newLoginRateLimiter(ident, policy.LoginRateLimitAttempts, policy.LoginRateLimitWindow)

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

	mux.HandleFunc("POST /auth/login", func(w http.ResponseWriter, r *http.Request) {
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

		session, err := ident.AuthenticatePassword(req.Username, req.Password, req.LocationID, clientMetadataFromRequest(r), policy.SessionTTL)
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

		tokenManager := identity.NewTokenManagerFromEnv()
		token, err := tokenManager.IssueSessionToken(session)
		if err != nil {
			_, _ = ident.RevokeSession(session.ID, time.Now().UTC())
			respondError(w, err)
			return
		}
		csrfCookie, err := buildCSRFCookie(session.ID)
		if err != nil {
			_, _ = ident.RevokeSession(session.ID, time.Now().UTC())
			respondError(w, err)
			return
		}

		http.SetCookie(w, buildSessionCookie(token, session.ExpiresAt))
		http.SetCookie(w, csrfCookie)
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

	mux.HandleFunc("POST /auth/password/change", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid password change payload"))
			return
		}
		if err := ident.ChangePasswordWithPolicy(p.userID, req.CurrentPassword, req.NewPassword, policy.PasswordMinLength); err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:auth:password_change:" + p.userID,
			Action:        "auth.password.change",
			TargetType:    "user",
			TargetID:      p.userID,
			ActorID:       p.userID,
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{"status": "password_changed"})
	})

	mux.HandleFunc("POST /auth/refresh", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users")
		if !ok {
			return
		}
		var req createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid user create payload"))
			return
		}
		user, err := ident.CreateUserWithPasswordPolicy(req.Username, req.Password, strings.TrimSpace(req.DefaultLocationID), strings.TrimSpace(req.RoleID), strings.TrimSpace(req.ScopeType), strings.TrimSpace(req.ScopeID), policy.PasswordMinLength)
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
		userID, ok := userResetPasswordPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		p, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users")
		if !ok {
			return
		}
		var req resetPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid password reset payload"))
			return
		}
		if err := ident.ResetPasswordWithPolicy(userID, req.NewPassword, policy.PasswordMinLength); err != nil {
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
}

func buildSessionCookie(token string, expiresAt time.Time) *http.Cookie {
	secure := true
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) {
	case "", "development", "dev", "test":
		secure = false
	}
	return &http.Cookie{
		Name:     sessionCookieName,
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
	_ = auditSvc.Record(event)
}
