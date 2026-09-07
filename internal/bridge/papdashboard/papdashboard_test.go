package papdashboard

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// fakeSource serves canned facts and task records.
type fakeSource struct {
	mu    sync.Mutex
	facts []journal.Fact
	tasks map[string]task.Task
}

func (f *fakeSource) Facts(_ context.Context, after int64) ([]journal.Fact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []journal.Fact

	for _, x := range f.facts {
		if x.Seq > after {
			out = append(out, x)
		}
	}

	return out, nil
}

func (f *fakeSource) Get(_ context.Context, id task.ID) (task.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	t, ok := f.tasks[string(id)]
	if !ok {
		return task.Task{}, task.ErrNotFound
	}

	return t, nil
}

func (f *fakeSource) add(fcts ...journal.Fact) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.facts = append(f.facts, fcts...)
}

// recordedIngest is one request the fake PapDashboard captured.
type recordedIngest struct {
	Event          string          `json:"type"`
	AggregateID    string          `json:"aggregateId"`
	Payload        json.RawMessage `json:"payload"`
	IdempotencyKey string
	Authorization  string
}

// fakePap captures ingest calls; failNext makes the next call return 500.
type fakePap struct {
	mu        sync.Mutex
	server    *httptest.Server
	got       []recordedIngest
	failNext5 int
}

