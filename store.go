package ratelimit

import (
	"sync"
	"time"
)

// Store persists per-key algorithm state. The default implementation is
// in-memory; callers can supply their own (e.g. backed by Redis) to share
// limits across multiple processes.
type Store interface {
	Get(key string) (state []byte, ok bool)
	Set(key string, state []byte, ttl time.Duration)
}

type entry struct {
	state    []byte
	expireAt time.Time
}

// KeyState is a point-in-time snapshot of one key's raw state as held by a
// Store, used by Limiter.Snapshot to build dashboard views. The State blob
// is algorithm-specific; decode it via the owning Limiter's Inspect logic
// rather than interpreting it directly.
type KeyState struct {
	Key      string
	State    []byte
	ExpireAt time.Time
}

// keyEnumerator is implemented by Stores that can list their current
// entries. The default in-memory store implements it; custom Stores (e.g.
// redisstore) generally don't, since enumerating a shared backend can be
// expensive or unsafe to do on demand.
type keyEnumerator interface {
	snapshot() []KeyState
}

// memoryStore is a sync.Map-backed Store with periodic expiry sweeps.
type memoryStore struct {
	mu       sync.RWMutex
	data     map[string]entry
	stopCh   chan struct{}
	closeOne sync.Once
}

func newMemoryStore() *memoryStore {
	s := &memoryStore{
		data:   make(map[string]entry),
		stopCh: make(chan struct{}),
	}
	go s.gcLoop()
	return s
}

func (s *memoryStore) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok || time.Now().After(e.expireAt) {
		return nil, false
	}
	return e.state, true
}

func (s *memoryStore) Set(key string, state []byte, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = entry{state: state, expireAt: time.Now().Add(ttl)}
}

func (s *memoryStore) gcLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.sweep()
		case <-s.stopCh:
			return
		}
	}
}

func (s *memoryStore) sweep() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.data {
		if now.After(e.expireAt) {
			delete(s.data, k)
		}
	}
}

// snapshot returns a copy of all live (non-expired) entries.
func (s *memoryStore) snapshot() []KeyState {
	now := time.Now()
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]KeyState, 0, len(s.data))
	for k, e := range s.data {
		if now.After(e.expireAt) {
			continue
		}
		out = append(out, KeyState{Key: k, State: e.state, ExpireAt: e.expireAt})
	}
	return out
}

// Close stops the background expiry sweep. Safe to call more than once.
func (s *memoryStore) Close() error {
	s.closeOne.Do(func() {
		close(s.stopCh)
	})
	return nil
}
