// Package workers runs background jobs from the Postgres queue and the
// periodic scheduler that enqueues them.
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dev-fritz/newsp/api/internal/models"
)

// JobStore is the persistence the queue needs.
type JobStore interface {
	Enqueue(ctx context.Context, j models.Job) (uuid.UUID, bool, error)
	Claim(ctx context.Context, workerID string, kinds []string, now time.Time) (models.Job, error)
	Touch(ctx context.Context, j models.Job, now time.Time) error
	Complete(ctx context.Context, j models.Job, result json.RawMessage, now time.Time) error
	Retry(ctx context.Context, j models.Job, errMsg string, runAt, now time.Time) error
	Fail(ctx context.Context, j models.Job, errMsg string, now time.Time) error
	RequeueStale(ctx context.Context, cutoff, now time.Time) (int64, error)
	DeleteFinishedBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// EnqueueOptions tune a job.
type EnqueueOptions struct {
	UserID      *uuid.UUID // owner, for status checks by the API
	DedupeKey   string     // at most one active job per key
	RunAt       time.Time  // zero means now
	Priority    int16      // higher runs first
	MaxAttempts int        // zero means the default
}

// DefaultMaxAttempts is used when EnqueueOptions.MaxAttempts is zero.
const DefaultMaxAttempts = 5

// Queue enqueues jobs. Services depend on it to schedule background work.
type Queue struct {
	store JobStore
	now   func() time.Time
}

// NewQueue builds a Queue.
func NewQueue(store JobStore, now func() time.Time) *Queue {
	return &Queue{store: store, now: now}
}

// Enqueue adds a job with a JSON payload. With a dedupe key, an equivalent
// active job is reused and its ID returned.
func (q *Queue) Enqueue(ctx context.Context, kind string, payload any, opts EnqueueOptions) (uuid.UUID, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("encode %s payload: %w", kind, err)
	}
	now := q.now()
	job := models.Job{
		ID: uuid.New(), Kind: kind, Payload: raw, UserID: opts.UserID, Priority: opts.Priority,
		RunAt: opts.RunAt, MaxAttempts: opts.MaxAttempts, CreatedAt: now,
	}
	if job.RunAt.IsZero() {
		job.RunAt = now
	}
	if job.MaxAttempts == 0 {
		job.MaxAttempts = DefaultMaxAttempts
	}
	if opts.DedupeKey != "" {
		job.DedupeKey = &opts.DedupeKey
	}
	id, _, err := q.store.Enqueue(ctx, job)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
