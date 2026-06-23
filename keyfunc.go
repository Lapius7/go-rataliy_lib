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
// passed as "X-API-Key".
func ByHeader(name string) KeyFunc {
	return func(r *http.Request) string {
		return r.Header.Get(name)
	}
}
