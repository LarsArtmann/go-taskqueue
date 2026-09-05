package journal

import (
	"context"
	"testing"
)

func TestMemoryJournalAppendAll(t *testing.T) {
	j := NewMemoryJournal()
	ctx := context.Background()
	for i := range 5 {
		if _, err := j.Append(ctx, Fact{TaskID: "t1", Type: Enqueued, Attempt: i}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	facts, err := j.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(facts) != 5 {
		t.Fatalf("len(All) = %d, want 5", len(facts))
	}
	for i, f := range facts {
		if f.Seq != int64(i+1) {
			t.Errorf("facts[%d].Seq = %d, want %d", i, f.Seq, i+1)
		}
		if f.Time.IsZero() {
			t.Errorf("facts[%d].Time not set", i)
		}
	}
}

func TestMemoryJournalSince(t *testing.T) {
	j := NewMemoryJournal()
	ctx := context.Background()
	for range 5 {
		if _, err := j.Append(ctx, Fact{TaskID: "t1", Type: Heartbeat}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	facts, err := j.Since(ctx, 3)
	if err != nil {
		t.Fatalf("Since: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("len(Since(3)) = %d, want 2", len(facts))
	}
	if facts[0].Seq != 4 || facts[1].Seq != 5 {
		t.Fatalf("Since(3) seqs = %d,%d, want 4,5", facts[0].Seq, facts[1].Seq)
	}
}

func TestMemoryJournalConcurrency(t *testing.T) {
	j := NewMemoryJournal()
	ctx := context.Background()
	done := make(chan struct{})
	for g := range 8 {
		go func() {
			for k := range 100 {
				if _, err := j.Append(ctx, Fact{TaskID: "g", Type: Heartbeat, Attempt: g + k}); err != nil {
					t.Errorf("Append: %v", err)
				}
			}
			done <- struct{}{}
		}()
	}
	for range 8 {
		<-done
	}
	facts, _ := j.All(ctx)
	if len(facts) != 800 {
		t.Fatalf("len(All) = %d, want 800", len(facts))
	}
}
