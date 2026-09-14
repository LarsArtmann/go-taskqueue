//go:build unix

package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// gitFixtureRepo creates a committed git repo and returns the repo path
// plus helpers to write files and commit them with an optional trailer
// footer.
func gitFixtureRepo(t *testing.T) (string, func(name, body string), func(msg, trailer string)) {
	t.Helper()

	repo := t.TempDir()

	git := func(args ...string) {
		t.Helper()

		cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", repo}, args...)...)

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	git("init", "-q")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "t")

	write := func(name, body string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}

		git("add", name)
	}

	commit := func(msg, trailer string) {
		t.Helper()

		if trailer != "" {
			git("commit", "-qm", msg, "-m", trailer)

			return
		}

		git("commit", "-qm", msg)
	}

	write("seed.txt", "seed")
	commit("seed", "")

	return repo, write, commit
}

func TestDeriveOutcomeAttributesFooterCommits(t *testing.T) {
	repo, write, commit := gitFixtureRepo(t)

	id := task.ID("000001a0testid000000000000")

	write("a.go", "one")
	commit("work one", TaskTrailer+": "+id.String())
	write("b.go", "two")
	commit("work two", TaskTrailer+": "+id.String())
	write("unrelated.txt", "no footer")
	commit("foreign", "")

	got := deriveOutcome(context.Background(), repo, "", id)

	if len(got.Commits) != 2 {
		t.Fatalf("commits = %+v, want exactly the two footer commits", got.Commits)
	}

	if got.Commits[0].Subject != "work one" || got.Commits[1].Subject != "work two" {
		t.Fatalf("order wrong (want oldest first): %+v", got.Commits)
	}

	if len(got.Files) != 2 || got.Files[0] != "a.go" || got.Files[1] != "b.go" {
		t.Fatalf("files = %v, want sorted union [a.go b.go]", got.Files)
	}
}

func TestDeriveOutcomeDegradesOnNonGitDir(t *testing.T) {
	got := deriveOutcome(context.Background(), t.TempDir(), "", task.ID("000001a0testid000000000000"))

	if len(got.Commits) != 0 || len(got.Files) != 0 || got.SessionCostUSD != 0 {
		t.Fatalf("derivation on a non-git dir must be zero, got %+v", got)
	}
}
