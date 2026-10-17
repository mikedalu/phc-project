// Package ratelimit provides a fixed-window limiter for login attempts.
//
// ═══ SCOPE AND LIMITATION — READ THIS ═══
//
// This is IN-MEMORY, so the count is PER CLOUD RUN INSTANCE. With
// --max-instances=5 an attacker effectively gets 5× the configured allowance by
// hitting different instances. It is a speed bump, not a wall.
//
// That is acceptable now because the meaningful control is the ACCOUNT LOCKOUT,
// which is database-backed and therefore consistent across instances. This
// limiter exists to blunt rapid credential-stuffing from one source.
//
// The production answer is a shared store (Redis or a Postgres counter table).
// That is recorded as future work rather than pretended away — see
// docs/05-SECURITY.md §9 and ROADMAP §10.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter is a fixed-window counter keyed by an arbitrary string.
//
// Fixed windows are less precise than a sliding window or token bucket: a burst
// spanning a window boundary can briefly double the effective rate. For login
// throttling that is fine, and it avoids the per-key timestamp slices that a
// sliding window needs.
type Limiter struct {
	mu      sync.Mutex
	window  time.Duration
	limit   int
	buckets map[string]*bucket

	now func() time.Time

	// Sweep bookkeeping, so the map cannot grow without bound. An attacker
	// spraying random IPs would otherwise leak memory until the instance dies.
	lastSweep  time.Time
	sweepEvery time.Duration
}

type bucket struct {
	count   int
	resetAt time.Time
}

// New builds a Limiter allowing `limit` events per `window` per key.
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		window:     window,
		limit:      limit,
		buckets:    make(map[string]*bucket),
		now:        time.Now,
		sweepEvery: window,
	}
}

// Allow records an attempt for key and reports whether it is permitted.
//
// Returns (allowed, remaining, retryAfter):
//   - retryAfter is only meaningful when allowed is false.
func (l *Limiter) Allow(key string) (bool, int, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()

	// Opportunistic sweep. Deleting expired keys here — rather than in a
	// background goroutine — keeps the type testable and avoids a goroutine leak
	// when the limiter is discarded.
	if now.Sub(l.lastSweep) >= l.sweepEvery {
		for k, b := range l.buckets {
			if now.After(b.resetAt) {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}

	b, ok := l.buckets[key]
	if !ok || now.After(b.resetAt) {
		// New or expired window.
		l.buckets[key] = &bucket{count: 1, resetAt: now.Add(l.window)}
		return true, l.limit - 1, 0
	}

	if b.count >= l.limit {
		return false, 0, time.Until(b.resetAt)
	}

	b.count++
	return true, l.limit - b.count, 0
}

// Reset clears the window for a key. Called after a successful login, so a user
// who mistyped twice and then succeeded is not left halfway to lockout.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// ResetAll clears every window.
//
// Needed by tests: several scenarios log in repeatedly for the same account, and
// a shared limiter would otherwise exhaust the budget and fail them for a reason
// unrelated to what they assert.
//
// Also useful operationally — an admin who has just verified a user's identity
// can clear a lockout without waiting for the window to expire.
func (l *Limiter) ResetAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buckets = make(map[string]*bucket)
}

// Size reports the number of tracked keys. Used by tests and by an admin health
// endpoint to spot a memory leak.
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
