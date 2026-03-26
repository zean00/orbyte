package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/runtimeconfig"
	"orbyte/internal/platform/shared"
)

type totpCodeRequest struct {
	Code            string `json:"code"`
	LoginEnabled    bool   `json:"login_enabled"`
	ApprovalEnabled bool   `json:"approval_enabled"`
}

type interactiveLoginResult struct {
	RequiresChallenge bool
	Challenge         identity.AuthChallenge
	Enrollment        *identity.TOTPEnrollment
	QRURI             string
}

func registerTwoFactorAuthRoutes(mux *http.ServeMux, cfg *config.Service, ident *identity.Service, auditSvc *audit.Service) {
	mux.HandleFunc("GET /auth/2fa", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		policy := cfg.AuthPolicy()
		enrollment, hasEnrollment, loginRequired, approvalRequired := resolveTOTPState(policy, ident, p.userID)
		respondJSON(w, http.StatusOK, map[string]any{
			"enabled":                 policy.TOTPEnabled,
			"enrollment_allowed":      policy.TOTPEnabled && policy.TOTPEnrollmentAllowed,
			"issuer":                  policy.TOTPIssuer,
			"login_mode":              policy.TOTPLoginMode,
			"approval_mode":           policy.TOTPApprovalMode,
			"approval_step_up_active": !p.approvalStepUpUntil.IsZero() && time.Now().UTC().Before(p.approvalStepUpUntil),
			"approval_step_up_until":  p.approvalStepUpUntil,
			"login_required":          loginRequired,
			"approval_required":       approvalRequired,
			"enrollment":              serializeTOTPEnrollment(enrollment, hasEnrollment),
		})
	})

	mux.HandleFunc("POST /auth/2fa/enroll", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		policy := cfg.AuthPolicy()
		if !policy.TOTPEnabled || !policy.TOTPEnrollmentAllowed {
			respondError(w, shared.Forbidden("2FA enrollment is disabled"))
			return
		}
		enrollment, qrURI, err := ident.BeginTOTPEnrollment(p.userID, policy.TOTPIssuer)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, buildTOTPEnrollmentPayload(enrollment, qrURI))
	})

	mux.HandleFunc("POST /auth/2fa/verify-enrollment", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req totpCodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid 2FA verification payload"))
			return
		}
		policy := cfg.AuthPolicy()
		loginEnabled, approvalEnabled := enforceTOTPPreferences(policy, req.LoginEnabled, req.ApprovalEnabled)
		enrollment, err := ident.VerifyTOTPEnrollment(p.userID, req.Code, loginEnabled, approvalEnabled, time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:auth:2fa:verify-enrollment:" + p.userID,
			Action:        "auth.2fa.verify_enrollment",
			TargetType:    "user",
			TargetID:      p.userID,
			ActorID:       p.userID,
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, buildTOTPEnrollmentPayload(enrollment, ""))
	})

	mux.HandleFunc("PUT /auth/2fa/preferences", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req totpCodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid 2FA preference payload"))
			return
		}
		policy := cfg.AuthPolicy()
		loginEnabled, approvalEnabled := enforceTOTPPreferences(policy, req.LoginEnabled, req.ApprovalEnabled)
		enrollment, err := ident.ConfigureVerifiedTOTP(p.userID, loginEnabled, approvalEnabled)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, buildTOTPEnrollmentPayload(enrollment, ""))
	})

	mux.HandleFunc("DELETE /auth/2fa", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		policy := cfg.AuthPolicy()
		if policyRequiresMode(policy.TOTPLoginMode) || policyRequiresMode(policy.TOTPApprovalMode) {
			respondError(w, shared.Forbidden("2FA is required by policy"))
			return
		}
		if err := ident.DisableTOTP(p.userID, time.Now().UTC()); err != nil {
			respondError(w, err)
			return
		}
		http.SetCookie(w, clearedSessionCookie())
		http.SetCookie(w, clearedCSRFCookie())
		http.SetCookie(w, clearedAuthChallengeCookie())
		respondJSON(w, http.StatusOK, map[string]any{"status": "disabled", "logged_out": true})
	})

	mux.HandleFunc("POST /auth/2fa/approval/verify", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.userID == "" || p.sessionID == "" {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req totpCodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid 2FA approval payload"))
			return
		}
		session, err := ident.VerifySessionTOTP(p.sessionID, p.userID, req.Code, cfg.AuthPolicy().TOTPStepUpTTL)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"status":                  "verified",
			"approval_step_up_until":  session.ApprovalStepUpUntil,
			"approval_step_up_active": true,
		})
	})

	mux.HandleFunc("GET /auth/2fa/challenge", func(w http.ResponseWriter, r *http.Request) {
		challenge, enrollment, qrURI, err := currentChallengeDetails(r, ident)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, buildChallengePayload(challenge, enrollment, qrURI))
	})

	mux.HandleFunc("POST /auth/2fa/challenge/verify", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(authChallengeCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			respondError(w, shared.NotFound("2FA challenge not found"))
			return
		}
		var req totpCodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid 2FA challenge payload"))
			return
		}
		policy := cfg.AuthPolicy()
		session, challenge, err := ident.CompleteAuthChallenge(cookie.Value, req.Code, policy.SessionTTL)
		if err != nil {
			respondError(w, err)
			return
		}
		if err := issueAuthenticatedSession(w, ident, session); err != nil {
			respondError(w, err)
			return
		}
		http.SetCookie(w, clearedAuthChallengeCookie())
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:auth:2fa:challenge:" + challenge.ID,
			Action:        "auth.2fa.challenge.complete",
			TargetType:    "session",
			TargetID:      session.ID,
			ActorID:       session.UserID,
			OccurredAt:    time.Now().UTC(),
			CorrelationID: logging.CorrelationID(r.Context()),
		})
		respondJSON(w, http.StatusOK, map[string]any{"session": session})
	})
}

