//go:build unix

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// gitRepo builds a scratch repo with the given commits (subject, body).
// Git is a hard requirement of every environment that runs this suite
// (same stance as the executor's git fixtures); the shape tests below
// stay platform-free.
func gitRepo(t *testing.T, commits ...[2]string) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := t.TempDir()

	run := func(args ...string) {
		t.Helper()

		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	run("init", "-q", "-b", "main")

	for i, c := range commits {
		name := filepath.Join(repo, "f.txt")

		if err := os.WriteFile(name, []byte{byte('a' + i)}, 0o644); err != nil {
			t.Fatal(err)
		}

		run("add", "-A")

		if c[1] == "" {
			run("commit", "-qm", c[0])
		} else {
			run("commit", "-qm", c[0], "-m", c[1])
		}
	}

	return repo
}

// commitsTask is the minimal task record the footer scan needs: an ID and
// a payload naming the repo.
func commitsTask(id, repo string) task.Task {
	payload, err := json.Marshal(map[string]string{"repo": repo})
	if err != nil {
		panic(err)
	}

	return task.Task{ID: task.ID(id), Type: "agent", Payload: payload}
}

// TestCommitsForTaskFoldsAdjacentDaemonCommits pins the 147bd17 shape:
// footer-less auto-commit-daemon commits directly above AND below the
// footer-bearing work commit surface as "folded here" with their
// relation, while a footer-less human commit stays silent.
func TestCommitsForTaskFoldsAdjacentDaemonCommits(t *testing.T) {
	t.Parallel()

	const id = "000001a0db237aa8"

	repo := gitRepo(t,
		[2]string{"chore: auto-commit 1 changed file(s) (heuristic)", ""},
		[2]string{"agent work", "Task-Queue-ID: " + id},
		[2]string{"chore: auto-commit 2 changed file(s) (heuristic)", ""},
		[2]string{"unrelated human commit", ""},
	)

	view := commitsForTask(commitsTask(id, repo))

	if view["count"] != 1 {
		t.Fatalf("count = %v, want 1", view["count"])
	}

	raw, err := json.Marshal(view["folded_here"])
	if err != nil {
		t.Fatal(err)
	}

	var folded []foldedCommit
	if err := json.Unmarshal(raw, &folded); err != nil {
		t.Fatal(err)
	}

	if len(folded) != 2 {
		t.Fatalf("folded_here = %s, want parent + child daemon commits", raw)
	}

	if folded[0].Relation != "parent" || folded[1].Relation != "child" {
		t.Fatalf("relations = %q, %q; want parent, child", folded[0].Relation, folded[1].Relation)
	}

	for _, f := range folded {
		if f.SHA == "" || f.Subject == "" {
			t.Fatalf("folded commit missing identity: %+v", f.commitHit)
		}
	}
}

// TestCommitsForTaskNoFoldForHumanNeighbors pins the negative: footer-less
// human commits adjacent to the work commit are NOT claimed as folds, and
// the folded_here key stays absent from the view.
func TestCommitsForTaskNoFoldForHumanNeighbors(t *testing.T) {
	t.Parallel()

	const id = "000001a0db237aa8"

	repo := gitRepo(t,
		[2]string{"human docs commit", ""},
		[2]string{"agent work", "Task-Queue-ID: " + id},
		[2]string{"another human commit", ""},
	)

	view := commitsForTask(commitsTask(id, repo))

	if _, ok := view["folded_here"]; ok {
		t.Fatalf("folded_here present for human neighbors: %v", view["folded_here"])
	}
}

// TestDaemonCommitSubjectShape pins the fold-claim shape: only the exact
// auto-commit-daemon subject line qualifies, across count variants; human
// subjects and near-misses never do.
func TestDaemonCommitSubjectShape(t *testing.T) {
	t.Parallel()

	matching := []string{
		"chore: auto-commit 0 changed file(s) (heuristic)",
		"chore: auto-commit 1 changed file(s) (heuristic)",
		"chore: auto-commit 113 changed file(s) (heuristic)",
	}
	for _, s := range matching {
		if !daemonCommitSubject.MatchString(s) {
			t.Errorf("daemon subject %q not matched", s)
		}
	}

	foreign := []string{
		"chore: auto-commit 1 changed files (heuristic)",
		"chore: auto-commit 1 changed file(s)",
		"chore: auto-commit 1 changed file(s) (heuristic) extra",
		"docs: ADR-0019 S4 third window report",
		"agent work",
		"",
	}
	for _, s := range foreign {
		if daemonCommitSubject.MatchString(s) {
			t.Errorf("non-daemon subject %q matched", s)
		}
	}
}
