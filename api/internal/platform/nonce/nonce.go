// Package nonce remembers signed-request nonces so a captured request cannot be
// replayed inside the timestamp window.
//
// Why this is separate from the timestamp check: the ±5 minute window alone only
// rejects OLD captures. An attacker who captures a push and replays it 30 seconds
// later is inside the window, and without a nonce cache that replay is accepted.
// The two mechanisms cover each other's gap — see docs/05-SECURITY.md §4.4.1.
package nonce

import (
	"sync"
	"time"
)

// DefaultMaxEntries bounds memory. A signed request is only consumed after its
// signature verifies, so only trusted devices can add entries — but one faulty
// device in a retry loop should not be able to grow this map without limit.
const DefaultMaxEntries = 100_000

// Cache records nonces and answers "have I seen this before?".
//
// Deliberately an interface: the in-memory implementation below is correct for a
// single Cloud Run instance and WRONG for several. With more than one instance a
// replay can land on an instance that has never seen the nonce, so the production
// implementation is a shared store (Redis or a Postgres table with a TTL sweep).
// Recorded as future work rather than pretended away — the same trade-off the
// login rate limiter makes.
type Cache interface {
	// Consume records key and reports whether it was previously unseen.
	// A false return means "replay".
	Consume(key string, ttl time.Duration, now time.Time) bool
	// Len reports how many entries are retained. For tests and metrics.
	Len() int
}

// Memory is a mutex-guarded map with lazy expiry.
type Memory struct {
	mu         sync.Mutex
	seen       map[string]time.Time
	maxEntries int
	lastSweep  time.Time
	sweepEvery time.Duration
}

// NewMemory builds an in-process nonce cache.
func NewMemory() *Memory {
	return &Memory{
		seen:       make(map[string]time.Time),
		maxEntries: DefaultMaxEntries,
		sweepEvery: 30 * time.Second,
	}
}

// Consume implements Cache.
//
// Order matters at the call site: the caller must verify the signature FIRST and
// only then consume. Consuming first would let an unauthenticated flood fill the
// cache and evict legitimate nonces, turning a defence into a denial of service.
func (m *Memory) Consume(key string, ttl time.Duration, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sweepLocked(now)

	if expiry, seen := m.seen[key]; seen && now.Before(expiry) {
		return false
	}

	if len(m.seen) >= m.maxEntries {
		// Full even after sweeping. Fail CLOSED: accepting an unrecorded nonce
		// would silently reopen the replay window, which is the failure this
		// whole type exists to prevent.
		return false
	}

	// TTL must exceed the acceptance window, or a nonce expires from the cache
	// while the request that used it is still inside the window. The caller
	// passes 2× the window.
	m.seen[key] = now.Add(ttl)
	return true
}

// Len implements Cache.
func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.seen)
}

// sweepLocked drops expired entries, at most once per sweepEvery.
func (m *Memory) sweepLocked(now time.Time) {
	if now.Sub(m.lastSweep) < m.sweepEvery && len(m.seen) < m.maxEntries {
		return
	}
	for key, expiry := range m.seen {
		if !now.Before(expiry) {
			delete(m.seen, key)
		}
	}
	m.lastSweep = now
}
