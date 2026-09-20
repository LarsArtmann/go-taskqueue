package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// harvestPayloadFor is a harvest-shaped agent payload carrying the item
// identity (dedup key + marker level) the provenance section reads.
func harvestPayloadFor(item, key string, marker int) json.RawMessage {
	payload, err := json.Marshal(map[string]any{
		"repo":        "/tmp/demo",
		"item":        item,
		"dedup":       key,
		"markerLevel": marker,
	})
	if err != nil {
		panic(err)
	}

	return payload
}

func TestTaskDetailShowsPriorityProvenance(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()

	tk, err := s.Enqueue(ctx, task.New{
		Type:    "agent",
		Project: "demo",
		Payload: harvestPayloadFor("fix the flaky test", "todo:fix-the-flaky-test", 2),
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if err := s.UpdatePendingPriority(ctx, tk.ID, 42, "marker", "P2 marker added"); err != nil {
		t.Fatalf("UpdatePendingPriority: %v", err)
	}

	if err := s.SavePriorityScore(
		ctx,
		queue.PriorityScore{ //nolint:exhaustruct // display only reads score/source/reasoning
			ItemKey:   "todo:fix-the-flaky-test",
			Score:     87,
			Source:    "ai:batch-scorer",
			Reasoning: "high churn area, cheap fix",
			ScoredAt:  time.Now().UnixMilli(),
		},
	); err != nil {
		t.Fatalf("SavePriorityScore: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/task/"+tk.ID.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body)
	}

	body := rec.Body.String()

	for _, want := range []string{
		"priority provenance",
		"42 (backlog)",
		"todo:fix-the-flaky-test",
		"P2",
		"score 87",
		"ai:batch-scorer",
		"high churn area, cheap fix",
		"repri ",
		"0 → 42 · marker: P2 marker added",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("detail page missing %q", want)
		}
	}
}

func TestTaskDetailProvenanceDefaults(t *testing.T) {
	srv, s := newTestServer(t)
	tk := enqueue(t, s, "sh", "demo")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/task/"+tk.ID.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body)
	}

	body := rec.Body.String()

	for _, want := range []string{"priority provenance", "0 (backlog)", "none recorded"} {
		if !strings.Contains(body, want) {
			t.Errorf("detail page missing %q", want)
		}
	}

	for _, unwanted := range []string{"item key", "ai verdict"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("detail page shows %q for a non-harvest task", unwanted)
		}
	}
}
