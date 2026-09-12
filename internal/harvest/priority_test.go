package harvest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue"
)

func TestSplitMarker(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		wantOK   bool
		level    int
		stripped string
	}{
		{"marker with note", "Fix the token check — P1: security-adjacent", true, 1, "Fix the token check"},
		{"bare marker", "Refresh vendored CSS — P3", true, 3, "Refresh vendored CSS"},
		{"P4", "Minor cleanup — P4", true, 4, "Minor cleanup"},
		{"mid-text P1 is not a marker", "Add a P1 DNS record", false, 0, "Add a P1 DNS record"},
		{"P5 is not a marker", "Something — P5", false, 0, "Something — P5"},
		{"hyphen is not a marker", "Something - P2", false, 0, "Something - P2"},
		{"no marker", "Plain item", false, 0, "Plain item"},
		{"marker-only line is not a marker", "— P2: only a marker", false, 0, "— P2: only a marker"},
		{"note may contain colons", "Do it — P2: yes: really", true, 2, "Do it"},
		{"lowercase p is not a marker", "Something — p1", false, 0, "Something — p1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			level, stripped, ok := SplitMarker(tc.text)
			if ok != tc.wantOK || level != tc.level || stripped != tc.stripped {
				t.Fatalf("SplitMarker(%q) = (%d, %q, %v), want (%d, %q, %v)",
					tc.text, level, stripped, ok, tc.level, tc.stripped, tc.wantOK)
			}
		})
	}
}

func TestMarkerPriority(t *testing.T) {
	cases := map[int]int{1: 90, 2: 70, 3: 50, 4: 30, 0: 0, 5: 0, -1: 0}

	for level, want := range cases {
		if got := MarkerPriority(level); got != want {
			t.Fatalf("MarkerPriority(%d) = %d, want %d", level, got, want)
		}
	}
}

// TestMarkerEditKeepsDedupKey pins the fork-prevention invariant
// (ADR-0015 §2): editing an item's marker must re-derive the SAME dedup
// key as the marker-free line — a priority edit is never a reword.
func TestMarkerEditKeepsDedupKey(t *testing.T) {
	base := ItemKey("repo", "Fix the token check")

	variants := []string{
		"Fix the token check",
		"Fix the token check — P1",
		"Fix the token check — P2: calmer now",
		"Fix the token check — P4",
	}

	for _, v := range variants {
		text := v
		if _, stripped, ok := SplitMarker(text); ok {
			text = stripped
		}

		if got := ItemKey("repo", text); got != base {
			t.Fatalf("marker variant %q derived key %s, want the marker-free key %s", v, got, base)
		}
	}

	// A REAL reword still re-arms the item (prune-stale semantics).
	if reworded := ItemKey("repo", "Fix the token check properly"); reworded == base {
		t.Fatal("reworded item derived the same key as the original")
	}
}

