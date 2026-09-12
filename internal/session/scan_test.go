package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTrailerCommitsKeepsOnlyMatchingFooter(t *testing.T) {
	out := strings.Join([]string{
		"ccccdd", "foreign commit", "",                 // no trailer at all
		"bbbbbb", "mentions footer in body", "Crush-Session: other-session",
		"aaaaaa", "the session's commit", "  " + "sess-abc",
		"",
	}, "\n")

	got := parseTrailerCommits(out, "sess-abc")
	if len(got) != 1 || got[0].SHA != "aaaaaa" || got[0].Subject != "the session's commit" {
		t.Fatalf("parsed = %+v, want only aaaaaa", got)
	}
}

func TestParseTrailerCommitsOrdersOldestFirst(t *testing.T) {
	out := strings.Join([]string{
		"111111", "newest", "sess-abc",
		"222222", "middle", "sess-abc",
		"333333", "oldest", "sess-abc",
		"",
	}, "\n")

	got := parseTrailerCommits(out, "sess-abc")
	if len(got) != 3 {
		t.Fatalf("parsed %d commits, want 3", len(got))
	}

	if got[0].SHA != "333333" || got[2].SHA != "111111" {
		t.Fatalf("order = %s..%s, want oldest first", got[0].SHA, got[2].SHA)
	}
}

func TestParseTrailerCommitsMultiLineTrailers(t *testing.T) {
	// A second trailer value on its own line must not swallow the next
	// record: the hex-SHA record start wins over continuation parsing.
	out := strings.Join([]string{
		"111111", "has two values", "other-id", "sess-abc",
		"222222", "next record", "sess-abc",
		"",
	}, "\n")

	got := parseTrailerCommits(out, "sess-abc")
	if len(got) != 2 || got[0].SHA != "111111" || got[1].SHA != "222222" {
		t.Fatalf("parsed = %+v, want both commits", got)
	}
}

func TestParseTrailerCommitsExactValueOnly(t *testing.T) {
	out := strings.Join([]string{
		"111111", "prefix match is not a match", "sess-abc-suffix",
		"222222", "exact", "sess-abc",
		"",
	}, "\n")

	got := parseTrailerCommits(out, "sess-abc")
	if len(got) != 1 || got[0].SHA != "222222" {
		t.Fatalf("parsed = %+v, want only the exact footer match", got)
	}
}

func TestParseTrailerCommitsEmptyOutput(t *testing.T) {
	if got := parseTrailerCommits("", "sess-abc"); got != nil {
		t.Fatalf("empty output parsed to %+v", got)
	}
}

func TestIsHexSHAAcceptsSHA1AndSHA256(t *testing.T) {
	if !isHexSHA(strings.Repeat("a", 40)) || !isHexSHA(strings.Repeat("b", 64)) {
		t.Fatal("valid SHAs rejected")
	}

	if isHexSHA(strings.Repeat("g", 40)) || isHexSHA("aaaa") || isHexSHA("") {
		t.Fatal("invalid fields accepted")
	}
}
