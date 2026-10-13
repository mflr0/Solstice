package auth

import (
	"testing"
	"time"
)

func TestLimiterLocksAfterRepeatedFailures(t *testing.T) {
	clock := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	l := NewLimiter()
	l.now = func() time.Time { return clock }

	for range maxFailures - 1 {
		l.Failure("user:paul")
	}
	if d := l.Locked("user:paul"); d != 0 {
		t.Fatalf("locked after %d failures", maxFailures-1)
	}
	l.Failure("user:paul")
	if d := l.Locked("ip:1.2.3.4", "user:paul"); d != firstLock {
		t.Fatalf("lock %v, want %v", d, firstLock)
	}

	// Once the lock is over, five more failures lock for twice as long.
	clock = clock.Add(firstLock + time.Second)
	if d := l.Locked("user:paul"); d != 0 {
		t.Fatalf("still locked: %v", d)
	}
	for range maxFailures {
		l.Failure("user:paul")
	}
	if d := l.Locked("user:paul"); d != 2*firstLock {
		t.Fatalf("second lock %v, want %v", d, 2*firstLock)
	}
}

func TestLimiterForgetsOnSuccessAndOverTime(t *testing.T) {
	clock := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	l := NewLimiter()
	l.now = func() time.Time { return clock }
	for range maxFailures - 1 {
		l.Failure("user:marie")
	}
	l.Success("user:marie")
	l.Failure("user:marie")
	if d := l.Locked("user:marie"); d != 0 {
		t.Fatal("success did not reset the count")
	}

	for range maxFailures - 1 {
		l.Failure("user:luc")
	}
	clock = clock.Add(failureReset + time.Minute)
	l.Failure("user:luc")
	if d := l.Locked("user:luc"); d != 0 {
		t.Fatal("old failures still counted")
	}
}
