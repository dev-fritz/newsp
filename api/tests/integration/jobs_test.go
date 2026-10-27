package integration

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/workers"
	"github.com/dev-fritz/newsp/api/tests/apitest"
	"github.com/dev-fritz/newsp/api/tests/testdb"
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *clock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newQueue(t *testing.T) (*repositories.JobRepository, *workers.Queue, *clock) {
	t.Helper()
	jobs := repositories.NewJobRepository(testdb.New(t))
	clk := &clock{now: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)}
	return jobs, workers.NewQueue(jobs, clk.Now), clk
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestEnqueueDeduplicatesActiveJobs(t *testing.T) {
	t.Parallel()
	jobs, queue, clk := newQueue(t)
	ctx := context.Background()

	first, err := queue.Enqueue(ctx, "collect", map[string]string{"source": "hn"}, workers.EnqueueOptions{DedupeKey: "collect:hn"})
	require.NoError(t, err)
	second, err := queue.Enqueue(ctx, "collect", map[string]string{"source": "hn"}, workers.EnqueueOptions{DedupeKey: "collect:hn"})
	require.NoError(t, err)
	assert.Equal(t, first, second, "an active job with the same key is reused")

	job, err := jobs.Claim(ctx, "w1", []string{"collect"}, clk.Now())
	require.NoError(t, err)
	require.NoError(t, jobs.Complete(ctx, job, nil, clk.Now()))

	third, err := queue.Enqueue(ctx, "collect", map[string]string{"source": "hn"}, workers.EnqueueOptions{DedupeKey: "collect:hn"})
	require.NoError(t, err)
	assert.NotEqual(t, first, third, "finished jobs do not block new ones")
}

func TestClaimIsExclusiveUnderConcurrency(t *testing.T) {
	t.Parallel()
	jobs, queue, clk := newQueue(t)
	ctx := context.Background()

	const total = 40
	for i := range total {
		_, err := queue.Enqueue(ctx, "work", map[string]int{"n": i}, workers.EnqueueOptions{})
		require.NoError(t, err)
	}

	var (
		mu      sync.Mutex
		claimed = map[uuid.UUID]int{}
		wg      sync.WaitGroup
	)
	for w := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				job, err := jobs.Claim(ctx, "w", []string{"work"}, clk.Now())
				if errors.Is(err, repositories.ErrNotFound) {
					return
				}
				if !assert.NoError(t, err) {
					return
				}
				mu.Lock()
				claimed[job.ID]++
				mu.Unlock()
				_ = w
			}
		}()
	}
	wg.Wait()

	assert.Len(t, claimed, total)
	for id, n := range claimed {
		assert.Equal(t, 1, n, "job %s claimed more than once", id)
	}
}

func TestRunnerLifecycle(t *testing.T) {
	t.Parallel()
	jobs, queue, clk := newQueue(t)
	ctx := context.Background()
	runner := workers.NewRunner(jobs, workers.RunnerOptions{WorkerID: "test", LockTimeout: time.Minute}, discardLogger(), clk.Now)

	calls := map[string]int{}
	runner.Register("ok", workers.HandlerConfig{Handler: func(context.Context, models.Job) (any, error) {
		calls["ok"]++
		return map[string]int{"items": 3}, nil
	}})
	runner.Register("flaky", workers.HandlerConfig{Handler: func(context.Context, models.Job) (any, error) {
		calls["flaky"]++
		if calls["flaky"] == 1 {
			return nil, errors.New("upstream timeout")
		}
		return nil, nil
	}})
	runner.Register("bad", workers.HandlerConfig{Handler: func(context.Context, models.Job) (any, error) {
		return nil, workers.Permanent(errors.New("invalid payload"))
	}})
	runner.Register("always", workers.HandlerConfig{Handler: func(context.Context, models.Job) (any, error) {
		return nil, errors.New("still failing")
	}})
	kinds := runner.Kinds()

	okID, _ := queue.Enqueue(ctx, "ok", nil, workers.EnqueueOptions{})
	flakyID, _ := queue.Enqueue(ctx, "flaky", nil, workers.EnqueueOptions{})
	badID, _ := queue.Enqueue(ctx, "bad", nil, workers.EnqueueOptions{})
	alwaysID, _ := queue.Enqueue(ctx, "always", nil, workers.EnqueueOptions{MaxAttempts: 2})

	drain := func() {
		for {
			worked, err := runner.RunOnce(ctx, "test", kinds)
			require.NoError(t, err)
			if !worked {
				return
			}
		}
	}

	drain()
	get := func(id uuid.UUID) models.Job {
		j, err := jobs.GetByID(ctx, id)
		require.NoError(t, err)
		return j
	}
	assert.Equal(t, models.JobDone, get(okID).Status)
	assert.JSONEq(t, `{"items":3}`, string(get(okID).Result))
	assert.Equal(t, models.JobFailed, get(badID).Status, "permanent errors are not retried")
	assert.Equal(t, models.JobPending, get(flakyID).Status, "transient errors are retried later")
	assert.True(t, get(flakyID).RunAt.After(clk.Now()))
	assert.Equal(t, "upstream timeout", *get(flakyID).LastError)

	clk.Add(2 * time.Hour) // past any backoff
	drain()
	assert.Equal(t, models.JobDone, get(flakyID).Status)
	assert.Equal(t, models.JobFailed, get(alwaysID).Status, "max attempts reached")
	assert.Equal(t, 2, get(alwaysID).Attempts)
}

