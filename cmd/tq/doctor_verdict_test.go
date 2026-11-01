package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctorVerdictChannel pins the paid-turn recording-channel probe (row
// 806 slice 3): the prompts bake `tq verdict`/`tq ask` rails, so a PATH
// without tq must WARN, a matching hit must pass clean, and a different
// PATH hit must still pass while naming the shadowing.
func TestDoctorVerdictChannel(t *testing.T) {
	plantTQ := func(t *testing.T, dir string) string {
		t.Helper()

		bin := filepath.Join(dir, "tq")
		if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("plant tq: %v", err)
		}

		return bin
	}

	t.Run("warns when tq is missing from PATH", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		results := doctorVerdictChannel("/opt/tq")

		if len(results) != 1 {
			t.Fatalf("results = %+v, want exactly one", results)
		}

		if results[0].Name != "verdict-channel" || results[0].Status != checkWarn {
			t.Fatalf("result = %+v, want verdict-channel warn", results[0])
		}

		if !strings.Contains(results[0].Detail, "not found on PATH") {
			t.Fatalf("detail = %q, want the missing-binary guidance", results[0].Detail)
		}
	})

	t.Run("passes clean when the PATH hit is the running binary", func(t *testing.T) {
		dir := t.TempDir()
		self := plantTQ(t, dir)

		t.Setenv("PATH", dir)

		results := doctorVerdictChannel(self)

		if len(results) != 1 || results[0].Status != checkOK {
			t.Fatalf("results = %+v, want one ok", results)
		}

		if strings.Contains(results[0].Detail, "shadowing") {
			t.Fatalf("detail = %q, want no skew note for the running binary", results[0].Detail)
		}
	})

	t.Run("passes with a skew note when PATH shadows the running binary", func(t *testing.T) {
		plantTQ(t, t.TempDir()+"/shadow") // never on PATH

		dir := t.TempDir()
		hit := plantTQ(t, dir)

		t.Setenv("PATH", dir)

		results := doctorVerdictChannel(hit + ".elsewhere")

		if len(results) != 1 || results[0].Status != checkOK {
			t.Fatalf("results = %+v, want one ok (skew is a note, not a warn)", results)
		}

		if !strings.Contains(results[0].Detail, "shadowing") {
			t.Fatalf("detail = %q, want the PATH-shadowing note", results[0].Detail)
		}
	})

	t.Run("unknown self skips the skew comparison", func(t *testing.T) {
		dir := t.TempDir()
		plantTQ(t, dir)

		t.Setenv("PATH", dir)

		results := doctorVerdictChannel("")

		if len(results) != 1 || results[0].Status != checkOK {
			t.Fatalf("results = %+v, want one ok with empty self", results)
		}

		if strings.Contains(results[0].Detail, "shadowing") {
			t.Fatalf("detail = %q, want no skew note without a self path", results[0].Detail)
		}
	})
}
