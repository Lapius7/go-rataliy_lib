package ratelimit

import (
	"math"
	"net/http"
	"strconv"
)

// Middleware wraps next, rejecting requests that exceed the limiter's rate
// with a 429 Too Many Requests response and a Retry-After header.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := l.cfg.KeyFunc(r)
		allowed, retryAfter := l.Allow(key)
		if !allowed {
			seconds := int(math.Ceil(retryAfter.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limit exceeded"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
