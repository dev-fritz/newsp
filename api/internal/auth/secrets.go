package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// Password limits. bcrypt silently ignores bytes past 72.
const (
	MinPasswordRunes = 12
	MaxPasswordBytes = 72
	bcryptCost       = 12
)

// ErrWeakPassword is returned when a password breaks the length rules.
var ErrWeakPassword = errors.New("password must have at least 12 characters and at most 72 bytes")

// NewOpaqueToken returns a random URL-safe token and the SHA-256 hash to store.
func NewOpaqueToken() (plain string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, HashToken(plain), nil
}

// HashToken hashes an opaque token for lookup. Tokens have 256 bits of
// entropy, so a fast hash is enough and lets the database index the value.
func HashToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}

// ValidatePassword enforces the password length rules.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordRunes || len(password) > MaxPasswordBytes {
		return ErrWeakPassword
	}
	return nil
}

// HashPassword validates and hashes a password with bcrypt.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

// CheckPassword compares a password with its bcrypt hash in constant time.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// DummyPasswordHash is compared against when the user does not exist, so login
// takes the same time either way and does not reveal which e-mails exist.
var DummyPasswordHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcryptCost)
	return string(h)
}()
