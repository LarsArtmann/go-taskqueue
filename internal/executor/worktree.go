package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WorktreeBranchPrefix names the per-task delivery branches created in
// worktree mode (branch tq/<task-id> forked from the repo's HEAD). The
// merge/reap machinery keys off this prefix (worktree-per-agent design,
// docs/planning/archived/2026-09-12_worktree-per-agent-design.md).
const WorktreeBranchPrefix = "tq/"

// worktreeRails are the repo-local contract files a fresh worktree does
// NOT inherit when they are uncommitted: a worktree is a clean checkout of
// tracked files only, so an uncommitted .crushrc (provider config) or
// .tq-verify (verify gate) would silently change the run's contract or
// fail the gate for a reason the main checkout never sees. Worktree mode
// turns tq doctor's untracked-rails warning into a hard preflight failure.
var worktreeRails = []string{".crushrc", ".tq-verify"}

// worktreeDirName is the sibling-of-repos directory holding per-task
// linked checkouts: <parent-of-repo>/.tq-worktrees/<repo>/<task-id>.
// Outside the main working tree so it never trips assertCleanTree or
// harvest scans (design: WORKTREE_DIR layout, open question 9).
const worktreeDirName = ".tq-worktrees"

// gitRunner is the exec seam for git calls in this file (test seam).
type gitRunner func(dir string, args ...string) (string, error)

// createWorktree forks a linked checkout for the task: branch tq/<id>
// from the repo's current HEAD, worktree at
// <parent-of-repo>/.tq-worktrees/<repo>/<id>. The MAIN tree may be dirty —
// a worktree branches from HEAD regardless of WIP, which is the property
// that dissolves the dirty-tree parking problem. Returns ok=false (no
// error) when the repo is not a git checkout, so callers fall back to the
// in-place run. Committed rails are REQUIRED: an uncommitted .crushrc or
// .tq-verify would not survive the checkout.
func createWorktree(repoDir, id, projectsDir string, run gitRunner) (dir, branch string, ok bool, err error) {
	if _, err := run(repoDir, "rev-parse", "--git-dir"); err != nil {
		return "", "", false, nil //nolint:nilerr // not a git repo: in-place fallback is the contract
	}

	if err := checkWorktreeRails(repoDir, run); err != nil {
		return "", "", false, err
	}

	branch = WorktreeBranchPrefix + id

	if _, err := run(repoDir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		return "", "", false, Permanent(fmt.Errorf("agent: branch %s already exists — a previous worktree run for this task never merged or reaped; delete it (git branch -D %s) or run the reaper before retrying", branch, branch))
	}

	base := filepath.Base(repoDir)
	dir = filepath.Join(worktreeRoot(repoDir, projectsDir), base, id)

	if out, err := run(repoDir, "worktree", "add", "-b", branch, dir, "HEAD"); err != nil {
		return "", "", false, fmt.Errorf("agent: worktree add %s: %w: %s", dir, err, tailOutput(out))
	}

	// Submodules need the explicit init per worktree (git does not share
	// submodule checkouts across linked worktrees).
	if _, statErr := os.Stat(filepath.Join(dir, ".gitmodules")); statErr == nil {
		if out, err := run(dir, "submodule", "update", "--init"); err != nil {
			_, _ = removeWorktree(repoDir, dir, run)

			return "", "", false, fmt.Errorf("agent: worktree submodule init: %w: %s", err, tailOutput(out))
		}
	}

	return dir, branch, true, nil
}

// worktreeRoot resolves the worktree home: a sibling of the repo's parent
// when the repo lives under the pool's projects dir, else beside the repo
// itself.
func worktreeRoot(repoDir, projectsDir string) string {
	if projectsDir != "" && strings.HasPrefix(repoDir, projectsDir+string(filepath.Separator)) {
		return filepath.Join(projectsDir, worktreeDirName)
	}

	return filepath.Join(filepath.Dir(repoDir), worktreeDirName)
}

// checkWorktreeRails fails the run permanently when a contract rail is
// sitting uncommitted in the main checkout: a fresh worktree cannot see
// it, so the run's provider config or verify gate would silently differ.
func checkWorktreeRails(repoDir string, run gitRunner) error {
	for _, rail := range worktreeRails {
		if _, err := os.Stat(filepath.Join(repoDir, rail)); err != nil {
			continue // absent rails are fine (verify falls back to detect)
		}

		out, err := run(repoDir, "status", "--porcelain", "--", rail)
		if err != nil {
			return fmt.Errorf("agent: worktree rail check %s: %w: %s", rail, err, tailOutput(out))
		}

		if strings.TrimSpace(out) != "" {
			return Permanent(fmt.Errorf("agent: %s is uncommitted in %s — a worktree checkout would run WITHOUT it; commit the rail first (worktree mode makes tq doctor's untracked-rails warning a hard failure)", rail, repoDir))
		}
	}

	return nil
}

// removeWorktree deletes the linked checkout but KEEPS the tq/<id> branch:
// until the merge machinery lands, the branch is the delivery (policy (c)
// in the design — branches for the owner); removing it would destroy the
// agent's commits. Best-effort: a leftover directory is reaper food, not a
// task failure.
func removeWorktree(repoDir, dir string, run gitRunner) (removed bool, err error) {
	out, err := run(repoDir, "worktree", "remove", "--force", dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		_, _ = run(repoDir, "worktree", "prune")

		return false, fmt.Errorf("agent: worktree remove %s: %w: %s", dir, err, tailOutput(out))
	}

	return true, nil
}
