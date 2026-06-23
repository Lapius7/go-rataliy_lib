module github.com/Lapius7/go-ratelimit/redisstore

go 1.26.4

require (
	github.com/Lapius7/go-ratelimit v0.0.0
	github.com/redis/go-redis/v9 v9.21.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
)

replace github.com/Lapius7/go-ratelimit => ../
