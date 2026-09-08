package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSweepSidecarsRemovesAgedKeepsLive(t *testing.T) {
	dir := t.TempDir()

	aged := filepath.Join(dir, "0001-aged.log")
	live := filepath.Join(dir, "0002-live.log")
	foreign := filepath.Join(dir, "0003-aged.txt")

	for _, p := range []string{aged, live, foreign} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(aged, old, old); err != nil {
		t.Fatal(err)
	}

	if err := os.Chtimes(foreign, old, old); err != nil {
		t.Fatal(err)
	}

	removed, err := SweepSidecars(dir, 24*time.Hour)
	if err != nil {
		t.Fatalf("SweepSidecars: %v", err)
	}

	if removed != 1 {
		t.Fatalf("removed %d files, want 1 (only the aged .log)", removed)
	}

	if _, err := os.Stat(aged); !os.IsNotExist(err) {
		t.Error("aged sidecar survived the sweep")
	}

	if _, err := os.Stat(live); err != nil {
		t.Error("live sidecar was swept before its age")
	}

	if _, err := os.Stat(foreign); err != nil {
		t.Error("non-log file was swept — the sweep must touch only sidecar logs")
	}
}

func TestSweepSidecarsDisabled(t *testing.T) {
	// Zero age and empty dir are no-ops, never errors.
	if n, err := SweepSidecars("", time.Hour); n != 0 || err != nil {
		t.Fatalf("empty dir = %d/%v, want 0/nil", n, err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.log"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if n, err := SweepSidecars(dir, 0); n != 0 || err != nil {
		t.Fatalf("zero max-age = %d/%v, want 0/nil (retention off)", n, err)
	}

	if _, err := os.Stat(filepath.Join(dir, "a.log")); err != nil {
		t.Error("zero max-age must not delete anything")
	}
}

// TestSweepSidecarsByBytes pins the byte-budget retention (the round-6
// retention item's open half): over-budget dirs lose their OLDEST logs
// first, under-budget dirs keep everything, and non-.log files never count.
func TestSweepSidecarsByBytes(t *testing.T) {
	dir := t.TempDir()

	write := func(name, body string, age time.Duration) {
		t.Helper()

		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}

		past := time.Now().Add(-age)
		if err := os.Chtimes(path, past, past); err != nil {
			t.Fatal(err)
		}
	}

	// 3 logs of 100 bytes, oldest first; one non-log file that must survive.
	write("oldest.log", strings.Repeat("a", 100), 3*time.Hour)
	write("middle.log", strings.Repeat("b", 100), 2*time.Hour)
	write("newest.log", strings.Repeat("c", 100), 1*time.Hour)
	write("keep.txt", strings.Repeat("x", 500), 4*time.Hour)

	removed, err := SweepSidecarsByBytes(dir, 250)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if removed != 1 {
		t.Fatalf("removed = %d, want 1 (only the oldest over-budget log)", removed)
	}

	if _, err := os.Stat(filepath.Join(dir, "oldest.log")); !os.IsNotExist(err) {
		t.Error("oldest.log survived an over-budget sweep")
	}

	for _, kept := range []string{"middle.log", "newest.log", "keep.txt"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s must survive: %v", kept, err)
		}
	}

	// An impossible budget still leaves nothing behind but never errors.
	removed, err = SweepSidecarsByBytes(dir, 1)
	if err != nil || removed != 2 {
		t.Fatalf("hard-cap sweep removed = %d err = %v, want 2/nil", removed, err)
	}

	// Disabled and empty-dir cases are no-ops.
	if n, err := SweepSidecarsByBytes(dir, 0); err != nil || n != 0 {
		t.Fatalf("disabled sweep = %d/%v, want 0/nil", n, err)
	}

	if n, err := SweepSidecarsByBytes("", 100); err != nil || n != 0 {
		t.Fatalf("empty-dir sweep = %d/%v, want 0/nil", n, err)
	}
}
