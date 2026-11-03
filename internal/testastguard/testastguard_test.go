package testastguard

import (
	"go/ast"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const src = `package fake

import "maps"

func with(base map[string]string, add map[string]string) map[string]string {
	out := map[string]string{}
	maps.Copy(out, base)
	maps.Copy(out, add)
	return out
}

func rogue(base map[string]string) map[string]string {
	out := map[string]string{}
	maps.Copy(out, base)
	return out
}

func noCalls() {}
`

func writeFixture(t *testing.T) *File {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fake.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	return Parse(t, path)
}

func TestParseFatalsOnMissingFile(t *testing.T) {
	mt := newMockT(t)

	defer mt.recover()

	Parse(mt, filepath.Join(t.TempDir(), "absent.go"))

	if !mt.failed {
		t.Fatal("Parse did not fail on a missing file")
	}
}

func TestFuncBodyFatalsOnMissingFunc(t *testing.T) {
	f := writeFixture(t)

	mt := newMockT(t)

	defer mt.recover()

	f.FuncBody(mt, "absent")

	if !mt.failed {
		t.Fatal("FuncBody did not fail on a missing anchor func")
	}
}

func TestInSpanClassifiesCallPositions(t *testing.T) {
	f := writeFixture(t)
	withBody := f.FuncBody(t, "with")

	var inside, outside []*ast.CallExpr

	f.CensusCalls(
		func(call *ast.CallExpr) bool {
			sel, ok := call.Fun.(*ast.SelectorExpr)
			return ok && sel.Sel.Name == "Copy"
		},
		func(call *ast.CallExpr) {
			if InSpan(withBody.Pos(), withBody.End(), call.Pos()) {
				inside = append(inside, call)
			} else {
				outside = append(outside, call)
			}
		},
	)

	if len(inside) != 2 || len(outside) != 1 {
		t.Fatalf("span census inside=%d outside=%d, want 2/1", len(inside), len(outside))
	}
}

func TestCallNamesAndFuncDecls(t *testing.T) {
	f := writeFixture(t)

	got := map[string][]string{}

	f.FuncDecls(
		func(name string) bool { return true },
		func(name string, body ast.Node) { got[name] = CallNames(body) },
	)

	if want := []string{"Copy", "Copy"}; !slices.Equal(got["with"], want) {
		t.Errorf("with calls = %v, want %v", got["with"], want)
	}

	if _, ok := got["noCalls"]; !ok {
		t.Error("FuncDecls skipped a body-less-free decl that should be visited")
	}

	if _, ok := got["rogue"]; !ok {
		t.Error("FuncDecls missed rogue")
	}
}

func TestInSpanBoundaries(t *testing.T) {
	if !InSpan(10, 20, 10) || !InSpan(10, 20, 19) {
		t.Error("InSpan must include start, exclude end")
	}

	if InSpan(10, 20, 20) || InSpan(10, 20, 9) {
		t.Error("InSpan boundary violated")
	}
}

// mockT wraps the real t so a Fatalf inside the helper records failure
// (FailNow is overridden to abort via panic instead of ending the test
// binary's goroutine) without going through the testing.TB interface
// method by method.
type mockT struct {
	*testing.T
	failed bool
}

func newMockT(t *testing.T) *mockT { return &mockT{T: t} }

func (m *mockT) Fatalf(_ string, _ ...any) {
	m.failed = true

	panic(mockAbort{})
}

func (m *mockT) FailNow() {
	m.failed = true

	panic(mockAbort{})
}

func (m *mockT) recover() {
	if r := recover(); r != nil {
		if _, ok := r.(mockAbort); !ok {
			panic(r)
		}
	}
}

type mockAbort struct{}
