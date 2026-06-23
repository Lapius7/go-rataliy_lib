package ratelimit

import (
	"encoding/binary"
	"time"
)

// fixedWindowState: count in current window + window start (unix nanos).
type fixedWindowAlgo struct{}

func encodeFixedWindow(count uint32, windowStart time.Time) []byte {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf[0:4], count)
	binary.BigEndian.PutUint64(buf[4:12], uint64(windowStart.UnixNano()))
	return buf
}

func decodeFixedWindow(b []byte) (count uint32, windowStart time.Time) {
	count = binary.BigEndian.Uint32(b[0:4])
	windowStart = time.Unix(0, int64(binary.BigEndian.Uint64(b[4:12])))
	return count, windowStart
}

// rollFixedWindow resets count to zero and windowStart to now if the
// window has elapsed. It performs no I/O, so both Allow and Inspect can
// share it.
func rollFixedWindow(count uint32, windowStart, now time.Time, per time.Duration) (uint32, time.Time) {
	if now.Sub(windowStart) >= per {
		return 0, now
	}
	return count, windowStart
}

func (fixedWindowAlgo) Allow(key string, cfg Config, store Store) Result {
	now := time.Now()
	per := cfg.Per

	var count uint32
	windowStart := now

	if raw, ok := store.Get(key); ok {
		storedCount, storedStart := decodeFixedWindow(raw)
		count, windowStart = rollFixedWindow(storedCount, storedStart, now, per)
	}

	allowed := count < uint32(cfg.Rate)
	var retryAfter time.Duration
	if allowed {
		count++
	} else {
		retryAfter = per - now.Sub(windowStart)
	}

	store.Set(key, encodeFixedWindow(count, windowStart), per)

	remaining := cfg.Rate - int(count)
	if remaining < 0 {
		remaining = 0
	}

	return Result{
		Allowed:    allowed,
		Remaining:  remaining,
		RetryAfter: retryAfter,
		ResetAt:    windowStart.Add(per),
	}
}

// Inspect reports the current state for key's stored blob without
// consuming a request or writing back to the store.
func (fixedWindowAlgo) Inspect(state []byte, cfg Config, now time.Time) Result {
	per := cfg.Per
	storedCount, storedStart := decodeFixedWindow(state)
	count, windowStart := rollFixedWindow(storedCount, storedStart, now, per)

	remaining := cfg.Rate - int(count)
	if remaining < 0 {
		remaining = 0
	}

	return Result{
		Allowed:   count < uint32(cfg.Rate),
		Remaining: remaining,
		ResetAt:   windowStart.Add(per),
	}
}
