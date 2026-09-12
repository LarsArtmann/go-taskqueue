package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// The payload projection is the detail page's content layer: it must lead
// with the work item, collapse the prompt, keep the executor contract as
// fields, and degrade to the raw pane rather than fabricate structure.

func TestPayloadViewAgent(t *testing.T) {
	t.Parallel()

	payload := `{"repo":"/repos/demo","prompt":"Contract:\n1. Read AGENTS.md\n2. Do the work",` +
		`"item":"Anti-ghost-archive gate: CI check","verify":"go build ./...","model":"prov/model-x",` +
		`"dedup":"todo:abc123","timeout_minutes":45,"yolo":true}`
	pv := payloadViewFor(task.Task{Type: executor.TaskTypeAgent, Payload: json.RawMessage(payload)})

	if pv.Kind != payloadAgent {
		t.Fatalf("kind = %q, want %q", pv.Kind, payloadAgent)
	}

	if pv.Lede != "Anti-ghost-archive gate: CI check" {
		t.Errorf("lede = %q, want the work item (the task's content)", pv.Lede)
	}

	if pv.Prompt != "Contract:\n1. Read AGENTS.md\n2. Do the work" {
		t.Errorf("prompt = %q, want it collapsed behind the fold", pv.Prompt)
	}

	fields := map[string]string{}
	for _, f := range pv.Fields {
		fields[f.Label] = f.Value
	}

	want := map[string]string{
		"repo":        "/repos/demo",
		"verify gate": "go build ./...",
		"model":       "prov/model-x",
		"dedup key":   "todo:abc123",
		"timeout":     "45 min",
		"autonomy":    "yolo",
	}

	if !reflect.DeepEqual(fields, want) {
		t.Errorf("fields = %v, want %v", fields, want)
	}

	if !pv.hasRaw() {
		t.Error("raw pane must stay available for zero information loss")
	}

	if !strings.Contains(pv.Raw, "\n") {
		t.Errorf("raw = %q, want JSON pretty-printed", pv.Raw)
	}
}

func TestPayloadViewAgentAutoDetectAndNoItem(t *testing.T) {
	t.Parallel()

	payload := `{"repo":"/repos/demo","prompt":"Do the thing"}`
	pv := payloadViewFor(task.Task{Type: executor.TaskTypeAgent, Payload: json.RawMessage(payload)})

	for _, f := range pv.Fields {
		if f.Label == "verify gate" && f.Value != "auto-detect" {
			t.Errorf("verify gate = %q, want auto-detect surfaced", f.Value)
		}
	}

	// Without a harvested item the prompt IS the content: it takes the
	// lede instead of hiding behind a fold.
	if pv.Lede != "Do the thing" {
		t.Errorf("lede = %q, want the prompt as content", pv.Lede)
	}

	if pv.Prompt != "" {
		t.Errorf("prompt fold = %q, want empty (already the lede)", pv.Prompt)
	}

	// The raw pane still earns its place: the JSON object carries fields
	// (repo) the lede does not.
	if !pv.hasRaw() {
		t.Error("raw pane must stay available for the JSON object")
	}
}

func TestPayloadViewAgentUnparseableFallsBackToRaw(t *testing.T) {
	t.Parallel()

	pv := payloadViewFor(task.Task{Type: executor.TaskTypeAgent, Payload: json.RawMessage(`{"repo":`)})

	if pv.Kind != payloadRaw {
		t.Fatalf("kind = %q, want %q (never fabricate structure)", pv.Kind, payloadRaw)
	}

	if pv.Lede != "" || len(pv.Fields) != 0 {
		t.Errorf("fallback must not invent fields, got lede %q + %d fields", pv.Lede, len(pv.Fields))
	}

	if !pv.hasRaw() {
		t.Error("raw pane must carry the payload")
	}
}

func TestPayloadViewReview(t *testing.T) {
	t.Parallel()

	payload := `{"repo":"/repos/demo","reviewed_task":"000001a08edfbc90bf02dd35ec0d5e7bf524",` +
		`"item":"fix the gate","commit_sha":"abc123","files_changed":["a.go","b.go"],` +
		`"extra":"focus on the CI wiring"}`
	pv := payloadViewFor(task.Task{Type: executor.TaskTypeReview, Payload: json.RawMessage(payload)})

	if pv.Kind != payloadReview {
		t.Fatalf("kind = %q, want %q", pv.Kind, payloadReview)
	}

	if pv.Lede != "fix the gate" {
		t.Errorf("lede = %q, want the reviewed item", pv.Lede)
	}

	if pv.Note != "focus on the CI wiring" {
		t.Errorf("note = %q, want the operator focus instructions", pv.Note)
	}

	var linked bool

	for _, f := range pv.Fields {
		if f.Label == "reviewed task" {
			linked = f.Href == "/task/000001a08edfbc90bf02dd35ec0d5e7bf524"
		}
	}

	if !linked {
		t.Error("reviewed task must link to its detail page")
	}

	for _, f := range pv.Fields {
		if f.Label == "files changed" && f.Value != "a.go, b.go" {
			t.Errorf("files changed = %q, want the joined list", f.Value)
		}
	}
}

