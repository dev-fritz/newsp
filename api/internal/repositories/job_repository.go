package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/models"
)

const jobColumns = `id, kind, payload, user_id, status, priority, run_at, attempts, max_attempts,
	last_error, result, dedupe_key, locked_by, locked_at, created_at, updated_at, finished_at`

// JobRepository is the Postgres-backed job queue.
type JobRepository struct {
	db *sql.DB
}

// NewJobRepository builds a JobRepository.
func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

func scanJob(row interface{ Scan(...any) error }) (models.Job, error) {
	var j models.Job
	var payload, result []byte
	err := row.Scan(&j.ID, &j.Kind, &payload, &j.UserID, &j.Status, &j.Priority, &j.RunAt, &j.Attempts, &j.MaxAttempts,
		&j.LastError, &result, &j.DedupeKey, &j.LockedBy, &j.LockedAt, &j.CreatedAt, &j.UpdatedAt, &j.FinishedAt)
	j.Payload, j.Result = payload, result
	return j, err
}

// Enqueue inserts a job. When an active job (pending or running) already has
// the same dedupe key, nothing is inserted and that job's ID is returned with
// created=false.
func (r *JobRepository) Enqueue(ctx context.Context, j models.Job) (id uuid.UUID, created bool, err error) {
	conn := database.Conn(ctx, r.db)
	const q = `
		INSERT INTO jobs (id, kind, payload, user_id, status, priority, run_at, max_attempts, dedupe_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9, $9)
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL AND status IN ('pending', 'running') DO NOTHING
		RETURNING id`
	err = conn.QueryRowContext(ctx, q, j.ID, j.Kind, []byte(j.Payload), j.UserID, j.Priority, j.RunAt, j.MaxAttempts, j.DedupeKey, j.CreatedAt).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) || j.DedupeKey == nil {
		return uuid.Nil, false, fmt.Errorf("enqueue job: %w", translate(err))
	}
	const existing = `SELECT id FROM jobs WHERE dedupe_key = $1 AND status IN ('pending', 'running')`
	if err := conn.QueryRowContext(ctx, existing, *j.DedupeKey).Scan(&id); err != nil {
		return uuid.Nil, false, fmt.Errorf("find deduplicated job: %w", translate(err))
	}
	return id, false, nil
}

// Claim locks the next runnable job of one of kinds for workerID. Concurrent
// workers never get the same job thanks to SKIP LOCKED. It returns ErrNotFound
// when the queue is empty.
func (r *JobRepository) Claim(ctx context.Context, workerID string, kinds []string, now time.Time) (models.Job, error) {
	q := `
		UPDATE jobs SET status = 'running', attempts = attempts + 1, locked_by = $1, locked_at = $2, updated_at = $2
		WHERE id = (
			SELECT id FROM jobs
			WHERE status = 'pending' AND run_at <= $2 AND kind = ANY($3)
			ORDER BY priority DESC, run_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING ` + jobColumns
	j, err := scanJob(database.Conn(ctx, r.db).QueryRowContext(ctx, q, workerID, now, kinds))
	if err != nil {
		return models.Job{}, fmt.Errorf("claim job: %w", translate(err))
	}
	return j, nil
}

// The calls below act on a job only while the caller still holds the lease it
// got from Claim: same worker and same attempt. A worker whose lease was
// reaped (and maybe claimed again) cannot overwrite the new owner's state;
// they return ErrNotFound instead.
const leaseHeld = `id = $1 AND status = 'running' AND locked_by = $2 AND attempts = $3`

func lease(j models.Job) (uuid.UUID, string, int) {
	owner := ""
	if j.LockedBy != nil {
		owner = *j.LockedBy
	}
	return j.ID, owner, j.Attempts
}

// Touch renews a running job's lock so the stale reaper leaves it alone.
func (r *JobRepository) Touch(ctx context.Context, j models.Job, now time.Time) error {
	id, owner, attempt := lease(j)
	q := `UPDATE jobs SET locked_at = $4, updated_at = $4 WHERE ` + leaseHeld
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, owner, attempt, now)
	return checkAffected(res, err, "touch job")
}

