package ratelimit

import (
	"encoding/binary"
	"math"
	"time"
)

// tokenBucketState: tokens remaining (float64) + last refill time (unix nanos).
type tokenBucketAlgo struct{}

func encodeTokenBucket(tokens float64, lastRefill time.Time) []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[0:8], math.Float64bits(tokens))
	binary.BigEndian.PutUint64(buf[8:16], uint64(lastRefill.UnixNano()))
	return buf
}

func decodeTokenBucket(b []byte) (tokens float64, lastRefill time.Time) {
	tokens = math.Float64frombits(binary.BigEndian.Uint64(b[0:8]))
	lastRefill = time.Unix(0, int64(binary.BigEndian.Uint64(b[8:16])))
	return tokens, lastRefill
}

func (tokenBucketAlgo) Allow(key string, cfg Config, store Store) Result {
	capacity := float64(cfg.burst())
	refillPerNano := float64(cfg.Rate) / float64(cfg.Per.Nanoseconds())

	now := time.Now()
	tokens := capacity
	lastRefill := now

	if raw, ok := store.Get(key); ok {
		tokens, lastRefill = decodeTokenBucket(raw)
		elapsed := now.Sub(lastRefill)
		tokens += float64(elapsed.Nanoseconds()) * refillPerNano
		if tokens > capacity {
			tokens = capacity
		}
	}

	allowed := tokens >= 1
	var retryAfter time.Duration
	if allowed {
		tokens--
	} else {
		missing := 1 - tokens
		retryAfter = time.Duration(missing/refillPerNano) * time.Nanosecond
	}

	store.Set(key, encodeTokenBucket(tokens, now), cfg.Per)

	remaining := int(tokens)
	if remaining < 0 {
		remaining = 0
	}
	// Time until the bucket is back to full capacity.
	missingForFull := capacity - tokens
	resetAt := now.Add(time.Duration(missingForFull/refillPerNano) * time.Nanosecond)

	return Result{
		Allowed:    allowed,
		Remaining:  remaining,
		RetryAfter: retryAfter,
		ResetAt:    resetAt,
	}
}
