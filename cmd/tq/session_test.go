//go:build unix

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/session"
)

// TestCmdSessionBridgeEndToEnd pins the CLI flow against real git and a real
// sqlite store: begin records the opening, close attributes the footer
// commits and mints exactly one review + one status task over the range.
// Explicit --db flags throughout: a test must never fall back to $TQ_DB.
func TestCmdSessionBridgeEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	dbPath := filepath.Join(t.TempDir(), "tasks.db")
	repo := t.TempDir()

	git := func(args ...string) {
		t.Helper()

		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	git("init", "-q", "-b", "main")

	write := func(name string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(repo, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("a.txt")
	git("add", "-A")
	git("commit", "-qm", "work one", "-m", "Crush-Session: sess-e2e")

	if err := sessionBegin([]string{"--id", "sess-e2e", "--repo", repo, "--project", "demo", "--db", dbPath}); err != nil {
		t.Fatalf("begin: %v", err)
	}

	if err := sessionBegin([]string{"--id", "sess-e2e", "--repo", repo, "--project", "demo", "--db", dbPath}); err == nil {
		t.Fatal("double begin accepted")
	}

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()

	opened, err := store.FactsForTask(ctx, session.SyntheticTaskID("sess-e2e").String(), 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(opened) != 1 || opened[0].Type != journal.SessionOpened {
		t.Fatalf("opened facts = %+v", opened)
	}

	write("b.txt")
	git("add", "-A")
	git("commit", "-qm", "work two", "-m", "Crush-Session: sess-e2e")

	write("c.txt")
	git("add", "-A")
	git("commit", "-qm", "unattributed after-hours commit")

	if err := sessionClose([]string{"--id", "sess-e2e", "--repo", repo, "--project", "demo",
		"--summary", "Shipped the bridge", "--db", dbPath}); err != nil {
		t.Fatalf("close: %v", err)
	}

	reviews, err := store.List(ctx, queue.Filter{Project: ptr("demo"), Type: ptr(executor.TaskTypeReview)})
	if err != nil {
		t.Fatal(err)
	}

	statuses, err := store.List(ctx, queue.Filter{Project: ptr("demo"), Type: ptr(executor.TaskTypeStatus)})
	if err != nil {
		t.Fatal(err)
	}

	if len(reviews) != 1 || len(statuses) != 1 {
		t.Fatalf("close minted %d reviews, %d statuses; want 1/1", len(reviews), len(statuses))
	}

	// A replay close holds the dedup and does not mint again.
	if err := sessionClose([]string{"--id", "sess-e2e", "--repo", repo, "--project", "demo", "--db", dbPath}); err != nil {
		t.Fatalf("re-close: %v", err)
	}

	reviews2, err := store.List(ctx, queue.Filter{Project: ptr("demo"), Type: ptr(executor.TaskTypeReview)})
	if err != nil {
		t.Fatal(err)
	}

	if len(reviews2) != 1 {
		t.Fatalf("re-close minted again: %d reviews", len(reviews2))
	}

	closed, err := store.FactsForTask(ctx, session.SyntheticTaskID("sess-e2e").String(), 0)
	if err != nil {
		t.Fatal(err)
	}

	var closedCount int
	for _, f := range closed {
		if f.Type == journal.SessionClosed {
			closedCount++
		}
	}

	if closedCount != 2 {
		t.Fatalf("closed facts = %d, want 2 (one per close)", closedCount)
	}
}

func TestCmdSessionRejectsMissingID(t *testing.T) {
	t.Setenv("CRUSH_SESSION_ID", "")

	if err := sessionBegin([]string{"--db", filepath.Join(t.TempDir(), "x.db")}); err == nil {
		t.Fatal("begin without id accepted")
	}

	if err := sessionClose([]string{"--db", filepath.Join(t.TempDir(), "x.db")}); err == nil {
		t.Fatal("close without id accepted")
	}
}

//go:fix inline
func ptr[v any](val v) *v {
	return new(val)
}
