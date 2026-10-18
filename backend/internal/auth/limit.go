package auth

import (
	"context"
	"sync"
	"time"
)

// Limiter slows password guessing down. After maxFailures failed attempts
// for the same key, the key is locked for a while, longer at each new lock.
// It also caps how many password checks run at once: each Argon2id check
// takes 19 MiB.
//
// The sign-in code (api.checkPassword) uses two kinds of keys, so that nobody
// can lock an account they do not own:
//
//   - the client address, and the pair (account, client address), are locked:
//     an address that keeps failing is refused, even with the right password.
//     Locking the pair as well makes the attempts on one account from one
//     address escalate on their own. These locks only ever hit the address
//     that failed, never another one.
//   - the account alone, across every address, is never locked: a correct
//     password is always accepted, so an attacker who knows a user name (an
//     administrator's, say) cannot keep its owner out. Once the account has
//     had accountFreeFailures failures from anywhere, each further wrong
//     password is answered later and later (AccountFailure), which makes a
//     guess from many addresses at once slow, and counts double toward the
//     (account, address) lock. The count is by canonical user name, whether
//     or not the account exists, so the answer does not tell.
//
// What this does not do: bound the number of guesses an attacker with very
// many addresses can make, as a hard account lock did. A guess can only be
// told from the right password by checking it, so any bound that refuses
// before checking would also refuse the owner.
type Limiter struct {
	mu       sync.Mutex
	keys     map[string]*attempts
	accounts map[string]*accountFailures
	slots    chan struct{}
	now      func() time.Time
}

// accountFailures counts the recent wrong passwords of an account, from any
// address.
type accountFailures struct {
	failures int
	last     time.Time
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

	accountFreeFailures = 10                     // wrong passwords per account before answers slow down
	accountStep         = 500 * time.Millisecond // delay added by each failure beyond that
	accountMaxDelay     = 10 * time.Second
)

func NewLimiter() *Limiter {
	return &Limiter{keys: map[string]*attempts{}, accounts: map[string]*accountFailures{},
		slots: make(chan struct{}, 4), now: time.Now}
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

// Failure records a failed attempt for each key, and reports whether it
// started a lock on any of them.
func (l *Limiter) Failure(keys ...string) (locked bool) {
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
			locked = true
		}
	}
	if len(l.keys) > 10000 {
		for k, a := range l.keys {
			if now.Sub(a.last) > maxLock+failureReset && a.until.Before(now) {
				delete(l.keys, k)
			}
		}
	}
	return locked
}

// lockCount keeps the escalation of a key that was locked recently.
func lockCount(a *attempts, now time.Time) int {
	if a == nil || now.Sub(a.until) > maxLock {
		return 0
	}
	return a.locks
}

// AccountFailure records a wrong password for an account, whichever address
// it came from, and returns how long the answer to this attempt should be
// held back: nothing for the first accountFreeFailures failures, then
// accountStep more for each, up to accountMaxDelay. It never locks anything.
// Failures older than failureReset are forgotten.
func (l *Limiter) AccountFailure(account string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	a, ok := l.accounts[account]
	if !ok || now.Sub(a.last) > failureReset {
		a = &accountFailures{}
		l.accounts[account] = a
	}
	a.failures++
	a.last = now
	if len(l.accounts) > 10000 {
		for k, o := range l.accounts {
			if now.Sub(o.last) > failureReset {
				delete(l.accounts, k)
			}
		}
	}
	if a.failures <= accountFreeFailures {
		return 0
	}
	return min(time.Duration(a.failures-accountFreeFailures)*accountStep, accountMaxDelay)
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
