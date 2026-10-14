package executor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

	type helper struct{ file string }

	census := map[string]helper{}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}

		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !strings.Contains(strings.ToLower(fn.Name.Name), "tail") {
				continue
			}

			census[fn.Name.Name] = helper{file: file}

			var calls []string

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					switch callee := call.Fun.(type) {
					case *ast.Ident:
						calls = append(calls, callee.Name)
					case *ast.SelectorExpr:
						calls = append(calls, callee.Sel.Name)
					}
				}

				return true
			})

			delegates := slices.ContainsFunc(calls, func(name string) bool {
				if name == "redactOutput" || name == "redactBytes" {
					return true
				}

				return strings.Contains(strings.ToLower(name), "tail")
			})

			if !delegates {
				t.Errorf("%s: tail helper %s does not call redactOutput/redactBytes (nor a tail helper that does) — output tails must pass the secrets-in-logs pass before reaching journal facts", file, fn.Name.Name)
			}
		}
	}

	want := []string{"tailBytes", "tailOutput"}

	got := make([]string, 0, len(census))
	for name := range census {
		got = append(got, name)
	}
	slices.Sort(got)

	if !slices.Equal(got, want) {
		t.Errorf("tail-helper census = %v, want %v — a new tail helper must delegate to redactOutput/redactBytes and join this pin (or justify its absence here)", got, want)
	}
}
