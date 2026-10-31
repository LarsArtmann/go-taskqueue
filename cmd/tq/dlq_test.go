package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func TestParseAgentPoolOptionsDLQFixFlag(t *testing.T) {
	t.Parallel()

	opts, err := parseAgentPoolOptions([]string{"--repos", "/tmp/somewhere", "--dlq-fix"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !opts.dlqFix {
		t.Fatal("--dlq-fix did not reach the options struct")
	}

	opts, err = parseAgentPoolOptions([]string{"--repos", "/tmp/somewhere"})
	if err != nil {
		t.Fatalf("parse default: %v", err)
	}

	if opts.dlqFix {
		t.Fatal("dlq-fix must default to off")
	}
}

// seedDeadTask enqueues, claims and fails one task so it lands Dead with
// the given failure detail (nil = no evidence JSON on the failed fact).
func seedDeadTask(t *testing.T, store *sqlite.Store, project string, failReason string, evidence executor.FailureEvidence) task.Task {
	t.Helper()

	ctx := context.Background()

	enq, err := store.Enqueue(ctx, task.New{
		Type:        executor.TaskTypeAgent,
		Project:     project,
		Payload:     []byte(`{"repo":"demo","prompt":"p"}`),
		MaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	_, claim, err := store.ClaimDue(ctx, "seed", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	var raw []byte

	if (evidence != executor.FailureEvidence{}) {
		raw, err = json.Marshal(evidence)
		if err != nil {
			t.Fatalf("marshal evidence: %v", err)
		}
	}

	if err := store.Fail(ctx, enq.ID, claim, failReason, 0, raw); err != nil {
		t.Fatalf("fail: %v", err)
	}

	got, err := store.Get(ctx, enq.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	return got
}

// TestCmdDLQJSONStream pins the --json contract (TODO rows 1416 + 1413):
// a lead summary object {count,total,evidence_read} then one row object
// per dead task — machine consumers count rows from `count`, never grep
// row text — and each row carries its failure evidence decoded.
func TestCmdDLQJSONStream(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "dlq-json.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	dead := seedDeadTask(t, store, "demo", "boom", executor.FailureEvidence{Stage: "command", ExitCode: 3})

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdDLQ([]string{"--db", dbPath, "--json"}); err != nil {
			t.Errorf("cmdDLQ --json: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected summary + 1 row, saw %d lines:\n%s", len(lines), out)
	}

	var summary dlqJSONSummary

	if err := json.Unmarshal([]byte(lines[0]), &summary); err != nil {
		t.Fatalf("summary line not JSON: %v\n%s", err, lines[0])
	}

	if summary.Count != 1 || summary.Total != 1 || !summary.EvidenceRead {
		t.Fatalf("summary = %+v, want count=1 total=1 evidence_read=true", summary)
	}

	var row dlqJSONRow

	if err := json.Unmarshal([]byte(lines[1]), &row); err != nil {
		t.Fatalf("row line not JSON: %v\n%s", err, lines[1])
	}

	if row.ID != dead.ID.String() || row.Project != "demo" || row.Attempts != 1 {
		t.Fatalf("row = %+v, want id=%s project=demo attempts=1", row, dead.ID)
	}

	if row.Evidence == nil || row.Evidence.Stage != "command" || row.Evidence.ExitCode != 3 {
		t.Fatalf("row evidence = %+v, want stage=command exit=3", row.Evidence)
	}
}

// TestCmdDLQEvidenceReadCap pins the bounded prefetch (TODO row 1413): an
// unfiltered listing over dlqEvidenceReadCap dead tasks skips the
// per-task trail reads (summary evidence_read=false, rows carry no
// evidence), while an explicit --evidence-stage filter still reads.
func TestCmdDLQEvidenceReadCap(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "dlq-cap.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	for range dlqEvidenceReadCap + 1 {
		seedDeadTask(t, store, "cap", "boom", executor.FailureEvidence{Stage: "command"})
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	runJSON := func(args ...string) (dlqJSONSummary, []dlqJSONRow) {
		t.Helper()

		out := captureStdout(t, func() {
			if err := cmdDLQ(append([]string{"--db", dbPath, "--json"}, args...)); err != nil {
				t.Errorf("cmdDLQ: %v", err)
			}
		})

		lines := strings.Split(strings.TrimSpace(out), "\n")

		var summary dlqJSONSummary

		if err := json.Unmarshal([]byte(lines[0]), &summary); err != nil {
			t.Fatalf("summary line not JSON: %v", err)
		}

		rows := make([]dlqJSONRow, 0, len(lines)-1)

		for _, line := range lines[1:] {
			var row dlqJSONRow

			if err := json.Unmarshal([]byte(line), &row); err != nil {
				t.Fatalf("row line not JSON: %v", err)
			}

			rows = append(rows, row)
		}

		return summary, rows
	}

	summary, rows := runJSON()
	if summary.Count != dlqEvidenceReadCap+1 || summary.Total != dlqEvidenceReadCap+1 {
		t.Fatalf("summary = %+v, want count=total=%d", summary, dlqEvidenceReadCap+1)
	}

	if summary.EvidenceRead {
		t.Fatal("over-cap unfiltered run must report evidence_read=false")
	}

	for _, row := range rows {
		if row.Evidence != nil {
			t.Fatalf("over-cap unfiltered row carried evidence: %+v", row.Evidence)
		}
	}

	summary, rows = runJSON("--evidence-stage", "command")
	if !summary.EvidenceRead {
		t.Fatal("filtered run must always read evidence")
	}

	if summary.Count != dlqEvidenceReadCap+1 || summary.Total != dlqEvidenceReadCap+1 {
		t.Fatalf("filtered summary = %+v, want count=total=%d", summary, dlqEvidenceReadCap+1)
	}

	for _, row := range rows {
		if row.Evidence == nil || row.Evidence.Stage != "command" {
			t.Fatalf("filtered row missing evidence: %+v", row)
		}
	}
}

// TestCmdDLQDismissDeadTask runs the operator lever end-to-end against a
// scratch store: a dead task becomes cancelled with the reason recorded on
// the cancelled fact.
func TestCmdDLQDismissDeadTask(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "dlq.db")

	seed, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open seed store: %v", err)
	}

	ctx := context.Background()

	enq, err := seed.Enqueue(ctx, task.New{
		Type:        executor.TaskTypeAgent,
		Project:     "demo",
		Payload:     []byte(`{"repo":"demo","prompt":"p"}`),
		MaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, claimW, err := seed.ClaimDue(ctx, "w", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	} else if err := seed.Fail(ctx, enq.ID, claimW, "boom", 0, nil); err != nil {
		t.Fatalf("fail: %v", err)
	}

	if got, _ := seed.Get(ctx, enq.ID); got.Status != task.Dead {
		t.Fatalf("seed status = %s, want dead", got.Status)
	}

	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	cmdOut := captureStdout(t, func() {
		if err := cmdDLQ(
			[]string{"--db", dbPath, "--dismiss", enq.ID.String(), "--reason", "autopsy ruled it unfixable"},
		); err != nil {
			t.Errorf("cmdDLQ: %v", err)
		}
	})

	if !strings.Contains(cmdOut, "dismissed "+enq.ID.String()) {
		t.Fatalf("output %q missing dismissal line", cmdOut)
	}

	verify, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	defer verify.Close()

	got, err := verify.Get(ctx, enq.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != task.Cancelled {
		t.Fatalf("status after dismiss = %s, want cancelled", got.Status)
	}
}

// TestCmdDLQCrossLinksPoisonedBackoff pins the DLQ-side surface: with
// the backoff window resolvable, rows whose repo is currently in
// poisoned-repo backoff carry the same poisoned line pool-health warns
// with — the shared fold's off-ramps (flag off, a completion healing
// the repo) stay silent (closeout 2026-10-28 04-18 §f6).
func TestCmdDLQCrossLinksPoisonedBackoff(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	deadIn := func(t *testing.T, dbPath, project string) task.ID {
		t.Helper()

		store, err := sqlite.Open(dbPath)
		if err != nil {
			t.Fatalf("open seed store: %v", err)
		}

		defer store.Close()

		enq, err := store.Enqueue(ctx, task.New{Project: project, Type: "sh", Payload: []byte(`"true"`), MaxAttempts: 1})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		_, claim, err := store.ClaimDue(ctx, "w", time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		if err := store.FailPermanent(ctx, enq.ID, claim, "boom", nil); err != nil {
			t.Fatalf("fail permanent: %v", err)
		}

		return enq.ID
	}

	t.Run("active backoff cross-links the row", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "dlq.db")
		enqID := deadIn(t, dbPath, "poisoned")

		out := captureStdout(t, func() {
			if err := cmdDLQ([]string{"--db", dbPath, "--dlq-backoff", "1h"}); err != nil {
				t.Errorf("cmdDLQ: %v", err)
			}
		})

		for _, want := range []string{
			enqID.String(),
			"poisoned: repo in DLQ backoff until ",
			"(fix the repo or rescue dead tasks)",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("output %q lacks %q", out, want)
			}
		}
	})

	t.Run("a completion heals the repo despite the dead row", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "dlq.db")

		store, err := sqlite.Open(dbPath)
		if err != nil {
			t.Fatalf("open seed store: %v", err)
		}

		done, err := store.Enqueue(ctx, task.New{Project: "mixed", Type: "sh", Payload: []byte(`"true"`), MaxAttempts: 1})
		if err != nil {
			t.Fatalf("enqueue completed: %v", err)
		}

		_, claim, err := store.ClaimDue(ctx, "w", time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		if err := store.Complete(ctx, done.ID, claim, nil); err != nil {
			t.Fatalf("complete: %v", err)
		}

		if err := store.Close(); err != nil {
			t.Fatal(err)
		}

		deadIn(t, dbPath, "mixed")

		out := captureStdout(t, func() {
			if err := cmdDLQ([]string{"--db", dbPath, "--dlq-backoff", "1h"}); err != nil {
				t.Errorf("cmdDLQ: %v", err)
			}
		})

		if strings.Contains(out, "poisoned:") {
			t.Fatalf("output %q cross-links backoff despite the repo's completion", out)
		}

		if !strings.Contains(out, "attempts=1/1") {
			t.Fatalf("output %q lacks the dead row itself", out)
		}
	})

	t.Run("flag off stays silent", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "dlq.db")
		deadIn(t, dbPath, "quiet")

		out := captureStdout(t, func() {
			if err := cmdDLQ([]string{"--db", dbPath}); err != nil {
				t.Errorf("cmdDLQ: %v", err)
			}
		})

		if strings.Contains(out, "poisoned:") {
			t.Fatalf("output %q cross-links backoff with the flag off", out)
		}
	})
}

// TestCmdDLQRowsCarryEvidenceSuffix pins the self-describing DLQ listing:
// rows whose last task.failed fact carries FailureEvidence render the
// evidence inline ("evidence=verify/e2e-timeout exit=…", the same Summary
// shape the webui detail line uses) whether or not --evidence-stage
// filtered the list (2026-10-26 01-40 §e3).
func TestCmdDLQRowsCarryEvidenceSuffix(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "dlq-suffix.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	ctx := context.Background()

	seedDead := func(failReason string, evidence any) task.ID {
		t.Helper()

		var raw []byte

		if evidence != nil {
			raw, err = json.Marshal(evidence)
			if err != nil {
				t.Fatalf("marshal evidence: %v", err)
			}
		}

		enq, err := store.Enqueue(ctx, task.New{
			Type:        executor.TaskTypeAgent,
			Project:     "demo",
			Payload:     []byte(`{"repo":"demo","prompt":"p"}`),
			MaxAttempts: 1,
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		_, claim, err := store.ClaimDue(ctx, "suffix-test", time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		if err := store.Fail(ctx, enq.ID, claim, failReason, 0, raw); err != nil {
			t.Fatalf("fail: %v", err)
		}

		return enq.ID
	}

	withEvidence := seedDead("verify failed", executor.FailureEvidence{
		Stage:       "verify",
		VerifyStage: executor.VerifyStageE2ETimeout,
		ExitCode:    2,
	})
	withoutEvidence := seedDead("boom", nil)

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdDLQ([]string{"--db", dbPath}); err != nil {
			t.Errorf("cmdDLQ: %v", err)
		}
	})

	if !strings.Contains(out, withEvidence.String()) ||
		!strings.Contains(out, "evidence=verify/"+string(executor.VerifyStageE2ETimeout)+" exit=2") {
		t.Fatalf("row with evidence lacks the inline suffix:\n%s", out)
	}

	rowStart := strings.Index(out, withoutEvidence.String())
	row := out[rowStart : rowStart+strings.Index(out[rowStart:], "\n")]

	if strings.Contains(row, "evidence=") {
		t.Fatalf("evidence-less row carries a suffix: %q", row)
	}
}
