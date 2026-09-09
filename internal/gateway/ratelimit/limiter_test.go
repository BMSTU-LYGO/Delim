package ratelimit

import (
	"strconv"
	"testing"
	"time"
)

type fakeClock struct {
	current time.Time
}

func (c *fakeClock) Now() time.Time { return c.current }

func (c *fakeClock) Advance(d time.Duration) { c.current = c.current.Add(d) }

func newTestLimiter(perMinute, maxKeys int) (*Limiter, *fakeClock) {
	limiter := New(perMinute, maxKeys)
	clock := &fakeClock{current: time.Unix(1700000000, 0)}
	limiter.now = clock.Now
	return limiter, clock
}

func TestNewDisabledForNonPositiveRate(t *testing.T) {
	t.Parallel()
	if limiter := New(0, 10); limiter != nil {
		t.Fatal("expected nil limiter for perMinute <= 0")
	}
	var nilLimiter *Limiter
	if ok, _ := nilLimiter.Allow("any"); !ok {
		t.Fatal("nil limiter must allow")
	}
}

func TestAllowBurstThenDenyWithRetryAfter(t *testing.T) {
	t.Parallel()
	limiter, _ := newTestLimiter(3, 100)
	for i := 0; i < 3; i++ {
		if ok, _ := limiter.Allow("key"); !ok {
			t.Fatalf("request %d denied inside burst", i)
		}
	}
	ok, retry := limiter.Allow("key")
	if ok {
		t.Fatal("expected denial after burst")
	}
	if retry < time.Second || retry > 20*time.Second {
		t.Fatalf("retry-after out of range: %v", retry)
	}
}

func TestAllowRefillsOverTime(t *testing.T) {
	t.Parallel()
	limiter, clock := newTestLimiter(60, 100) // 1 token per second
	for i := 0; i < 60; i++ {
		if ok, _ := limiter.Allow("key"); !ok {
			t.Fatalf("burst request %d denied", i)
		}
	}
	if ok, _ := limiter.Allow("key"); ok {
		t.Fatal("expected denial after exhausting burst")
	}
	clock.Advance(2 * time.Second)
	if ok, _ := limiter.Allow("key"); !ok {
		t.Fatal("expected refill after waiting")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	t.Parallel()
	limiter, _ := newTestLimiter(1, 100)
	if ok, _ := limiter.Allow("a"); !ok {
		t.Fatal("first allow for a denied")
	}
	if ok, _ := limiter.Allow("a"); ok {
		t.Fatal("expected denial for exhausted a")
	}
	if ok, _ := limiter.Allow("b"); !ok {
		t.Fatal("expected b unaffected by a")
	}
}

func TestBoundedKeyTablePurgesIdleAndRejectsNew(t *testing.T) {
	t.Parallel()
	limiter, clock := newTestLimiter(2, 2)
	if ok, _ := limiter.Allow("a"); !ok {
		t.Fatal("a: unexpected denial")
	}
	if ok, _ := limiter.Allow("b"); !ok {
		t.Fatal("b: unexpected denial")
	}
	// Table full of exhausted keys: brand-new keys must be rejected.
	if ok, _ := limiter.Allow("c"); ok {
		t.Fatal("expected new-key rejection while table is full of active keys")
	}
	// Once buckets replenish, idle keys are purged on insert.
	clock.Advance(time.Minute)
	if ok, _ := limiter.Allow("c"); !ok {
		t.Fatal("expected idle buckets purged for new key")
	}
}

func TestDeniesRepeatedRapidKeysWithinBound(t *testing.T) {
	t.Parallel()
	limiter, _ := newTestLimiter(1000, 50)
	denials := 0
	for i := 0; i < 200; i++ {
		if ok, _ := limiter.Allow("k" + strconv.Itoa(i)); !ok {
			denials++
		}
	}
	limiter.mu.Lock()
	size := len(limiter.buckets)
	limiter.mu.Unlock()
	if size > 50 {
		t.Fatalf("tracked keys %d exceed bound 50", size)
	}
	if denials == 0 {
		t.Fatal("expected some new keys to be rejected once bound is full")
	}
}
