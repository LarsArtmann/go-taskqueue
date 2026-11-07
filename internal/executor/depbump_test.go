package executor

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

func depBumpTaskT(t *testing.T, payload DepBumpPayload) task.Task {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return task.Task{ID: task.NewID(), Type: TaskTypeDepBump, Payload: raw}
}

func TestDepBumpTailOutputRedactsSecrets(t *testing.T) {
	t.Parallel()

	filler := strings.Repeat("go get: module fetch line\n", 30)
	out := tailOutput(filler + fakeGitHub + "\nfatal: unable to access")

	if strings.Contains(out, fakeGitHub) {
		t.Error("depbump tail carries the raw secret")
	}

	if !strings.Contains(out, RedactMarker) || !strings.Contains(out, "fatal: unable to access") {
		t.Errorf("tail lost content: %q", out)
	}
}

func TestIsStableSemver(t *testing.T) {
	t.Parallel()

	cases := []struct {
		version string
		want    bool
	}{
		{"v1.2.3", true},
		{"v0.0.1", true},
		{"v10.20.30", true},
		{"", false},
		{"1.2.3", false},    // missing v prefix
		{"v1.2", false},     // two segments
		{"v1.2.3.4", false}, // four segments
		{"v1.2.3-rc1", false},
		{"v1.2.3-dev", false},
		{"v1.2.x", false},
		{"v1.2.-3", false},
		{"0.0.0-dev", false},
	}

	for _, tc := range cases {
		if got := IsStableSemver(tc.version); got != tc.want {
			t.Errorf("IsStableSemver(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}

func TestCommitMessageForBumps(t *testing.T) {
	t.Parallel()

	one := commitMessageForBumps([]DepBump{{Module: "example.com/a", Version: "v1.0.0"}})
	if one != "chore(deps): bump example.com/a to v1.0.0" {
		t.Errorf("single bump message = %q", one)
	}

	many := commitMessageForBumps([]DepBump{
		{Module: "example.com/b", Version: "v2.0.0"},
		{Module: "example.com/a", Version: "v1.0.0"},
	})
	if !strings.HasPrefix(many, "chore(deps): bump 2 modules (") ||
		strings.Index(many, "example.com/a") > strings.Index(many, "example.com/b") {
		t.Errorf("multi bump message must name sorted modules: %q", many)
	}
}

func TestDepBumpPayloadContract(t *testing.T) {
	t.Parallel()

	e := &DepBumpExecutor{}

	// Empty payload is permanent.
	err := e.Execute(context.Background(), task.Task{ID: task.NewID(), Type: TaskTypeDepBump})
	if !isPermanent(t, err) {
		t.Fatalf("empty payload must be permanent, got %v", err)
	}

	if !errors.Is(err, ErrDepBumpEmptyPayload) {
		t.Errorf("sentinel mismatch: %v", err)
	}

	// No repo: permanent.
	err = e.Execute(
		context.Background(),
		depBumpTaskT(t, DepBumpPayload{Bumps: []DepBump{{Module: "m", Version: "v1.0.0"}}}),
	)
	if !isPermanent(t, err) {
		t.Fatalf("missing repo must be permanent, got %v", err)
	}

	// Neither bumps nor release: permanent.
	err = e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{Repo: t.TempDir()}))
	if !isPermanent(t, err) || !errors.Is(err, ErrDepBumpNoWork) {
		t.Fatalf("no-work payload must be permanent ErrDepBumpNoWork, got %v", err)
	}

	// Unstable target: permanent.
	err = e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{
		Repo:  t.TempDir(),
		Bumps: []DepBump{{Module: "m", Version: "v1.2.3-dev"}},
	}))
	if !isPermanent(t, err) || !errors.Is(err, ErrDepBumpBadVersion) {
		t.Fatalf("dev target must be permanent ErrDepBumpBadVersion, got %v", err)
	}

	// Payload contract version from the future: permanent.
	raw := `{"v":99,"repo":"/tmp","bumps":[{"module":"m","version":"v1.0.0"}]}`

	err = e.Execute(context.Background(), task.Task{ID: task.NewID(), Type: TaskTypeDepBump, Payload: []byte(raw)})
	if !isPermanent(t, err) || !strings.Contains(err.Error(), "upgrade tq") {
		t.Fatalf("future payload version must fail permanently with upgrade guidance, got %v", err)
	}
}

func isPermanent(t *testing.T, err error) bool {
	t.Helper()

	permanent, ok := errors.AsType[*PermanentError](err)

	return ok && permanent != nil
}

// TQ_FAKE_GO_SLEEP makes the test binary double as a hermetic fake `go`
// for TestDepBumpRunGoDeadlineClamp: the child sleeps for the duration,
// so the parent observes ctx kills instead of spawning a real toolchain.
func TestDepBumpRunGoFakeSleeper(t *testing.T) {
	raw := os.Getenv("TQ_FAKE_GO_SLEEP")
	if raw == "" {
		t.Skip("parent-only helper: runs only as the fake go subprocess")
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		t.Fatalf("bad TQ_FAKE_GO_SLEEP: %v", err)
	}

	time.Sleep(d)
}

