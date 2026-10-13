package executor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeGit builds a gitRunner over a scripted response map (subcommand →
// output, err) so the worktree preflight logic is testable without git.
func fakeGit(responses map[string]struct {
	out string
	err error
},
) gitRunner {
	return func(dir string, args ...string) (string, error) {
		r, ok := responses[args[0]]
		if !ok {
			return "", errors.New("unexpected git subcommand: " + args[0])
		}

		return r.out, r.err
	}
}

func TestCreateWorktreeSkipsNonGitRepo(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	_, _, ok, err := createWorktree(repo, "0001", "", fakeGit(map[string]struct {
		out string
		err error
	}{"rev-parse": {err: errors.New("not a repo")}}))
	if err != nil {
		t.Fatalf("non-git repo must fall back in-place without error, got %v", err)
	}

	if ok {
		t.Fatal("non-git repo must report ok=false")
	}
}

func TestCreateWorktreeRefusesUncommittedRails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rail     string
		modified string
	}{
		{rail: ".crushrc", modified: " M .crushrc"},
		{rail: ".tq-verify", modified: "?? .tq-verify"},
	}

	for _, tt := range tests {
		t.Run(tt.rail, func(t *testing.T) {
			t.Parallel()

			repo := t.TempDir()

			if err := os.WriteFile(filepath.Join(repo, tt.rail), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}

			git := fakeGit(map[string]struct {
				out string
				err error
			}{
				"rev-parse": {},
				"status":    {out: tt.modified + "\n"},
			})

			_, _, _, err := createWorktree(repo, "0001", "", git)
			if err == nil {
				t.Fatal("uncommitted rail must fail the run")
			}

			if !isPermanent(t, err) {
				t.Fatalf("rail miss is a permanent input-contract failure, got %v", err)
			}

			if !strings.Contains(err.Error(), tt.rail) {
				t.Fatalf("error must name the rail %s, got %v", tt.rail, err)
			}
		})
	}
}

func TestCreateWorktreeAdoptsExistingBranch(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	projects := filepath.Dir(repo)

	id := "0001"
	dir := filepath.Join(projects, worktreeDirName, filepath.Base(repo), id)

	runner := func(rdir string, args ...string) (string, error) {
		switch {
		case len(args) >= 3 && args[0] == "rev-parse" && args[1] == "--verify":
			return "", nil // branch exists
		case args[0] == "rev-parse", args[0] == "status":
			return "", nil
		case args[0] == "worktree" && args[1] == "list":
			return "other-worktree\n", nil // registered worktrees do NOT include dir
		case args[0] == "worktree" && args[1] == "add":
			return "", nil // re-checkout of the existing branch
		default:
			return "", errors.New("unexpected: " + strings.Join(args, " "))
		}
	}

	gotDir, gotBranch, ok, err := createWorktree(repo, id, projects, runner)
	if err != nil || !ok {
		t.Fatalf("existing branch must be adopted, got ok=%v err=%v", ok, err)
	}

	if gotDir != dir || gotBranch != WorktreeBranchPrefix+id {
		t.Fatalf("adopted dir/branch = %s/%s, want %s/%s", gotDir, gotBranch, dir, WorktreeBranchPrefix+id)
	}
}

func TestCreateWorktreeReusesRegisteredWorktree(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	projects := filepath.Dir(repo)

	id := "0001"
	dir := filepath.Join(projects, worktreeDirName, filepath.Base(repo), id)

	runner := func(rdir string, args ...string) (string, error) {
		switch {
		case len(args) >= 3 && args[0] == "rev-parse" && args[1] == "--verify":
			return "", nil // branch exists
		case args[0] == "rev-parse", args[0] == "status":
			return "", nil
		case args[0] == "worktree" && args[1] == "list":
			return "worktree " + dir + "\n", nil // already registered: plain reuse
		default:
			return "", errors.New("unexpected: " + strings.Join(args, " "))
		}
	}

	gotDir, _, ok, err := createWorktree(repo, id, projects, runner)
	if err != nil || !ok {
		t.Fatalf("registered worktree must be reused, got ok=%v err=%v", ok, err)
	}

	if gotDir != dir {
		t.Fatalf("dir = %s, want %s", gotDir, dir)
	}
}

func TestCreateWorktreeHappyPath(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	projects := filepath.Dir(repo)

	var calls [][]string

	runner := func(dir string, args ...string) (string, error) {
		calls = append(calls, args)

		switch {
		case args[0] == "rev-parse" && len(args) >= 2 && args[1] == "--verify":
			return "", errors.New("branch absent") // the happy path has no tq/ branch yet
		case args[0] == "rev-parse", args[0] == "status":
			return "", nil
		case args[0] == "worktree":
			return "", nil
		default:
			return "", errors.New("unexpected: " + strings.Join(args, " "))
		}
	}

	dir, branch, ok, err := createWorktree(repo, "abc123", projects, runner)
	if err != nil || !ok {
		t.Fatalf("happy path: ok=%v err=%v", ok, err)
	}

	if branch != WorktreeBranchPrefix+"abc123" {
		t.Fatalf("branch = %q, want tq/abc123", branch)
	}

	wantDir := filepath.Join(projects, worktreeDirName, filepath.Base(repo), "abc123")
	if dir != wantDir {
		t.Fatalf("dir = %q, want %q", dir, wantDir)
	}

	var added bool

	for _, c := range calls {
		if c[0] == "worktree" && c[1] == "add" {
			added = true
		}
	}

	if !added {
		t.Fatal("worktree add never ran")
	}
}

