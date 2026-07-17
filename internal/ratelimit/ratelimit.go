// Package ratelimit implements the in-process token buckets from section
// 6. v1 runs one service instance, so in-process buckets are sufficient —
// losing them on restart is an accepted tradeoff, not a bug (section 6:
// "adding Redis is not [acceptable]").
package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

// Limiter is a keyed set of token buckets (e.g. one per source IP, per
// project, or per installation), all sharing the same rate/burst.
//
// ponytail: buckets are never evicted, so long-running processes with many
// distinct keys (e.g. per-installation limiters) grow this map for the
// life of the process. Acceptable at v1 scale (section 16 triggers a
// review well before this matters); add idle-eviction if a memory profile
// ever shows it's the actual ceiling.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
	r       rate.Limit
	burst   int
}

// NewLimiter creates a limiter allowing burst immediate events per key,
// refilling at r events/second thereafter.
func NewLimiter(r rate.Limit, burst int) *Limiter {
	return &Limiter{
		buckets: make(map[string]*rate.Limiter),
		r:       r,
		burst:   burst,
	}
}

// PerMinute expresses a "N per minute, burst B" limit from section 6's
// table as a rate.Limit (events/second).
func PerMinute(n float64) rate.Limit { return rate.Limit(n / 60) }

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	b, ok := l.buckets[key]
	if !ok {
		b = rate.NewLimiter(l.r, l.burst)
		l.buckets[key] = b
	}
	l.mu.Unlock()
	return b.Allow()
}
