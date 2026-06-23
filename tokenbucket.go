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

// refillTokenBucket computes the token count as of now, given a possibly
// stale (tokens, lastRefill) pair. It performs no I/O and consumes nothing,
// so both Allow and Inspect can share it.
func refillTokenBucket(tokens float64, lastRefill, now time.Time, capacity, refillPerNano float64) float64 {
	elapsed := now.Sub(lastRefill)
	tokens += float64(elapsed.Nanoseconds()) * refillPerNano
	if tokens > capacity {
		tokens = capacity
	}
	return tokens
}

func tokenBucketResetAt(tokens, capacity, refillPerNano float64, now time.Time) time.Time {
	missingForFull := capacity - tokens
	return now.Add(time.Duration(missingForFull/refillPerNano) * time.Nanosecond)
}

func (tokenBucketAlgo) Allow(key string, cfg Config, store Store) Result {
	capacity := float64(cfg.burst())
	refillPerNano := float64(cfg.Rate) / float64(cfg.Per.Nanoseconds())

	now := time.Now()
	tokens := capacity

	if raw, ok := store.Get(key); ok {
		storedTokens, storedRefill := decodeTokenBucket(raw)
		tokens = refillTokenBucket(storedTokens, storedRefill, now, capacity, refillPerNano)
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

	return Result{
		Allowed:    allowed,
		Remaining:  remaining,
		RetryAfter: retryAfter,
		ResetAt:    tokenBucketResetAt(tokens, capacity, refillPerNano, now),
	}
}

// Inspect reports the current state for key's stored blob without
// consuming a token or writing back to the store. Used for read-only
// displays such as Dashboard.
func (tokenBucketAlgo) Inspect(state []byte, cfg Config, now time.Time) Result {
	capacity := float64(cfg.burst())
	refillPerNano := float64(cfg.Rate) / float64(cfg.Per.Nanoseconds())

	storedTokens, storedRefill := decodeTokenBucket(state)
	tokens := refillTokenBucket(storedTokens, storedRefill, now, capacity, refillPerNano)

	remaining := int(tokens)
	if remaining < 0 {
		remaining = 0
	}

	return Result{
		Allowed:   tokens >= 1,
		Remaining: remaining,
		ResetAt:   tokenBucketResetAt(tokens, capacity, refillPerNano, now),
	}
}
