package executor

import (
	"go/ast"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/testastguard"
)

// TestTailHelpersDelegateRedaction is the structural guard (02-26 §b2/§f4):
// every executor helper that turns raw process output into a tail must run
// the secrets-in-logs pass, so a future third tail helper cannot silently
// bypass redaction and persist a live credential into a task.failed fact.
// The scan is syntactic (go/parser over the package's non-test sources):
// any function whose name mentions "tail" must either call redactOutput /
// redactBytes directly or delegate to a helper whose own name mentions
// "tail". A NEW tail helper trips the census until it delegates — a
// deliberate speed bump, not a bureaucratic one: the author must look at
// the redaction contract before their helper reaches a journal fact.
func TestTailHelpersDelegateRedaction(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob package sources: %v", err)
	}

	census := map[string]bool{}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		f := testastguard.Parse(t, file)

		f.FuncDecls(
			func(name string) bool { return namesTailWord(name) },
			func(name string, body ast.Node) {
				census[name] = true

				delegates := slices.ContainsFunc(testastguard.CallNames(body), func(callee string) bool {
					return callee == "redactOutput" || callee == "redactBytes" || namesTailWord(callee)
				})

				if !delegates {
					t.Errorf(
						"%s: tail helper %s does not call redactOutput/redactBytes (nor a tail helper that does) â output tails must pass the secrets-in-logs pass before reaching journal facts",
						file,
						name,
					)
				}
			},
		)
	}

	want := []string{"tailBytes", "tailOutput"}

	got := make([]string, 0, len(census))
	for name := range census {
		got = append(got, name)
	}

	slices.Sort(got)

	if !slices.Equal(got, want) {
		t.Errorf(
			"tail-helper census = %v, want %v â a new tail helper must delegate to redactOutput/redactBytes and join this pin (or justify its absence here)",
			got,
			want,
		)
	}
}

// namesTailWord reports whether a function name uses "tail" as a whole
// camelCase word. Substring matching false-positived on the "tail" inside
// "Detail" (SetResultDetail/Sink.Detail are structured outcome accessors,
// not output tails) and dragged them into the census; word matching keeps
// the pin on real tail helpers only.
func namesTailWord(name string) bool {
	for _, w := range splitCamelWords(name) {
		if strings.EqualFold(w, "tail") {
			return true
		}
	}

	return false
}

// splitCamelWords splits an identifier at lower→upper boundaries:
// "SetResultDetail" → [Set Result Detail], "tailBytes" → [tail Bytes].
func splitCamelWords(name string) []string {
	var words []string

	start := 0

	for i := 1; i < len(name); i++ {
		if name[i-1] >= 'a' && name[i-1] <= 'z' && name[i] >= 'A' && name[i] <= 'Z' {
			words = append(words, name[start:i])
			start = i
		}
	}

	return append(words, name[start:])
}
