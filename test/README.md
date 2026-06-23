# go-rataliy_lib example

A runnable demo of [go-rataliy_lib](https://github.com/Lapius7/go-rataliy_lib):
two endpoints with different limits, dispatched through a `Router`.

## Run it

```bash
git clone https://github.com/Lapius7/go-rataliy_lib
cd go-rataliy_lib/test
go run .
```

This starts a server on `:18181` with:

- `/hello` — token bucket, 5 requests/minute
- `/strict` — fixed window, 2 requests/minute

## Try it

In another terminal:

```bash
# First 5 requests succeed, the 6th is rate limited
for i in $(seq 1 6); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:18181/hello; done

# Inspect the rate limit headers and the 429 response
curl -i http://localhost:18181/hello

# /strict has its own, lower limit
for i in $(seq 1 3); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:18181/strict; done
```

Every response carries `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and
`X-RateLimit-Reset` headers so you can watch the budget deplete. A denied
request also gets a `Retry-After` header telling you how long to wait.

## Using your own checkout

`go.mod` in this directory has a `replace` pointing at the parent
directory, so `go run .` always uses whatever is in your local
`go-rataliy_lib` checkout — useful if you're modifying the library itself.
