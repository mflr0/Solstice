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
		if l.Failure("user:paul") {
			t.Fatal("Failure reported a lock before the limit")
		}
	}
	if d := l.Locked("user:paul"); d != 0 {
		t.Fatalf("locked after %d failures", maxFailures-1)
	}
	if !l.Failure("user:paul") {
		t.Fatal("Failure did not report the lock it started")
	}
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
	if l.Failure("user:paul") {
		t.Fatal("Failure reported a lock that was already running")
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

func TestAccountSlowdownNeverLocks(t *testing.T) {
	clock := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	l := NewLimiter()
	l.now = func() time.Time { return clock }

	// The first failures, from any address, cost nothing.
	for i := range accountFreeFailures {
		if d := l.AccountFailure("account:admin"); d != 0 {
			t.Fatalf("failure %d delayed by %v", i+1, d)
		}
	}
	// Then each wrong answer is held back a little longer, up to a limit.
	prev := time.Duration(0)
	for range 200 {
		d := l.AccountFailure("account:admin")
		if d < prev || d > accountMaxDelay {
			t.Fatalf("delay %v after %v, limit %v", d, prev, accountMaxDelay)
		}
		prev = d
	}
	if prev != accountMaxDelay {
		t.Fatalf("delay stops at %v, want %v", prev, accountMaxDelay)
	}
	// The account itself is never locked, whatever the number of failures.
	if d := l.Locked("account:admin", "user:admin", "ip:203.0.113.9"); d != 0 {
		t.Fatalf("account locked for %v", d)
	}
	// Another account is not slowed.
	if d := l.AccountFailure("account:marie"); d != 0 {
		t.Fatalf("other account delayed by %v", d)
	}
	// A quiet period clears the count.
	clock = clock.Add(failureReset + time.Second)
	if d := l.AccountFailure("account:admin"); d != 0 {
		t.Fatalf("delay %v after a quiet period", d)
	}
}
