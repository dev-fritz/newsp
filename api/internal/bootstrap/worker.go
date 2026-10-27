package bootstrap

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/internal/workers"
)

// finishedJobRetention is how long finished jobs stay queryable.
const finishedJobRetention = 7 * 24 * time.Hour

// NewWorker builds the job runner and the scheduler with every job handler
// and periodic task registered.
func NewWorker(c *Container) (*workers.Runner, *workers.Scheduler) {
	host, _ := os.Hostname()
	runner := workers.NewRunner(c.Jobs, workers.RunnerOptions{
		WorkerID:    fmt.Sprintf("%s-%d", host, os.Getpid()),
		Concurrency: 4,
	}, c.Logger, c.Now)

	scheduler := workers.NewScheduler(c.Logger, c.Now, 15*time.Second)
	scheduler.Add(workers.Task{
		Name:     "requeue-abandoned-jobs",
		Interval: time.Minute,
		Run:      func(ctx context.Context, _ time.Time) error { return runner.ReapStale(ctx) },
	})
	scheduler.Add(workers.Task{
		Name:     "delete-finished-jobs",
		Interval: time.Hour,
		Run: func(ctx context.Context, now time.Time) error {
			_, err := c.Jobs.DeleteFinishedBefore(ctx, now.Add(-finishedJobRetention))
			return err
		},
	})

	runner.Register(services.JobCollectSource, workers.HandlerConfig{Handler: c.Sources.CollectSource, Timeout: 3 * time.Minute})
	runner.Register(services.JobEmbedItems, workers.HandlerConfig{Handler: c.Sources.EmbedPending, Timeout: 10 * time.Minute})
	scheduler.Add(workers.Task{
		Name:     "enqueue-due-sources",
		Interval: 30 * time.Second,
		Run:      c.Sources.EnqueueDue,
	})

	// AI generations can take minutes on local models.
	runner.Register(services.JobGenerateReport, workers.HandlerConfig{Handler: c.Reports.Generate, Timeout: 20 * time.Minute})
	runner.Register(services.JobGenerateIdeas, workers.HandlerConfig{Handler: c.Ideas.Generate, Timeout: 30 * time.Minute})
	runner.Register(services.JobDeepenIdea, workers.HandlerConfig{Handler: c.Ideas.Deepen, Timeout: 20 * time.Minute})
	scheduler.Add(workers.Task{
		Name:     "enqueue-due-reports",
		Interval: time.Minute,
		Run:      c.Reports.EnqueueDue,
	})
	scheduler.Add(workers.Task{
		Name:     "fail-orphaned-reports",
		Interval: time.Minute,
		Run:      c.Reports.ReconcileOrphans,
	})

	runner.Register(services.JobPasswordReset, workers.HandlerConfig{Handler: c.Auth.SendPasswordReset, Timeout: time.Minute})
	runner.Register(services.JobSendEmail, workers.HandlerConfig{Handler: c.Emails.Deliver, Timeout: time.Minute})

	runner.Register(services.JobRetention, workers.HandlerConfig{Handler: c.Maintenance.Retention, Timeout: 10 * time.Minute})
	runner.Register(services.JobReindexEmbeddings, workers.HandlerConfig{Handler: c.Maintenance.Reindex, Timeout: 30 * time.Minute})
	scheduler.Add(workers.Task{
		Name:     "enqueue-retention",
		Interval: 6 * time.Hour,
		Run:      c.Maintenance.EnqueueRetention,
	})

	return runner, scheduler
}
