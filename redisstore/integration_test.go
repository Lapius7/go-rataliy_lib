//go:build redis_integration

// Run with a Redis instance available on localhost:6379:
//
//	go test -tags redis_integration ./...
package redisstore

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestStore_SetAndGet(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	s := New(client, "ratelimit-test:")

	if _, ok := s.Get("missing"); ok {
		t.Fatal("expected missing key to return ok=false")
	}

	s.Set("k", []byte("hello"), time.Minute)

	val, ok := s.Get("k")
	if !ok {
		t.Fatal("expected key to be found after Set")
	}
	if string(val) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", val)
	}
}
