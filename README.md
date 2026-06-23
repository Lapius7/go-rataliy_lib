# go-rataliy_lib

[日本語](README.ja.md)

[![CI](https://github.com/Lapius7/go-rataliy_lib/actions/workflows/ci.yml/badge.svg)](https://github.com/Lapius7/go-rataliy_lib/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Lapius7/go-rataliy_lib.svg)](https://pkg.go.dev/github.com/Lapius7/go-rataliy_lib)
[![Go Report Card](https://goreportcard.com/badge/github.com/Lapius7/go-rataliy_lib)](https://goreportcard.com/report/github.com/Lapius7/go-rataliy_lib)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

HTTP rate limiting for Go, with a choice of algorithms and a drop-in
`net/http` middleware. Zero dependencies in the core package.

```go
import "github.com/Lapius7/go-rataliy_lib"
```

A runnable example lives in [`test/`](test/) — clone the repo and `go run`
it to see rate limiting, response headers, and per-route rules in action.

## Contents

- [Why](#why)
- [Quick start](#quick-start)
- [Limiting by API key instead of IP](#limiting-by-api-key-instead-of-ip)
- [Different limits for different routes](#different-limits-for-different-routes)
- [Using the limiter directly, without the middleware](#using-the-limiter-directly-without-the-middleware)
- [Shutting down cleanly](#shutting-down-cleanly)
- [Algorithms](#algorithms)
- [Storage](#storage)
- [Known limitations](#known-limitations)
- [FAQ](#faq)
- [Contributing](#contributing)
- [License](#license)

## Why

`golang.org/x/time/rate` gives you a token bucket primitive, but you still
have to wire it into your HTTP handlers, pick a key strategy, and handle the
429 response yourself. go-rataliy_lib does that part, and lets you choose
between three algorithms depending on the trade-off you want — without
pulling in any dependencies to do it.

## Quick start

```bash
go get github.com/Lapius7/go-rataliy_lib
```

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

`ratelimit.New` panics if `Config.Rate` or `Config.Per` is not positive —
fail fast on a misconfiguration rather than silently rate-limiting nothing
(or everything).

## Limiting by API key instead of IP

```go
limiter := ratelimit.New(ratelimit.SlidingWindow, ratelimit.Config{
	Rate:    1000,
	Per:     time.Hour,
	KeyFunc: ratelimit.ByHeader("X-API-Key"),
})
```

Requests with no `X-API-Key` header all share a single fallback budget —
they don't each get their own unlimited bucket, and they don't collide with
a request that legitimately sent an empty header value.

## Different limits for different routes

`Router` dispatches to a different `Limiter` per pattern, using the same
syntax as `http.ServeMux`. Unmatched requests pass through unlimited.

```go
strict := ratelimit.New(ratelimit.FixedWindow, ratelimit.Config{Rate: 2, Per: time.Minute})
defer strict.Close()

rt := ratelimit.NewRouter()
rt.Handle("/admin/", strict)

http.ListenAndServe(":8080", rt.Middleware(mux))
```

## Using the limiter directly, without the middleware

```go
result := limiter.Allow("some-key")
if !result.Allowed {
	// wait result.RetryAfter, or reject
}
```

`Allow` returns a `Result` with `Allowed`, `Remaining`, `RetryAfter`, and
`ResetAt` — the same data the middleware puts in response headers.

## Shutting down cleanly

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
goroutine — see [Shutting down cleanly](#shutting-down-cleanly) above),
which is enough for a single process. **It does not coordinate across
multiple instances**: if you run several copies of your service behind a
load balancer, each one enforces the configured rate independently, so the
effective limit scales with the number of instances.

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

Read this before you ship — none of these are bugs, but each one changes
how the limiter behaves under conditions you should plan for.

- **Unbounded key cardinality (in-memory store).** The default store has no
  cap on the number of distinct keys it tracks, only a TTL-based sweep. A
  client that can generate many distinct keys quickly — e.g. spoofed
  `X-Forwarded-For` values if you key by an untrusted header — can grow
  memory usage for the lifetime of each key's TTL. If your `KeyFunc` derives
  from anything client-controlled, keep the TTL (`Config.Per`, roughly)
  short, or supply a `Store` with its own eviction policy.
- **Redis failures fail open.** `redisstore.Store.Get` treats *every* Redis
  error — including a timeout or a dropped connection — the same as "key not
  found." If Redis is unreachable, every request looks like a fresh bucket
  and is allowed. This favors availability over strict enforcement, which
  is usually the right trade-off for a rate limiter, but it does mean an
  outage in Redis silently disables your rate limiting rather than
  rejecting traffic. Monitor Redis availability separately if you're
  relying on the limit for abuse prevention rather than just smoothing load.
- **`ByIP` trusts `RemoteAddr` directly.** Behind a reverse proxy, that's
  the proxy's address, not the client's — every client behind the proxy
  shares one bucket. Use `ByHeader("X-Forwarded-For")` (or a custom
  `KeyFunc`) if you need the real client IP, and only do so if your
  deployment guarantees that header is set by a proxy you trust, not by the
  client directly (an untrusted `X-Forwarded-For` lets a client claim any
  IP it wants and dodge the limit).
- **`SlidingWindow` is an approximation** (a weighted two-window counter),
  not an exact sliding log of timestamps. It's accurate enough for typical
  API limits but can admit slightly more or fewer requests than the nominal
  rate near window boundaries.
- **Custom `Store` implementations must round-trip exact byte slices.**
  Each algorithm encodes its state as a small fixed-size binary blob and
  assumes whatever `Get` returns is exactly what a prior `Set` wrote (or
  nothing). A `Store` that truncates, re-encodes, or otherwise mutates the
  bytes will cause a panic or incorrect limiting — don't interpret or
  reformat the `state` argument, just store and return it as-is.

## FAQ

**Does this work with Gin / Echo / Chi / [framework]?**
Yes — `Limiter.Middleware` and `Router.Middleware` are plain
`func(http.Handler) http.Handler`. Any framework that can wrap a standard
handler (or expose one to wrap) can use them directly.

**Can I rate limit gRPC or non-HTTP traffic?**
Not via the middleware, but `Limiter.Allow(key) Result` has no HTTP
dependency — call it directly with whatever key makes sense (e.g. a peer
address from `peer.FromContext` in gRPC) and act on `Result.Allowed`
yourself.

**Why panic in `New` instead of returning an error?**
An invalid `Config` (zero or negative `Rate`/`Per`) is a programming error,
not a runtime condition to recover from — the same category as an
out-of-range slice index. Panicking at startup, before any traffic is
served, surfaces the mistake immediately instead of letting a
misconfigured limiter silently admit or reject everything in production.

## Contributing

Issues and pull requests are welcome. Before sending a PR:

```bash
go build ./... && go vet ./... && go test ./... -race
gofmt -l .   # should print nothing
```

If you change `redisstore/`, also run `go build ./...` and `go vet ./...`
inside that directory (it's a separate module). The Redis-backed
integration test is gated behind a build tag and needs a local Redis:

```bash
cd redisstore
go test -tags redis_integration ./...
```

## License

MIT — see [LICENSE](LICENSE).
