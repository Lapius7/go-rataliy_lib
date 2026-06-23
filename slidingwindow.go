package ratelimit

import (
	"encoding/binary"
	"time"
)

// slidingWindowState: previous window count + current window count + current window start (unix nanos).
type slidingWindowAlgo struct{}

func encodeSlidingWindow(prevCount, currCount uint32, windowStart time.Time) []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint32(buf[0:4], prevCount)
	binary.BigEndian.PutUint32(buf[4:8], currCount)
	binary.BigEndian.PutUint64(buf[8:16], uint64(windowStart.UnixNano()))
	return buf
}

func decodeSlidingWindow(b []byte) (prevCount, currCount uint32, windowStart time.Time) {
	prevCount = binary.BigEndian.Uint32(b[0:4])
	currCount = binary.BigEndian.Uint32(b[4:8])
	windowStart = time.Unix(0, int64(binary.BigEndian.Uint64(b[8:16])))
	return prevCount, currCount, windowStart
}

// rollSlidingWindow advances (prevCount, currCount, windowStart) to be
// current as of now, without consuming any request. It performs no I/O,
// so both Allow and Inspect can share it.
func rollSlidingWindow(prevCount, currCount uint32, windowStart, now time.Time, per time.Duration) (uint32, uint32, time.Time) {
	elapsed := now.Sub(windowStart)
	if elapsed >= 2*per {
		return 0, 0, now
	}
	if elapsed >= per {
		return currCount, 0, windowStart.Add(per)
	}
	return prevCount, currCount, windowStart
}

func slidingWindowEstimate(prevCount, currCount uint32, windowStart, now time.Time, per time.Duration) float64 {
	elapsedInCurrent := now.Sub(windowStart)
	weight := 1 - float64(elapsedInCurrent)/float64(per)
	if weight < 0 {
		weight = 0
	}
	return float64(currCount) + weight*float64(prevCount)
}

// Allow uses the sliding window counter approximation: the estimated count
// is the current window's count plus a fraction of the previous window's
// count, weighted by how much of the previous window still overlaps the
// sliding lookback period.
func (slidingWindowAlgo) Allow(key string, cfg Config, store Store) Result {
	now := time.Now()
	per := cfg.Per

	var prevCount, currCount uint32
	windowStart := now

	if raw, ok := store.Get(key); ok {
		storedPrev, storedCurr, storedStart := decodeSlidingWindow(raw)
		prevCount, currCount, windowStart = rollSlidingWindow(storedPrev, storedCurr, storedStart, now, per)
	}

	estimated := slidingWindowEstimate(prevCount, currCount, windowStart, now, per)

	allowed := estimated < float64(cfg.Rate)
	var retryAfter time.Duration
	if allowed {
		currCount++
	} else {
		retryAfter = per - now.Sub(windowStart)
	}

	store.Set(key, encodeSlidingWindow(prevCount, currCount, windowStart), 2*per)

	remaining := cfg.Rate - int(estimated) - 1
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
func (slidingWindowAlgo) Inspect(state []byte, cfg Config, now time.Time) Result {
	per := cfg.Per
	storedPrev, storedCurr, storedStart := decodeSlidingWindow(state)
	prevCount, currCount, windowStart := rollSlidingWindow(storedPrev, storedCurr, storedStart, now, per)

	estimated := slidingWindowEstimate(prevCount, currCount, windowStart, now, per)

	remaining := cfg.Rate - int(estimated)
	if remaining < 0 {
		remaining = 0
	}

	return Result{
		Allowed:   estimated < float64(cfg.Rate),
		Remaining: remaining,
		ResetAt:   windowStart.Add(per),
	}
}
