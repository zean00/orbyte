package identity

import (
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/organization"
)

func TestTOTPHelpers(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate secret failed: %v", err)
	}
	if len(secret) < 16 {
		t.Fatalf("expected non-trivial secret, got %q", secret)
	}

	uri := BuildTOTPURI("ABC123", "Orbyte Platform", "user@example.com")
	if !strings.HasPrefix(uri, "otpauth://totp/Orbyte+Platform:user%40example.com?") {
		t.Fatalf("unexpected totp uri: %q", uri)
	}
	if !strings.Contains(uri, "issuer=Orbyte+Platform") || !strings.Contains(uri, "digits=6") || !strings.Contains(uri, "period=30") {
		t.Fatalf("expected issuer and otp parameters in uri: %q", uri)
	}

	at := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	code := generateTOTPCode(secret, at)
	if len(code) != totpDigits {
		t.Fatalf("expected %d-digit code, got %q", totpDigits, code)
	}
	if !ValidateTOTP(secret, code, at) {
		t.Fatal("expected totp code to validate at issuance time")
	}
	if !ValidateTOTP(secret, code, at.Add(totpPeriod)) {
		t.Fatal("expected adjacent time window to validate")
	}
	if ValidateTOTP(secret, "12345", at) {
		t.Fatal("expected invalid code length to fail")
	}
	if ValidateTOTP("not-a-valid-secret", code, at) {
		t.Fatal("expected invalid secret to fail validation")
	}
}

func TestTOTPEnrollmentAndChallengeLifecycle(t *testing.T) {
	svc := NewService(organization.NewService())
	user, err := svc.CreateUser("totp-user", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	enrollment, uri, err := svc.BeginTOTPEnrollment(user.ID, "Orbyte")
	if err != nil {
		t.Fatalf("begin enrollment failed: %v", err)
	}
	if enrollment.UserID != user.ID || !strings.Contains(uri, "issuer=Orbyte") {
		t.Fatalf("unexpected enrollment or uri: %+v %q", enrollment, uri)
	}

	storedEnrollment, ok := svc.repo.FindTOTPEnrollmentByUserID(user.ID)
	if !ok || strings.TrimSpace(storedEnrollment.Secret) == "" {
		t.Fatalf("expected stored enrollment secret, got %+v ok=%v", storedEnrollment, ok)
	}

	now := time.Now().UTC()
	code := generateTOTPCode(storedEnrollment.Secret, now)
	verified, err := svc.VerifyTOTPEnrollment(user.ID, code, true, false, now)
	if err != nil {
		t.Fatalf("verify enrollment failed: %v", err)
	}
	if verified.VerifiedAt.IsZero() || !verified.LoginEnabled || verified.ApprovalEnabled {
		t.Fatalf("unexpected verified enrollment: %+v", verified)
	}

	configured, err := svc.ConfigureVerifiedTOTP(user.ID, true, true)
	if err != nil {
		t.Fatalf("configure verified totp failed: %v", err)
	}
	if !configured.LoginEnabled || !configured.ApprovalEnabled {
		t.Fatalf("expected login and approval enabled, got %+v", configured)
	}

	challenge, err := svc.CreateAuthChallenge(user.ID, user.Username, "password", "loc_hq", AuthChallengePurposeTOTPVerify, map[string]any{"approval_enabled": true}, time.Minute)
	if err != nil {
		t.Fatalf("create auth challenge failed: %v", err)
	}
	session, consumed, err := svc.CompleteAuthChallenge(challenge.ID, generateTOTPCode(storedEnrollment.Secret, time.Now().UTC()), time.Hour)
	if err != nil {
		t.Fatalf("complete auth challenge failed: %v", err)
	}
	if session.UserID != user.ID || session.LoginStepUpAt.IsZero() {
		t.Fatalf("unexpected session after challenge completion: %+v", session)
	}
	if consumed.Status != "consumed" || consumed.ConsumedAt.IsZero() {
		t.Fatalf("expected consumed challenge, got %+v", consumed)
	}

	stepUpSession, err := svc.VerifySessionTOTP(session.ID, user.ID, generateTOTPCode(storedEnrollment.Secret, time.Now().UTC()), 15*time.Minute)
	if err != nil {
		t.Fatalf("verify session totp failed: %v", err)
	}
	if stepUpSession.ApprovalStepUpAt.IsZero() || stepUpSession.ApprovalStepUpUntil.IsZero() {
		t.Fatalf("expected approval step-up timestamps, got %+v", stepUpSession)
	}

	if err := svc.DisableTOTP(user.ID, now.Add(time.Minute)); err != nil {
		t.Fatalf("disable totp failed: %v", err)
	}
	disabled, ok := svc.FindTOTPEnrollmentByUserID(user.ID)
	if !ok || disabled.Secret != "" || disabled.LoginEnabled || disabled.ApprovalEnabled || disabled.DisabledAt.IsZero() {
		t.Fatalf("expected disabled enrollment, got %+v ok=%v", disabled, ok)
	}
	revokedSession, ok := svc.repo.FindSession(session.ID)
	if !ok || revokedSession.Status != "revoked" {
		t.Fatalf("expected revoked session after disable, got %+v ok=%v", revokedSession, ok)
	}
}

func TestAuthChallengeValidationAndExpirationBranches(t *testing.T) {
	svc := NewService(organization.NewService())
	if _, err := svc.CreateAuthChallenge("", "", "", "", "", nil, 0); err == nil {
		t.Fatal("expected invalid auth challenge to fail")
	}

	user, err := svc.CreateUser("expired-user", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	if _, _, err := svc.BeginTOTPEnrollment(user.ID, "Orbyte"); err != nil {
		t.Fatalf("begin enrollment failed: %v", err)
	}
	enrollment, _ := svc.repo.FindTOTPEnrollmentByUserID(user.ID)
	expiredChallenge, err := svc.CreateAuthChallenge(user.ID, user.Username, "password", "loc_hq", AuthChallengePurposeTOTPEnroll, map[string]any{"login_enabled": "true"}, time.Millisecond)
	if err != nil {
		t.Fatalf("create challenge failed: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, _, err := svc.CompleteAuthChallenge(expiredChallenge.ID, generateTOTPCode(enrollment.Secret, time.Now().UTC()), time.Hour); err == nil {
		t.Fatal("expected expired challenge to fail")
	}

	if got := TOTPAccountName(User{Username: "named-user"}); got != "named-user" {
		t.Fatalf("unexpected account name for username: %q", got)
	}
	if got := TOTPAccountName(User{ID: "user_123"}); got != "user:user_123" {
		t.Fatalf("unexpected account name fallback: %q", got)
	}
}