func newFakePap(t *testing.T) *fakePap {
	f := &fakePap{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/ingest", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		if f.failNext5 > 0 {
			f.failNext5--

			w.WriteHeader(http.StatusBadGateway)

			return
		}

		body, _ := io.ReadAll(r.Body)

		var rec recordedIngest
		if err := json.Unmarshal(body, &rec); err != nil {
			t.Errorf("bad ingest body: %v", err)
		}

		rec.IdempotencyKey = r.Header.Get("Idempotency-Key")
		rec.Authorization = r.Header.Get("Authorization")
		f.got = append(f.got, rec)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"a1","status":"created","version":1}`))
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)

	return f
}

func (f *fakePap) calls() []recordedIngest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]recordedIngest(nil), f.got...)
}

func (f *fakePap) fail(times int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failNext5 = times
}

func deadLetterFacts() ([]journal.Fact, map[string]task.Task) {
	dead := task.Task{ID: task.ID("t-dead"), Project: "infra", Type: "deploy", Attempts: 3, Status: task.Dead}
	done := task.Task{ID: task.ID("t-done"), Project: "infra", Type: "scrape", Attempts: 1, Status: task.Completed}
	facts := []journal.Fact{
		{Seq: 1, TaskID: "t-done", Type: journal.Completed},
		{Seq: 2, TaskID: "t-dead", Type: journal.DeadLettered, Error: "exit status 1:\nmore detail"},
	}

	return facts, map[string]task.Task{"t-dead": dead, "t-done": done}
}

func forwardAll(t *testing.T, b *Bridge, src *fakeSource) {
	t.Helper()

	facts, err := src.Facts(context.Background(), 0)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}

	for _, f := range facts {
		if err := b.forward(context.Background(), f); err != nil {
			t.Fatalf("forward seq %d: %v", f.Seq, err)
		}
	}
}

func TestDeadLetterBecomesAlert(t *testing.T) {
	pap := newFakePap(t)
	facts, tasks := deadLetterFacts()
	src := &fakeSource{facts: facts, tasks: tasks}
	b := New(src, Config{Endpoint: pap.server.URL, APIKey: "secret", Logger: quietLogger()})

	forwardAll(t, b, src)

	calls := pap.calls()
	if len(calls) != 1 {
		t.Fatalf("got %d ingests, want 1: %+v", len(calls), calls)
	}

	c := calls[0]
	if c.Event != "alert.triggered" {
		t.Errorf("event = %q, want alert.triggered", c.Event)
	}

	if c.AggregateID != "t-dead" {
		t.Errorf("aggregateId = %q, want t-dead", c.AggregateID)
	}

	if c.Authorization != "Bearer secret" {
		t.Errorf("authorization = %q", c.Authorization)
	}

	if c.IdempotencyKey != SourceApp+"-dlq-2" {
		t.Errorf("idempotency key = %q", c.IdempotencyKey)
	}

	var payload struct {
		Severity  string            `json:"severity"`
		Title     string            `json:"title"`
		Body      string            `json:"body"`
		SourceApp string            `json:"sourceApp"`
		Metadata  map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(c.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}

	if payload.Severity != "critical" || payload.SourceApp != SourceApp {
		t.Errorf("severity/sourceApp = %q/%q", payload.Severity, payload.SourceApp)
	}

	if payload.Title != "infra/deploy task t-dead dead-lettered" {
		t.Errorf("title = %q", payload.Title)
	}

	if payload.Metadata["attempts"] != "3" || payload.Metadata["taskType"] != "deploy" {
		t.Errorf("metadata = %+v", payload.Metadata)
	}
}

func TestCompletionAfterAlertResolves(t *testing.T) {
	pap := newFakePap(t)
	facts, tasks := deadLetterFacts()
	facts = append(facts, journal.Fact{Seq: 3, TaskID: "t-dead", Type: journal.Completed})
	src := &fakeSource{facts: facts, tasks: tasks}
	b := New(src, Config{Endpoint: pap.server.URL, Logger: quietLogger()})

	forwardAll(t, b, src)

	calls := pap.calls()
	if len(calls) != 2 {
		t.Fatalf("got %d ingests, want trigger+resolve: %+v", len(calls), calls)
	}

	resolve := calls[1]
	if resolve.Event != "alert.resolved" {
		t.Fatalf("second event = %q, want alert.resolved", resolve.Event)
	}

	var payload struct {
		Title     string `json:"title"`
		SourceApp string `json:"sourceApp"`
	}
	if err := json.Unmarshal(resolve.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}

	if payload.Title != "infra/deploy task t-dead dead-lettered" {
		t.Errorf("resolve title = %q, want the alert title", payload.Title)
	}
}

func TestCompletionWithoutAlertIsSilent(t *testing.T) {
	pap := newFakePap(t)
	facts, tasks := deadLetterFacts()
	src := &fakeSource{facts: facts, tasks: tasks}
	b := New(src, Config{Endpoint: pap.server.URL, Logger: quietLogger()})

	forwardAll(t, b, src)

	if got := len(pap.calls()); got != 1 {
		t.Fatalf("completed task with no alert produced %d extra ingests, want 1 total", got)
	}
}

func TestServerErrorRetriesWithSameIdempotencyKey(t *testing.T) {
	pap := newFakePap(t)
	pap.fail(1)

	facts, tasks := deadLetterFacts()
	src := &fakeSource{facts: facts, tasks: tasks}
	b := New(src, Config{Endpoint: pap.server.URL, Logger: quietLogger()})

	if err := b.forward(context.Background(), facts[1]); err == nil {
		t.Fatal("expected 502 to surface as retryable error")
	}

	if err := b.forward(context.Background(), facts[1]); err != nil {
		t.Fatalf("retry after 502: %v", err)
	}

	calls := pap.calls()
	if len(calls) != 1 {
		t.Fatalf("got %d accepted ingests, want 1", len(calls))
	}

	if calls[0].IdempotencyKey != SourceApp+"-dlq-2" {
		t.Errorf("retry changed idempotency key: %q", calls[0].IdempotencyKey)
	}
}

func TestRunForwardsNewFactsAndStops(t *testing.T) {
	pap := newFakePap(t)
	facts, tasks := deadLetterFacts()
	src := &fakeSource{facts: facts[:1], tasks: tasks}
	b := New(src, Config{Endpoint: pap.server.URL, Logger: quietLogger(), PollInterval: 2 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()

	time.Sleep(20 * time.Millisecond)
	src.add(facts[1])

	deadline := time.After(2 * time.Second)

	for len(pap.calls()) == 0 {
		select {
		case <-deadline:
			t.Fatal("bridge never forwarded the dead-letter fact")
		case <-time.After(2 * time.Millisecond):
		}
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop on cancel")
	}
}

func quietLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// ctxAwareSource behaves like the real store: Facts fails once ctx is done.
type ctxAwareSource struct {
	fakeSource
}

func (s *ctxAwareSource) Facts(ctx context.Context, after int64) ([]journal.Fact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return s.fakeSource.Facts(ctx, after)
}

func TestRunWithCancelledContextReturnsNil(t *testing.T) {
	pap := newFakePap(t)
	facts, tasks := deadLetterFacts()
	src := &ctxAwareSource{fakeSource{facts: facts, tasks: tasks}}
	b := New(src, Config{Endpoint: pap.server.URL, APIKey: "secret", Logger: quietLogger()})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := b.Run(ctx); err != nil {
		t.Fatalf("Run with cancelled ctx = %v, want nil", err)
	}
}
