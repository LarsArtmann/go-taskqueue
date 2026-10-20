package readmodel_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
)

// TestTasksPageCursorRoundTrip pins the row-rich read's keyset pagination:
// pages of the ledger walk newest-first with neither drops nor duplicates
// (the sqlite engine's compound sort-key cursor keeps the millisecond
// created_at ties intact), the cursor string round-trips, a malformed
// cursor FAILS the read instead of silently restarting, the default page
// size applies, and Tasks stays the unbounded export path.
func TestTasksPageCursorRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	f := &fixture{t: t, store: store}

	const total = 9
	ids := make([]string, 0, total)

	ties := 0

	var lastCreated int64

	for range total {
		tk, err := store.Enqueue(ctx, newEnqueue("web", "sh"))
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		if tk.CreatedAt == lastCreated {
			ties++
		}

		lastCreated = tk.CreatedAt
		ids = append(ids, tk.ID)
	}

	if ties == 0 {
		t.Log("no created_at ties observed this run; tie paging rides the engine's compound cursor")
	} else {
		t.Logf("%d created_at tie(s) exercised through the compound cursor", ties)
	}

	m, err := readmodel.Open(ctx, t.TempDir()+"/projection.db", store)
	if err != nil {
		t.Fatalf("open model: %v", err)
	}

	t.Cleanup(func() { _ = m.Close() })

	if err := m.CatchUp(ctx); err != nil {
		t.Fatalf("catch up: %v", err)
	}

	// Walk the ledger in pages of 4: exactly 4+4+1 rows, newest first,
	// no duplicates, empty cursor at the end.
	var (
		paged []readmodel.TaskRow
		cursor string
		pages  int
	)

	for {
		rows, next, err := m.TasksPage(ctx, readmodel.TaskFilter{}, readmodel.TaskPage{Limit: 4, Cursor: cursor})
		if err != nil {
			t.Fatalf("page %d: %v", pages+1, err)
		}

		pages++
		paged = append(paged, rows...)

		if next == "" {
			break
		}

		if pages > total {
			t.Fatal("cursor never terminated — pagination walked past the ledger")
		}

		cursor = next
	}

	if pages != 3 {
		t.Fatalf("walked %d pages, want 3 (4+4+1 over %d rows)", pages, total)
	}

	if len(paged) != total {
		t.Fatalf("paged %d rows, want %d", len(paged), total)
	}

	seen := map[string]bool{}
	for _, r := range paged {
		if seen[r.ID] {
			t.Fatalf("duplicate row %s across pages", r.ID)
		}

		seen[r.ID] = true
	}

	export, err := m.Tasks(ctx, readmodel.TaskFilter{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if len(export) != total {
		t.Fatalf("export path returned %d rows, want %d (export stays unbounded)", len(export), total)
	}

	for i := range paged {
		if paged[i].ID != export[i].ID {
			t.Fatalf("page walk row %d = %s, want export order %s", i, paged[i].ID, export[i].ID)
		}
	}

	// A malformed cursor fails the read (WithCursorString's silent
	// restart-to-page-one is never allowed to lie through this seam).
	if _, _, err := m.TasksPage(ctx, readmodel.TaskFilter{}, readmodel.TaskPage{Limit: 4, Cursor: "!!!not-a-cursor"}); err == nil {
		t.Fatal("malformed cursor accepted — pagination silently restarted")
	}

	// Limit <= 0 takes the default page size: one page over a ledger
	// smaller than the default, empty continuation cursor.
	rows, next, err := m.TasksPage(ctx, readmodel.TaskFilter{}, readmodel.TaskPage{})
	if err != nil {
		t.Fatalf("default page: %v", err)
	}

	if len(rows) != total || next != "" {
		t.Fatalf("default page = %d rows cursor %q, want %d rows and no cursor", len(rows), next, total)
	}

	// The filter composes with pagination: one project-scoped page.
	pagedProject := fmt.Sprintf("web")
	prows, _, err := m.TasksPage(
		ctx,
		readmodel.TaskFilter{Project: &pagedProject},
		readmodel.TaskPage{Limit: total},
	)
	if err != nil {
		t.Fatalf("project page: %v", err)
	}

	if len(prows) != total {
		t.Fatalf("project page = %d rows, want %d", len(prows), total)
	}
}
