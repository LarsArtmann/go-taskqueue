package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func TestHarvestConfigFromOptionsExpandsBareRepoNames(t *testing.T) {
	t.Parallel()

	// Inputs and expectations follow the running OS's path rules: POSIX
	// string literals failed windows-latest (filepath.Join emits
	// backslashes and IsAbs wants a volume there), and the expansion's
	// cross-platform honesty is part of what this test pins.
	volume := ""
	if runtime.GOOS == "windows" {
		volume = `C:`
	}

	root := volume + string(filepath.Separator)
	projectsDir := filepath.Join(root, "home", "lars", "projects")
	srv := func(name string) string { return filepath.Join(root, "srv", name) }

	tests := []struct {
		name  string
		repos string
		want  []string
	}{
		{
			name:  "bare names join the projects dir",
			repos: "CV,go-taskqueue",
			want:  []string{filepath.Join(projectsDir, "CV"), filepath.Join(projectsDir, "go-taskqueue")},
		},
		{
			name:  "absolute repos stay untouched",
			repos: srv("cv") + "," + filepath.Join(projectsDir, "go-taskqueue"),
			want:  []string{srv("cv"), filepath.Join(projectsDir, "go-taskqueue")},
		},
		{
			name:  "mixed entries with spacing",
			repos: " overview , " + srv("overview"),
			want:  []string{filepath.Join(projectsDir, "overview"), srv("overview")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := harvestConfigFromOptions(agentPoolOptions{
				projectsDir: projectsDir,
				repos:       tt.repos,
			})
			if err != nil {
				t.Fatalf("harvestConfigFromOptions: %v", err)
			}

			if cfg.ProjectsDir != "" {
				t.Fatalf("ProjectsDir = %q, want \"\" when --repos is set", cfg.ProjectsDir)
			}

			if len(cfg.Repos) != len(tt.want) {
				t.Fatalf("Repos = %v, want %v", cfg.Repos, tt.want)
			}

			for i := range tt.want {
				if want := filepath.Clean(tt.want[i]); cfg.Repos[i] != want {
					t.Fatalf("Repos[%d] = %q, want %q", i, cfg.Repos[i], want)
				}
			}
		})
	}
}

func TestHarvestConfigFromOptionsKeepsProjectsDirForDiscovery(t *testing.T) {
	t.Parallel()

	cfg, err := harvestConfigFromOptions(agentPoolOptions{projectsDir: "/home/lars/projects"})
	if err != nil {
		t.Fatalf("harvestConfigFromOptions: %v", err)
	}

	if cfg.ProjectsDir != "/home/lars/projects" {
		t.Fatalf("ProjectsDir = %q, want it preserved for discovery when --repos is unset", cfg.ProjectsDir)
	}
}

// TestParseAgentPoolOptionsBatchItems pins the batching flag: the DEFAULT is
// 3 (2026-10-11 micro-batch waste fix — docs/problems/major-prompting-and-batching.md),
// explicit values within 0..10 plumb into the harvest config, values above
// fail fast (one session working more than ~10 items drowns in its own history).
func TestParseAgentPoolOptionsBatchItems(t *testing.T) {
	t.Parallel()

	opts, err := parseAgentPoolOptions([]string{"--projects-dir", t.TempDir(), "--batch-items", "3"})
	if err != nil {
		t.Fatalf("parse --batch-items 3: %v", err)
	}

	if opts.batchItems != 3 {
		t.Fatalf("batchItems = %d, want 3", opts.batchItems)
	}

	cfg, err := harvestConfigFromOptions(opts)
	if err != nil {
		t.Fatalf("harvestConfigFromOptions: %v", err)
	}

	if cfg.BatchItems != 3 {
		t.Fatalf("harvest cfg BatchItems = %d, want the flag plumbed through", cfg.BatchItems)
	}

	def, err := parseAgentPoolOptions([]string{"--projects-dir", t.TempDir()})
	if err != nil {
		t.Fatalf("parse defaults: %v", err)
	}

	if def.batchItems != 3 {
		t.Fatalf("default batchItems = %d, want 3 (batching on by default)", def.batchItems)
	}

	if _, err := parseAgentPoolOptions([]string{"--projects-dir", t.TempDir(), "--batch-items", "11"}); err == nil {
		t.Fatal("--batch-items 11 must fail fast (context-explosion guard)")
	}
}

