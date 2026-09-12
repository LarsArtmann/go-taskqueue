//go:build unix

package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGitLogScannerAttributesRealCommits pins the scanner against real git:
// only the commit carrying the exact footer is attributed, oldest first. Git
// is a hard requirement of every environment that runs this suite (same
// stance as the executor's git fixtures); the parser tests above stay
// platform-free.
func TestGitLogScannerAttributesRealCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

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

	write := func(name, body string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("a.txt", "one")
	git("add", "-A")
	git("commit", "-qm", "unattributed human commit")

	write("b.txt", "two")
	git("add", "-A")
	git("commit", "-qm", "session work one", "-m", "Crush-Session: sess-42")

	write("c.txt", "three")
	git("add", "-A")
	git("commit", "-qm", "session work two", "-m", "Crush-Session: sess-42")

	scanner := GitLogScanner{}

	got, err := scanner.CommitsByTrailer(context.Background(), repo, Trailer, "sess-42")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("attributed %+v, want exactly the two session commits", got)
	}

	if got[0].Subject != "session work one" || got[1].Subject != "session work two" {
		t.Fatalf("order wrong: %+v", got)
	}

	foreign, err := scanner.CommitsByTrailer(context.Background(), repo, Trailer, "sess-other")
	if err != nil {
		t.Fatal(err)
	}

	if len(foreign) != 0 {
		t.Fatalf("foreign session attributed %+v", foreign)
	}
}

// TestGitLogScannerEmptyRepoAttributesNothing pins the unborn-HEAD case: a
// begin-on-fresh-repo close is an empty session, not a git failure.
func TestGitLogScannerEmptyRepoAttributesNothing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := t.TempDir()

	cmd := exec.Command("git", "init", "-q", "-b", "main")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	got, err := GitLogScanner{}.CommitsByTrailer(context.Background(), repo, Trailer, "sess-1")
	if err != nil {
		t.Fatalf("empty repo scan: %v", err)
	}

	if got != nil {
		t.Fatalf("empty repo attributed %+v", got)
	}
}

// TestGitLogScannerNotARepoFails pins that a non-repo surfaces as an error
// (wrong --repo must be loud, not an empty session).
func TestGitLogScannerNotARepoFails(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	_, err := GitLogScanner{}.CommitsByTrailer(context.Background(), t.TempDir(), Trailer, "x")
	if err == nil {
		t.Fatal("non-repo scan must fail")
	}
}
