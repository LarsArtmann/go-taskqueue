package session

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func testStore(t *testing.T) *sqlite.Store {
	t.Helper()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

func stubScanner(commits []Commit) GitScanner {
	return GitScannerFunc(func(context.Context, string, string, string) ([]Commit, error) {
		return commits, nil
	})
}

var closeInput = CloseInput{
	ID:      "sess-abc",
	Repo:    "/repos/demo",
	Project: "demo",
	Summary: "Added the frobnicator",
}

func TestBeginRecordsOpenedFactAndRefusesDoubleBegin(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	if err := Begin(ctx, s, "sess-abc", "/repos/demo", "demo"); err != nil {
		t.Fatalf("begin: %v", err)
	}

	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(facts) != 1 {
		t.Fatalf("facts = %d, want exactly the opened fact", len(facts))
	}

	f := facts[0]
	if f.Type != journal.SessionOpened || f.TaskID != "session:sess-abc" {
		t.Fatalf("fact = %s/%s, want session.opened on session:sess-abc", f.Type, f.TaskID)
	}

	var detail OpenDetail
	if err := json.Unmarshal(f.Detail, &detail); err != nil {
		t.Fatalf("opened detail: %v", err)
	}

	if detail.SessionID != "sess-abc" || detail.Repo != "/repos/demo" || detail.Project != "demo" {
		t.Fatalf("opened detail = %+v", detail)
	}

	if _, err := Begin(ctx, s, "sess-abc", "/repos/demo", "demo"); err == nil || !strings.Contains(err.Error(), "already open") {
		t.Fatalf("double begin err = %v, want already-open refusal", err)
	}
}

func TestBeginNeedsID(t *testing.T) {
	if err := Begin(context.Background(), testStore(t), "", "/repos/demo", "demo"); err == nil {
		t.Fatal("empty id accepted")
	}
}

func TestCloseMintsReviewAndStatusOverAttributedRange(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	commits := []Commit{
		{SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Subject: "first"},
		{SHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Subject: "second"},
	}

	res, err := Close(ctx, s, stubScanner(commits), closeInput)
	if err != nil {
		t.Fatalf("close: %v", err)
	}

	if !res.ReviewFresh || !res.StatusFresh {
		t.Fatalf("first close should mint fresh, got review=%v status=%v", res.ReviewFresh, res.StatusFresh)
	}

	reviewTask, err := s.Get(ctx, res.ReviewTask.ID)
	if err != nil {
		t.Fatal(err)
	}

	if reviewTask.Type != executor.TaskTypeReview || reviewTask.Project != "demo" {
		t.Fatalf("review task = %s/%s", reviewTask.Type, reviewTask.Project)
	}

	var rp executor.ReviewPayload
	if err := json.Unmarshal(reviewTask.Payload, &rp); err != nil {
		t.Fatal(err)
	}

	if rp.Repo != "/repos/demo" || rp.ReviewedTask != "session:sess-abc" {
		t.Fatalf("review payload repo/reviewed = %q/%q", rp.Repo, rp.ReviewedTask)
	}

	if rp.Item != "Added the frobnicator" {
		t.Fatalf("review item = %q, want the operator summary as the bar", rp.Item)
	}

	if rp.CommitSHA != commits[1].SHA {
		t.Fatalf("review commit = %q, want range head", rp.CommitSHA)
	}

	if !rp.Yolo {
		t.Fatal("review must run autonomously (yolo)")
	}

	if rp.RequireClean == nil || !*rp.RequireClean {
		t.Fatal("default stance is require-clean")
	}

	for _, sha := range []string{commits[0].SHA, commits[1].SHA, "^.."} {
		if !strings.Contains(rp.Extra, sha) {
			t.Fatalf("review extra missing %q:\n%s", sha, rp.Extra)
		}
	}

	statusTask, err := s.Get(ctx, res.StatusTask.ID)
	if err != nil {
		t.Fatal(err)
	}

	if statusTask.Type != executor.TaskTypeStatus || statusTask.Project != "demo" {
		t.Fatalf("status task = %s/%s", statusTask.Type, statusTask.Project)
	}

	var sp executor.StatusPayload
	if err := json.Unmarshal(statusTask.Payload, &sp); err != nil {
		t.Fatal(err)
	}

	if sp.Repo != "/repos/demo" || sp.Project != "demo" || len(sp.Completed) != 1 {
		t.Fatalf("status payload = %+v", sp)
	}

	entry := sp.Completed[0]
	if entry.TaskID != "session:sess-abc" || entry.Commit != commits[1].SHA || entry.CompletedAt == "" {
		t.Fatalf("status completion = %+v", entry)
	}

	// The closed fact carries the whole lineage.
	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var closed *journal.Fact
	for i := range facts {
		if facts[i].Type == journal.SessionClosed {
			closed = &facts[i]
		}
	}

	if closed == nil {
		t.Fatal("no session.closed fact")
	}

	var cd CloseDetail
	if err := json.Unmarshal(closed.Detail, &cd); err != nil {
		t.Fatal(err)
	}

	if cd.ReviewTask != res.ReviewTask.ID.String() || cd.StatusTask != res.StatusTask.ID.String() {
		t.Fatalf("closed detail lineage = %q/%q", cd.ReviewTask, cd.StatusTask)
	}

	if len(cd.Commits) != 2 || cd.Commits[0].SHA != commits[0].SHA {
		t.Fatalf("closed detail commits = %+v, want oldest first", cd.Commits)
	}
}

func TestCloseDedupHoldsOnReplay(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	commits := []Commit{{SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Subject: "only"}}

	first, err := Close(ctx, s, stubScanner(commits), closeInput)
	if err != nil {
		t.Fatal(err)
	}

	second, err := Close(ctx, s, stubScanner(commits), closeInput)
	if err != nil {
		t.Fatalf("re-close: %v", err)
	}

	if second.ReviewFresh || second.StatusFresh {
		t.Fatal("re-close minted duplicate tasks")
	}

	if second.ReviewTask.ID != first.ReviewTask.ID || second.StatusTask.ID != first.StatusTask.ID {
		t.Fatal("re-close returned different tasks")
	}

	reviews, err := s.List(ctx, queue.Filter{Type: ptr(executor.TaskTypeReview)})
	if err != nil {
		t.Fatal(err)
	}

	statuses, err := s.List(ctx, queue.Filter{Type: ptr(executor.TaskTypeStatus)})
	if err != nil {
		t.Fatal(err)
	}

	if len(reviews) != 1 || len(statuses) != 1 {
		t.Fatalf("dedup leaked: %d reviews, %d statuses", len(reviews), len(statuses))
	}
}

func TestCloseAllowDirtyMirrorsRequireClean(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	commits := []Commit{{SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Subject: "only"}}

	dirty := closeInput
	dirty.AllowDirty = true

	res, err := Close(ctx, s, stubScanner(commits), dirty)
	if err != nil {
		t.Fatal(err)
	}

	reviewTask, err := s.Get(ctx, res.ReviewTask.ID)
	if err != nil {
		t.Fatal(err)
	}

	var rp executor.ReviewPayload
	if err := json.Unmarshal(reviewTask.Payload, &rp); err != nil {
		t.Fatal(err)
	}

	if rp.RequireClean == nil || *rp.RequireClean {
		t.Fatal("--allow-dirty must mint RequireClean=false payloads")
	}
}

func TestCloseWithoutAttributedCommitsMintsNothing(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	res, err := Close(ctx, s, stubScanner(nil), closeInput)
	if err != nil {
		t.Fatalf("close: %v", err)
	}

	if len(res.Commits) != 0 || res.ReviewTask.ID != "" || res.StatusTask.ID != "" {
		t.Fatalf("empty session minted work: %+v", res)
	}

	tasks, err := s.List(ctx, queue.Filter{})
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 0 {
		t.Fatalf("empty session enqueued %d tasks", len(tasks))
	}

	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(facts) != 1 || facts[0].Type != journal.SessionClosed {
		t.Fatalf("closed fact missing: %+v", facts)
	}
}

func TestCloseNeedsIDAndRepo(t *testing.T) {
	ctx := context.Background()

	if _, err := Close(ctx, testStore(t), stubScanner(nil), CloseInput{Repo: "/r"}); err == nil {
		t.Fatal("empty id accepted")
	}

	if _, err := Close(ctx, testStore(t), stubScanner(nil), CloseInput{ID: "x"}); err == nil {
		t.Fatal("empty repo accepted")
	}
}

func TestSyntheticTaskIDNamespaced(t *testing.T) {
	if got := SyntheticTaskID("abc").String(); got != "session:abc" {
		t.Fatalf("synthetic id = %q", got)
	}
}

func TestReviewItemDefaultsWhenSummaryEmpty(t *testing.T) {
	in := closeInput
	in.Summary = ""

	if item := reviewItem(in); item == "" {
		t.Fatal("empty item would be a permanent executor failure")
	}

	in.Summary = "  spaced  \n second line"

	if got := reviewItem(in); got != "spaced  \n second line" {
		t.Fatalf("summary not trimmed: %q", got)
	}
}

//go:fix inline
func ptr[v any](val v) *v {
	return new(val)
}
