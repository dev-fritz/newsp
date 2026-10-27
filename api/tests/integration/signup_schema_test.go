package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/tests/testdb"
)

func TestSignupMigrationBackfillsVerifiedEmail(t *testing.T) {
	t.Parallel()
	db, url := testdb.NewEmpty(t)
	ctx := context.Background()

	m, err := database.NewMigrator(url)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(9))
	_, _ = m.Close()

	id := uuid.New()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	mustExec(t, db, `INSERT INTO users (id, email, name, password_hash, role, created_at) VALUES ($1, 'a@b.c', 'A', 'x', 'admin', $2)`, id, created)

	require.NoError(t, database.MigrateUp(url))

	var verified time.Time
	require.NoError(t, db.QueryRowContext(ctx, `SELECT email_verified_at FROM users WHERE id = $1`, id).Scan(&verified))
	assert.True(t, created.Equal(verified), "existing accounts count as verified at creation time")

	// Down removes the new tokens and restores the original constraints.
	m, err = database.NewMigrator(url)
	require.NoError(t, err)
	mustExec(t, db, `INSERT INTO user_tokens (id, kind, token_hash, user_id, expires_at) VALUES ($1, 'email_verification', '\x01', $2, now())`, uuid.New(), id)
	require.NoError(t, m.Migrate(9)) // back to before 000010, whatever the latest version is
	_, _ = m.Close()
	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM user_tokens`).Scan(&n))
	assert.Zero(t, n)
	_, err = db.ExecContext(ctx, `INSERT INTO user_tokens (id, kind, token_hash, user_id, expires_at) VALUES ($1, 'email_verification', '\x02', $2, now())`, uuid.New(), id)
	assert.Equal(t, "23514", pgCode(t, err))
	require.NoError(t, database.MigrateUp(url))
}

func TestEmailVerificationTokenConstraints(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()

	userID := uuid.New()
	mustExec(t, db, `INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, 'a@b.c', 'A', 'x', 'member')`, userID)

	const q = `INSERT INTO user_tokens (id, kind, token_hash, user_id, role, email, expires_at) VALUES ($1, $2, $3, $4, $5, $6, now())`
	_, err := db.ExecContext(ctx, q, uuid.New(), "email_verification", []byte{1}, userID, nil, nil)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, q, uuid.New(), "email_verification", []byte{2}, nil, nil, nil)
	assert.Equal(t, "23514", pgCode(t, err), "email_verification requires a user")

	_, err = db.ExecContext(ctx, q, uuid.New(), "email_verification", []byte{3}, userID, "member", nil)
	assert.Equal(t, "23514", pgCode(t, err), "email_verification forbids a role")

	_, err = db.ExecContext(ctx, q, uuid.New(), "invite", []byte{4}, nil, "member", "")
	assert.Equal(t, "23514", pgCode(t, err), "an empty e-mail is rejected")
}

func TestInstanceSettingsAccountDefaults(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()

	s, err := repositories.NewSettingsRepository(db).Get(ctx)
	require.NoError(t, err)
	assert.False(t, s.SignupEnabled)
	assert.Equal(t, 30, s.InactivityPauseDays)

	for _, days := range []int{-1, 3651} {
		_, err = db.ExecContext(ctx, `UPDATE instance_settings SET inactivity_pause_days = $1`, days)
		assert.Equal(t, "23514", pgCode(t, err))
	}
	_, err = db.ExecContext(ctx, `UPDATE instance_settings SET inactivity_pause_days = 0`)
	assert.NoError(t, err, "0 disables the pause")
}

func TestSignupRepositoriesRoundTrip(t *testing.T) {
	t.Parallel()
	db := testdb.New(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	users := repositories.NewUserRepository(db)
	tokens := repositories.NewUserTokenRepository(db)
	settings := repositories.NewSettingsRepository(db)

	u := models.User{
		ID: uuid.New(), Email: "ana@example.com", Name: "Ana", PasswordHash: "x",
		Role: models.RoleMember, Status: models.UserActive, Locale: "pt-BR", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, users.Create(ctx, u))
	got, err := users.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Nil(t, got.EmailVerifiedAt)

	verifiedAt := now.Add(time.Minute)
	require.NoError(t, users.MarkEmailVerified(ctx, u.ID, verifiedAt))
	got, err = users.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.NotNil(t, got.EmailVerifiedAt)
	assert.True(t, verifiedAt.Equal(*got.EmailVerifiedAt))

	require.NoError(t, users.MarkEmailVerified(ctx, u.ID, verifiedAt.Add(time.Hour)))
	got, err = users.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.True(t, verifiedAt.Equal(*got.EmailVerifiedAt), "the first verification time is kept")

	assert.ErrorIs(t, users.MarkEmailVerified(ctx, uuid.New(), now), repositories.ErrNotFound)

	created := models.User{
		ID: uuid.New(), Email: "bia@example.com", Name: "Bia", PasswordHash: "x",
		Role: models.RoleMember, Status: models.UserActive, Locale: "en", EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, users.Create(ctx, created))
	got, err = users.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got.EmailVerifiedAt)
	assert.True(t, now.Equal(*got.EmailVerifiedAt))

	role, email := models.RoleMember, "new@example.com"
	invite := models.UserToken{
		ID: uuid.New(), Kind: models.TokenInvite, TokenHash: []byte{1}, Role: &role, Email: &email,
		ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}
	require.NoError(t, tokens.Create(ctx, invite))
	gotToken, err := tokens.GetUsable(ctx, models.TokenInvite, []byte{1}, now)
	require.NoError(t, err)
	require.NotNil(t, gotToken.Email)
	assert.Equal(t, email, *gotToken.Email)

	verification := models.UserToken{
		ID: uuid.New(), Kind: models.TokenEmailVerification, TokenHash: []byte{2}, UserID: &u.ID,
		ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}
	require.NoError(t, tokens.Create(ctx, verification))
	gotToken, err = tokens.GetUsable(ctx, models.TokenEmailVerification, []byte{2}, now)
	require.NoError(t, err)
	assert.Nil(t, gotToken.Email)

	s, err := settings.Get(ctx)
	require.NoError(t, err)
	s.SignupEnabled, s.InactivityPauseDays, s.UpdatedAt = true, 7, now
	require.NoError(t, settings.UpdateAccounts(ctx, s))
	s, err = settings.Get(ctx)
	require.NoError(t, err)
	assert.True(t, s.SignupEnabled)
	assert.Equal(t, 7, s.InactivityPauseDays)
}
