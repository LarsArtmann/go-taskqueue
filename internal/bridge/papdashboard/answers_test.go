package papdashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// --- forward: task.question-asked → PapDashboard question.asked ---

func questionAskedFacts() journal.Fact {
	return journal.Fact{
		Seq:    7,
		TaskID: "t-ask",
		Type:   journal.QuestionAsked,
		Detail: jsonDetail(queue.QuestionAskedDetail{
			Ref:       "q-1",
			Type:      queue.QuestionTypeConfirmation,
			Question:  "Ship as v3 now?\nSecond line of context.",
			Options:   []string{"ship v3", "stay on v2"},
			ExpiresAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC).UnixMilli(),
		}),
	}
}

func jsonDetail(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return raw
}

func TestQuestionAskedForwardsToPap(t *testing.T) {
	pap := newFakePap(t)
	b := New(papSource(t, questionAskedFacts()), nil, Config{Endpoint: pap.server.URL, APIKey: "k"})
	forwardAll(t, b, papSource(t, questionAskedFacts()))

	calls := pap.calls()
	if len(calls) != 1 {
		t.Fatalf("ingest calls = %d, want 1", len(calls))
	}

	call := calls[0]
	if call.Event != "question.asked" {
		t.Errorf("event = %s, want question.asked", call.Event)
	}

	if call.AggregateID != "t-ask" {
		t.Errorf("aggregateId = %s, want t-ask", call.AggregateID)
	}

	if call.IdempotencyKey != "go-taskqueue-question-7" {
		t.Errorf("idempotency key = %s, want go-taskqueue-question-7", call.IdempotencyKey)
	}

	if call.Authorization != "Bearer k" {
		t.Errorf("authorization = %q", call.Authorization)
	}

	var payload struct {
		Type      string `json:"type"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		SourceApp string `json:"sourceApp"`
		ExpiresAt string `json:"expiresAt"`
	}
	if err := json.Unmarshal(call.Payload, &payload); err != nil {
		t.Fatalf("payload: %v (%s)", err, call.Payload)
	}

	if payload.Type != "confirmation" {
		t.Errorf("question type = %s, want confirmation", payload.Type)
	}

	if payload.Title != "Ship as v3 now?" {
		t.Errorf("title = %q, want the asked first line", payload.Title)
	}

	for _, want := range []string{"task:t-ask", "qref:q-1", "Ship as v3 now?", "- ship v3", "- stay on v2"} {
		if !strings.Contains(payload.Body, want) {
			t.Errorf("body missing %q:\n%s", want, payload.Body)
		}
	}

	// Correlation tokens lead the body: a truncated question can never
	// sever the answer's route home.
	if !strings.HasPrefix(payload.Body, "task:t-ask\nqref:q-1\n") {
		t.Errorf("correlation tokens must lead the body, got %q", payload.Body)
	}

	if payload.SourceApp != "go-taskqueue" {
		t.Errorf("sourceApp = %s", payload.SourceApp)
	}

	if payload.ExpiresAt != "2026-09-20T12:00:00Z" {
		t.Errorf("expiresAt = %s, want RFC3339", payload.ExpiresAt)
	}
}

func TestQuestionAskedWithoutRefIsSkipped(t *testing.T) {
	pap := newFakePap(t)

	fact := journal.Fact{
		Seq:    8,
		TaskID: "t-ask",
		Type:   journal.QuestionAsked,
		Detail: jsonDetail(queue.QuestionAskedDetail{Ref: "", Question: "no ref"}),
	}

	b := New(papSource(t, fact), nil, Config{Endpoint: pap.server.URL, APIKey: "k"})
	forwardAll(t, b, papSource(t, fact))

	if calls := pap.calls(); len(calls) != 0 {
		t.Fatalf("malformed question forwarded %d calls, want 0", len(calls))
	}
}

func TestQuestionAskedUnparseableDetailIsSkipped(t *testing.T) {
	pap := newFakePap(t)

	fact := journal.Fact{Seq: 9, TaskID: "t-ask", Type: journal.QuestionAsked, Detail: json.RawMessage("{bad")}

	b := New(papSource(t, fact), nil, Config{Endpoint: pap.server.URL, APIKey: "k"})
	forwardAll(t, b, papSource(t, fact))

	if calls := pap.calls(); len(calls) != 0 {
		t.Fatalf("unparseable question forwarded %d calls, want 0", len(calls))
	}
}

// --- AnswerPoller ---

type fakeAnswerStore struct {
	mu       sync.Mutex
	recorded []queue.AnswerRecord
	onTasks  []string
	err      error
}

func (f *fakeAnswerStore) RecordAnswer(_ context.Context, id task.ID, ans queue.AnswerRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	f.recorded = append(f.recorded, ans)
	f.onTasks = append(f.onTasks, id.String())

	return nil
}

// fakeQuestionAPI serves GET /api/questions with canned pages and logs
// every request's query + auth.
type fakeQuestionAPI struct {
	mu        sync.Mutex
	server    *httptest.Server
	questions []papQuestion
	requests  []string
	listErr   int
}

func newFakeQuestionAPI(t *testing.T, questions []papQuestion) *fakeQuestionAPI {
	t.Helper()

	f := &fakeQuestionAPI{questions: questions}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/questions", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		f.requests = append(f.requests, r.URL.Query().Encode())

		if r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		if f.listErr != 0 {
			w.WriteHeader(f.listErr)

			return
		}

		limit, offset := 50, 0
		if v := r.URL.Query().Get("limit"); v != "" {
			fmt.Sscanf(v, "%d", &limit)
		}

		if v := r.URL.Query().Get("offset"); v != "" {
			fmt.Sscanf(v, "%d", &offset)
		}

		end := min(offset+limit, len(f.questions))
		if offset > len(f.questions) {
			offset = len(f.questions)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"body": map[string]any{"data": f.questions[min(offset, len(f.questions)):end]},
		})
	})

	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)

	return f
}

func (f *fakeQuestionAPI) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.requests)
}

func answeredQuestion(id, body, answer string, answeredAt time.Time) papQuestion {
	return papQuestion{
		ID: id, Title: "t", Body: body, SourceApp: "go-taskqueue",
		IsAnswered: true, Answer: answer, AnsweredAt: answeredAt,
	}
}

// errStoreDown is the fake store's failure for the record-error retry test
// (static: err113 wants no dynamic error constructors).
var errStoreDown = errors.New("store down")

func correlatedBody(taskID, ref, question string) string {
	return fmt.Sprintf("task:%s\nqref:%s\n\n%s", taskID, ref, question)
}

func TestParseQuestionCorrelation(t *testing.T) {
	body := correlatedBody("0001abcDEF01234567", "q-9", "Which module?")

	taskID, ref, ok := parseQuestionCorrelation(body)
	if !ok || taskID != "0001abcDEF01234567" || ref != "q-9" {
		t.Fatalf("parse = %q/%q/%v, want the task id and q-9", taskID, ref, ok)
	}

	if _, _, ok := parseQuestionCorrelation("no tokens at all, task: mentioned inline"); ok {
		t.Error("prose mentioning task: must not correlate")
	}

	if _, _, ok := parseQuestionCorrelation("task:0001abc but no ref line"); ok {
		t.Error("missing qref must not correlate")
	}
}

func TestAnswerPollerRoutesAnswerHome(t *testing.T) {
	at := time.Now().Add(-time.Minute)

	api := newFakeQuestionAPI(t, []papQuestion{
		answeredQuestion("pap-1", correlatedBody("0001task0000aaaa", "q-1", "Ship v3?"), "Stay on v2.", at),
	})

	store := &fakeAnswerStore{}
	wm := newFakeWatermarks()

	p := NewAnswerPoller(store, wm, AnswerConfig{Endpoint: api.server.URL, APIKey: "k", Interval: time.Hour})
	p.client = api.server.Client()

	// Bootstrap-at-now would skip a minute-old answer; seed the checkpoint
	// to simulate an already-running poller.
	seed := at.Add(-time.Hour).UnixNano()
	if err := wm.SaveWatermark(context.Background(), p.consumerKey(), seed); err != nil {
		t.Fatal(err)
	}

	next, err := p.pollOnce(context.Background(), time.Unix(0, seed))
	if err != nil {
		t.Fatalf("pollOnce: %v", err)
	}

	if len(store.recorded) != 1 {
		t.Fatalf("recorded answers = %d, want 1", len(store.recorded))
	}

	got := store.recorded[0]
	if got.Ref != "q-1" || got.Answer != "Stay on v2." || got.PapID != "pap-1" {
		t.Errorf("record = %+v", got)
	}

	if store.onTasks[0] != "0001task0000aaaa" {
		t.Errorf("routed to task %s, want 0001task0000aaaa", store.onTasks[0])
	}

	if !got.AnsweredAt.Equal(at) {
		t.Errorf("answeredAt = %v, want %v", got.AnsweredAt, at)
	}

	if !next.Equal(at) {
		t.Errorf("cursor = %v, want the answer's AnsweredAt %v", next, at)
	}

	if seq := wm.current(p.consumerKey()); seq != at.UnixNano() {
		t.Errorf("checkpoint = %d, want %d", seq, at.UnixNano())
	}
}

func TestAnswerPollerSkipsForeignAndUnanswered(t *testing.T) {
	now := time.Now()

	api := newFakeQuestionAPI(t, []papQuestion{
		{ID: "pap-u", SourceApp: "go-taskqueue"}, // unanswered
		answeredQuestion("pap-f", "operator question, no tokens", "manual answer", now.Add(-time.Minute)),
	})

	store := &fakeAnswerStore{}
	wm := newFakeWatermarks()

	p := NewAnswerPoller(store, wm, AnswerConfig{Endpoint: api.server.URL, APIKey: "k"})
	p.client = api.server.Client()

	old := now.Add(-time.Hour)

	next, err := p.pollOnce(context.Background(), old)
	if err != nil {
		t.Fatalf("pollOnce: %v", err)
	}

	if len(store.recorded) != 0 {
		t.Fatalf("foreign/unanswered recorded: %+v", store.recorded)
	}

	// The foreign ANSWERED question still advances the cursor: it is
	// consumed (logged skip), never re-read.
	if !next.After(old) {
		t.Errorf("cursor did not advance past consumed questions: %v", next)
	}
}

func TestAnswerPollerBootstrapSkipsHistory(t *testing.T) {
	api := newFakeQuestionAPI(t, []papQuestion{
		answeredQuestion("pap-old", correlatedBody("t-1", "q-1", "old"), "answer", time.Now().Add(-time.Hour)),
	})

	store := &fakeAnswerStore{}

	p := NewAnswerPoller(store, nil, AnswerConfig{Endpoint: api.server.URL, APIKey: "k"})
	p.client = api.server.Client()

	cursor, err := p.startCursor(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// First start bootstraps at NOW: the hour-old answer is history.
	if _, err := p.pollOnce(context.Background(), cursor); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}

	if len(store.recorded) != 0 {
		t.Fatalf("history replayed on bootstrap: %+v", store.recorded)
	}
}

func TestAnswerPollerStopsPagingWhenPageIsOld(t *testing.T) {
	old := time.Now().Add(-2 * time.Hour)

	page := make([]papQuestion, 0, answerPageLimit)
	for i := range answerPageLimit {
		page = append(page, answeredQuestion(
			fmt.Sprintf("pap-%d", i),
			correlatedBody("0001task0000aaaa", fmt.Sprintf("q-%d", i), "old"),
			"a", old))
	}

	api := newFakeQuestionAPI(t, page)
	store := &fakeAnswerStore{}

	p := NewAnswerPoller(store, nil, AnswerConfig{Endpoint: api.server.URL, APIKey: "k"})
	p.client = api.server.Client()

	// The cursor sits AFTER the page: everything on it is old, so the
	// (newest-first) API has nothing deeper — stop after one request.
	if _, err := p.pollOnce(context.Background(), old.Add(time.Minute)); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}

	if n := api.requestCount(); n != 1 {
		t.Fatalf("list requests = %d, want 1 (a page with nothing newer must stop paging)", n)
	}
}

// TestAnswerPollerBoundsAnswersPerPoll pins the stampede guard: a poll
// with MaxAnswersPerPoll=2 over four fresh answers applies exactly the two
// OLDEST, the take-max cursor lands on the newest APPLIED answer (the
// applied/remainder boundary), and the next poll drains the newer
// remainder in order — none lost, none applied twice.
func TestAnswerPollerBoundsAnswersPerPoll(t *testing.T) {
	t.Parallel()

	base := time.Now().Add(-time.Hour)

	// The API serves newest-first; four answers a minute apart.
	api := newFakeQuestionAPI(t, []papQuestion{
		answeredQuestion("pap-3", correlatedBody("0001task00000003", "q-3", "q?"), "a3", base.Add(3*time.Minute)),
		answeredQuestion("pap-2", correlatedBody("0001task00000002", "q-2", "q?"), "a2", base.Add(2*time.Minute)),
		answeredQuestion("pap-1", correlatedBody("0001task00000001", "q-1", "q?"), "a1", base.Add(1*time.Minute)),
		answeredQuestion("pap-0", correlatedBody("0001task00000000", "q-0", "q?"), "a0", base),
	})

	store := &fakeAnswerStore{}
	wm := newFakeWatermarks()

	p := NewAnswerPoller(store, wm, AnswerConfig{
		Endpoint:          api.server.URL,
		APIKey:            "k",
		MaxAnswersPerPoll: 2,
	})
	p.client = api.server.Client()

	if p.cfg.MaxAnswersPerPoll != 2 {
		t.Fatalf("explicit bound lost: %d", p.cfg.MaxAnswersPerPoll)
	}

	cursor := base.Add(-time.Minute)

	next, err := p.pollOnce(context.Background(), cursor)
	if err != nil {
		t.Fatalf("first pollOnce: %v", err)
	}

	if len(store.recorded) != 2 {
		t.Fatalf("first poll applied %d answers, want the bound 2", len(store.recorded))
	}

	if store.recorded[0].Ref != "q-0" || store.recorded[1].Ref != "q-1" {
		t.Fatalf("first poll applied %s then %s, want the two oldest (q-0, q-1)",
			store.recorded[0].Ref, store.recorded[1].Ref)
	}

	// The cursor stops at the newest APPLIED answer — the applied
	// remainder boundary — never past the unapplied newer tail.
	if !next.Equal(base.Add(1 * time.Minute)) {
		t.Fatalf("cursor = %v, want the newest applied answer's time %v", next, base.Add(1*time.Minute))
	}

	if seq := wm.current(p.consumerKey()); seq != next.UnixNano() {
		t.Fatalf("checkpoint = %d, want %d", seq, next.UnixNano())
	}

	next2, err := p.pollOnce(context.Background(), next)
	if err != nil {
		t.Fatalf("second pollOnce: %v", err)
	}

	if len(store.recorded) != 4 {
		t.Fatalf("second poll total = %d, want the full backlog 4", len(store.recorded))
	}

	if store.recorded[2].Ref != "q-2" || store.recorded[3].Ref != "q-3" {
		t.Fatalf("second poll applied %s then %s, want the remainder in order (q-2, q-3)",
			store.recorded[2].Ref, store.recorded[3].Ref)
	}

	if !next2.Equal(base.Add(3 * time.Minute)) {
		t.Fatalf("final cursor = %v, want %v", next2, base.Add(3*time.Minute))
	}
}

func TestAnswerPollerRecordErrorKeepsCursor(t *testing.T) {
	at := time.Now().Add(-time.Minute)

	api := newFakeQuestionAPI(t, []papQuestion{
		answeredQuestion("pap-1", correlatedBody("0001task0000aaaa", "q-1", "?"), "a", at),
	})

	boom := errStoreDown
	store := &fakeAnswerStore{err: boom}
	wm := newFakeWatermarks()

	p := NewAnswerPoller(store, wm, AnswerConfig{Endpoint: api.server.URL, APIKey: "k"})
	p.client = api.server.Client()

	before := at.Add(-time.Hour)

	if _, err := p.pollOnce(context.Background(), before); !errors.Is(err, boom) {
		t.Fatalf("pollOnce err = %v, want the store error", err)
	}

	if wm.saves != 0 {
		t.Error("failed batch must not checkpoint")
	}

	// Store recovers: the same answer is picked up again (at-least-once).
	store.err = nil

	if _, err := p.pollOnce(context.Background(), before); err != nil {
		t.Fatalf("retry pollOnce: %v", err)
	}

	if len(store.recorded) != 1 {
		t.Fatalf("recorded after retry = %d, want 1", len(store.recorded))
	}
}

func TestAnswerPollerListErrorSurfaces(t *testing.T) {
	api := newFakeQuestionAPI(t, nil)
	api.listErr = http.StatusInternalServerError

	p := NewAnswerPoller(&fakeAnswerStore{}, nil, AnswerConfig{Endpoint: api.server.URL, APIKey: "k"})
	p.client = api.server.Client()

	if _, err := p.pollOnce(context.Background(), time.Now()); err == nil {
		t.Fatal("HTTP 500 must surface as an error, not a silent skip")
	}
}

// papSource builds the fake fact source for the question-forward tests.
func papSource(t *testing.T, facts ...journal.Fact) *fakeSource {
	t.Helper()

	src := &fakeSource{tasks: map[string]task.Task{}}
	src.add(facts...)

	return src
}
