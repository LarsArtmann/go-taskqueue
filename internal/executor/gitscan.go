package executor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// Commit is one trailer-attributed commit (git footer `key: value`).
type Commit struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

// GitScanner attributes commits: which commits in repo carry the git footer
// `key: value`. The interface keeps consumers (the session-close bridge,
// outcome derivation) testable; GitLogScanner is the real implementation.
type GitScanner interface {
	CommitsByTrailer(ctx context.Context, repo, key, value string) ([]Commit, error)
}

// GitScannerFunc adapts a function to GitScanner.
type GitScannerFunc func(ctx context.Context, repo, key, value string) ([]Commit, error)

// CommitsByTrailer implements GitScanner.
func (f GitScannerFunc) CommitsByTrailer(ctx context.Context, repo, key, value string) ([]Commit, error) {
	return f(ctx, repo, key, value)
}

// GitLogScanner attributes commits by scanning `git log` for the footer
// trailer. Commits come back oldest first. Requires git >= 2.15
// (%(trailers) format); every supported environment clears that floor by
// years.
type GitLogScanner struct {
	// Bin is the git binary; empty means "git" from PATH.
	Bin string
}

// errEmptyRepo is the empty-path rejection (a caller bug, not a scan
// failure).
var errEmptyRepo = errors.New("gitscan: empty repo path")

// minGitVersion is the floor for %(trailers) support; git older than this
// renders unknown placeholders literally and would silently attribute
// nothing.
const minGitMajor, minGitMinor = 2, 15

const (
	maxGitVersionParts = 3
	minGitVersionParts = 2
)

// errGitTooOld is the static sentinel wrapped by checkGitVersion (err113:
// no dynamic error construction at the call site).
var errGitTooOld = errors.New("gitscan: git too old for trailer attribution")

func (s GitLogScanner) bin() string {
	if s.Bin != "" {
		return s.Bin
	}

	return "git"
}

// checkGitVersion refuses git installs older than %(trailers) support: old
// git prints the placeholder literally, so a scan would succeed with zero
// attributions while the session's commits go unattributed. An unparseable
// or failing --version fails open — exotic environments scan best-effort.
func checkGitVersion(ctx context.Context, bin, repo string) error {
	out, err := exec.CommandContext(ctx, bin, "-C", repo, "--version").Output()
	if err != nil {
		return nil //nolint:nilerr // deliberate fail-open on a failing --version (see doc comment)
	}

	major, minor, ok := parseGitVersion(string(out))
	if !ok {
		return nil
	}

	if major < minGitMajor || (major == minGitMajor && minor < minGitMinor) {
		return fmt.Errorf(
			"%w: git %s in %s is version %d.%d; trailer attribution needs git >= %d.%d — upgrade git",
			errGitTooOld, bin, repo, major, minor, minGitMajor, minGitMinor,
		)
	}

	return nil
}

// parseGitVersion extracts the major/minor from `git version X.Y[.Z]`
// output; ok is false when the text is not a recognizable version line.
func parseGitVersion(out string) (int, int, bool) {
	fields := strings.Fields(out)

	for i, f := range fields {
		if f != "version" || i+1 >= len(fields) {
			continue
		}

		parts := strings.SplitN(fields[i+1], ".", maxGitVersionParts)
		if len(parts) < minGitVersionParts {
			return 0, 0, false
		}

		major, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, false
		}

		minor, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}

		return major, minor, true
	}

	return 0, 0, false
}

// CommitsByTrailer implements GitScanner. A repository without any commit
// yet (unborn HEAD) attributes nothing — that is an empty session, not an
// error.
func (s GitLogScanner) CommitsByTrailer(ctx context.Context, repo, key, value string) ([]Commit, error) {
	if repo == "" {
		return nil, errEmptyRepo
	}

	if err := checkGitVersion(ctx, s.bin(), repo); err != nil {
		return nil, err
	}

	bin := s.bin()

	// One invocation, fields NUL-free: SHA, subject and the trailer values
	// (one per line) separated by \x1f. The trailer machinery matches the
	// footer exactly (value-only, key-scoped), so a message merely quoting
	// someone else's footer is never attributed.
	cmd := exec.CommandContext(
		ctx, bin, "-C", repo, "log",
		"--format=%H%x1f%s%x1f%(trailers:key="+key+",valueonly)",
	)

	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && strings.Contains(string(exit.Stderr), "does not have any commits yet") {
			return nil, nil
		}

		return nil, fmt.Errorf("gitscan: git log %s: %w", repo, err)
	}

	return parseTrailerCommits(string(out), value), nil
}

// parseTrailerCommits splits git-log output into commit records (a record
// starts at a hex SHA field; every line after it that is not a new record is
// one of that commit's trailer values) and keeps those whose trailer block
// contains a line equal to the session id. Records arrive newest first; the
// result is oldest first.
func parseTrailerCommits(out, sessionID string) []Commit {
	type record struct {
		commit   Commit
		trailers []string
	}

	var (
		records []record
		current *record
	)

	for line := range strings.SplitSeq(out, "\n") {
		parts := strings.SplitN(line, "\x1f", 3)
		if len(parts) == 3 && isHexSHA(parts[0]) {
			records = append(records, record{commit: Commit{SHA: parts[0], Subject: parts[1]}})
			current = &records[len(records)-1]
		}

		if current == nil {
			continue
		}

		// The record line's own trailer field, or a continuation line of a
		// multi-value trailer block.
		if trailer := strings.TrimSpace(parts[len(parts)-1]); trailer != "" {
			current.trailers = append(current.trailers, trailer)
		}
	}

	var commits []Commit

	for _, r := range records {
		if slices.Contains(r.trailers, sessionID) {
			commits = append(commits, r.commit)
		}
	}

	// git log is newest first; the bridge reasons in ranges (oldest..newest).
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	return commits
}

func isHexSHA(s string) bool {
	if len(s) != 40 && len(s) != 64 { // SHA-1 and SHA-256 repositories
		return false
	}

	for _, c := range s {
		if c < '0' || (c > '9' && c < 'a') || c > 'f' {
			return false
		}
	}

	return true
}
