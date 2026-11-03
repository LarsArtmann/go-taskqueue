package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// The receipt surfaces (`tq show`, `tq dlq`) exist because an artifact nobody
// renders is worth nothing: the whole point of recording which gate ran, at
// which HEAD, with which ok/fail tallies is that an operator can read it
// without SSH-ing into logs or trusting a report's hand-grep.
//
// See internal/executor/receipt.go for the types and docs/research/
// 2026-11-01_okf-lessons.md lesson 10 for why they exist.

// TestVerifyReceiptViewDecodesExecutorSpelling pins the cmd/tq twin against the
// REAL executor struct: marshal a receipt + verdict with the executor types,
// decode it through the local view, and require the same values. This is the
// drift alarm for the ADR-0017 tag lag — the day the spelling changes, this
// fails instead of silently rendering an empty section.
func TestVerifyReceiptViewDecodesExecutorSpelling(t *testing.T) {
	t.Parallel()

	detail, err := json.Marshal(executor.AgentResult{
		Verify: &executor.VerifyReceipt{
			Command: "go build ./... && go vet ./...",
			RC:      0,
			Head:    "0123456789abcdef0123456789abcdef01234567",
			OK:      6,
			Fail:    1,
		},
		Attestation: &executor.VerifyVerdict{
			OK:      false,
			Refusal: executor.AttestationCountMismatch,
			Detail:  "receipt says 6 ok, output says 5 ok",
			Checked: []string{executor.AttestationCheckReceipt, executor.AttestationCheckCounts},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	view := buildVerifyReceiptView([]journal.Fact{{Type: journal.Completed, Detail: detail}})
	if view == nil {
		t.Fatal("twin decoded no receipt")
	}

	if view.Command != "go build ./... && go vet ./..." || view.RC != 0 ||
		view.Head != "0123456789abcdef0123456789abcdef01234567" ||
		view.OK != 6 || view.Fail != 1 {
		t.Fatalf("receipt fields drifted from the executor spelling: %+v", view)
	}

	if view.Verdict != executor.AttestationCountMismatch || view.Detail == "" {
		t.Fatalf("verdict = %q / %q, want the refusal class and detail", view.Verdict, view.Detail)
	}

	if len(view.Checked) != 2 {
		t.Fatalf("checked = %v, want the two performed comparisons", view.Checked)
	}
}

// TestVerifyReceiptViewAbsentIsNil keeps absence honest: a legacy detail has no
// receipt and the view is nil, so the surfaces print nothing instead of an
// empty green block.
func TestVerifyReceiptViewAbsentIsNil(t *testing.T) {
	t.Parallel()

	legacy := []journal.Fact{{Type: journal.Completed, Detail: []byte(`{"verify_tail":"ok: build"}`)}}
	if view := buildVerifyReceiptView(legacy); view != nil {
		t.Fatalf("legacy detail produced a receipt view: %+v", view)
	}

	if view := buildVerifyReceiptView(nil); view != nil {
		t.Fatalf("empty trail produced a receipt view: %+v", view)
	}
}

// TestCmdShowRendersVerifySection pins the first surface: a completed agent run
// with a receipt renders a top-level `verify` section carrying the verdict, so
// `tq show` answers "which gate proved this?" without a log hunt.
func TestCmdShowRendersVerifySection(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "show-verify.db")
	store := openTestStore(t, dbPath)
	ctx := context.Background()

	id := seedCompletedVerifyingTask(t, store, ctx, &executor.VerifyReceipt{
		Command: "go build ./...",
		Head:    "0123456",
		OK:      3,
	}, &executor.VerifyVerdict{OK: true, Checked: []string{"receipt", "counts"}})

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reader, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer reader.Close()

	resolved, err := resolveTask(ctx, reader, id.String())
	if err != nil {
		t.Fatalf("resolve task: %v", err)
	}

	var buf bytes.Buffer
	if err := showTaskJSON(ctx, reader, &buf, resolved, false); err != nil {
		t.Fatalf("showTaskJSON: %v", err)
	}

	out := buf.String()

	var doc struct {
		Verify *struct {
			Command string `json:"command"`
			OK      int    `json:"ok"`
			Verdict string `json:"verdict"`
		} `json:"verify"`
	}

	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("decode show output: %v\n%s", err, out)
	}

	if doc.Verify == nil {
		t.Fatalf("show output carries no verify section:\n%s", out)
	}

	if doc.Verify.Command != "go build ./..." || doc.Verify.OK != 3 || doc.Verify.Verdict != "ok" {
		t.Fatalf("verify section = %+v, want the receipt with an ok verdict", doc.Verify)
	}
}

// TestCmdDLQRowsCarryVerifySuffix pins the second surface: a dead task's row
// carries the receipt verdict inline, beside the existing evidence= suffix, so
// a refusal is visible in a listing without opening anything.
func TestCmdDLQRowsCarryVerifySuffix(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "dlq-verify.db")
	store := openTestStore(t, dbPath)
	ctx := context.Background()

	enq, err := store.Enqueue(ctx, task.New{
		Type:        executor.TaskTypeAgent,
		Project:     "demo",
		Payload:     []byte(`{"repo":"demo","prompt":"p"}`),
		MaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	_, claim, err := store.ClaimDue(ctx, "dlq-verify", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	evidence, err := json.Marshal(executor.FailureEvidence{
		Stage:       "verify",
		ExitCode:    1,
		VerifyStage: executor.VerifyStageTest,
		Verify: &executor.VerifyReceipt{
			Command: "go test ./...",
			RC:      1,
			OK:      2,
			Fail:    1,
		},
		Attestation: &executor.VerifyVerdict{
			Refusal: executor.AttestationCommandMismatch,
			Detail:  `executed "go test ./...", pinned "go test ./... -race"`,
			Checked: []string{executor.AttestationCheckReceipt, executor.AttestationCheckCommand},
		},
	})
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}

	if err := store.Fail(ctx, enq.ID, claim, "verify failed", 0, evidence); err != nil {
		t.Fatalf("fail: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runDLQ([]string{"--db", dbPath}, &buf); err != nil {
		t.Errorf("runDLQ: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "evidence=verify/test") {
		t.Fatalf("row lost the evidence= suffix:\n%s", out)
	}

	if !strings.Contains(out, "verify="+executor.AttestationCommandMismatch) {
		t.Fatalf("row lacks the receipt verdict:\n%s", out)
	}
}

// openTestStore opens a sqlite store at path or fails the test.
func openTestStore(t *testing.T, path string) *sqlite.Store {
	t.Helper()

	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	return store
}

// seedCompletedVerifyingTask enqueues + claims + completes one agent task with a
// verify receipt in its completion detail, returning its id.
func seedCompletedVerifyingTask(
	t *testing.T,
	store *sqlite.Store,
	ctx context.Context,
	receipt *executor.VerifyReceipt,
	verdict *executor.VerifyVerdict,
) task.ID {
	t.Helper()

	enq, err := store.Enqueue(ctx, task.New{
		Type:        executor.TaskTypeAgent,
		Project:     "demo",
		Payload:     []byte(`{"repo":"demo","prompt":"p"}`),
		MaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	_, claim, err := store.ClaimDue(ctx, "show-verify", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	detail, err := json.Marshal(executor.AgentResult{Verify: receipt, Attestation: verdict})
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}

	if err := store.Complete(ctx, enq.ID, claim, detail); err != nil {
		t.Fatalf("complete: %v", err)
	}

	return enq.ID
}
