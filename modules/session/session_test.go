package session

import (
	"encoding/hex"
	"testing"
	"time"

	utils "goSwitch/modules/utils"
)

func testConfig(maxSessions, ttlSeconds, idleSeconds int) *utils.Config {
	return &utils.Config{
		Dim:                       3,
		Cheat:                     false,
		ToggleSequence:            []bool{true, false, true},
		AvailableToggleSequence:   []int{0, 4, 8},
		MaxSessions:               maxSessions,
		SessionTTLSeconds:         ttlSeconds,
		SessionIdleTimeoutSeconds: idleSeconds,
	}
}

func TestNewIDIsUniqueAndWellFormed(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id := NewID()

		if _, err := hex.DecodeString(id); err != nil {
			t.Fatalf("NewID() = %q is not valid hex: %v", id, err)
		}
		if len(id) != 32 {
			t.Fatalf("NewID() = %q has length %d, want 32", id, len(id))
		}
		if seen[id] {
			t.Fatalf("NewID() produced a duplicate: %q", id)
		}
		seen[id] = true
	}
}

func TestClaimCreatesThenTouchesSameSession(t *testing.T) {
	m := NewManager(testConfig(10, 1800, 300))

	sess, ok := m.Claim("client-a")
	if !ok {
		t.Fatal("Claim() failed on an empty manager with capacity available")
	}
	if sess.Dim != 3 {
		t.Errorf("new session Dim = %d, want 3 (from config default)", sess.Dim)
	}

	// Rewind LastUpdatedAt to prove the second Claim() actually touches it.
	sess.LastUpdatedAt = time.Now().Add(-time.Hour)

	again, ok := m.Claim("client-a")
	if !ok {
		t.Fatal("Claim() on an existing id failed")
	}
	if again != sess {
		t.Fatal("Claim() on an existing id returned a different *Session")
	}
	if time.Since(again.LastUpdatedAt) > time.Second {
		t.Fatalf("Claim() did not bump LastUpdatedAt: %v", again.LastUpdatedAt)
	}

	if got := m.Count(); got != 1 {
		t.Errorf("Count() = %d, want 1", got)
	}
}

func TestClaimEnforcesMaxSessions(t *testing.T) {
	m := NewManager(testConfig(2, 1800, 300))

	if _, ok := m.Claim("a"); !ok {
		t.Fatal("Claim(a) should have succeeded")
	}
	if _, ok := m.Claim("b"); !ok {
		t.Fatal("Claim(b) should have succeeded")
	}
	if _, ok := m.Claim("c"); ok {
		t.Fatal("Claim(c) should have failed: manager is at capacity with no evictable session")
	}
	if got := m.Count(); got != 2 {
		t.Errorf("Count() = %d, want 2", got)
	}

	// Re-claiming an existing id must still work even at capacity.
	if _, ok := m.Claim("a"); !ok {
		t.Fatal("Claim(a) on an existing id should succeed even at capacity")
	}
}

func TestClaimNeverEvictsWhileUnderCapacity(t *testing.T) {
	m := NewManager(testConfig(2, 1800, 300))

	sess, _ := m.Claim("a")
	// Make "a" look both TTL- and idle-expired.
	sess.CreatedAt = time.Now().Add(-2 * time.Hour)
	sess.LastUpdatedAt = time.Now().Add(-2 * time.Hour)

	// There is still a free slot (MaxSessions=2, only 1 session exists), so "a"
	// must NOT be purged just because a new id shows up.
	if _, ok := m.Claim("b"); !ok {
		t.Fatal("Claim(b) should have succeeded (a free slot exists)")
	}
	if m.Count() != 2 {
		t.Fatalf("Count() = %d, want 2 -- expired session 'a' must not be purged while under capacity", m.Count())
	}
}

func TestClaimEvictsExpiredOnlyAtCapacity(t *testing.T) {
	m := NewManager(testConfig(1, 1800, 300))

	sess, _ := m.Claim("a")
	sess.CreatedAt = time.Now().Add(-2 * time.Hour) // well past the 1800s TTL

	// Manager is now at capacity (MaxSessions=1); "a" is TTL-expired, so it should
	// be reclaimed for the new id.
	if _, ok := m.Claim("b"); !ok {
		t.Fatal("Claim(b) should have succeeded: 'a' is TTL-expired and should be purged")
	}
	if _, stillThere := m.Claim("a"); stillThere {
		t.Fatal("'a' should have been evicted, not still present")
	}
}

func TestClaimEvictsIdleOnlyAtCapacity(t *testing.T) {
	m := NewManager(testConfig(1, 1800, 300))

	sess, _ := m.Claim("a")
	sess.CreatedAt = time.Now()                             // fresh, well under TTL
	sess.LastUpdatedAt = time.Now().Add(-10 * time.Minute) // past the 300s idle timeout

	if _, ok := m.Claim("b"); !ok {
		t.Fatal("Claim(b) should have succeeded: 'a' is idle-expired and should be purged")
	}
}

func TestClaimRefusesWhenFullAndNothingEvictable(t *testing.T) {
	m := NewManager(testConfig(1, 1800, 300))

	m.Claim("a") // fresh: neither TTL- nor idle-expired

	if _, ok := m.Claim("b"); ok {
		t.Fatal("Claim(b) should have failed: 'a' is fresh, nothing to evict")
	}
}
