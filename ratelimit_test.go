package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucket_AllowsWithinRate(t *testing.T) {
	l := New(TokenBucket, Config{Rate: 3, Per: time.Minute})
	for i := 0; i < 3; i++ {
		if res := l.Allow("k"); !res.Allowed {
			t.Fatalf("request %d: expected allowed, got denied", i)
		}
	}
	res := l.Allow("k")
	if res.Allowed {
		t.Fatal("expected 4th request to be denied")
	}
	if res.RetryAfter <= 0 {
		t.Fatal("expected positive RetryAfter when denied")
	}
	if res.Remaining != 0 {
		t.Fatalf("expected Remaining 0 when denied, got %d", res.Remaining)
	}
}

func TestTokenBucket_RefillsOverTime(t *testing.T) {
	l := New(TokenBucket, Config{Rate: 10, Per: 100 * time.Millisecond})
	for i := 0; i < 10; i++ {
		if res := l.Allow("k"); !res.Allowed {
			t.Fatalf("request %d: expected allowed", i)
		}
	}
	if res := l.Allow("k"); res.Allowed {
		t.Fatal("expected bucket to be empty")
	}
	time.Sleep(120 * time.Millisecond)
	if res := l.Allow("k"); !res.Allowed {
		t.Fatal("expected bucket to have refilled after waiting")
	}
}

func TestTokenBucket_KeysAreIndependent(t *testing.T) {
	l := New(TokenBucket, Config{Rate: 1, Per: time.Minute})
	if res := l.Allow("a"); !res.Allowed {
		t.Fatal("expected key a to be allowed")
	}
	if res := l.Allow("a"); res.Allowed {
		t.Fatal("expected key a's second request to be denied")
	}
	if res := l.Allow("b"); !res.Allowed {
		t.Fatal("expected key b to be allowed independently of key a")
	}
}

func TestFixedWindow_AllowsWithinRate(t *testing.T) {
	l := New(FixedWindow, Config{Rate: 2, Per: 100 * time.Millisecond})
	if res := l.Allow("k"); !res.Allowed {
		t.Fatal("expected 1st request to be allowed")
	}
	if res := l.Allow("k"); !res.Allowed {
		t.Fatal("expected 2nd request to be allowed")
	}
	if res := l.Allow("k"); res.Allowed {
		t.Fatal("expected 3rd request to be denied")
	}
}

func TestFixedWindow_ResetsAfterWindow(t *testing.T) {
	l := New(FixedWindow, Config{Rate: 1, Per: 80 * time.Millisecond})
	if res := l.Allow("k"); !res.Allowed {
		t.Fatal("expected 1st request to be allowed")
	}
	if res := l.Allow("k"); res.Allowed {
		t.Fatal("expected 2nd request to be denied")
	}
	time.Sleep(100 * time.Millisecond)
	if res := l.Allow("k"); !res.Allowed {
		t.Fatal("expected request to be allowed after window reset")
	}
}

func TestSlidingWindow_AllowsWithinRate(t *testing.T) {
	l := New(SlidingWindow, Config{Rate: 3, Per: 100 * time.Millisecond})
	for i := 0; i < 3; i++ {
		if res := l.Allow("k"); !res.Allowed {
			t.Fatalf("request %d: expected allowed", i)
		}
	}
	if res := l.Allow("k"); res.Allowed {
		t.Fatal("expected 4th request to be denied")
	}
}

func TestSlidingWindow_RecoversAfterFullWindow(t *testing.T) {
	l := New(SlidingWindow, Config{Rate: 2, Per: 80 * time.Millisecond})
	l.Allow("k")
	l.Allow("k")
	if res := l.Allow("k"); res.Allowed {
		t.Fatal("expected request to be denied at capacity")
	}
	time.Sleep(200 * time.Millisecond)
	if res := l.Allow("k"); !res.Allowed {
		t.Fatal("expected request to be allowed after waiting past the window")
	}
}

func TestLimiter_Close(t *testing.T) {
	l := New(TokenBucket, Config{Rate: 1, Per: time.Minute})
	if err := l.Close(); err != nil {
		t.Fatalf("expected Close to succeed, got %v", err)
	}
	// Closing twice must not panic.
	if err := l.Close(); err != nil {
		t.Fatalf("expected second Close to succeed, got %v", err)
	}
}

func TestNew_RejectsInvalidConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"zero rate", Config{Rate: 0, Per: time.Minute}},
		{"negative rate", Config{Rate: -1, Per: time.Minute}},
		{"zero per", Config{Rate: 1, Per: 0}},
		{"negative per", Config{Rate: 1, Per: -time.Minute}},
		{"negative burst", Config{Rate: 1, Per: time.Minute, Burst: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected New to panic on invalid config")
				}
			}()
			New(TokenBucket, tc.cfg)
		})
	}
}
