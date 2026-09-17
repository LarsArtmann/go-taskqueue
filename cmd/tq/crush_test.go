//go:build unix

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/session"
)

// crushStub writes a crush stand-in: it prints the given stdout (a template
// with {{ID}} substituted) and exits with the given code.
func crushStub(t *testing.T, stdoutTmpl string, exitCode int) string {
	t.Helper()

	script := filepath.Join(t.TempDir(), "crush-stub.sh")
	body := "#!/bin/sh\ncat <<'EOF'\n" + strings.ReplaceAll(stdoutTmpl, "{{ID}}", "sess-wrap") +
		"\nEOF\nexit " + strconv.Itoa(exitCode) + "\n"

	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	return script
}

// TestCrushWrapperClosesOnExit pins trigger #2: the wrapper tees the child's
// output, extracts the session id, and on exit runs the same replay-safe
// close — the minted review + status land in the store the wrapper closed.
func TestCrushWrapperClosesOnExit(t *testing.T) {
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

	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	git("add", "-A")
	git("commit", "-qm", "wrapped work", "-m", "Crush-Session: sess-wrap")

	stub := crushStub(t, "TQ_RESULT: ok session_id=\"sess-wrap\"", 0)

	var out, errOut bytes.Buffer

	code, err := crushRun([]string{
		"--bin", stub, "--repo", repo, "--project", "demo", "--db", dbPath,
		"run", "-m", "do the thing",
	}, nil, &out, &errOut)
	if err != nil {
		t.Fatalf("crushRun: %v (stderr: %s)", err, errOut.String())
	}

	if code != 0 {
		t.Fatalf("child exit code = %d, want 0", code)
	}

	if !strings.Contains(out.String(), "TQ_RESULT") {
		t.Fatalf("child stdout not forwarded: %q", out.String())
	}

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = store.Close() })

	ctx := t.Context()
	assertMinted(t, ctx, store, dbPath, repo, "sess-wrap", "demo")
}

// TestCrushWrapperClosesOnCrash pins the crash path: a failing child still
// gets its session closed and the child's exit code is preserved.
func TestCrushWrapperClosesOnCrash(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	dbPath := filepath.Join(t.TempDir(), "tasks.db")
	repo := t.TempDir()

	cmd := exec.Command("git", "init", "-q", "-b", "main")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	stub := crushStub(t, "INFO Created session for non-interactive run session_id=sess-wrap", 3)

	var out, errOut bytes.Buffer

	code, err := crushRun([]string{
		"--bin", stub, "--repo", repo, "--project", "demo", "--db", dbPath,
	}, nil, &out, &errOut)
	if err != nil {
		t.Fatalf("crushRun: %v", err)
	}

	if code != 3 {
		t.Fatalf("child exit code = %d, want 3", code)
	}

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = store.Close() })

	ctx := t.Context()
	assertMinted(t, ctx, store, dbPath, repo, "sess-wrap", "demo")
}

// TestCrushWrapperNoSessionID pins the honest no-id path: without an id from
// flag/env/output, nothing is closed and the child's status still surfaces.
func TestCrushWrapperNoSessionID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tasks.db")
	repo := t.TempDir()

	stub := crushStub(t, "plain output, no session marker", 0)

	var out, errOut bytes.Buffer

	code, err := crushRun([]string{
		"--bin", stub, "--repo", repo, "--project", "demo", "--db", dbPath,
	}, nil, &out, &errOut)
	if err != nil {
		t.Fatalf("crushRun: %v", err)
	}

	if code != 0 {
		t.Fatalf("child exit code = %d, want 0", code)
	}

	if !strings.Contains(errOut.String(), "no session id") {
		t.Fatalf("missing no-session-id note, stderr: %q", errOut.String())
	}

	if _, err := os.Stat(dbPath); err == nil {
		t.Fatal("db opened despite no session to close")
	}
}

func assertMinted(
	t *testing.T,
	ctx context.Context,
	store *sqlite.Store,
	dbPath, repo, id, project string,
) {
	t.Helper()

	reviews, err := store.List(ctx, queue.Filter{
		Project: ptr(project), Type: ptr(executor.TaskTypeReview),
	})
	if err != nil {
		t.Fatal(err)
	}

	statuses, err := store.List(ctx, queue.Filter{
		Project: ptr(project), Type: ptr(executor.TaskTypeStatus),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(reviews) != 1 || len(statuses) != 1 {
		t.Fatalf("close minted %d reviews, %d statuses; want 1/1", len(reviews), len(statuses))
	}

	facts, err := store.FactsForTask(ctx, session.SyntheticTaskID(id).String(), 0)
	if err != nil {
		t.Fatal(err)
	}

	closed := 0
	for _, f := range facts {
		if f.Type == journal.SessionClosed {
			closed++
		}
	}

	if closed != 1 {
		t.Fatalf("session.closed facts = %d, want 1", closed)
	}
}
