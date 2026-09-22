// Package lockout holds the strike limiter behind both HTTP surfaces: a
// client that fails MaxHits times is locked out of the guarded routes for
// a fixed window; a success resets the strikes. webui's CSRF write routes
// and httpapi's bearer auth share the mechanics so the two surfaces behave
// identically to operators.
package lockout

import (
	"sync"
	"time"
)

// Config builds one Limiter. MaxHits, Lockout, IdleKeep and MaxKeys are
// required (each surface pins its own documented constants).
type Config struct {
	// MaxHits is the strike count that triggers a lockout.
	MaxHits int
	// Lockout is the lockout window, measured from the locking strike.
	Lockout time.Duration
	// IdleKeep is how long an unlocked entry survives without contact
	// before per-contact pruning drops it.
	IdleKeep time.Duration
	// MaxKeys caps the strikes map against rotating source IPs.
	MaxKeys int
	// Now overrides the clock (tests). nil selects time.Now.
	Now func() time.Time
	// OnLock fires once when a key locks out (the surface's warning log).
	// nil logs nothing here.
	OnLock func(key string, lockout time.Duration)
}

// Limiter throttles repeated failures from keyed clients. It is safe for
// concurrent use.
type Limiter struct {
	maxHits  int
	lockout  time.Duration
	idleKeep time.Duration
	maxKeys  int
	nowFunc  func() time.Time
	onLock   func(key string, lockout time.Duration)

	mu      sync.Mutex
	strikes map[string]*strikes
}

type strikes struct {
	count       int
	lockedUntil time.Time
	last        time.Time
}

// New builds a Limiter from cfg.
func New(cfg Config) *Limiter {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return &Limiter{
		maxHits:  cfg.MaxHits,
		lockout:  cfg.Lockout,
		idleKeep: cfg.IdleKeep,
		maxKeys:  cfg.MaxKeys,
		nowFunc:  now,
		onLock:   cfg.OnLock,
		strikes:  make(map[string]*strikes),
	}
}

// Locked reports the remaining lockout for key (false when none), pruning
// the entry on contact when it went idle past IdleKeep unlocked.
func (l *Limiter) Locked(key string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.pruneLocked(key)
	if entry == nil {
		return 0, false
	}

	now := l.nowFunc()
	if now.Before(entry.lockedUntil) {
		return entry.lockedUntil.Sub(now), true
	}

	return 0, false
}

// Add records one failed attempt for key; at MaxHits the key is locked out
// and OnLock (when set) fires once.
func (l *Limiter) Add(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.pruneLocked(key)
	if entry == nil {
		entry = &strikes{}
		l.strikes[key] = entry
	}

	entry.last = l.nowFunc()
	entry.count++

	if entry.count >= l.maxHits {
		entry.lockedUntil = entry.last.Add(l.lockout)
		entry.count = 0

		if l.onLock != nil {
			l.onLock(key, l.lockout)
		}
	}

	l.boundLocked()
}

// Reset clears key's strikes after a success.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.pruneLocked(key)
	if entry == nil {
		return
	}

	entry.count = 0
	entry.lockedUntil = time.Time{}
}

// pruneLocked drops the entry for key when it has been idle past idleKeep
// and is not locked; returns the live entry (or nil) without removing it.
// Caller holds mu.
func (l *Limiter) pruneLocked(key string) *strikes {
	entry, ok := l.strikes[key]
	if !ok {
		return nil
	}

	now := l.nowFunc()
	if now.Before(entry.lockedUntil) || now.Sub(entry.last) < l.idleKeep {
		return entry
	}

	delete(l.strikes, key)

	return nil
}

// boundLocked caps the map against rotating source IPs: per-contact pruning
// only fires for a key's OWN next request, so keys that never come back
// would grow the map without limit. Entries idle past idleKeep (and not
// locked) are swept globally — the exact per-contact predicate applied to
// every key; if the map is still over maxKeys, the least-recently-active
// entries are evicted oldest-first (a locked entry is evicted only when the
// whole map is locked — under that pressure the bound wins).
// Caller holds mu.
func (l *Limiter) boundLocked() {
	if len(l.strikes) <= l.maxKeys {
		return
	}

	now := l.nowFunc()
	for key, entry := range l.strikes {
		if now.Before(entry.lockedUntil) || now.Sub(entry.last) < l.idleKeep {
			continue
		}

		delete(l.strikes, key)
	}

	for len(l.strikes) > l.maxKeys {
		oldestKey := ""

		var oldest *strikes

		for key, entry := range l.strikes {
			if oldest == nil || entry.last.Before(oldest.last) {
				oldestKey, oldest = key, entry
			}
		}

		delete(l.strikes, oldestKey)
	}
}
