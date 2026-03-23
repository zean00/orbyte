package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

type TokenManager struct {
	secret []byte
	issuer string
	now    func() time.Time
}

type TokenClaims struct {
	Subject           string `json:"sub"`
	SessionID         string `json:"session_id,omitempty"`
	ServicePrincipal  string `json:"service_principal_id,omitempty"`
	DelegationGrantID string `json:"delegation_grant_id,omitempty"`
	DeepLinkGrantID   string `json:"deep_link_grant_id,omitempty"`
	EffectiveUserID   string `json:"effective_user_id,omitempty"`
	TargetType        string `json:"target_type,omitempty"`
	TargetID          string `json:"target_id,omitempty"`
	Kind              string `json:"kind"`
	IssuedAt          int64  `json:"iat"`
	ExpiresAt         int64  `json:"exp"`
	Issuer            string `json:"iss"`
}

func NewTokenManagerFromEnv() *TokenManager {
	issuer := os.Getenv("APP_JWT_ISSUER")
	if issuer == "" {
		issuer = "orbyte"
	}
	return &TokenManager{
		secret: []byte(os.Getenv("APP_JWT_SECRET")),
		issuer: issuer,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func (m *TokenManager) IssueSessionToken(session Session) (string, error) {
	if session.ID == "" || session.UserID == "" {
		return "", errors.New("invalid session token subject")
	}
	return m.issue(TokenClaims{
		Subject:   session.UserID,
		SessionID: session.ID,
		Kind:      "session",
		IssuedAt:  m.now().Unix(),
		ExpiresAt: session.ExpiresAt.Unix(),
		Issuer:    m.issuer,
	})
}

func (m *TokenManager) IssueServicePrincipalToken(principal ServicePrincipal, ttl time.Duration) (string, error) {
	if principal.ID == "" {
		return "", errors.New("invalid service principal token subject")
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	now := m.now()
	return m.issue(TokenClaims{
		Subject:          principal.ID,
		ServicePrincipal: principal.ID,
		Kind:             "service",
		IssuedAt:         now.Unix(),
		ExpiresAt:        now.Add(ttl).Unix(),
		Issuer:           m.issuer,
	})
}

func (m *TokenManager) IssueDelegationToken(grant DelegationGrant) (string, error) {
	delegateID := strings.TrimSpace(grant.DelegateID)
	if delegateID == "" {
		delegateID = strings.TrimSpace(grant.DelegateUserID)
	}
	delegateKind := strings.ToLower(strings.TrimSpace(grant.DelegateKind))
	if delegateKind == "" {
		delegateKind = "user"
	}
	if grant.ID == "" || delegateID == "" || grant.GrantorUserID == "" || delegateKind != "user" {
		return "", errors.New("invalid delegation token subject")
	}
	now := m.now()
	expiresAt := grant.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = now.Add(time.Hour)
	}
	return m.issue(TokenClaims{
		Subject:           delegateID,
		DelegationGrantID: grant.ID,
		EffectiveUserID:   grant.GrantorUserID,
		Kind:              "delegation",
		IssuedAt:          now.Unix(),
		ExpiresAt:         expiresAt.Unix(),
		Issuer:            m.issuer,
	})
}

func (m *TokenManager) IssueDeepLinkToken(grant DeepLinkGrant) (string, error) {
	if grant.ID == "" || grant.UserID == "" || grant.TargetType == "" || grant.TargetID == "" {
		return "", errors.New("invalid deep link token subject")
	}
	now := m.now()
	expiresAt := grant.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = now.Add(15 * time.Minute)
	}
	return m.issue(TokenClaims{
		Subject:         grant.UserID,
		DeepLinkGrantID: grant.ID,
		TargetType:      grant.TargetType,
		TargetID:        grant.TargetID,
		Kind:            "link",
		IssuedAt:        now.Unix(),
		ExpiresAt:       expiresAt.Unix(),
		Issuer:          m.issuer,
	})
}

func (m *TokenManager) IssueDeepLinkStepUpToken(grant DeepLinkGrant, ttl time.Duration) (string, error) {
	if grant.ID == "" || grant.UserID == "" {
		return "", errors.New("invalid deep link step-up token subject")
	}
	now := m.now()
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	expiresAt := now.Add(ttl)
	if !grant.ExpiresAt.IsZero() && grant.ExpiresAt.Before(expiresAt) {
		expiresAt = grant.ExpiresAt
	}
	return m.issue(TokenClaims{
		Subject:         grant.UserID,
		DeepLinkGrantID: grant.ID,
		TargetType:      grant.TargetType,
		TargetID:        grant.TargetID,
		Kind:            "link_step_up",
		IssuedAt:        now.Unix(),
		ExpiresAt:       expiresAt.Unix(),
		Issuer:          m.issuer,
	})
}

func (m *TokenManager) Parse(token string) (TokenClaims, error) {
	if len(m.secret) == 0 {
		return TokenClaims{}, errors.New("jwt secret is not configured")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return TokenClaims{}, errors.New("invalid token format")
	}
	signingInput := parts[0] + "." + parts[1]
	expected := signHS256([]byte(signingInput), m.secret)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return TokenClaims{}, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return TokenClaims{}, err
	}
	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return TokenClaims{}, err
	}
	if claims.Issuer != m.issuer {
		return TokenClaims{}, errors.New("invalid token issuer")
	}
	if claims.ExpiresAt > 0 && m.now().Unix() > claims.ExpiresAt {
		return TokenClaims{}, errors.New("token expired")
	}
	return claims, nil
}

func (m *TokenManager) issue(claims TokenClaims) (string, error) {
	if len(m.secret) == 0 {
		return "", errors.New("jwt secret is not configured")
	}
	headerJSON, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + payload
	signature := signHS256([]byte(signingInput), m.secret)
	return signingInput + "." + signature, nil
}

func signHS256(payload, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
