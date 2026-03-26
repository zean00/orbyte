package identity

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/shared"
)

const (
	AuthChallengePurposeTOTPVerify = "totp_verify"
	AuthChallengePurposeTOTPEnroll = "totp_enroll"
)

func (s *Service) FindTOTPEnrollmentByUserID(userID string) (TOTPEnrollment, bool) {
	enrollment, ok := s.repo.FindTOTPEnrollmentByUserID(strings.TrimSpace(userID))
	if !ok {
		return TOTPEnrollment{}, false
	}
	return normalizeTOTPEnrollment(enrollment), true
}

func (s *Service) BeginTOTPEnrollment(userID, issuer string) (TOTPEnrollment, string, error) {
	user, ok := s.repo.FindUser(strings.TrimSpace(userID))
	if !ok {
		return TOTPEnrollment{}, "", shared.NotFound("user not found")
	}
	secret, err := GenerateTOTPSecret()
	if err != nil {
		return TOTPEnrollment{}, "", err
	}
	now := time.Now().UTC()
	enrollment := TOTPEnrollment{
		UserID:      user.ID,
		Secret:      secret,
		Issuer:      strings.TrimSpace(issuer),
		AccountName: user.Username,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if existing, ok := s.repo.FindTOTPEnrollmentByUserID(user.ID); ok {
		enrollment.CreatedAt = existing.CreatedAt
	}
	if err := s.repo.SaveTOTPEnrollment(enrollment); err != nil {
		return TOTPEnrollment{}, "", err
	}
	return normalizeTOTPEnrollment(enrollment), BuildTOTPURI(secret, enrollment.Issuer, enrollment.AccountName), nil
}

func (s *Service) ConfigureVerifiedTOTP(userID string, loginEnabled, approvalEnabled bool) (TOTPEnrollment, error) {
	enrollment, ok := s.repo.FindTOTPEnrollmentByUserID(strings.TrimSpace(userID))
	if !ok {
		return TOTPEnrollment{}, shared.NotFound("2FA enrollment not found")
	}
	enrollment = normalizeTOTPEnrollment(enrollment)
	if enrollment.VerifiedAt.IsZero() {
		return TOTPEnrollment{}, shared.Conflict("2FA enrollment is not verified")
	}
	enrollment.LoginEnabled = loginEnabled
	enrollment.ApprovalEnabled = approvalEnabled
	enrollment.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveTOTPEnrollment(enrollment); err != nil {
		return TOTPEnrollment{}, err
	}
	return normalizeTOTPEnrollment(enrollment), nil
}

func (s *Service) DisableTOTP(userID string, now time.Time) error {
	enrollment, ok := s.repo.FindTOTPEnrollmentByUserID(strings.TrimSpace(userID))
	if !ok {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	enrollment.Secret = ""
	enrollment.LoginEnabled = false
	enrollment.ApprovalEnabled = false
	enrollment.VerifiedAt = time.Time{}
	enrollment.DisabledAt = now
	enrollment.UpdatedAt = now
	if err := s.repo.SaveTOTPEnrollment(enrollment); err != nil {
		return err
	}
	return s.revokeUserSessions(enrollment.UserID, now)
}

func (s *Service) VerifyTOTPEnrollment(userID, code string, loginEnabled, approvalEnabled bool, now time.Time) (TOTPEnrollment, error) {
	enrollment, ok := s.repo.FindTOTPEnrollmentByUserID(strings.TrimSpace(userID))
	if !ok {
		return TOTPEnrollment{}, shared.NotFound("2FA enrollment not found")
	}
	enrollment = normalizeTOTPEnrollment(enrollment)
	if enrollment.Secret == "" {
		return TOTPEnrollment{}, shared.Conflict("2FA enrollment is disabled")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !ValidateTOTP(enrollment.Secret, code, now) {
		return TOTPEnrollment{}, shared.Unauthorized("invalid authentication code")
	}
	enrollment.VerifiedAt = now
	enrollment.DisabledAt = time.Time{}
	enrollment.LoginEnabled = loginEnabled
	enrollment.ApprovalEnabled = approvalEnabled
	enrollment.UpdatedAt = now
	if err := s.repo.SaveTOTPEnrollment(enrollment); err != nil {
		return TOTPEnrollment{}, err
	}
	return normalizeTOTPEnrollment(enrollment), nil
}

func (s *Service) CreateAuthChallenge(userID, username, authMethod, locationID, purpose string, clientMetadata map[string]any, ttl time.Duration) (AuthChallenge, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	now := time.Now().UTC()
	challenge := AuthChallenge{
		ID:                shared.NewID("auth-challenge"),
		UserID:            strings.TrimSpace(userID),
		Username:          strings.TrimSpace(username),
		AuthMethod:        strings.TrimSpace(authMethod),
		CurrentLocationID: strings.TrimSpace(locationID),
		Status:            "pending",
		Purpose:           strings.TrimSpace(purpose),
		ExpiresAt:         now.Add(ttl),
		CreatedAt:         now,
		ClientMetadata:    cloneMetadata(clientMetadata),
	}
	if challenge.UserID == "" || challenge.Username == "" || challenge.Purpose == "" {
		return AuthChallenge{}, shared.Validation("auth challenge is invalid")
	}
	if err := s.repo.SaveAuthChallenge(challenge); err != nil {
		return AuthChallenge{}, err
	}
	return challenge, nil
}

func (s *Service) FindAuthChallenge(id string) (AuthChallenge, bool) {
	challenge, ok := s.repo.FindAuthChallenge(strings.TrimSpace(id))
	if !ok {
		return AuthChallenge{}, false
	}
	return normalizeAuthChallenge(challenge), true
}

func (s *Service) CompleteAuthChallenge(challengeID, code string, sessionTTL time.Duration) (Session, AuthChallenge, error) {
	challenge, ok := s.repo.FindAuthChallenge(strings.TrimSpace(challengeID))
	if !ok {
		return Session{}, AuthChallenge{}, shared.NotFound("2FA challenge not found")
	}
	challenge = normalizeAuthChallenge(challenge)
	now := time.Now().UTC()
	if challenge.Status != "pending" {
		return Session{}, AuthChallenge{}, shared.Conflict("2FA challenge is no longer active")
	}
	if !challenge.ExpiresAt.IsZero() && !now.Before(challenge.ExpiresAt) {
		challenge.Status = "expired"
		challenge.ConsumedAt = now
		challenge.ClientMetadata = cloneMetadata(challenge.ClientMetadata)
		_ = s.repo.SaveAuthChallenge(challenge)
		return Session{}, AuthChallenge{}, shared.Forbidden("2FA challenge expired")
	}
	enrollment, ok := s.repo.FindTOTPEnrollmentByUserID(challenge.UserID)
	if !ok {
		return Session{}, AuthChallenge{}, shared.NotFound("2FA enrollment not found")
	}
	enrollment = normalizeTOTPEnrollment(enrollment)
	if enrollment.Secret == "" {
		return Session{}, AuthChallenge{}, shared.Conflict("2FA enrollment is disabled")
	}
	if !ValidateTOTP(enrollment.Secret, code, now) {
		return Session{}, AuthChallenge{}, shared.Unauthorized("invalid authentication code")
	}
	if enrollment.VerifiedAt.IsZero() {
		enrollment.VerifiedAt = now
		enrollment.DisabledAt = time.Time{}
		enrollment.LoginEnabled = metadataBool(challenge.ClientMetadata, "login_enabled")
		enrollment.ApprovalEnabled = metadataBool(challenge.ClientMetadata, "approval_enabled")
		enrollment.UpdatedAt = now
		if err := s.repo.SaveTOTPEnrollment(enrollment); err != nil {
			return Session{}, AuthChallenge{}, err
		}
	}
	user, ok := s.repo.FindUser(challenge.UserID)
	if !ok {
		return Session{}, AuthChallenge{}, shared.NotFound("user not found")
	}
	session, err := s.startSessionForUser(user, challenge.CurrentLocationID, challenge.AuthMethod, challenge.ClientMetadata, sessionTTL)
	if err != nil {
		return Session{}, AuthChallenge{}, err
	}
	session.LoginStepUpAt = now
	if err := s.repo.SaveSession(session); err != nil {
		return Session{}, AuthChallenge{}, err
	}
	challenge.Status = "consumed"
	challenge.ConsumedAt = now
	if err := s.repo.SaveAuthChallenge(challenge); err != nil {
		return Session{}, AuthChallenge{}, err
	}
	return session, normalizeAuthChallenge(challenge), nil
}

func (s *Service) VerifySessionTOTP(sessionID, userID, code string, ttl time.Duration) (Session, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	session, ok := s.repo.FindSession(strings.TrimSpace(sessionID))
	if !ok {
		return Session{}, shared.NotFound("session not found")
	}
	if strings.TrimSpace(userID) != "" && session.UserID != strings.TrimSpace(userID) {
		return Session{}, shared.Forbidden("session user mismatch")
	}
	enrollment, ok := s.repo.FindTOTPEnrollmentByUserID(session.UserID)
	if !ok {
		return Session{}, shared.NotFound("2FA enrollment not found")
	}
	enrollment = normalizeTOTPEnrollment(enrollment)
	if !ValidateTOTP(enrollment.Secret, code, time.Now().UTC()) {
		return Session{}, shared.Unauthorized("invalid authentication code")
	}
	now := time.Now().UTC()
	session.ApprovalStepUpAt = now
	session.ApprovalStepUpUntil = now.Add(ttl)
	if err := s.repo.SaveSession(session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func normalizeTOTPEnrollment(enrollment TOTPEnrollment) TOTPEnrollment {
	if !enrollment.DisabledAt.IsZero() {
		enrollment.LoginEnabled = false
		enrollment.ApprovalEnabled = false
	}
	return enrollment
}

func normalizeAuthChallenge(challenge AuthChallenge) AuthChallenge {
	now := time.Now().UTC()
	if challenge.Status == "pending" && !challenge.ExpiresAt.IsZero() && !now.Before(challenge.ExpiresAt) {
		challenge.Status = "expired"
	}
	return challenge
}

func metadataBool(metadata map[string]any, key string) bool {
	raw, ok := metadata[strings.TrimSpace(key)]
	if !ok {
		return false
	}
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

func TOTPAccountName(user User) string {
	if strings.TrimSpace(user.Username) != "" {
		return user.Username
	}
	return fmt.Sprintf("user:%s", user.ID)
}
