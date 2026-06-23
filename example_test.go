package ratelimit_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/Lapius7/go-rataliy_lib"
)

// Example demonstrates wrapping a handler with a 60-requests-per-minute
// token bucket limiter, keyed by client IP.
func Example() {
	limiter := ratelimit.New(ratelimit.TokenBucket, ratelimit.Config{
		Rate:    60,
		Per:     time.Minute,
		KeyFunc: ratelimit.ByIP,
	})
	defer limiter.Close()

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
	defer limiter.Close()

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

// Example_router demonstrates applying different limits to different
// endpoints with Router. See the test/ directory for a runnable version
// of this pattern with more than one limiter.
func Example_router() {
	strict := ratelimit.New(ratelimit.FixedWindow, ratelimit.Config{Rate: 2, Per: time.Minute})
	defer strict.Close()

	rt := ratelimit.NewRouter()
	rt.Handle("/admin", strict)

	mux := http.NewServeMux()
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "admin")
	})

	server := httptest.NewServer(rt.Middleware(mux))
	defer server.Close()

	resp, err := http.Get(server.URL + "/admin")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)
	// Output: 200
}
