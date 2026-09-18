package executor

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

const (
	shaA = "aaaaaa0123456789abcdef0123456789abcdef01"
	shaB = "bbbbbb0123456789abcdef0123456789abcdef01"
	shaC = "cccccc0123456789abcdef0123456789abcdef01"
)

// rec renders one git-log record the way --format=%H%x1f%s%x1f%(trailers)
// prints it: SHA, subject and the trailer values separated by \x1f.
func rec(sha, subject string, trailers ...string) string {
	return sha + "\x1f" + subject + "\x1f" + strings.Join(trailers, "\n")
}

func TestParseTrailerCommitsKeepsOnlyMatchingFooter(t *testing.T) {
	out := strings.Join([]string{
		rec(shaC, "foreign commit"),
		rec(shaB, "mentions footer in body", "Crush-Session: other-session"),
		rec(shaA, "the session's commit", "  sess-abc"),
		"",
	}, "\n")

	got := parseTrailerCommits(out, "sess-abc")
	if len(got) != 1 || got[0].SHA != shaA || got[0].Subject != "the session's commit" {
		t.Fatalf("parsed = %+v, want only %s…", got, shaA[:6])
	}
}

func TestParseTrailerCommitsOrdersOldestFirst(t *testing.T) {
	out := strings.Join([]string{
		rec(shaC, "newest", "sess-abc"),
		rec(shaB, "middle", "sess-abc"),
		rec(shaA, "oldest", "sess-abc"),
		"",
	}, "\n")

	got := parseTrailerCommits(out, "sess-abc")
	if len(got) != 3 {
		t.Fatalf("parsed %d commits, want 3", len(got))
	}

	if got[0].SHA != shaA || got[2].SHA != shaC {
		t.Fatalf("order = %s..%s, want oldest first", got[0].SHA[:6], got[2].SHA[:6])
	}
}

func TestParseTrailerCommitsMultiValueTrailers(t *testing.T) {
	// A second trailer value on its own line must not swallow the next
	// record: the hex-SHA record start wins over continuation parsing.
	out := strings.Join([]string{
		rec(shaB, "has two values", "other-id", "sess-abc"),
		rec(shaA, "next record", "sess-abc"),
		"",
	}, "\n")

	got := parseTrailerCommits(out, "sess-abc")
	if len(got) != 2 || got[0].SHA != shaA || got[1].SHA != shaB {
		t.Fatalf("parsed = %+v, want both commits, oldest first", got)
	}
}

func TestParseTrailerCommitsExactValueOnly(t *testing.T) {
	out := strings.Join([]string{
		rec(shaB, "prefix match is not a match", "sess-abc-suffix"),
		rec(shaA, "exact", "sess-abc"),
		"",
	}, "\n")

	got := parseTrailerCommits(out, "sess-abc")
	if len(got) != 1 || got[0].SHA != shaA {
		t.Fatalf("parsed = %+v, want only the exact footer match", got)
	}
}

func TestParseTrailerCommitsSHA256Records(t *testing.T) {
	out := rec(strings.Repeat("d", 64), "sha256 repo", "sess-abc") + "\n"

	got := parseTrailerCommits(out, "sess-abc")
	if len(got) != 1 || got[0].SHA != strings.Repeat("d", 64) {
		t.Fatalf("parsed = %+v, want the sha256 record", got)
	}
}

func TestParseTrailerCommitsEmptyOutput(t *testing.T) {
	if got := parseTrailerCommits("", "sess-abc"); got != nil {
		t.Fatalf("empty output parsed to %+v", got)
	}
}

// TestParseTrailerCommitsIgnoresBlankAndWhitespaceFooters pins the footer
// edge cases (03-28 §f17): empty trailer fields, whitespace-only trailer
// lines and empty footer VALUES never attribute a commit — and an empty
// session id can never match anything.
func TestParseTrailerCommitsIgnoresBlankAndWhitespaceFooters(t *testing.T) {
	out := strings.Join([]string{
		rec(shaC, "empty footer value", ""),
		rec(shaB, "whitespace-only footer", "   "),
		rec(shaA, "real footer", "sess-abc"),
		"",
	}, "\n")

	if got := parseTrailerCommits(out, "sess-abc"); len(got) != 1 || got[0].SHA != shaA {
		t.Fatalf("parsed = %+v, want only the real footer's commit", got)
	}

	if got := parseTrailerCommits(out, ""); got != nil {
		t.Fatalf("empty session id attributed %+v, want nothing", got)
	}
}

// TestParseTrailerCommitsRepeatedFooterAttributesOnce pins the repeated
// footer case (03-28 §f17): the same session id twice on one commit (e.g. an
// amend that stacked trailers) attributes the commit exactly once.
func TestParseTrailerCommitsRepeatedFooterAttributesOnce(t *testing.T) {
	out := rec(shaA, "amended twice", "sess-abc", "sess-abc") + "\n"

	got := parseTrailerCommits(out, "sess-abc")
	if len(got) != 1 || got[0].SHA != shaA {
		t.Fatalf("parsed = %+v, want the commit once", got)
	}
}

func TestIsHexSHAAcceptsSHA1AndSHA256(t *testing.T) {
	if !isHexSHA(shaA) || !isHexSHA(strings.Repeat("b", 64)) {
		t.Fatal("valid SHAs rejected")
	}

	if isHexSHA(strings.Repeat("g", 40)) || isHexSHA("aaaa") || isHexSHA("") {
		t.Fatal("invalid fields accepted")
	}
}

func TestParseGitVersion(t *testing.T) {
	cases := []struct {
		name         string
		out          string
		major, minor int
		ok           bool
	}{
		{"modern", "git version 2.51.0", 2, 51, true},
		{"two-part", "git version 2.14", 2, 14, true},
		{"prefix noise", "git for computers git version 1.9.1", 1, 9, true},
		{"no version token", "something else entirely", 0, 0, false},
		{"non-numeric", "git version banana", 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			major, minor, ok := parseGitVersion(tc.out)
			if ok != tc.ok || major != tc.major || minor != tc.minor {
				t.Fatalf("parseGitVersion(%q) = %d, %d, %v; want %d, %d, %v", tc.out, major, minor, ok, tc.major, tc.minor, tc.ok)
			}
		})
	}
}

func TestCheckGitVersionRefusesPre215(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := t.TempDir()

	cmd := exec.CommandContext(context.Background(), "git", "init", "-q", "-b", "main")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	if err := checkGitVersion(context.Background(), "git", repo); err != nil {
		t.Fatalf("current git rejected: %v", err)
	}
}
