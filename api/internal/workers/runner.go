package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
)

// Handler processes one job. It must be idempotent: a job can run again after
// a crash or a retry. The returned value is stored as the job result (JSON).
type Handler func(ctx context.Context, job models.Job) (any, error)

// HandlerConfig registers a handler for a job kind.
type HandlerConfig struct {
	Handler Handler
	Timeout time.Duration // per attempt
}

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent marks an error as not worth retrying (bad payload, missing row).
func Permanent(err error) error { return permanentError{err: err} }

// IsPermanent reports whether err, or an error it wraps, came from Permanent.
func IsPermanent(err error) bool {
	var perm permanentError
	return errors.As(err, &perm)
}

// RunnerOptions tune the consumer pool.
type RunnerOptions struct {
	WorkerID     string
	Concurrency  int
	PollInterval time.Duration // wait when the queue is empty
	LockTimeout  time.Duration // a running job whose lock was not renewed for this long is considered abandoned
	// HeartbeatInterval is how often a running job's lock is renewed; it must
	// be well under LockTimeout. Zero means a third of LockTimeout.
	HeartbeatInterval time.Duration
}

// Runner consumes jobs with a fixed pool of goroutines.
type Runner struct {
	store    JobStore
	handlers map[string]HandlerConfig
	opts     RunnerOptions
	logger   *slog.Logger
	now      func() time.Time
}

// NewRunner builds a Runner.
func NewRunner(store JobStore, opts RunnerOptions, logger *slog.Logger, now func() time.Time) *Runner {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = time.Second
	}
	if opts.LockTimeout <= 0 {
		opts.LockTimeout = 3 * time.Minute
	}
	if opts.HeartbeatInterval <= 0 {
		opts.HeartbeatInterval = opts.LockTimeout / 3
	}
	return &Runner{store: store, handlers: map[string]HandlerConfig{}, opts: opts, logger: logger, now: now}
}

// Register adds a handler for kind. Call before Run.
func (r *Runner) Register(kind string, cfg HandlerConfig) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	r.handlers[kind] = cfg
}

// Kinds lists the registered job kinds.
func (r *Runner) Kinds() []string {
	kinds := make([]string, 0, len(r.handlers))
	for k := range r.handlers {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

// Run consumes jobs until ctx is canceled, then waits for in-flight jobs.
// In-flight jobs keep a context detached from ctx (bounded by their timeout)
// so a shutdown lets them finish instead of failing halfway.
func (r *Runner) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := range r.opts.Concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.loop(ctx, fmt.Sprintf("%s-%d", r.opts.WorkerID, i))
		}()
	}
	wg.Wait()
}

func (r *Runner) loop(ctx context.Context, workerID string) {
	kinds := r.Kinds()
	for {
		if ctx.Err() != nil {
			return
		}
		worked, err := r.RunOnce(ctx, workerID, kinds)
		if err != nil {
			r.logger.ErrorContext(ctx, "job loop", "error", err)
		}
		if worked {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(r.opts.PollInterval):
		}
	}
}

