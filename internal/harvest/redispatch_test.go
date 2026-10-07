package harvest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// candidateWith builds a mint candidate carrying the payload fields the
// mint-time gate reads (the task.New mirror of donepreflight's taskWith).
func candidateWith(dedup, item, prompt, rejectedSHA, anchor string) task.New {
	payload := `{"repo":"preflight","prompt":` + jsonString(prompt) +
		`,"dedup":` + jsonString(dedup) + `,"item":` + jsonString(item)
	if rejectedSHA != "" || anchor != "" {
		payload += `,"rejected_sha":` + jsonString(rejectedSHA) +
			`,"anchor":` + jsonString(anchor)
	}

	return task.New{Type: "agent", Payload: []byte(payload + `}`)}
}

// TestRedispatchCheckFixCuredByReword is the 2026-10-07 four-paid-lap
// class: the rejected SHA still exists, no commit cites it, and the anchor
// text has zero hits at HEAD — the finding was reworded away, so the
// re-verification dispatch must be refused at mint (footer-join dedup).
func TestRedispatchCheckFixCuredByReword(t *testing.T) {
	fixture := newDonePreflightFixture(t, "- [ ] open work\n")

	bad := gitCommit(t, fixture.repo, "the rejected commit")
	removeLine(t, fixture.repo, "commit-marker")

	gitCommit(t, fixture.repo, "reworded the finding away")

	err := fixture.h.RedispatchCheck(
		context.Background(), candidateWith("", "", "re-verify the finding", bad, "the rejected commit"),
	)

	refusal, ok := errors.AsType[*RedispatchRefusal](err)
	if !ok {
		t.Fatalf("expected a redispatch refusal, got %v", err)
	}

	if !strings.Contains(refusal.Reason, "unreferenced and anchor text absent") {
		t.Fatalf("reason should name the unreferenced-SHA cure, got %q", refusal.Reason)
	}

	if !errors.Is(err, ErrRedispatchRefused) {
		t.Fatal("refusal must unwrap to ErrRedispatchRefused")
	}
}

// TestRedispatchCheckItemClosedAtMint is the check-off race (row 164): the
// row was open at scan time and ticked (or removed) before the mint — the
// re-read at mint time refuses the dispatch.
func TestRedispatchCheckItemClosedAtMint(t *testing.T) {
	itemText := "close the status-append loop"
	ticked := newDonePreflightFixture(t, "- [x] "+itemText+"\n")
	absent := newDonePreflightFixture(t, "- [ ] unrelated row\n")

	key := ItemKey("preflight", itemText)

	for name, fixture := range map[string]donePreflightFixture{
		"ticked": ticked,
		"absent": absent,
	} {
		t.Run(name, func(t *testing.T) {
			err := fixture.h.RedispatchCheck(
				context.Background(), candidateWith(key, itemText, "do the thing", "", ""),
			)

			refusal, ok := errors.AsType[*RedispatchRefusal](err)
			if !ok {
				t.Fatalf("expected a redispatch refusal, got %v", err)
			}

			if !strings.Contains(refusal.Reason, "TODO_LIST item") {
				t.Fatalf("reason should name the item signal, got %q", refusal.Reason)
			}
		})
	}
}

// TestRedispatchCheckBatchPartialClosure: a batch candidate whose members
// are PARTIALLY closed is not refused — the batch still owes the open
// member's work.
func TestRedispatchCheckBatchPartialClosure(t *testing.T) {
	open := "open member"
	closed := "closed member"

	fixture := newDonePreflightFixture(t, "- [ ] "+open+"\n- [x] "+closed+"\n")

	err := fixture.h.RedispatchCheck(context.Background(), task.New{Type: "agent", Payload: []byte(
		`{"repo":"preflight","prompt":"work the run","dedup":"batch:abc","itemKeys":[` +
			jsonString(ItemKey("preflight", open)) + "," + jsonString(ItemKey("preflight", closed)) + `]}`,
	)})
	if err != nil {
		t.Fatalf("partially closed batch must not be refused, got %v", err)
	}
}

