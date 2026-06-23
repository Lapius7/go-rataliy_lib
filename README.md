# go-rataliy_lib

HTTP rate limiting for Go, with a choice of algorithms and a drop-in
`net/http` middleware. Zero dependencies in the core package.

```go
import "github.com/Lapius7/go-rataliy_lib"
```

A runnable example lives in [`test/`](test/) — clone the repo and `go run`
it to see rate limiting, response headers, and per-route rules in action.

## Why

`golang.org/x/time/rate` gives you a token bucket primitive, but you still
have to wire it into your HTTP handlers, pick a key strategy, and handle the
429 response yourself. go-rataliy_lib does that part, and lets you choose
between three algorithms depending on the trade-off you want.

## Usage

```go
package main

import (
	"net/http"
	"time"

	"github.com/Lapius7/go-rataliy_lib"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/hello", helloHandler)

	limiter := ratelimit.New(ratelimit.TokenBucket, ratelimit.Config{
		Rate:    60,             // 60 requests
		Per:     time.Minute,    // per minute
		KeyFunc: ratelimit.ByIP, // limit per client IP
	})
	defer limiter.Close()

	http.ListenAndServe(":8080", limiter.Middleware(mux))
}
```

Requests that exceed the limit receive `429 Too Many Requests` with a
`Retry-After` header. Every response — allowed or not — also carries
`X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset`
headers so clients can see their budget.

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

### Different limits for different routes

`Router` dispatches to a different `Limiter` per pattern, using the same
syntax as `http.ServeMux`. Unmatched requests pass through unlimited.

```go
strict := ratelimit.New(ratelimit.FixedWindow, ratelimit.Config{Rate: 2, Per: time.Minute})
defer strict.Close()

rt := ratelimit.NewRouter()
rt.Handle("/admin/", strict)

http.ListenAndServe(":8080", rt.Middleware(mux))
```

### Using the limiter directly, without the middleware

```go
result := limiter.Allow("some-key")
if !result.Allowed {
	// wait result.RetryAfter, or reject
}
```

`Allow` returns a `Result` with `Allowed`, `Remaining`, `RetryAfter`, and
`ResetAt` — the same data the middleware puts in response headers.

### Shutting down cleanly

The default in-memory store runs a background goroutine to sweep expired
keys. Call `Limiter.Close()` when you're done with a limiter (e.g. during
graceful shutdown) to stop it.

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

The default is an in-memory store (sweeps expired keys via a background
goroutine — see "Shutting down cleanly" above), which is enough for a
single process. **It does not coordinate across multiple instances**: if
you run several copies of your service behind a load balancer, each one
enforces the configured rate independently, so the effective limit scales
with the number of instances.

For a limit that's shared across instances, use
[`redisstore`](redisstore/), a separate module backed by Redis:

```go
import (
	"github.com/Lapius7/go-rataliy_lib"
	"github.com/Lapius7/go-rataliy_lib/redisstore"
	"github.com/redis/go-redis/v9"
)

client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
store := redisstore.New(client, "myapp:ratelimit:")

limiter := ratelimit.New(ratelimit.TokenBucket, ratelimit.Config{
	Rate:  60,
	Per:   time.Minute,
	Store: store,
})
```

`redisstore` is a separate Go module specifically so the core
`go-rataliy_lib` package keeps zero dependencies — you only pull in
`go-redis` if you actually need distributed limits.

## Known limitations

- `ByIP` reads `RemoteAddr` directly. Behind a reverse proxy, that's the
  proxy's address, not the client's — use `ByHeader("X-Forwarded-For")` (or
  a custom `KeyFunc`) if you need the real client IP, and make sure you
  trust that header in your deployment.
- `SlidingWindow` is an approximation (weighted two-window counter), not an
  exact sliding log. It's accurate enough for typical API limits but can
  be off by a small margin near window boundaries.

## License

MIT