// TestParseAgentPoolOptionsModelRetired pins the .crushrc ruling (05-29
// e4/g2): model + effort live ONLY in the repo .crushrc managed block — a
// payload-level model makes the executor pass `crush run -m`, which RESETS
// the reasoning effort, so the flag must refuse at options-parse time, via
// the CLI AND a pool-config `model =` line alike.
func TestParseAgentPoolOptionsModelRetired(t *testing.T) {
	t.Parallel()

	opts, err := parseAgentPoolOptions([]string{"--projects-dir", t.TempDir()})
	if err != nil || opts.model != "" {
		t.Fatalf("parse without --model: opts=%+v err=%v, want a clean parse", opts, err)
	}

	_, err = parseAgentPoolOptions([]string{"--projects-dir", t.TempDir(), "--model", "anthropic/claude-sonnet-4-5"})
	if err == nil {
		t.Fatal("--model must refuse (a payload-level model resets the reasoning effort)")
	}

	for _, want := range []string{"retired", ".crushrc", "tq bootstrap"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q must carry the %q remediation", err, want)
		}
	}

	conf := filepath.Join(t.TempDir(), "pool.conf")
	if err := os.WriteFile(conf, []byte("model = anthropic/claude-sonnet-4-5\n"), 0o600); err != nil {
		t.Fatalf("write pool config: %v", err)
	}

	if _, err := parseAgentPoolOptions([]string{"--projects-dir", t.TempDir(), "--config", conf}); err == nil {
		t.Fatal("a pool-config `model =` line must refuse exactly like the CLI flag")
	}
}

// TestGroupedSkipsKeepsFullExample pins the observability contract: the
// aggregate harvest-skip log line must carry one full example reason, or a
// pool that cannot see any repo ("scan failed: open …: no such file") reads
// as a healthy quiet one.
func TestGroupedSkipsKeepsFullExample(t *testing.T) {
	t.Parallel()

	skips := []harvest.Skipped{
		{Reason: "scan failed: open /mnt/pool/services/tq/CV/TODO_LIST.md: no such file or directory"},
		{Reason: "scan failed: open /mnt/pool/services/tq/go-taskqueue/TODO_LIST.md: no such file or directory"},
		{Reason: "blocked: owner"},
	}

	groups := groupedSkips(skips)

	scan, ok := groups[harvest.ReasonScanFailed]
	if !ok {
		t.Fatalf("no %q group in %v", harvest.ReasonScanFailed, groups)
	}

	if scan.count != 2 {
		t.Fatalf("count = %d, want 2", scan.count)
	}

	want := "scan failed: open /mnt/pool/services/tq/CV/TODO_LIST.md: no such file or directory"
	if scan.example != want {
		t.Fatalf("example = %q, want the full first reason %q", scan.example, want)
	}

	if blocked := groups["blocked"]; blocked.count != 1 || blocked.example != "blocked: owner" {
		t.Fatalf("blocked group = %+v, want count 1 with the full reason", blocked)
	}
}

func TestGroupedSkipsTruncatesLongExamples(t *testing.T) {
	t.Parallel()

	groups := groupedSkips([]harvest.Skipped{{Reason: "blocked: " + strings.Repeat("x", 250)}})

	got := groups["blocked"].example
	if utf8.RuneCountInString(got) != 201 || !strings.HasSuffix(got, "…") {
		t.Fatalf("long example not truncated to 200 runes + ellipsis: len=%d tail=%q", len(got), got[len(got)-5:])
	}
}

