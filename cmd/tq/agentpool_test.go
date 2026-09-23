package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/larsartmann/go-taskqueue/internal/harvest"
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

// TestParseAgentPoolOptionsBatchItems pins the batching flag: values within
// 0..10 plumb into the harvest config, values above fail fast (one session
// working more than ~10 items drowns in its own history).
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

	if _, err := parseAgentPoolOptions([]string{"--projects-dir", t.TempDir(), "--batch-items", "11"}); err == nil {
		t.Fatal("--batch-items 11 must fail fast (context-explosion guard)")
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
