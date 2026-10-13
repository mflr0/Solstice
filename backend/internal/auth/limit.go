package auth

import (
	"context"
	"sync"
	"time"
)

// Limiter slows password guessing down. After maxFailures failed attempts
// for the same key (a user name, an address), the key is locked for a while,
// longer at each new lock. It also caps how many password checks run at
// once: each Argon2id check takes 19 MiB.
type Limiter struct {
	mu    sync.Mutex
	keys  map[string]*attempts
	slots chan struct{}
	now   func() time.Time
}

type attempts struct {
	failures int
	last     time.Time
	locks    int
	until    time.Time
}

const (
	maxFailures  = 5
	failureReset = 15 * time.Minute // failures older than this are forgotten
	firstLock    = time.Minute
	maxLock      = 15 * time.Minute
)

func NewLimiter() *Limiter {
	return &Limiter{keys: map[string]*attempts{}, slots: make(chan struct{}, 4), now: time.Now}
}

// Locked returns how long the first locked key stays locked, or 0.
func (l *Limiter) Locked(keys ...string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, k := range keys {
		if a, ok := l.keys[k]; ok && a.until.After(now) {
			return a.until.Sub(now)
		}
	}
	return 0
}

// Failure records a failed attempt for each key.
func (l *Limiter) Failure(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, k := range keys {
		a, ok := l.keys[k]
		if !ok || now.Sub(a.last) > failureReset {
			a = &attempts{locks: lockCount(a, now)}
			l.keys[k] = a
		}
		a.failures++
		a.last = now
		if a.failures >= maxFailures {
			lock := firstLock << a.locks
			if lock > maxLock || lock <= 0 {
				lock = maxLock
			}
			a.until = now.Add(lock)
			a.locks++
			a.failures = 0
		}
	}
	if len(l.keys) > 10000 {
		for k, a := range l.keys {
			if now.Sub(a.last) > maxLock+failureReset && a.until.Before(now) {
				delete(l.keys, k)
			}
		}
	}
}

// lockCount keeps the escalation of a key that was locked recently.
func lockCount(a *attempts, now time.Time) int {
	if a == nil || now.Sub(a.until) > maxLock {
		return 0
	}
	return a.locks
}

// Success forgets the failures of the keys.
func (l *Limiter) Success(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.keys, k)
	}
}

// Acquire waits for one of the few password-checking slots.
func (l *Limiter) Acquire(ctx context.Context) (release func(), err error) {
	select {
	case l.slots <- struct{}{}:
		return func() { <-l.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
