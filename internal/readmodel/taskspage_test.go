package readmodel_test

import (
	"context"
	"encoding/json/jsontext"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
	"github.com/larsartmann/go-taskqueue/internal/task"
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

	const total = 9

	ids := make([]string, 0, total)

	ties := 0

	var lastCreated int64

	for range total {
		tk, err := store.Enqueue(ctx, task.New{
			Project: "web",
			Type:    "sh",
			Payload: jsontext.Value(`"x"`),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		created := tk.CreatedAt.UnixMilli()
		if created == lastCreated {
			ties++
		}

		lastCreated = created

		ids = append(ids, string(tk.ID))
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
		paged  []readmodel.TaskRow
		cursor string
		pages  int
	)

	for {
		rows, next, err := m.TasksPage(ctx, readmodel.TaskFilter{}, readmodel.TaskPage{Limit: 4, Cursor: cursor})
		if err != nil {
			t.Fatalf("page %d: %v", pages+1, err)
		}

		t.Logf("page %d: %d rows, has-next=%v", pages+1, len(rows), next != "")
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
	if _, _, err := m.TasksPage(
		ctx,
		readmodel.TaskFilter{},
		readmodel.TaskPage{Limit: 4, Cursor: "!!!not-a-cursor"},
	); err == nil {
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
	pagedProject := "web"

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

// TestTasksPagePrefetchRepeatWindow pins the repeat-window cache (pareto
// T10, 2026-11-12): a page fetch caches its overflow under the next-page
// cursor key, the walk's following page is served from that cache (no new
// entry — the hit returns early), a fold batch clears the cache so stale
// pages can never outlive the ledger state they were fetched under, and
// the unbounded export path never feeds the cache.
func TestTasksPagePrefetchRepeatWindow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	const (
		total = 12
		limit = 4
	)

	for range total {
		if _, err := store.Enqueue(ctx, task.New{
			Project: "web",
			Type:    "sh",
			Payload: jsontext.Value(`"x"`),
		}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	m, err := readmodel.Open(ctx, t.TempDir()+"/projection.db", store)
	if err != nil {
		t.Fatalf("open model: %v", err)
	}

	t.Cleanup(func() { _ = m.Close() })

	if err := m.CatchUp(ctx); err != nil {
		t.Fatalf("catch up: %v", err)
	}

	if m.PageCacheLen() != 0 {
		t.Fatalf("cache holds %d page(s) before any read, want 0", m.PageCacheLen())
	}

	rows1, cur1, err := m.TasksPage(ctx, readmodel.TaskFilter{}, readmodel.TaskPage{Limit: limit})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}

	if len(rows1) != limit || cur1 == "" {
		t.Fatalf("page 1 = %d rows cursor %q, want %d rows and a cursor", len(rows1), cur1, limit)
	}

	// The 2x fetch cached its overflow under the next-page key.
	if m.PageCacheLen() != 1 {
		t.Fatalf("cache holds %d page(s) after page 1, want 1 (the overflow)", m.PageCacheLen())
	}

	// Page 2 is the repeat window: served from the cache — the hit
	// returns early and caches nothing new, so the count must not move.
	rows2, cur2, err := m.TasksPage(ctx, readmodel.TaskFilter{}, readmodel.TaskPage{Limit: limit, Cursor: cur1})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}

	if len(rows2) != limit || cur2 == "" {
		t.Fatalf("page 2 = %d rows cursor %q, want %d rows and a cursor", len(rows2), cur2, limit)
	}

	if m.PageCacheLen() != 1 {
		t.Fatalf("cache holds %d page(s) after the page-2 hit, want 1 (hit must not Put)", m.PageCacheLen())
	}

	for i := range rows2 {
		if rows1[i].ID == rows2[i].ID {
			t.Fatalf("page 2 row %d repeats page 1 row %s — cursor walked nothing", i, rows2[i].ID)
		}
	}

	// The unbounded export path never feeds the page cache: same row
	// count afterwards, cache still holding exactly the one overflow.
	export, err := m.Tasks(ctx, readmodel.TaskFilter{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if len(export) != total {
		t.Fatalf("export = %d rows, want %d", len(export), total)
	}

	if m.PageCacheLen() != 1 {
		t.Fatalf("cache holds %d page(s) after export, want 1 (export must not touch the cache)", m.PageCacheLen())
	}

	// A fold batch invalidates: after the ledger changes the cached
	// overflow is gone, and the next walk re-fetches fresh.
	if _, err := store.Enqueue(ctx, task.New{
		Project: "web",
		Type:    "sh",
		Payload: jsontext.Value(`"x"`),
	}); err != nil {
		t.Fatalf("enqueue invalidator: %v", err)
	}

	if err := m.CatchUp(ctx); err != nil {
		t.Fatalf("catch up after fold: %v", err)
	}

	if m.PageCacheLen() != 0 {
		t.Fatalf("cache holds %d page(s) after a fold batch, want 0 (cleared)", m.PageCacheLen())
	}

	rows3, cur3, err := m.TasksPage(ctx, readmodel.TaskFilter{}, readmodel.TaskPage{Limit: limit})
	if err != nil {
		t.Fatalf("page 1 after fold: %v", err)
	}

	if len(rows3) != limit || cur3 == "" {
		t.Fatalf("post-fold page 1 = %d rows cursor %q, want %d rows and a cursor", len(rows3), cur3, limit)
	}

	if m.PageCacheLen() != 1 {
		t.Fatalf("cache holds %d page(s) after the post-fold page 1, want 1 (fresh overflow)", m.PageCacheLen())
	}

	if rows3[0].ID == rows1[0].ID {
		t.Fatal("post-fold page served the pre-fold newest row — stale read through the cleared cache")
	}
}
