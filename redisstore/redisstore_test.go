package redisstore

import "github.com/Lapius7/go-ratelimit"

// Compile-time assertion that Store satisfies ratelimit.Store.
var _ ratelimit.Store = (*Store)(nil)
