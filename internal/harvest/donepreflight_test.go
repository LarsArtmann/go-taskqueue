package harvest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// donePreflightFixture is one repo + task under test: a git-initialized
// repo (footer/SHA signals need real git) with a todo file.
type donePreflightFixture struct {
	repo string
	h    *Harvester
}

// newDonePreflightFixture builds the repo, commits the todo file, and
// returns a Harvester over the projects dir. Commits made by the helper
// carry NO task footer, so the footer signal starts empty.
func newDonePreflightFixture(t *testing.T, todo string) donePreflightFixture {
	t.Helper()

	projects := t.TempDir()
	repo := writeRepo(t, projects, "preflight", todo)

	gitInit(t, repo)
	gitCommit(t, repo, "seed todo")

	return donePreflightFixture{
		repo: repo,
		h:    New(nil, Config{ProjectsDir: projects}),
	}
}

// taskWith builds a synthetic claimed task carrying the payload fields the
// gate reads.
func taskWith(t *testing.T, id task.ID, dedup, item, rejectedSHA, anchor string) task.Task {
	t.Helper()

	payload := `{"repo":"preflight","prompt":"do the thing","dedup":` +
		jsonString(dedup) + `,"item":` + jsonString(item)
	if rejectedSHA != "" || anchor != "" {
		payload += `,"rejected_sha":` + jsonString(rejectedSHA) +
			`,"anchor":` + jsonString(anchor)
	}

	payload += `}`

	return task.Task{ID: id, Type: "agent", Payload: []byte(payload)}
}

func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func gitInit(t *testing.T, repo string) {
	t.Helper()

	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitCommit(t *testing.T, repo, msg string) string {
	t.Helper()

	if err := os.WriteFile(filepath.Join(repo, "commit-marker"), []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "--quiet", "-m", msg},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}

	return strings.TrimSpace(string(out))
}

// TestDonePreflightFooterCommit is signal 1: a commit under the task's own
// Task-Queue-ID footer (the 2026-10-02 class — 21 footer commits, 15
// claims, every claim a paid no-op) marks the task done even with the todo
// item still open.
func TestDonePreflightFooterCommit(t *testing.T) {
	f := newDonePreflightFixture(t, "- [ ] still open item\n")
	id := task.NewID()
	gitCommit(t, f.repo, "land the work\n\nTask-Queue-ID: "+id.String())

	done, reason := f.h.DonePreflight(context.Background(), taskWith(t, id, "", "still open item", "", ""))
	if !done {
		t.Fatal("footer commit exists: want done, got not done")
	}

	if !strings.Contains(reason, "Task-Queue-ID footer") {
		t.Errorf("reason = %q, want the footer signal named", reason)
	}
}

// TestDonePreflightItemTicked is signal 3's ticked rule: the item's dedup
// key now parses as `[x]`.
func TestDonePreflightItemTicked(t *testing.T) {
	f := newDonePreflightFixture(t, "- [x] closed item\n")
	id := task.NewID()
	key := ItemKey("preflight", "closed item")

	done, reason := f.h.DonePreflight(
		context.Background(), taskWith(t, id, key, "closed item", "", ""),
	)
	if !done {
		t.Fatal("item ticked: want done, got not done")
	}

	if !strings.Contains(reason, "[x]") {
		t.Errorf("reason = %q, want the ticked signal named", reason)
	}
}

// TestDonePreflightItemAbsent is signal 3's absent rule: the item text is
// gone from the file (completed-and-deleted convention).
func TestDonePreflightItemAbsent(t *testing.T) {
	f := newDonePreflightFixture(t, "- [ ] unrelated open item\n")
	id := task.NewID()
	key := ItemKey("preflight", "withdrawn item")

	done, reason := f.h.DonePreflight(
		context.Background(), taskWith(t, id, key, "withdrawn item", "", ""),
	)
	if !done {
		t.Fatal("item absent: want done, got not done")
	}

	if !strings.Contains(reason, "no longer present") {
		t.Errorf("reason = %q, want the absent signal named", reason)
	}
}

// TestDonePreflightOpenItemNotDone is the negative control: an open item,
// no footer commits, no fix fields, no report — the gate must NOT mark
// live work done.
func TestDonePreflightOpenItemNotDone(t *testing.T) {
	f := newDonePreflightFixture(t, "- [ ] live item\n")
	id := task.NewID()

	done, _ := f.h.DonePreflight(
		context.Background(), taskWith(t, id, ItemKey("preflight", "live item"), "live item", "", ""),
	)
	if done {
		t.Fatal("open live item: want not done, got done (the gate would skip real work)")
	}
}

