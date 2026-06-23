# go-ratelimit

HTTP rate limiting for Go, with a choice of algorithms and a drop-in
`net/http` middleware. Zero dependencies.

```go
import "github.com/Lapius7/go-ratelimit"
```

## Why

`golang.org/x/time/rate` gives you a token bucket primitive, but you still
have to wire it into your HTTP handlers, pick a key strategy, and handle the
429 response yourself. go-ratelimit does that part, and lets you choose
between three algorithms depending on the trade-off you want.

## Usage

```go
package main

import (
	"net/http"
	"time"

	"github.com/Lapius7/go-ratelimit"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/hello", helloHandler)

	limiter := ratelimit.New(ratelimit.TokenBucket, ratelimit.Config{
		Rate:    60,             // 60 requests
		Per:     time.Minute,    // per minute
		KeyFunc: ratelimit.ByIP, // limit per client IP
	})

	http.ListenAndServe(":8080", limiter.Middleware(mux))
}
```

Requests that exceed the limit receive `429 Too Many Requests` with a
`Retry-After` header. Everything else passes through to your handler
unchanged.

This is a standard `http.Handler` middleware, so it works the same way with
any router or framework that accepts one (Chi, Gorilla, or by wrapping a
framework's own handler type).

### Limiting by API key instead of IP

```go
limiter := ratelimit.New(ratelimit.SlidingWindow, ratelimit.Config{
	Rate:    1000,
	Per:     time.Hour,
	KeyFunc: ratelimit.ByHeader("X-API-Key"),
})
```

### Using the limiter directly, without the middleware

```go
allowed, retryAfter := limiter.Allow("some-key")
if !allowed {
	// wait retryAfter, or reject
}
```

## Algorithms

| Algorithm       | Behavior                                                                 | Memory per key |
|------------------|---------------------------------------------------------------------------|-----------------|
| `TokenBucket`    | Smooths bursts; allows short spikes up to `Burst` (defaults to `Rate`).  | 16 bytes        |
| `SlidingWindow`  | Approximates a true sliding window using a weighted two-window counter. No hard edge at window boundaries. | 16 bytes |
| `FixedWindow`    | Simplest and cheapest; resets sharply at each window boundary, which allows brief 2x bursts across a boundary. | 12 bytes |

If you're not sure which to pick, start with `TokenBucket` — it's the
standard choice for general-purpose API rate limiting.

## Storage

State is held behind the `Store` interface:

```go
type Store interface {
	Get(key string) (state []byte, ok bool)
	Set(key string, state []byte, ttl time.Duration)
}
```

The default is an in-memory store (`sync.Map`-backed, with a background
sweep of expired keys), which is enough for a single process. For rate
limits shared across multiple instances, implement `Store` against Redis or
another shared store and pass it via `Config.Store`.

## License

MIT
