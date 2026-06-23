package ratelimit

import (
	"net/http/httptest"
	"testing"
)

func TestByHeader_MissingHeaderSharesOneKey(t *testing.T) {
	keyFunc := ByHeader("X-API-Key")

	reqA := httptest.NewRequest("GET", "/", nil)
	reqB := httptest.NewRequest("GET", "/", nil)

	keyA := keyFunc(reqA)
	keyB := keyFunc(reqB)

	if keyA != keyB {
		t.Fatalf("expected requests missing the header to share a key, got %q and %q", keyA, keyB)
	}
	if keyA == "" {
		t.Fatal("expected the missing-header key to be non-empty so it can't collide with a real empty value")
	}
}

func TestByHeader_DistinctValuesGetDistinctKeys(t *testing.T) {
	keyFunc := ByHeader("X-API-Key")

	reqA := httptest.NewRequest("GET", "/", nil)
	reqA.Header.Set("X-API-Key", "tenant-a")
	reqB := httptest.NewRequest("GET", "/", nil)
	reqB.Header.Set("X-API-Key", "tenant-b")

	if keyFunc(reqA) == keyFunc(reqB) {
		t.Fatal("expected distinct header values to produce distinct keys")
	}
}

func TestByIP_SplitsPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.5:54321"

	if got := ByIP(req); got != "203.0.113.5" {
		t.Fatalf("expected ByIP to strip the port, got %q", got)
	}
}
