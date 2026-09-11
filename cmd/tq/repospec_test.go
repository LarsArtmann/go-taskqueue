package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/harvest"
)

// TestCmdAuditResolvesBareRepoNamesAgainstProjectsDir pins the
// cwd-independence contract: `tq audit --repos alpha` must resolve the bare
// name against --projects-dir no matter the working directory (02:00 f8) —
// the harvest package Abs()es repo entries, so an un-expanded name silently
// audits <cwd>/alpha and reports the real repo as a scan failure.
func TestCmdAuditResolvesBareRepoNamesAgainstProjectsDir(t *testing.T) {
	projects := t.TempDir()

	repo := filepath.Join(projects, "drifty")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	todo := "## Backlog\n\n- [ ] open item\n"
	if err := os.WriteFile(filepath.Join(repo, harvest.DefaultTodoFile), []byte(todo), 0o644); err != nil {
		t.Fatalf("write todo: %v", err)
	}

	db := filepath.Join(t.TempDir(), "audit.db")

	// A working directory that does NOT contain the repo: pre-fix, the
	// audit scanned <cwd>/drifty, found nothing, and reported a failure.
	scratch := t.TempDir()
	t.Chdir(scratch)

	out := captureStdout(t, func() {
		err := cmdAudit([]string{
			"--projects-dir", projects,
			"--repos", "drifty",
			"--db", db,
			"--dry-run",
		})
		if err != nil {
			t.Errorf("cmdAudit: %v", err)
		}
	})

	want := "audit: 1 repos, 0 stale-open (0 catch-ups enqueued), 0 stale-done, 0 scan failures"
	if !strings.Contains(out, want) {
		t.Errorf("audit output missing summary %q, got:\n%s", want, out)
	}

	if strings.Contains(out, "ERROR") {
		t.Errorf("audit reported scan failures despite the projects-dir resolution:\n%s", out)
	}
}

// TestCmdDoctorResolvesBareRepoNamesAgainstProjectsDir pins the doctor half
// of the same contract: `tq doctor --repos alpha` must stat the bare name
// against --projects-dir no matter the working directory (02:00 f8). Pre-fix
// it stat'ed <cwd>/alpha and warned "no TODO_LIST.md" for a healthy repo.
func TestCmdDoctorResolvesBareRepoNamesAgainstProjectsDir(t *testing.T) {
	projects := t.TempDir()

	repo := filepath.Join(projects, "drifty")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(repo, harvest.DefaultTodoFile),
		[]byte("## Work\n\n- [ ] item\n"),
		0o644,
	); err != nil {
		t.Fatalf("write todo: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repo, ".crushrc"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write crushrc: %v", err)
	}

	db := filepath.Join(t.TempDir(), "doctor.db")

	// A working directory that does NOT contain the repo: pre-fix, doctor
	// stat'ed <cwd>/drifty and flagged the healthy repo as unharvestable.
	scratch := t.TempDir()
	t.Chdir(scratch)

	// The agent-binary check points at the test binary itself: the nix
	// sandbox has no crush on PATH (see TestDoctorHealthyEmptyDB).
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("abs test binary: %v", err)
	}

	out := captureStdout(t, func() {
		err := cmdDoctor([]string{
			"--projects-dir", projects,
			"--repos", "drifty",
			"--db", db,
			"--agent-bin", self,
		})
		if err != nil {
			t.Errorf("cmdDoctor: %v", err)
		}
	})

	if !strings.Contains(out, "TODO_LIST.md present") {
		t.Errorf("doctor output missing ok repo:drifty detail, got:\n%s", out)
	}

	if strings.Contains(out, "no TODO_LIST.md") {
		t.Errorf("doctor warned about a repo that resolves via --projects-dir:\n%s", out)
	}

	if !strings.Contains(out, ".crushrc present") {
		t.Errorf("doctor output missing ok autonomy:drifty detail, got:\n%s", out)
	}
}

// TestResolveHarvestReposExpandsBareNames pins the harvest command's half of
// the same contract (--prune-stale shares this path).
func TestResolveHarvestReposExpandsBareNames(t *testing.T) {
	volume := ""
	if runtime.GOOS == "windows" {
		volume = `C:`
	}

	projects := filepath.Join(volume, string(filepath.Separator), "home", "lars", "projects")
	absRepo := filepath.Join(volume, string(filepath.Separator), "srv", "overview")

	scratch := t.TempDir()
	t.Chdir(scratch)

	cfg := harvest.Config{TodoFile: harvest.DefaultTodoFile}

	err := resolveHarvestRepos(&cfg, projects, " drifty , "+absRepo, "")
	if err != nil {
		t.Fatalf("resolveHarvestRepos: %v", err)
	}

	want := []string{filepath.Join(projects, "drifty"), absRepo}
	if len(cfg.Repos) != len(want) {
		t.Fatalf("Repos = %v, want %v", cfg.Repos, want)
	}

	for i := range want {
		if got := filepath.Clean(cfg.Repos[i]); got != want[i] {
			t.Errorf("Repos[%d] = %q, want %q", i, got, want[i])
		}
	}
}

// TestExpandRepoSpecs pins the resolution policy shared by the harvest,
// audit, and doctor commands: absolute paths and existing cwd-relative paths
// pass through, bare names join the projects dir, and without a projects
// dir the spec is left for the sweep to report.
func TestExpandRepoSpecs(t *testing.T) {
	volume := ""
	if runtime.GOOS == "windows" {
		volume = `C:`
	}

	root := volume + string(filepath.Separator)
	projects := filepath.Join(root, "home", "lars", "projects")
	absRepo := filepath.Join(root, "srv", "overview")

	existing := t.TempDir()

	cwdRepo := filepath.Join(existing, "localrepo")
	if err := os.MkdirAll(cwdRepo, 0o755); err != nil {
		t.Fatalf("mkdir cwd repo: %v", err)
	}

	scratch := existing
	t.Chdir(scratch)

	relative := "./localrepo"

	tests := []struct {
		name   string
		specs  []string
		want   []string
		proDir string
	}{
		{
			name:   "bare names join the projects dir",
			specs:  []string{"drifty", "gone"},
			want:   []string{filepath.Join(projects, "drifty"), filepath.Join(projects, "gone")},
			proDir: projects,
		},
		{
			name:   "absolute paths stay untouched",
			specs:  []string{absRepo},
			want:   []string{absRepo},
			proDir: projects,
		},
		{
			name:   "existing cwd-relative paths win over the projects dir",
			specs:  []string{relative},
			want:   []string{relative},
			proDir: projects,
		},
		{
			name:   "bare name without a projects dir stays for the sweep to report",
			specs:  []string{"drifty"},
			want:   []string{"drifty"},
			proDir: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandRepoSpecs(tt.proDir, tt.specs)
			if len(got) != len(tt.want) {
				t.Fatalf("expandRepoSpecs(%q, %v) = %v, want %v", tt.proDir, tt.specs, got, tt.want)
			}

			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("expandRepoSpecs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