func TestPayloadViewStatus(t *testing.T) {
	t.Parallel()

	payload := `{"repo":"/repos/demo","project":"demo","verify":"go test ./...",` +
		`"completed":[{"task_id":"task-a","item":"one"},{"task_id":"task-b","item":"two"}]}`
	pv := payloadViewFor(task.Task{Type: executor.TaskTypeStatus, Payload: json.RawMessage(payload)})

	if pv.Kind != payloadStatus {
		t.Fatalf("kind = %q, want %q", pv.Kind, payloadStatus)
	}

	if pv.Lede != "reporting window: 2 completed tasks" {
		t.Errorf("lede = %q, want the window summary", pv.Lede)
	}

	if !reflect.DeepEqual(pv.Window, []string{"task-a", "task-b"}) {
		t.Errorf("window = %v, want the completed task ids", pv.Window)
	}
}

func TestPayloadViewSh(t *testing.T) {
	t.Parallel()

	t.Run("json cmd envelope", func(t *testing.T) {
		t.Parallel()

		pv := payloadViewFor(task.Task{Type: "sh", Payload: json.RawMessage(`{"cmd":"go test ./..."}`)})

		if pv.Kind != payloadSh || pv.Lede != "go test ./..." || !pv.LedeMono {
			t.Fatalf("sh view = %+v, want the unwrapped command as the lede", pv)
		}

		if !pv.hasRaw() {
			t.Error("envelope payload keeps its raw pane (the exact stored JSON)")
		}
	})

	t.Run("raw line hides the redundant raw pane", func(t *testing.T) {
		t.Parallel()

		pv := payloadViewFor(task.Task{Type: "sh", Payload: json.RawMessage(`echo hi`)})

		if pv.Lede != "echo hi" {
			t.Fatalf("lede = %q, want the raw shell line", pv.Lede)
		}

		if pv.hasRaw() {
			t.Error("raw pane duplicates the lede for a plain shell line")
		}
	})
}

func TestPayloadViewUnknownTypeIsRaw(t *testing.T) {
	t.Parallel()

	pv := payloadViewFor(task.Task{Type: "webhook", Payload: json.RawMessage(`{"url":"https://x","secret":"s"}`)})

	if pv.Kind != payloadRaw || !pv.hasRaw() {
		t.Fatalf("unknown type must degrade to the raw pane, got %+v", pv)
	}

	if !strings.Contains(pv.Raw, "https://x") {
		t.Errorf("raw = %q, want the payload preserved verbatim", pv.Raw)
	}
}

func TestRetryTrail(t *testing.T) {
	t.Parallel()

	t1 := time.Date(2026, 9, 11, 7, 17, 0, 0, time.UTC)
	t2 := t1.Add(3 * time.Minute)
	t3 := t1.Add(6 * time.Minute)

	facts := []journalFactView{
		{Seq: 1, Type: journal.Enqueued, Time: t1},
		{Seq: 2, Type: journal.Claimed, Time: t1},
		{Seq: 3, Type: journal.Requeued, Error: "preflight: repo dirty", Time: t1},
		{Seq: 4, Type: journal.Claimed, Time: t2},
		{Seq: 5, Type: journal.Requeued, Detail: json.RawMessage(`{"reason":"preflight: repo dirty"}`), Time: t2},
		{Seq: 6, Type: journal.Claimed, Time: t3},
		{Seq: 7, Type: journal.Failed, Error: "verify failed", Attempt: 2, Time: t3},
	}

	trail := retryTrail(facts)
	if len(trail) != 2 {
		t.Fatalf("trail = %+v, want 2 distinct reasons", trail)
	}

	if trail[0].Reason != "preflight: repo dirty" || trail[0].Count != 2 {
		t.Errorf("first reason = %+v, want the doubled refusal loudest", trail[0])
	}

	if trail[1].Reason != "verify failed" || trail[1].Count != 1 {
		t.Errorf("second reason = %+v, want the single failure", trail[1])
	}
}

