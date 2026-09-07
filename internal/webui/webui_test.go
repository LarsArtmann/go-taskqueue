package webui

import (
	"context"
	"encoding/json"
	"fmt"
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

	tk, err := s.Enqueue(context.Background(), task.New{Type: typ, Project: project, Payload: json.RawMessage(`"echo hi"`)})
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
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

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
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/stats", nil))

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

	var wg sync.WaitGroup

	got := make([][]sseEvent, 3)
	for i := range got {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			ch := hub.Subscribe()
			defer hub.Unsubscribe(ch)

			evt := <-ch
			got[i] = append(got[i], sseEvent{typ: evt.Event, id: evt.ID.Get()})
		}(i)
	}

	time.Sleep(20 * time.Millisecond) // let all subscribers register
	hub.Notify(42)
	wg.Wait()

	for i, events := range got {
		if len(events) != 1 || events[0].typ != "tick" || events[0].id != "42" {
			t.Errorf("subscriber %d = %v, want one tick with id 42", i, events)
		}
	}
}

type sseEvent struct {
	typ, id string
}

func TestTailNotifiesOnNewFacts(t *testing.T) {
	srv, s := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
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
		if evt.Event != "tick" {
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

	var fragIDs []string
	var sawTitle bool

	for _, evt := range events {
		switch evt.Type {
		case "frag":
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
		if evt.Type == "frag" && strings.Contains(evt.Data(), tk.ID.String()) {
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

	tailCtx, stopTail := context.WithCancel(context.Background())
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
		if evt.Type == "frag" && strings.Contains(evt.Data(), "live-project") {
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
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/?project=alpha", nil))

	table := tableFragment(rec.Body.String())
	if !strings.Contains(table, "alpha") || strings.Contains(table, "beta") {
		t.Error("project filter did not narrow the table")
	}

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/?status=completed", nil))

	table = tableFragment(rec.Body.String())
	if strings.Contains(table, ">pending<") || strings.Contains(table, "sh") {
		t.Error("status filter did not narrow the table")
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
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/task/"+tk.ID.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "fact timeline") {
		t.Error("detail page missing fact timeline")
	}

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/task/nonexistent", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// TestResumeAfterFactsBacklog: a client that reconnects (with or without a
// valid Last-Event-ID) receives a full snapshot covering current state —
// never a partial patch.
func TestResumeAfterFactsBacklog(t *testing.T) {
	srv, s := newTestServer(t)

	for range 100 {
		enqueue(t, s, "sh", "bulk")
	}

	for _, lastID := range []string{"", "1", "999999"} {
		opts := []ssetest.RequestOption{ssetest.WithPath("/api/events")}
		if lastID != "" {
			opts = append(opts, ssetest.WithLastEventID(lastID))
		}

		events := ssetest.CollectN(t, srv.Handler(), 6, opts...)

		var tableFrag string

		for _, evt := range events {
			if evt.Type == "frag" {
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

	for _, path := range []string{"/static/dashboard.css", "/static/app.js"} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))

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

	for i := range 3 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			_ = ssetest.CollectWithTimeout(t, handler, 100*time.Millisecond, ssetest.WithPath("/api/events"))
		}(i)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		for j := range 10 {
			enqueue(t, s, "sh", fmt.Sprintf("race-%d", j))
		}
	}()

	wg.Wait()
}
