package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/shared"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

type GoogleAuthSettings struct {
	Enabled    bool
	ClientID   string
	Issuer     string
	JWKSURL    string
	HostedDomain string
	Timeout    time.Duration
}

type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	PictureURL    string
	HostedDomain  string
	Issuer        string
}

type GoogleVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string, settings GoogleAuthSettings) (GoogleIdentity, error)
}

type OIDCGoogleVerifier struct {
	HTTPClient *http.Client
}

func (v OIDCGoogleVerifier) VerifyIDToken(ctx context.Context, idToken string, settings GoogleAuthSettings) (GoogleIdentity, error) {
	if !settings.Enabled {
		return GoogleIdentity{}, shared.Forbidden("google authentication is not enabled")
	}
	if strings.TrimSpace(settings.ClientID) == "" {
		return GoogleIdentity{}, shared.Validation("google client id is not configured")
	}
	if strings.TrimSpace(settings.JWKSURL) == "" {
		return GoogleIdentity{}, shared.Validation("google jwks url is not configured")
	}
	if strings.TrimSpace(idToken) == "" {
		return GoogleIdentity{}, shared.Validation("google id token is required")
	}
	if settings.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, settings.Timeout)
		defer cancel()
	}
	fetchOptions := []jwk.FetchOption{}
	if v.HTTPClient != nil {
		fetchOptions = append(fetchOptions, jwk.WithHTTPClient(v.HTTPClient))
	}
	set, err := jwk.Fetch(ctx, settings.JWKSURL, fetchOptions...)
	if err != nil {
		return GoogleIdentity{}, err
	}
	token, err := jwt.Parse([]byte(idToken),
		jwt.WithKeySet(set, jws.WithInferAlgorithmFromKey(true), jws.WithUseDefault(true)),
		jwt.WithValidate(false),
	)
	if err != nil {
		return GoogleIdentity{}, err
	}
	validateOptions := []jwt.ValidateOption{jwt.WithAudience(settings.ClientID)}
	if issuer := strings.TrimSpace(settings.Issuer); issuer != "" {
		validateOptions = append(validateOptions, jwt.WithIssuer(issuer))
	}
	if err := jwt.Validate(token, validateOptions...); err != nil {
		return GoogleIdentity{}, err
	}

	issuer, _ := token.Issuer()
	subject, _ := token.Subject()
	email, _ := stringClaim(token, "email")
	name, _ := stringClaim(token, "name")
	picture, _ := stringClaim(token, "picture")
	hostedDomain, _ := stringClaim(token, "hd")
	emailVerified, _ := boolClaim(token, "email_verified")

	if subject == "" {
		return GoogleIdentity{}, errors.New("google subject claim is missing")
	}
	if email == "" {
		return GoogleIdentity{}, errors.New("google email claim is missing")
	}
	if !emailVerified {
		return GoogleIdentity{}, errors.New("google email is not verified")
	}
	if settings.HostedDomain != "" && !strings.EqualFold(settings.HostedDomain, hostedDomain) {
		return GoogleIdentity{}, shared.Forbidden("google hosted domain is not allowed")
	}
	return GoogleIdentity{
		Subject:       subject,
		Email:         strings.ToLower(strings.TrimSpace(email)),
		EmailVerified: emailVerified,
		Name:          name,
		PictureURL:    picture,
		HostedDomain:  hostedDomain,
		Issuer:        issuer,
	}, nil
}

func stringClaim(token jwt.Token, name string) (string, bool) {
	var value string
	if err := token.Get(name, &value); err != nil {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func boolClaim(token jwt.Token, name string) (bool, bool) {
	var value bool
	if err := token.Get(name, &value); err == nil {
		return value, true
	}
	var text string
	if err := token.Get(name, &text); err == nil {
		return strings.EqualFold(strings.TrimSpace(text), "true"), true
	}
	return false, false
}
