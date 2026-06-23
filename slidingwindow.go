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

// Allow uses the sliding window counter approximation: the estimated count
// is the current window's count plus a fraction of the previous window's
// count, weighted by how much of the previous window still overlaps the
// sliding lookback period.
func (slidingWindowAlgo) Allow(key string, cfg Config, store Store) (bool, time.Duration) {
	now := time.Now()
	per := cfg.Per

	var prevCount, currCount uint32
	windowStart := now

	if raw, ok := store.Get(key); ok {
		prevCount, currCount, windowStart = decodeSlidingWindow(raw)
		elapsed := now.Sub(windowStart)
		if elapsed >= 2*per {
			prevCount, currCount = 0, 0
			windowStart = now
		} else if elapsed >= per {
			prevCount = currCount
			currCount = 0
			windowStart = windowStart.Add(per)
		}
	}

	elapsedInCurrent := now.Sub(windowStart)
	weight := 1 - float64(elapsedInCurrent)/float64(per)
	if weight < 0 {
		weight = 0
	}
	estimated := float64(currCount) + weight*float64(prevCount)

	allowed := estimated < float64(cfg.Rate)
	var retryAfter time.Duration
	if allowed {
		currCount++
	} else {
		retryAfter = per - elapsedInCurrent
	}

	store.Set(key, encodeSlidingWindow(prevCount, currCount, windowStart), 2*per)
	return allowed, retryAfter
}
