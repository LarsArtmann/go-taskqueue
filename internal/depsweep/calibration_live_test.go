package depsweep

// The live-calibration entry the 06-44 §f4 / 05-20 §f12 follow-ups asked
// for: run the REAL planner + sweeper over a REAL projects dir and report
// what a first live `--dep-sweep` pool run would mint — before anyone
// points a real pool at the ecosystem. Deliberately env-gated
// (TQ_DEPSWEEP_LIVE_CALIB=1): hermetic suites stay hermetic (no host
// planner exists in nix checkPhase), and a calibration is an operator
// act, not CI. Zero production side effects by construction: the store is
// scratch sqlite in t.TempDir(), the planner is a read-only analyzer, and
// nothing here EXECUTES the minted tasks — the executor (real tags, real
// commits in real repos) only runs under a supervised pool.

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue"
)

func TestLiveDepSweepCalibration(t *testing.T) {
	if os.Getenv("TQ_DEPSWEEP_LIVE_CALIB") == "" {
		t.Skip("live calibration is env-gated: TQ_DEPSWEEP_LIVE_CALIB=1 TQ_DEPSWEEP_PROJECTS_DIR=<dir> " +
			"[TQ_DEPSWEEP_BIN=<planner>] — runs the real planner over a real projects dir into a scratch journal")
	}

	dir := os.Getenv("TQ_DEPSWEEP_PROJECTS_DIR")
	if dir == "" {
		t.Fatal("TQ_DEPSWEEP_PROJECTS_DIR is required: the projects dir the planner scans")
	}

	s := newTestStore(t)

	sweeper := NewSweeper(s, SweeperConfig{
		Source: DepgraphSource{
			Bin: os.Getenv("TQ_DEPSWEEP_BIN"),
			Dir: dir,
		},
		Log: slog.New(slog.NewTextHandler(testLogWriter{t}, nil)),
	})

	stats, err := sweeper.Sweep(context.Background())
	if err != nil {
		t.Fatalf("live sweep over %s: %v", dir, err)
	}

	t.Logf("live plan over %s: minted=%d known=%d skipped=%d",
		dir, stats.Minted, stats.Known, len(stats.Skips))

	for _, skip := range stats.Skips {
		t.Logf("skip %s: %s", skip.Repo, skip.Reason)
	}

	tasks, err := s.List(context.Background(), queue.Filter{})
	if err != nil {
		t.Fatalf("list minted tasks: %v", err)
	}

	for _, tk := range tasks {
		t.Logf("mint %s %s depsweep-dedup=%s", tk.ID, tk.Project, tk.DedupKey)
	}
}

// testLogWriter routes slog output through t.Log so the calibration log
// lands in the test transcript (-v shows it).
type testLogWriter struct{ t *testing.T }

func (w testLogWriter) Write(p []byte) (int, error) {
	w.t.Log(string(trimTrailingNewline(p)))

	return len(p), nil
}

func trimTrailingNewline(p []byte) []byte {
	if len(p) > 0 && p[len(p)-1] == '\n' {
		return p[:len(p)-1]
	}

	return p
}
