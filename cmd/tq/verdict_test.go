package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdVerdictWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	t.Setenv("TQ_RESULT_FILE", path)

	if err := cmdVerdict([]string{`{"verdict":"approve","summary":"ok"}`}); err != nil {
		t.Fatalf("verdict: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result file: %v", err)
	}

	if string(raw) != `{"verdict":"approve","summary":"ok"}` {
		t.Fatalf("file = %s, want the JSON verbatim", raw)
	}
}

func TestCmdVerdictRejectsInvalidJSON(t *testing.T) {
	t.Setenv("TQ_RESULT_FILE", filepath.Join(t.TempDir(), "r.json"))

	err := cmdVerdict([]string{"not json"})
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("err = %v, want the invalid-JSON rejection", err)
	}
}

func TestCmdVerdictRequiresResultFile(t *testing.T) {
	t.Setenv("TQ_RESULT_FILE", "")

	if err := cmdVerdict([]string{`{}`}); err == nil ||
		!strings.Contains(err.Error(), "TQ_RESULT_FILE is not set") {
		t.Fatalf("err = %v, want the missing-channel guidance", err)
	}
}

func TestCmdVerdictNeedsExactlyOneArg(t *testing.T) {
	if err := cmdVerdict(nil); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("err = %v, want usage", err)
	}
}
