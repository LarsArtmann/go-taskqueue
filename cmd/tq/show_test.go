package main

import (
	"context"
	"encoding/json/v2"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestBuildPriorityProvenance pins the tq show priority section: band and
// current value always, item identity + cached AI verdict for
// harvest-minted tasks, and the reprioritization history distilled from
// the fact trail.
func TestBuildPriorityProvenance(t *testing.T) {
	t.Parallel()

	store, got := seedProvenanceTask(t)

	ctx := context.Background()

	trail, err := store.FactsForTask(ctx, got.ID.String(), 0)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	provenance := buildPriorityProvenance(ctx, store, got, trail)

	if provenance.Current != 85 || provenance.Band != "backlog" {
		t.Fatalf("current/band = %d/%q, want 85/backlog", provenance.Current, provenance.Band)
	}

	if provenance.ItemKey != "todo:abc" || provenance.MarkerLevel != 0 {
		t.Fatalf("item identity = %q/%d, want todo:abc/0", provenance.ItemKey, provenance.MarkerLevel)
	}

	if provenance.CachedScore == nil || provenance.CachedScore.Score != 85 ||
		provenance.CachedScore.Source != "ai:batch-scorer" {
		t.Fatalf("cached score = %+v", provenance.CachedScore)
	}

	if len(provenance.RepriHistory) != 1 {
		t.Fatalf("repri history = %+v, want one event", provenance.RepriHistory)
	}

	event := provenance.RepriHistory[0]
	if event.Old != 50 || event.New != 85 || event.Source != "ai" || event.Reason != "unblocks the release" {
		t.Fatalf("repri event = %+v", event)
	}

	if event.At == "" {
		t.Fatal("repri event carries no timestamp")
	}
}

// seedProvenanceTask builds a scored, once-reprioritized harvest task over
// a scratch store and returns the store plus the task's current record.
func seedProvenanceTask(t *testing.T) (*sqlite.Store, task.Task) {
	t.Helper()

	ctx := context.Background()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	payload, err := json.Marshal(map[string]any{
		"repo":        "demo",
		"prompt":      "work",
		"item":        "Fix the frobnicator",
		"dedup":       "todo:abc",
		"markerLevel": 0,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	enq, err := store.Enqueue(ctx, task.New{
		Type:     "agent",
		Project:  "demo",
		Priority: 50,
		Payload:  payload,
		DedupKey: "todo:abc",
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := store.SavePriorityScore(ctx, queuePriorityScore()); err != nil {
		t.Fatalf("save score: %v", err)
	}

	if err := store.UpdatePendingPriority(ctx, enq.ID, 85, "ai", "unblocks the release"); err != nil {
		t.Fatalf("reprioritize: %v", err)
	}

	got, err := store.Get(ctx, enq.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	return store, got
}

// TestBuildPriorityProvenanceForeignTask pins the section for a
// non-harvest task: band still reported, item identity and cached score
// omitted, no crash.
func TestBuildPriorityProvenanceForeignTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	raw, err := json.Marshal(map[string]any{"cmd": "echo hi"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	enq, err := store.Enqueue(ctx, task.New{Type: "sh", Project: "demo", Priority: 150, Payload: raw})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	provenance := buildPriorityProvenance(ctx, store, enq, nil)

	if provenance.Current != 150 || provenance.Band != "machine" {
		t.Fatalf("current/band = %d/%q, want 150/machine", provenance.Current, provenance.Band)
	}

	if provenance.ItemKey != "" || provenance.CachedScore != nil || provenance.RepriHistory != nil {
		t.Fatalf("foreign task provenance = %+v, want band-only", provenance)
	}
}

func queuePriorityScore() queue.PriorityScore {
	return queue.PriorityScore{
		ItemKey:       "todo:abc",
		Score:         85,
		EffortMinutes: 30,
		Source:        "ai:batch-scorer",
		Reasoning:     "unblocks the release",
	}
}

// TestShowJSONCarriesDerivedSessionUsage is the e2e wire pin for derived
// session usage on `tq show`: a status AND a prioritize task completed
// against a scratch sqlite store must carry session_cost_usd,
// session_prompt_tokens and session_completion_tokens through the show
// JSON's wholesale-marshaled result section (04-07 report f1).
// resultDetail's decode (TestResultDetailDecodesTypedResults) and the
// webui render (TestResultUsageRendersOnDetailPage) are pinned
// separately — this is the only gate on the encoder step between them.
func TestShowJSONCarriesDerivedSessionUsage(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "usage.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	ctx := context.Background()

	seeds := []struct {
		taskType string
		payload  any
		detail   any
		wantCost float64
		wantIn   int64
		wantOut  int64
		id       task.ID
	}{
		{
			taskType: executor.TaskTypeStatus,
			payload: executor.StatusPayload{
				Repo:      "demo",
				Project:   "demo",
				Completed: []executor.StatusCompletion{{TaskID: "000001a0c2452641", Item: "Fix the frobnicator"}},
			},
			detail: executor.StatusResult{
				Report:                  "docs/status/2026-09-21_10-00_demo.md",
				NextItems:               2,
				SessionID:               "sess-status",
				SessionCostUSD:          0.0042,
				SessionPromptTokens:     1200,
				SessionCompletionTokens: 340,
			},
			wantCost: 0.0042,
			wantIn:   1200,
			wantOut:  340,
		},
		{
			taskType: executor.TaskTypePrioritize,
			payload: executor.PrioritizePayload{
				Repo:  "demo",
				Items: []executor.PrioritizeItem{{Key: "todo:abc", Text: "Fix the frobnicator"}},
			},
			detail: executor.PrioritizeResult{
				Verdicts:                []executor.PrioritizeVerdict{{ItemKey: "todo:abc", Score: 70}},
				SessionID:               "sess-prioritize",
				SessionCostUSD:          0.0137,
				SessionPromptTokens:     4200,
				SessionCompletionTokens: 910,
			},
			wantCost: 0.0137,
			wantIn:   4200,
			wantOut:  910,
		},
	}

	for i := range seeds {
		raw, err := json.Marshal(seeds[i].payload)
		if err != nil {
			t.Fatalf("marshal %s payload: %v", seeds[i].taskType, err)
		}

		enq, err := store.Enqueue(ctx, task.New{Type: seeds[i].taskType, Project: "demo", Payload: raw})
		if err != nil {
			t.Fatalf("enqueue %s: %v", seeds[i].taskType, err)
		}

		claimed, err := store.ClaimDue(ctx, "show-usage-e2e", time.Minute)
		if err != nil {
			t.Fatalf("claim %s: %v", seeds[i].taskType, err)
		}

		if claimed.ID != enq.ID {
			t.Fatalf("claimed %s, want the enqueued %s task %s", claimed.ID, seeds[i].taskType, enq.ID)
		}

		detail, err := json.Marshal(seeds[i].detail)
		if err != nil {
			t.Fatalf("marshal %s result: %v", seeds[i].taskType, err)
		}

		if err := store.Complete(ctx, enq.ID, "show-usage-e2e", detail); err != nil {
			t.Fatalf("complete %s: %v", seeds[i].taskType, err)
		}

		seeds[i].id = enq.ID
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	for _, seed := range seeds {
		t.Run(seed.taskType, func(t *testing.T) {
			out := captureStdout(t, func() {
				if err := cmdShow([]string{"--db", dbPath, seed.id.String()}); err != nil {
					t.Errorf("cmdShow: %v", err)
				}
			})

			var doc struct {
				Task struct {
					ID     string `json:"id"`
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"task"`
				Result struct {
					SessionCostUSD          float64 `json:"session_cost_usd"`
					SessionPromptTokens     int64   `json:"session_prompt_tokens"`
					SessionCompletionTokens int64   `json:"session_completion_tokens"`
				} `json:"result"`
			}

			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("decode show output: %v\n%s", err, out)
			}

			if doc.Task.ID != seed.id.String() || doc.Task.Type != seed.taskType {
				t.Fatalf("task = %s/%s, want %s/%s", doc.Task.ID, doc.Task.Type, seed.id, seed.taskType)
			}

			if doc.Task.Status != "completed" {
				t.Fatalf("task status = %q, want completed", doc.Task.Status)
			}

			if doc.Result.SessionCostUSD != seed.wantCost {
				t.Errorf("result.session_cost_usd = %v, want %v", doc.Result.SessionCostUSD, seed.wantCost)
			}

			if doc.Result.SessionPromptTokens != seed.wantIn {
				t.Errorf("result.session_prompt_tokens = %v, want %v", doc.Result.SessionPromptTokens, seed.wantIn)
			}

			if doc.Result.SessionCompletionTokens != seed.wantOut {
				t.Errorf("result.session_completion_tokens = %v, want %v", doc.Result.SessionCompletionTokens, seed.wantOut)
			}
		})
	}
}
