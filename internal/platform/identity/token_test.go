package identity

import (
	"testing"
	"time"
)

func TestTokenManagerIssueAndParseSessionToken(t *testing.T) {
	manager := &TokenManager{
		secret: []byte("test-secret"),
		issuer: "test-issuer",
		now:    func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
	token, err := manager.IssueSessionToken(Session{
		ID:        "s1",
		UserID:    "u1",
		ExpiresAt: time.Unix(1700003600, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("issue session token failed: %v", err)
	}
	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("parse session token failed: %v", err)
	}
	if claims.Kind != "session" || claims.SessionID != "s1" || claims.Subject != "u1" {
		t.Fatalf("unexpected session claims: %+v", claims)
	}
}

func TestTokenManagerIssueAndParseServicePrincipalToken(t *testing.T) {
	manager := &TokenManager{
		secret: []byte("test-secret"),
		issuer: "test-issuer",
		now:    func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
	token, err := manager.IssueServicePrincipalToken(ServicePrincipal{ID: "sp1"}, time.Hour)
	if err != nil {
		t.Fatalf("issue service token failed: %v", err)
	}
	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("parse service token failed: %v", err)
	}
	if claims.Kind != "service" || claims.ServicePrincipal != "sp1" {
		t.Fatalf("unexpected service claims: %+v", claims)
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager := &TokenManager{
		secret: []byte("test-secret"),
		issuer: "test-issuer",
		now:    func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
	token, err := manager.IssueSessionToken(Session{
		ID:        "s1",
		UserID:    "u1",
		ExpiresAt: time.Unix(1699999999, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("issue expired token failed: %v", err)
	}
	if _, err := manager.Parse(token); err == nil {
		t.Fatal("expected expired token parse failure")
	}
}

func TestTokenManagerRejectsTamperedToken(t *testing.T) {
	manager := &TokenManager{
		secret: []byte("test-secret"),
		issuer: "test-issuer",
		now:    func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
	token, err := manager.IssueSessionToken(Session{
		ID:        "s1",
		UserID:    "u1",
		ExpiresAt: time.Unix(1700003600, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	if _, err := manager.Parse(token + "tampered"); err == nil {
		t.Fatal("expected tampered token parse failure")
	}
}
