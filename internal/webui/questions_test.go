package webui

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// parkOnQuestionWebui mirrors the conform suite's parkOnQuestion setup:
// claim a task, record a question-asked fact, requeue without attempt
// burn on the question class — the exact shape `tq ask` + the worker's
// QuestionPendingError park leaves behind.
func parkOnQuestionWebui(t *testing.T, s *sqlite.Store, ref, question string) task.Task {
	t.Helper()

	ctx := context.Background()

	tk, err := s.Enqueue(ctx, task.New{
		Type:    "agent",
		Project: "demo",
		Payload: json.RawMessage(`{"prompt":"do the thing"}`),
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claimed, claim, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil || claimed.ID != tk.ID {
		t.Fatalf("ClaimDue: claimed=%v err=%v", claimed.ID, err)
	}

	if err := s.AppendFact(ctx, journal.Fact{
		TaskID: tk.ID.String(),
		Type:   journal.QuestionAsked,
		Detail: []byte(`{"ref":"` + ref + `","type":"confirmation","question":"` + question + `","options":["ship it","hold"]}`),
	}); err != nil {
		t.Fatalf("append asked fact: %v", err)
	}

	if err := s.Requeue(ctx, tk.ID, claim, "question pending: "+ref, time.Hour, false, queue.RequeueClassQuestion, ""); err != nil {
		t.Fatalf("park requeue: %v", err)
	}

	return tk
}

// TestOpenQuestionsDerivation pins the pane's data seam: a parked task
// with an unanswered question-asked fact yields one QuestionView; the
// matching question-answered fact retires it (the AnswerPoller's and the
// pane's home seam share the ref pairing).
func TestOpenQuestionsDerivation(t *testing.T) {
	s := newTestStore(t)
	tk := parkOnQuestionWebui(t, s, "q-1", "Ship as v3 now?")

	ctx := context.Background()

	open := openQuestions(ctx, s)
	if len(open) != 1 {
		t.Fatalf("openQuestions = %d rows, want 1", len(open))
	}

	got := open[0]
	if got.TaskID != tk.ID || got.Ref != "q-1" || got.Question != "Ship as v3 now?" {
		t.Fatalf("row = %+v", got)
	}
	if got.Type != queue.QuestionTypeConfirmation || len(got.Options) != 2 {
		t.Fatalf("row type/options = %q/%v", got.Type, got.Options)
	}

	if err := s.RecordAnswer(ctx, tk.ID, queue.AnswerRecord{Ref: "q-1", Answer: "ship it", AnsweredAt: time.Now()}); err != nil {
		t.Fatalf("RecordAnswer: %v", err)
	}

	if open := openQuestions(ctx, s); len(open) != 0 {
		t.Fatalf("openQuestions after answer = %d rows, want 0", len(open))
	}
}

// TestQuestionsPaneRendersAnswerForm pins the pane's render contract:
// with AllowWrites the answer form (action, ref, csrf) renders; without
// it the pane stays read-only with the CLI hint; with no open questions
// the pane renders nothing at all.
func TestQuestionsPaneRendersAnswerForm(t *testing.T) {
	s := newTestStore(t)
	tk := parkOnQuestionWebui(t, s, "q-1", "Ship as v3 now?")

	data := DashboardData{
		Questions:   openQuestions(context.Background(), s),
		Now:         time.Now(),
		AllowWrites: true,
	}

	var sb strings.Builder
	if err := QuestionsPane(data).Render(context.WithValue(context.Background(), csrfCtxKey{}, "tok"), &sb); err != nil {
		t.Fatalf("render pane: %v", err)
	}

	html := sb.String()
	for _, want := range []string{"open questions", "Ship as v3 now?", "ship it", "hold"} {
		if !strings.Contains(html, want) {
			t.Fatalf("pane missing %q", want)
		}
	}
	if !strings.Contains(html, `action="/task/`+tk.ID.String()+`/answer"`) {
		t.Fatalf("pane missing answer action: %s", html)
	}
	if !strings.Contains(html, `name="ref" value="q-1"`) {
		t.Fatalf("pane missing hidden ref: %s", html)
	}
	if !strings.Contains(html, `name="csrf" value="tok"`) {
		t.Fatalf("pane missing csrf field: %s", html)
	}

	data.AllowWrites = false
	sb.Reset()
	if err := QuestionsPane(data).Render(context.Background(), &sb); err != nil {
		t.Fatalf("render read-only pane: %v", err)
	}

	readOnly := sb.String()
	if strings.Contains(readOnly, "<form") {
		t.Fatalf("read-only pane must not carry a form: %s", readOnly)
	}
	if !strings.Contains(readOnly, "--allow-writes") {
		t.Fatalf("read-only pane missing the CLI hint: %s", readOnly)
	}

	sb.Reset()
	if err := QuestionsPane(DashboardData{Now: time.Now()}).Render(context.Background(), &sb); err != nil {
		t.Fatalf("render empty pane: %v", err)
	}

	if strings.TrimSpace(sb.String()) != "" {
		t.Fatalf("empty pane must render nothing, got %q", sb.String())
	}
}

// TestRenderFragmentsCarryQuestionsPane pins the SSE contract: every
// dashboard fragment burst includes #frag-questions, so an answered
// question disappears from the pane live.
func TestRenderFragmentsCarryQuestionsPane(t *testing.T) {
	frags := renderFragments(context.Background(), DashboardData{})

	found := false
	for _, f := range frags {
		if f.ID == fragQuestions {
			found = true
		}
	}
	if !found {
		t.Fatalf("renderFragments missing %s: %+v", fragQuestions, frags)
	}
}

// TestWriteFlowAnswerQuestion exercises the answer endpoint end to end:
// CSRF challenge, ref staleness guard (409 on unknown/lapsed ref), the
// happy path routing through RecordAnswer (payload answered-injection +
// park clear + question-answered fact), and the required-field 400.
func TestWriteFlowAnswerQuestion(t *testing.T) {
	s := newTestStore(t)
	tk := parkOnQuestionWebui(t, s, "q-1", "Ship as v3 now?")

	srv := New(s, Config{
		Poll: time.Millisecond, Heartbeat: time.Millisecond,
		AuthToken: "sekrit", AllowWrites: true,
	})
	handler := srv.Handler()

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/?token=sekrit", nil))

	csrf := csrfFrom(first)
	if csrf == "" {
		t.Fatal("no CSRF cookie issued on page GET")
	}

	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/task/"+tk.ID.String()+"/answer", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer sekrit")

		for _, c := range first.Result().Cookies() {
			req.AddCookie(c)
		}

		handler.ServeHTTP(rec, req)

		return rec
	}

	// Wrong ref: 409, nothing recorded.
	if rec := post("csrf=" + csrf + "&ref=q-other&answer=nope"); rec.Code != http.StatusConflict {
		t.Fatalf("unknown ref: status = %d, want 409", rec.Code)
	}

	// Missing answer: 400.
	if rec := post("csrf=" + csrf + "&ref=q-1&answer="); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing answer: status = %d, want 400", rec.Code)
	}

	// Happy path: 303 redirect, answer routed home.
	rec := post("csrf=" + csrf + "&ref=q-1&answer=ship+it")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("answer: status = %d, want 303 (body %s)", rec.Code, rec.Body.String())
	}

	ctx := context.Background()

	got, err := s.Get(ctx, tk.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(string(got.Payload), `"answered"`) || !strings.Contains(string(got.Payload), "ship it") {
		t.Fatalf("payload after answer = %s, want answered ship it", got.Payload)
	}
	if got.NotBefore.After(time.Now().Add(time.Minute)) {
		t.Fatalf("park not cleared: not_before = %v (still ~an hour out)", got.NotBefore)
	}

	facts, err := s.FactsForTask(ctx, tk.ID.String(), 0)
	if err != nil {
		t.Fatalf("FactsForTask: %v", err)
	}

	answered := false
	for _, f := range facts {
		if f.Type == journal.QuestionAnswered && strings.Contains(string(f.Detail), "ship it") {
			answered = true
		}
	}
	if !answered {
		t.Fatalf("no question-answered fact carrying the answer: %v", facts)
	}

	// Answering again (stale pane) is a 409, not a second answer.
	if rec := post("csrf=" + csrf + "&ref=q-1&answer=again"); rec.Code != http.StatusConflict {
		t.Fatalf("re-answer: status = %d, want 409", rec.Code)
	}
}

