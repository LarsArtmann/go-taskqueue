package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoParallelGlobalStdoutCapture pins the capture discipline mechanically:
// any Test* function that calls t.Parallel must NOT open a captureStdout /
// captureStdoutFn window — the helper swaps the process-global os.Stdout and
// a parallel sibling writing during the window corrupts the stream (the
// TestShowJSONCarriesFailureEvidence exposure; 105c1cb6→0960e857). Parallel
// tests render via the writer-parameterized functions instead (see the
// captureMu doc comment in main_test.go for the full list). Non-parallel
// sites keep the window; captureMu serializes the windows among themselves.
func TestNoParallelGlobalStdoutCapture(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no .go files found — census ran over nothing")
	}

	for _, file := range files {
		if !strings.HasSuffix(file, "_test.go") {
			continue
		}

		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), file, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}

		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}

			var parallel, capture bool

			ast.Inspect(fn, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok &&
						(id.Name == "captureStdout" || id.Name == "captureStdoutFn") {
						capture = true
					}
				}

				if sel, ok := n.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "t" && sel.Sel.Name == "Parallel" {
						parallel = true
					}
				}

				return true
			})

			if parallel && capture {
				t.Errorf("%s: %s calls t.Parallel and captureStdout — render via a writer-parameterized function into a bytes.Buffer instead (the showTaskJSON pattern)",
					file, fn.Name.Name)
			}
		}
	}
}
