//go:build unix

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupDoctorTreeRepo builds a real git work tree (identity configured, no
// commits needed: check-ignore answers from .gitignore alone).
func setupDoctorTreeRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	return dir
}

// TestDoctorTreeGofmt pins the M11 live-tree probe: unformatted
// NON-gitignored files warn (they are real deaths under the scoped verify
// gate), gitignored drift stays ok (harmless since M6), and a clean tree
// is ok. POSIX-only: the fixture drives the real git and gofmt binaries.
func TestDoctorTreeGofmt(t *testing.T) {
	if resolveGofmt() == "" {
		t.Skip("gofmt not resolvable on this host")
	}

	ctx := context.Background()

	write := func(t *testing.T, dir, name, content string) {
		t.Helper()

		path := filepath.Join(dir, name)

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	const formatted = "package demo\n\nfunc Ok() {}\n"
	const ugly = "package demo\n\nfunc  Bad() int {\nreturn 1\n}\n"

	// Clean tree: ok.
	clean := setupDoctorTreeRepo(t)
	write(t, clean, "main.go", formatted)

	if r := doctorTreeGofmt(ctx, clean); r.Status != checkOK {
		t.Errorf("clean tree = %s (%s %+v), want ok", r.Status, r.Detail, r.Items)
	}

	// Gitignored drift only: ok, and the detail says why it is harmless.
	ignored := setupDoctorTreeRepo(t)
	write(t, ignored, "main.go", formatted)
	write(t, ignored, ".gitignore", "vendor/\n")
	write(t, ignored, filepath.Join("vendor", "bad.go"), ugly)

	r := doctorTreeGofmt(ctx, ignored)
	if r.Status != checkOK {
		t.Errorf("gitignored drift = %s (%s %+v), want ok", r.Status, r.Detail, r.Items)
	}

	if !strings.Contains(r.Detail, "gitignored") {
		t.Errorf("ok detail should name the gitignored confinement: %s", r.Detail)
	}

	// Non-gitignored drift: warn, the file itemized.
	flagged := setupDoctorTreeRepo(t)
	write(t, flagged, "main.go", formatted)
	write(t, flagged, "rootbad.go", ugly)

	r = doctorTreeGofmt(ctx, flagged)
	if r.Status != checkWarn {
		t.Fatalf("live-tree drift = %s (%s), want warn", r.Status, r.Detail)
	}

	if len(r.Items) != 1 || !strings.HasSuffix(r.Items[0], "rootbad.go") {
		t.Errorf("items = %+v, want exactly rootbad.go", r.Items)
	}

	if !strings.Contains(r.Detail, "scoped verify gate") {
		t.Errorf("warn detail should name the death class: %s", r.Detail)
	}
}
