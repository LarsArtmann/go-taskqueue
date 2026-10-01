package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupGitRepo creates a committed git repo at dir so the clean-tree guard
// has something real to inspect. Untagged on purpose: the gitscan tests
// that call it are cross-platform (they skip when git is missing), and it
// must stay reachable from them on windows too.
func setupGitRepo(t *testing.T, dir string) {
	t.Helper()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir

		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")

	// Repo-local identity: stub agents commit inside the fixture with the
	// process env only — on CI runners without a global gitconfig that dies
	// with "Author identity unknown" (exit 128) and turns gate tests into
	// run-failures. Local config keeps the fixture hermetic.
	run("config", "user.name", "t")
	run("config", "user.email", "t@t")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	run("add", "-A")
	run("commit", "-qm", "init")
}
