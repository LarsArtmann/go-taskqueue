package effort

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/harvest"
)

func TestMinutesFromScore(t *testing.T) {
	t.Parallel()

	cases := []struct {
		score float64
		want  int
	}{
		{0, 10},    // exact level 0
		{1, 25},    // exact level 1
		{2, 60},    // exact level 2
		{3, 150},   // exact level 3
		{4, 240},   // exact level 4 (top)
		{-1, 10},   // below the rubric clamps to the floor
		{9, 240},   // above the rubric clamps to the ceiling
		{0.5, 20},  // 10 + 0.5*(25-10) = 17.5 → up to 20
		{1.5, 45},  // 25 + 0.5*(60-25) = 42.5 → up to 45
		{2.4, 100}, // 60 + 0.4*(150-60) = 96 → up to 100
	}

	for _, tc := range cases {
		if got := MinutesFromScore(tc.score); got != tc.want {
			t.Errorf("MinutesFromScore(%v) = %d, want %d", tc.score, got, tc.want)
		}
	}
}

// testItems builds harvest items with stable keys.
func testItems(texts ...string) []harvest.Item {
	items := make([]harvest.Item, len(texts))
	for i, text := range texts {
		items[i] = harvest.Item{
			Repo:     "/repos/proj",
			RepoName: "proj",
			Heading:  "## Bugs",
			Text:     text,
			Key:      harvest.ItemKey("proj", text),
		}
	}

	return items
}

// jevStub answers every request with the given per-question scores and
// counts requests.
func jevStub(t *testing.T, scores map[string]float64, hits *atomic.Int32) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)

		var req systemOneRequest
		if err := json.Unmarshal(readAll(t, r), &req); err != nil {
			t.Errorf("stub decode request: %v", err)
		}

		if req.Model == "" {
			t.Errorf("request model empty")
		}

		resp := systemOneResponse{Model: req.Model, Answers: map[string]scoreAnswer{}}
		for name := range req.Questions {
			resp.Answers[name] = scoreAnswer{
				Type:       "score",
				Score:      scores[name],
				Confidence: 0.9,
			}
		}

		body, err := json.Marshal(resp)
		if err != nil {
			t.Errorf("stub encode response: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))

	t.Cleanup(server.Close)

	return server
}

func readAll(t *testing.T, r *http.Request) []byte {
	t.Helper()

	data := make([]byte, 0, 4096)
	buf := make([]byte, 4096)

	for {
		n, err := r.Body.Read(buf)
		data = append(data, buf[:n]...)

		if err != nil {
			break
		}
	}

	return data
}

// TestJevEstimatorContractAndCache pins the request shape (one score
// question per item under stable todos.<name> paths, shared repo state)
// and the cache: a second pass answers from the sidecar without
// another request.
func TestJevEstimatorContractAndCache(t *testing.T) {
	var hits atomic.Int32

	server := jevStub(t, map[string]float64{"t0": 2, "t1": 4}, &hits)

	cachePath := filepath.Join(t.TempDir(), "effort.json")

	est, err := NewJevEstimator(JevConfig{APIKey: "k", BaseURL: server.URL, CachePath: cachePath})
	if err != nil {
		t.Fatal(err)
	}

	items := testItems("fix the null deref", "rewrite the scheduler")
	got := est.EstimateEffort(context.Background(), items)

	if want := harvest.ItemKey("proj", "fix the null deref"); got[want] != 60 {
		t.Errorf("estimate for the level-2 item = %d, want 60", got[want])
	}

	if want := harvest.ItemKey("proj", "rewrite the scheduler"); got[want] != 240 {
		t.Errorf("estimate for the level-4 item = %d, want 240", got[want])
	}

	if hits.Load() != 1 {
		t.Fatalf("server hits = %d after first pass, want 1", hits.Load())
	}

	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache sidecar not written: %v", err)
	}

	again := est.EstimateEffort(context.Background(), items)
	if len(again) != 2 {
		t.Fatalf("second pass = %v, want both keys from cache", again)
	}

	if hits.Load() != 1 {
		t.Errorf("server hits = %d after cached pass, want 1 (no re-ask)", hits.Load())
	}
}

