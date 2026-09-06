package harvest

import (
	"path/filepath"
	"testing"
)

// TestItemKeyUnicodeVectors pins dedup keys for non-ASCII items to golden
// values computed independently (sha256 of repo + NUL + whitespace-collapsed
// text, hex-truncated), so a hashing or normalization regression cannot
// silently re-enqueue every open item as "new work". Collapsing treats every
// Unicode whitespace as a separator: an item reflowed with a non-breaking
// space keeps its key.
func TestItemKeyUnicodeVectors(t *testing.T) {
	cases := []struct {
		repo, text, want string
	}{
		{"repò", "修复 中文 的 bug", "todo:4facfca9f812510f"},
		{"repò", "ship the 🚀 launch", "todo:8b94f7a47087ade6"},
		{"repò", "a b", "todo:fa1040571ec0a671"},
		{"repò", "a   b", "todo:fa1040571ec0a671"},
		{"repò", "café", "todo:be8c4d30ca4f1a9e"},
		{"repò", "café", "todo:cefc20a12a4e97e6"},
	}
	for _, c := range cases {
		if got := ItemKey(c.repo, c.text); got != c.want {
			t.Errorf("ItemKey(%q, %q) = %q, want %q", c.repo, c.text, got, c.want)
		}
	}
}

// TestItemKeyPathSpellingIndependent pins the portability contract that
// matters for dedup: the key depends on the repo's base name only, never on
// the full path or its separators, so harvesting the same repo via a
// relative or absolute spelling (or after a projects-root move — including
// one across platforms) can never double-enqueue its items.
func TestItemKeyPathSpellingIndependent(t *testing.T) {
	dir := t.TempDir()
	repo := writeRepo(t, dir, "portable", "# H\n- [ ] same item everywhere\n")

	t.Chdir(dir) // relative spellings below resolve against the temp dir
	spellings := []string{repo, filepath.Join(".", "portable"), "portable"}
	var keys []string
	for _, s := range spellings {
		items, err := ParseRepoAll(s, DefaultTodoFile)
		if err != nil {
			t.Fatalf("ParseRepoAll(%q): %v", s, err)
		}
		if len(items) != 1 {
			t.Fatalf("ParseRepoAll(%q) = %d items, want 1", s, len(items))
		}
		// Repo must be reported in native form (filepath semantics, no
		// hardcoded separators) and be stable across spellings.
		if items[0].Repo != repo {
			t.Errorf("ParseRepoAll(%q).Repo = %q, want native %q", s, items[0].Repo, repo)
		}
		keys = append(keys, items[0].Key)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i] != keys[0] {
			t.Errorf("key changed with path spelling: %q vs %q", keys[0], keys[i])
		}
	}
}