// TestDeadPoolDetectorStreakLifecycle pins the dead-pool detection
// contract: a streak counts CONSECUTIVE all-repos scan-failed ticks, fires
// its notify exactly once per streak at the configured tick, and a
// readable tick resolves the standing alert and resets the count.
func TestDeadPoolDetectorStreakLifecycle(t *testing.T) {
	t.Parallel()

	blind := harvest.Result{Repos: 2, Skipped: []harvest.Skipped{
		{Reason: "scan failed: open /srv/CV/TODO_LIST.md: no such file or directory"},
		{Reason: "scan failed: open /srv/go-taskqueue/TODO_LIST.md: no such file or directory"},
	}}
	healthy := harvest.Result{Repos: 2, Skipped: []harvest.Skipped{{Reason: "blocked: owner"}}}
	partial := harvest.Result{Repos: 2, Skipped: []harvest.Skipped{
		{Reason: "scan failed: open /srv/CV/TODO_LIST.md: no such file or directory"},
	}}

	tests := []struct {
		name       string
		ticks      int
		observes   []harvest.Result
		wantNotifs []string
		wantStreak int
	}{
		{
			name:     "disabled detector never notifies",
			ticks:    0,
			observes: []harvest.Result{blind, blind, blind},
		},
		{
			name:     "zero watched repos means nothing to go blind on",
			ticks:    1,
			observes: []harvest.Result{{Repos: 0}},
		},
		{
			name:       "streak below the threshold stays quiet",
			ticks:      3,
			observes:   []harvest.Result{blind, blind},
			wantStreak: 2,
		},
		{
			name:       "full streak fires once, not per extra tick",
			ticks:      3,
			observes:   []harvest.Result{blind, blind, blind, blind},
			wantNotifs: []string{"triggered"},
			wantStreak: 4,
		},
		{
			name:     "partial blindness never counts as a dead pool",
			ticks:    2,
			observes: []harvest.Result{partial, partial, partial},
		},
		{
			name:       "recovery resolves, resets, and can re-fire",
			ticks:      2,
			observes:   []harvest.Result{blind, blind, healthy, blind, blind},
			wantNotifs: []string{"triggered", "resolved", "triggered"},
			wantStreak: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got []string

			d := &deadPoolDetector{ticks: tt.ticks}
			d.notify = func(triggered bool, _ int, _ string, _ int) bool {
				if triggered {
					got = append(got, "triggered")
				} else {
					got = append(got, "resolved")
				}

				return true
			}

			for _, res := range tt.observes {
				d.observe(res)
			}

			if len(got) != len(tt.wantNotifs) {
				t.Fatalf("notifications = %v, want %v", got, tt.wantNotifs)
			}

			for i := range got {
				if got[i] != tt.wantNotifs[i] {
					t.Fatalf("notifications = %v, want %v", got, tt.wantNotifs)
				}
			}

			if d.streak != tt.wantStreak {
				t.Errorf("final streak = %d, want %d", d.streak, tt.wantStreak)
			}
		})
	}
}

// TestDeadPoolDetectorRetriesFailedDelivery pins the 06-06 §b1 fix: a
// transient notify failure must NOT arm the streak, so the next blind
// tick retries the raise. The old code set alerted=true BEFORE calling
// notify — one failed post burned the streak's only alert forever.
func TestDeadPoolDetectorRetriesFailedDelivery(t *testing.T) {
	t.Parallel()

	blind := harvest.Result{Repos: 1, Skipped: []harvest.Skipped{
		{Reason: "scan failed: no TODO_LIST.md"},
	}}

	var calls int

	d := &deadPoolDetector{ticks: 2}
	d.notify = func(triggered bool, _ int, _ string, _ int) bool {
		calls++

		return calls > 1
	}

	d.observe(blind) // streak 1 — quiet
	d.observe(blind) // streak 2 — fires, delivery fails
	d.observe(blind) // streak 3 — retries the raise, delivered

	if calls != 2 {
		t.Fatalf("raise attempts = %d, want 2 (failed delivery must retry next tick)", calls)
	}

	d.observe(blind)

	if calls != 2 {
		t.Fatalf("raise attempts = %d, want 2 (the delivered alert must arm the streak)", calls)
	}

	d.notify = func(bool, int, string, int) bool { return true }
	d.observe(harvest.Result{Repos: 1, Skipped: []harvest.Skipped{{Reason: "harvested"}}})

	if d.alerted {
		t.Error("healthy tick must clear the standing alert")
	}
}