// RunOnce claims and runs at most one job. It reports whether a job ran.
func (r *Runner) RunOnce(ctx context.Context, workerID string, kinds []string) (bool, error) {
	job, err := r.store.Claim(ctx, workerID, kinds, r.now())
	if errors.Is(err, repositories.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	r.execute(context.WithoutCancel(ctx), job)
	return true, nil
}

func (r *Runner) execute(ctx context.Context, job models.Job) {
	cfg := r.handlers[job.Kind]
	log := r.logger.With("job_id", job.ID, "kind", job.Kind, "attempt", job.Attempts)
	start := r.now()

	runCtx, cancel := context.WithCancel(ctx)
	stopHeartbeat := r.heartbeat(runCtx, cancel, job, log)
	result, err := r.call(runCtx, cfg, job)
	stopHeartbeat()
	cancel()

	var raw json.RawMessage
	if err == nil && result != nil {
		var encErr error
		if raw, encErr = json.Marshal(result); encErr != nil {
			err = Permanent(fmt.Errorf("encode result: %w", encErr))
		}
	}
	if err == nil {
		if cerr := r.store.Complete(ctx, job, raw, r.now()); cerr != nil {
			// The job stays "running"; the stale reaper will retry it later.
			logStoreError(ctx, log, "complete job", cerr)
			return
		}
		log.InfoContext(ctx, "job done", "duration", r.now().Sub(start))
		return
	}

	if IsPermanent(err) || job.Attempts >= job.MaxAttempts {
		log.ErrorContext(ctx, "job failed", "error", err)
		if ferr := r.store.Fail(ctx, job, truncate(err.Error()), r.now()); ferr != nil {
			logStoreError(ctx, log, "mark job failed", ferr)
		}
		return
	}
	retryAt := r.now().Add(Backoff(job.Attempts))
	log.WarnContext(ctx, "job attempt failed, will retry", "error", err, "retry_at", retryAt)
	if rerr := r.store.Retry(ctx, job, truncate(err.Error()), retryAt, r.now()); rerr != nil {
		logStoreError(ctx, log, "schedule retry", rerr)
	}
}

// logStoreError reports a failed state change. ErrNotFound means the lease was
// lost: the reaper requeued the job, so this attempt's outcome is discarded.
func logStoreError(ctx context.Context, log *slog.Logger, op string, err error) {
	if errors.Is(err, repositories.ErrNotFound) {
		log.WarnContext(ctx, "job lease lost, outcome discarded", "op", op)
		return
	}
	log.ErrorContext(ctx, op, "error", err)
}

// heartbeat renews the job's lock every HeartbeatInterval until stop is
// called, so long jobs are not mistaken for abandoned ones. If the lease is
// lost anyway (the lock could not be renewed for LockTimeout and the reaper
// requeued the job), it cancels the run so two workers do not do the same work.
func (r *Runner) heartbeat(ctx context.Context, cancel context.CancelFunc, job models.Job, log *slog.Logger) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(r.opts.HeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := r.store.Touch(ctx, job, r.now())
				if errors.Is(err, repositories.ErrNotFound) {
					log.WarnContext(ctx, "job lease lost, canceling the run")
					cancel()
					return
				}
				if err != nil {
					log.ErrorContext(ctx, "renew job lock", "error", err)
				}
			}
		}
	}()
	return func() {
		close(done)
		wg.Wait()
	}
}

// call runs the handler with a timeout, turning panics and unknown kinds into errors.
func (r *Runner) call(ctx context.Context, cfg HandlerConfig, job models.Job) (result any, err error) {
	if cfg.Handler == nil {
		return nil, Permanent(fmt.Errorf("no handler for job kind %q", job.Kind))
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			err = Permanent(fmt.Errorf("handler panic: %v", p))
		}
	}()
	return cfg.Handler(ctx, job)
}

// ReapStale requeues jobs abandoned by dead workers.
func (r *Runner) ReapStale(ctx context.Context) error {
	now := r.now()
	n, err := r.store.RequeueStale(ctx, now.Add(-r.opts.LockTimeout), now)
	if err != nil {
		return err
	}
	if n > 0 {
		r.logger.WarnContext(ctx, "requeued abandoned jobs", "count", n)
	}
	return nil
}

// Backoff is the delay before retry number attempt (1-based): 30s doubled per
// attempt, capped at one hour, with up to 20% jitter to spread retries.
func Backoff(attempt int) time.Duration {
	d := 30 * time.Second << min(attempt-1, 7)
	d = min(d, time.Hour)
	jitter := time.Duration(rand.Int64N(int64(d / 5))) //nolint:gosec // jitter does not need a CSPRNG
	return d + jitter
}

func truncate(s string) string {
	const limit = 2000
	if len(s) > limit {
		return s[:limit]
	}
	return s
}
