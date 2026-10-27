package workers

import (
	"context"
	"log/slog"
	"time"
)

// Task is a periodic function, e.g. "enqueue the sources that are due".
// Tasks must be safe to run on several worker replicas at once; enqueueing
// with dedupe keys gives that for free.
type Task struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context, now time.Time) error
}

// Scheduler runs periodic tasks until its context is canceled.
type Scheduler struct {
	tasks  []Task
	logger *slog.Logger
	now    func() time.Time
	tick   time.Duration
}

// NewScheduler builds a Scheduler that checks its tasks every tick.
func NewScheduler(logger *slog.Logger, now func() time.Time, tick time.Duration) *Scheduler {
	if tick <= 0 {
		tick = 15 * time.Second
	}
	return &Scheduler{logger: logger, now: now, tick: tick}
}

// Add registers a task. Call before Run.
func (s *Scheduler) Add(t Task) { s.tasks = append(s.tasks, t) }

// Run executes each task immediately and then at its interval.
func (s *Scheduler) Run(ctx context.Context) {
	last := make([]time.Time, len(s.tasks))
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()
	for {
		now := s.now()
		for i, t := range s.tasks {
			if !last[i].IsZero() && now.Sub(last[i]) < t.Interval {
				continue
			}
			last[i] = now
			if err := t.Run(ctx, now); err != nil && ctx.Err() == nil {
				s.logger.ErrorContext(ctx, "scheduled task failed", "task", t.Name, "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
