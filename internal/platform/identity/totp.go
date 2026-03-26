package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	totpDigits = 6
	totpPeriod = 30 * time.Second
)

func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(buf), "="), nil
}

func BuildTOTPURI(secret, issuer, accountName string) string {
	escapedIssuer := url.QueryEscape(strings.TrimSpace(issuer))
	escapedAccount := url.QueryEscape(strings.TrimSpace(accountName))
	label := escapedAccount
	if escapedIssuer != "" && escapedAccount != "" {
		label = escapedIssuer + ":" + escapedAccount
	}
	values := url.Values{}
	values.Set("secret", strings.TrimSpace(secret))
	if issuer != "" {
		values.Set("issuer", strings.TrimSpace(issuer))
	}
	values.Set("algorithm", "SHA1")
	values.Set("digits", strconv.Itoa(totpDigits))
	values.Set("period", strconv.Itoa(int(totpPeriod/time.Second)))
	return fmt.Sprintf("otpauth://totp/%s?%s", label, values.Encode())
}

func ValidateTOTP(secret, code string, now time.Time) bool {
	secret = strings.TrimSpace(secret)
	code = strings.TrimSpace(code)
	if secret == "" || len(code) != totpDigits {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	for _, offset := range []int64{-1, 0, 1} {
		if generateTOTPCode(secret, now.Add(time.Duration(offset)*totpPeriod)) == code {
			return true
		}
	}
	return false
}

func generateTOTPCode(secret string, at time.Time) string {
	normalized := strings.ToUpper(strings.TrimSpace(secret))
	if rem := len(normalized) % 8; rem != 0 {
		normalized += strings.Repeat("=", 8-rem)
	}
	key, err := base32.StdEncoding.DecodeString(normalized)
	if err != nil || len(key) == 0 {
		return ""
	}
	counter := uint64(at.UTC().Unix() / int64(totpPeriod/time.Second))
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
	modulo := 1
	for range totpDigits {
		modulo *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, truncated%modulo)
}
