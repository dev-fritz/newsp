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

const sourceColumns = `id, kind, name, config, interval_minutes, enabled, next_run_at, last_run_at, last_success_at,
	last_error, consecutive_failures, secret_enc, created_at, updated_at`

// MaxSources caps the source lists; admins manage a handful of feeds.
const MaxSources = 500

// SourceStats is a source with the number of items collected in a window.
type SourceStats struct {
	Source models.Source
	Items  int
}

// SourceRepository persists sources.
type SourceRepository struct {
	db *sql.DB
}

// NewSourceRepository builds a SourceRepository.
func NewSourceRepository(db *sql.DB) *SourceRepository {
	return &SourceRepository{db: db}
}

func scanSource(row interface{ Scan(...any) error }, extra ...any) (models.Source, error) {
	var s models.Source
	var config []byte
	dest := append([]any{&s.ID, &s.Kind, &s.Name, &config, &s.IntervalMinutes, &s.Enabled, &s.NextRunAt, &s.LastRunAt,
		&s.LastSuccessAt, &s.LastError, &s.ConsecutiveFailures, &s.SecretEnc, &s.CreatedAt, &s.UpdatedAt}, extra...)
	err := row.Scan(dest...)
	s.Config = config
	return s, err
}

// Create inserts a source.
func (r *SourceRepository) Create(ctx context.Context, s models.Source) error {
	const q = `
		INSERT INTO sources (id, kind, name, config, interval_minutes, enabled, next_run_at, secret_enc, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`
	_, err := database.Conn(ctx, r.db).ExecContext(ctx, q, s.ID, s.Kind, s.Name, []byte(s.Config), s.IntervalMinutes, s.Enabled, s.NextRunAt,
		s.SecretEnc, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("create source: %w", translate(err))
	}
	return nil
}

// GetByID returns a source or ErrNotFound.
func (r *SourceRepository) GetByID(ctx context.Context, id uuid.UUID) (models.Source, error) {
	q := `SELECT ` + sourceColumns + ` FROM sources WHERE id = $1`
	s, err := scanSource(database.Conn(ctx, r.db).QueryRowContext(ctx, q, id))
	if err != nil {
		return models.Source{}, fmt.Errorf("get source: %w", translate(err))
	}
	return s, nil
}

// GetWithStats returns a source and how many of its items were collected since.
func (r *SourceRepository) GetWithStats(ctx context.Context, id uuid.UUID, since time.Time) (SourceStats, error) {
	q := `
		SELECT ` + sourceColumns + `,
			(SELECT count(*) FROM items i WHERE i.source_id = sources.id AND i.collected_at >= $2)
		FROM sources WHERE id = $1`
	var items int
	s, err := scanSource(database.Conn(ctx, r.db).QueryRowContext(ctx, q, id, since), &items)
	if err != nil {
		return SourceStats{}, fmt.Errorf("get source: %w", translate(err))
	}
	return SourceStats{Source: s, Items: items}, nil
}

// ListWithStats returns every source by name, each with the number of items
// collected since, computed in the same query.
func (r *SourceRepository) ListWithStats(ctx context.Context, since time.Time) ([]SourceStats, error) {
	const q = `
		SELECT s.id, s.kind, s.name, s.config, s.interval_minutes, s.enabled, s.next_run_at, s.last_run_at, s.last_success_at,
			s.last_error, s.consecutive_failures, s.secret_enc, s.created_at, s.updated_at, COALESCE(c.n, 0)
		FROM sources s
		LEFT JOIN (
			SELECT source_id, count(*) AS n FROM items WHERE collected_at >= $1 GROUP BY source_id
		) c ON c.source_id = s.id
		ORDER BY lower(s.name), s.id
		LIMIT $2`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, since, MaxSources)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []SourceStats
	for rows.Next() {
		var items int
		s, err := scanSource(rows, &items)
		if err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		out = append(out, SourceStats{Source: s, Items: items})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	return out, nil
}

// List returns every source by name, without statistics.
func (r *SourceRepository) List(ctx context.Context) ([]models.Source, error) {
	q := `SELECT ` + sourceColumns + ` FROM sources ORDER BY lower(name), id LIMIT $1`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, MaxSources)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.Source
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	return out, nil
}

// Update writes the admin-editable fields (name, config, secret, interval,
// enabled and the next run). Run bookkeeping is left alone so a running collection and an
// edit never overwrite each other.
func (r *SourceRepository) Update(ctx context.Context, s models.Source) error {
	const q = `
		UPDATE sources SET name = $2, config = $3, secret_enc = $4, interval_minutes = $5, enabled = $6, next_run_at = $7,
			updated_at = $8
		WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, s.ID, s.Name, []byte(s.Config), s.SecretEnc, s.IntervalMinutes, s.Enabled,
		s.NextRunAt, s.UpdatedAt)
	return checkAffected(res, err, "update source")
}

// Delete removes a source and its items. It returns ErrReferenced while one
// of its items is cited by a report or an idea.
func (r *SourceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, `DELETE FROM sources WHERE id = $1`, id)
	return checkAffected(res, err, "delete source")
}

// ListDue returns up to limit enabled sources whose next run is due, the
// most overdue first.
func (r *SourceRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	const q = `SELECT id FROM sources WHERE enabled AND next_run_at <= $1 ORDER BY next_run_at, id LIMIT $2`
	rows, err := database.Conn(ctx, r.db).QueryContext(ctx, q, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list due sources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan due source: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list due sources: %w", err)
	}
	return ids, nil
}

// MarkSuccess records a successful collection and schedules the next one.
func (r *SourceRepository) MarkSuccess(ctx context.Context, id uuid.UUID, now, nextRunAt time.Time) error {
	const q = `
		UPDATE sources SET last_run_at = $2, last_success_at = $2, last_error = NULL, consecutive_failures = 0, next_run_at = $3
		WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, now, nextRunAt)
	return checkAffected(res, err, "mark source success")
}

// MarkFailure records a failed collection, bumps the failure counter and
// schedules the retry.
func (r *SourceRepository) MarkFailure(ctx context.Context, id uuid.UUID, now time.Time, errMsg string, nextRunAt time.Time) error {
	const q = `
		UPDATE sources SET last_run_at = $2, last_error = $3, consecutive_failures = consecutive_failures + 1, next_run_at = $4
		WHERE id = $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, now, errMsg, nextRunAt)
	return checkAffected(res, err, "mark source failure")
}
