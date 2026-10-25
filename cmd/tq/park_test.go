package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// parkVerbStore opens, hands the path to the verb under test, and reopens:
// cmdPark owns its connection, so the test never holds a second write pool.
func parkVerbStore(t *testing.T, path string) *sqlite.Store {
	t.Helper()

	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	return store
}

func TestParkVerbSuppressesClaimsAndReleaseReArms(t *testing.T) {
	t.Setenv("USER", "lars")

	path := t.TempDir() + "/q.db"
	ctx := context.Background()

	seed := parkVerbStore(t, path)

	tk, err := seed.Enqueue(ctx, task.New{Project: "p", Type: "sh", Payload: []byte(`"true"`)})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	_ = seed.Close()

	if err := cmdPark([]string{tk.ID.String(), "--until", "1h", "--reason", "owner hold", "--db", path}); err != nil {
		t.Fatalf("park: %v", err)
	}

	store := parkVerbStore(t, path)

	if _, _, err := store.ClaimDue(ctx, "w1", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
		t.Fatalf("claim while parked = %v, want ErrNoTaskDue", err)
	}

	facts, err := store.FactsForTask(ctx, tk.ID.String(), 0)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	parked := 0

	for _, fact := range facts {
		if fact.Type != journal.Parked {
			continue
		}

		parked++

		if fact.Owner != "lars" || !strings.Contains(string(fact.Detail), `"reason":"owner hold"`) {
			t.Fatalf("park fact = owner %s detail %s, want lars + owner-hold reason", fact.Owner, fact.Detail)
		}
	}

	if parked != 1 {
		t.Fatalf("task.parked facts = %d, want 1", parked)
	}

	if fresh, err := store.Get(ctx, tk.ID); err != nil || fresh.Attempts != 0 || fresh.Status != task.Pending {
		t.Fatalf("park mutated attempts/status: %+v (%v)", fresh, err)
	}

	_ = store.Close()

	if err := cmdPark([]string{tk.ID.String(), "--release", "--reason", "resume", "--db", path}); err != nil {
		t.Fatalf("release: %v", err)
	}

	freed := parkVerbStore(t, path)

	claimed, claim, err := freed.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim after release: %v", err)
	}

	if claimed.ID != tk.ID {
		t.Fatalf("claimed = %s, want %s", claimed.ID, tk.ID)
	}

	if err := freed.Complete(ctx, claimed.ID, claim, nil); err != nil {
		t.Fatalf("complete: %v", err)
	}
}

func TestParkVerbRefusals(t *testing.T) {
	path := t.TempDir() + "/q.db"
	ctx := context.Background()

	seed := parkVerbStore(t, path)

	sacrificial, err := seed.Enqueue(ctx, task.New{Project: "p", Type: "sh", Payload: []byte(`"true"`)})
	if err != nil {
		t.Fatalf("enqueue sacrificial: %v", err)
	}

	running, _, err := seed.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if running.ID != sacrificial.ID {
		t.Fatalf("claim picked %s, want the only pending task %s", running.ID, sacrificial.ID)
	}

	tk, err := seed.Enqueue(ctx, task.New{Project: "p", Type: "sh", Payload: []byte(`"true"`)})
	if err != nil {
		t.Fatalf("enqueue pending: %v", err)
	}

	_ = seed.Close()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "running task refused",
			args: []string{running.ID.String(), "--until", "1h", "--db", path},
			want: "not pending",
		},
		{
			name: "bad duration",
			args: []string{tk.ID.String(), "--until", "tomorrow", "--db", path},
			want: "not a positive duration",
		},
		{
			name: "over the 7d cap",
			args: []string{tk.ID.String(), "--until", "170h", "--db", path},
			want: "cap",
		},
		{
			name: "until and release are exclusive",
			args: []string{tk.ID.String(), "--until", "1h", "--release", "--db", path},
			want: "mutually exclusive",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := cmdPark(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("cmdPark(%v) = %v, want error containing %q", tc.args, err, tc.want)
			}
		})
	}

	still := parkVerbStore(t, path)

	fresh, err := still.Get(ctx, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if fresh.Status != task.Pending {
		t.Fatalf("refused park leaked a mutation: %s", fresh.Status)
	}
}
