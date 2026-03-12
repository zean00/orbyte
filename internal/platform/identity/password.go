package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordArgon2Time    uint32 = 1
	passwordArgon2Memory  uint32 = 64 * 1024
	passwordArgon2Threads uint8  = 4
	passwordArgon2KeyLen  uint32 = 32
	passwordSaltLen              = 16
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, passwordArgon2Time, passwordArgon2Memory, passwordArgon2Threads, passwordArgon2KeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		passwordArgon2Memory,
		passwordArgon2Time,
		passwordArgon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(encodedHash, password string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("invalid password hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, err
	}
	if version != argon2.Version {
		return false, fmt.Errorf("unsupported argon2 version")
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false, fmt.Errorf("invalid password hash parameters")
	}
	memory, err := parseUint32Param(params[0], "m=")
	if err != nil {
		return false, err
	}
	iterations, err := parseUint32Param(params[1], "t=")
	if err != nil {
		return false, err
	}
	threads, err := parseUint8Param(params[2], "p=")
	if err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	computed := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(decodedHash)))
	return subtle.ConstantTimeCompare(decodedHash, computed) == 1, nil
}

func parseUint32Param(value, prefix string) (uint32, error) {
	raw := strings.TrimPrefix(value, prefix)
	parsed, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(parsed), nil
}

func parseUint8Param(value, prefix string) (uint8, error) {
	raw := strings.TrimPrefix(value, prefix)
	parsed, err := strconv.ParseUint(raw, 10, 8)
	if err != nil {
		return 0, err
	}
	return uint8(parsed), nil
}
