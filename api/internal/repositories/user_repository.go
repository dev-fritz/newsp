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

const userColumns = `id, email, name, password_hash, role, status, locale, last_seen_at, email_verified_at, created_at, updated_at`

// UserRepository persists users.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository builds a UserRepository.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func scanUser(row interface{ Scan(...any) error }) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.Status, &u.Locale, &u.LastSeenAt, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// Create inserts a user. A taken e-mail returns ErrDuplicate.
func (r *UserRepository) Create(ctx context.Context, u models.User) error {
	const q = `
		INSERT INTO users (id, email, name, password_hash, role, status, locale, email_verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`
	_, err := database.Conn(ctx, r.db).ExecContext(ctx, q, u.ID, u.Email, u.Name, u.PasswordHash, u.Role, u.Status, u.Locale, u.EmailVerifiedAt, u.CreatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", translate(err))
	}
	return nil
}

// GetByID returns a user or ErrNotFound.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	q := `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	u, err := scanUser(database.Conn(ctx, r.db).QueryRowContext(ctx, q, id))
	if err != nil {
		return models.User{}, fmt.Errorf("get user: %w", translate(err))
	}
	return u, nil
}

// GetByIDForUpdate locks the user row for the rest of the transaction.
func (r *UserRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (models.User, error) {
	q := `SELECT ` + userColumns + ` FROM users WHERE id = $1 FOR UPDATE`
	u, err := scanUser(database.Conn(ctx, r.db).QueryRowContext(ctx, q, id))
	if err != nil {
		return models.User{}, fmt.Errorf("get user for update: %w", translate(err))
	}
	return u, nil
}

// GetByEmail finds a user by e-mail, case-insensitively.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (models.User, error) {
	q := `SELECT ` + userColumns + ` FROM users WHERE lower(email) = lower($1)`
	u, err := scanUser(database.Conn(ctx, r.db).QueryRowContext(ctx, q, email))
	if err != nil {
		return models.User{}, fmt.Errorf("get user by email: %w", translate(err))
	}
	return u, nil
}

// List returns a page of users ordered by creation and the total count.
func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]models.User, int, error) {
	conn := database.Conn(ctx, r.db)
	var total int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	q := `SELECT ` + userColumns + ` FROM users ORDER BY created_at, id LIMIT $1 OFFSET $2`
	rows, err := conn.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	users := make([]models.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return users, total, nil
}

// LockActiveAdmins locks every active admin row and returns their IDs, so
// concurrent demotions cannot leave the instance without an admin.
func (r *UserRepository) LockActiveAdmins(ctx context.Context) ([]uuid.UUID, error) {
	const q = `SELECT id FROM users WHERE role = 'admin' AND status = 'active' ORDER BY id FOR UPDATE`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("lock admins: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan admin: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpdateAccess changes a user's role and status.
func (r *UserRepository) UpdateAccess(ctx context.Context, id uuid.UUID, role models.Role, status models.UserStatus, now time.Time) error {
	const q = `UPDATE users SET role = $2, status = $3, updated_at = $4 WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, role, status, now)
	return checkAffected(res, err, "update user access")
}

// UpdatePassword stores a new password hash.
func (r *UserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hash string, now time.Time) error {
	const q = `UPDATE users SET password_hash = $2, updated_at = $3 WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, hash, now)
	return checkAffected(res, err, "update password")
}

// TouchLastSeen records the last authentication time (sign-in or session
// refresh). It writes at most once an hour per user, since refreshes are frequent.
func (r *UserRepository) TouchLastSeen(ctx context.Context, id uuid.UUID, now time.Time) error {
	const q = `UPDATE users SET last_seen_at = $2
		WHERE id = $1 AND (last_seen_at IS NULL OR last_seen_at <= $2::timestamptz - interval '1 hour')`
	_, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, now)
	if err != nil {
		return fmt.Errorf("touch last seen: %w", err)
	}
	return nil
}

func checkAffected(res sql.Result, err error, op string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", op, translate(err))
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", op, ErrNotFound)
	}
	return nil
}

// MarkEmailVerified records that the user proved control of the address. The
// first verification time is kept if it is already set.
func (r *UserRepository) MarkEmailVerified(ctx context.Context, id uuid.UUID, now time.Time) error {
	const q = `UPDATE users SET email_verified_at = COALESCE(email_verified_at, $2), updated_at = $2 WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, now)
	return checkAffected(res, err, "mark email verified")
}

// UpdateLocale sets the interface language of a user.
func (r *UserRepository) UpdateLocale(ctx context.Context, id uuid.UUID, locale string, now time.Time) error {
	const q = `UPDATE users SET locale = $2, updated_at = $3 WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, locale, now)
	return checkAffected(res, err, "update locale")
}
