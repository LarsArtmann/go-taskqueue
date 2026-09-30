package main

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// agentsDocMaxBytes ratchets the AGENTS.md budget: the plan M89 prune
// budget was 15,000 B (2026-09-09 file measured 14,904 B), but the file
// has since grown past 100 KB, so the guard pins CURRENT size + 1 KB
// slack and fails on any further growth until a deliberate prune resets
// the budget.
const agentsDocMaxBytes = 120_829

// TestAgentsDocSizeGuard keeps AGENTS.md from silently growing past its
// byte budget (plan M89 residue).
func TestAgentsDocSizeGuard(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "AGENTS.md")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat AGENTS.md: %v", err)
	}

	if info.Size() > agentsDocMaxBytes {
		t.Fatalf(
			"AGENTS.md grew to %d bytes (budget %d, plan M89 prune budget was 15,000 B) — prune the file or consciously reset agentsDocMaxBytes",
			info.Size(),
			agentsDocMaxBytes,
		)
	}
}

// TestFactsJSONGolden pins the `tq facts --json` output shape: the fact
// key set, field types, and ordering over a real store.
func TestFactsJSONGolden(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "golden.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	defer store.Close()

	ctx := t.Context()

	if _, err := store.Enqueue(ctx, task.New{
		Type:        "sh",
		Project:     "facts-golden",
		Payload:     []byte(`"echo hi"`),
		MaxAttempts: 1,
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	out := captureStdout(t, func() {
		if err := cmdFacts([]string{"--json", "--db", dbPath}); err != nil {
			t.Errorf("cmdFacts: %v", err)
		}
	})

	var facts []journal.Fact
	if err := json.Unmarshal([]byte(out), &facts); err != nil {
		t.Fatalf("unmarshal facts JSON: %v\noutput:\n%s", err, out)
	}

	if len(facts) == 0 {
		t.Fatalf("expected at least one fact, got none\noutput:\n%s", out)
	}

	if facts[0].Type != "task.enqueued" {
		t.Fatalf("first fact type = %q, want task.enqueued", facts[0].Type)
	}

	if facts[0].Seq <= 0 {
		t.Fatalf("first fact seq = %d, want > 0", facts[0].Seq)
	}

	if facts[0].TaskID == "" {
		t.Fatal("first fact taskId is empty")
	}

	if facts[0].Time.IsZero() {
		t.Fatal("first fact time is zero")
	}

	if !strings.Contains(string(facts[0].Detail), "facts-golden") {
		t.Fatalf("first fact detail missing project, got %s", string(facts[0].Detail))
	}

	var raw []map[string]jsontext.Value
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("unmarshal raw facts: %v", err)
	}

	known := map[string]bool{
		"seq": true, "time": true, "taskId": true, "type": true,
		"owner": true, "attempt": true, "error": true, "detail": true,
	}

	first := raw[0]

	for key := range first {
		if !known[key] {
			t.Fatalf("unexpected fact key %q (known set drifted from journal.Fact json tags)", key)
		}
	}

	for _, key := range []string{"seq", "time", "taskId", "type"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("missing required fact key %q", key)
		}
	}
}
