package readmodel

import (
	"encoding/json"
	"encoding/json/jsontext"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// The local detail structs in events.go decode the wire keys of
// queue-owned fact details by hand. A queue-side tag rename would not
// fail compilation here — it would silently decode zero values into the
// fold (a reprioritized task reporting priority 0, a requeued task losing
// its park window, an enqueued task losing its project/type fallback).
// These tests marshal the REAL queue types and pin the local decode, the
// same key-parity discipline budget_test applies to the executor result
// shapes (internal/budget/budget.go, sessionUsageDetail).
func TestRepriDetailMirrorsQueueEvidence(t *testing.T) {
	t.Parallel()

	src, err := json.Marshal(queue.ReprioritizeEvidence{
		OldPriority: 3,
		NewPriority: 7,
		Source:      "ai",
		Reason:      "rescored",
	})
	if err != nil {
		t.Fatalf("marshal queue.ReprioritizeEvidence: %v", err)
	}

	var got repriDetail
	if err := json.Unmarshal(src, &got); err != nil {
		t.Fatalf("unmarshal into repriDetail: %v", err)
	}

	want := repriDetail{OldPriority: 3, NewPriority: 7, Source: "ai", Reason: "rescored"}
	if got != want {
		t.Errorf("repriDetail = %+v, want %+v (a queue-side tag renamed or drifted)", got, want)
	}
}

func TestRequeueDetailMirrorsQueueEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  queue.RequeueEvidence
		want requeueDetail
	}{
		{
			name: "full evidence decodes all keys",
			src: queue.RequeueEvidence{
				Reason:         "env not ready",
				RetryIn:        3_600_000,
				ResumeCloseout: true,
				Class:          queue.RequeueClassBudget,
			},
			want: requeueDetail{RetryIn: 3_600_000, Class: queue.RequeueClassBudget},
		},
		{
			// Legacy facts predate the class field entirely (omitempty
			// drops it on marshal); the local decode must read it as the
			// empty string so requeuedEvent normalizes to "unknown".
			name: "absent class decodes empty",
			src:  queue.RequeueEvidence{RetryIn: 5},
			want: requeueDetail{RetryIn: 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src, err := json.Marshal(tt.src)
			if err != nil {
				t.Fatalf("marshal queue.RequeueEvidence: %v", err)
			}

			var got requeueDetail
			if err := json.Unmarshal(src, &got); err != nil {
				t.Fatalf("unmarshal into requeueDetail: %v", err)
			}

			if got != tt.want {
				t.Errorf("requeueDetail = %+v, want %+v (a queue-side tag renamed or drifted)", got, tt.want)
			}
		})
	}
}

func TestEnqueueDetailMirrorsQueueDetail(t *testing.T) {
	t.Parallel()

	priority := 2

	src, err := json.Marshal(queue.EnqueueDetail{
		Project:  "web",
		Type:     "sh",
		Priority: &priority,
		Payload:  `"x"`,
	})
	if err != nil {
		t.Fatalf("marshal queue.EnqueueDetail: %v", err)
	}

	var got enqueueDetail
	if err := json.Unmarshal(src, &got); err != nil {
		t.Fatalf("unmarshal into enqueueDetail: %v", err)
	}

	want := enqueueDetail{Project: "web", Type: "sh"}
	if got != want {
		t.Errorf("enqueueDetail = %+v, want %+v (a queue-side tag renamed or drifted)", got, want)
	}
}

// TestRepriEventDetailPolicy pins repriEvent's decode policy: unlike
// requeuedEvent (empty/malformed detail degrades to the legacy shape) and
// unlike queue.ParseReprioritizeEvidence (skip-don't-fail), an unparsable
// reprioritized detail is a MALFORMED FACT — the pump must surface it as
// an error so journal drift is visible instead of folding a priority-0
// row.
func TestRepriEventDetailPolicy(t *testing.T) {
	t.Parallel()

	base := journal.Fact{
		Seq:    1,
		Time:   time.UnixMilli(1_000),
		TaskID: "t1",
		Type:   journal.Reprioritized,
	}

	t.Run("malformed detail surfaces journal drift", func(t *testing.T) {
		t.Parallel()

		fact := base
		fact.Detail = jsontext.Value(`{"new_priority":`)

		evt, ok, err := repriEvent(fact, fact.Time.UnixMilli())
		if err == nil {
			t.Fatalf("repriEvent malformed detail: want error, got nil (evt=%v ok=%v)", evt, ok)
		}

		if ok {
			t.Errorf("repriEvent malformed detail: ok=true, want false")
		}
	})

	t.Run("valid detail folds the new priority", func(t *testing.T) {
		t.Parallel()

		fact := base
		fact.Detail = jsontext.Value(`{"old_priority":3,"new_priority":7,"source":"ai","reason":"rescored"}`)

		evt, ok, err := repriEvent(fact, fact.Time.UnixMilli())
		if err != nil || !ok {
			t.Fatalf("repriEvent valid detail: err=%v ok=%v, want nil/true", err, ok)
		}

		want := evtReprioritized{ID: "t1", Priority: 7, At: 1_000}
		if evt != any(want) {
			t.Errorf("repriEvent = %+v, want %+v", evt, want)
		}
	})
}
