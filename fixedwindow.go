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

func (fixedWindowAlgo) Allow(key string, cfg Config, store Store) Result {
	now := time.Now()
	per := cfg.Per

	var count uint32
	windowStart := now

	if raw, ok := store.Get(key); ok {
		count, windowStart = decodeFixedWindow(raw)
		if now.Sub(windowStart) >= per {
			count = 0
			windowStart = now
		}
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
