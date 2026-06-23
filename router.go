package ratelimit

import (
	"context"
	"net/http"
)

// Router applies a different Limiter to different request patterns, using
// the same pattern syntax as http.ServeMux (e.g. "/api/", "GET /admin/{id}").
// Requests that don't match any registered pattern pass through unlimited.
type Router struct {
	mux *http.ServeMux
}

// NewRouter creates an empty Router.
func NewRouter() *Router {
	return &Router{mux: http.NewServeMux()}
}

// Handle registers limiter to apply to requests matching pattern.
func (rt *Router) Handle(pattern string, limiter *Limiter) {
	rt.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if m, ok := r.Context().Value(routeMatchKey).(*routeMatch); ok {
			m.limiter = limiter
		}
	})
}

// Middleware wraps next, applying whichever Limiter (if any) matches the
// request's pattern before calling next.
func (rt *Router) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := &routeMatch{}
		ctx := context.WithValue(r.Context(), routeMatchKey, m)
		rt.mux.ServeHTTP(discardResponseWriter{}, r.WithContext(ctx))

		if m.limiter == nil {
			next.ServeHTTP(w, r)
			return
		}
		m.limiter.Middleware(next).ServeHTTP(w, r)
	})
}

type routeMatch struct {
	limiter *Limiter
}

type contextKey int

const routeMatchKey contextKey = 0

// discardResponseWriter satisfies http.ResponseWriter for the internal
// pattern-matching dispatch in Middleware, which never actually writes a
// response — it only runs the handler registered via Handle to find out
// which Limiter (if any) was attached to the matched pattern.
type discardResponseWriter struct{}

func (discardResponseWriter) Header() http.Header       { return http.Header{} }
func (discardResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (discardResponseWriter) WriteHeader(int)           {}
