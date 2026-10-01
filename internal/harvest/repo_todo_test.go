package harvest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepoTodoListParses is the CI guard for the machine contract documented
// at the top of TODO_LIST.md: the harvester consumes open `- [ ]` checkboxes
// in this repository's own backlog file. A format regression must fail CI
// here instead of surfacing later as a silently empty enqueue in a live pool.
func TestRepoTodoListParses(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, DefaultTodoFile)); err != nil {
		t.Skipf("no %s above the package (not a repo checkout?): %v", DefaultTodoFile, err)
	}

	items, err := ParseRepo(root, DefaultTodoFile)
	if err != nil {
		t.Fatalf("ParseRepo(TODO_LIST.md): %v — the backlog file must stay harvester-parseable", err)
	}

	if len(items) == 0 {
		t.Fatal("TODO_LIST.md parsed to zero open items; if that is truly intended, update this guard")
	}

	for _, it := range items {
		if strings.TrimSpace(it.Text) == "" {
			t.Errorf("item with empty text under heading %q", it.Heading)
		}

		if it.Key == "" {
			t.Errorf("item %q has no dedup key", it.Text)
		}
	}

	t.Logf("TODO_LIST.md parses: %d open items", len(items))
}

// TestDamagedCheckboxPrefix pins the malformed-bullet rejection (04-46
// §d4/§f1): a row shipped as `--- [ ] …` was silently invisible to the
// parser AND check-todo-list.sh until caught by eye. Damaged shapes are
// flagged for rejection; well-formed bullets, the space-less tolerated
// `-[ ]` shape, and plain prose bullets never match.
func TestDamagedCheckboxPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		line string
		want bool
	}{
		{"--- [ ] the 04-40 shape", true},
		{"* - [x] doubled bullet", true},
		{"- - [ ] space-separated bullets", true},
		{"-- [x] no space before bracket", true},
		{"- [ ] well-formed open", false},
		{"- [x] well-formed done", false},
		{"* [X] star bullet", false},
		{"-[ ] tolerated space-less shape", false},
		{"- prose mentioning [x] later", false},
		{"plain paragraph", false},
		{"--- just dashes", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := damagedCheckbox(tc.line) != ""; got != tc.want {
			t.Errorf("damagedCheckbox(%q) flagged = %v, want %v", tc.line, got, tc.want)
		}
	}
}

// TestStrayCheckboxPrefix pins the double-checkbox rejection (TODO row
// 206): the 2026-10-01 tick batch mangled four rows into `- [ ] [x] …` —
// checkboxOf ACCEPTS those lines (the prefix is exactly `- [ ] `), so a
// human-ticked row stayed machine-open and every consumer stayed silent.
// Only a bracket in the first characters of the item text counts; quoted
// or mid-sentence mentions never match.
func TestStrayCheckboxPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text string
		want bool
	}{
		{"[x] leftover tick from a mangled batch", true},
		{"[ ] leftover open from a mangled batch", true},
		{"[X] uppercase leftover", true},
		{"[x]", true},
		{"normal item text", false},
		{"`[x]` quoted convention mention", false},
		{"prose mentioning [x] later", false},
		{"", false},
		{"[", false},
		{"[y] not a checkbox", false},
		{"[] empty brackets are not checkboxes", false},
	}

	for _, tc := range cases {
		if got := strayCheckbox(tc.text) != ""; got != tc.want {
			t.Errorf("strayCheckbox(%q) flagged = %v, want %v", tc.text, got, tc.want)
		}
	}
}

// TestParseRepoAllRejectsDoubleCheckbox pins the runtime half of the
// double-checkbox guard (TODO row 206): a `- [ ] [x] …` mangled-tick row
// is an ERROR, not a silently machine-open row.
func TestParseRepoAllRejectsDoubleCheckbox(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	body := "# Backlog\n\n- [ ] good row one\n- [ ] [x] the mangled tick\n- [ ] good row two\n"

	if err := os.WriteFile(filepath.Join(repo, "TODO_LIST.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write todo file: %v", err)
	}

	if _, err := ParseRepoAll(repo, "TODO_LIST.md"); err == nil {
		t.Fatal("ParseRepoAll accepted a double-checkbox row, want error")
	} else if !strings.Contains(err.Error(), "double checkbox") {
		t.Fatalf("error %v does not name the double-checkbox defect", err)
	}
}

// TestParseRepoAllRejectsDamagedCheckbox pins the runtime half: a todo file
// carrying a malformed-bullet row is an ERROR, not a silently skipped line
// — the repo's harvest scan fails loudly instead of minting nothing.
func TestParseRepoAllRejectsDamagedCheckbox(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	body := "# Backlog\n\n- [ ] good row one\n--- [ ] the damaged row\n- [ ] good row two\n"

	if err := os.WriteFile(filepath.Join(repo, "TODO_LIST.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write todo file: %v", err)
	}

	if _, err := ParseRepoAll(repo, "TODO_LIST.md"); err == nil {
		t.Fatal("ParseRepoAll accepted a file with a malformed-bullet checkbox row, want error")
	} else if !strings.Contains(err.Error(), "malformed bullet") {
		t.Fatalf("error %v does not name the malformed-bullet defect", err)
	}
}
