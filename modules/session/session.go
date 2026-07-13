// Package session implements per-client game sessions: a capacity-bounded,
// cookie-keyed Manager that lazily purges TTL-expired or idle sessions only when
// a new one needs a slot.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	grid "goSwitch/modules/grid"
	utils "goSwitch/modules/utils"
)

// Session holds one client's isolated game state. Dim, Cheat, ToggleSequence, and Game
// are guarded by the embedded sync.Mutex -- callers must sess.Lock()/sess.Unlock()
// around any access. CreatedAt and LastUpdatedAt are a different lock domain, owned by
// Manager: CreatedAt is written once at construction (under m.mu, before the session is
// ever handed out) and never changes afterward, so reading it is safe without any lock;
// LastUpdatedAt is repeatedly bumped by Claim under m.mu and must not be read directly
// from outside the session package -- use Manager.SessionMaxAge for the one thing
// callers actually need it for (the session's remaining TTL).
type Session struct {
	ID             string
	Dim            int
	Cheat          bool
	ToggleSequence []bool
	Game           *grid.Grid
	CreatedAt      time.Time
	LastUpdatedAt  time.Time

	sync.Mutex
}

// NewID returns a random, URL/cookie-safe session identifier.
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Manager tracks live sessions and enforces MaxSessions/TTL/idle-timeout, purging
// lazily: only when a slot is actually needed and the manager is at capacity.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session

	maxSessions int
	ttl         time.Duration
	idleTimeout time.Duration

	defaultDim            int
	defaultCheat          bool
	defaultToggleSequence []bool
	defaultNeighborhood   []int
}

func NewManager(config *utils.Config) *Manager {
	return &Manager{
		sessions:              make(map[string]*Session),
		maxSessions:           config.MaxSessions,
		ttl:                   time.Duration(config.SessionTTLSeconds) * time.Second,
		idleTimeout:           time.Duration(config.SessionIdleTimeoutSeconds) * time.Second,
		defaultDim:            config.Dim,
		defaultCheat:          config.Cheat,
		defaultToggleSequence: append([]bool(nil), config.ToggleSequence...),
		defaultNeighborhood:   utils.BuildNeighborhoodFromConfig(config),
	}
}

// Claim returns the existing session for id, bumping its LastUpdatedAt, and reports
// existed=true. If no session exists for id, it tries to create one (existed=false),
// opportunistically evicting TTL-expired then idle-timed-out sessions if the manager
// is at capacity. Returns ok=false only when still at capacity after eviction
// attempts -- the caller must have the client wait.
func (m *Manager) Claim(id string) (sess *Session, ok bool, existed bool) {
	m.mu.Lock()

	if s, found := m.sessions[id]; found {
		s.LastUpdatedAt = time.Now()
		m.mu.Unlock()
		return s, true, true
	}

	now := time.Now()

	if len(m.sessions) >= m.maxSessions {
		m.evictExpiredLocked(now)
	}
	if len(m.sessions) >= m.maxSessions {
		m.evictIdleLocked(now)
	}
	if len(m.sessions) >= m.maxSessions {
		m.mu.Unlock()
		return nil, false, false
	}

	s, neighborhood := m.reserveSessionLocked(id, now)
	m.mu.Unlock()

	// grid.NewGrid can internally retry up to its own bounded limit for structurally
	// degenerate (dim, neighborhood) combinations -- built outside m.mu so it can't
	// stall every other client's session operations for however long that takes.
	// s is already reserved in the map and locked (see reserveSessionLocked), so a
	// concurrent evict pass will correctly skip it via TryLock instead of deleting a
	// still-being-built session out from under this goroutine.
	s.Game = grid.NewGrid(s.Dim, neighborhood)
	s.Unlock()

	return s, true, false
}

// Count returns the number of currently live sessions.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.sessions)
}

// SessionMaxAge returns how much longer sess has left before its absolute TTL (from
// creation) expires, floored at zero. Intended for setting a session cookie's MaxAge so
// the client-visible cookie lifetime actually reflects the server-side deadline, instead
// of a fixed MaxAge that keeps rolling forward on every request regardless of how close
// the session actually is to being evicted.
func (m *Manager) SessionMaxAge(sess *Session) time.Duration {
	remaining := m.ttl - time.Since(sess.CreatedAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// reserveSessionLocked registers a placeholder Session in the map (so capacity
// bookkeeping is correct the instant this returns) and takes its lock before
// releasing m.mu, so Claim can build the -- potentially slow -- Grid outside the
// manager lock without any other goroutine observing a half-built session (they'd
// block on sess.Lock() first, per this package's existing locking convention). The
// caller must set s.Game and call s.Unlock() once construction finishes.
func (m *Manager) reserveSessionLocked(id string, now time.Time) (s *Session, neighborhood []int) {
	s = &Session{
		ID:             id,
		Dim:            m.defaultDim,
		Cheat:          m.defaultCheat,
		ToggleSequence: append([]bool(nil), m.defaultToggleSequence...),
		CreatedAt:      now,
		LastUpdatedAt:  now,
	}
	s.Lock()

	m.sessions[id] = s

	return s, append([]int(nil), m.defaultNeighborhood...)
}

func (m *Manager) evictExpiredLocked(now time.Time) {
	for id, s := range m.sessions {
		if now.Sub(s.CreatedAt) >= m.ttl {
			m.evictLocked(id, s)
		}
	}
}

func (m *Manager) evictIdleLocked(now time.Time) {
	for id, s := range m.sessions {
		if now.Sub(s.LastUpdatedAt) >= m.idleTimeout {
			m.evictLocked(id, s)
		}
	}
}

// evictLocked removes id from the map, but only if sess isn't actively in use. A
// session held by an in-flight request (sess.Lock() already taken by a handler, or by
// reserveSessionLocked while a new Grid is still being built) fails TryLock and is left
// in place for this pass, instead of being deleted -- and its in-progress work silently
// discarded -- out from under whoever holds it.
func (m *Manager) evictLocked(id string, sess *Session) {
	if !sess.TryLock() {
		return
	}
	delete(m.sessions, id)
	sess.Unlock()
}