// TestAgentFeedAttachesBeforeAnyOtherCall pins the M6 ordering contract
// (the row-1445 precondition): the events attach is the FIRST
// workspace-scoped request the feed collector makes. Crush delivers
// prompts only to SSE-attached clients whose current-session views the
// session — attach FIRST, then current-session, else 409 — so any future
// prompt-viewing call wired into attachWorkspace must never precede the
// attach. The fake server records every workspace-scoped request path in
// arrival order and the test asserts the events attach leads the log.
func TestAgentFeedAttachesBeforeAnyOtherCall(t *testing.T) {
	var mu sync.Mutex
	var calls []string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/workspaces", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaces":[{"id":"ws-1","path":"/repos/demo"}]}`))
	})
	mux.HandleFunc("/v1/workspaces/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.URL.Path)
		mu.Unlock()

		if strings.HasSuffix(r.URL.Path, "/events") {
			w.Header().Set("Content-Type", "text/event-stream")
			w.(http.Flusher).Flush()
			<-r.Context().Done()

			return
		}

		w.WriteHeader(http.StatusTeapot) // any future current-session-style call
	})

	ln, err := net.Listen("unix", filepath.Join(t.TempDir(), "crush.sock"))
	if err != nil {
		t.Fatalf("unix listen: %v", err)
	}

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	feed := NewAgentFeed()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = RunAgentFeed(ctx, "unix://"+ln.Addr().String(), feed, discardLogger()) }()

	deadline := time.Now().Add(10 * time.Second)
	for {
		mu.Lock()
		n := len(calls)
		mu.Unlock()

		if n > 0 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("no workspace-scoped call arrived before timeout")
		}

		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	if !strings.HasSuffix(calls[0], "/events") {
		t.Fatalf("first workspace-scoped call = %q, want the events attach (M6: attach BEFORE current-session)", calls[0])
	}
}