func TestWorktreeRootLayout(t *testing.T) {
	t.Parallel()

	projects := string(filepath.Separator) + filepath.Join("home", "lars", "projects")
	repo := filepath.Join(projects, "demo")

	if got := worktreeRoot(repo, projects); got != filepath.Join(projects, worktreeDirName) {
		t.Fatalf("under projects dir: got %q", got)
	}

	// A repo OUTSIDE the projects dir parks its worktrees beside itself,
	// never inside a stranger's tree.
	outside := filepath.Join(string(filepath.Separator), "tmp", "scratch-repo")
	if got := worktreeRoot(outside, projects); got != filepath.Join(filepath.Dir(outside), worktreeDirName) {
		t.Fatalf("outside projects dir: got %q", got)
	}
}

func TestRemoveWorktreeKeepsBranchAndPrunesOnFailure(t *testing.T) {
	t.Parallel()

	var calls [][]string

	runner := func(dir string, args ...string) (string, error) {
		calls = append(calls, args)

		if args[0] == "worktree" && args[1] == "remove" {
			return "", errors.New("git worktree remove: contains modified files")
		}

		return "", nil
	}

	dir := t.TempDir()

	removed, err := removeWorktree(dir, dir, runner)
	if err == nil {
		t.Fatal("degraded removal must surface what happened")
	}

	if removed {
		t.Fatal("prune path reports removed=false")
	}

	var pruned bool

	for _, c := range calls {
		if len(c) >= 2 && c[0] == "worktree" && c[1] == "prune" {
			pruned = true
		}
	}

	if !pruned {
		t.Fatal("failed remove must prune stale admin files")
	}
}

// The in-repo integration pin: real git, real worktree, branch survives
// removal (the delivery), and the worktree never lands inside the main
// checkout. Cross-platform like the gitscan fixtures: skips when git is
// unusable, hermetic identity via setupGitRepo's local config.
func TestCreateWorktreeIntegrationRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	projects := t.TempDir()
	repo := filepath.Join(projects, "demo")

	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	setupGitRepo(t, repo)

	dir, branch, ok, err := createWorktree(repo, "abc123", projects, gitRun)
	if err != nil || !ok {
		t.Fatalf("create: ok=%v err=%v", ok, err)
	}

	if !strings.HasPrefix(filepath.Base(branch), "") || branch != WorktreeBranchPrefix+"abc123" {
		t.Fatalf("branch = %q", branch)
	}

	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		t.Fatalf("worktree is not a checkout of the repo: %v", err)
	}

	if removed, err := removeWorktree(repo, dir, gitRun); err != nil || !removed {
		t.Fatalf("remove: removed=%v err=%v", removed, err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("worktree dir must be gone after removal")
	}

	// The branch is the delivery: removal keeps it.
	if out, err := gitRun(repo, "branch", "--list", branch); err != nil || !strings.Contains(out, branch) {
		t.Fatalf("branch %s must survive worktree removal (out=%q err=%v)", branch, out, err)
	}
}

// scriptedGit answers calls in order (per-subcommand output, err); an
// exhausted script fails the call loudly.
func scriptedGit(steps []struct {
	out string
	err error
},
) gitRunner {
	i := 0

	return func(dir string, args ...string) (string, error) {
		if i >= len(steps) {
			return "", errors.New("scriptedGit: unexpected extra call: " + strings.Join(args, " "))
		}

		s := steps[i]
		i++

		return s.out, s.err
	}
}

func TestMergeWorktreeBranchPlumbingMerge(t *testing.T) {
	t.Parallel()

	var dirs []string

	run := func(dir string, args ...string) (string, error) {
		dirs = append(dirs, dir)

		return scriptedGit([]struct {
			out string
			err error
		}{
			{out: "", err: nil}, // rev-parse --verify branch
			{out: "", err: nil}, // rev-parse --verify base
			{out: "basesha", err: nil},
			{out: "branchsha", err: nil},
			{out: "treeoid\nconflict-info", err: nil}, // merge-tree --write-tree
			{out: "mergesha", err: nil},               // commit-tree
			{out: "", err: nil},                       // update-ref
		})(dir, args...)
	}

	if err := mergeWorktreeBranch("/repo", "tq/0001", "master", "merge it", run); err != nil {
		t.Fatalf("plumbing merge must succeed: %v", err)
	}

	for _, d := range dirs {
		if d != "/repo" {
			t.Fatalf("every git call must run in the repo dir, got %s", d)
		}
	}
}

func TestMergeWorktreeBranchRefusals(t *testing.T) {
	t.Parallel()

	if err := mergeWorktreeBranch("/repo", "master", "master", "m", scriptedGit(nil)); err == nil {
		t.Fatal("branch == base must refuse")
	}

	// Conflict: merge-tree exits non-zero — nothing after it may run.
	var calls int

	run := func(dir string, args ...string) (string, error) {
		calls++

		if args[0] == "merge-tree" {
			return "CONFLICT", errors.New("exit 1")
		}

		if args[0] == "rev-parse" && len(args) == 2 {
			return "sha" + strconv.Itoa(calls), nil // distinct tips
		}

		return "sha", nil
	}

	err := mergeWorktreeBranch("/repo", "tq/0001", "master", "m", run)
	if err == nil {
		t.Fatal("merge conflict must refuse")
	}

	if calls != 5 {
		t.Fatalf("conflict must stop before commit-tree/update-ref, got %d calls", calls)
	}

	if !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflict error must say conflicts, got %v", err)
	}
}

func TestMergeWorktreeBranchAlreadyMerged(t *testing.T) {
	t.Parallel()

	run := func(dir string, args ...string) (string, error) {
		if args[0] == "rev-parse" && len(args) == 2 {
			return "samesha", nil // base tip == branch tip
		}

		return "", nil
	}

	if err := mergeWorktreeBranch("/repo", "tq/0001", "master", "m", run); err != nil {
		t.Fatalf("already-merged must be a no-op success, got %v", err)
	}
}
