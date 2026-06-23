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
	// If the Store implements io.Closer, Limiter.Close calls it.
	Store Store
}

func (c Config) burst() int {
	if c.Burst > 0 {
		return c.Burst
	}
	return c.Rate
}

// Result is the outcome of a rate limit check.
type Result struct {
	// Allowed reports whether the request is permitted.
	Allowed bool

	// Remaining is the number of additional requests allowed before the
	// limit is hit, given the state at the time of this check.
	Remaining int

	// RetryAfter indicates how long to wait before retrying. It is zero
	// when Allowed is true.
	RetryAfter time.Duration

	// ResetAt is when the limit window resets and Remaining returns to
	// its maximum.
	ResetAt time.Time
}

type algorithm interface {
	Allow(key string, cfg Config, store Store) Result
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
// limiter's configuration.
func (l *Limiter) Allow(key string) Result {
	return l.algo.Allow(key, l.cfg, l.cfg.Store)
}

// Close releases resources held by the limiter's Store, such as the
// in-memory store's background cleanup goroutine. It is a no-op if the
// configured Store does not need closing.
func (l *Limiter) Close() error {
	if c, ok := l.cfg.Store.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
