package session

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Commit is one session-attributed commit.
type Commit struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

// GitScanner attributes commits: which commits in repo carry the git footer
// `key: value`. The interface keeps Close hermetic to test; GitLogScanner is
// the real implementation.
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

// CommitsByTrailer implements GitScanner. A repository without any commit
// yet (unborn HEAD) attributes nothing — that is an empty session, not an
// error.
func (s GitLogScanner) CommitsByTrailer(ctx context.Context, repo, key, value string) ([]Commit, error) {
	if repo == "" {
		return nil, errors.New("session: empty repo path")
	}

	bin := s.Bin
	if bin == "" {
		bin = "git"
	}

	// One invocation, fields NUL-free: SHA, subject and the trailer values
	// (one per line) separated by \x1f. The trailer machinery matches the
	// footer exactly (value-only, key-scoped), so a message merely quoting
	// someone else's footer is never attributed.
	cmd := exec.CommandContext( //nolint:gosec // G204: repo is the operator's own path, the format literal is fixed; running git over user-named repos is the bridge's core feature (AGENTS.md gosec triage)
		ctx, bin, "-C", repo, "log",
		"--format=%H%x1f%s%x1f%(trailers:key="+key+",valueonly)",
	)

	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && strings.Contains(string(exit.Stderr), "does not have any commits yet") {
			return nil, nil
		}

		return nil, fmt.Errorf("session: git log %s: %w", repo, err)
	}

	return parseTrailerCommits(string(out), value), nil
}

// parseTrailerCommits splits git-log output into commit records (a record
// starts at a hex SHA field) and keeps those whose trailer block contains a
// line equal to the session id. Records arrive newest first; the result is
// oldest first.
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
		sha, subject, trailer, ok := splitRecordLine(line)
		if ok {
			records = append(records, record{commit: Commit{SHA: sha, Subject: subject}})
			current = &records[len(records)-1]
		}

		if current != nil && trailer != "" {
			current.trailers = append(current.trailers, trailer)
		}
	}

	var commits []Commit

	for _, r := range records {
		for _, trailer := range r.trailers {
			if strings.TrimSpace(trailer) == sessionID {
				commits = append(commits, r.commit)

				break
			}
		}
	}

	// git log is newest first; the bridge reasons in ranges (oldest..newest).
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	return commits
}

// splitRecordLine splits one output line into (sha, subject, trailer-line,
// ok). A record line carries two \x1f separators and a hex SHA first field;
// continuation lines (multi-value trailer output) return ok=false.
func splitRecordLine(line string) (sha, subject, trailer string, ok bool) {
	parts := strings.SplitN(line, "\x1f", 3)
	if len(parts) != 3 || !isHexSHA(parts[0]) {
		return "", "", "", false
	}

	return parts[0], parts[1], parts[2], true
}

func isHexSHA(s string) bool {
	if len(s) != 40 && len(s) != 64 { // SHA-1 and SHA-256 repositories
		return false
	}

	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}

	return true
}
