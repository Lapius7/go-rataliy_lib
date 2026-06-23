// Package redisstore provides a ratelimit.Store backed by Redis, so rate
// limits can be shared across multiple instances of a service instead of
// being tracked separately per process.
package redisstore

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store implements ratelimit.Store using a Redis client. Keys are written
// with the given Prefix to avoid colliding with unrelated data in the same
// Redis instance.
type Store struct {
	Client *redis.Client
	Prefix string
	// Timeout bounds each Redis round trip. Defaults to 100ms if zero.
	Timeout time.Duration
}

// New creates a Store using client, namespacing all keys under prefix.
func New(client *redis.Client, prefix string) *Store {
	return &Store{Client: client, Prefix: prefix}
}

func (s *Store) timeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 100 * time.Millisecond
}

// Get implements ratelimit.Store.
func (s *Store) Get(key string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout())
	defer cancel()

	val, err := s.Client.Get(ctx, s.Prefix+key).Bytes()
	if err != nil {
		return nil, false
	}
	return val, true
}

// Set implements ratelimit.Store.
func (s *Store) Set(key string, state []byte, ttl time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout())
	defer cancel()

	s.Client.Set(ctx, s.Prefix+key, state, ttl)
}
