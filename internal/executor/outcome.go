package executor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	crushdata "github.com/LarsArtmann/go-crush-data"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TaskTrailer is the git-footer key that attributes a commit to a queue
// task: agents end commit messages with `Task-Queue-ID: <id>` (the prompt
// contracts teach it; the executor resolves the placeholder at run time).
// Outcome derivation reads the same trailer the session-close bridge reads
// for `Crush-Session`.
const TaskTrailer = "Task-Queue-ID"

// Derivation is best-effort: these sentinels never surface to users, they
// only mark which enrichment source came back empty.
var (
	errDeriveDiscover = errors.New("derive: discover crush projects")
	errDeriveOpenDB   = errors.New("derive: open crush db")
	errDeriveSession  = errors.New("derive: read session")
	errDeriveProject  = errors.New("derive: no crush project registered for repo")
)

// deriveTimeout bounds outcome derivation (trailer scan, file listing,
// crush registry lookup). Derivation is best-effort enrichment and must
// never stall or fail a task that already passed its verify gate.
const deriveTimeout = 15 * time.Second

// derivedOutcome is what the queue learns about a finished run WITHOUT the
// agent reporting back: the commits its Task-Queue-ID footer attributed
// (git is the source of truth for what SHIPPED) plus what the crush session
// cost (go-crush-data, when the run's session id was extractable).
type derivedOutcome struct {
	Commits []Commit
	Files   []string

	SessionCostUSD          float64
	SessionPromptTokens     int64
	SessionCompletionTokens int64
	SessionMessageCount     int
}

// deriveOutcome attributes a finished run's work. Best-effort by design:
// a non-git repo, an unreadable registry or an unknown session leaves the
// corresponding fields zero — derivation degrades, it never errors.
func deriveOutcome(ctx context.Context, repoDir, sessionID string, id task.ID) derivedOutcome {
	var out derivedOutcome

	// The parent task context, not the run timeout: the run may have spent
	// nearly all of its budget by the time verify passed, and derivation
	// still deserves its own short window.
	dctx, cancel := context.WithTimeout(ctx, deriveTimeout)
	defer cancel()

	if commits, err := (GitLogScanner{}).CommitsByTrailer(dctx, repoDir, TaskTrailer, id.String()); err == nil {
		out.Commits = commits
		out.Files = filesInCommits(dctx, repoDir, commits)
	}

	if sessionID != "" {
		if session, err := crushSession(dctx, repoDir, sessionID); err == nil {
			out.SessionCostUSD = session.CostUSD
			out.SessionPromptTokens = session.PromptTokens
			out.SessionCompletionTokens = session.CompletionTokens
			out.SessionMessageCount = session.MessageCount
		}
	}

	return out
}

// filesInCommits lists the union of files the commits touched, oldest
// commit first, deduplicated and sorted. `git diff-tree --no-commit-id
// --name-only -r` per commit; a commit whose listing fails simply
// contributes nothing.
func filesInCommits(ctx context.Context, repoDir string, commits []Commit) []string {
	var files []string

	for _, c := range commits {
		out, err := exec.CommandContext(
			ctx, "git", "-C", repoDir, "diff-tree", "--no-commit-id", "--name-only", "-r", c.SHA,
		).Output()
		if err != nil {
			continue
		}

		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if line = strings.TrimSpace(line); line != "" && !slices.Contains(files, line) {
				files = append(files, line)
			}
		}
	}

	slices.Sort(files)

	return files
}

// crushSession looks the run's crush session up through the local projects
// registry (projects.json in the global crush data dir) and returns its
// usage row. Read-only and one-shot: open, read, close.
func crushSession(ctx context.Context, repoDir, sessionID string) (crushdata.Session, error) {
	projects, err := crushdata.DiscoverProjects(ctx, crushdata.DiscoverOptions{})
	if err != nil {
		return crushdata.Session{}, fmt.Errorf("%w: %w", errDeriveDiscover, err)
	}

	for _, project := range projects {
		if filepath.Clean(project.Path) != filepath.Clean(repoDir) {
			continue
		}

		db, err := crushdata.OpenContext(ctx, project.DataDir)
		if err != nil {
			return crushdata.Session{}, fmt.Errorf("%w: %w", errDeriveOpenDB, err)
		}

		session, err := db.Session(ctx, sessionID)
		_ = db.Close()

		if err != nil {
			return crushdata.Session{}, fmt.Errorf("%w: %w", errDeriveSession, err)
		}

		return session, nil
	}

	return crushdata.Session{}, fmt.Errorf("%w: %s", errDeriveProject, repoDir)
}
