package watermark

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

var errRejected = errors.New("handler rejected the fact")

// memSource is an in-memory Source: append-only facts, one watermark row.
// Start EMPTY and build the cursor first — first use bootstraps at head —
// then appendFacts the traffic the test sweeps.
type memSource struct {
	facts []journal.Fact
	rows  map[string]int64
}

func newMemSource() *memSource {
	return &memSource{rows: map[string]int64{}}
}

func (m *memSource) appendFacts(count int) {
	for i := 1; i <= count; i++ {
		m.facts = append(m.facts, journal.Fact{
			Seq:    int64(len(m.facts) + 1),
			TaskID: fmt.Sprintf("t%d", len(m.facts)+1),
			Type:   journal.Completed,
		})
	}
}

func (m *memSource) Facts(_ context.Context, after int64, limit int) ([]journal.Fact, error) {
	var out []journal.Fact

	for _, f := range m.facts {
		if f.Seq <= after {
			continue
		}

		out = append(out, f)
		if len(out) == limit {
			break
		}
	}

	return out, nil
}

func (m *memSource) HeadSeq(_ context.Context) (int64, error) {
	if len(m.facts) == 0 {
		return 0, nil
	}

	return m.facts[len(m.facts)-1].Seq, nil
}

func (m *memSource) Watermark(_ context.Context, consumer string) (int64, bool, error) {
	seq, ok := m.rows[consumer]

	return seq, ok, nil
}

func (m *memSource) SaveWatermark(_ context.Context, consumer string, seq int64) error {
	m.rows[consumer] = seq

	return nil
}

// TestCursorRedeliversRejectedFactsOnRetrySweep pins the Handler
// contract's at-least-once BOTH ways: a handler rejection holds the
// persisted checkpoint, and the retry sweep of the SAME cursor (the
// in-process pending-checkpoint gate) redelivers the rejected fact instead
// of silently persisting past it.
func TestCursorRedeliversRejectedFactsOnRetrySweep(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		rejectAt    int64
		wantFirst   []int64
		wantFirstWM int64
		wantRetry   []int64
	}{
		{
			name:        "rejected first fact",
			rejectAt:    1,
			wantFirst:   []int64{1},
			wantFirstWM: 0,
			wantRetry:   []int64{1, 2, 3},
		},
		{
			name:        "rejected fact after accepted work",
			rejectAt:    2,
			wantFirst:   []int64{1, 2},
			wantFirstWM: 0,
			wantRetry:   []int64{2, 3},
		},
		{
			name:        "everything accepted",
			rejectAt:    0,
			wantFirst:   []int64{1, 2, 3},
			wantFirstWM: 3,
			wantRetry:   nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runRedeliveryCase(t, tc)
		})
	}
}

// redeliveryCase is one row of the at-least-once table: which fact the
// handler rejects (0 = none), the deliveries and persisted watermark the
// first sweep must leave, and the redeliveries the retry sweep must make.
type redeliveryCase struct {
	name        string
	rejectAt    int64
	wantFirst   []int64
	wantFirstWM int64
	wantRetry   []int64
}

func runRedeliveryCase(t *testing.T, tc redeliveryCase) {
	t.Helper()

	ctx := context.Background()
	src := newMemSource()

	cur, err := New(ctx, Config{Store: src, Key: "test", Domain: "test sweep"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	src.appendFacts(3)

	rejected := false

	sweep := func() ([]int64, error) {
		var seen []int64

		err := cur.Sweep(ctx, func(_ context.Context, f journal.Fact) error {
			seen = append(seen, f.Seq)

			if f.Seq == tc.rejectAt && !rejected {
				rejected = true

				return errRejected
			}

			return nil
		})

		return seen, err
	}

	first, err := sweep()
	if tc.rejectAt == 0 && err != nil {
		t.Fatalf("first sweep: %v", err)
	}

	if tc.rejectAt != 0 && !errors.Is(err, errRejected) {
		t.Fatalf("first sweep err = %v, want rejection", err)
	}

	assertSeqs(t, "first sweep", first, tc.wantFirst)

	wm, ok, err := src.Watermark(ctx, "test")
	if err != nil || !ok {
		t.Fatalf("watermark: %v (ok %v)", err, ok)
	}

	if wm != tc.wantFirstWM {
		t.Fatalf("persisted watermark after first sweep = %d, want %d", wm, tc.wantFirstWM)
	}

	retry, err := sweep()
	if err != nil {
		t.Fatalf("retry sweep: %v", err)
	}

	assertSeqs(t, "retry sweep", retry, tc.wantRetry)

	if wm, _, _ := src.Watermark(ctx, "test"); wm != 3 {
		t.Fatalf("final watermark = %d, want 3", wm)
	}
}

func assertSeqs(t *testing.T, what string, got, want []int64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s delivered %v, want %v", what, got, want)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s delivered %v, want %v", what, got, want)
		}
	}
}
