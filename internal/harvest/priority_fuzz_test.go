package harvest

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzSplitMarker pins the marker grammar against arbitrary text: the
// invariants are (a) a parsed marker's level is always 1–4, (b) the
// stripped text is never empty and is a strict prefix of the input
// (progress: repeated splits terminate), and (c) no-marker inputs are
// returned untouched.
func FuzzSplitMarker(f *testing.F) {
	f.Add("Fix the token check — P1: security-adjacent")
	f.Add("Refresh vendored CSS — P3")
	f.Add("Add a P1 DNS record")
	f.Add("Something — P5")
	f.Add("Something - P2")
	f.Add("— P2: only a marker")
	f.Add(" —P1")
	f.Add("0 —P1 —P1")
	f.Add("note — P2: yes: really: colons")
	f.Add("— P4 — P1: double marker")
	f.Add("trailing spaces   — P2   ")
	f.Add("")

	f.Fuzz(func(t *testing.T, text string) {
		level, stripped, ok := SplitMarker(text)

		if ok && (level < 1 || level > MaxMarkerLevel) {
			t.Fatalf("SplitMarker(%q) level %d out of range", text, level)
		}

		if ok {
			if stripped == "" || stripped == text {
				t.Fatalf("SplitMarker(%q) made no progress: %q", text, stripped)
			}

			if len(stripped) >= len(text) || text[:len(stripped)] != stripped {
				t.Fatalf("SplitMarker(%q) stripped %q is not a strict prefix", text, stripped)
			}
		}

		if !ok && stripped != text {
			t.Fatalf("SplitMarker(%q) mutated text without a marker: %q", text, stripped)
		}
	})
}

// FuzzReadImportance pins the metadata reader: arbitrary file content
// either yields a value in 0–100 or an error — never a panic, never an
// out-of-range importance.
func FuzzReadImportance(f *testing.F) {
	f.Add("importance: 50\n")
	f.Add("tags: [go]\nimportance: 70\ncreated_at: 2026-01-01\n")
	f.Add("importance: \"55\"\n")
	f.Add("importance:\n  importance: 999\n")
	f.Add("importance: high\n")
	f.Add("importance: -1\n")
	f.Add("# importance: 90\n")
	f.Add("")

	repo := f.TempDir()

	f.Fuzz(func(t *testing.T, content string) {
		if err := os.MkdirAll(filepath.Join(repo, ".config"), 0o755); err != nil {
			t.Skip()
		}

		if err := os.WriteFile(filepath.Join(repo, ".config", "metadata.yaml"), []byte(content), 0o644); err != nil {
			t.Skip()
		}

		got, err := ReadImportance(repo)
		if err != nil {
			return // any error is acceptable; panics are not
		}

		if got < 0 || got > 100 {
			t.Fatalf("ReadImportance(%q) = %d, out of range 0-100", content, got)
		}
	})
}
