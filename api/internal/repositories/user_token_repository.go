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

const userTokenColumns = `id, kind, token_hash, user_id, role, email, created_by, expires_at, used_at, revoked_at, created_at` //nolint:gosec // column list, not a credential

// UserTokenRepository persists one-time links (invites and password resets).
type UserTokenRepository struct {
	db *sql.DB
}

// NewUserTokenRepository builds a UserTokenRepository.
func NewUserTokenRepository(db *sql.DB) *UserTokenRepository {
	return &UserTokenRepository{db: db}
}

func scanUserToken(row interface{ Scan(...any) error }) (models.UserToken, error) {
	var t models.UserToken
	err := row.Scan(&t.ID, &t.Kind, &t.TokenHash, &t.UserID, &t.Role, &t.Email, &t.CreatedBy, &t.ExpiresAt, &t.UsedAt, &t.RevokedAt, &t.CreatedAt)
	return t, err
}

// Create inserts a token.
func (r *UserTokenRepository) Create(ctx context.Context, t models.UserToken) error {
	const q = `
		INSERT INTO user_tokens (id, kind, token_hash, user_id, role, email, created_by, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := database.Conn(ctx, r.db).ExecContext(ctx, q, t.ID, t.Kind, t.TokenHash, t.UserID, t.Role, t.Email, t.CreatedBy, t.ExpiresAt, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("create user token: %w", translate(err))
	}
	return nil
}

// GetUsableForUpdate returns an unused, unrevoked, unexpired token of kind and
// locks it, so a link cannot be consumed twice concurrently.
func (r *UserTokenRepository) GetUsableForUpdate(ctx context.Context, kind models.UserTokenKind, hash []byte, now time.Time) (models.UserToken, error) {
	q := `SELECT ` + userTokenColumns + ` FROM user_tokens
		WHERE kind = $1 AND token_hash = $2 AND used_at IS NULL AND revoked_at IS NULL AND expires_at > $3
		FOR UPDATE`
	t, err := scanUserToken(database.Conn(ctx, r.db).QueryRowContext(ctx, q, kind, hash, now))
	if err != nil {
		return models.UserToken{}, fmt.Errorf("get user token: %w", translate(err))
	}
	return t, nil
}

// GetUsable returns a usable token without locking (for read-only checks).
func (r *UserTokenRepository) GetUsable(ctx context.Context, kind models.UserTokenKind, hash []byte, now time.Time) (models.UserToken, error) {
	q := `SELECT ` + userTokenColumns + ` FROM user_tokens
		WHERE kind = $1 AND token_hash = $2 AND used_at IS NULL AND revoked_at IS NULL AND expires_at > $3`
	t, err := scanUserToken(database.Conn(ctx, r.db).QueryRowContext(ctx, q, kind, hash, now))
	if err != nil {
		return models.UserToken{}, fmt.Errorf("get user token: %w", translate(err))
	}
	return t, nil
}

// MarkUsed consumes a token.
func (r *UserTokenRepository) MarkUsed(ctx context.Context, id uuid.UUID, now time.Time) error {
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, `UPDATE user_tokens SET used_at = $2 WHERE id = $1 AND used_at IS NULL`, id, now)
	return checkAffected(res, err, "mark token used")
}

// RevokeInvite revokes a pending invite.
func (r *UserTokenRepository) RevokeInvite(ctx context.Context, id uuid.UUID, now time.Time) error {
	const q = `UPDATE user_tokens SET revoked_at = $2
		WHERE id = $1 AND kind = 'invite' AND used_at IS NULL AND revoked_at IS NULL`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, now)
	return checkAffected(res, err, "revoke invite")
}

// RevokePendingResets revokes every unused password reset of a user.
func (r *UserTokenRepository) RevokePendingResets(ctx context.Context, userID uuid.UUID, now time.Time) error {
	const q = `UPDATE user_tokens SET revoked_at = $2
		WHERE user_id = $1 AND kind = 'password_reset' AND used_at IS NULL AND revoked_at IS NULL`
	if _, err := database.Conn(ctx, r.db).ExecContext(ctx, q, userID, now); err != nil {
		return fmt.Errorf("revoke pending resets: %w", err)
	}
	return nil
}

// RevokePendingSelfResets revokes the unused password resets a user requested
// for themselves, leaving the ones an admin created.
func (r *UserTokenRepository) RevokePendingSelfResets(ctx context.Context, userID uuid.UUID, now time.Time) error {
	const q = `UPDATE user_tokens SET revoked_at = $2
		WHERE user_id = $1 AND kind = 'password_reset' AND created_by IS NULL AND used_at IS NULL AND revoked_at IS NULL`
	if _, err := database.Conn(ctx, r.db).ExecContext(ctx, q, userID, now); err != nil {
		return fmt.Errorf("revoke pending self resets: %w", err)
	}
	return nil
}

// ListPendingInvites returns a page of invites that can still be accepted.
func (r *UserTokenRepository) ListPendingInvites(ctx context.Context, now time.Time, limit, offset int) ([]models.UserToken, int, error) {
	conn := database.Conn(ctx, r.db)
	const where = `kind = 'invite' AND used_at IS NULL AND revoked_at IS NULL AND expires_at > $1`

	var total int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM user_tokens WHERE `+where, now).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count invites: %w", err)
	}
	q := `SELECT ` + userTokenColumns + ` FROM user_tokens WHERE ` + where + ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := conn.QueryContext(ctx, q, now, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list invites: %w", err)
	}
	defer func() { _ = rows.Close() }()
	tokens := make([]models.UserToken, 0)
	for rows.Next() {
		t, err := scanUserToken(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan invite: %w", err)
		}
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list invites: %w", err)
	}
	return tokens, total, nil
}
