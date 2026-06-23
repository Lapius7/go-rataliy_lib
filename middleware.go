package ratelimit

import (
	"math"
	"net/http"
	"strconv"
)

// Middleware wraps next, rejecting requests that exceed the limiter's rate
// with a 429 Too Many Requests response and a Retry-After header. All
// requests (allowed or not) receive X-RateLimit-Limit, X-RateLimit-Remaining,
// and X-RateLimit-Reset headers.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := l.cfg.KeyFunc(r)
		result := l.Allow(key)

		h := w.Header()
		h.Set("X-RateLimit-Limit", strconv.Itoa(l.cfg.Rate))
		h.Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		h.Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed {
			seconds := int(math.Ceil(result.RetryAfter.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			h.Set("Retry-After", strconv.Itoa(seconds))
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limit exceeded"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
