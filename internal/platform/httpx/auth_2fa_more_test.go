package httpx

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/shared"
)

func TestTOTPPolicyAndPayloadHelpers(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	user, err := ident.CreateUser("2fa-helpers", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	requiredPolicy := config.AuthPolicy{TOTPEnabled: true, TOTPEnrollmentAllowed: true, TOTPLoginMode: "required", TOTPApprovalMode: "optional", TOTPIssuer: "Orbyte"}
	enrollment, qrURI, err := ident.BeginTOTPEnrollment(user.ID, requiredPolicy.TOTPIssuer)
	if err != nil {
		t.Fatalf("begin enrollment failed: %v", err)
	}
	code := testTOTPCode(enrollment.Secret, time.Now().UTC())
	verified, err := ident.VerifyTOTPEnrollment(user.ID, code, true, true, time.Now().UTC())
	if err != nil {
		t.Fatalf("verify enrollment failed: %v", err)
	}

	resolved, hasEnrollment, loginRequired, approvalRequired := resolveTOTPState(requiredPolicy, ident, user.ID)
	if !hasEnrollment || !loginRequired || !approvalRequired {
		t.Fatalf("unexpected resolved totp state: has=%v login=%v approval=%v", hasEnrollment, loginRequired, approvalRequired)
	}
	if resolved.UserID != user.ID || resolved.Secret == "" {
		t.Fatalf("unexpected resolved enrollment: %+v", resolved)
	}

	if login, approval := enforceTOTPPreferences(config.AuthPolicy{TOTPLoginMode: "disabled", TOTPApprovalMode: "required"}, true, false); login || !approval {
		t.Fatalf("unexpected enforced preferences: login=%v approval=%v", login, approval)
	}
	if !enforceTOTPLoginPreference(requiredPolicy, verified) {
		t.Fatal("expected login preference to remain enabled")
	}
	if !policyRequiresMode(" required ") || !policyOptionalMode(" OPTIONAL ") {
		t.Fatal("expected policy mode helpers to recognize trimmed values")
	}

	if payload := serializeTOTPEnrollment(identity.TOTPEnrollment{}, false); payload["configured"] != false {
		t.Fatalf("unexpected empty enrollment serialization: %+v", payload)
	}
	payload := buildTOTPEnrollmentPayload(verified, qrURI)
	if payload["qr_uri"] != qrURI || payload["secret"] != verified.Secret {
		t.Fatalf("unexpected enrollment payload: %+v", payload)
	}

	challenge, err := ident.CreateAuthChallenge(user.ID, user.Username, "password", "loc_hq", identity.AuthChallengePurposeTOTPEnroll, map[string]any{"login_enabled": true}, time.Minute)
	if err != nil {
		t.Fatalf("create challenge failed: %v", err)
	}
	challengePayload := buildChallengePayload(challenge, &verified, qrURI)
	challengeMap, ok := challengePayload["challenge"].(map[string]any)
	if !ok || challengeMap["id"] != challenge.ID {
		t.Fatalf("unexpected challenge payload: %+v", challengePayload)
	}
	if challengePayload["qr_uri"] != qrURI || challengePayload["secret"] != verified.Secret {
		t.Fatalf("unexpected challenge payload secret/qr: %+v", challengePayload)
	}

	t.Setenv("APP_ENV", "dev")
	cookie := buildAuthChallengeCookie("challenge-1", challenge.ExpiresAt)
	if cookie.Name != authChallengeCookieName || cookie.Value != "challenge-1" || cookie.Secure {
		t.Fatalf("unexpected auth challenge cookie: %+v", cookie)
	}
	cleared := clearedAuthChallengeCookie()
	if cleared.Name != authChallengeCookieName || cleared.MaxAge != -1 || cleared.Value != "" {
		t.Fatalf("unexpected cleared auth challenge cookie: %+v", cleared)
	}
}

func TestCurrentChallengeDetailsBranches(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	user, err := ident.CreateUser("2fa-challenge", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/auth/2fa/challenge", nil)
	if _, _, _, err := currentChallengeDetails(req, ident); !isPlatformError(err, shared.KindNotFound) {
		t.Fatalf("expected missing cookie not found error, got %v", err)
	}

	req.AddCookie(httpCookie(authChallengeCookieName, "missing"))
	if _, _, _, err := currentChallengeDetails(req, ident); !isPlatformError(err, shared.KindNotFound) {
		t.Fatalf("expected missing challenge not found error, got %v", err)
	}

	verifyChallenge, err := ident.CreateAuthChallenge(user.ID, user.Username, "password", "loc_hq", identity.AuthChallengePurposeTOTPVerify, nil, time.Minute)
	if err != nil {
		t.Fatalf("create verify challenge failed: %v", err)
	}
	req = httptest.NewRequest("GET", "/auth/2fa/challenge", nil)
	req.AddCookie(httpCookie(authChallengeCookieName, verifyChallenge.ID))
	challenge, enrollment, qrURI, err := currentChallengeDetails(req, ident)
	if err != nil || challenge.ID != verifyChallenge.ID || enrollment != nil || qrURI != "" {
		t.Fatalf("unexpected verify challenge details: challenge=%+v enrollment=%+v qr=%q err=%v", challenge, enrollment, qrURI, err)
	}

	expired, err := ident.CreateAuthChallenge(user.ID, user.Username, "password", "loc_hq", identity.AuthChallengePurposeTOTPEnroll, nil, time.Millisecond)
	if err != nil {
		t.Fatalf("create expired challenge failed: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	req = httptest.NewRequest("GET", "/auth/2fa/challenge", nil)
	req.AddCookie(httpCookie(authChallengeCookieName, expired.ID))
	if _, _, _, err := currentChallengeDetails(req, ident); !isPlatformError(err, shared.KindForbidden) {
		t.Fatalf("expected expired challenge forbidden error, got %v", err)
	}

	noEnrollment, err := ident.CreateAuthChallenge(user.ID, user.Username, "password", "loc_hq", identity.AuthChallengePurposeTOTPEnroll, nil, time.Minute)
	if err != nil {
		t.Fatalf("create enrollment challenge failed: %v", err)
	}
	req = httptest.NewRequest("GET", "/auth/2fa/challenge", nil)
	req.AddCookie(httpCookie(authChallengeCookieName, noEnrollment.ID))
	if _, _, _, err := currentChallengeDetails(req, ident); !isPlatformError(err, shared.KindNotFound) {
		t.Fatalf("expected missing enrollment not found error, got %v", err)
	}

	pending, pendingQR, err := ident.BeginTOTPEnrollment(user.ID, "Orbyte")
	if err != nil {
		t.Fatalf("begin enrollment failed: %v", err)
	}
	enrollChallenge, err := ident.CreateAuthChallenge(user.ID, user.Username, "password", "loc_hq", identity.AuthChallengePurposeTOTPEnroll, nil, time.Minute)
	if err != nil {
		t.Fatalf("create enroll challenge failed: %v", err)
	}
	req = httptest.NewRequest("GET", "/auth/2fa/challenge", nil)
	req.AddCookie(httpCookie(authChallengeCookieName, enrollChallenge.ID))
	challenge, enrollment, qrURI, err = currentChallengeDetails(req, ident)
	if err != nil || challenge.ID != enrollChallenge.ID || enrollment == nil || enrollment.Secret != pending.Secret || qrURI != pendingQR {
		t.Fatalf("unexpected enroll challenge details: challenge=%+v enrollment=%+v qr=%q err=%v", challenge, enrollment, qrURI, err)
	}
}

func TestBeginInteractiveLoginAndIssueAuthenticatedSession(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	user, err := ident.CreateUser("2fa-login", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	rr := httptest.NewRecorder()
	noChallenge, err := beginInteractiveLogin(rr, ident, config.AuthPolicy{TOTPEnabled: true, TOTPEnrollmentAllowed: true, TOTPLoginMode: "optional", TOTPApprovalMode: "disabled"}, user, "password", "loc_hq", nil)
	if err != nil || noChallenge.RequiresChallenge {
		t.Fatalf("expected no login challenge, got result=%+v err=%v", noChallenge, err)
	}
	if len(rr.Result().Cookies()) != 0 {
		t.Fatalf("expected no cookies for non-required login, got %+v", rr.Result().Cookies())
	}

	enrollment, _, err := ident.BeginTOTPEnrollment(user.ID, "Orbyte")
	if err != nil {
		t.Fatalf("begin enrollment failed: %v", err)
	}
	code := testTOTPCode(enrollment.Secret, time.Now().UTC())
	if _, err := ident.VerifyTOTPEnrollment(user.ID, code, true, false, time.Now().UTC()); err != nil {
		t.Fatalf("verify enrollment failed: %v", err)
	}

	rr = httptest.NewRecorder()
	verifyResult, err := beginInteractiveLogin(rr, ident, config.AuthPolicy{TOTPEnabled: true, TOTPEnrollmentAllowed: true, TOTPLoginMode: "required", TOTPApprovalMode: "disabled"}, user, "password", "loc_hq", map[string]any{"ip": "127.0.0.1"})
	if err != nil || !verifyResult.RequiresChallenge || verifyResult.Challenge.Purpose != identity.AuthChallengePurposeTOTPVerify || verifyResult.Enrollment != nil {
		t.Fatalf("unexpected verify login result: %+v err=%v", verifyResult, err)
	}
	assertHasCookie(t, rr, authChallengeCookieName)

	secondUser, err := ident.CreateUser("2fa-enroll", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create second user failed: %v", err)
	}
	rr = httptest.NewRecorder()
	enrollResult, err := beginInteractiveLogin(rr, ident, config.AuthPolicy{TOTPEnabled: true, TOTPEnrollmentAllowed: true, TOTPIssuer: "Orbyte", TOTPLoginMode: "required", TOTPApprovalMode: "required"}, secondUser, "password", "loc_hq", nil)
	if err != nil || !enrollResult.RequiresChallenge || enrollResult.Challenge.Purpose != identity.AuthChallengePurposeTOTPEnroll || enrollResult.Enrollment == nil || enrollResult.QRURI == "" {
		t.Fatalf("unexpected enrollment login result: %+v err=%v", enrollResult, err)
	}
	assertHasCookie(t, rr, authChallengeCookieName)

	thirdUser, err := ident.CreateUser("2fa-forbidden", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create third user failed: %v", err)
	}
	if _, err := beginInteractiveLogin(httptest.NewRecorder(), ident, config.AuthPolicy{TOTPEnabled: false, TOTPEnrollmentAllowed: false, TOTPLoginMode: "required"}, thirdUser, "password", "loc_hq", nil); !isPlatformError(err, shared.KindForbidden) {
		t.Fatalf("expected required enrollment forbidden error, got %v", err)
	}

	t.Setenv("APP_JWT_SECRET", "test-secret")
	t.Setenv("APP_ENV", "dev")
	session := identity.Session{
		ID:                   shared.NewID("sess"),
		UserID:               user.ID,
		Status:               "active",
		IssuedAt:             time.Now().UTC(),
		LastSeenAt:           time.Now().UTC(),
		ExpiresAt:            time.Now().UTC().Add(time.Hour),
		AuthenticationMethod: "password",
		CurrentLocationID:    "loc_hq",
	}
	if err := ident.SaveSession(session); err != nil {
		t.Fatalf("save session failed: %v", err)
	}
	rr = httptest.NewRecorder()
	if err := issueAuthenticatedSession(rr, ident, session); err != nil {
		t.Fatalf("issueAuthenticatedSession failed: %v", err)
	}
	assertHasCookie(t, rr, sessionCookieName)
	assertHasCookie(t, rr, csrfCookieName)
	assertHasCookie(t, rr, authChallengeCookieName)
}

func testTOTPCode(secret string, at time.Time) string {
	normalized := strings.ToUpper(strings.TrimSpace(secret))
	if rem := len(normalized) % 8; rem != 0 {
		normalized += strings.Repeat("=", 8-rem)
	}
	key, err := base32.StdEncoding.DecodeString(normalized)
	if err != nil || len(key) == 0 {
		return ""
	}
	counter := uint64(at.UTC().Unix() / 30)
	msg := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		msg[i] = byte(counter & 0xff)
		counter >>= 8
	}
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(msg)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	truncated := (int(sum[offset])&0x7f)<<24 |
		int(sum[offset+1])<<16 |
		int(sum[offset+2])<<8 |
		int(sum[offset+3])
	return fmt.Sprintf("%06d", truncated%1000000)
}

func isPlatformError(err error, kind shared.Kind) bool {
	var platformErr shared.Error
	return err != nil && errors.As(err, &platformErr) && platformErr.Kind == kind
}

func assertHasCookie(t *testing.T, rr *httptest.ResponseRecorder, name string) {
	t.Helper()
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == name {
			return
		}
	}
	t.Fatalf("expected cookie %q, got %+v", name, rr.Result().Cookies())
}

func httpCookie(name, value string) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/"}
}