func TestRetryTrailQuietUntilRepeated(t *testing.T) {
	t.Parallel()

	if got := retryTrail(nil); got != nil {
		t.Errorf("empty facts = %+v, want nil", got)
	}

	one := retryTrail([]journalFactView{
		{Seq: 1, Type: journal.Requeued, Error: "preflight: repo dirty"},
	})

	if one != nil {
		t.Errorf("single occurrence = %+v, want nil (the trail already shows it)", one)
	}

	claims := retryTrail([]journalFactView{
		{Seq: 1, Type: journal.Claimed},
		{Seq: 2, Type: journal.Claimed},
	})

	if claims != nil {
		t.Errorf("claims are not retries = %+v, want nil", claims)
	}
}

func TestRetryTrailReasonlessDefaults(t *testing.T) {
	t.Parallel()

	trail := retryTrail([]journalFactView{
		{Seq: 1, Type: journal.Requeued},
		{Seq: 2, Type: journal.Released},
	})

	if len(trail) != 1 || trail[0].Reason != "(no reason recorded)" || trail[0].Count != 2 {
		t.Fatalf("trail = %+v, want one honest placeholder row ×2", trail)
	}
}

func TestRetryTrailCountsDeadLetter(t *testing.T) {
	t.Parallel()

	t1 := time.Date(2026, 9, 11, 7, 17, 0, 0, time.UTC)
	t2 := t1.Add(3 * time.Minute)

	trail := retryTrail([]journalFactView{
		{Seq: 1, Type: journal.Requeued, Error: "preflight: repo dirty", Time: t1},
		{Seq: 2, Type: journal.DeadLettered, Error: "verify failed", Attempt: 3, Time: t2},
	})

	if len(trail) != 2 {
		t.Fatalf("trail = %+v, want the refusal and the dead-letter reason", trail)
	}

	if trail[0].Reason != "verify failed" || trail[0].Count != 1 {
		t.Errorf("first reason = %+v, want the newest (dead-letter) failure first", trail[0])
	}

	if trail[1].Reason != "preflight: repo dirty" || trail[1].Count != 1 {
		t.Errorf("second reason = %+v, want the earlier refusal", trail[1])
	}
}

// TestPayloadSectionGoldenRender pins the RENDERED payload-section HTML for
// the two owner-approved shapes (2026-09-11 detail-page decisions): an
// item'd agent payload LEADS with the item and keeps the prompt COLLAPSED;
// an item-less payload leads WITH the prompt. The golden markers are the
// structural substrings (element classes and order) any payload regression
// would disturb.
func TestPayloadSectionGoldenRender(t *testing.T) {
	t.Parallel()

	render := func(t *testing.T, tk task.Task) string {
		t.Helper()

		var buf bytes.Buffer

		if err := payloadSection(tk, payloadViewFor(tk)).Render(context.Background(), &buf); err != nil {
			t.Fatalf("payloadSection render: %v", err)
		}

		return buf.String()
	}

	t.Run("item leads, prompt collapsed", func(t *testing.T) {
		t.Parallel()

		payload := `{"repo":"/repos/demo","prompt":"Secret contract text",` +
			`"item":"Anti-ghost-archive gate","verify":"go build ./..."}`
		html := render(t, task.Task{Type: executor.TaskTypeAgent, Payload: json.RawMessage(payload)})

		itemIdx := strings.Index(html, "Anti-ghost-archive gate")
		if itemIdx < 0 {
			t.Fatalf("rendered section missing the work item lede: %s", html)
		}

		// The prompt lives inside the collapsed <details class="tq-fold">
		// fold, which renders BELOW the item lede: present in the DOM,
		// hidden behind the fold.
		foldIdx := strings.Index(html, `<details class="tq-fold"><summary>prompt</summary>`)
		if foldIdx < itemIdx {
			t.Errorf("prompt fold (at %d) must render below the item lede (at %d)", foldIdx, itemIdx)
		}

		if !strings.Contains(html, "Secret contract text") {
			t.Error("the fold must still carry the full prompt text in the DOM")
		}

		for _, marker := range []string{"tq-payload-item", "verify gate", "go build ./..."} {
			if !strings.Contains(html, marker) {
				t.Errorf("rendered section missing %q", marker)
			}
		}
	})

	t.Run("item-less leads with the prompt", func(t *testing.T) {
		t.Parallel()

		payload := `{"repo":"/repos/demo","prompt":"Do the thing"}`
		html := render(t, task.Task{Type: executor.TaskTypeAgent, Payload: json.RawMessage(payload)})

		if !strings.Contains(html, "Do the thing") {
			t.Error("item-less payload must lead with the prompt as content")
		}
	})

	t.Run("unparseable degrades to the raw pane", func(t *testing.T) {
		t.Parallel()

		html := render(t, task.Task{Type: executor.TaskTypeAgent, Payload: json.RawMessage(`{"repo":`)})

		if !strings.Contains(html, "raw payload") || !strings.Contains(html, "tq-payload-pre") {
			t.Error("raw pane fold missing for the unparseable payload")
		}
	})
}
