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

// TestProvenanceItemsOrderingAndAge pins the §f31 contract: the repri
// history renders AS-IS in the slice order harvest.BuildProvenance
// produced — fact-trail order, oldest→newest, never re-sorted — and each
// row carries the coarse relative age beside its absolute 01-02 15:04
// term, so "when did this reprioritize" reads without calendar math while
// the exact instant stays on the page.
func TestProvenanceItemsOrderingAndAge(t *testing.T) {
	t.Parallel()

	oldest := time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC)
	latest := oldest.Add(72 * time.Hour)
	now := latest.Add(2 * time.Hour)

	v := priorityProvenanceView{
		Current: 70,
		Band:    "backlog",
		History: []repriEventView{
			{At: oldest, Old: 50, New: 60, Source: "ai:batch-scorer", Reason: "first bump"},
			{At: latest, Old: 60, New: 70, Source: "marker", Reason: "P2 marker"},
		},
	}

	items := provenanceItems(v, now)

	var repriIdx []int

	for i, item := range items {
		if strings.HasPrefix(item.Term, "repri ") {
			repriIdx = append(repriIdx, i)
		}
	}

	if len(repriIdx) != 2 {
		t.Fatalf("repri rows = %d, want one per history event", len(repriIdx))
	}

	first, second := items[repriIdx[0]], items[repriIdx[1]]

	if first.Term != "repri 11-01 09:30" {
		t.Errorf("first repri term = %q, want the OLDEST event first (history renders as-is)", first.Term)
	}

	if second.Term != "repri 11-04 09:30" {
		t.Errorf("second repri term = %q, want the newer event second", second.Term)
	}

	if !strings.HasSuffix(first.Detail, "· 3d ago") || !strings.Contains(first.Detail, "50 → 60 · ai:batch-scorer") {
		t.Errorf("first repri detail = %q, want the story plus the relative age", first.Detail)
	}

	if !strings.HasSuffix(second.Detail, "· 2h ago") {
		t.Errorf("second repri detail = %q, want the relative age beside the absolute term", second.Detail)
	}
}

// TestRepriAgeBuckets pins the coarse relative-age vocabulary: minute/
// hour/day/week buckets with "just now" for fresh events — the same voice
// the card's created/updated rows speak.
func TestRepriAgeBuckets(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		ago  time.Duration
		want string
	}{
		{ago: 30 * time.Second, want: "just now"},
		{ago: time.Minute, want: "1m ago"},
		{ago: 59 * time.Minute, want: "59m ago"},
		{ago: time.Hour, want: "1h ago"},
		{ago: 23 * time.Hour, want: "23h ago"},
		{ago: 24 * time.Hour, want: "1d ago"},
		{ago: 6 * 24 * time.Hour, want: "6d ago"},
		{ago: 7 * 24 * time.Hour, want: "1w ago"},
		{ago: 20 * 24 * time.Hour, want: "2w ago"},
	} {
		if got := repriAge(now.Add(-tc.ago), now); got != tc.want {
			t.Errorf("repriAge(%s ago) = %q, want %q", tc.ago, got, tc.want)
		}
	}
}
