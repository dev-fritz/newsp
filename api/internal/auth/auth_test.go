package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/models"
)

var secret = []byte("test-secret-with-at-least-32-bytes-000")

func TestTokenIssuerRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	issuer := NewTokenIssuer(secret, 15*time.Minute, func() time.Time { return now })
	userID := uuid.New()

	token, exp, err := issuer.Issue(userID, models.RoleAdmin)
	require.NoError(t, err)
	assert.Equal(t, now.Add(15*time.Minute), exp)

	id, err := issuer.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, Identity{UserID: userID, Role: models.RoleAdmin}, id)
}

func TestTokenIssuerRejects(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	issuer := NewTokenIssuer(secret, 15*time.Minute, clock)
	valid, _, err := issuer.Issue(uuid.New(), models.RoleMember)
	require.NoError(t, err)

	later := NewTokenIssuer(secret, 15*time.Minute, func() time.Time { return now.Add(16 * time.Minute) })
	other := NewTokenIssuer([]byte("another-secret-with-at-least-32-bytes"), 15*time.Minute, clock)

	none := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": uuid.NewString(), "role": "admin"})
	noneToken, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	hs512 := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"sub": uuid.NewString(), "role": "admin", "iss": "newsp", "aud": "newsp-api",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	hs512Token, err := hs512.SignedString(secret)
	require.NoError(t, err)

	tests := []struct {
		name   string
		issuer *TokenIssuer
		token  string
	}{
		{name: "expired", issuer: later, token: valid},
		{name: "wrong secret", issuer: other, token: valid},
		{name: "alg none", issuer: issuer, token: noneToken},
		{name: "unexpected algorithm", issuer: issuer, token: hs512Token},
		{name: "tampered", issuer: issuer, token: valid[:len(valid)-2] + "xx"},
		{name: "garbage", issuer: issuer, token: "not-a-jwt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := tt.issuer.Parse(tt.token)
			assert.ErrorIs(t, err, ErrInvalidToken)
		})
	}
}

func TestOpaqueToken(t *testing.T) {
	t.Parallel()
	plain, hash, err := NewOpaqueToken()
	require.NoError(t, err)
	assert.Len(t, plain, 43)
	assert.Equal(t, hash, HashToken(plain))

	other, _, err := NewOpaqueToken()
	require.NoError(t, err)
	assert.NotEqual(t, plain, other)
}

func TestPasswords(t *testing.T) {
	t.Parallel()
	assert.ErrorIs(t, ValidatePassword("short"), ErrWeakPassword)
	assert.ErrorIs(t, ValidatePassword(strings.Repeat("a", 73)), ErrWeakPassword)
	assert.NoError(t, ValidatePassword("correct horse battery"))

	hash, err := HashPassword("correct horse battery")
	require.NoError(t, err)
	assert.True(t, CheckPassword(hash, "correct horse battery"))
	assert.False(t, CheckPassword(hash, "wrong horse battery"))
}

func TestPermissions(t *testing.T) {
	t.Parallel()
	admin := Identity{Role: models.RoleAdmin}
	member := Identity{Role: models.RoleMember}
	assert.True(t, Can(admin, PermUsersManage))
	assert.False(t, Can(member, PermUsersManage))
	assert.Empty(t, Permissions(models.RoleMember))
	assert.Len(t, Permissions(models.RoleAdmin), 5)
}
