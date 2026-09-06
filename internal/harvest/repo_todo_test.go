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
