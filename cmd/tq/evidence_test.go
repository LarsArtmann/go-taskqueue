package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestEvidenceStageSetMatches pins the comma-separated term spec with
// `!`-negation on top of the single-term matcher: a task survives when it
// hits at least one positive term and no negated term; negation-only
// specs match every non-hit; blank specs match nothing.
func TestEvidenceStageSetMatches(t *testing.T) {
	t.Parallel()

	gateEnv := executor.FailureEvidence{Stage: "verify"}
	gateEnvError := `agent verify gate environmental signature [vendor-gofmt] ("gofmt"): boom`

	tests := []struct {
		name  string
		spec  string
		match bool
		setup func(evidence *executor.FailureEvidence, lastError *string)
	}{
		{
			name:  "positive and negative, positive hit survives",
			spec:  "gate-env,!gate-slow",
			match: true,
			setup: func(evidence *executor.FailureEvidence, lastError *string) {
				*evidence, *lastError = gateEnv, gateEnvError
			},
		},
		{
			name:  "negated hit excludes",
			spec:  "verify,!gate-env",
			match: false,
			setup: func(evidence *executor.FailureEvidence, lastError *string) {
				*evidence, *lastError = gateEnv, gateEnvError
			},
		},
		{
			name:  "negation-only spec passes non-hits",
			spec:  "!gate-slow",
			match: true,
		},
		{
			name:  "negation-only spec excludes its hit",
			spec:  "!gate-env",
			match: false,
			setup: func(evidence *executor.FailureEvidence, lastError *string) {
				*evidence, *lastError = gateEnv, gateEnvError
			},
		},
		{
			name:  "two positives, second hits",
			spec:  "gate-dead, e2e-timeout",
			match: true,
			setup: func(evidence *executor.FailureEvidence, lastError *string) {
				evidence.Stage, evidence.VerifyStage = "verify", executor.VerifyStageE2ETimeout
			},
		},
		{
			name:  "no positive hits",
			spec:  "gate-dead,gate-slow",
			match: false,
			setup: func(evidence *executor.FailureEvidence, lastError *string) {
				*evidence, *lastError = gateEnv, gateEnvError
			},
		},
		{
			name:  "blank spec matches nothing",
			spec:  " , ! ,",
			match: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				evidence  executor.FailureEvidence
				lastError string
			)

			if tt.setup != nil {
				tt.setup(&evidence, &lastError)
			}

			if got := executor.MatchesEvidenceStageSet(evidence, lastError, tt.spec); got != tt.match {
				t.Fatalf("MatchesEvidenceStageSet(%+v, %q, %q) = %v, want %v", evidence, lastError, tt.spec, got, tt.match)
			}
		})
	}
}

// TestRequeuesCarryNoFailureEvidence pins the dead-only BY DESIGN ruling
// (2026-10-26 01-40 §f5): the non-death refusals — provider 429 requeues,
// budget-parked claims, preflight (env) requeues — record task.requeued
// facts whose detail is RequeueEvidence, never FailureEvidence, so
// evidence-stage triage can never classify a live parked task as a
// death. Pins both halves: the shared trail reader returns zero evidence
// for a requeue-only trail, and the requeue detail JSON carries no
// stage/verify_stage/exit_code fields that could collide with a future
// scan-all-facts reader.
func TestRequeuesCarryNoFailureEvidence(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "requeue-evidence.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	defer store.Close()

	ctx := context.Background()

	classes := []struct {
		name     string
		class    string
		provider string
	}{
		{name: "429 requeue", class: queue.RequeueClassRateLimit, provider: "anthropic"},
		{name: "budget park", class: queue.RequeueClassBudget},
		{name: "env requeue", class: queue.RequeueClassPreflight},
	}

	for _, tc := range classes {
		t.Run(tc.name, func(t *testing.T) {
			enq, err := store.Enqueue(ctx, task.New{
				Type:        executor.TaskTypeAgent,
				Project:     "demo",
				Payload:     []byte(`{"repo":"demo","prompt":"p"}`),
				MaxAttempts: 3,
			})
			if err != nil {
				t.Fatalf("enqueue: %v", err)
			}

			_, claim, err := store.ClaimDue(ctx, "requeue-evidence", time.Minute)
			if err != nil {
				t.Fatalf("claim: %v", err)
			}

			if err := store.Requeue(ctx, enq.ID, claim, "not the task's fault", 15*time.Minute, false, tc.class, tc.provider); err != nil {
				t.Fatalf("requeue: %v", err)
			}

			trail, err := store.FactsForTask(ctx, enq.ID.String(), 0)
			if err != nil {
				t.Fatalf("facts: %v", err)
			}

			for _, fact := range trail {
				if fact.Type == journal.Failed {
					t.Fatalf("requeue recorded a task.failed fact (class %s): %+v", tc.class, fact)
				}
			}

			if evidence := executor.LastFailureEvidenceFromTrail(trail); evidence != (executor.FailureEvidence{}) {
				t.Fatalf("requeue-only trail yielded failure evidence %+v (class %s)", evidence, tc.class)
			}

			var (
				asFailure  executor.FailureEvidence
				asRequeue  queue.RequeueEvidence
				rawDetails []string
			)

			for _, fact := range trail {
				if fact.Type == journal.Requeued {
					rawDetails = append(rawDetails, string(fact.Detail))
				}
			}

			if len(rawDetails) == 0 {
				t.Fatalf("no task.requeued fact on the trail: %+v", trail)
			}

			for _, raw := range rawDetails {
				if err := json.Unmarshal([]byte(raw), &asFailure); err != nil {
					t.Fatalf("decode requeue detail as failure evidence: %v (%s)", err, raw)
				}

				if asFailure.Stage != "" || asFailure.VerifyStage != "" || asFailure.ExitCode != 0 {
					t.Fatalf("requeue detail collides with FailureEvidence fields: %s -> %+v", raw, asFailure)
				}

				if err := json.Unmarshal([]byte(raw), &asRequeue); err != nil {
					t.Fatalf("decode requeue detail: %v (%s)", err, raw)
				}

				if asRequeue.Class != tc.class {
					t.Fatalf("requeue detail class = %q, want %q", asRequeue.Class, tc.class)
				}
			}
		})
	}
}

