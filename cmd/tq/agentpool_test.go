package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/larsartmann/go-taskqueue/internal/harvest"
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
