package session

import (
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

func TestIsHexSHAAcceptsSHA1AndSHA256(t *testing.T) {
	if !isHexSHA(shaA) || !isHexSHA(strings.Repeat("b", 64)) {
		t.Fatal("valid SHAs rejected")
	}

	if isHexSHA(strings.Repeat("g", 40)) || isHexSHA("aaaa") || isHexSHA("") {
		t.Fatal("invalid fields accepted")
	}
}
