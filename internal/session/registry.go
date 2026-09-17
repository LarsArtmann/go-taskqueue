package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The PreToolUse session registry: the interim trigger for interactive
// sessions until crush #3146 ships SessionEnd hooks everywhere. A PreToolUse
// hook (`tq session ping`, typically via scripts/hook-session-registry.sh)
// appends one RegistryEntry per tool call; `tq session sweep` periodically
// closes every session that has gone quiet AND is no longer owned by a live
// crush process, minting the same review + status close-out that
// `tq session close` mints. Entries are append-only JSON lines; load is
// latest-wins per session id, and sweep rewrites the file without the ids it
// closed so the registry never grows without bound.

const (
	registryDirPerm  = 0o755
	registryFilePerm = 0o644
)

// RegistryEntry is one ping's observation: which session, working where, seen
// when. Snake_case keys: the file is a machine-payload wire format, appended
// by hooks and consumed by the sweeper (tagliatelle exclusion below).
type RegistryEntry struct {
	ID       string    `json:"id"`
	CWD      string    `json:"cwd"`
	LastSeen time.Time `json:"last_seen"`
}

// RegistryPath resolves the registry file: $TQ_SESSION_REGISTRY if set, else
// <user cache dir>/tq/session-registry.jsonl.
func RegistryPath() (string, error) {
	if p := os.Getenv("TQ_SESSION_REGISTRY"); p != "" {
		return p, nil
	}

	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("session: resolve registry path: %w", err)
	}

	return filepath.Join(base, "tq", "session-registry.jsonl"), nil
}

// PingRegistry appends one observation for id/cwd at now. The line is a
// single small write on an O_APPEND handle, so concurrent hook processes
// never interleave mid-line on POSIX.
func PingRegistry(path, id, cwd string, now time.Time) error {
	if id == "" {
		return errors.New("session: ping needs a session id (--id or $CRUSH_SESSION_ID)")
	}

	if cwd == "" {
		return errors.New("session: ping needs a working directory")
	}

	if err := os.MkdirAll(filepath.Dir(path), registryDirPerm); err != nil {
		return fmt.Errorf("session: create registry dir: %w", err)
	}

	line, err := json.Marshal(RegistryEntry{ID: id, CWD: cwd, LastSeen: now.UTC()})
	if err != nil {
		return fmt.Errorf("session: encode registry entry: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, registryFilePerm)
	if err != nil {
		return fmt.Errorf("session: open registry: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("session: append registry entry: %w", err)
	}

	return nil
}

// LoadRegistry folds the file into the latest entry per session id. Missing
// file means an empty registry, not an error. Blank and unparseable lines are
// skipped (a torn append must not poison the whole registry).
func LoadRegistry(path string) ([]RegistryEntry, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("session: open registry: %w", err)
	}

	defer func() { _ = file.Close() }()

	latest := map[string]RegistryEntry{}
	order := []string{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry RegistryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil || entry.ID == "" {
			continue
		}

		if _, seen := latest[entry.ID]; !seen {
			order = append(order, entry.ID)
		}

		latest[entry.ID] = entry
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("session: read registry: %w", err)
	}

	entries := make([]RegistryEntry, 0, len(order))
	for _, id := range order {
		entries = append(entries, latest[id])
	}

	return entries, nil
}

// RewriteRegistry replaces the file with entries (an empty slice empties it).
// The sweeper uses it to drop the sessions it closed.
func RewriteRegistry(path string, entries []RegistryEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), registryDirPerm); err != nil {
		return fmt.Errorf("session: create registry dir: %w", err)
	}

	var b strings.Builder

	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("session: encode registry entry: %w", err)
		}

		b.Write(line)
		b.WriteByte('\n')
	}

	if err := os.WriteFile(path, []byte(b.String()), registryFilePerm); err != nil {
		return fmt.Errorf("session: write registry: %w", err)
	}

	return nil
}

// ProcessOwner reports whether a live crush process owns a session id. The
// default implementation shells out to pgrep; tests stub it.
type ProcessOwner interface {
	Owns(ctx context.Context, id string) (bool, error)
}

// PgrepOwner is the production ProcessOwner: a session id counts as owned
// while any process whose full command line mentions it is alive.
type PgrepOwner struct{}

// Owns reports whether pgrep -f finds the session id on any live command
// line. A missing pgrep is an error, not a silent "unowned": closing a live
// session behind the sweeper's back is the failure mode this check exists
// to prevent.
func (PgrepOwner) Owns(ctx context.Context, id string) (bool, error) {
	if _, err := exec.LookPath("pgrep"); err != nil {
		return false, fmt.Errorf("session: pgrep unavailable, cannot verify session ownership: %w", err)
	}

	cmd := exec.CommandContext(ctx, "pgrep", "-f", id)

	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, fmt.Errorf("session: pgrep: %w", err)
}

// SweepInput configures one sweep pass.
type SweepInput struct {
	RegistryPath string
	StaleAfter   time.Duration
	Owner        ProcessOwner
	AllowDirty   bool
	Summary      string
	Now          time.Time
}

// SweepOutcome records what the sweeper did to one registry session.
type SweepOutcome struct {
	Entry  RegistryEntry
	Closed bool
	Reason string
	Err    error
}

// Sweep closes every registry session that is both quiet for StaleAfter and
// not owned by a live crush process, minting the ordinary close (replay-safe
// via the dedup keys) and rewriting the registry without the closed ids.
// Entries that are fresh, still owned, or whose close failed stay in the file.
func Sweep(ctx context.Context, s Store, scanner GitScanner, input SweepInput) ([]SweepOutcome, error) {
	if input.Owner == nil {
		input.Owner = PgrepOwner{}
	}

	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}

	entries, err := LoadRegistry(input.RegistryPath)
	if err != nil {
		return nil, err
	}

	outcomes := []SweepOutcome{}
	kept := []RegistryEntry{}

	for _, entry := range entries {
		outcome := SweepOutcome{Entry: entry}

		owned, err := input.Owner.Owns(ctx, entry.ID)
		if err != nil {
			outcome.Err = err
			outcome.Reason = "ownership check failed"
			outcomes = append(outcomes, outcome)
			kept = append(kept, entry)

			continue
		}

		if owned {
			outcome.Reason = "crush process still owns the session"
			outcomes = append(outcomes, outcome)
			kept = append(kept, entry)

			continue
		}

		quiet := now.Sub(entry.LastSeen)
		if quiet < input.StaleAfter {
			outcome.Reason = fmt.Sprintf("last seen %s ago (quiet threshold %s)", quiet.Round(time.Second), input.StaleAfter)
			outcomes = append(outcomes, outcome)
			kept = append(kept, entry)

			continue
		}

		_, err = Close(ctx, s, scanner, CloseInput{
			ID:         entry.ID,
			Repo:       entry.CWD,
			Project:    filepath.Base(entry.CWD),
			Summary:    input.Summary,
			AllowDirty: input.AllowDirty,
		})
		if err != nil {
			outcome.Err = fmt.Errorf("close: %w", err)
			outcome.Reason = "close failed"
			outcomes = append(outcomes, outcome)
			kept = append(kept, entry)

			continue
		}

		outcome.Closed = true
		outcome.Reason = "closed"
		outcomes = append(outcomes, outcome)
	}

	if err := RewriteRegistry(input.RegistryPath, kept); err != nil {
		return outcomes, err
	}

	return outcomes, nil
}
