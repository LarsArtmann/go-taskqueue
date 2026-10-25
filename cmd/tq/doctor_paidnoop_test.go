package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// completePaid enqueues one task of typ, claims it, and completes it with
// detail as the completion-fact result (empty string = nil detail, the
// legacy empty-completion shape).
func completePaid(t *testing.T, s *sqlite.Store, typ, detail string) {
	t.Helper()

	ctx := context.Background()

	if _, err := s.Enqueue(ctx, task.New{Type: typ}); err != nil {
		t.Fatalf("enqueue %s: %v", typ, err)
	}

	claimed, claim, err := s.ClaimDue(ctx, "doctor-noop", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	var result []byte
	if detail != "" {
		result = []byte(detail)
	}

	if err := s.Complete(ctx, claimed.ID, claim, result); err != nil {
		t.Fatalf("complete %s: %v", typ, err)
	}
}

func TestDoctorPaidNoopsHealthyWhenOutputAttributed(t *testing.T) {
	path := doctorTestStore(t)

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	completePaid(t, s, executor.TaskTypeAgent, `{"commits":[{"sha":"abc1234","subject":"fix: thing"}]}`)
	completePaid(t, s, executor.TaskTypeAgent, `{"preflight_done":"item already ticked"}`)
	completePaid(t, s, executor.TaskTypeReview, `{"verdict":"approve"}`)
	completePaid(t, s, executor.TaskTypeStatus, `{"report":"docs/status/x.md","next_items":2}`)
	completePaid(t, s, executor.TaskTypePrioritize, `{"verdicts":[{"item_key":"a","score":50}]}`)
	completePaid(t, s, executor.TaskTypeDLQFix, `{"verdict":"wontfix","summary":"root cause is outside the repo"}`)

	results := doctorPaidNoops(context.Background(), s)
	if len(results) != 1 {
		t.Fatalf("want 1 paid-noops result, got %d: %+v", len(results), results)
	}
	if results[0].Status != checkOK {
		t.Fatalf("want ok, got %s (%s)", results[0].Status, results[0].Detail)
	}
}

func TestDoctorPaidNoopsFlagsZeroOutputCompletions(t *testing.T) {
	path := doctorTestStore(t)

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	// The twins class: completed with detail that attributes nothing, and
	// the legacy shape with no detail at all.
	completePaid(t, s, executor.TaskTypeAgent, `{"verify_tail":"all green"}`)
	completePaid(t, s, executor.TaskTypeAgent, "")
	completePaid(t, s, executor.TaskTypeReview, `{}`)

	results := doctorPaidNoops(context.Background(), s)
	if len(results) != 1 {
		t.Fatalf("want 1 paid-noops result, got %d: %+v", len(results), results)
	}

	r := results[0]
	if r.Status != checkWarn {
		t.Fatalf("want warn, got %s (%s)", r.Status, r.Detail)
	}
	if len(r.Items) != 3 {
		t.Fatalf("want 3 flagged items, got %d: %+v", len(r.Items), r.Items)
	}
	for _, item := range r.Items {
		if !strings.Contains(item, "no completion detail") &&
			!strings.Contains(item, "no commits, no files, no preflight-done") &&
			!strings.Contains(item, "no verdict") {
			t.Fatalf("unexpected item reason: %q", item)
		}
	}
}

func TestDoctorPaidNoopsIgnoresUnpaidTypes(t *testing.T) {
	path := doctorTestStore(t)

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	completePaid(t, s, "sh", "")

	results := doctorPaidNoops(context.Background(), s)
	if len(results) != 1 || results[0].Status != checkOK {
		t.Fatalf("unpaid zero-output completion must not surface: %+v", results)
	}
}

func TestRunGateBoardReportsRedAndGreen(t *testing.T) {
	dir := t.TempDir()

	pass := filepath.Join(dir, "check-pass.sh")
	if err := os.WriteFile(pass, []byte("#!/usr/bin/env bash\necho 'pass summary: 3 checks ok, 0 failed'\n"), 0o755); err != nil {
		t.Fatalf("write pass: %v", err)
	}
	fail := filepath.Join(dir, "check-fail.sh")
	if err := os.WriteFile(fail, []byte("#!/usr/bin/env bash\necho 'FAIL: something broke' >&2\necho 'fail summary: 2 ok, 1 failed'\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write fail: %v", err)
	}

	r := runGateBoard(context.Background(), dir, []string{"check-pass.sh", "check-fail.sh"})
	if r.Name != "gate-board" {
		t.Fatalf("want gate-board name, got %q", r.Name)
	}
	if r.Status != checkWarn {
		t.Fatalf("want warn (one red), got %s (%s)", r.Status, r.Detail)
	}
	if len(r.Items) != 2 {
		t.Fatalf("want 2 rows, got %+v", r.Items)
	}
	if !strings.Contains(r.Items[0], "GREEN") || !strings.Contains(r.Items[0], "pass summary") {
		t.Fatalf("passing gate row wrong: %q", r.Items[0])
	}
	if !strings.Contains(r.Items[1], "RED") || !strings.Contains(r.Items[1], "fail summary") {
		t.Fatalf("failing gate row wrong: %q", r.Items[1])
	}
}

func TestRunGateBoardWarnsOutsideCheckout(t *testing.T) {
	r := runGateBoard(context.Background(), filepath.Join(t.TempDir(), "absent"), []string{"check-x.sh"})
	if r.Status != checkWarn || !strings.Contains(r.Detail, "not found") {
		t.Fatalf("missing scripts dir must WARN, got %s (%s)", r.Status, r.Detail)
	}
}
