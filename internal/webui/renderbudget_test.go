package webui

// Render-budget guard (WebUI/UX review follow-ups 2026-11-28, item 1):
// the 537-row / 26,927-pixel DLQ shipped and stayed because nothing in CI
// could notice an unbounded render. This file pins the server-side truth —
// a seeded journal renders bounded rows everywhere — while
// browser_test.go pins what a real browser actually lays out (document
// height, horizontal overflow). The journal is seeded at incident scale:
// 537 dead tasks with long error messages, 220 pending tasks, and a fact
// journal well past one feed window.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// seedScale mirrors the incident the guard exists for: enough dead rows
// that any unbounded DLQ render blows the budget, and enough pending work
// that the paged table and the active strip both fill.
const (
	seedDeadTasks    = 537
	seedPendingTasks = 220
)

// seedLongError is longer than the 38ch error-cell clamp so an unbounded
// render of the FULL message is visible in any width/height budget.
const seedLongError = "boom: permanent failure while reconciling the ledger after a very long data-less window — this message is deliberately far longer than the clamp"

// seedBudgetJournal fills the store with 537 dead and 220 pending tasks,
// each enqueue/fail appending facts so the feed window overflows too.
func seedBudgetJournal(t *testing.T, s *sqlite.Store) {
	t.Helper()

	ctx := context.Background()

	for i := range seedDeadTasks {
		tk, err := s.Enqueue(ctx, task.New{
			Type:        "agent",
			Project:     "demo",
			Payload:     []byte(`"echo hi"`),
			MaxAttempts: 1,
		})
		if err != nil {
			t.Fatalf("Enqueue dead %d: %v", i, err)
		}

		_, claim, err := s.ClaimDue(ctx, "budget-seed", time.Minute)
		if err != nil {
			t.Fatalf("ClaimDue dead %d: %v", i, err)
		}

		if err := s.Fail(ctx, tk.ID, claim, seedLongError, 0, nil); err != nil {
			t.Fatalf("Fail dead %d: %v", i, err)
		}
	}

	for i := range seedPendingTasks {
		if _, err := s.Enqueue(ctx, task.New{
			Type:    "sh",
			Project: "demo",
			Payload: []byte(`"true"`),
		}); err != nil {
			t.Fatalf("Enqueue pending %d: %v", i, err)
		}
	}
}

// seededFixtureServer starts a full webui server over a seeded journal and
// returns it plus one dead task id (for the detail page).
func seededFixtureServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	srv, s := newTestServer(t)
	seedBudgetJournal(t, s)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	id := firstTaskHref(t, rec.Body.String(), `id="frag-dlq"`, `id="sec-metrics"`)

	return ts, id
}

var taskHrefRe = regexp.MustCompile(`/task/([0-9a-f]+)`)

// firstTaskHref returns the first /task/ href inside a body slice.
func firstTaskHref(t *testing.T, body, startMarker, endMarker string) string {
	t.Helper()

	start := strings.Index(body, startMarker)
	if start < 0 {
		t.Fatalf("marker %q not found in page", startMarker)
	}

	rest := body[start:]
	if end := strings.Index(rest, endMarker); end >= 0 {
		rest = rest[:end]
	}

	m := taskHrefRe.FindStringSubmatch(rest)
	if m == nil {
		t.Fatalf("no /task/ href inside %q slice", startMarker)
	}

	return m[1]
}

// taskHrefs returns the DISTINCT task ids linked inside a body slice. One
// row renders exactly one /task/ link, so distinct hrefs == rendered rows.
func taskHrefs(body, startMarker, endMarker string) []string {
	start := strings.Index(body, startMarker)
	if start < 0 {
		return nil
	}

	rest := body[start:]
	if end := strings.Index(rest, endMarker); end >= 0 {
		rest = rest[:end]
	}

	seen := make(map[string]struct{})
	ids := make([]string, 0, 64)

	for _, m := range taskHrefRe.FindAllStringSubmatch(rest, -1) {
		if _, ok := seen[m[1]]; ok {
			continue
		}

		seen[m[1]] = struct{}{}
		ids = append(ids, m[1])
	}

	return ids
}

// countFeedLines counts rendered fact-feed lines in the frag-feed slice.
func countFeedLines(body string) int {
	start := strings.Index(body, `id="frag-feed"`)
	if start < 0 {
		return -1
	}

	return strings.Count(body[start:], `data-fact-type`)
}

// TestRenderBudgetSeededCounts pins the server-side half of the render
// budget on the incident-scale seeded journal: the DLQ window stays at
// deadTableLimit rows, the paged task table at taskTableLimit rows (the
// active strip adds at most activeStripLimit more), and the fact feed at
// exactly factFeedLen lines. Rows are counted as distinct /task/ hrefs —
// one row renders one link, headers render none.
func TestRenderBudgetSeededCounts(t *testing.T) {
	srv, s := newTestServer(t)
	seedBudgetJournal(t, s)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: rc=%d", rec.Code)
	}

	body := rec.Body.String()

	table := taskHrefs(body, `id="frag-table"`, `id="sec-dlq"`)
	boundTable := taskTableLimit + activeStripLimit
	if len(table) > boundTable {
		t.Errorf("task table rendered %d rows, bound is %d (paged page %d + active strip %d)",
			len(table), boundTable, taskTableLimit, activeStripLimit)
	}

	dlq := taskHrefs(body, `id="frag-dlq"`, `id="sec-metrics"`)
	if len(dlq) > deadTableLimit {
		t.Errorf("DLQ rendered %d rows, bound is deadTableLimit=%d", len(dlq), deadTableLimit)
	}

	if feed := countFeedLines(body); feed != factFeedLen {
		t.Errorf("fact feed rendered %d lines, want exactly factFeedLen=%d", feed, factFeedLen)
	}

	// The seeded journal is incident-scale: the bounds must BIND (a seed
	// too small would pass an unbounded renderer too).
	if len(dlq) != deadTableLimit {
		t.Errorf("seed must saturate the DLQ window to make the bound meaningful: got %d rows, want %d", len(dlq), deadTableLimit)
	}

	// Board and detail pages must render too (they ride the same journal).
	for _, path := range []string{"/?view=board", "/task/" + dlq[0]} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: rc=%d", path, rec.Code)
		}
	}

	if !slices.Contains(table, dlq[0]) {
		t.Errorf("dead task %s missing from the paged table (severity order should rank dead first)", dlq[0])
	}
}