// TestDonePreflightFixSuperseded is signal 2's supersede rule: a fix
// ticket whose rejected SHA is cited by a later commit (the superseding
// fix landed) is done.
func TestDonePreflightFixSuperseded(t *testing.T) {
	f := newDonePreflightFixture(t, "- [ ] host item\n")
	rejected := gitCommit(t, f.repo, "the rejected change")
	id := task.NewID()

	gitCommit(t, f.repo, "supersede: rework of "+rejected)

	done, reason := f.h.DonePreflight(
		context.Background(), taskWith(t, id, "", "", rejected, "anchor text"),
	)
	if !done {
		t.Fatal("rejected SHA superseded: want done, got not done")
	}

	if !strings.Contains(reason, "superseding commit") {
		t.Errorf("reason = %q, want the supersede signal named", reason)
	}
}

// TestDonePreflightFixCuredByRebase is signal 2's anchor-gone rule: the
// rejected SHA was rebased away AND the anchor text no longer exists in
// the working tree — the finding is cured.
func TestDonePreflightFixCuredByRebase(t *testing.T) {
	f := newDonePreflightFixture(t, "- [ ] host item\n")
	rejected := gitCommit(t, f.repo, "the rejected change")
	id := task.NewID()

	// Drop the rejected commit from every ref AND from the object store
	// (reflog + gc), so cat-file -e genuinely fails — a mere reset leaves
	// the dangling object resolvable and the gate would see it as present.
	for _, args := range [][]string{
		{"reset", "--quiet", "--hard", "HEAD~1"},
		{"reflog", "expire", "--expire=now", "--all"},
		{"gc", "--prune=now", "--quiet"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", f.repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	done, reason := f.h.DonePreflight(
		context.Background(), taskWith(t, id, "", "", rejected, "anchor that exists nowhere"),
	)
	if !done {
		t.Fatal("rejected SHA gone + anchor absent: want done, got not done")
	}

	if !strings.Contains(reason, "anchor text absent") {
		t.Errorf("reason = %q, want the anchor-gone signal named", reason)
	}
}

// TestDonePreflightFixStillLive is signal 2's negative control: the
// rejected SHA exists, nothing supersedes it, the anchor text is still in
// the tree — the finding is live, the fix task must run.
func TestDonePreflightFixStillLive(t *testing.T) {
	f := newDonePreflightFixture(t, "- [ ] host item\n")
	rejected := gitCommit(t, f.repo, "the rejected change\n\ncontains ANCHOR-XYZ in its message")
	id := task.NewID()

	if err := os.WriteFile(
		filepath.Join(f.repo, "code.txt"), []byte("ANCHOR-XYZ lives here\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	gitCommit(t, f.repo, "unrelated commit")

	done, _ := f.h.DonePreflight(
		context.Background(), taskWith(t, id, "", "", rejected, "ANCHOR-XYZ lives here"),
	)
	if done {
		t.Fatal("live finding (sha present, anchor live, no supersede): want not done")
	}
}

// TestDonePreflightReportExists is signal 4: the closeout report
// convention (docs/status/*_task-<id>*).
func TestDonePreflightReportExists(t *testing.T) {
	f := newDonePreflightFixture(t, "- [ ] still open item\n")
	id := task.NewID()

	if err := os.MkdirAll(filepath.Join(f.repo, "docs", "status"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(f.repo, "docs", "status", "2026-10-06_19-00_task-"+id.String()+".md"),
		[]byte("report"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	done, reason := f.h.DonePreflight(
		context.Background(), taskWith(t, id, ItemKey("preflight", "still open item"), "still open item", "", ""),
	)
	if !done {
		t.Fatal("closeout report exists: want done, got not done")
	}

	if !strings.Contains(reason, "closeout report") {
		t.Errorf("reason = %q, want the report signal named", reason)
	}
}

// TestDonePreflightReportExistsInTasksDir is signal 4 at the O7
// report-placement location: docs/status/tasks/*_task-<id>*.
func TestDonePreflightReportExistsInTasksDir(t *testing.T) {
	f := newDonePreflightFixture(t, "- [ ] still open item\n")
	id := task.NewID()

	if err := os.MkdirAll(filepath.Join(f.repo, "docs", "status", "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(f.repo, "docs", "status", "tasks", "2026-10-07_10-00_task-"+id.String()+".md"),
		[]byte("report"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	done, reason := f.h.DonePreflight(
		context.Background(), taskWith(t, id, ItemKey("preflight", "still open item"), "still open item", "", ""),
	)
	if !done {
		t.Fatal("closeout report in tasks/: want done, got not done")
	}

	if !strings.Contains(reason, "closeout report") {
		t.Errorf("reason = %q, want the report signal named", reason)
	}
}

// TestDonePreflightForeignPayloadNotDone pins the fail-open contract:
// payloads without the minimum identity (Repo/Prompt) are invisible to the
// gate, as are tasks whose repo cannot be resolved.
func TestDonePreflightForeignPayloadNotDone(t *testing.T) {
	f := newDonePreflightFixture(t, "- [ ] item\n")

	// No payload at all.
	if done, _ := f.h.DonePreflight(context.Background(), task.Task{ID: task.NewID()}); done {
		t.Fatal("empty payload: want not done")
	}

	// External shape: repo + prompt present, but the repo does not exist
	// under the projects dir.
	external := task.Task{
		ID:   task.NewID(),
		Type: "agent",
		Payload: []byte(
			`{"repo":"no-such-repo","prompt":"x"}`,
		),
	}
	if done, _ := f.h.DonePreflight(context.Background(), external); done {
		t.Fatal("unresolvable repo: want not done (gate never guesses)")
	}
}

// TestDonePreflightCatchesLandingAfterMint pins the row-34 timeline
// (C13.1/C13.3): a task legitimately minted while its item was OPEN is
// caught at claim time when the landing happens between mint and claim —
// the re-dispatch completes with zero agent spend instead of a paid no-op.
// Both landing shapes count: the ticked row, and the footer commit alone
// (the tick raced in a later commit).
func TestDonePreflightCatchesLandingAfterMint(t *testing.T) {
	for _, tc := range []struct {
		name string
		land func(t *testing.T, f donePreflightFixture, id task.ID)
	}{
		{
			name: "ticked row",
			land: func(t *testing.T, f donePreflightFixture, _ task.ID) {
				t.Helper()

				if err := os.WriteFile(
					filepath.Join(f.repo, DefaultTodoFile), []byte("- [x] timeline item\n"), 0o644,
				); err != nil {
					t.Fatal(err)
				}

				gitCommit(t, f.repo, "work: land and tick")
			},
		},
		{
			name: "footer commit only",
			land: func(t *testing.T, f donePreflightFixture, id task.ID) {
				t.Helper()
				gitCommit(t, f.repo, "work: landed\n\nTask-Queue-ID: "+id.String())
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Mint phase: the item is open, the task is legitimately enqueued.
			f := newDonePreflightFixture(t, "- [ ] timeline item\n")
			id := task.NewID()

			if done, reason := f.h.DonePreflight(
				context.Background(), taskWith(t, id, ItemKey("preflight", "timeline item"), "timeline item", "", ""),
			); done {
				t.Fatalf("pre-landing claim reported done (%q): the mint was legitimate", reason)
			}

			// Landing phase: the work lands between mint and claim.
			tc.land(t, f, id)

			// Claim phase: the gate short-circuits the paid re-run.
			done, reason := f.h.DonePreflight(
				context.Background(), taskWith(t, id, ItemKey("preflight", "timeline item"), "timeline item", "", ""),
			)
			if !done {
				t.Fatal("post-landing claim not caught: the re-dispatch would burn a paid window")
			}

			if !strings.Contains(reason, "done preflight:") {
				t.Errorf("reason = %q, want a named preflight signal", reason)
			}
		})
	}
}

// batchTaskWith builds a batch task whose payload carries the member keys
// the gate judges staleness by.
func batchTaskWith(id task.ID, keys ...string) task.Task {
	quoted := make([]string, 0, len(keys))
	for _, k := range keys {
		quoted = append(quoted, jsonString(k))
	}

	payload := `{"repo":"preflight","prompt":"do the thing","itemKeys":[` +
		strings.Join(quoted, ",") + `]}`

	return task.Task{ID: id, Type: "agent", Payload: []byte(payload)}
}

// TestDonePreflightBatchFooterRequiresAllMembersLanded pins the
// 000001a1382a1765 reclaim class: a footer commit over a MULTI-member
// batch with open members is NOT a done verdict — the reclaiming claim
// falls through and the agent re-runs over the unlanded keys instead of
// completing the task zero-cost with 1/3 items landed.
func TestDonePreflightBatchFooterRequiresAllMembersLanded(t *testing.T) {
	for _, tc := range []struct {
		name string
		todo string
		want bool
	}{
		{
			name: "partial members open (the reclaim incident)",
			todo: "- [x] batch alpha\n- [ ] batch beta\n- [ ] batch gamma\n",
			want: false,
		},
		{
			name: "every member ticked",
			todo: "- [x] batch alpha\n- [x] batch beta\n- [x] batch gamma\n",
			want: true,
		},
		{
			name: "every member gone",
			todo: "- [ ] unrelated open item\n",
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDonePreflightFixture(t, tc.todo)
			id := task.NewID()
			gitCommit(t, f.repo, "work: land member alpha\n\nTask-Queue-ID: "+id.String())

			keys := []string{
				ItemKey("preflight", "batch alpha"),
				ItemKey("preflight", "batch beta"),
				ItemKey("preflight", "batch gamma"),
			}

			done, reason := f.h.DonePreflight(context.Background(), batchTaskWith(id, keys...))
			if done != tc.want {
				t.Fatalf("done = %v (%q), want %v", done, reason, tc.want)
			}

			if tc.want && !strings.Contains(reason, "Task-Queue-ID footer") {
				t.Errorf("reason = %q, want the footer signal named", reason)
			}
		})
	}
}
