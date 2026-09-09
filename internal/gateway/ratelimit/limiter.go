// Package ratelimit provides a bounded in-memory token bucket limiter for
// the Gateway's public API surface. It is intentionally single-node: no
// distributed state, no Redis, and a hard cap on tracked keys so memory
// usage stays bounded under abuse.
package ratelimit

import (
	"sync"
	"time"
)

// defaultMaxKeys bounds the number of distinct keys tracked when a caller
// does not provide an explicit limit.
const defaultMaxKeys = 10000

// Limiter implements a per-key token bucket with a bounded key set.
// The zero value is not usable; construct via New. A nil *Limiter is a
// valid disabled limiter.
type Limiter struct {
	mu       sync.Mutex
	capacity float64
	rate     float64 // tokens per second
	maxKeys  int
	now      func() time.Time
	buckets  map[string]*bucket
}

type bucket struct {
	tokens  float64
	updated time.Time
}

// New returns a limiter allowing perMinute requests per key with a burst up
// to perMinute. A perMinute value <= 0 disables limiting and returns nil,
// which is a safe no-op limiter. maxKeys <= 0 falls back to the default.
func New(perMinute, maxKeys int) *Limiter {
	if perMinute <= 0 {
		return nil
	}
	if maxKeys <= 0 {
		maxKeys = defaultMaxKeys
	}
	return &Limiter{
		capacity: float64(perMinute),
		rate:     float64(perMinute) / 60.0,
		maxKeys:  maxKeys,
		now:      time.Now,
		buckets:  make(map[string]*bucket),
	}
}

// Allow consumes one token for the given key. When denied it reports how
// long the client should wait before retrying. When the key table is full,
// exhausted-but-idle buckets are purged first; if no bucket can be freed,
// brand-new keys are rejected (existing keys are unaffected) so the map
// never grows past maxKeys.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if l == nil {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	entry, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.maxKeys {
			l.purgeLocked(now)
			if len(l.buckets) >= l.maxKeys {
				return false, time.Second
			}
		}
		entry = &bucket{tokens: l.capacity, updated: now}
		l.buckets[key] = entry
	}
	entry.tokens = min(l.capacity, entry.tokens+now.Sub(entry.updated).Seconds()*l.rate)
	entry.updated = now
	if entry.tokens >= 1 {
		entry.tokens--
		return true, 0
	}
	wait := time.Duration((1 - entry.tokens) / l.rate * float64(time.Second))
	if wait < time.Second {
		wait = time.Second
	}
	return false, wait
}

// purgeLocked removes buckets that are effectively idle (fully replenished).
func (l *Limiter) purgeLocked(now time.Time) {
	for key, entry := range l.buckets {
		replenished := entry.tokens + now.Sub(entry.updated).Seconds()*l.rate
		if replenished >= l.capacity {
			delete(l.buckets, key)
		}
	}
}
