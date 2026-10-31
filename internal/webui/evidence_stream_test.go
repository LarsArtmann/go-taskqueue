package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-sse/ssetest"
	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// seedDeadEvidence enqueues, claims, and fails one agent task whose failed
// fact carries the given FailureEvidence detail (nil = no evidence).
func seedDeadEvidence(t *testing.T, s *sqlite.Store, owner, failReason string, evidence any) task.Task {
	t.Helper()

	var detail []byte

	if evidence != nil {
		raw, err := json.Marshal(evidence)
		if err != nil {
			t.Fatalf("marshal evidence: %v", err)
		}

		detail = raw
	}

	ctx := context.Background()

	tk, err := s.Enqueue(ctx, task.New{
		Type:        "agent",
		Project:     "demo",
		Payload:     json.RawMessage(`"echo hi"`),
		MaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	_, claim, err := s.ClaimDue(ctx, owner, time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if err := s.Fail(ctx, tk.ID, claim, failReason, 0, detail); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	return tk
}

// TestStreamSnapshotHonorsEvidenceFilter pins TODO row 1410: the SSE
// snapshot on /api/events renders the DLQ fragment under the request's
// ?evidence= filter, so a live filtered view is never clobbered by the
// unfiltered DLQ — the same contract TestStreamSnapshotHonorsFilter pins
// for the project filter, extended to the evidence axis app.js forwards.
func TestStreamSnapshotHonorsEvidenceFilter(t *testing.T) {
	srv, s := newTestServer(t)

	timeout := seedDeadEvidence(t, s, "evidence-stream-a", "verify failed", executor.FailureEvidence{
		Stage:       "verify",
		VerifyStage: executor.VerifyStageE2ETimeout,
		ExitCode:    2,
	})
	plain := seedDeadEvidence(t, s, "evidence-stream-b", "boom", nil)

	events := ssetest.CollectN(t, srv.Handler(), 6, ssetest.WithPath("/api/events?evidence=e2e-timeout"))

	dlqFrag := ""

	for _, evt := range events {
		if evt.Type != testFragEvent {
			continue
		}

		var frag fragment
		if err := json.Unmarshal([]byte(evt.Data()), &frag); err != nil {
			t.Fatalf("decode frag: %v", err)
		}

		if frag.ID == fragDLQ {
			dlqFrag = frag.HTML
		}
	}

	if dlqFrag == "" {
		t.Fatal("no DLQ fragment in the stream snapshot")
	}

	if !strings.Contains(dlqFrag, timeout.ID.String()) || strings.Contains(dlqFrag, plain.ID.String()) {
		t.Errorf("stream snapshot ignored the evidence filter; got: %s", dlqFrag)
	}
}

// TestEvidenceChipRendersRawSpec pins TODO row 1421: the active-filter
// chip renders the raw evidence spec VERBATIM — negations included — so a
// round-tripped `?evidence=gate-env,!gate-slow` URL shows the operator
// exactly what MatchesEvidenceStageSet will apply, not a normalized
// subset.
func TestEvidenceChipRendersRawSpec(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?evidence=gate-env,!gate-slow", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "gate-env,!gate-slow") {
		t.Errorf("page lost the raw evidence spec; want the verbatim chip text in: %.600s", rec.Body.String())
	}
}