func TestDepBumpTimeoutContextDecision(t *testing.T) {
	t.Parallel()

	origin := time.Now()

	cases := []struct {
		name     string
		deadline time.Time
		hasDL    bool
		timeout  time.Duration
		applied  bool
	}{
		{
			name:    "no deadline applies the timeout",
			timeout: 300 * time.Millisecond,
			applied: true,
		},
		{
			name:     "expired deadline is kept, never resurrected",
			deadline: origin.Add(-time.Millisecond),
			hasDL:    true,
			timeout:  10 * time.Second,
			applied:  false,
		},
		{
			name:     "tighter caller deadline is kept",
			deadline: origin.Add(50 * time.Millisecond),
			hasDL:    true,
			timeout:  10 * time.Second,
			applied:  false,
		},
		{
			name:     "looser caller deadline is tightened",
			deadline: origin.Add(10 * time.Second),
			hasDL:    true,
			timeout:  300 * time.Millisecond,
			applied:  true,
		},
		{
			name:     "deadline equal to the timeout is kept",
			deadline: origin.Add(300 * time.Millisecond),
			hasDL:    true,
			timeout:  300 * time.Millisecond,
			applied:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tc.hasDL {
				var cancel context.CancelFunc

				ctx, cancel = context.WithDeadline(context.Background(), tc.deadline)
				defer cancel()
			}

			got, cancel, applied := depbumpTimeoutContext(ctx, tc.timeout)
			if applied != tc.applied {
				t.Fatalf("applied = %v, want %v", applied, tc.applied)
			}

			if (cancel != nil) != applied {
				t.Fatalf("cancel non-nil = %v, want %v", cancel != nil, applied)
			}

			deadline, ok := got.Deadline()
			if tc.hasDL && !ok {
				t.Fatalf("deadline lost from context")
			}

			if !tc.applied {
				return
			}

			if !ok {
				t.Fatalf("clamped context carries no deadline")
			}

			want := origin.Add(tc.timeout)
			if deadline.Before(want.Add(-50*time.Millisecond)) || deadline.After(want.Add(50*time.Millisecond)) {
				t.Fatalf("clamped deadline %v, want ~%v", deadline, want)
			}
		})
	}
}

// TestDepBumpRunGoDeadlineClamp pins runGo's deadline clamp behaviorally:
// the fake go child sleeps 30s, so elapsed time tells apart a clamped
// context (killed at the runGo timeout) from a kept caller deadline
// (killed at that deadline) — and a wrongly loosened timeout would let
// the child run its full sleep and fail the elapsed bounds.
func TestDepBumpRunGoDeadlineClamp(t *testing.T) {
	t.Parallel()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	e := &DepBumpExecutor{
		GoBin:    bin,
		ExtraEnv: []string{"TQ_FAKE_GO_SLEEP=30s"},
	}
	dir := t.TempDir()
	args := []string{"-test.run=^TestDepBumpRunGoFakeSleeper$"}
	const clamp = 300 * time.Millisecond

	t.Run("no deadline clamps to the runGo timeout", func(t *testing.T) {
		start := time.Now()

		_, err := e.runGo(context.Background(), dir, clamp, args...)

		elapsed := time.Since(start)
		if err == nil {
			t.Fatalf("clamped run must fail (child killed at %v)", clamp)
		}

		if elapsed < 200*time.Millisecond || elapsed > 5*time.Second {
			t.Fatalf("elapsed %v, want a kill at ~%v (not the 30s child sleep)", elapsed, clamp)
		}
	})

	t.Run("looser caller deadline is tightened to the runGo timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		start := time.Now()

		_, err := e.runGo(ctx, dir, clamp, args...)

		elapsed := time.Since(start)
		if err == nil {
			t.Fatalf("tightened run must fail (child killed at %v)", clamp)
		}

		if elapsed < 200*time.Millisecond || elapsed > 5*time.Second {
			t.Fatalf("elapsed %v, want a kill at ~%v (not the caller's 30s deadline)", elapsed, clamp)
		}
	})

	t.Run("tighter caller deadline is kept", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), clamp)
		defer cancel()

		start := time.Now()

		_, err := e.runGo(ctx, dir, 30*time.Second, args...)

		elapsed := time.Since(start)
		if err == nil {
			t.Fatalf("run under a tighter deadline must fail (child killed at %v)", clamp)
		}

		if elapsed < 200*time.Millisecond || elapsed > 5*time.Second {
			t.Fatalf("elapsed %v, want a kill at the caller's ~%v deadline (not a loosened 30s)", elapsed, clamp)
		}
	})

	t.Run("expired caller deadline fails immediately", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Millisecond))
		defer cancel()

		start := time.Now()

		_, err := e.runGo(ctx, dir, 30*time.Second, args...)

		elapsed := time.Since(start)
		if err == nil {
			t.Fatalf("run under an expired deadline must fail without restarting the clock")
		}

		if elapsed > 2*time.Second {
			t.Fatalf("elapsed %v, want an immediate context-deadline failure (the clamp must not resurrect a past deadline into a 30s window)", elapsed)
		}
	})
}
