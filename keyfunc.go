package ratelimit

import (
	"net"
	"net/http"
)

// KeyFunc extracts the rate limit key from an incoming request, e.g. the
// client's IP address or an API key header.
type KeyFunc func(r *http.Request) string

// ByIP keys requests by the client's remote IP address, ignoring the port.
func ByIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ByHeader keys requests by the value of the named header, e.g. an API key
// passed as "X-API-Key". Requests missing the header are grouped under a
// single shared key distinct from any real header value, so they share one
// rate limit budget rather than bypassing the limit individually.
func ByHeader(name string) KeyFunc {
	return func(r *http.Request) string {
		v := r.Header.Get(name)
		if v == "" {
			return "\x00missing:" + name
		}
		return v
	}
}
