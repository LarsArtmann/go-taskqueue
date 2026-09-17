package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/larsartmann/go-taskqueue/internal/executor"
)

// cmdCrush is session-close trigger #2, the no-hook fallback: it wraps a
// crush session so the close-out happens on process exit, even when neither
// the PreToolUse registry hook nor #3146 SessionEnd is available. The child
// runs with inherited stdio (the operator sees everything live); the wrapper
// tees the output to find the session id, and after the child exits — clean
// or crashed — runs the same replay-safe `tq session close` the sweeper uses.
func cmdCrush(args []string) error {
	code, err := crushRun(args, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	// The wrapper's status is the crush session's status; the close flow has
	// already run by the time we get here.
	if code != 0 {
		os.Exit(code)
	}

	return nil
}

// crushRun runs the wrapped crush child over the given streams and then the
// session-close flow. It returns the child's exit code (0 on success or when
// the child could not even be identified as failing) so cmdCrush can stay
// exit-code free and testable.
func crushRun(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("crush", flag.ExitOnError)

	bin := fs.String("bin", executor.DefaultAgentBinary, "crush binary to run")
	id := fs.String(
		"id",
		os.Getenv("CRUSH_SESSION_ID"),
		"session id (default $CRUSH_SESSION_ID, else scanned from the child output)",
	)
	repo := fs.String("repo", ".", "repository the session works in")
	project := fs.String("project", "", "queue project (default: the repo directory's name)")
	summary := fs.String("summary", "", "one-paragraph summary attached to every minted close")
	allowDirty := fs.Bool("allow-dirty", false, "minted review/status tolerate an uncommitted tree")
	db := dbFlag(fs)

	// Flag parsing stops at the first non-flag argument: everything from
	// there on is the crush invocation (`tq crush run -m "..."`).
	if err := fs.Parse(args); err != nil {
		return 0, err
	}

	abs, err := filepath.Abs(*repo)
	if err != nil {
		return 0, fmt.Errorf("tq crush: resolve repo: %w", err)
	}

	if *project == "" {
		*project = filepath.Base(abs)
	}

	child := exec.Command(*bin, fs.Args()...)
	child.Dir = abs
	child.Stdin = stdin

	var tee bytes.Buffer
	child.Stdout = io.MultiWriter(stdout, &tee)
	child.Stderr = io.MultiWriter(stderr, &tee)

	if err := child.Start(); err != nil {
		return 0, fmt.Errorf("tq crush: start %s: %w", *bin, err)
	}

	done := make(chan struct{})
	go forwardSignals(child, done)

	code := 0
	if err := child.Wait(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return 0, fmt.Errorf("tq crush: wait: %w", err)
		}

		code = exitErr.ExitCode()
	}

	close(done)

	sessionID := *id
	if sessionID == "" {
		sessionID = executor.ExtractSessionID(tee.String())
	}

	if sessionID == "" {
		fmt.Fprintln(
			stderr,
			"tq crush: no session id ($CRUSH_SESSION_ID unset, none in the output) — nothing to close",
		)

		return code, nil
	}

	if err := runSessionClose(sessionID, abs, *project, *summary, *allowDirty, resolveDB(*db)); err != nil {
		return code, fmt.Errorf("tq crush: session close: %w", err)
	}

	return code, nil
}

// forwardSignals relays interrupt/terminate to the child so a Ctrl-C aimed at
// the wrapper still lets crush shut down; the close flow then runs on wait.
func forwardSignals(child *exec.Cmd, done <-chan struct{}) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	select {
	case s := <-sigs:
		_ = child.Process.Signal(s)
	case <-done:
	}
}
