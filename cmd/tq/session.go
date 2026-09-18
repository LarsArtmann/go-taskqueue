package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/session"
)

// cmdSession is the interactive-session close-out bridge (`tq session begin`
// / `tq session close`): every interactive crush session gets the same
// review + status close-out pool agents get. Begin records the session's
// opening in the journal; close attributes the session's commits (the
// `Crush-Session: <id>` git footer) and directly enqueues ONE review task
// over the commit range plus ONE done-prompt status task. Both ride the
// ordinary pool; close itself is enqueue-only so a SessionEnd hook (2s
// budget) can call it.
func cmdSession(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tq session begin | close | list | ping | sweep")
	}

	switch args[0] {
	case "begin":
		return sessionBegin(args[1:])
	case "close":
		return sessionClose(args[1:])
	case "list":
		return sessionList(args[1:])
	case "ping":
		return sessionPing(args[1:])
	case "sweep":
		return sessionSweep(args[1:])
	default:
		return fmt.Errorf("unknown tq session command %q (want begin, close, ping, or sweep)", args[0])
	}
}

// sessionPing is the PreToolUse hook entry point: append one {id, cwd,
// last_seen} observation to the session registry. It touches no database —
// only the registry file — so it fits any hook budget.
func sessionPing(args []string) error {
	fs := flag.NewFlagSet("session ping", flag.ExitOnError)

	id := fs.String("id", os.Getenv("CRUSH_SESSION_ID"), "interactive session id (default $CRUSH_SESSION_ID)")
	cwd := fs.String("cwd", "", "working directory to record (default: the current directory)")
	registry := fs.String("registry", "", "registry file (default $TQ_SESSION_REGISTRY or the user cache dir)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *id == "" {
		return errors.New("tq session ping: no session id — pass --id or run inside crush ($CRUSH_SESSION_ID)")
	}

	if *cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("tq session ping: resolve working directory: %w", err)
		}

		*cwd = wd
	}

	path := *registry
	if path == "" {
		p, err := session.RegistryPath()
		if err != nil {
			return err
		}

		path = p
	}

	return session.PingRegistry(path, *id, *cwd, time.Now())
}

