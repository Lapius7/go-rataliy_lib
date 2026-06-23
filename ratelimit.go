// Package ratelimit provides HTTP rate limiting with a choice of
// algorithms (token bucket, sliding window, fixed window) and a
// net/http middleware that can be dropped into any handler chain.
package ratelimit

import "time"

// Algorithm selects which rate limiting strategy a Limiter uses.
type Algorithm string

const (
	TokenBucket   Algorithm = "token_bucket"
	SlidingWindow Algorithm = "sliding_window"
	FixedWindow   Algorithm = "fixed_window"
)

// Config controls the rate limit applied by a Limiter.
type Config struct {
	// Rate is the number of requests allowed per Per duration.
	Rate int

	// Per is the time window over which Rate applies.
	Per time.Duration

	// Burst is the maximum number of tokens a TokenBucket limiter can
	// accumulate. If zero, it defaults to Rate. Ignored by other algorithms.
	Burst int

	// KeyFunc extracts the rate limit key from an incoming request.
	// Defaults to ByIP if nil.
	KeyFunc KeyFunc

	// Store persists per-key state. Defaults to an in-memory store if nil.
	Store Store
}

func (c Config) burst() int {
	if c.Burst > 0 {
		return c.Burst
	}
	return c.Rate
}

type algorithm interface {
	Allow(key string, cfg Config, store Store) (bool, time.Duration)
}

// Limiter enforces a Config's rate limit using a chosen Algorithm.
type Limiter struct {
	cfg  Config
	algo algorithm
}

// New creates a Limiter for the given algorithm and configuration.
func New(algo Algorithm, cfg Config) *Limiter {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = ByIP
	}
	if cfg.Store == nil {
		cfg.Store = newMemoryStore()
	}

	var impl algorithm
	switch algo {
	case SlidingWindow:
		impl = slidingWindowAlgo{}
	case FixedWindow:
		impl = fixedWindowAlgo{}
	default:
		impl = tokenBucketAlgo{}
	}

	return &Limiter{cfg: cfg, algo: impl}
}

// Allow reports whether a request identified by key is allowed under the
// limiter's configuration. When denied, retryAfter indicates how long the
// caller should wait before the next request is likely to succeed.
func (l *Limiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	return l.algo.Allow(key, l.cfg, l.cfg.Store)
}