// TestRedispatchCheckCloseoutCited is the row-110 class: the prompt cites
// a prior task ID whose closeout report is already indexed — a
// re-verification dispatch of closed work.
func TestRedispatchCheckCloseoutCited(t *testing.T) {
	fixture := newDonePreflightFixture(t, "- [ ] open work\n")

	cited := strings.Repeat("1", 36)
	reportDir := filepath.Join(fixture.repo, "docs", "status", "tasks")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatal(err)
	}

	report := filepath.Join(reportDir, "2026-10-07_04-25_task-"+cited+".md")
	if err := os.WriteFile(report, []byte("# done\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := fixture.h.RedispatchCheck(
		context.Background(),
		candidateWith("", "", "re-verify task "+cited+" against HEAD", "", ""),
	)

	refusal, ok := errors.AsType[*RedispatchRefusal](err)
	if !ok {
		t.Fatalf("expected a redispatch refusal, got %v", err)
	}

	if !strings.Contains(refusal.Reason, cited[:8]) {
		t.Fatalf("reason should name the cited task, got %q", refusal.Reason)
	}
}

// TestRedispatchCheckOpenWorkAllowed: an open row, a live finding, no
// cited closeout — the mint proceeds (the gate refuses only on positive
// proof of done work, never by default).
func TestRedispatchCheckOpenWorkAllowed(t *testing.T) {
	itemText := "fresh open work"

	fixture := newDonePreflightFixture(t, "- [ ] "+itemText+"\n")
	gitCommit(t, fixture.repo, "unrelated history")

	cases := map[string]task.New{
		"open item": candidateWith(ItemKey("preflight", itemText), itemText, "do the thing", "", ""),
		"git sha in prompt": candidateWith("", "", "rebase onto "+strings.Repeat("a", 40), "", ""),
		"short sha in prompt": candidateWith("", "", "cured by abcdef01", "", ""),
		"foreign payload": {Type: "agent", Payload: []byte(`{"repo":"","prompt":""}`)},
	}

	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if err := fixture.h.RedispatchCheck(context.Background(), candidate); err != nil {
				t.Fatalf("open work must not be refused, got %v", err)
			}
		})
	}
}

// TestRedispatchForceEscape: Config.ForceRedispatch (the O4 escape hatch)
// bypasses the mint gate even when every signal would refuse.
func TestRedispatchForceEscape(t *testing.T) {
	itemText := "already closed row"

	fixture := newDonePreflightFixture(t, "- [x] "+itemText+"\n")
	fixture.h.cfg.ForceRedispatch = true

	err := fixture.h.refuseUnlessForced(
		context.Background(), candidateWith(ItemKey("preflight", itemText), itemText, "verify anyway", "", ""),
	)
	if err != nil {
		t.Fatalf("forced re-dispatch must pass the gate, got %v", err)
	}
}

// TestDonePreflightFixCuredByReword pins the claim-time half of the new
// cure branch: the shared fixTicketCured means DonePreflight completes the
// re-claimed ticket without an agent run too.
func TestDonePreflightFixCuredByReword(t *testing.T) {
	fixture := newDonePreflightFixture(t, "- [ ] open work\n")

	bad := gitCommit(t, fixture.repo, "the rejected commit")
	removeLine(t, fixture.repo, "commit-marker")

	gitCommit(t, fixture.repo, "reworded the finding away")

	done, reason := fixture.h.DonePreflight(
		context.Background(),
		taskWith(t, "000001a10e49405bef56b19e5ff500000000", "", "", bad, "the rejected commit"),
	)
	if !done {
		t.Fatal("SHA present + unreferenced + anchor absent must read cured at claim time")
	}

	if !strings.Contains(reason, "unreferenced and anchor text absent") {
		t.Fatalf("reason should name the reword cure, got %q", reason)
	}
}

// removeLine deletes one file line (helper for reword fixtures: the anchor
// text lived in commit-marker and is gone at HEAD).
func removeLine(t *testing.T, repo, file string) {
	t.Helper()

	if err := os.Remove(filepath.Join(repo, file)); err != nil {
		t.Fatal(err)
	}
}
