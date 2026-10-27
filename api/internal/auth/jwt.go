package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/models"
)

const (
	issuer   = "newsp"
	audience = "newsp-api"
)

// ErrInvalidToken is returned for any access token that fails verification.
var ErrInvalidToken = errors.New("invalid access token")

type claims struct {
	Role models.Role `json:"role"`
	jwt.RegisteredClaims
}

// TokenIssuer signs and verifies short-lived access tokens (HS256).
type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewTokenIssuer builds a TokenIssuer.
func NewTokenIssuer(secret []byte, ttl time.Duration, now func() time.Time) *TokenIssuer {
	return &TokenIssuer{secret: secret, ttl: ttl, now: now}
}

// Issue signs an access token for user.
func (i *TokenIssuer) Issue(userID uuid.UUID, role models.Role) (string, time.Time, error) {
	now := i.now()
	exp := now.Add(i.ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
	})
	signed, err := token.SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, exp, nil
}

// Parse verifies the token with a fixed algorithm and returns its identity.
func (i *TokenIssuer) Parse(raw string) (Identity, error) {
	var c claims
	_, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return i.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithIssuedAt(),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(i.now),
	)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	userID, err := uuid.Parse(c.Subject)
	if err != nil || (c.Role != models.RoleAdmin && c.Role != models.RoleMember) {
		return Identity{}, ErrInvalidToken
	}
	return Identity{UserID: userID, Role: c.Role}, nil
}
