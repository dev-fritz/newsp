// Package ratelimit provides an in-memory, per-key token bucket limiter.
//
// State lives in the process, which fits a single API instance (the
// self-hosted default). Several replicas would each enforce their own budget.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// sweepThreshold is the number of keys above which idle entries are evicted.
const sweepThreshold = 10_000

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter allows `burst` events per key, refilled evenly over `per`.
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	limit   rate.Limit
	burst   int
	idle    time.Duration
	now     func() time.Time
}

// New builds a Limiter allowing burst events per window for each key.
func New(burst int, per time.Duration, now func() time.Time) *Limiter {
	return &Limiter{
		entries: make(map[string]*entry),
		limit:   rate.Every(per / time.Duration(burst)),
		burst:   burst,
		idle:    per,
		now:     now,
	}
}

// Allow consumes one event for key. When the budget is exhausted it returns
// false and how long the caller should wait.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if len(l.entries) > sweepThreshold {
		for k, e := range l.entries {
			if now.Sub(e.lastSeen) > l.idle {
				delete(l.entries, k)
			}
		}
	}

	e, ok := l.entries[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = e
	}
	e.lastSeen = now

	res := e.limiter.ReserveN(now, 1)
	if delay := res.DelayFrom(now); delay > 0 {
		res.CancelAt(now)
		return false, delay
	}
	return true, 0
}
