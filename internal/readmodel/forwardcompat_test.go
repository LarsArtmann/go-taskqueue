package readmodel

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// The fold is a CONSUMER of the journal contract, and journal facts are
// produced by whatever binary wrote them — an older tq, a newer tq, or the
// upstream engine after a fact family lands there first (ADR-0019 S2 makes
// FactType an upstream type, so new values arrive without tq changing).
// That makes the fold's forward-compatibility posture a contract, not an
// implementation detail, and it is the same contract every knowledge-format
// consumer is held to:
//
//   - an unknown TYPE is skipped, never rejected (it may simply be a family
//     this binary has no fold for yet);
//   - an unknown KEY inside a known detail is ignored, so a producer can add
//     fields additively with no consumer change.
//
// Both are already true; nothing pinned them. The failure they prevent is
// asymmetric: readmodel open would still succeed, but the projection host
// dead-letters the unknown fact as corruption and burns the restart budget
// over what is really a version-skew. Sources: OKF v0.2 SPEC §11
// (consumers "MUST NOT reject a bundle because of unknown type values,
// unknown additional frontmatter keys") and docs/research/
// 2026-11-01_okf-lessons.md lessons 1/2.
//
// Deliberately NOT extended to the execution payloads: those gate on a
// contract version and refuse anything newer (internal/executor/agent.go,
// the `p.V > 1` check), because a misparsed execution payload runs the wrong
// action rather than merely rendering blank. See the lessons note, lesson 15.

// TestEventForSkipsUnknownFactType pins the skip-don't-reject policy for
// fact types this binary has never heard of.
func TestEventForSkipsUnknownFactType(t *testing.T) {
	t.Parallel()

	fact := journal.Fact{
		Seq:     1,
		Time:    time.UnixMilli(1_700_000_000_000),
		TaskID:  "000001a0unknownfacttype000000000000000",
		Type:    journal.FactType("engine.future.family"),
		Detail:  []byte(`{"anything":"goes","nested":{"still":"fine"}}`),
		Attempt: 1,
	}

	evt, ok, err := eventFor(context.Background(), fact, nil)
	if err != nil {
		t.Fatalf("eventFor(unknown type) error = %v, want nil — an unknown fact family is VERSION SKEW, not corruption", err)
	}

	if ok || evt != nil {
		t.Fatalf("eventFor(unknown type) = (%v, %v), want (nil, false) — the fold must skip what it cannot place", evt, ok)
	}
}

// TestKnownFactDetailsIgnoreUnknownKeys pins the additive-growth half: a
// producer that adds fields to an existing detail must not break a consumer
// that predates them.
func TestKnownFactDetailsIgnoreUnknownKeys(t *testing.T) {
	t.Parallel()

	t.Run("reprioritized detail", func(t *testing.T) {
		t.Parallel()

		fact := journal.Fact{
			TaskID: "000001a0repriunknownkeys00000000000000",
			Type:   journal.Reprioritized,
			// evidence_ref/confidence are fields a future producer could add;
			// the known four must still decode.
			Detail: []byte(`{"old_priority":3,"new_priority":7,"source":"ai","reason":"rescored","evidence_ref":"abc123","confidence":0.9,"schema":2}`),
		}

		evt, ok, err := eventFor(context.Background(), fact, nil)
		if err != nil {
			t.Fatalf("eventFor(known type, extra detail keys) error = %v, want nil", err)
		}

		if !ok {
			t.Fatal("eventFor(known type, extra detail keys) ok = false, want true")
		}

		got, isRepri := evt.(evtReprioritized)
		if !isRepri {
			t.Fatalf("eventFor(reprioritized) = %T, want evtReprioritized", evt)
		}

		if got.Priority != 7 {
			t.Errorf("Priority = %d, want 7 — an unknown sibling key must not disturb the known ones", got.Priority)
		}
	})

	t.Run("requeue detail", func(t *testing.T) {
		t.Parallel()

		fact := journal.Fact{
			TaskID: "000001a0requeueunknownkeys0000000000000",
			Type:   journal.Requeued,
			Detail: []byte(`{"retry_in_ms":3600000,"class":"budget","provider":"anthropic","requeue_seq":4}`),
		}

		evt, ok, err := eventFor(context.Background(), fact, nil)
		if err != nil {
			t.Fatalf("eventFor(requeued, extra detail keys) error = %v, want nil", err)
		}

		if !ok {
			t.Fatal("eventFor(requeued, extra detail keys) ok = false, want true")
		}

		got, isRequeued := evt.(evtRequeued)
		if !isRequeued {
			t.Fatalf("eventFor(requeued) = %T, want evtRequeued", evt)
		}

		if got.RetryInMs != 3_600_000 {
			t.Errorf("RetryInMs = %d, want 3600000", got.RetryInMs)
		}

		if got.Class != RequeueClass(queue.RequeueClassBudget) {
			t.Errorf("Class = %q, want %q", got.Class, queue.RequeueClassBudget)
		}
	})
}

// TestKnownFactDetailStillRejectsCorruption is the boundary of the two tests
// above: tolerance is for UNKNOWN KEYS, not for unparseable detail. A
// genuinely corrupted fact must still be a corruption-class error so the
// projection host dead-letters it instead of silently folding zero values.
func TestKnownFactDetailStillRejectsCorruption(t *testing.T) {
	t.Parallel()

	fact := journal.Fact{
		TaskID: "000001a0repriCorruption0000000000000000",
		Type:   journal.Reprioritized,
		Detail: []byte(`{"old_priority":`),
	}

	_, _, err := eventFor(context.Background(), fact, nil)
	if err == nil {
		t.Fatal("eventFor(truncated detail) error = nil, want a corruption-class error — tolerance must not extend to unparseable bytes")
	}
}
