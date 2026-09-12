package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
		return errors.New("usage: tq session begin | close")
	}

	switch args[0] {
	case "begin":
		return sessionBegin(args[1:])
	case "close":
		return sessionClose(args[1:])
	default:
		return fmt.Errorf("unknown tq session command %q (want begin or close)", args[0])
	}
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

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	res, err := session.Close(context.Background(), store, session.GitLogScanner{}, session.CloseInput{
		ID:         *id,
		Repo:       abs,
		Project:    *project,
		Summary:    *summary,
		AllowDirty: *allowDirty,
	})
	if err != nil {
		return err
	}

	if len(res.Commits) == 0 {
		fmt.Printf("session %s closed (repo %s)\n", *id, abs)
		fmt.Printf("no commits carry the %s: %s footer — nothing to review or report\n", session.Trailer, *id)

		return nil
	}

	fmt.Printf("session %s closed (repo %s): %d attributed commit(s), oldest first:\n", *id, abs, len(res.Commits))

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
