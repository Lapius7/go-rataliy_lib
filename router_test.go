package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRouter_AppliesPerPatternLimiter(t *testing.T) {
	strict := New(TokenBucket, Config{Rate: 1, Per: time.Minute})
	loose := New(TokenBucket, Config{Rate: 100, Per: time.Minute})

	rt := NewRouter()
	rt.Handle("/strict", strict)
	rt.Handle("/loose", loose)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rt.Middleware(next)

	req := httptest.NewRequest(http.MethodGet, "/strict", nil)
	req.RemoteAddr = "1.2.3.4:1111"

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first /strict request to succeed, got %d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second /strict request to be limited, got %d", rec2.Code)
	}

	looseReq := httptest.NewRequest(http.MethodGet, "/loose", nil)
	looseReq.RemoteAddr = "1.2.3.4:1111"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, looseReq)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected /loose request to succeed under its own higher limit, got %d", rec3.Code)
	}
}

func TestRouter_UnmatchedPathPassesThrough(t *testing.T) {
	strict := New(TokenBucket, Config{Rate: 1, Per: time.Minute})
	rt := NewRouter()
	rt.Handle("/strict", strict)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rt.Middleware(next)

	req := httptest.NewRequest(http.MethodGet, "/unrelated", nil)
	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected unmatched path to always pass through, got %d", i, rec.Code)
		}
	}
}
