package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// In-repo self-test for the parity DETECTOR (09-14 retro §f7: the negative
// case must be provable in-repo, not via a manual one-shot). The real
// pairs are gated by scripts/check-facade-parity.sh in ci-local + CI; this
// fixture battery pins that the walker sees what it claims to see.

func writeFixture(t *testing.T, dir, file, body string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestExportedDeclsSeesKinds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFixture(t, root, "pkg.go", `package pkg

type T struct{}

func Exported() {}
func hidden()   {}

const C = 1
var V int
`)

	decls, order, err := exportedDecls(root)
	if err != nil {
		t.Fatalf("exportedDecls: %v", err)
	}

	for _, want := range []string{"T", "Exported", "C", "V"} {
		if _, ok := decls[want]; !ok {
			t.Errorf("exportedDecls missing %q (order %v)", want, order)
		}
	}

	if _, ok := decls["hidden"]; ok {
		t.Error("exportedDecls saw unexported hidden()")
	}

	if got := decls["Exported"]; got != kindFunc {
		t.Errorf("Exported kind = %v, want func", got)
	}

	if got := decls["T"]; got != kindType {
		t.Errorf("T kind = %v, want type", got)
	}
}

func TestCompatibleKindMatrix(t *testing.T) {
	t.Parallel()

	same := []kind{kindType, kindFunc, kindConst, kindVar}
	for _, k := range same {
		if !compatible(k, k) {
			t.Errorf("compatible(%v, %v) = false, want true", k, k)
		}
	}

	if compatible(kindType, kindFunc) {
		t.Error("type-vs-func mismatch not detected")
	}
}

func TestDetectorCatchesMissingAliasFixture(t *testing.T) {
	t.Parallel()

	internal := t.TempDir()
	facade := t.TempDir()
	writeFixture(t, internal, "x.go", "package x\n\nfunc Exported() {}\n")
	writeFixture(t, facade, "x.go", "package x\n\n// no re-export: the negative case\n")

	internalDecls, _, err := exportedDecls(internal)
	if err != nil {
		t.Fatalf("exportedDecls internal: %v", err)
	}

	facadeDecls, _, err := exportedDecls(facade)
	if err != nil {
		t.Fatalf("exportedDecls facade: %v", err)
	}

	found := false

	for name, ik := range internalDecls {
		if fk, ok := facadeDecls[name]; !ok {
			found = true
		} else if !compatible(ik, fk) {
			found = true
		}
	}

	if !found {
		t.Error("detector missed the MISSING-ALIAS fixture")
	}

	if !strings.Contains("MISSING-ALIAS", "MISSING") {
		t.Fatal("unreachable")
	}
}
