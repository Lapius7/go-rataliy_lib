package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucket_AllowsWithinRate(t *testing.T) {
	l := New(TokenBucket, Config{Rate: 3, Per: time.Minute})
	for i := 0; i < 3; i++ {
		allowed, _ := l.Allow("k")
		if !allowed {
			t.Fatalf("request %d: expected allowed, got denied", i)
		}
	}
	allowed, retryAfter := l.Allow("k")
	if allowed {
		t.Fatal("expected 4th request to be denied")
	}
	if retryAfter <= 0 {
		t.Fatal("expected positive retryAfter when denied")
	}
}

func TestTokenBucket_RefillsOverTime(t *testing.T) {
	l := New(TokenBucket, Config{Rate: 10, Per: 100 * time.Millisecond})
	for i := 0; i < 10; i++ {
		if allowed, _ := l.Allow("k"); !allowed {
			t.Fatalf("request %d: expected allowed", i)
		}
	}
	if allowed, _ := l.Allow("k"); allowed {
		t.Fatal("expected bucket to be empty")
	}
	time.Sleep(120 * time.Millisecond)
	if allowed, _ := l.Allow("k"); !allowed {
		t.Fatal("expected bucket to have refilled after waiting")
	}
}

func TestTokenBucket_KeysAreIndependent(t *testing.T) {
	l := New(TokenBucket, Config{Rate: 1, Per: time.Minute})
	if allowed, _ := l.Allow("a"); !allowed {
		t.Fatal("expected key a to be allowed")
	}
	if allowed, _ := l.Allow("a"); allowed {
		t.Fatal("expected key a's second request to be denied")
	}
	if allowed, _ := l.Allow("b"); !allowed {
		t.Fatal("expected key b to be allowed independently of key a")
	}
}

func TestFixedWindow_AllowsWithinRate(t *testing.T) {
	l := New(FixedWindow, Config{Rate: 2, Per: 100 * time.Millisecond})
	if allowed, _ := l.Allow("k"); !allowed {
		t.Fatal("expected 1st request to be allowed")
	}
	if allowed, _ := l.Allow("k"); !allowed {
		t.Fatal("expected 2nd request to be allowed")
	}
	if allowed, _ := l.Allow("k"); allowed {
		t.Fatal("expected 3rd request to be denied")
	}
}

func TestFixedWindow_ResetsAfterWindow(t *testing.T) {
	l := New(FixedWindow, Config{Rate: 1, Per: 80 * time.Millisecond})
	if allowed, _ := l.Allow("k"); !allowed {
		t.Fatal("expected 1st request to be allowed")
	}
	if allowed, _ := l.Allow("k"); allowed {
		t.Fatal("expected 2nd request to be denied")
	}
	time.Sleep(100 * time.Millisecond)
	if allowed, _ := l.Allow("k"); !allowed {
		t.Fatal("expected request to be allowed after window reset")
	}
}

func TestSlidingWindow_AllowsWithinRate(t *testing.T) {
	l := New(SlidingWindow, Config{Rate: 3, Per: 100 * time.Millisecond})
	for i := 0; i < 3; i++ {
		if allowed, _ := l.Allow("k"); !allowed {
			t.Fatalf("request %d: expected allowed", i)
		}
	}
	if allowed, _ := l.Allow("k"); allowed {
		t.Fatal("expected 4th request to be denied")
	}
}

func TestSlidingWindow_RecoversAfterFullWindow(t *testing.T) {
	l := New(SlidingWindow, Config{Rate: 2, Per: 80 * time.Millisecond})
	l.Allow("k")
	l.Allow("k")
	if allowed, _ := l.Allow("k"); allowed {
		t.Fatal("expected request to be denied at capacity")
	}
	time.Sleep(200 * time.Millisecond)
	if allowed, _ := l.Allow("k"); !allowed {
		t.Fatal("expected request to be allowed after waiting past the window")
	}
}
