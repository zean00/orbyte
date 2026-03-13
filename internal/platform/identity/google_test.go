package identity

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
)

func TestOIDCGoogleVerifierVerifyIDToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	pubKey, err := jwk.PublicKeyOf(key)
	if err != nil {
		t.Fatalf("jwk public key failed: %v", err)
	}
	if err := pubKey.Set(jwk.KeyIDKey, "kid-1"); err != nil {
		t.Fatalf("set key id failed: %v", err)
	}
	set := jwk.NewSet()
	if err := set.AddKey(pubKey); err != nil {
		t.Fatalf("add jwk key failed: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(set)
	}))
	defer server.Close()

	token := signRS256Token(t, key, "kid-1", map[string]any{
		"iss":            "https://accounts.google.com",
		"sub":            "sub-123",
		"aud":            "client-123",
		"email":          "user@example.com",
		"email_verified": true,
		"exp":            time.Now().UTC().Add(time.Hour).Unix(),
		"iat":            time.Now().UTC().Unix(),
	})
	verified, err := (OIDCGoogleVerifier{}).VerifyIDToken(context.Background(), token, GoogleAuthSettings{
		Enabled:  true,
		ClientID: "client-123",
		Issuer:   "https://accounts.google.com",
		JWKSURL:  server.URL,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("verify id token failed: %v", err)
	}
	if verified.Subject != "sub-123" || verified.Email != "user@example.com" {
		t.Fatalf("unexpected verified identity: %+v", verified)
	}
}

func signRS256Token(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid})
	if err != nil {
		t.Fatalf("marshal header failed: %v", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + payload
	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}
	return fmt.Sprintf("%s.%s", signingInput, strings.TrimSpace(base64.RawURLEncoding.EncodeToString(signature)))
}
