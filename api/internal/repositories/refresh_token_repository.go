package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
)

// RefreshTokenRepository persists hashed refresh tokens.
type RefreshTokenRepository struct {
	db *sql.DB
}

// NewRefreshTokenRepository builds a RefreshTokenRepository.
func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// Create inserts a refresh token.
func (r *RefreshTokenRepository) Create(ctx context.Context, t models.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := database.Conn(ctx, r.db).ExecContext(ctx, q, t.ID, t.UserID, t.FamilyID, t.TokenHash, t.ExpiresAt, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", translate(err))
	}
	return nil
}

// GetByHashForUpdate returns a token by hash, whatever its state, and locks it.
func (r *RefreshTokenRepository) GetByHashForUpdate(ctx context.Context, hash []byte) (models.RefreshToken, error) {
	const q = `
		SELECT id, user_id, family_id, token_hash, expires_at, rotated_at, revoked_at, created_at
		FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`
	var t models.RefreshToken
	err := database.Conn(ctx, r.db).QueryRowContext(ctx, q, hash).
		Scan(&t.ID, &t.UserID, &t.FamilyID, &t.TokenHash, &t.ExpiresAt, &t.RotatedAt, &t.RevokedAt, &t.CreatedAt)
	if err != nil {
		return models.RefreshToken{}, fmt.Errorf("get refresh token: %w", translate(err))
	}
	return t, nil
}

// MarkRotated records that a token was exchanged for a new one.
func (r *RefreshTokenRepository) MarkRotated(ctx context.Context, id uuid.UUID, now time.Time) error {
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, `UPDATE refresh_tokens SET rotated_at = $2 WHERE id = $1`, id, now)
	return checkAffected(res, err, "rotate refresh token")
}

// RevokeFamily revokes every token descending from the same login.
func (r *RefreshTokenRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID, now time.Time) error {
	const q = `UPDATE refresh_tokens SET revoked_at = $2 WHERE family_id = $1 AND revoked_at IS NULL`
	if _, err := database.Conn(ctx, r.db).ExecContext(ctx, q, familyID, now); err != nil {
		return fmt.Errorf("revoke token family: %w", err)
	}
	return nil
}

// RevokeAllForUser ends every session of a user.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, now time.Time) error {
	const q = `UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`
	if _, err := database.Conn(ctx, r.db).ExecContext(ctx, q, userID, now); err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}
