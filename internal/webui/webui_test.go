package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-sse/ssetest"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/templ-components/display"
)

// newTestStore opens a throwaway SQLite store.
func newTestStore(t *testing.T) *queue.SQLiteStore {
	t.Helper()

	s, err := queue.OpenSQLite(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

func newTestServer(t *testing.T) (*Server, *queue.SQLiteStore) {
	t.Helper()

	s := newTestStore(t)
	srv := New(s, Config{Addr: "127.0.0.1:0", Poll: 20 * time.Millisecond, Heartbeat: 100 * time.Millisecond})

	return srv, s
}

func enqueue(t *testing.T, s *queue.SQLiteStore, typ, project string) task.Task {
	t.Helper()

	tk, err := s.Enqueue(
		context.Background(),
		task.New{Type: typ, Project: project, Payload: json.RawMessage(`"echo hi"`)},
	)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	return tk
}

// waitFor polls cond until true or the timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", what)
}

func TestIndexRendersFragments(t *testing.T) {
	srv, s := newTestServer(t)
	tk := enqueue(t, s, "sh", "demo")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body)
	}

	body := rec.Body.String()

	for _, want := range []string{
		"id=\"frag-stats\"", "id=\"frag-table\"", "id=\"frag-dlq\"", "id=\"frag-feed\"",
		tk.ID.String(), "demo", "sh", "pending",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("index page missing %q", want)
		}
	}
}

func TestStatsJSONShape(t *testing.T) {
	srv, s := newTestServer(t)
	enqueue(t, s, "sh", "demo")
	enqueue(t, s, "sh", "demo")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stats", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var stats map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}

	if stats["pending"] != 2 || stats["total"] != 2 {
		t.Errorf("stats = %v, want pending=2 total=2", stats)
	}
}

func TestHubFanOut(t *testing.T) {
	hub := NewHub()

	var (
		wg    sync.WaitGroup
		gotMu sync.Mutex
	)

	got := [][]sseEvent{nil, nil, nil}
	for i := range got {
		wg.Go(func() {
			ch := hub.Subscribe()
			defer hub.Unsubscribe(ch)

			evt := <-ch

			gotMu.Lock()
			defer gotMu.Unlock()

			got[i] = []sseEvent{{typ: evt.Event, id: evt.ID.Get()}}
		})
	}

	time.Sleep(20 * time.Millisecond) // let all subscribers register
	hub.Notify(42)
	wg.Wait()

	for i, events := range got {
		if len(events) != 1 || events[0].typ != testTickEvent || events[0].id != "42" {
			t.Errorf("subscriber %d = %v, want one tick with id 42", i, events)
		}
	}
}

type sseEvent struct {
	typ, id string
}

const (
	testTickEvent = "tick"
	testFragEvent = "frag"
)