// TestCmdDLQEvidenceStageFilter runs the filter end-to-end against a
// scratch store: only dead tasks whose failure evidence names the class
// survive, matched through either the structured verify_stage stamp or the
// gate-dead error text.
func TestCmdDLQEvidenceStageFilter(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "dlq-filter.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	ctx := context.Background()

	seedDead := func(failReason string, evidence any) task.ID {
		t.Helper()

		raw, err := json.Marshal(evidence)
		if err != nil {
			t.Fatalf("marshal evidence: %v", err)
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

		_, claim, err := store.ClaimDue(ctx, "filter-test", time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		if err := store.Fail(ctx, enq.ID, claim, failReason, 0, raw); err != nil {
			t.Fatalf("fail: %v", err)
		}

		return enq.ID
	}

	timeoutID := seedDead("verify failed", executor.FailureEvidence{
		Stage:       "verify",
		VerifyStage: executor.VerifyStageE2ETimeout,
		Tail:        "panic: test timed out after 10m0s",
	})
	gateDeadID := seedDead(`agent verify gate dead ("gofmt"): failure is pre-existing`, executor.FailureEvidence{Stage: "verify"})
	plainID := seedDead("boom", nil)

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	var out string

	out = captureStdout(t, func() {
		if err := cmdDLQ([]string{"--db", dbPath, "--evidence-stage", "e2e-timeout"}); err != nil {
			t.Errorf("cmdDLQ: %v", err)
		}
	})

	if !strings.Contains(out, timeoutID.String()) || strings.Contains(out, gateDeadID.String()) || strings.Contains(out, plainID.String()) {
		t.Fatalf("--evidence-stage e2e-timeout picked wrong tasks:\n%s", out)
	}

	out = captureStdout(t, func() {
		if err := cmdDLQ([]string{"--db", dbPath, "--evidence-stage", "gate-dead"}); err != nil {
			t.Errorf("cmdDLQ: %v", err)
		}
	})

	if !strings.Contains(out, gateDeadID.String()) || strings.Contains(out, timeoutID.String()) || strings.Contains(out, plainID.String()) {
		t.Fatalf("--evidence-stage gate-dead picked wrong tasks:\n%s", out)
	}

	// Set spec with negation (2026-10-26 01-40 §f4): the e2e-timeout task
	// survives `verify,!gate-dead` while the gate-dead task is excluded.
	out = captureStdout(t, func() {
		if err := cmdDLQ([]string{"--db", dbPath, "--evidence-stage", "verify,!gate-dead"}); err != nil {
			t.Errorf("cmdDLQ: %v", err)
		}
	})

	if !strings.Contains(out, timeoutID.String()) || strings.Contains(out, gateDeadID.String()) || strings.Contains(out, plainID.String()) {
		t.Fatalf("--evidence-stage verify,!gate-dead picked wrong tasks:\n%s", out)
	}
}

// TestShowJSONCarriesFailureEvidence pins the tq show failure section:
// a dead task's last task.failed fact surfaces as structured evidence
// (stage, verify_stage, exit code, tail) on the show wire.
func TestShowJSONCarriesFailureEvidence(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "show-evidence.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

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

	_, claim, err := store.ClaimDue(ctx, "show-evidence", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	evidence, err := json.Marshal(executor.FailureEvidence{
		Stage:       "verify",
		ExitCode:    2,
		VerifyStage: executor.VerifyStageTest,
		Tail:        "--- FAIL: TestShipTheThing\nFAIL",
		Class:       "environment",
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

	out := captureStdout(t, func() {
		if err := cmdShow([]string{"--db", dbPath, enq.ID.String()}); err != nil {
			t.Errorf("cmdShow: %v", err)
		}
	})

	var doc struct {
		Failure *struct {
			Stage       string `json:"stage"`
			VerifyStage string `json:"verify_stage"`
			Class       string `json:"class"`
			ExitCode    int    `json:"exit_code"`
			Tail        string `json:"tail"`
		} `json:"failure"`
	}

	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("decode show output: %v\n%s", err, out)
	}

	if doc.Failure == nil {
		t.Fatalf("show output carries no failure section:\n%s", out)
	}

	if doc.Failure.Stage != "verify" || doc.Failure.VerifyStage != "test" ||
		doc.Failure.Class != "environment" || doc.Failure.ExitCode != 2 ||
		!strings.Contains(doc.Failure.Tail, "FAIL: TestShipTheThing") {
		t.Fatalf("failure section = %+v", doc.Failure)
	}
}