func beginInteractiveLogin(w http.ResponseWriter, ident *identity.Service, policy config.AuthPolicy, user identity.User, authMethod, locationID string, clientMetadata map[string]any) (interactiveLoginResult, error) {
	enrollment, hasEnrollment, loginRequired, approvalRequired := resolveTOTPState(policy, ident, user.ID)
	if !loginRequired {
		return interactiveLoginResult{}, nil
	}
	if hasEnrollment && !enrollment.VerifiedAt.IsZero() && enrollment.Secret != "" {
		challenge, err := ident.CreateAuthChallenge(user.ID, user.Username, authMethod, locationID, identity.AuthChallengePurposeTOTPVerify, map[string]any{
			"login_enabled":    enforceTOTPLoginPreference(policy, enrollment),
			"approval_enabled": approvalRequired || enrollment.ApprovalEnabled,
		}, 10*time.Minute)
		if err != nil {
			return interactiveLoginResult{}, err
		}
		http.SetCookie(w, buildAuthChallengeCookie(challenge.ID, challenge.ExpiresAt))
		return interactiveLoginResult{RequiresChallenge: true, Challenge: challenge}, nil
	}
	if !policy.TOTPEnabled || !policy.TOTPEnrollmentAllowed {
		return interactiveLoginResult{}, shared.Forbidden("2FA enrollment is required by policy")
	}
	pendingEnrollment, qrURI, err := ident.BeginTOTPEnrollment(user.ID, policy.TOTPIssuer)
	if err != nil {
		return interactiveLoginResult{}, err
	}
	challenge, err := ident.CreateAuthChallenge(user.ID, user.Username, authMethod, locationID, identity.AuthChallengePurposeTOTPEnroll, map[string]any{
		"login_enabled":    true,
		"approval_enabled": approvalRequired,
	}, 10*time.Minute)
	if err != nil {
		return interactiveLoginResult{}, err
	}
	http.SetCookie(w, buildAuthChallengeCookie(challenge.ID, challenge.ExpiresAt))
	return interactiveLoginResult{RequiresChallenge: true, Challenge: challenge, Enrollment: &pendingEnrollment, QRURI: qrURI}, nil
}

func resolveTOTPState(policy config.AuthPolicy, ident *identity.Service, userID string) (identity.TOTPEnrollment, bool, bool, bool) {
	enrollment, ok := ident.FindTOTPEnrollmentByUserID(userID)
	if !policy.TOTPEnabled || !ok || enrollment.Secret == "" || enrollment.VerifiedAt.IsZero() {
		return enrollment, ok, policyRequiresMode(policy.TOTPLoginMode), policyRequiresMode(policy.TOTPApprovalMode)
	}
	loginRequired := policyRequiresMode(policy.TOTPLoginMode) || (policyOptionalMode(policy.TOTPLoginMode) && enrollment.LoginEnabled)
	approvalRequired := policyRequiresMode(policy.TOTPApprovalMode) || (policyOptionalMode(policy.TOTPApprovalMode) && enrollment.ApprovalEnabled)
	return enrollment, true, loginRequired, approvalRequired
}

func serializeTOTPEnrollment(enrollment identity.TOTPEnrollment, hasEnrollment bool) map[string]any {
	if !hasEnrollment {
		return map[string]any{"configured": false}
	}
	return map[string]any{
		"configured":       enrollment.Secret != "",
		"verified":         !enrollment.VerifiedAt.IsZero(),
		"login_enabled":    enrollment.LoginEnabled,
		"approval_enabled": enrollment.ApprovalEnabled,
		"issuer":           enrollment.Issuer,
		"account_name":     enrollment.AccountName,
		"verified_at":      enrollment.VerifiedAt,
		"disabled_at":      enrollment.DisabledAt,
	}
}

