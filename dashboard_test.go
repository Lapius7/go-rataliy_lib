package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDashboard_SnapshotJSON(t *testing.T) {
	hello := New(TokenBucket, Config{Rate: 5, Per: time.Minute})
	defer hello.Close()
	hello.Allow("1.2.3.4")
	hello.Allow("1.2.3.4")

	strict := New(FixedWindow, Config{Rate: 2, Per: time.Minute})
	defer strict.Close()

	dash := NewDashboard(map[string]*Limiter{
		"hello":  hello,
		"strict": strict,
	})

	server := httptest.NewServer(dash.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/snapshot")
	if err != nil {
		t.Fatalf("GET /api/snapshot: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var views []dashboardLimiterView
	if err := json.NewDecoder(resp.Body).Decode(&views); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("expected 2 limiter views, got %d", len(views))
	}

	byName := make(map[string]dashboardLimiterView)
	for _, v := range views {
		byName[v.Name] = v
	}

	helloView, ok := byName["hello"]
	if !ok {
		t.Fatal("expected a view named 'hello'")
	}
	if !helloView.Enumerable {
		t.Fatal("expected hello's in-memory store to be enumerable")
	}
	if len(helloView.Keys) != 1 {
		t.Fatalf("expected 1 tracked key for hello, got %d", len(helloView.Keys))
	}
	if got := helloView.Keys[0].Remaining; got != 3 {
		t.Fatalf("expected 3 remaining after 2 of 5 requests, got %d", got)
	}

	strictView, ok := byName["strict"]
	if !ok {
		t.Fatal("expected a view named 'strict'")
	}
	if len(strictView.Keys) != 0 {
		t.Fatalf("expected no tracked keys for strict before any request, got %d", len(strictView.Keys))
	}
}

func TestDashboard_HTMLPageServesAtRoot(t *testing.T) {
	l := New(TokenBucket, Config{Rate: 1, Per: time.Minute})
	defer l.Close()

	dash := NewDashboard(map[string]*Limiter{"only": l})
	server := httptest.NewServer(dash.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected HTML content type, got %q", ct)
	}
}

func TestDashboard_UnknownPathIs404(t *testing.T) {
	l := New(TokenBucket, Config{Rate: 1, Per: time.Minute})
	defer l.Close()

	dash := NewDashboard(map[string]*Limiter{"only": l})
	server := httptest.NewServer(dash.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("GET /nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