// TestJevEstimatorRequestShape pins the state contract: repo name and
// purpose under named fields, each item under todos.<name> with text
// and section, and the question naming its subject by path.
func TestJevEstimatorRequestShape(t *testing.T) {
	var hits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)

		var req systemOneRequest
		if err := json.Unmarshal(readAll(t, r), &req); err != nil {
			t.Errorf("decode request: %v", err)
		}

		state := req.State

		repo, _ := state["repo"].(map[string]any)
		if repo["name"] != "proj" {
			t.Errorf("state repo.name = %v, want proj", repo["name"])
		}

		if _, ok := repo["purpose"]; !ok {
			t.Error("state repo.purpose missing")
		}

		todos, _ := state["todos"].(map[string]any)
		first, _ := todos["t0"].(map[string]any)
		if first == nil || first["text"] != "fix the null deref" || first["section"] != "## Bugs" {
			t.Errorf("todos.t0 = %v, want text+section fields", todos["t0"])
		}

		question, ok := req.Questions["t0"]
		if !ok || question.Type != "score" {
			t.Fatalf("question t0 = %+v, want a score question", req.Questions["t0"])
		}

		if want := "How much work is `todos.t0` for a senior engineer in this repo?"; question.Instructions != want {
			t.Errorf("question t0 instructions = %q, want %q", question.Instructions, want)
		}

		if len(question.Criteria) != len(rubric) {
			t.Errorf("question t0 criteria = %d levels, want %d", len(question.Criteria), len(rubric))
		}

		w.Header().Set("Content-Type", "application/json")

		body, err := json.Marshal(systemOneResponse{
			Model:   req.Model,
			Answers: map[string]scoreAnswer{"t0": {Type: "score", Score: 1}},
		})
		if err != nil {
			t.Errorf("encode response: %v", err)
		}

		_, _ = w.Write(body)
	}))

	t.Cleanup(server.Close)

	est, err := NewJevEstimator(JevConfig{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	est.EstimateEffort(context.Background(), testItems("fix the null deref"))

	if hits.Load() != 1 {
		t.Fatalf("server hits = %d, want 1", hits.Load())
	}
}

// TestJevEstimatorFallsBackOnError pins the resilience contract: a
// failing API degrades to NO estimate (the harvest flat default
// applies), never an error and never a fabricated low answer.
func TestJevEstimatorFallsBackOnError(t *testing.T) {
	var hits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))

	t.Cleanup(server.Close)

	est, err := NewJevEstimator(JevConfig{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	got := est.EstimateEffort(context.Background(), testItems("anything"))
	if len(got) != 0 {
		t.Fatalf("estimates after server failure = %v, want none (flat default applies)", got)
	}

	if hits.Load() != 1 {
		t.Errorf("server hits = %d, want 1 (one attempt, no retry storm)", hits.Load())
	}
}

// TestJevEstimatorModelSwapReEstimates pins the source stamp: a cached
// verdict from another model is a miss, never reused for sizing.
func TestJevEstimatorModelSwapReEstimates(t *testing.T) {
	var hits atomic.Int32

	server := jevStub(t, map[string]float64{"t0": 2}, &hits)

	est, err := NewJevEstimator(JevConfig{APIKey: "k", BaseURL: server.URL, Model: "jev-b"})
	if err != nil {
		t.Fatal(err)
	}

	item := testItems("one task")[0]

	cache := NewCache()
	cache.Put(item.Key, 200, "jev-a", 0.9)

	est.cache = cache

	got := est.EstimateEffort(context.Background(), []harvest.Item{item})
	if got[item.Key] != 60 {
		t.Fatalf("estimate = %d, want 60 (re-estimated by jev-b, not the cached jev-a verdict)", got[item.Key])
	}

	if hits.Load() != 1 {
		t.Errorf("server hits = %d, want 1 (the stale-stamped entry re-asked)", hits.Load())
	}
}

// TestJevEstimatorChunksLargePasses pins the request cap: 25 fresh
// items ask in two requests, never one unbounded payload.
func TestJevEstimatorChunksLargePasses(t *testing.T) {
	var hits atomic.Int32

	server := jevStub(t, map[string]float64{}, &hits)

	est, err := NewJevEstimator(JevConfig{APIKey: "k", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	texts := make([]string, 25)
	for i := range texts {
		texts[i] = "task"
	}

	est.EstimateEffort(context.Background(), testItems(texts...))

	if hits.Load() != 2 {
		t.Errorf("server hits = %d for 25 items, want 2 (chunked at %d)", hits.Load(), maxQuestionsPerRequest)
	}
}

func TestNewJevEstimatorRequiresKey(t *testing.T) {
	t.Parallel()

	if _, err := NewJevEstimator(JevConfig{}); err == nil {
		t.Fatal("empty API key must fail fast, not run estimator-less")
	}
}

func TestCacheRoundtripAndPrune(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "effort.json")

	// A sidecar carrying one fresh and one stale entry: the load keeps
	// the fresh, prunes the stale.
	sidecar := cacheFile{
		Version: cacheVersion,
		Entries: map[string]Entry{
			"fresh": {Minutes: 60, Source: SourceFor("jev-latest"), Confidence: 0.9, At: time.Now()},
			"stale": {Minutes: 30, Source: SourceFor("jev-latest"), At: time.Now().Add(-cacheMaxAge - time.Hour)},
		},
	}

	sidecarBytes, err := json.Marshal(sidecar)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, sidecarBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded := NewCache()
	if err := loaded.Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}

	if _, ok := loaded.Get("stale", "jev-latest"); ok {
		t.Error("stale entry survived the load prune")
	}

	entry, ok := loaded.Get("fresh", "jev-latest")
	if !ok {
		t.Fatal("fresh entry missing after roundtrip")
	}

	if entry.Minutes != 60 {
		t.Errorf("fresh entry minutes = %d, want 60", entry.Minutes)
	}

	// A corrupt sidecar loads empty, never errors the caller.
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := loaded.Load(path); err == nil {
		t.Error("corrupt sidecar must surface a parse error (the caller degrades to empty)")
	}
}
