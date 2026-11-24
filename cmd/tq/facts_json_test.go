package main

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// agentsDocMaxBytes pins the AGENTS.md byte budget (plan M89). The full
// reset lineage — every conscious reset, its trigger, and the doctrine
// that a reset pays for net-new LOAD-BEARING knowledge only — lives in
// docs/agents-size-resets.md (moved there 2026-11-24, window-3 §f21/§f31:
// the in-code log had grown ~115 lines and three entries deep in one day).
// Last reset 28_100 → 28_250 (2026-11-23, foreign facade-rename growth);
// the twin scripts/check-agents-size.sh re-syncs in the same change per
// the sync-pins-same-change convention (AGENTS.md §Conventions).
const agentsDocMaxBytes = 28_250

// TestAgentsDocSizeGuard keeps AGENTS.md from silently growing past its
// byte budget (plan M89 residue); the failure names the top sections so a
// prune is a 2-minute targeted fix (row 424).
func TestAgentsDocSizeGuard(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "AGENTS.md")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat AGENTS.md: %v", err)
	}

	if info.Size() <= agentsDocMaxBytes {
		return
	}

	t.Fatalf(
		"AGENTS.md grew to %d bytes (budget %d, over by %d) — prune the file or consciously reset agentsDocMaxBytes. Top sections by size:\n%s",
		info.Size(),
		agentsDocMaxBytes,
		info.Size()-agentsDocMaxBytes,
		topDocSections(path, 3),
	)
}

// topDocSections reports the n largest `## ` sections of the file by byte
// size (header lines counted into their own section, so the sizes sum to
// the file size — consistent with wc -c).
func topDocSections(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("  (read error: %v)", err)
	}

	return topSectionReport(string(data), n)
}

// topSectionReport is topDocSections over in-memory content (unit-pinned).
func topSectionReport(content string, n int) string {
	type section struct {
		name string
		size int
	}

	var sections []section

	cur := section{name: "(preamble)"}

	for line := range strings.SplitAfterSeq(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			sections = append(sections, cur)
			cur = section{name: strings.TrimRight(line, "\n"), size: len(line)}

			continue
		}

		cur.size += len(line)
	}

	sections = append(sections, cur)
	sort.Slice(sections, func(i, j int) bool { return sections[i].size > sections[j].size })

	if len(sections) > n {
		sections = sections[:n]
	}

	var b strings.Builder
	for _, s := range sections {
		fmt.Fprintf(&b, "  %6d B  %s\n", s.size, s.name)
	}

	return b.String()
}

func TestTopSectionReport(t *testing.T) {
	t.Parallel()

	content := "intro\n## One\nalpha\nbeta\n## Two\ngamma\n## Three\ndelta\n"
	got := topSectionReport(content, 2)

	want := "      18 B  ## One\n      15 B  ## Three\n"
	if got != want {
		t.Fatalf("top-2 sections:\ngot:\n%s\nwant:\n%s", got, want)
	}

	all := topSectionReport(content, 10)
	sum := 0

	for line := range strings.SplitSeq(strings.TrimSuffix(all, "\n"), "\n") {
		var size int
		if _, err := fmt.Sscanf(line, "%6d", &size); err != nil {
			t.Fatalf("parse %q: %v", line, err)
		}

		sum += size
	}

	if sum != len(content) {
		t.Fatalf("section sizes sum to %d, content is %d B", sum, len(content))
	}
}

// TestFactsJSONGolden pins the `tq facts --json` output shape: the fact
// key set, field types, and ordering over a real store. The decode
// target is factJSONView, not journal.Fact, so a Detail wire
// regression (embedded JSON silently becoming base64) fails here
// instead of round-tripping undetected.
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

	var facts []factJSONView
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

	detailRaw, ok := first["detail"]
	if !ok {
		t.Fatal("first fact missing detail key")
	}

	var detailObj map[string]jsontext.Value
	if err := json.Unmarshal(detailRaw, &detailObj); err != nil {
		t.Fatalf("detail is not an embedded JSON object (base64 regression?): %v", err)
	}
}

// TestFormatFactSurfacesEffectDisposition pins that `tq facts` renders a
// task.released fact's effect disposition in the human line, so a
// crash-reclaim is visible without --json/--detail.
func TestFormatFactSurfacesEffectDisposition(t *testing.T) {
	t.Parallel()

	line := formatFact(journal.Fact{
		Seq:    7,
		TaskID: "t1",
		Type:   journal.Released,
		Owner:  "w2",
		Detail: jsontext.Value(`{"effect":"unknown","reason":"lease-expiry"}`),
	})

	if !strings.Contains(line, "[effect=unknown]") {
		t.Fatalf("formatFact = %q, want [effect=unknown]", line)
	}
}