// sessionSweep closes every registry session that has gone quiet and is no
// longer owned by a live crush process: the interim trigger for the
// interactive-session close-out until crush #3146 SessionEnd hooks land.
// Close is replay-safe, so re-sweeping never duplicates minted tasks.
func sessionSweep(args []string) error {
	fs := flag.NewFlagSet("session sweep", flag.ExitOnError)

	registry := fs.String("registry", "", "registry file (default $TQ_SESSION_REGISTRY or the user cache dir)")
	staleAfter := fs.Duration("stale-after", 10*time.Minute, "close a session only after this much silence")
	allowDirty := fs.Bool("allow-dirty", false, "minted review/status tolerate an uncommitted tree")
	summary := fs.String("summary", "", "one-paragraph summary attached to every minted close")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := *registry
	if path == "" {
		p, err := session.RegistryPath()
		if err != nil {
			return err
		}

		path = p
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	outcomes, err := session.Sweep(context.Background(), store, session.GitLogScanner{}, session.SweepInput{
		RegistryPath: path,
		StaleAfter:   *staleAfter,
		AllowDirty:   *allowDirty,
		Summary:      *summary,
	})
	if err != nil {
		return err
	}

	closed := 0

	for _, out := range outcomes {
		if out.Err != nil {
			fmt.Printf("session %s (%s): %v\n", out.Entry.ID, out.Entry.CWD, out.Err)

			continue
		}

		if out.Closed {
			closed++
		}
		fmt.Printf("session %s (%s): %s\n", out.Entry.ID, out.Entry.CWD, out.Reason)
	}

	fmt.Printf("swept %d session(s), closed %d\n", len(outcomes), closed)

	return nil
}

func sessionList(args []string) error {
	fs := flag.NewFlagSet("session list", flag.ExitOnError)
	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	open, err := session.List(context.Background(), store)
	if err != nil {
		return err
	}

	if len(open) == 0 {
		fmt.Println("no open sessions")

		return nil
	}

	fmt.Printf("%d open session(s), newest first:\n", len(open))

	for _, s := range open {
		project := s.Project
		if project == "" {
			project = "-"
		}

		fmt.Printf(
			"  %s  opened %s  repo %s  project %s\n",
			s.ID,
			s.OpenedAt.Format("2006-01-02 15:04:05"),
			s.Repo,
			project,
		)
	}

	return nil
}

func sessionBegin(args []string) error {
	fs := flag.NewFlagSet("session begin", flag.ExitOnError)

	id := fs.String("id", os.Getenv("CRUSH_SESSION_ID"), "interactive session id (default $CRUSH_SESSION_ID)")
	repo := fs.String("repo", ".", "repository the session works in")
	project := fs.String("project", "", "queue project (default: the repo directory's name)")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *id == "" {
		return errors.New("tq session begin: no session id — pass --id or run inside crush ($CRUSH_SESSION_ID)")
	}

	abs, err := filepath.Abs(*repo)
	if err != nil {
		return fmt.Errorf("tq session begin: resolve repo: %w", err)
	}

	if *project == "" {
		*project = filepath.Base(abs)
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	if err := session.Begin(context.Background(), store, *id, abs, *project); err != nil {
		return err
	}

	fmt.Printf("session %s opened (repo %s, project %s)\n", *id, abs, *project)
	fmt.Printf(
		"attribute your commits by ending the commit message with this footer line:\n\n%s: %s\n",
		session.Trailer,
		*id,
	)

	return nil
}

func sessionClose(args []string) error {
	fs := flag.NewFlagSet("session close", flag.ExitOnError)

	id := fs.String("id", os.Getenv("CRUSH_SESSION_ID"), "interactive session id (default $CRUSH_SESSION_ID)")
	repo := fs.String("repo", ".", "repository the session worked in")
	project := fs.String("project", "", "queue project (default: the repo directory's name)")
	summary := fs.String(
		"summary",
		"",
		"one-paragraph account of what the session did (the review's bar, the status entry's item)",
	)
	allowDirty := fs.Bool(
		"allow-dirty",
		false,
		"minted review/status tolerate an uncommitted tree (mirror of the pool's --allow-dirty)",
	)
	dryRun := fs.Bool(
		"dry-run",
		false,
		"preview the attributed commits and what close would enqueue, without touching the database",
	)

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *id == "" {
		return errors.New("tq session close: no session id — pass --id or run inside crush ($CRUSH_SESSION_ID)")
	}

	abs, err := filepath.Abs(*repo)
	if err != nil {
		return fmt.Errorf("tq session close: resolve repo: %w", err)
	}

	if *project == "" {
		*project = filepath.Base(abs)
	}

	if *dryRun {
		return runSessionDryRun(*id, abs)
	}

	return runSessionClose(*id, abs, *project, *summary, *allowDirty, resolveDB(*db))
}

// runSessionDryRun previews close's attribution without enqueueing anything:
// the same git scan Close runs, printed oldest first, plus the tasks close
// WOULD mint. No database access.
func runSessionDryRun(id, abs string) error {
	commits, err := session.GitLogScanner{}.CommitsByTrailer(context.Background(), abs, session.Trailer, id)
	if err != nil {
		return err
	}

	if len(commits) == 0 {
		fmt.Printf("dry run: no commits carry the %s: %s footer — close would enqueue nothing\n", session.Trailer, id)

		return nil
	}

	fmt.Printf("dry run: %d attributed commit(s), oldest first:\n", len(commits))

	for _, c := range commits {
		fmt.Printf("  %s %s\n", shortSHA(c.SHA), c.Subject)
	}

	fmt.Println("close would enqueue one review task and one status task over these commits (dedup-keyed, replay-safe)")

	return nil
}

// runSessionClose is the shared close flow behind `tq session close` and the
// `tq crush` wrapper: attribute the session's footer commits, record the
// session.closed fact, mint the review + status close-out, and print the
// human outcome. Replay-safe via the close dedup keys.
func runSessionClose(id, abs, project, summary string, allowDirty bool, dbPath string) error {
	store := mustOpenDB(dbPath)
	defer store.Close()

	res, err := session.Close(context.Background(), store, session.GitLogScanner{}, session.CloseInput{
		ID:         id,
		Repo:       abs,
		Project:    project,
		Summary:    summary,
		AllowDirty: allowDirty,
	})
	if err != nil {
		return err
	}

	if len(res.Commits) == 0 {
		fmt.Printf("session %s closed (repo %s)\n", id, abs)
		fmt.Printf("no commits carry the %s: %s footer — nothing to review or report\n", session.Trailer, id)
		fmt.Printf(
			"next time, attribute your commits by ending the commit message with this footer line:\n\n%s: %s\n",
			session.Trailer,
			id,
		)

		return nil
	}

	fmt.Printf("session %s closed (repo %s): %d attributed commit(s), oldest first:\n", id, abs, len(res.Commits))

	for _, c := range res.Commits {
		fmt.Printf("  %s %s\n", shortSHA(c.SHA), c.Subject)
	}

	state := mintState(res.ReviewFresh)
	fmt.Printf("review task %s %s\n", res.ReviewTask.ID, state)
	fmt.Printf("status task %s %s\n", res.StatusTask.ID, mintState(res.StatusFresh))

	return nil
}

// mintState names the enqueue outcome for humans: fresh mint or dedup hit.
func mintState(fresh bool) string {
	if fresh {
		return "enqueued"
	}

	return "known (dedup)"
}

func shortSHA(sha string) string {
	if len(sha) > 9 {
		return sha[:9]
	}

	return sha
}
