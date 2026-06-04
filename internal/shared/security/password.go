package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"

	"golang.org/x/crypto/argon2"
)

const (
	saltSize = 16
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return base64.RawStdEncoding.EncodeToString(salt) + ":" + base64.RawStdEncoding.EncodeToString(hash), nil
}

func CheckPassword(password, encoded string) error {
	parts := splitOnce(encoded, ":")
	if len(parts) != 2 {
		return errors.New("invalid password hash")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return err
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	got := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	if subtle.ConstantTimeCompare(got, expected) != 1 {
		return errors.New("invalid credentials")
	}
	return nil
}

func splitOnce(s, sep string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
