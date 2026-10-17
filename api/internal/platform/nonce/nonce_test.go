package nonce_test

import (
	"testing"
	"time"

	"github.com/mikedalu/phc-records/api/internal/platform/nonce"
)

func TestConsumeRejectsReplayInsideTTL(t *testing.T) {
	m := nonce.NewMemory()
	now := time.Now()

	if !m.Consume("dev:abc", time.Minute, now) {
		t.Fatal("first consume should succeed")
	}
	if m.Consume("dev:abc", time.Minute, now.Add(30*time.Second)) {
		t.Fatal("second consume inside the TTL should be rejected")
	}
}

// TestConsumeAllowsReuseAfterTTL documents the interaction that matters: the TTL
// must outlive the acceptance window, or a nonce expires while its request is
// still inside the window and the replay is accepted.
func TestConsumeAllowsReuseAfterTTL(t *testing.T) {
	m := nonce.NewMemory()
	now := time.Now()

	if !m.Consume("dev:abc", time.Minute, now) {
		t.Fatal("first consume should succeed")
	}
	if !m.Consume("dev:abc", time.Minute, now.Add(2*time.Minute)) {
		t.Fatal("consume after the TTL should succeed")
	}
}

func TestConsumeIsKeyedPerDevice(t *testing.T) {
	m := nonce.NewMemory()
	now := time.Now()

	if !m.Consume("device-a:same", time.Minute, now) {
		t.Fatal("device-a first consume should succeed")
	}
	// A different device with the same nonce string must not be blocked.
	if !m.Consume("device-b:same", time.Minute, now) {
		t.Fatal("device-b should not be affected by device-a's nonce")
	}
}

func TestSweepDropsExpiredEntries(t *testing.T) {
	m := nonce.NewMemory()
	now := time.Now()

	m.Consume("dev:one", time.Second, now)
	if m.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", m.Len())
	}

	// Past the TTL and past the sweep interval.
	later := now.Add(2 * time.Minute)
	m.Consume("dev:two", time.Second, later)

	if m.Len() != 1 {
		t.Fatalf("expired entries should have been swept, got %d entries", m.Len())
	}
}