func TestTailNotifiesOnNewFacts(t *testing.T) {
	srv, s := newTestServer(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	tailDone := make(chan struct{})

	go func() {
		_ = srv.tail(ctx)

		close(tailDone)
	}()

	ch := srv.hub.Subscribe()
	defer srv.hub.Unsubscribe(ch)

	time.Sleep(30 * time.Millisecond) // tailer passes the head once
	enqueue(t, s, "sh", "demo")

	select {
	case evt := <-ch:
		if evt.Event != testTickEvent {
			t.Fatalf("event = %q, want tick", evt.Event)
		}

		if evt.ID.String() == "" {
			t.Fatal("tick event has empty id")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no tick within 2s of enqueue")
	}

	cancel()
	<-tailDone
}

// TestSSEFullSnapshotOnConnect verifies the connect-time snapshot: a client
// with zero prior knowledge sees every fragment pre-rendered.
func TestSSEFullSnapshotOnConnect(t *testing.T) {
	srv, s := newTestServer(t)
	tk := enqueue(t, s, "sh", "demo")

	events := ssetest.CollectN(t, srv.Handler(), 6, ssetest.WithPath("/api/events"))

	var (
		fragIDs  []string
		sawTitle bool
	)

	for _, evt := range events {
		switch evt.Type {
		case testFragEvent:
			var frag fragment
			if err := json.Unmarshal([]byte(evt.Data()), &frag); err != nil {
				t.Fatalf("decode frag: %v", err)
			}

			fragIDs = append(fragIDs, frag.ID)
		case "title":
			sawTitle = true
		}
	}

	want := []string{fragStats, fragFilters, fragTable, fragDLQ, fragFeed}
	if len(fragIDs) != len(want) {
		t.Fatalf("frag ids = %v, want %v", fragIDs, want)
	}

	for i, id := range want {
		if fragIDs[i] != id {
			t.Errorf("frag[%d] = %q, want %q", i, fragIDs[i], id)
		}
	}

	if !sawTitle {
		t.Error("no title event in snapshot")
	}

	for _, evt := range events {
		if evt.Type == testFragEvent && strings.Contains(evt.Data(), tk.ID.String()) {
			return // table fragment carries the enqueued task
		}
	}

	t.Error("no fragment contains the enqueued task id")
}

// TestSSELiveUpdateAfterEnqueue drives the full loop: connect a client,
// run the tailer, enqueue a task, and assert a fresh snapshot containing
// the new task reaches the client.
func TestSSELiveUpdateAfterEnqueue(t *testing.T) {
	srv, s := newTestServer(t)

	handler := srv.Handler()

	tailCtx, stopTail := context.WithCancel(t.Context())
	defer stopTail()

	go func() { _ = srv.tail(tailCtx) }()

	done := make(chan []ssetest.Event, 1)
	go func() {
		done <- ssetest.CollectWithTimeout(t, handler, 3*time.Second, ssetest.WithPath("/api/events"))
	}()

	time.Sleep(100 * time.Millisecond) // let the SSE connect

	enqueue(t, s, "sh", "live-project")

	events := <-done

	sawProject := false

	for _, evt := range events {
		if evt.Type == testFragEvent && strings.Contains(evt.Data(), "live-project") {
			sawProject = true
		}
	}

	if !sawProject {
		t.Fatalf("no fragment containing the live-enqueued project; got %d events", len(events))
	}
}

func TestFiltersNarrowTable(t *testing.T) {
	srv, s := newTestServer(t)
	enqueue(t, s, "sh", "alpha")
	enqueue(t, s, "sh", "beta")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?project=alpha", nil))

	table := tableFragment(rec.Body.String())
	if !strings.Contains(table, "alpha") || strings.Contains(table, "beta") {
		t.Error("project filter did not narrow the table")
	}

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?status=completed", nil))

	table = tableFragment(rec.Body.String())
	if strings.Contains(table, ">pending<") || strings.Contains(table, "sh") {
		t.Error("status filter did not narrow the table")
	}
}

// TestStreamSnapshotHonorsFilter pins the /api/events contract app.js relies
// on when it forwards the page's filter query to the EventSource URL: every
// snapshot (initial, live tick, reconnect) renders under the request's
// filter, so a filtered view is never clobbered by the unfiltered table.
func TestStreamSnapshotHonorsFilter(t *testing.T) {
	srv, s := newTestServer(t)
	enqueue(t, s, "sh", "alpha")
	enqueue(t, s, "sh", "beta")

	events := ssetest.CollectN(t, srv.Handler(), 6, ssetest.WithPath("/api/events?project=alpha"))

	var tableFrag string
	for _, evt := range events {
		if evt.Type != testFragEvent {
			continue
		}

		var frag fragment
		if err := json.Unmarshal([]byte(evt.Data()), &frag); err != nil {
			t.Fatalf("decode frag: %v", err)
		}

		if frag.ID == fragTable {
			tableFrag = frag.HTML
		}
	}

	if !strings.Contains(tableFrag, "alpha") || strings.Contains(tableFrag, "beta") {
		t.Error("stream snapshot ignored the project filter")
	}
}

// tableFragment extracts the rendered #frag-table content from a full page.
func tableFragment(body string) string {
	start := strings.Index(body, `id="frag-table"`)
	if start < 0 {
		return ""
	}

	end := strings.Index(body[start:], `id="frag-dlq"`)
	if end < 0 {
		return body[start:]
	}

	return body[start : start+end]
}

func TestTaskDetailAnd404(t *testing.T) {
	srv, s := newTestServer(t)
	tk := enqueue(t, s, "sh", "demo")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/task/"+tk.ID.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "fact timeline") {
		t.Error("detail page missing fact timeline")
	}

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/task/nonexistent", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// TestResumeAfterFactsBacklog: a client that reconnects (with or without a
// valid Last-Event-ID) receives a full snapshot covering current state —
// never a partial patch.
func TestResumeAfterFactsBacklog(t *testing.T) {
	srv, s := newTestServer(t)

	for i := range 100 {
		enqueue(t, s, fmt.Sprintf("bulk-%d", i%3), "bulk")
	}

	for _, lastID := range []string{"", "1", "999999"} {
		opts := []ssetest.RequestOption{ssetest.WithPath("/api/events")}
		if lastID != "" {
			opts = append(opts, ssetest.WithLastEventID(lastID))
		}

		events := ssetest.CollectN(t, srv.Handler(), 6, opts...)

		var tableFrag string

		for _, evt := range events {
			if evt.Type == testFragEvent {
				var frag fragment
				if err := json.Unmarshal([]byte(evt.Data()), &frag); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if frag.ID == fragTable {
					tableFrag = frag.HTML
				}
			}
		}

		if !strings.Contains(tableFrag, "bulk") {
			t.Errorf("last-id %q: table snapshot missing backlog tasks", lastID)
		}
	}
}

func TestStaticAssetsServed(t *testing.T) {
	srv, _ := newTestServer(t)

	for _, path := range []string{"/static/app.css", "/static/app.js", "/static/favicon.svg"} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d", path, rec.Code)
		}
	}
}

