package executor

import (
	"os"
	"path/filepath"
	"strings"
)

// WorktreeOrphan is one tq/* delivery remnant whose task is no longer
// in-flight: a linked checkout directory, a delivery branch, or both.
// Branch-only orphans carry Merged=true when the branch is already an
// ancestor of the repo's checked-out branch — only merged branches are
// safe to delete (an unmerged tq/<id> is still the delivery, design
// pipeline step 6 / forensic retention question).
type WorktreeOrphan struct {
	Repo   string // repo directory (projectsDir/<repo> or absolute)
	Dir    string // linked checkout directory; "" when branch-only
	Branch string // tq/<task-id> delivery branch; "" when dir-only
	TaskID string
	Merged bool
}

// ScanWorktreeOrphans sweeps the worktree home
// (<projectsDir>/.tq-worktrees/<repo>/<task-id>) plus the tq/ branch
// namespace of every repo under projectsDir, returning remnants whose
// task id is not in-flight. inFlight is the store-backed liveness probe
// (PENDING/RUNNING) plus the executor's parked-closeout registry.
func ScanWorktreeOrphans(projectsDir string, inFlight func(taskID string) bool) []WorktreeOrphan {
	if projectsDir == "" || inFlight == nil {
		return nil
	}

	var orphans []WorktreeOrphan

	seen := map[string]bool{}

	root := filepath.Join(projectsDir, worktreeDirName)
	if repos, err := os.ReadDir(root); err == nil {
		for _, r := range repos {
			if !r.IsDir() {
				continue
			}

			repoDir := filepath.Join(projectsDir, r.Name())

			dirs, err := os.ReadDir(filepath.Join(root, r.Name()))
			if err != nil {
				continue
			}

			for _, d := range dirs {
				if !d.IsDir() {
					continue
				}

				id := d.Name()

				if inFlight(id) {
					continue
				}

				seen[id] = true

				orphans = append(orphans, WorktreeOrphan{
					Repo:   repoDir,
					Dir:    filepath.Join(root, r.Name(), id),
					Branch: WorktreeBranchPrefix + id,
					TaskID: id,
					Merged: worktreeBranchMerged(repoDir, WorktreeBranchPrefix+id),
				})
			}
		}
	}

	// Branch-only orphans: the run died between branch creation and
	// worktree registration, or the checkout was cleared but the branch
	// (the delivery) survived.
	if repos, err := os.ReadDir(projectsDir); err == nil {
		for _, r := range repos {
			if !r.IsDir() || r.Name() == worktreeDirName {
				continue
			}

			repoDir := filepath.Join(projectsDir, r.Name())

			for id := range listWorktreeBranches(repoDir) {
				if inFlight(id) || seen[id] {
					continue
				}

				seen[id] = true

				orphans = append(orphans, WorktreeOrphan{
					Repo:   repoDir,
					Branch: WorktreeBranchPrefix + id,
					TaskID: id,
					Merged: worktreeBranchMerged(repoDir, WorktreeBranchPrefix+id),
				})
			}
		}
	}

	return orphans
}

// ReapWorktreeOrphans removes every scanned orphan: the linked checkout
// (best-effort git worktree remove, degrade-to-clear like removeWorktree)
// and the branch when it is merged. Unmerged branches survive — they are
// still the delivery. Returns human-readable lines of what was reaped.
func ReapWorktreeOrphans(projectsDir string, inFlight func(taskID string) bool) []string {
	var reaped []string

	for _, o := range ScanWorktreeOrphans(projectsDir, inFlight) {
		if o.Dir != "" {
			_, _ = removeWorktree(o.Repo, o.Dir, gitRun)
		}

		if o.Branch != "" && o.Merged {
			if err := DeleteWorktreeBranch(o.Repo, o.Branch); err != nil {
				continue // keep the orphan visible to the next sweep
			}
		}

		reaped = append(reaped, o.Repo+" "+o.Branch)
	}

	return reaped
}

// listWorktreeBranches returns the tq/<task-id> branches of the repo keyed
// by task id. Empty for non-repos and git failures (the sweep is
// best-effort by design).
func listWorktreeBranches(repoDir string) map[string]bool {
	out, err := gitRun(repoDir, "for-each-ref", "--format=%(refname:short)", "refs/heads/"+WorktreeBranchPrefix+"*")
	if err != nil {
		return nil
	}

	branches := map[string]bool{}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, WorktreeBranchPrefix) {
			continue
		}

		branches[strings.TrimPrefix(line, WorktreeBranchPrefix)] = true
	}

	return branches
}

// worktreeBranchMerged reports whether branch is an ancestor of the repo's
// checked-out branch (true also when the branch is gone — nothing left to
// reap). Unmergeable probes read as unmerged: never delete on a guess.
func worktreeBranchMerged(repoDir, branch string) bool {
	head := currentBranch(repoDir, gitRun)
	if head == "" || head == branch {
		return false
	}

	_, err := gitRun(repoDir, "merge-base", "--is-ancestor", branch, head)

	return err == nil
}