func TestReapStaleRequeuesAbandonedJobs(t *testing.T) {
	t.Parallel()
	jobs, queue, clk := newQueue(t)
	ctx := context.Background()
	runner := workers.NewRunner(jobs, workers.RunnerOptions{WorkerID: "test", LockTimeout: time.Minute}, discardLogger(), clk.Now)

	id, _ := queue.Enqueue(ctx, "slow", nil, workers.EnqueueOptions{})
	_, err := jobs.Claim(ctx, "dead-worker", []string{"slow"}, clk.Now())
	require.NoError(t, err)

	require.NoError(t, runner.ReapStale(ctx))
	j, _ := jobs.GetByID(ctx, id)
	assert.Equal(t, models.JobRunning, j.Status, "recent locks are left alone")

	clk.Add(2 * time.Minute)
	require.NoError(t, runner.ReapStale(ctx))
	j, _ = jobs.GetByID(ctx, id)
	assert.Equal(t, models.JobPending, j.Status)
}

func TestReapedOwnerCannotOverwriteTheNewOwner(t *testing.T) {
	t.Parallel()
	jobs, queue, clk := newQueue(t)
	ctx := context.Background()

	id, _ := queue.Enqueue(ctx, "slow", nil, workers.EnqueueOptions{})
	stale, err := jobs.Claim(ctx, "w1", []string{"slow"}, clk.Now())
	require.NoError(t, err)
	clk.Add(2 * time.Minute)
	_, err = jobs.RequeueStale(ctx, clk.Now().Add(-time.Minute), clk.Now())
	require.NoError(t, err)
	current, err := jobs.Claim(ctx, "w2", []string{"slow"}, clk.Now())
	require.NoError(t, err)
	require.Equal(t, id, current.ID)

	assert.ErrorIs(t, jobs.Complete(ctx, stale, nil, clk.Now()), repositories.ErrNotFound)
	assert.ErrorIs(t, jobs.Retry(ctx, stale, "late", clk.Now(), clk.Now()), repositories.ErrNotFound)
	assert.ErrorIs(t, jobs.Fail(ctx, stale, "late", clk.Now()), repositories.ErrNotFound)
	assert.ErrorIs(t, jobs.Touch(ctx, stale, clk.Now()), repositories.ErrNotFound)
	j, _ := jobs.GetByID(ctx, id)
	assert.Equal(t, models.JobRunning, j.Status)
	assert.Equal(t, "w2", *j.LockedBy, "the new owner keeps the job")

	require.NoError(t, jobs.Complete(ctx, current, nil, clk.Now()))
	j, _ = jobs.GetByID(ctx, id)
	assert.Equal(t, models.JobDone, j.Status)
}

