// Command facadeparity audits ADR-0016 facade modules: every exported
// declaration of each internal package must carry a same-named, kind-
// compatible re-export in its public facade, and a facade declaration
// must not dangle past its internal counterpart. Exit 1 on any skew.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// kind classify an exported top-level declaration.
type kind string

const (
	kindType  kind = "type"
	kindConst kind = "const"
	kindVar   kind = "var"
	kindFunc  kind = "func"
)

// compatible reports whether an internal declaration of kind ik may be
// re-exported by a facade declaration of kind fk. Functions cannot be
// aliased in Go, so the facade convention re-exports them as vars.
func compatible(ik, fk kind) bool {
	switch ik {
	case kindType:
		return fk == kindType
	case kindConst:
		return fk == kindConst
	case kindVar:
		return fk == kindVar
	case kindFunc:
		return fk == kindVar
	}
	return false
}

// decl is one exported top-level declaration.
type decl struct {
	name string
	k    kind
}

// exportedDecls parses the package's non-test Go files and returns the
// exported top-level declarations (receiver methods ride the type alias
// and are intentionally not tracked).
func exportedDecls(dir string) (map[string]kind, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no non-test Go files in %s", dir)
	}

	fset := token.NewFileSet()
	filter := func(fi os.FileInfo) bool {
		name := fi.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}
	pkgs, err := parser.ParseDir(fset, dir, filter, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}

	decls := make(map[string]kind)
	var order []string
	add := func(name string, k kind) {
		if !ast.IsExported(name) {
			return
		}
		if _, seen := decls[name]; !seen {
			order = append(order, name)
		}
		decls[name] = k
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, d := range file.Decls {
				switch n := d.(type) {
				case *ast.FuncDecl:
					if n.Recv == nil {
						add(n.Name.Name, kindFunc)
					}
				case *ast.GenDecl:
					for _, spec := range n.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							add(s.Name.Name, kindType)
						case *ast.ValueSpec:
							k := kindVar
							if n.Tok == token.CONST {
								k = kindConst
							}
							for _, name := range s.Names {
								add(name.Name, k)
							}
						}
					}
				}
			}
		}
	}
	return decls, order, nil
}

type pair struct {
	internal string
	facade   string
}

func run(root string) error {
	pairs := []pair{
		{"internal/task", "task"},
		{"internal/journal", "journal"},
		{"internal/queue", "queue"},
		{"internal/queue/sqlite", "queue/sqlite"},
		{"internal/queue/postgres", "queue/postgres"},
		{"internal/executor", "executor"},
		{"internal/worker", "worker"},
	}

	skew := 0
	for _, p := range pairs {
		internalDecls, order, err := exportedDecls(filepath.Join(root, p.internal))
		if err != nil {
			return fmt.Errorf("parse %s: %w", p.internal, err)
		}
		facadeDecls, _, err := exportedDecls(filepath.Join(root, p.facade))
		if err != nil {
			return fmt.Errorf("parse %s: %w", p.facade, err)
		}

		for _, name := range order {
			ik := internalDecls[name]
			if fk, ok := facadeDecls[name]; !ok {
				fmt.Printf("MISSING-ALIAS %s: %s.%s (%s) has no facade re-export\n", p.facade, p.internal, name, ik)
				skew++
			} else if !compatible(ik, fk) {
				fmt.Printf("KIND-MISMATCH %s: %s.%s is %s internally but %s in the facade\n", p.facade, p.internal, name, ik, fk)
				skew++
			}
		}
		for name, fk := range facadeDecls {
			if _, ok := internalDecls[name]; !ok {
				fmt.Printf("DANGLING-ALIAS %s: %s (%s) has no counterpart in %s\n", p.facade, name, fk, p.internal)
				skew++
			}
		}
	}
	if skew > 0 {
		return fmt.Errorf("%d facade parity skew finding(s) — add the alias in the same change that adds the export (ADR-0016)", skew)
	}
	fmt.Println("facade parity OK: 7 facades mirror their internal packages")
	return nil
}

func main() {
	root := "."
	if len(os.Args) == 2 {
		root = os.Args[1]
	} else if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: facadeparity [repo-root]")
		os.Exit(2)
	}
	if err := run(root); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}
