package lockout

import (
	"bytes"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// newTestLimiter builds a Limiter over a fake clock the test advances
// explicitly (row 708: the package had zero test files while being the
// shared constant source for both HTTP surfaces).
func newTestLimiter(t *testing.T, mutate func(*Config)) (*Limiter, *time.Time) {
	t.Helper()

	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

	cfg := Config{
		MaxHits:  DefaultMaxHits,
		Lockout:  DefaultLockout,
		IdleKeep: DefaultIdleKeep,
		MaxKeys:  DefaultMaxKeys,
		Now:      func() time.Time { return now },
	}
	if mutate != nil {
		mutate(&cfg)
	}

	return New(cfg), &now
}

func advance(t *testing.T, now *time.Time, d time.Duration) {
	t.Helper()

	*now = now.Add(d)
}

func TestStrikeTriggerLocksAtMaxHits(t *testing.T) {
	l, now := newTestLimiter(t, nil)

	for range DefaultMaxHits - 1 {
		l.Add("k")
	}

	if _, locked := l.Locked("k"); locked {
		t.Fatalf("locked after %d strikes, want unlocked below MaxHits", DefaultMaxHits-1)
	}

	l.Add("k")

	remain, locked := l.Locked("k")
	if !locked {
		t.Fatal("not locked at MaxHits strikes")
	}

	if remain <= 0 || remain > DefaultLockout {
		t.Fatalf("remaining lockout %v, want (0,%v]", remain, DefaultLockout)
	}

	if l.Len() != 1 || !l.Has("k") {
		t.Fatalf("Len=%d Has=%v, want the key tracked once", l.Len(), l.Has("k"))
	}

	_ = now
}

func TestLockoutWindowExpiresAndRestartsStrikes(t *testing.T) {
	l, now := newTestLimiter(t, nil)

	for range DefaultMaxHits {
		l.Add("k")
	}

	if _, locked := l.Locked("k"); !locked {
		t.Fatal("want locked at MaxHits")
	}

	advance(t, now, DefaultLockout)

	if _, locked := l.Locked("k"); locked {
		t.Fatal("still locked after the window passed")
	}

	// The lock reset the strike count: MaxHits-1 fresh strikes must not
	// re-lock; the MaxHits-th does.
	for range DefaultMaxHits - 1 {
		l.Add("k")
	}

	if _, locked := l.Locked("k"); locked {
		t.Fatal("re-locked before a full fresh strike set")
	}

	l.Add("k")

	if _, locked := l.Locked("k"); !locked {
		t.Fatal("want re-lock on a full fresh strike set")
	}
}

func TestOnLockFiresExactlyOncePerLockout(t *testing.T) {
	var fired []string

	l, _ := newTestLimiter(t, func(c *Config) {
		c.MaxHits = 2
		c.OnLock = func(key string, _ time.Duration) { fired = append(fired, key) }
	})

	l.Add("k") // strike 1
	l.Add("k") // strike 2 → lock

	if len(fired) != 1 {
		t.Fatalf("OnLock fired %d times after one lockout, want 1", len(fired))
	}

	l.Add("k") // strike 1 of the next cycle — must NOT re-fire

	if len(fired) != 1 {
		t.Fatalf("OnLock re-fired without a lockout (%d)", len(fired))
	}

	l.Add("k") // strike 2 → second lock

	if len(fired) != 2 || fired[0] != "k" || fired[1] != "k" {
		t.Fatalf("OnLock fired %v, want exactly one per lockout event", fired)
	}
}

func TestResetClearsStrikes(t *testing.T) {
	l, _ := newTestLimiter(t, func(c *Config) { c.MaxHits = 3 })

	l.Add("k")
	l.Add("k")
	l.Reset("k")

	if !l.Has("k") {
		t.Fatal("Reset keeps the entry (it zeroes count + lock), only idle prune drops it")
	}
	// A success resets the count: two more strikes must not lock.
	l.Add("k")
	l.Add("k")

	if _, locked := l.Locked("k"); locked {
		t.Fatal("locked after Reset + 2 strikes, want the count restarted")
	}

	l.Reset("absent") // must be a no-op, not a panic
}

func TestIdlePruneOnContact(t *testing.T) {
	l, now := newTestLimiter(t, func(c *Config) { c.IdleKeep = 10 * time.Minute })

	l.Add("k")
	advance(t, now, 9*time.Minute)

	if !l.Has("k") {
		t.Fatal("entry dropped before IdleKeep elapsed")
	}

	advance(t, now, 2*time.Minute) // 11m idle

	if _, _ = l.Locked("k"); l.Has("k") {
		t.Fatal("per-contact prune must drop the idle unlocked entry")
	}

	if l.Len() != 0 {
		t.Fatalf("Len=%d after prune, want 0", l.Len())
	}
}

func TestLockedEntrySurvivesIdleWindow(t *testing.T) {
	l, now := newTestLimiter(t, func(c *Config) { c.IdleKeep = 10 * time.Minute })

	for range DefaultMaxHits {
		l.Add("k")
	}

	advance(t, now, DefaultLockout+15*time.Minute) // locked window over AND idle past keep
	// A locked-then-expired entry is NOT idle-pruned while its lockout
	// still covers the contact… but after the lockout lapses the entry is
	// stale (last touch older than IdleKeep): the next contact prunes it.
	if _, _ = l.Locked("k"); l.Has("k") {
		t.Fatal("expired-lock entry must prune on contact past IdleKeep")
	}
}

func TestBoundEvictsLeastRecentlyActiveFirst(t *testing.T) {
	l, now := newTestLimiter(t, func(c *Config) { c.MaxKeys = 3 })

	// Four keys, staggered last-touch times, all fresh (inside IdleKeep) so
	// the idle sweep keeps them and LRA eviction does the work.
	for _, k := range []string{"a", "b", "c"} {
		l.Add(k)
		advance(t, now, time.Minute)
	}

	l.Add("d") // over cap → evicts "a" (oldest touch)

	if l.Has("a") {
		t.Fatal("LRA eviction kept the oldest key")
	}

	for _, k := range []string{"b", "c", "d"} {
		if !l.Has(k) {
			t.Fatalf("lost %v to eviction, want only 'a' evicted", k)
		}
	}

	if l.Len() != 3 {
		t.Fatalf("Len=%d after bound, want the cap enforced", l.Len())
	}
}

func TestBoundIdleSweepRunsBeforeLRA(t *testing.T) {
	l, now := newTestLimiter(t, func(c *Config) {
		c.MaxKeys = 3
		c.IdleKeep = 10 * time.Minute
	})

	l.Add("stale") // will go idle
	advance(t, now, 20*time.Minute)
	l.Add("fresh1") // last-touch t=20m
	l.Add("fresh2") // len 3 ≤ cap, no bound yet
	advance(t, now, time.Minute)
	// stale idled 21m ≥ keep; fresh1/2 idled 1m < keep. The cap pressure
	// arrives with fresh3: len 4 > 3 → the idle pass sweeps stale, LRA
	// never fires.
	l.Add("fresh3")

	if l.Has("stale") {
		t.Fatal("idle sweep should have removed the stale key without LRA eviction")
	}

	if l.Len() > 3 {
		t.Fatalf("Len=%d, want the cap enforced via the idle sweep", l.Len())
	}

	for _, k := range []string{"fresh1", "fresh2", "fresh3"} {
		if !l.Has(k) {
			t.Fatalf("fresh key %v evicted although the idle pass freed a slot", k)
		}
	}
}

func TestBoundIdlePassSparesLockedButLRABoundWins(t *testing.T) {
	l, now := newTestLimiter(t, func(c *Config) { c.MaxKeys = 2 })

	// Two keys locked mid-window + one fresh: the idle pass spares the
	// locked entries (now < lockedUntil), but the LRA pass is lock-blind —
	// the bound wins and the least-recently-active (a locked key) goes.
	for range DefaultMaxHits {
		l.Add("lockA")
	}

	for range DefaultMaxHits {
		l.Add("lockB")
	}

	advance(t, now, DefaultLockout/2) // both still locked
	l.Add("fresh")

	if l.Len() != 2 {
		t.Fatalf("Len=%d, want the cap enforced", l.Len())
	}

	if !l.Has("fresh") {
		t.Fatal("the freshest key must survive LRA eviction")
	}

	kept, gone := 0, 0

	for _, k := range []string{"lockA", "lockB"} {
		if l.Has(k) {
			kept++
		} else {
			gone++
		}
	}

	if kept != 1 || gone != 1 {
		t.Fatalf("want exactly one locked key evicted (kept=%d gone=%d)", kept, gone)
	}
}

func TestBoundEvictsOldestLockedWhenWholeMapLocked(t *testing.T) {
	l, now := newTestLimiter(t, func(c *Config) { c.MaxKeys = 2 })

	// Whole map locked and over cap: the bound wins, oldest locked goes
	// (the docstring's "under that pressure the bound wins" branch).
	for _, k := range []string{"a", "b"} {
		for range DefaultMaxHits {
			l.Add(k)
		}

		advance(t, now, time.Second)
	}

	for range DefaultMaxHits {
		l.Add("c") // all three locked, len 3 > cap 2
	}

	if l.Len() != 2 {
		t.Fatalf("Len=%d, want the cap enforced over an all-locked map", l.Len())
	}

	if !l.Has("b") || !l.Has("c") || l.Has("a") {
		t.Fatal("want the oldest locked key evicted, the rest kept")
	}
}

func TestEvictionWarnsOnce(t *testing.T) {
	var buf bytes.Buffer

	prev := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	l, now := newTestLimiter(t, func(c *Config) { c.MaxKeys = 1 })
	for _, k := range []string{"a", "b", "c"} {
		l.Add(k)
		advance(t, now, time.Minute)
	}

	if n := bytes.Count(buf.Bytes(), []byte("strikes map over cap")); n != 1 {
		t.Fatalf("eviction warning logged %d times, want exactly once (sync.Once)", n)
	}
}

func TestConcurrentAddAndLocked(t *testing.T) {
	l, _ := newTestLimiter(t, nil)

	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()

			key := string(rune('a' + g))
			for range 50 {
				l.Add(key)
				l.Locked(key)

				if g%2 == 0 {
					l.Reset(key)
				}
			}
		}(g)
	}

	wg.Wait()

	if l.Len() > DefaultMaxKeys {
		t.Fatalf("Len=%d exceeded the cap under concurrency", l.Len())
	}
}
