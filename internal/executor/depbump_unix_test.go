//go:build unix

package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// depBumpFixtureRepo creates a committed git repo with a Go module that
// requires stretchr/testify at oldVersion, tagged v0.1.0 with that pin —
// the canonical bump scenario (a real module fetchable from the proxy).
func depBumpFixtureRepo(t *testing.T, oldVersion string) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := t.TempDir()

	write := func(name, content string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git := func(args ...string) {
		t.Helper()

		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	write("go.mod", "module example.com/fixture\n\ngo 1.26\n")
	write("main.go", "package main\n\nfunc main() {}\n")

	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "init")

	runGo(t, repo, "mod", "edit", "-require=github.com/stretchr/testify@"+oldVersion)
	write("main_test.go", `package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestX(t *testing.T) { assert.True(t, true) }
`)
	runGo(t, repo, "mod", "tidy")
	runGo(t, repo, "mod", "edit", "-go=1.26") // tidy may raise the floor; normalize back

	git("add", "-A")
	git("commit", "-qm", "use testify")
	git("tag", "-a", "v0.1.0", "-m", "v0.1.0")

	return repo
}

func runGo(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOEXPERIMENT=jsonv2", "GOFLAGS=")

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %v: %v: %s", args, err, out)
	}
}

// TestDepBumpExecutorBumpCommitAndTag runs the full deterministic flow
// against a real fixture repo: baseline, bump, verify, commit, tag. This is
// the executor's mechanical-gate proof — everything the sweep relies on.
func TestDepBumpExecutorBumpCommitAndTag(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := depBumpFixtureRepo(t, "v1.10.0")
	e := &DepBumpExecutor{ExtraEnv: []string{GoEnvExperiment}}

	err := e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{
		Repo: repo,
		Bumps: []DepBump{
			{Module: "github.com/stretchr/testify", Version: "v1.11.1"},
		},
		Release: &DepBumpRelease{Version: "v0.1.1"},
	}))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	pinned := runGoOut(t, repo, "mod", "edit", "-json")
	if !strings.Contains(pinned, "v1.11.1") {
		t.Fatalf("pin must be v1.11.1, go.mod says: %s", pinned)
	}

	log := runGitOut(t, repo, "log", "-1", "--format=%s")
	if !strings.HasPrefix(log, "chore(deps): bump github.com/stretchr/testify to v1.11.1") {
		t.Fatalf("commit message wrong: %q", log)
	}

	tags := runGitOut(t, repo, "tag")
	if !strings.Contains(tags, "v0.1.1") {
		t.Fatalf("release tag missing, tags: %q", tags)
	}

	status := runGitOut(t, repo, "status", "--porcelain")
	if strings.TrimSpace(status) != "" {
		t.Fatalf("tree must be clean after success, status: %q", status)
	}
}

// TestDepBumpExecutorDirtyTreePreflight pins the concurrent-work guard: a
// dirty tree is a PreflightError (requeue without burning an attempt), and
// the WIP survives untouched.
func TestDepBumpExecutorDirtyTreePreflight(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := depBumpFixtureRepo(t, "v1.10.0")

	wip := filepath.Join(repo, "wip.txt")
	if err := os.WriteFile(wip, []byte("someone's work"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := &DepBumpExecutor{}
	err := e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{
		Repo:  repo,
		Bumps: []DepBump{{Module: "github.com/stretchr/testify", Version: "v1.11.1"}},
	}))

	preflight, ok := err.(*PreflightError) //nolint:errorlint // direct class assertion for the test
	if !ok {
		t.Fatalf("dirty tree must be a PreflightError, got %T: %v", err, err)
	}

	if !strings.Contains(preflight.Error(), "uncommitted changes") {
		t.Errorf("preflight must explain the dirty tree: %v", preflight)
	}

	body, err := os.ReadFile(wip)
	if err != nil || string(body) != "someone's work" {
		t.Fatal("WIP must survive untouched")
	}
}

// TestDepBumpExecutorRollsBackFailedBump proves a failed bump leaves the
// repo exactly as it started (touched-path rollback): a target that cannot
// resolve fails the task AND restores go.mod/go.sum.
func TestDepBumpExecutorRollsBackFailedBump(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := depBumpFixtureRepo(t, "v1.10.0")

	goModBefore := readFile(t, filepath.Join(repo, "go.mod"))
	goSumBefore := readFile(t, filepath.Join(repo, "go.sum"))

	e := &DepBumpExecutor{ExtraEnv: []string{GoEnvExperiment}}
	err := e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{
		Repo: repo,
		Bumps: []DepBump{
			// This module path does not exist: go get fails and the executor
			// must roll the touched paths back.
			{Module: "example.invalid/does/not/exist", Version: "v1.0.0"},
		},
	}))
	if err == nil {
		t.Fatal("unresolvable bump must fail")
	}

	if !strings.Contains(err.Error(), "rolled back") {
		t.Errorf("error must state the rollback: %v", err)
	}

	if after := readFile(t, filepath.Join(repo, "go.mod")); after != goModBefore {
		t.Error("go.mod must be restored after a failed bump")
	}

	if after := readFile(t, filepath.Join(repo, "go.sum")); after != goSumBefore {
		t.Error("go.sum must be restored after a failed bump")
	}

	status := runGitOut(t, repo, "status", "--porcelain")
	if strings.TrimSpace(status) != "" {
		t.Fatalf("tree must be clean after rollback, status: %q", status)
	}
}