// TestParseRepoStripsMarker pins the parse-time integration: Text and Key
// carry marker-free text, MarkerLevel carries the signal.
func TestParseRepoStripsMarker(t *testing.T) {
	repo := t.TempDir()

	todo := "- [ ] Fix login — P1: burning\n- [ ] Plain item\n- [x] Done — P2\n"
	if err := os.WriteFile(filepath.Join(repo, DefaultTodoFile), []byte(todo), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := ParseRepo(repo, DefaultTodoFile)
	if err != nil {
		t.Fatalf("ParseRepo: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("got %d open items, want 2: %+v", len(items), items)
	}

	if items[0].Text != "Fix login" || items[0].MarkerLevel != 1 {
		t.Fatalf("marked item = %+v, want stripped text + level 1", items[0])
	}

	if items[1].Text != "Plain item" || items[1].MarkerLevel != 0 {
		t.Fatalf("plain item = %+v, want no marker", items[1])
	}

	if items[0].Key != ItemKey(items[0].RepoName, "Fix login") {
		t.Fatalf("key %s not derived from stripped text", items[0].Key)
	}
}

func TestKeywordBump(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"Fix the security hole", 30},
		{"patch CVE-2026-1234", 30},
		{"the Vulnerability is nested", 30},
		{"URGENT: redo the deploy", 25},
		{"this is critical", 25},
		{"asap please", 25},
		{"production database down", 20},
		{"breaking change in API", 20},
		{"outage in eu-west", 20},
		{"security issue in production", 30}, // strongest wins, no stacking
		{"nothing special here", 0},
		{"", 0},
	}

	for _, tc := range cases {
		if got := KeywordBump(tc.text); got != tc.want {
			t.Fatalf("KeywordBump(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

func TestReadImportance(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		repo := writeMetadata(t, "tags: [go]\nimportance: 70\ncreated_at: 2026-01-01\n")

		got, err := ReadImportance(repo)
		if err != nil || got != 70 {
			t.Fatalf("ReadImportance = (%d, %v), want (70, nil)", got, err)
		}
	})

	t.Run("quoted scalar", func(t *testing.T) {
		repo := writeMetadata(t, "importance: \"55\"\n")

		got, err := ReadImportance(repo)
		if err != nil || got != 55 {
			t.Fatalf("ReadImportance = (%d, %v), want (55, nil)", got, err)
		}
	})

	t.Run("missing file defaults to 50", func(t *testing.T) {
		got, err := ReadImportance(t.TempDir())
		if err != nil || got != DefaultImportance {
			t.Fatalf("ReadImportance = (%d, %v), want (%d, nil)", got, err, DefaultImportance)
		}
	})

	t.Run("file without importance defaults to 50", func(t *testing.T) {
		repo := writeMetadata(t, "tags: [rust]\n")

		got, err := ReadImportance(repo)
		if err != nil || got != DefaultImportance {
			t.Fatalf("ReadImportance = (%d, %v), want (%d, nil)", got, err, DefaultImportance)
		}
	})

	t.Run("nested license importance is ignored", func(t *testing.T) {
		repo := writeMetadata(t, "tags: []\nimportance: 40\nlicense:\n  importance: 999\n")

		got, err := ReadImportance(repo)
		if err != nil || got != 40 {
			t.Fatalf("ReadImportance = (%d, %v), want (40, nil) — nested keys must not win", got, err)
		}
	})

	t.Run("malformed integer errors", func(t *testing.T) {
		repo := writeMetadata(t, "importance: high\n")
		if _, err := ReadImportance(repo); !errors.Is(err, errImportanceMalformed) {
			t.Fatalf("ReadImportance err = %v, want errImportanceMalformed", err)
		}
	})

	t.Run("out of range errors", func(t *testing.T) {
		repo := writeMetadata(t, "importance: 101\n")
		if _, err := ReadImportance(repo); !errors.Is(err, errImportanceMalformed) {
			t.Fatalf("ReadImportance err = %v, want errImportanceMalformed", err)
		}
	})
}

func writeMetadata(t *testing.T, content string) string {
	t.Helper()

	repo := t.TempDir()
	writeMetadataTo(t, repo, content)

	return repo
}

func writeMetadataTo(t *testing.T, repo, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(repo, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(repo, ".config", "metadata.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePriority(t *testing.T) {
	ai := func(n int) *int { return &n }

	cases := []struct {
		name       string
		input      ResolveInput
		want       int
		wantSource PrioritySource
	}{
		{
			"hot beats marker",
			ResolveInput{
				Text:              "see /tmp/scratch",
				MarkerLevel:       1,
				HotPriority:       120,
				ImportanceEnabled: true,
				AIScore:           ai(99),
			},
			120,
			PrioritySourceHot,
		},
		{
			"marker beats ai and importance",
			ResolveInput{Text: "fix it", MarkerLevel: 4, HotPriority: 0, ImportanceEnabled: true, AIScore: ai(95)},
			30, PrioritySourceMarker,
		},
		{
			"ai beats keyword",
			ResolveInput{Text: "security thing", ImportanceEnabled: true, AIScore: ai(10)},
			10, PrioritySourceAI,
		},
		{
			"keyword bumps importance and clamps at the backlog ceiling",
			ResolveInput{Text: "URGENT production security fix", Importance: 90, ImportanceEnabled: true},
			queue.BacklogMax, PrioritySourceKeyword,
		},
		{
			"bare importance",
			ResolveInput{Text: "mundane chore", Importance: 35, ImportanceEnabled: true},
			35, PrioritySourceImportance,
		},
		{
			"importance off falls back to flat",
			ResolveInput{Text: "anything", FlatPriority: 7},
			7, PrioritySourceDefault,
		},
		{
			"marker applies with importance off",
			ResolveInput{Text: "anything", MarkerLevel: 2, FlatPriority: 7},
			70, PrioritySourceMarker,
		},
		{
			"hot disabled at zero",
			ResolveInput{Text: "see /tmp/scratch", HotPriority: 0, FlatPriority: 3},
			3, PrioritySourceDefault,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, source := ResolvePriority(tc.input)
			if got != tc.want || source != tc.wantSource {
				t.Fatalf("ResolvePriority = (%d, %s), want (%d, %s)", got, source, tc.want, tc.wantSource)
			}
		})
	}
}

// TestHarvestPriorityWiring runs the real harvester over a repo with a
// metadata file and marked items, pinning the enqueue-time priorities
// (T15.3): default-50 fallback, marker override, keyword bump, and the
// malformed-metadata repo skip.
func TestHarvestPriorityWiring(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()

	// One item per repo: harvest pacing admits one NEW item per repo per
	// run, so multi-item repos would hide items behind pacing skips.
	writeRepo(t, projects, "calm", "- [ ] water the plants\n")
	writeRepo(t, projects, "p1", "- [ ] Fix the deploy — P1\n")
	writeRepo(t, projects, "p4", "- [ ] tidy the docs — P4\n")
	writeRepo(t, projects, "spicy", "- [ ] urgent production hotfix\n") // keyword bump
	writeRepo(t, projects, "broken", "- [ ] should never enqueue\n")

	// calm: importance 35 (below default). marked: no file (default 50).
	// spicy: importance 80 + keyword bump -> clamped. broken: malformed.
	writeMetadataTo(t, filepath.Join(projects, "calm"), "importance: 35\n")
	writeMetadataTo(t, filepath.Join(projects, "spicy"), "importance: 80\n")
	writeMetadataTo(t, filepath.Join(projects, "broken"), "importance: very\n")

	tq := openQueue(t)
	h := New(tq, Config{
		Repos: []string{
			filepath.Join(projects, "calm"),
			filepath.Join(projects, "p1"),
			filepath.Join(projects, "p4"),
			filepath.Join(projects, "spicy"),
			filepath.Join(projects, "broken"),
		},
		Type:           "agent",
		TodoFile:       DefaultTodoFile,
		MaxPerTick:     10,
		UseImportance:  true,
		PromptTemplate: "work {{ITEM}}",
	})

	res, err := h.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	prio := map[string]int{}

	for _, en := range res.Enqueued {
		tk, err := tq.Get(ctx, en.TaskID)
		if err != nil {
			t.Fatalf("Get %s: %v", en.TaskID, err)
		}

		prio[en.Item.Text] = tk.Priority
	}

	if got := prio["water the plants"]; got != 35 {
		t.Fatalf("importance-based priority = %d, want 35", got)
	}

	if got := prio["Fix the deploy"]; got != 90 {
		t.Fatalf("P1 marker priority = %d, want 90", got)
	}

	if got := prio["tidy the docs"]; got != 30 {
		t.Fatalf("P4 marker priority = %d, want 30", got)
	}

	if got := prio["urgent production hotfix"]; got != queue.BacklogMax {
		t.Fatalf("keyword-clamped priority = %d, want %d", got, queue.BacklogMax)
	}

	for _, sk := range res.Skipped {
		if sk.Item.Text == "should never enqueue" && !strings.Contains(sk.Reason, "metadata") {
			t.Fatalf("malformed metadata skip reason = %q, want a metadata reason", sk.Reason)
		}
	}
}

// TestHarvestPriorityLegacyOff pins backward compatibility: without
// UseImportance, unmarked items keep the flat --priority and markers still
// apply (they are per-item human opt-in).
func TestHarvestPriorityLegacyOff(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()
	repo := writeRepo(t, projects, "legacy", "- [ ] plain — P2\n- [ ] unmarked\n")
	writeMetadataTo(t, repo, "importance: 90\n")

	tq := openQueue(t)
	h := New(tq, Config{
		Repos:          []string{repo},
		Type:           "agent",
		TodoFile:       DefaultTodoFile,
		MaxPerTick:     10,
		Priority:       5,
		PromptTemplate: "work {{ITEM}}",
	})

	res, err := h.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, en := range res.Enqueued {
		tk, err := tq.Get(ctx, en.TaskID)
		if err != nil {
			t.Fatalf("Get %s: %v", en.TaskID, err)
		}

		want := 5
		if en.Item.Text == "plain" {
			want = 70
		}

		if tk.Priority != want {
			t.Fatalf("item %q priority = %d, want %d", en.Item.Text, tk.Priority, want)
		}
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

// TestMaxPendingPerRepo pins the admission knob (ADR working set): below
// the cap items admit, at the cap they wait with a reason, and a claim
// frees a slot so a later run admits the next item (no starvation at
// small caps).
func TestMaxPendingPerRepo(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()

	repo := writeRepo(t, projects, "capped", "- [ ] first\n- [ ] second\n- [ ] third\n")

	tq := openQueue(t)
	h := New(tq, Config{
		Repos:             []string{repo},
		Type:              "agent",
		TodoFile:          DefaultTodoFile,
		MaxPerTick:        10,
		MaxPendingPerRepo: 1,
		PromptTemplate:    "work {{ITEM}}",
	})

	// Run 1: the survey runs before any enqueue (pendingCount=0), so the
	// first item admits; the siblings wait behind the one-per-run pacing.
	res, err := h.Run(ctx)
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}

	if len(res.Enqueued) != 1 || res.Enqueued[0].Item.Text != "first" {
		t.Fatalf("run 1 enqueued = %+v, want exactly 'first'", res.Enqueued)
	}

	// Run 2: the cap (1 pending) now DENIES before pacing — the skip
	// reason must say admission, and nothing enqueues.
	res, err = h.Run(ctx)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}

	if len(res.Enqueued) != 0 {
		t.Fatalf("run 2 admitted past the cap: %+v", res.Enqueued)
	}

	held := 0

	for _, sk := range res.Skipped {
		if strings.Contains(sk.Reason, "admission:") {
			held++
		}
	}

	if held == 0 {
		t.Fatal("run 2 held no item with an admission reason")
	}

	// Complete the pending task: the slot frees and run 3 admits the next
	// item — small caps do not starve. (A mere CLAIM is not enough: the
	// one-agent-per-repo rule holds while the task runs.)
	claimed, err := tq.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := tq.Complete(ctx, claimed.ID, "w1", nil); err != nil {
		t.Fatalf("complete: %v", err)
	}

	res, err = h.Run(ctx)
	if err != nil {
		t.Fatalf("run 3: %v", err)
	}

	if len(res.Enqueued) != 1 || res.Enqueued[0].Item.Text != "second" {
		t.Fatalf("run 3 enqueued = %+v, want 'second' after the completion freed the slot", res.Enqueued)
	}
}
