package harvest

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestItemKeyProperty pins the dedup-key contract as a property: the key is
// a pure function of (repoName, whitespace-collapsed text). Whitespace
// reflow must not change it; any other text change must. Deterministic keys
// are what keeps repeated harvests from double-enqueueing.
func TestItemKeyProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	space := func() string {
		// random mix of spaces/tabs/newlines, sometimes none
		switch rng.Intn(4) {
		case 0:
			return ""
		case 1:
			return " "
		case 2:
			return strings.Repeat(" ", 1+rng.Intn(3))
		default:
			return strings.Repeat("\t\n ", 1+rng.Intn(3))
		}
	}
	collapse := func(s string) string { return strings.Join(strings.Fields(s), " ") }

	for range 200 {
		words := []string{"fix", "the", "parser", "for", "CRLF", "行", "дорога", "🚀"}
		var raw, collapsed []string
		for w := range words {
			raw = append(raw, words[w])
			collapsed = append(collapsed, words[w])
			// separator: random whitespace, never empty (an empty one
			// would merge WORDS, which legitimately changes the key)
			raw = append(raw, space()+" ")
			collapsed = append(collapsed, " ")
		}
		text := strings.Join(raw, "")
		want := ItemKey("repo", strings.Join(collapsed, ""))
		if got := ItemKey("repo", text); got != want {
			t.Fatalf("whitespace reflow changed the key: %q vs %q", got, want)
		}
		if ItemKey("other-repo", strings.Join(collapsed, "")) == want {
			t.Fatal("repo name must participate in the key")
		}
		// Key is a truncated sha256 of repo\0text: verify the construction.
		sum := sha256.Sum256([]byte("repo\x00" + collapse(text)))
		if want != "todo:"+hex.EncodeToString(sum[:])[:16] {
			t.Fatalf("key %q drifted from its documented construction", want)
		}
	}
}

// FuzzParseRepo: the parser sits at the trust boundary between arbitrary
// human-edited markdown and the enqueue path. It must never panic, must be
// deterministic, and every returned item must carry a usable dedup key.
func FuzzParseRepo(f *testing.F) {
	f.Add("## Work\n\n- [ ] normal item\n")
	f.Add("- [ ] no heading\r\n- [x] checked\r\n- [ ] open\r\n")
	f.Add("\xEF\xBB\xBF## BOM ahead\n\n- [ ] bom item\n")
	f.Add("### deep\n#### deeper\n##### deepest\n- [ ] nested\n  - [ ] indented\n\t- [ ] tabbed\n")
	f.Add("```md\n- [ ] inside fence\n```\n\n- [ ] outside fence\n")
	f.Add("- [ ]\n- []\n- [ ]\n- [ ]spaces   \n- [ ]\ttab\ttext\n")
	f.Add("- [ ] " + strings.Repeat("x", 10000) + "\n")
	f.Add(strings.Repeat("- [ ] a\n", 500))

	repo := f.TempDir()
	f.Fuzz(func(t *testing.T, content string) {
		file := filepath.Join(repo, DefaultTodoFile)
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Skip()
		}
		items1, err1 := ParseRepo(repo, DefaultTodoFile)
		items2, err2 := ParseRepo(repo, DefaultTodoFile)
		if (err1 == nil) != (err2 == nil) {
			t.Fatalf("non-deterministic error: %v vs %v", err1, err2)
		}
		if err1 != nil {
			t.Skip() // write errors (tmpdir) are fine; parser has no error path on content
		}
		if len(items1) != len(items2) {
			t.Fatalf("non-deterministic item count: %d vs %d", len(items1), len(items2))
		}
		for i := range items1 {
			a, b := items1[i], items2[i]
			if a.Key != b.Key || a.Text != b.Text || a.Heading != b.Heading {
				t.Fatalf("non-deterministic parse: %+v vs %+v", a, b)
			}
			if strings.TrimSpace(a.Text) == "" {
				t.Fatalf("empty item text parsed: %+v", a)
			}
			if a.Key == "" || !strings.HasPrefix(a.Key, "todo:") {
				t.Fatalf("item %q has unusable dedup key %q", a.Text, a.Key)
			}
		}
	})
}