// Complete marks a running job as done with an optional JSON result.
func (r *JobRepository) Complete(ctx context.Context, j models.Job, result json.RawMessage, now time.Time) error {
	id, owner, attempt := lease(j)
	q := `UPDATE jobs SET status = 'done', result = $4, last_error = NULL, locked_by = NULL, locked_at = NULL,
		updated_at = $5, finished_at = $5 WHERE ` + leaseHeld
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, owner, attempt, nullableJSON(result), now)
	return checkAffected(res, err, "complete job")
}

// ClearPayload erases the payload of a job, for payloads that must not outlive
// the work (a sealed e-mail with a one-time link).
func (r *JobRepository) ClearPayload(ctx context.Context, id uuid.UUID) error {
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, `UPDATE jobs SET payload = '{}' WHERE id = $1`, id)
	return checkAffected(res, err, "clear job payload")
}

// Retry puts a failed attempt back in the queue at runAt.
func (r *JobRepository) Retry(ctx context.Context, j models.Job, errMsg string, runAt, now time.Time) error {
	id, owner, attempt := lease(j)
	q := `UPDATE jobs SET status = 'pending', last_error = $4, run_at = $5, locked_by = NULL, locked_at = NULL,
		updated_at = $6 WHERE ` + leaseHeld
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, owner, attempt, errMsg, runAt, now)
	return checkAffected(res, err, "retry job")
}

// Fail marks a job as permanently failed.
func (r *JobRepository) Fail(ctx context.Context, j models.Job, errMsg string, now time.Time) error {
	id, owner, attempt := lease(j)
	q := `UPDATE jobs SET status = 'failed', last_error = $4, locked_by = NULL, locked_at = NULL,
		updated_at = $5, finished_at = $5 WHERE ` + leaseHeld
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, id, owner, attempt, errMsg, now)
	return checkAffected(res, err, "fail job")
}

// RequeueStale returns jobs whose worker died (locked before cutoff) to the
// queue; jobs out of attempts are failed instead. It returns how many moved.
func (r *JobRepository) RequeueStale(ctx context.Context, cutoff, now time.Time) (int64, error) {
	const q = `
		UPDATE jobs SET
			status = CASE WHEN attempts >= max_attempts THEN 'failed' ELSE 'pending' END,
			finished_at = CASE WHEN attempts >= max_attempts THEN $2::timestamptz ELSE NULL END,
			last_error = 'worker stopped while running the job',
			locked_by = NULL, locked_at = NULL, run_at = $2, updated_at = $2
		WHERE status = 'running' AND locked_at < $1`
	res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, cutoff, now)
	if err != nil {
		return 0, fmt.Errorf("requeue stale jobs: %w", err)
	}
	return res.RowsAffected()
}

// GetByID returns a job.
func (r *JobRepository) GetByID(ctx context.Context, id uuid.UUID) (models.Job, error) {
	j, err := scanJob(database.Conn(ctx, r.db).QueryRowContext(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id = $1`, id))
	if err != nil {
		return models.Job{}, fmt.Errorf("get job: %w", translate(err))
	}
	return j, nil
}

// DeleteFinishedBefore removes finished jobs older than cutoff, in batches.
func (r *JobRepository) DeleteFinishedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	const q = `DELETE FROM jobs WHERE id IN (
		SELECT id FROM jobs WHERE finished_at IS NOT NULL AND finished_at < $1 LIMIT 1000)`
	var total int64
	for {
		res, err := database.Conn(ctx, r.db).ExecContext(ctx, q, cutoff)
		if err != nil {
			return total, fmt.Errorf("delete finished jobs: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, fmt.Errorf("delete finished jobs: %w", err)
		}
		total += n
		if n < 1000 {
			return total, nil
		}
	}
}

func nullableJSON(v json.RawMessage) any {
	if len(v) == 0 {
		return nil
	}
	return []byte(v)
}
