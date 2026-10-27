package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := New(3, time.Minute, func() time.Time { return now })

	for range 3 {
		ok, _ := l.Allow("ip:1")
		assert.True(t, ok)
	}
	ok, wait := l.Allow("ip:1")
	assert.False(t, ok)
	assert.Equal(t, 20*time.Second, wait)

	ok, _ = l.Allow("ip:2")
	assert.True(t, ok, "keys have independent budgets")

	now = now.Add(20 * time.Second)
	ok, _ = l.Allow("ip:1")
	assert.True(t, ok, "budget refills over time")
}