// TestConcurrentClientsRace hammers the hub and handlers concurrently under
// -race.
func TestConcurrentClientsRace(t *testing.T) {
	srv, s := newTestServer(t)

	handler := srv.Handler()

	var wg sync.WaitGroup

	for range 3 {
		wg.Go(func() {
			_ = ssetest.CollectWithTimeout(t, handler, 100*time.Millisecond, ssetest.WithPath("/api/events"))
		})
	}

	wg.Go(func() {
		for j := range 10 {
			enqueue(t, s, "sh", fmt.Sprintf("race-%d", j))
		}
	})

	wg.Wait()
}

func TestDLQMirrorsDeadTasks(t *testing.T) {
	srv, s := newTestServer(t)
	tk := enqueue(t, s, "sh", "demo")

	if _, err := s.ClaimDue(context.Background(), "test-owner", time.Minute); err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if err := s.FailPermanent(context.Background(), tk.ID, "test-owner", "boom: permanent failure"); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	body := rec.Body.String()

	dlqStart := strings.Index(body, `id="frag-dlq"`)
	dlqEnd := dlqStart + strings.Index(body[dlqStart:], `id="frag-feed"`)

	dlq := body[dlqStart:dlqEnd]
	if !strings.Contains(dlq, "boom") || !strings.Contains(dlq, tk.ID.String()) {
		t.Errorf("DLQ fragment missing dead task; got: %s", dlq)
	}
}

// TestGoldenFragments pins the skeleton of each fragment for a fixed seed.
func TestGoldenFragments(t *testing.T) {
	srv, s := newTestServer(t)
	enqueue(t, s, "sh", "golden")

	data, err := srv.loadSnapshot(context.Background(), FilterState{})
	if err != nil {
		t.Fatalf("loadSnapshot: %v", err)
	}

	for _, frag := range renderFragments(context.Background(), data) {
		if frag.HTML == "" {
			t.Errorf("fragment %s rendered empty", frag.ID)
		}

		if !strings.HasPrefix(strings.TrimSpace(frag.HTML), "<") {
			t.Errorf("fragment %s is not HTML: %q", frag.ID, frag.HTML)
		}
	}

	stats := renderComponent(context.Background(), StatusCards(data))
	for _, want := range []string{
		"card-pending", "card-running", "card-completed", "card-dead", "card-cancelled", "card-total",
	} {
		if !strings.Contains(stats, want) {
			t.Errorf("stats fragment missing card %s", want)
		}
	}
}

// TestBudgetCardRendersFromSnapshot proves the wiring end to end: a server
// with DailyBudget set projects today's enqueues into the stats fragment.
func TestBudgetCardRendersFromSnapshot(t *testing.T) {
	s := newTestStore(t)
	srv := New(s, Config{Addr: "127.0.0.1:0", Poll: 20 * time.Millisecond, Heartbeat: 100 * time.Millisecond, DailyBudget: 5})
	enqueue(t, s, "sh", "demo")
	enqueue(t, s, "sh", "demo")

	data, err := srv.loadSnapshot(context.Background(), FilterState{})
	if err != nil {
		t.Fatalf("loadSnapshot: %v", err)
	}

	if data.Budget == nil || data.Budget.Cap != 5 || data.Budget.Spent != 2 {
		t.Fatalf("budget = %+v, want cap=5 spent=2", data.Budget)
	}

	stats := renderComponent(context.Background(), StatusCards(data))
	for _, want := range []string{"card-budget", "budget today", "2/5"} {
		if !strings.Contains(stats, want) {
			t.Errorf("stats fragment missing %q", want)
		}
	}
}