// TestWorkerDeathDetectorLifecycle pins the B9.3 marker watch: a standing
// marker raises once, repeats stay quiet, and a cleared marker (a fresh
// host) resolves. An empty db path (postgres deployment: no sqlite
// readmodel sidecar) never notifies.
func TestWorkerDeathDetectorLifecycle(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "q.db")

	var got []string

	d := &workerDeathDetector{dbPath: dbPath}
	d.notify = func(triggered bool, _ string, _ time.Time) bool {
		if triggered {
			got = append(got, "triggered")
		} else {
			got = append(got, "resolved")
		}

		return true
	}

	d.observe() // healthy: no marker

	if len(got) != 0 {
		t.Fatalf("healthy observe notified %v, want none", got)
	}

	failedAt := time.Now().Add(-10 * time.Minute)
	if err := readmodel.WriteWorkerFailedMarker(dbPath, readmodel.WorkerFailedMarker{
		Projection: "tasks",
		FailedAt:   failedAt,
	}); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	d.observe() // standing marker raises
	d.observe() // repeats stay quiet

	if len(got) != 1 || got[0] != "triggered" {
		t.Fatalf("notifications = %v, want [triggered]", got)
	}

	if err := os.Remove(readmodel.WorkerFailedMarkerPath(dbPath)); err != nil {
		t.Fatalf("remove marker: %v", err)
	}

	d.observe() // fresh host resolves

	if len(got) != 2 || got[1] != "resolved" {
		t.Fatalf("notifications = %v, want [triggered resolved]", got)
	}

	empty := &workerDeathDetector{dbPath: ""}
	empty.notify = func(bool, string, time.Time) bool {
		t.Error("empty db path (postgres deployment) must never notify")

		return true
	}

	empty.observe()
}

// TestWorkerDeathDetectorRetriesFailedDelivery mirrors the 06-06 §b1 rule
// for the marker watch: a failed raise delivery must retry the next tick
// instead of arming silently.
func TestWorkerDeathDetectorRetriesFailedDelivery(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "q.db")

	if err := readmodel.WriteWorkerFailedMarker(dbPath, readmodel.WorkerFailedMarker{
		Projection: "tasks",
		FailedAt:   time.Now(),
	}); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	var calls int

	d := &workerDeathDetector{dbPath: dbPath}
	d.notify = func(bool, string, time.Time) bool {
		calls++

		return calls > 1
	}

	d.observe() // raise attempt 1 — delivery fails
	d.observe() // raise attempt 2 — delivered

	if calls != 2 {
		t.Fatalf("raise attempts = %d, want 2", calls)
	}

	d.observe() // armed: no third call

	if calls != 2 {
		t.Fatalf("raise attempts = %d, want 2 (the delivered alert must arm)", calls)
	}
}

