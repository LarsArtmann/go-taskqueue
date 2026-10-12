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

	base := filepath.Base(repoDir)
	dir = filepath.Join(worktreeRoot(repoDir, projectsDir), base, id)

	_, branchExists := run(repoDir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)

	if branchExists == nil {
		// Retry-ladder adoption (the 00-46 §g1 collision bug): attempt 2
		// re-enters createWorktree and the delivery branch of attempt 1
		// still exists. A RETRYABLE failure must never turn permanent —
		// ADOPT the branch: reuse the checkout when git still knows it,
		// otherwise re-checkout the branch at the expected path (the
		// failed attempt's commits survive on the branch).
		registered, _ := run(repoDir, "worktree", "list", "--porcelain")
		if strings.Contains(registered, dir+"\n") {
			return dir, branch, true, nil
		}

		_ = os.RemoveAll(dir) // stale/unregistered path: a worktree add needs it gone

		if out, err := run(repoDir, "worktree", "add", dir, branch); err != nil {
			return "", "", false, fmt.Errorf("agent: worktree adopt %s at %s: %w: %s", branch, dir, err, tailOutput(out))
		}

		return dir, branch, true, nil
	}

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

// currentBranch returns the checked-out branch name of the repo at dir,
// or "" (detached HEAD, unborn branch, non-repo).
func currentBranch(dir string, run gitRunner) string {
	out, err := run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || out == "" || out == "HEAD" {
		return ""
	}

	return out
}

// MergeWorktreeBranch merges a worktree delivery branch into its base
// branch via git plumbing (merge-tree --write-tree → commit-tree →
// update-ref): a real merge commit, but NO checkout — the human's dirty
// main checkout is never touched and base drift (the auto-commit daemon
// moving master mid-task) is tolerated by construction. A conflicting
// branch is refused without touching anything (the branch stays the
// delivery for a human). Worktree-per-agent design pipeline step 5.
func MergeWorktreeBranch(repoDir, branch, base, message string) error {
	return mergeWorktreeBranch(repoDir, branch, base, message, gitRun)
}

func mergeWorktreeBranch(repoDir, branch, base, message string, run gitRunner) error {
	if branch == "" || base == "" || base == branch {
		return fmt.Errorf("agent: worktree merge needs a branch and a distinct base (got %q → %q)", branch, base)
	}

	for _, ref := range []string{branch, base} {
		if _, err := run(repoDir, "rev-parse", "--verify", "--quiet", "refs/heads/"+ref); err != nil {
			return fmt.Errorf("agent: worktree merge: branch %s does not exist in %s", ref, repoDir)
		}
	}

	baseTip, err := run(repoDir, "rev-parse", "refs/heads/"+base)
	if err != nil {
		return fmt.Errorf("agent: worktree merge base tip: %w", err)
	}

	branchTip, err := run(repoDir, "rev-parse", "refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("agent: worktree merge branch tip: %w", err)
	}

	if baseTip == branchTip {
		return nil // already merged (fast-forwarded elsewhere): nothing to do
	}

	tree, err := run(repoDir, "merge-tree", "--write-tree", base, branch)
	if err != nil {
		return fmt.Errorf("agent: worktree merge %s → %s conflicts — the branch stays the delivery for a human: %s",
			branch, base, tailOutput(tree))
	}

	treeOID, _, _ := strings.Cut(tree, "\n")

	commit, err := run(repoDir, "commit-tree", treeOID, "-p", baseTip, "-p", branchTip, "-m", message)
	if err != nil {
		return fmt.Errorf("agent: worktree merge commit: %w: %s", err, tailOutput(commit))
	}

	if out, err := run(repoDir, "update-ref", "refs/heads/"+base, commit); err != nil {
		return fmt.Errorf("agent: worktree merge update-ref %s: %w: %s", base, err, tailOutput(out))
	}

	return nil
}

// DeleteWorktreeBranch removes a merged delivery branch (the post-merge
// half of reap). Best-effort: an already-gone branch is not an error.
func DeleteWorktreeBranch(repoDir, branch string) error {
	if _, err := gitRun(repoDir, "branch", "-D", branch); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}

		return fmt.Errorf("agent: delete branch %s: %w", branch, err)
	}

	return nil
}

// RemoveWorktree removes the linked checkout directory (the delivery
// branch survives unless the caller deletes it separately). Exported for
// the review sweeper's post-merge cleanup and the reaper.
func RemoveWorktree(repoDir, dir string) error {
	_, err := removeWorktree(repoDir, dir, gitRun)

	return err
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
// agent's commits. Best-effort, never a task failure: when git refuses
// (modified files, races), the directory is cleared and stale admin files
// pruned; removed=false tells the caller the git-level removal skipped.
func removeWorktree(repoDir, dir string, run gitRunner) (removed bool, err error) {
	if out, err := run(repoDir, "worktree", "remove", "--force", dir); err != nil {
		_ = os.RemoveAll(dir)
		_, _ = run(repoDir, "worktree", "prune")

		return false, fmt.Errorf("agent: worktree remove %s degraded to clear+prune: %w: %s", dir, err, tailOutput(out))
	}

	return true, nil
}
