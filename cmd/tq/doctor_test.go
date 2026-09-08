package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/status"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func doctorTestStore(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "q.db")
	s, err := queue.OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	return path
}

func resultByName(results []checkResult, name string) checkResult {
	for _, r := range results {
		if r.Name == name {
			return r
		}
	}

	return checkResult{Name: name, Status: "missing", Detail: "not found"}
}

func TestDoctorHealthyEmptyDB(t *testing.T) {
	path := doctorTestStore(t)

	// Point the agent-binary check at the test binary itself: worst==ok
	// must not depend on `crush` being installed on the host (the nix
	// sandbox has no crush — its checkPhase caught this assumption).
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("abs test binary: %v", err)
	}

	results, err := runDoctor(context.Background(), doctorOptions{DBPath: path, AgentBin: self})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if worst := doctorWorst(results); worst != checkOK {
		t.Fatalf("worst = %s, want ok; results: %+v", worst, results)
	}

	for _, name := range []string{"db", "wal", "queue", "worker"} {
		if r := resultByName(results, name); r.Status != checkOK {
			t.Errorf("%s = %s (%s), want ok", name, r.Status, r.Detail)
		}
	}
}

func TestDoctorFlagsDeadWorker(t *testing.T) {
	path := doctorTestStore(t)

	s, err := queue.OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// A pending task nobody claims, plus a running task whose lease died
	// with its worker: the classic "pool is down" signature.
	if _, err := s.Enqueue(ctx, task.New{Type: "sh"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, err := s.ClaimDue(ctx, "dead-worker", time.Millisecond); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// The lease dies here; no heartbeat ever extends it.
	time.Sleep(20 * time.Millisecond)

	if _, err := s.Enqueue(ctx, task.New{Type: "sh"}); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	results, err := runDoctor(ctx, doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "worker"); r.Status != checkFail {
		t.Errorf("worker = %s (%s), want fail (pending work, no heartbeats)", r.Status, r.Detail)
	}

	if r := resultByName(results, "queue"); r.Status != checkWarn {
		t.Errorf("queue = %s (%s), want warn (expired lease unreclaimed)", r.Status, r.Detail)
	}

	if worst := doctorWorst(results); worst != checkFail {
		t.Errorf("worst = %s, want fail", worst)
	}
}

func TestDoctorBudgetAtCap(t *testing.T) {
	path := doctorTestStore(t)

	s, err := queue.OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	for range 2 {
		if _, err := s.Enqueue(ctx, task.New{Type: "sh"}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	results, err := runDoctor(ctx, doctorOptions{DBPath: path, DailyBudget: 2})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "budget"); r.Status != checkWarn {
		t.Errorf("budget = %s (%s), want warn at cap", r.Status, r.Detail)
	}
}

func TestDoctorCorruptDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.db")

	if err := os.WriteFile(path, []byte("this is definitely not a sqlite database"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runDoctor(context.Background(), doctorOptions{DBPath: path})
	if err == nil {
		t.Fatal("runDoctor on a corrupt file must fail")
	}
}

func TestDoctorRepoAutonomy(t *testing.T) {
	dir := t.TempDir()

	healthy := filepath.Join(dir, "healthy")
	if err := os.MkdirAll(filepath.Join(healthy, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(healthy, "TODO_LIST.md"), []byte("## Work\n\n- [ ] item\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(healthy, ".crushrc"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	bare := filepath.Join(dir, "bare")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}

	path := doctorTestStore(t)

	results, err := runDoctor(context.Background(), doctorOptions{DBPath: path, Repos: healthy + "," + bare})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "repo:healthy"); r.Status != checkOK {
		t.Errorf("repo:healthy = %s (%s), want ok", r.Status, r.Detail)
	}

	if r := resultByName(results, "autonomy:healthy"); r.Status != checkOK {
		t.Errorf("autonomy:healthy = %s (%s), want ok", r.Status, r.Detail)
	}

	if r := resultByName(results, "repo:bare"); r.Status != checkWarn {
		t.Errorf("repo:bare = %s (%s), want warn", r.Status, r.Detail)
	}

	if r := resultByName(results, "autonomy:bare"); r.Status != checkWarn {
		t.Errorf("autonomy:bare = %s (%s), want warn", r.Status, r.Detail)
	}
}

func TestDoctorJSONShape(t *testing.T) {
	path := doctorTestStore(t)

	results, err := runDoctor(context.Background(), doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	payload, err := json.Marshal(map[string]any{"status": doctorWorst(results), "checks": results})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(payload), `"name":"db"`) {
		t.Errorf("json payload missing db check: %s", payload)
	}
}

func TestDoctorMarkOrphans(t *testing.T) {
	path := doctorTestStore(t)

	s, err := queue.OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	enq, err := s.Enqueue(ctx, task.New{Type: "sh"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.ClaimDue(ctx, "victim", time.Millisecond); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Millisecond) // lease dies

	results, err := runDoctor(ctx, doctorOptions{DBPath: path, MarkOrphans: true})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	r := resultByName(results, "mark-orphans")
	if r.Status != checkOK || !strings.Contains(r.Detail, "1 stranded") {
		t.Errorf("mark-orphans = %s (%s), want ok with 1 stranded", r.Status, r.Detail)
	}

	// The fact is in the journal, exactly once.
	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for _, f := range facts {
		if f.TaskID == enq.ID.String() && f.Type == "task.orphaned" {
			count++
		}
	}

	if count != 1 {
		t.Fatalf("task.orphaned facts = %d, want 1", count)
	}
}

func TestDoctorWatermarkLiveness(t *testing.T) {
	s, err := queue.OpenSQLite(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// No cursors: the sweepers never ran here — idle, not sick.
	results := doctorWatermarkLiveness(ctx, s)
	for _, name := range []string{"review-sweeper", "status-sweeper"} {
		if r := resultByName(results, name); r.Status != checkOK {
			t.Errorf("%s = %s (%s), want ok", name, r.Status, r.Detail)
		}
	}

	// A cursor below the head is the "sweeper not running" signature.
	payload, err := json.Marshal(map[string]string{"cmd": "true"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "demo", Payload: payload}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := s.SaveWatermark(ctx, status.ConsumerKey, 1); err != nil {
		t.Fatalf("save watermark: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "demo", Payload: payload}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	results = doctorWatermarkLiveness(ctx, s)

	if r := resultByName(results, "status-sweeper"); r.Status != checkWarn {
		t.Errorf("status-sweeper = %s (%s), want warn (cursor lags the head)", r.Status, r.Detail)
	}

	if r := resultByName(results, "review-sweeper"); r.Status != checkOK {
		t.Errorf("review-sweeper = %s (%s), want ok (never ran)", r.Status, r.Detail)
	}
}
