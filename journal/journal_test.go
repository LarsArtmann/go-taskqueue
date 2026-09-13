package journal_test

import (
	"context"
	"testing"

	tq "github.com/larsartmann/go-taskqueue/journal"
)

func TestFacadeSurface(t *testing.T) {
	ctx := context.Background()
	j := tq.NewMemoryJournal()

	f, err := j.Append(ctx, tq.Fact{TaskID: "t1", Type: tq.Enqueued})
	if err != nil || f.Seq != 1 {
		t.Fatalf("append = seq %d (%v), want 1", f.Seq, err)
	}

	if f, err := j.Append(ctx, tq.Fact{TaskID: "t1", Type: tq.Claimed}); err != nil || f.Seq != 2 {
		t.Fatalf("append = seq %d (%v), want 2", f.Seq, err)
	}

	tail, err := j.Since(ctx, 1)
	if err != nil || len(tail) != 1 || tail[0].Type != tq.Claimed {
		t.Fatalf("since = %d facts (%v), want 1 claimed", len(tail), err)
	}

	all, err := j.All(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("all = %d facts (%v), want 2", len(all), err)
	}

	var _ tq.Journal = j
}
