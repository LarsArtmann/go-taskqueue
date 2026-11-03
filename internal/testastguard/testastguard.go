// Package testastguard is the shared syntax-census toolkit for structural
// "X is the only mechanism" test guards: parse a file, find a FuncDecl,
// and position test call sites against its body span. A new pin is a
// ~10-line census over these primitives instead of a hand-rolled
// go/parser walk (twentyfifth-dispatch §e2: extracted from the webui
// headermatrix and executor redaction guards).
package testastguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// File is a parsed Go source file with its position table.
type File struct {
	FSet *token.FileSet
	AST  *ast.File
	Path string
}

// Parse reads and parses path, failing t on either error.
func Parse(t testing.TB, path string) *File {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	fset := token.NewFileSet()

	node, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	return &File{FSet: fset, AST: node, Path: path}
}

// FuncBody returns the body of the named top-level FuncDecl, failing t
// when the function (or its body) is absent — a census over a missing
// mechanism is silently vacuous, so that failure is loud.
func (f *File) FuncBody(t testing.TB, name string) ast.Node {
	t.Helper()

	var body ast.Node

	ast.Inspect(f.AST, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == name && fn.Body != nil {
			body = fn.Body
		}

		return true
	})

	if body == nil {
		t.Fatalf("%s: func %s (with a body) not found — the census's anchor is gone", f.Path, name)
	}

	return body
}

// CensusCalls visits every CallExpr in the file for which match returns
// true.
func (f *File) CensusCalls(match func(call *ast.CallExpr) bool, visit func(call *ast.CallExpr)) {
	ast.Inspect(f.AST, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && match(call) {
			visit(call)
		}

		return true
	})
}

// InSpan reports whether pos lies within [start, end).
func InSpan(start, end, pos token.Pos) bool {
	return start <= pos && pos < end
}

// IsMaps reports whether the call's receiver identifier is "maps" (the
// maps.Copy / maps.Clone census qualifier).
func IsMaps(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)

	return ok && ident.Name == "maps"
}

// CallNames returns the callee names of every call inside node: bare
// idents by name, selector calls by the selected name.
func CallNames(node ast.Node) []string {
	var names []string

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		switch callee := call.Fun.(type) {
		case *ast.Ident:
			names = append(names, callee.Name)
		case *ast.SelectorExpr:
			names = append(names, callee.Sel.Name)
		}

		return true
	})

	return names
}

// FuncDecls iterates the file's top-level FuncDecls that have a body and
// whose name passes include.
func (f *File) FuncDecls(include func(name string) bool, visit func(name string, body ast.Node)) {
	for _, decl := range f.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !include(fn.Name.Name) {
			continue
		}

		visit(fn.Name.Name, fn.Body)
	}
}
