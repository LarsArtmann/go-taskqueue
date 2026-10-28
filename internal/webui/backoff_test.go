package webui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/dlqbackoff"
)

// TestDLQBackoffPanelRenders pins the dashboard's poisoned-repo panel:
// with deadlines in the snapshot, the dead-letter table renders one line
// per repo (sorted) carrying the absolute deadline, a forward "in"
// estimate, and the shared poisoned advice; without deadlines (surface
// off or nothing standing) nothing renders.
func TestDLQBackoffPanelRenders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 10, 28, 5, 0, 0, 0, time.UTC)

	data := DashboardData{
		Now: now,
		DLQBackoff: map[string]time.Time{
			"zeta":  now.Add(90 * time.Minute),
			"alpha": now.Add(30 * time.Minute),
		},
	}

	html := renderComponent(ctx, DeadLetterTable(data))

	for _, want := range []string{
		"tq-dlqbackoff",
		"alpha",
		"zeta",
		"in DLQ backoff until",
		"(in 30m)",
		dlqbackoff.PoisonedAdvice,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("dead-letter fragment missing %q", want)
		}
	}

	// Sorted: alpha's line must come before zeta's despite the map's
	// random iteration order.
	if strings.Index(html, "alpha") > strings.Index(html, "zeta") {
		t.Error("backoff panel rows are not repo-sorted")
	}

	empty := DashboardData{Now: now}

	if html := renderComponent(ctx, DeadLetterTable(empty)); strings.Contains(html, "tq-dlqbackoff") {
		t.Error("backoff panel rendered with no deadlines in the snapshot")
	}
}

// TestDLQBackoffChipRenders pins the nowband meta chip: one "backoff"
// chip counting the repos in poisoned backoff, absent otherwise.
func TestDLQBackoffChipRenders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()

	data := DashboardData{
		Now:        now,
		DLQBackoff: map[string]time.Time{"poisoned": now.Add(time.Hour)},
	}

	stats := renderComponent(ctx, StatusCards(data))
	for _, want := range []string{"card-dlqbackoff", "backoff 1"} {
		if !strings.Contains(stats, want) {
			t.Errorf("stats fragment missing %q", want)
		}
	}

	quiet := DashboardData{Now: now}

	if stats := renderComponent(ctx, StatusCards(quiet)); strings.Contains(stats, "card-dlqbackoff") {
		t.Error("stats fragment shows a backoff chip with no repos in backoff")
	}
}

// TestSnapshotLoadsDLQBackoff pins the snapshot wiring end to end: with
// Config.DLQBackoff set, a fresh dead letter puts its repo into the
// snapshot's deadline map; with the surface off the map stays nil and
// the journal is not folded.
func TestSnapshotLoadsDLQBackoff(t *testing.T) {
	ctx := context.Background()

	s := newTestStore(t)

	srv := New(s, Config{
		Addr:       "127.0.0.1:0",
		Poll:       20 * time.Millisecond,
		Heartbeat:  100 * time.Millisecond,
		DLQBackoff: time.Hour,
	})

	tk := enqueue(t, s, "sh", "poisoned")

	claimed, claim, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if claimed.ID != tk.ID {
		t.Fatalf("claimed %s, want the enqueued %s", claimed.ID, tk.ID)
	}

	if err := s.FailPermanent(ctx, claimed.ID, claim, "boom", nil); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	data, err := srv.loadSnapshot(ctx, FilterState{})
	if err != nil {
		t.Fatalf("loadSnapshot: %v", err)
	}

	until, ok := data.DLQBackoff["poisoned"]
	if !ok {
		t.Fatal("snapshot lacks the poisoned repo despite a fresh dead letter")
	}

	if !until.After(time.Now()) || until.After(time.Now().Add(2*time.Hour)) {
		t.Fatalf("deadline = %v, want ~1h past the dead letter", until)
	}

	off := New(s, Config{Addr: "127.0.0.1:0", Poll: 20 * time.Millisecond, Heartbeat: 100 * time.Millisecond})

	data, err = off.loadSnapshot(ctx, FilterState{})
	if err != nil {
		t.Fatalf("loadSnapshot off: %v", err)
	}

	if data.DLQBackoff != nil {
		t.Fatalf("DLQBackoff = %v, want nil with the surface off", data.DLQBackoff)
	}
}
