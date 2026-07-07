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

// Session holds one client's isolated game state. It embeds sync.Mutex so callers
// guard access with sess.Lock()/sess.Unlock() directly.
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
	defer m.mu.Unlock()

	if s, found := m.sessions[id]; found {
		s.LastUpdatedAt = time.Now()
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
		return nil, false, false
	}

	return m.newSessionLocked(id, now), true, false
}

// Count returns the number of currently live sessions.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.sessions)
}

func (m *Manager) newSessionLocked(id string, now time.Time) *Session {
	s := &Session{
		ID:             id,
		Dim:            m.defaultDim,
		Cheat:          m.defaultCheat,
		ToggleSequence: append([]bool(nil), m.defaultToggleSequence...),
		Game:           grid.NewGrid(m.defaultDim, append([]int(nil), m.defaultNeighborhood...)),
		CreatedAt:      now,
		LastUpdatedAt:  now,
	}

	m.sessions[id] = s

	return s
}

func (m *Manager) evictExpiredLocked(now time.Time) {
	for id, s := range m.sessions {
		if now.Sub(s.CreatedAt) >= m.ttl {
			delete(m.sessions, id)
		}
	}
}

func (m *Manager) evictIdleLocked(now time.Time) {
	for id, s := range m.sessions {
		if now.Sub(s.LastUpdatedAt) >= m.idleTimeout {
			delete(m.sessions, id)
		}
	}
}
