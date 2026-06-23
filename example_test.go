package ratelimit_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Lapius7/go-ratelimit"
)

// Example demonstrates wrapping a handler with a 60-requests-per-minute
// token bucket limiter, keyed by client IP.
func Example() {
	limiter := ratelimit.New(ratelimit.TokenBucket, ratelimit.Config{
		Rate:    60,
		Per:     time.Minute,
		KeyFunc: ratelimit.ByIP,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello")
	})

	server := httptest.NewServer(limiter.Middleware(mux))
	defer server.Close()

	resp, err := http.Get(server.URL + "/hello")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)
	// Output: 200
}

// Example_apiKey demonstrates limiting by an API key header instead of IP,
// useful for per-tenant rate limits in a multi-tenant API.
func Example_apiKey() {
	limiter := ratelimit.New(ratelimit.SlidingWindow, ratelimit.Config{
		Rate:    1000,
		Per:     time.Hour,
		KeyFunc: ratelimit.ByHeader("X-API-Key"),
	})

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.Header.Set("X-API-Key", "tenant-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)
	// Output: 200
}
