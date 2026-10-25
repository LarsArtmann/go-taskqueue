package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/session"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// ownerSessionFreshness bounds how recently a registry ping counts as a
// live owner: interactive sessions ping on every tool call, so anything
// quieter than this window has ended (the sweep closes it).
const ownerSessionFreshness = 10 * time.Minute

// ownerSessionParkReason rides the task.parked fact detail.
const ownerSessionParkReason = "owner session active"

// liveOwnerRepos resolves which repos under projectsDir hold a live owner
// session: a fresh registry ping whose cwd sits inside <projectsDir>/<repo>
// (at any depth — a session cd'd into a subdir is still writing that repo).
func liveOwnerRepos(entries []session.RegistryEntry, projectsDir string, now time.Time) map[string]bool {
	live := map[string]bool{}

	base, err := filepath.Abs(projectsDir)
	if err != nil {
		return live
	}

	for _, entry := range entries {
		if now.Sub(entry.LastSeen) > ownerSessionFreshness {
			continue
		}

		cwd, err := filepath.Abs(entry.CWD)
		if err != nil {
			continue
		}

		if cwd == base || !strings.HasPrefix(cwd, base+string(filepath.Separator)) {
			continue
		}

		repo := strings.TrimPrefix(cwd, base+string(filepath.Separator))
		if i := strings.IndexRune(repo, filepath.Separator); i >= 0 {
			repo = repo[:i]
		}

		if repo != "" {
			live[repo] = true
		}
	}

	return live
}

// parkBehindOwnerSessions is the two-writer policy (C15.3): while an owner
// crush session is live inside a harvested repo, that repo's PENDING tasks
// park for one hold horizon instead of claiming into the owner's live
// worktree — the claim/conflict/requeue churn loop becomes a visible,
// expiring park that burns no attempt. The pass runs once per pool tick
// BEFORE the harvest; parks fire only when a task is actually claimable
// (NotBefore already in the future = already suppressed, no fact spam), so
// a busy repo mints at most one park fact per task per hold window. When
// the session goes quiet, the last horizon lapses and claims resume on
// their own — no unpark sweep exists or needs to.
func parkBehindOwnerSessions(
	ctx context.Context,
	store queue.Store,
	projectsDir string,
	hold time.Duration,
	log *slog.Logger,
) {
	registryPath, err := session.RegistryPath()
	if err != nil {
		if log != nil {
			log.Warn("owner-session park: registry path unresolvable", "err", err)
		}

		return
	}

	entries, err := session.LoadRegistry(registryPath)
	if err != nil {
		if log != nil {
			log.Warn("owner-session park: registry unreadable", "err", err)
		}

		return
	}

	now := time.Now()

	live := liveOwnerRepos(entries, projectsDir, now)
	if len(live) == 0 {
		return
	}

	pending := task.Pending

	tasks, err := store.List(ctx, queue.Filter{Status: &pending})
	if err != nil {
		if log != nil {
			log.Warn("owner-session park: list failed", "err", err)
		}

		return
	}

	until := now.Add(hold)

	for _, t := range tasks {
		if !live[t.Project] || t.NotBefore.After(now) {
			continue
		}

		if err := store.Park(ctx, t.ID, ownerSessionParkReason, until, "pool"); err != nil {
			if log != nil {
				log.Warn("owner-session park failed", "repo", t.Project, "task", t.ID.String(), "err", err)
			}

			continue
		}

		if log != nil {
			log.Info("owner-session park",
				"repo", t.Project,
				"task", t.ID.String(),
				"until", until.Format(time.RFC3339),
			)
		}
	}
}
