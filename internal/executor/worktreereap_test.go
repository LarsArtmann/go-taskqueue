package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanWorktreeOrphansDirs(t *testing.T) {
	t.Parallel()

	projects := t.TempDir()

	// Two task checkouts; only the one whose task is in-flight survives.
	for _, id := range []string{"alive", "dead"} {
		if err := os.MkdirAll(filepath.Join(projects, worktreeDirName, "demo", id), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	orphans := ScanWorktreeOrphans(projects, func(id string) bool { return id == "alive" })

	if len(orphans) != 1 {
		t.Fatalf("want 1 orphan, got %+v", orphans)
	}

	o := orphans[0]

	if o.TaskID != "dead" || o.Branch != WorktreeBranchPrefix+"dead" {
		t.Fatalf("wrong orphan: %+v", o)
	}

	if want := filepath.Join(projects, worktreeDirName, "demo", "dead"); o.Dir != want {
		t.Fatalf("dir = %s, want %s", o.Dir, want)
	}

	if ScanWorktreeOrphans("", nil) != nil {
		t.Fatal("empty projects dir must scan to nil")
	}
}

func TestReapWorktreeOrphansRemovesDirKeepsUnmergedBranch(t *testing.T) {
	t.Parallel()

	projects := t.TempDir()

	id := "dead"
	dir := filepath.Join(projects, worktreeDirName, "demo", id)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "evidence.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	reaped := ReapWorktreeOrphans(projects, func(string) bool { return false })

	if len(reaped) != 1 {
		t.Fatalf("want 1 reap, got %v", reaped)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("orphan dir must be gone, got %v", err)
	}
}