// TestBudgetCardAbsentWithoutCap pins the default: no DailyBudget, no card.
func TestBudgetCardAbsentWithoutCap(t *testing.T) {
	srv, s := newTestServer(t)
	enqueue(t, s, "sh", "demo")

	data, err := srv.loadSnapshot(context.Background(), FilterState{})
	if err != nil {
		t.Fatalf("loadSnapshot: %v", err)
	}

	if data.Budget != nil {
		t.Fatalf("budget = %+v, want nil without a configured cap", data.Budget)
	}

	if stats := renderComponent(context.Background(), StatusCards(data)); strings.Contains(stats, "card-budget") {
		t.Error("stats fragment shows budget card without a configured cap")
	}
}

func TestBudgetViewTone(t *testing.T) {
	cases := []struct {
		cap, spent int
		want       display.StatTone
	}{
		{cap: 15, spent: 2, want: display.StatToneGreen},
		{cap: 15, spent: 10, want: display.StatToneGreen},
		{cap: 15, spent: 12, want: display.StatToneYellow},
		{cap: 15, spent: 15, want: display.StatToneRed},
		{cap: 15, spent: 20, want: display.StatToneRed},
	}
	for _, tc := range cases {
		if got := (BudgetView{Cap: tc.cap, Spent: tc.spent}).Tone(); got != tc.want {
			t.Errorf("Tone(cap=%d, spent=%d) = %s, want %s", tc.cap, tc.spent, got, tc.want)
		}
	}
}

func TestReadiness(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		task task.Task
		want string
	}{
		{"no notBefore", task.Task{ID: task.NewID(), Status: task.Pending}, ""},
		{"not pending", task.Task{ID: task.NewID(), Status: task.Running, NotBefore: now.Add(time.Hour)}, ""},
		{"future wait", task.Task{ID: task.NewID(), Status: task.Pending, NotBefore: now.Add(12 * time.Minute)}, "in 12m"},
		{"claimable", task.Task{ID: task.NewID(), Status: task.Pending, NotBefore: now.Add(-time.Minute)}, "ready"},
	}
	for _, tc := range cases {
		if got := readiness(now, tc.task); got != tc.want {
			t.Errorf("%s: readiness = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestReadinessColumnRenders pins the ready column: header present, waiting
// tasks show "in …", claimable ones show "ready", others stay empty.
func TestReadinessColumnRenders(t *testing.T) {
	now := time.Now()
	data := DashboardData{
		Counts: map[task.Status]int{task.Pending: 3},
		Now:    now,
		Tasks: []task.Task{
			{ID: task.NewID(), Status: task.Pending, NotBefore: now.Add(2 * time.Hour), CreatedAt: now, UpdatedAt: now},
			{ID: task.NewID(), Status: task.Pending, NotBefore: now.Add(-time.Minute), CreatedAt: now, UpdatedAt: now},
			{ID: task.NewID(), Status: task.Pending, CreatedAt: now, UpdatedAt: now},
		},
	}

	table := renderComponent(context.Background(), TaskTable(data))
	for _, want := range []string{"ready", "in 2h"} {
		if !strings.Contains(table, want) {
			t.Errorf("task table missing %q", want)
		}
	}
}

// TestTaskDetailSSESnapshot pins the per-task stream's connect snapshot:
// the detail card and fact timeline fragments plus the title watermark
// event, mirroring the /api/events protocol.
func TestTaskDetailSSESnapshot(t *testing.T) {
	srv, s := newTestServer(t)
	tk := enqueue(t, s, "sh", "demo")

	claimed, err := s.ClaimDue(context.Background(), "detail-owner", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if err := s.Complete(context.Background(), claimed.ID, claimed.LeaseOwner, json.RawMessage(`"done"`)); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	events := ssetest.CollectN(t, srv.Handler(), 3, ssetest.WithPath("/task/"+tk.ID.String()+"/events"))

	var fragIDs []string
	sawCompleted := false

	for _, evt := range events {
		if evt.Type != testFragEvent {
			continue
		}

		var frag fragment
		if err := json.Unmarshal([]byte(evt.Data()), &frag); err != nil {
			t.Fatalf("decode frag: %v", err)
		}

		fragIDs = append(fragIDs, frag.ID)

		if strings.Contains(frag.HTML, "completed") {
			sawCompleted = true
		}
	}

	want := []string{fragDetail, fragTimeline}
	if len(fragIDs) != len(want) {
		t.Fatalf("frag ids = %v, want %v", fragIDs, want)
	}

	for i, id := range want {
		if fragIDs[i] != id {
			t.Errorf("frag[%d] = %q, want %q", i, fragIDs[i], id)
		}
	}

	if !sawCompleted {
		t.Error("no detail fragment reflects the completed status")
	}
}

// TestTaskDetailSSELiveUpdate drives the task-scoped loop: a connected
// client sees the card flip to completed without reloading.
func TestTaskDetailSSELiveUpdate(t *testing.T) {
	srv, s := newTestServer(t)
	tk := enqueue(t, s, "sh", "demo")

	claimed, err := s.ClaimDue(context.Background(), "detail-owner", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	tailCtx, stopTail := context.WithCancel(t.Context())
	defer stopTail()

	go func() { _ = srv.tail(tailCtx) }()

	handler := srv.Handler()

	done := make(chan []ssetest.Event, 1)
	go func() {
		done <- ssetest.CollectWithTimeout(t, handler, 3*time.Second, ssetest.WithPath("/task/"+tk.ID.String()+"/events"))
	}()

	time.Sleep(100 * time.Millisecond) // let the SSE connect

	if err := s.Complete(context.Background(), claimed.ID, claimed.LeaseOwner, json.RawMessage(`"done"`)); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	sawCompleted := false

	for _, evt := range <-done {
		if evt.Type == testFragEvent && strings.Contains(evt.Data(), "completed") {
			sawCompleted = true
		}
	}

	if !sawCompleted {
		t.Fatal("task detail stream never reported the completed transition")
	}
}

func TestTaskDetailSSE404(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/task/does-not-exist/events", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown task stream status = %d, want 404", rec.Code)
	}
}

func TestStoreClosedErrorPaths(t *testing.T) {
	s := newTestStore(t)
	srv := New(s, Config{Poll: time.Millisecond, Heartbeat: time.Millisecond})

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stats", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("closed store: /api/stats status = %d, want 500", rec.Code)
	}

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("closed store: / status = %d, want 500", rec.Code)
	}

	events := ssetest.CollectWithTimeout(
		t, srv.Handler(), 300*time.Millisecond, ssetest.WithPath("/api/events"))
	if len(events) != 0 {
		t.Errorf("closed store: got %d SSE events, want 0", len(events))
	}
}

// captureDefaultLogger swaps the default slog logger for a text handler
// writing to the returned buffer, restored on cleanup.
func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &buf
}

