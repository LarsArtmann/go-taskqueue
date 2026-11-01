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
		!strings.Contains(err.Error(), "no verdict channel") {
		t.Fatalf("err = %v, want the missing-channel guidance", err)
	}
}

func TestCmdVerdictChannelSources(t *testing.T) {
	flagPath := filepath.Join(t.TempDir(), "flag.json")
	envPath := filepath.Join(t.TempDir(), "env.json")

	cases := []struct {
		name string
		env  string
		args []string
		want string // the file that must carry the verdict
	}{
		{
			name: "flag wins over env",
			env:  envPath,
			args: []string{"--file", flagPath, `{"src":"flag"}`},
			want: flagPath,
		},
		{
			name: "env fallback intact",
			env:  envPath,
			args: []string{`{"src":"env"}`},
			want: envPath,
		},
		{
			name: "flag without env",
			env:  "",
			args: []string{"--file", flagPath, `{"src":"flag-only"}`},
			want: flagPath,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TQ_RESULT_FILE", tc.env)

			if err := cmdVerdict(tc.args); err != nil {
				t.Fatalf("verdict: %v", err)
			}

			raw, err := os.ReadFile(tc.want)
			if err != nil {
				t.Fatalf("read %s: %v", tc.want, err)
			}

			if !strings.Contains(string(raw), `"src"`) {
				t.Fatalf("file %s = %s, want the verdict JSON", tc.want, raw)
			}

			if tc.want == flagPath && tc.env != "" {
				if _, err := os.Stat(envPath); !os.IsNotExist(err) {
					t.Fatalf("env path %s must stay untouched when the flag wins", envPath)
				}
			}
		})
	}
}

func TestCmdVerdictNeedsExactlyOneArg(t *testing.T) {
	if err := cmdVerdict(nil); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("err = %v, want usage", err)
	}
}