// TestDepBumpExecutorBaselineRefusesBrokenRepo pins that a red baseline
// (pre-existing breakage) fails the task without touching anything.
func TestDepBumpExecutorBaselineRefusesBrokenRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := depBumpFixtureRepo(t, "v1.10.0")

	if err := os.WriteFile(
		filepath.Join(repo, "broken.go"),
		[]byte("package main\n\nfunc broken( {\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-qm", "break the build")

	goModBefore := readFile(t, filepath.Join(repo, "go.mod"))

	e := &DepBumpExecutor{ExtraEnv: []string{GoEnvExperiment}}
	err := e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{
		Repo:  repo,
		Bumps: []DepBump{{Module: "github.com/stretchr/testify", Version: "v1.11.1"}},
	}))
	if err == nil || !strings.Contains(err.Error(), "baseline") {
		t.Fatalf("broken baseline must fail with baseline context, got %v", err)
	}

	if after := readFile(t, filepath.Join(repo, "go.mod")); after != goModBefore {
		t.Error("repo must stay untouched when the baseline is red")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(body)
}

func runGoOut(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOEXPERIMENT=jsonv2", "GOFLAGS=")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v: %v: %s", args, err, out)
	}

	return string(out)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func runGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}

	return string(out)
}

// commitFile writes one file into the fixture repo and commits it.
func commitFile(t *testing.T, repo, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-qm", "add "+name)
}

// TestDepBumpExecutorCommitScopeTemplRepo pins the templ-aware staging
// scope on a repo WITH templ sources and tracked artifacts: regeneration
// output joins the commit (the *_templ.go/*_templ.txt pathspecs are added
// exactly when such files exist — git fatals on pathspecs matching
// nothing; the no-templ mirror case is TestDepBumpExecutorBumpCommitAndTag).
// The generator is stubbed via TemplBin so the test never needs the real
// templ binary or its runtime dependency.
func TestDepBumpExecutorCommitScopeTemplRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := depBumpFixtureRepo(t, "v1.10.0")
	commitFile(t, repo, "hello.templ", "package main\n\ntempl hello() { \"hi\" }\n")
	commitFile(t, repo, "hello_templ.go", "package main\n\n// generated from hello.templ\n")

	stub := filepath.Join(t.TempDir(), "templ-stub")
	stubBody := "#!/bin/sh\nprintf '\\n// regenerated by stub\\n' >> hello_templ.go\n"

	if err := os.WriteFile(stub, []byte(stubBody), 0o755); err != nil {
		t.Fatal(err)
	}

	e := &DepBumpExecutor{ExtraEnv: []string{GoEnvExperiment}, TemplBin: stub}

	err := e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{
		Repo:  repo,
		Bumps: []DepBump{{Module: "github.com/stretchr/testify", Version: "v1.11.1"}},
	}))
	if err != nil {
		t.Fatalf("execute on templ repo: %v", err)
	}

	if status := runGitOut(t, repo, "status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Fatalf("tree must be clean after the templ-repo bump, status: %q", status)
	}

	if log := runGitOut(t, repo, "log", "-1", "--format=%s"); !strings.HasPrefix(log, "chore(deps): bump ") {
		t.Fatalf("commit message wrong: %q", log)
	}

	head := runGitOut(t, repo, "show", "HEAD:hello_templ.go")
	if !strings.Contains(head, "regenerated by stub") {
		t.Fatalf("regenerated templ artifact must be part of the bump commit, HEAD has: %q", head)
	}
}

// TestDepBumpExecutorRollbackRestoresRealChanges is the REAL rollback
// proof: bump 1 lands (go.mod/go.sum change), bump 2 cannot resolve, so
// the executor must restore the modified files. A git checkout carrying
// even one pathspec that matches nothing aborts WITHOUT restoring
// anything — the concrete-path scope is what makes this work, and this
// test is the regression pin for it.
func TestDepBumpExecutorRollbackRestoresRealChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := depBumpFixtureRepo(t, "v1.10.0")

	goModBefore := readFile(t, filepath.Join(repo, "go.mod"))
	goSumBefore := readFile(t, filepath.Join(repo, "go.sum"))

	e := &DepBumpExecutor{ExtraEnv: []string{GoEnvExperiment}}
	err := e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{
		Repo: repo,
		Bumps: []DepBump{
			{Module: "github.com/stretchr/testify", Version: "v1.11.1"},   // lands
			{Module: "example.invalid/does/not/exist", Version: "v1.0.0"}, // fails
		},
	}))
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("unresolvable second bump must fail with rollback context, got %v", err)
	}

	if after := readFile(t, filepath.Join(repo, "go.mod")); after != goModBefore {
		t.Error("go.mod must be restored after a mid-bump failure")
	}

	if after := readFile(t, filepath.Join(repo, "go.sum")); after != goSumBefore {
		t.Error("go.sum must be restored after a mid-bump failure")
	}

	if status := runGitOut(t, repo, "status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Fatalf("tree must be clean after rollback, status: %q", status)
	}
}

// TestDepBumpExecutorRebumpIsIdempotent pins the nothing-to-commit path:
// re-running the same bump must SUCCEED without adding a commit (the goal
// — repo pinned at the target version — is already met), not burn retries
// into the DLQ.
func TestDepBumpExecutorRebumpIsIdempotent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := depBumpFixtureRepo(t, "v1.10.0")

	e := &DepBumpExecutor{ExtraEnv: []string{GoEnvExperiment}}
	payload := DepBumpPayload{
		Repo:  repo,
		Bumps: []DepBump{{Module: "github.com/stretchr/testify", Version: "v1.11.1"}},
	}

	if err := e.Execute(context.Background(), depBumpTaskT(t, payload)); err != nil {
		t.Fatalf("first bump: %v", err)
	}

	head := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))

	if err := e.Execute(context.Background(), depBumpTaskT(t, payload)); err != nil {
		t.Fatalf("re-bump must succeed, not fail the task: %v", err)
	}

	if after := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD")); after != head {
		t.Errorf("re-bump must not add a commit, HEAD moved: %s -> %s", head, after)
	}
}