func TestHeartbeatKeepsLongJobsLocked(t *testing.T) {
	t.Parallel()
	jobs, queue, clk := newQueue(t)
	ctx := context.Background()
	runner := workers.NewRunner(jobs, workers.RunnerOptions{
		WorkerID: "test", LockTimeout: time.Minute, HeartbeatInterval: 10 * time.Millisecond,
	}, discardLogger(), clk.Now)

	started, release := make(chan struct{}), make(chan struct{})
	runner.Register("long", workers.HandlerConfig{Handler: func(context.Context, models.Job) (any, error) {
		close(started)
		<-release
		return nil, nil
	}})
	id, _ := queue.Enqueue(ctx, "long", nil, workers.EnqueueOptions{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, err := runner.RunOnce(ctx, "test", runner.Kinds())
		assert.NoError(t, err)
	}()
	<-started

	clk.Add(10 * time.Minute) // far past LockTimeout since the claim
	require.Eventually(t, func() bool {
		j, err := jobs.GetByID(ctx, id)
		return err == nil && j.LockedAt != nil && j.LockedAt.Equal(clk.Now())
	}, 5*time.Second, 10*time.Millisecond, "the heartbeat renews the lock")
	require.NoError(t, runner.ReapStale(ctx))
	j, _ := jobs.GetByID(ctx, id)
	assert.Equal(t, models.JobRunning, j.Status, "a job with a live heartbeat is not requeued")

	close(release)
	<-finished
	j, _ = jobs.GetByID(ctx, id)
	assert.Equal(t, models.JobDone, j.Status)
}

func TestLostLeaseCancelsTheRun(t *testing.T) {
	t.Parallel()
	jobs, queue, clk := newQueue(t)
	ctx := context.Background()
	runner := workers.NewRunner(jobs, workers.RunnerOptions{
		WorkerID: "test", LockTimeout: time.Minute, HeartbeatInterval: 10 * time.Millisecond,
	}, discardLogger(), clk.Now)

	started := make(chan struct{})
	runner.Register("long", workers.HandlerConfig{Handler: func(ctx context.Context, _ models.Job) (any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}})
	id, _ := queue.Enqueue(ctx, "long", nil, workers.EnqueueOptions{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, err := runner.RunOnce(ctx, "test", runner.Kinds())
		assert.NoError(t, err)
	}()
	<-started

	// The reaper takes the job away, as if the lock had not been renewed in time.
	_, err := jobs.RequeueStale(ctx, clk.Now().Add(time.Hour), clk.Now())
	require.NoError(t, err)

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the run was not canceled after losing its lease")
	}
	j, _ := jobs.GetByID(ctx, id)
	assert.Equal(t, models.JobPending, j.Status, "the canceled run does not overwrite the requeued job")
	assert.Equal(t, "worker stopped while running the job", *j.LastError)
}

func TestBackoffGrowsAndCaps(t *testing.T) {
	t.Parallel()
	assert.GreaterOrEqual(t, workers.Backoff(1), 30*time.Second)
	assert.Less(t, workers.Backoff(1), 37*time.Second)
	assert.GreaterOrEqual(t, workers.Backoff(3), 2*time.Minute)
	assert.LessOrEqual(t, workers.Backoff(20), time.Hour+12*time.Minute)
}

func TestJobStatusVisibility(t *testing.T) {
	t.Parallel()
	s := apitest.New(t)
	admin, _ := setupAdmin(t, s)

	res := s.Do(http.MethodPost, "/api/v1/admin/invites", admin, map[string]string{"role": "member"})
	invite := tokenFromURL(t, res.Data()["url"].(string))
	res = s.Do(http.MethodPost, "/api/v1/invites/accept", "", map[string]string{
		"token": invite, "name": "Ana", "email": "ana@example.com", "password": strongPassword,
	})
	member, _ := tokens(res)
	memberID := uuid.MustParse(res.Data()["user"].(map[string]any)["id"].(string))

	ctx := context.Background()
	own, err := s.Container.Queue.Enqueue(ctx, "report.generate", nil, workers.EnqueueOptions{UserID: &memberID})
	require.NoError(t, err)
	system, err := s.Container.Queue.Enqueue(ctx, "collect", nil, workers.EnqueueOptions{})
	require.NoError(t, err)

	res = s.Do(http.MethodGet, "/api/v1/jobs/"+own.String(), member, nil)
	require.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, "pending", res.Data()["status"])

	res = s.Do(http.MethodGet, "/api/v1/jobs/"+system.String(), member, nil)
	assert.Equal(t, http.StatusNotFound, res.Status, "members cannot see system jobs")
	res = s.Do(http.MethodGet, "/api/v1/jobs/"+own.String(), admin, nil)
	assert.Equal(t, http.StatusNotFound, res.Status, "nobody else sees a user's job")
	res = s.Do(http.MethodGet, "/api/v1/jobs/"+system.String(), admin, nil)
	assert.Equal(t, http.StatusOK, res.Status)
}