func TestRequestLoggingEnabled(t *testing.T) {
	logs := captureDefaultLogger(t)

	srv := New(newTestStore(t), Config{
		Poll: time.Millisecond, Heartbeat: time.Millisecond, RequestLog: true,
	})
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stats", nil))

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/task/does-not-exist", nil))

	out := logs.String()
	for _, want := range []string{
		`msg="webui: request"`, `method=GET`, `path=/api/stats`, `status=200`,
		`path=/task/does-not-exist`, `status=404`, `duration=`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("request log missing %q; log:\n%s", want, out)
		}
	}
}

func TestRequestLoggingOffByDefault(t *testing.T) {
	logs := captureDefaultLogger(t)

	srv, _ := newTestServer(t)
	srv.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/stats", nil))

	if out := logs.String(); out != "" {
		t.Errorf("requests logged with RequestLog disabled:\n%s", out)
	}
}

// TestRequestLoggingKeepsSSEStreaming proves the logging wrapper forwards
// Flush: an SSE client through the wrapped handler still receives the full
// connect snapshot (handleEvents rejects writers without Flush).
func TestRequestLoggingKeepsSSEStreaming(t *testing.T) {
	s := newTestStore(t)
	srv := New(s, Config{Poll: time.Millisecond, Heartbeat: time.Millisecond, RequestLog: true})
	tk := enqueue(t, s, "sh", "demo")

	events := ssetest.CollectN(t, srv.Handler(), 6, ssetest.WithPath("/api/events"))

	sawTask := false

	for _, evt := range events {
		if evt.Type == testFragEvent && strings.Contains(evt.Data(), tk.ID.String()) {
			sawTask = true
		}
	}

	if !sawTask {
		t.Fatalf("SSE snapshot through the request-logging wrapper is missing task %s", tk.ID)
	}
}