func buildTOTPEnrollmentPayload(enrollment identity.TOTPEnrollment, qrURI string) map[string]any {
	payload := map[string]any{"enrollment": serializeTOTPEnrollment(enrollment, true)}
	if qrURI != "" {
		payload["qr_uri"] = qrURI
		payload["secret"] = enrollment.Secret
	}
	return payload
}

func buildChallengePayload(challenge identity.AuthChallenge, enrollment *identity.TOTPEnrollment, qrURI string) map[string]any {
	payload := map[string]any{
		"challenge": map[string]any{
			"id":         challenge.ID,
			"status":     challenge.Status,
			"purpose":    challenge.Purpose,
			"expires_at": challenge.ExpiresAt,
			"username":   challenge.Username,
			"auth_method": challenge.AuthMethod,
		},
	}
	if enrollment != nil {
		payload["enrollment"] = serializeTOTPEnrollment(*enrollment, true)
		if qrURI != "" {
			payload["qr_uri"] = qrURI
			payload["secret"] = enrollment.Secret
		}
	}
	return payload
}

func currentChallengeDetails(r *http.Request, ident *identity.Service) (identity.AuthChallenge, *identity.TOTPEnrollment, string, error) {
	cookie, err := r.Cookie(authChallengeCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return identity.AuthChallenge{}, nil, "", shared.NotFound("2FA challenge not found")
	}
	challenge, ok := ident.FindAuthChallenge(cookie.Value)
	if !ok {
		return identity.AuthChallenge{}, nil, "", shared.NotFound("2FA challenge not found")
	}
	if challenge.Status != "pending" {
		return challenge, nil, "", shared.Conflict("2FA challenge is no longer active")
	}
	if !challenge.ExpiresAt.IsZero() && !time.Now().UTC().Before(challenge.ExpiresAt) {
		return challenge, nil, "", shared.Forbidden("2FA challenge expired")
	}
	if challenge.Purpose != identity.AuthChallengePurposeTOTPEnroll {
		return challenge, nil, "", nil
	}
	enrollment, ok := ident.FindTOTPEnrollmentByUserID(challenge.UserID)
	if !ok {
		return challenge, nil, "", shared.NotFound("2FA enrollment not found")
	}
	qrURI := ""
	if enrollment.Secret != "" {
		qrURI = identity.BuildTOTPURI(enrollment.Secret, enrollment.Issuer, enrollment.AccountName)
	}
	return challenge, &enrollment, qrURI, nil
}

func enforceTOTPPreferences(policy config.AuthPolicy, loginEnabled, approvalEnabled bool) (bool, bool) {
	if policyRequiresMode(policy.TOTPLoginMode) {
		loginEnabled = true
	}
	if policyRequiresMode(policy.TOTPApprovalMode) {
		approvalEnabled = true
	}
	if !policyOptionalMode(policy.TOTPLoginMode) && !policyRequiresMode(policy.TOTPLoginMode) {
		loginEnabled = false
	}
	if !policyOptionalMode(policy.TOTPApprovalMode) && !policyRequiresMode(policy.TOTPApprovalMode) {
		approvalEnabled = false
	}
	return loginEnabled, approvalEnabled
}

func enforceTOTPLoginPreference(policy config.AuthPolicy, enrollment identity.TOTPEnrollment) bool {
	loginEnabled, _ := enforceTOTPPreferences(policy, enrollment.LoginEnabled, enrollment.ApprovalEnabled)
	return loginEnabled
}

func policyRequiresMode(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), "required")
}

func policyOptionalMode(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), "optional")
}

func buildAuthChallengeCookie(challengeID string, expiresAt time.Time) *http.Cookie {
	secure := buildSessionCookie("", time.Now().UTC()).Secure
	return &http.Cookie{
		Name:     authChallengeCookieName,
		Value:    challengeID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	}
}

func clearedAuthChallengeCookie() *http.Cookie {
	return &http.Cookie{
		Name:     authChallengeCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   runtimeconfig.Current().CookieSecure(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
}

func issueAuthenticatedSession(w http.ResponseWriter, ident *identity.Service, session identity.Session) error {
	if _, ok := ident.FindSession(session.ID); ok {
		if err := ident.SaveSession(session); err != nil {
			return err
		}
	}
	tokenManager := identity.NewTokenManagerFromEnv()
	token, err := tokenManager.IssueSessionToken(session)
	if err != nil {
		_, _ = ident.RevokeSession(session.ID, time.Now().UTC())
		return err
	}
	csrfCookie, err := buildCSRFCookie(session.ID)
	if err != nil {
		_, _ = ident.RevokeSession(session.ID, time.Now().UTC())
		return err
	}
	http.SetCookie(w, buildSessionCookie(token, session.ExpiresAt))
	http.SetCookie(w, csrfCookie)
	http.SetCookie(w, clearedAuthChallengeCookie())
	return nil
}
