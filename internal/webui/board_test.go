package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-sse/ssetest"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestParseFilterView pins the ?view= allowlist and the board's status
// drop: only the two known projections pass through, and a status filter
// never reaches the board (columns ARE the statuses).
func TestParseFilterView(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		url       string
		wantView  string
		wantEmpty bool // board drops the status filter
	}{
		{name: "no view is the table", url: "/", wantView: viewTable},
		{name: "board passes through", url: "/?view=board", wantView: viewBoard},
		{name: "unknown view falls back to table", url: "/?view=kanban", wantView: viewTable},
		{name: "board drops the status filter", url: "/?view=board&status=dead", wantView: viewBoard, wantEmpty: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := parseFilter(httptest.NewRequest(http.MethodGet, tt.url, nil))

			if f.View != tt.wantView {
				t.Errorf("View = %q, want %q", f.View, tt.wantView)
			}

			if tt.wantEmpty && f.Status != "" {
				t.Errorf("board filter kept status %q, want dropped", f.Status)
			}
		})
	}
}

// TestViewToggleHref pins the toggle's href contract: switching keeps the
// project/query scope, resets sort/page, and the board also drops status.
func TestViewToggleHref(t *testing.T) {
	t.Parallel()

	f := FilterState{Project: "demo", Query: "sh", Sort: "age-desc", Page: 3}

	if got, want := viewToggleHref(f, viewBoard), "/?project=demo&q=sh&view=board"; got != want {
		t.Errorf("toggle to board = %q, want %q", got, want)
	}

	if got, want := viewToggleHref(f, viewTable), "/?project=demo&q=sh"; got != want {
		t.Errorf("toggle to table = %q, want %q", got, want)
	}

	board := FilterState{Project: "demo", View: viewBoard}
	if got, want := viewToggleHref(board, viewTable), "/?project=demo"; got != want {
		t.Errorf("toggle back to table = %q, want %q", got, want)
	}
}

// moveNextTask claims the oldest due task and drives it into the given
// state (complete / dead-letter / leave running under its lease),
// returning the task that moved — ClaimDue picks the victim, the test
// asserts by the returned id, never by enqueue order.
func moveNextTask(t *testing.T, s store, dest task.Status) task.Task {
	t.Helper()

	ctx := context.Background()

	tk, err := s.ClaimDue(ctx, "board-test", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	switch dest {
	case task.Completed:
		if err := s.Complete(ctx, tk.ID, "board-test", json.RawMessage(`{}`)); err != nil {
			t.Fatalf("Complete: %v", err)
		}
	case task.Dead:
		if err := s.FailPermanent(ctx, tk.ID, "board-test", "boom: board test", nil); err != nil {
			t.Fatalf("FailPermanent: %v", err)
		}
	case task.Running:
		// claimed is enough; the lease holds it running
	default:
		t.Fatalf("moveNextTask cannot reach %s", dest)
	}

	return tk
}

// store is the slice of queue.Store the board test helpers need.
type store interface {
	Enqueue(ctx context.Context, n task.New) (task.Task, error)
	ClaimDue(ctx context.Context, owner string, lease time.Duration) (task.Task, error)
	Complete(ctx context.Context, id task.ID, owner string, result json.RawMessage) error
	FailPermanent(ctx context.Context, id task.ID, owner string, errText string, evidence json.RawMessage) error
}

func TestBoardViewRendersColumns(t *testing.T) {
	srv, s := newTestServer(t)

	// Four tasks: one stays pending, one each moves to running, completed,
	// dead; cancelled stays empty for its placeholder assertion.
	for range 4 {
		enqueue(t, s, "sh", "alpha")
	}

	running := moveNextTask(t, s, task.Running)
	completed := moveNextTask(t, s, task.Completed)
	dead := moveNextTask(t, s, task.Dead)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?view=board", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body)
	}

	body := tableFragment(rec.Body.String())

	for _, want := range []string{
		`data-status="pending"`, `data-status="running"`, `data-status="completed"`,
		`data-status="dead"`, `data-status="cancelled"`,
		running.ID.String(), completed.ID.String(), dead.ID.String(),
		"alpha", "sh",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("board missing %q", want)
		}
	}

	// The dead card carries the error tail; the empty cancelled column says so.
	if !strings.Contains(body, "boom: board test") {
		t.Error("dead card missing its error preview")
	}

	if !strings.Contains(body, "empty") {
		t.Error("empty cancelled column missing its placeholder")
	}

	// The view toggle marks the board as current.
	if !strings.Contains(rec.Body.String(), `aria-current="true"`) {
		t.Error("view toggle missing aria-current on the active projection")
	}
}

func TestBoardEmptyQueueShowsEmptyState(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?view=board", nil))

	body := tableFragment(rec.Body.String())
	if !strings.Contains(body, "queue is empty") {
		t.Errorf("empty board should show the empty state, got: %s", body)
	}
}

func TestBoardHonorsProjectFilter(t *testing.T) {
	srv, s := newTestServer(t)
	enqueue(t, s, "sh", "alpha")
	enqueue(t, s, "sh", "beta")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?view=board&project=alpha", nil))

	board := tableFragment(rec.Body.String())
	if !strings.Contains(board, "alpha") || strings.Contains(board, "beta") {
		t.Error("board ignored the project filter")
	}
}

func TestBoardColumnTruncation(t *testing.T) {
	srv, s := newTestServer(t)

	for range boardColumnLimit + 3 {
		enqueue(t, s, "sh", "demo")
	}

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?view=board", nil))

	body := tableFragment(rec.Body.String())

	// The true count is shown even when the cards are truncated.
	if !strings.Contains(body, ">28<") {
		t.Error("pending column does not show the true count 28")
	}

	if !strings.Contains(body, "+3 older") {
		t.Error("truncated column missing the +3 older link")
	}

	if !strings.Contains(body, `href="/?status=pending"`) {
		t.Error("truncation link does not open the status-filtered table view")
	}
}

// TestStreamSnapshotHonorsView pins the /api/events contract app.js relies
// on when it forwards ?view=board: every snapshot renders the board into
// #frag-table, never the table.
func TestStreamSnapshotHonorsView(t *testing.T) {
	srv, s := newTestServer(t)
	enqueue(t, s, "sh", "demo")

	events := ssetest.CollectN(t, srv.Handler(), 6, ssetest.WithPath("/api/events?view=board"))

	var boardFrag string

	for _, evt := range events {
		if evt.Type != testFragEvent {
			continue
		}

		var frag fragment
		if err := json.Unmarshal([]byte(evt.Data()), &frag); err != nil {
			t.Fatalf("decode frag: %v", err)
		}

		if frag.ID == fragTable {
			boardFrag = frag.HTML
		}
	}

	if !strings.Contains(boardFrag, `data-status="pending"`) {
		t.Error("stream snapshot did not render the board for ?view=board")
	}

	if strings.Contains(boardFrag, "<table") {
		t.Error("stream snapshot rendered the table for ?view=board")
	}
}
