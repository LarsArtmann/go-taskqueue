package harvest

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// shaRe matches git SHA citations in item text: 7-40 lowercase hex chars on
// word boundaries (the repo convention cites short revs; full SHAs pass).
var shaRe = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)

// danglingSHAs returns the SHA-like tokens in text that resolve to commits
// in repo but are NOT ancestors of HEAD — the pre-rebase dangle class that
// made every closeout re-resolve counterparts (2026-09-14 window: five tasks
// cited pre-rebase SHAs). Candidates that do not resolve at all are prose
// (hash-like words) and are never flagged: only PROVEN dangles are reported.
func danglingSHAs(ctx context.Context, repo, text string) []string {
	var dangles []string

	seen := make(map[string]bool)

	for _, cand := range shaRe.FindAllString(text, -1) {
		if seen[cand] {
			continue
		}

		seen[cand] = true

		if !isDangling(ctx, repo, cand) {
			continue
		}

		dangles = append(dangles, cand)
	}

	return dangles
}

// isDangling probes one candidate with `git merge-base --is-ancestor`:
// exit 0 = reachable from HEAD, exit 1 = exists but NOT an ancestor (the
// dangle class), any other outcome (unresolvable token, not a repo, git
// missing) is never flagged — the check annotates, it never blocks.
func isDangling(ctx context.Context, repo, cand string) bool {
	err := exec.CommandContext(ctx, "git", "-C", repo, "merge-base", "--is-ancestor", cand, "HEAD").Run()

	var exitErr *exec.ExitError

	return err != nil && errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

// citationBlock renders the enqueue-time warning appended to a task prompt
// whose text cites unreachable SHAs: the working session re-resolves each
// to its reachable counterpart FIRST, instead of every closeout run
// rediscovering the dangle.
func citationBlock(dangles []string) string {
	return fmt.Sprintf(
		"CITATION CHECK: this task text cites git SHA(s) %s that are NOT reachable on the repo's current HEAD (pre-rebase dangles). Re-resolve each to its reachable counterpart (git log --all, reflog, or the item's own source report) before relying on it, and fix the source item to cite reachable SHAs only.",
		strings.Join(dangles, ", "),
	)
}