// TestFrozenSweepersDetectorLifecycle pins the B10.2 freeze contract over
// a real store: a watched consumer whose cursor stands still while the
// journal head advances accumulates a streak and fires once, a consumer
// that never ran is skipped, and a cursor advancing again resolves the
// standing alert.
// TestWorktreeInFlightTerminalStatusWins pins the reaper's liveness probe
// (00-11 report e5 closeoutPending lifetime audit): a PENDING task is
// in-flight, a CANCELLED task is NOT — the store's terminal status must
// outrank any parked executor entry, because a cancelled task never
// re-claims and a stale closeoutPending record would otherwise pin its
// worktree/branch until process restart.
func TestWorktreeInFlightTerminalStatusWins(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	defer func() { _ = s.Close() }()

	probe := worktreeInFlight(ctx, s, nil)

	tk, err := s.Enqueue(ctx, task.New{Type: "sh"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if !probe(tk.ID.String()) {
		t.Fatalf("PENDING task reported not in-flight")
	}

	if err := s.Cancel(ctx, tk.ID, "test: prune-stale"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if probe(tk.ID.String()) {
		t.Fatalf(
			"CANCELLED task still reported in-flight — stale closeoutPending entries would pin the worktree forever",
		)
	}

	if probe("00000000000000000000000000000000") {
		t.Fatalf("unknown task id reported in-flight")
	}
}

// TestWorktreeInFlightCancelledParkedWithExecutor pins the exact
// regression shape the store-status-side test above could not: an agent
// executor that ACTUALLY holds a parked closeoutPending entry for a
// CANCELLED task must still report the worktree not in-flight (terminal
// status wins over the executor OR branch). The entry is armed via the
// executor's test-only injection (ParkCloseoutForTest) — the same map
// runAgent's rate-limited close-out path fills — and a PENDING control
// proves the parked entry itself is what drives the OR branch.
func TestWorktreeInFlightCancelledParkedWithExecutor(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	defer func() { _ = s.Close() }()

	agentExec := executor.NewAgentExecutor(t.TempDir())
	probe := worktreeInFlight(ctx, s, agentExec)

	cancelled, err := s.Enqueue(ctx, task.New{Type: "agent"})
	if err != nil {
		t.Fatalf("enqueue cancelled: %v", err)
	}

	if err := s.Cancel(ctx, cancelled.ID, "test: cancelled while parked"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	agentExec.ParkCloseoutForTest(cancelled.ID.String(), filepath.Join(t.TempDir(), "repo"), "sess-1")

	if agentExec.WorktreeInUse(cancelled.ID.String()) != true {
		t.Fatalf("precondition: parked entry not visible on executor")
	}

	if probe(cancelled.ID.String()) {
		t.Fatalf(
			"CANCELLED task with parked closeoutPending reported in-flight — exactly the pin class (stale entry would hold the worktree until restart)",
		)
	}

	pending, err := s.Enqueue(ctx, task.New{Type: "agent"})
	if err != nil {
		t.Fatalf("enqueue pending: %v", err)
	}

	agentExec.ParkCloseoutForTest(pending.ID.String(), filepath.Join(t.TempDir(), "repo"), "sess-2")

	if !probe(pending.ID.String()) {
		t.Fatalf(
			"PENDING task with parked closeoutPending reported not in-flight — parked entry must keep the worktree alive while the task can re-claim",
		)
	}
}

func TestFrozenSweepersDetectorLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	defer func() { _ = s.Close() }()

	const (
		frozenConsumer = "review-sweeper"
		neverRan       = "status-sweeper"
	)

	var got []string

	d := &frozenSweepersDetector{
		ticks:  2,
		watch:  []string{frozenConsumer, neverRan},
		last:   map[string]int64{},
		streak: map[string]int{},
	}
	d.notify = func(triggered bool, frozen []string, streak int) bool {
		if triggered {
			got = append(got, fmt.Sprintf("triggered:%s:%d", strings.Join(frozen, ","), streak))
		} else {
			got = append(got, "resolved")
		}

		return true
	}

	// The consumer ran once and checkpointed; the journal advanced past it.
	if err := s.SaveWatermark(ctx, frozenConsumer, 0); err != nil {
		t.Fatalf("save watermark: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{Type: "sh"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	d.observe(ctx, s) // first observation only baselines
	d.observe(ctx, s) // streak 1 — below threshold
	d.observe(ctx, s) // streak 2 — fires once, naming only the frozen consumer

	if len(got) != 1 || got[0] != "triggered:"+frozenConsumer+":2" {
		t.Fatalf("notifications = %v, want [triggered:%s:2]", got, frozenConsumer)
	}

	d.observe(ctx, s) // still frozen, still quiet

	if len(got) != 1 {
		t.Fatalf("repeat frozen tick re-fired: %v", got)
	}

	// The sweep catches up: the cursor advances to the journal head.
	head, err := s.HeadSeq(ctx)
	if err != nil {
		t.Fatalf("head seq: %v", err)
	}

	if err := s.SaveWatermark(ctx, frozenConsumer, head); err != nil {
		t.Fatalf("save watermark: %v", err)
	}

	d.observe(ctx, s) // recovered — resolves

	if len(got) != 2 || got[1] != "resolved" {
		t.Fatalf("notifications = %v, want raise+resolve", got)
	}
}

// TestFrozenSweepersDetectorIdleAtHeadNoFire pins the idle rule: a
// consumer parked AT the journal head is caught up, not frozen — repeated
// ticks with no journal movement must never raise.
func TestFrozenSweepersDetectorIdleAtHeadNoFire(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	defer func() { _ = s.Close() }()

	const consumer = "incident-policy"

	var calls int

	d := &frozenSweepersDetector{
		ticks:  1,
		watch:  []string{consumer},
		last:   map[string]int64{},
		streak: map[string]int{},
	}
	d.notify = func(bool, []string, int) bool {
		calls++

		return true
	}

	if _, err := s.Enqueue(ctx, task.New{Type: "sh"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	head, err := s.HeadSeq(ctx)
	if err != nil {
		t.Fatalf("head seq: %v", err)
	}

	if err := s.SaveWatermark(ctx, consumer, head); err != nil {
		t.Fatalf("save watermark: %v", err)
	}

	for range 3 {
		d.observe(ctx, s) // cursor at head: idle, not frozen
	}

	if calls != 0 {
		t.Fatalf("idle-at-head fired %d times, want 0", calls)
	}
}

// TestFrozenSweepersDetectorRetriesFailedDelivery mirrors the 06-06 §b1
// rule: a failed raise delivery must retry on the next frozen tick, and
// the delivered alert must arm so recovered ticks stop notifying.
func TestFrozenSweepersDetectorRetriesFailedDelivery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	defer func() { _ = s.Close() }()

	const consumer = "dlqfix-sweeper"

	var calls int

	d := &frozenSweepersDetector{
		ticks:  1,
		watch:  []string{consumer},
		last:   map[string]int64{},
		streak: map[string]int{},
	}
	d.notify = func(bool, []string, int) bool {
		calls++

		return calls > 1
	}

	if err := s.SaveWatermark(ctx, consumer, 0); err != nil {
		t.Fatalf("save watermark: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{Type: "sh"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	d.observe(ctx, s) // baseline
	d.observe(ctx, s) // frozen — raise attempt 1, delivery fails
	d.observe(ctx, s) // frozen — retries the raise, delivered

	if calls != 2 {
		t.Fatalf("raise attempts = %d, want 2", calls)
	}

	d.observe(ctx, s) // armed: no third attempt

	if calls != 2 {
		t.Fatalf("raise attempts = %d, want 2 (the delivered alert must arm)", calls)
	}
}

// TestStarvationDetectorLifecycle pins the starvation alarm contract: the
// oldest PENDING task past the threshold fires ONCE per episode, further
// over-threshold ticks stay quiet, a tick back under the threshold (or an
// empty queue) resolves, and a disabled detector never notifies.
func TestStarvationDetectorLifecycle(t *testing.T) {
	t.Parallel()

	now := time.Now()
	old := &task.Task{CreatedAt: now.Add(-48 * time.Hour)}
	fresh := &task.Task{CreatedAt: now.Add(-1 * time.Hour)}

	tests := []struct {
		name       string
		after      time.Duration
		observes   []*task.Task
		wantNotifs []bool
	}{
		{
			name:     "disabled detector never notifies",
			after:    0,
			observes: []*task.Task{old, old, old},
		},
		{
			name:       "past threshold fires once, not per tick",
			after:      24 * time.Hour,
			observes:   []*task.Task{old, old, old},
			wantNotifs: []bool{true},
		},
		{
			name:     "under threshold stays quiet",
			after:    24 * time.Hour,
			observes: []*task.Task{fresh, fresh},
		},
		{
			name:       "recovery resolves and a later re-fire alerts again",
			after:      24 * time.Hour,
			observes:   []*task.Task{old, fresh, old},
			wantNotifs: []bool{true, false, true},
		},
		{
			name:       "empty queue resolves a standing alert",
			after:      24 * time.Hour,
			observes:   []*task.Task{old, nil},
			wantNotifs: []bool{true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got []bool

			detector := &starvationDetector{after: tt.after}
			detector.notify = func(triggered bool, _ time.Duration, _ string, _ int) bool {
				got = append(got, triggered)

				return true
			}

			for _, oldest := range tt.observes {
				detector.observe(now, oldest, 7)
			}

			if len(got) != len(tt.wantNotifs) {
				t.Fatalf("notifications = %v, want %v", got, tt.wantNotifs)
			}

			for i, want := range tt.wantNotifs {
				if got[i] != want {
					t.Fatalf("notification %d = %v, want %v (all: %v)", i, got[i], want, got)
				}
			}
		})
	}
}

func TestParseRepoDurations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		flag    string
		raw     string
		want    map[string]time.Duration
		wantErr string
	}{
		{
			name: "single pair",
			flag: "--repo-timeout",
			raw:  "CV=90m",
			want: map[string]time.Duration{"CV": 90 * time.Minute},
		},
		{
			name: "multiple pairs with spacing",
			flag: "--repo-interval",
			raw:  " CV = 1h , go-taskqueue=30s ",
			want: map[string]time.Duration{"CV": time.Hour, "go-taskqueue": 30 * time.Second},
		},
		{
			name: "empty specs are skipped",
			flag: "--repo-timeout",
			raw:  ",,",
			want: map[string]time.Duration{},
		},
		{
			name:    "missing duration fails with the flag named",
			flag:    "--repo-interval",
			raw:     "CV",
			wantErr: `--repo-interval: want name=duration, got "CV"`,
		},
		{
			name:    "unparseable duration fails with the flag named",
			flag:    "--repo-timeout",
			raw:     "CV=soon",
			wantErr: `--repo-timeout: "CV=soon"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseRepoDurations(tt.flag, tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseRepoDurations() error = %v, want containing %q", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseRepoDurations(): %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("parseRepoDurations() = %v, want %v", got, tt.want)
			}

			for name, want := range tt.want {
				if got[name] != want {
					t.Errorf("parseRepoDurations()[%q] = %v, want %v", name, got[name], want)
				}
			}
		})
	}
}

// TestAgentPoolDonePreflightDefaultOn pins the C13.3 flip: the claim-time
// done gate defaults ON — a pool started with no flags must short-circuit
// paid re-runs of landed work (the row-34 class: minted pre-commit,
// executed 20 minutes after the landing). --done-preflight=false stays the
// documented escape.
func TestAgentPoolDonePreflightDefaultOn(t *testing.T) {
	t.Parallel()

	opts, err := parseAgentPoolOptions(nil)
	if err != nil {
		t.Fatalf("parse bare: %v", err)
	}

	if !opts.donePreflight {
		t.Fatal("done-preflight default flipped back off — the phantom re-dispatch class returns")
	}

	opts, err = parseAgentPoolOptions([]string{"--done-preflight=false"})
	if err != nil {
		t.Fatalf("parse disabled: %v", err)
	}

	if opts.donePreflight {
		t.Fatal("--done-preflight=false must still disable the gate")
	}
}
