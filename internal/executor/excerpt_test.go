package executor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

func TestExcerpt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "single line",
			in:   "fix the login bug",
			want: "fix the login bug",
		},
		{
			name: "surrounding whitespace trimmed",
			in:   "  fix the login bug\nsecond line",
			want: "fix the login bug",
		},
		{
			name: "first line only",
			in:   "first\nsecond\nthird",
			want: "first",
		},
		{
			name: "at the bound stays intact",
			in:   strings.Repeat("a", excerptMaxLen) + "\nmore",
			want: strings.Repeat("a", excerptMaxLen),
		},
		{
			name: "over the bound gains ellipsis",
			in:   strings.Repeat("a", excerptMaxLen+10) + "\nmore",
			want: strings.Repeat("a", excerptMaxLen) + "…",
		},
		{
			name: "trailing space inside the cut is trimmed",
			in:   strings.Repeat("a", excerptMaxLen-2) + "  b\nnext",
			want: strings.Repeat("a", excerptMaxLen-2) + "…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Excerpt(tt.in); got != tt.want {
				t.Errorf("Excerpt() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestExcerptRuneSafe pins the byte cut backing off to a whole UTF-8
// boundary: a multi-byte prompt truncated at excerptMaxLen must not carry
// a broken rune (the row-266 class — historical copies all cut mid-rune).
func TestExcerptRuneSafe(t *testing.T) {
	t.Parallel()

	prefix := strings.Repeat("a", excerptMaxLen-1)
	multiByte := prefix + "漢漢漢漢" + strings.Repeat("b", 50)

	got := Excerpt(multiByte)

	if !utf8.ValidString(got) {
		t.Fatalf("excerpt is not valid UTF-8: %q", got)
	}

	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated excerpt %q must end with the ellipsis", got)
	}

	if body := strings.TrimSuffix(got, "…"); !utf8.ValidString(body) || strings.ContainsRune(body, utf8.RuneError) {
		t.Fatalf("excerpt body carries a broken rune: %q", body)
	}

	if short := Excerpt("漢字 only"); short != "漢字 only" {
		t.Fatalf("short multi-byte text = %q, want unchanged", short)
	}

	if firstLine := Excerpt("line one\nline two 漢"); firstLine != "line one" {
		t.Fatalf("first-line cut = %q, want 'line one'", firstLine)
	}
}

// TestRecordRunOutcome pins the paid-run bookkeeping: the sidecar path is
// stamped through the embedded sessionUsage (log_path inside the recorded
// detail, honoring TQ_LOG_DIR), the result's own fields ride along, and an
// unset TQ_LOG_DIR leaves log_path empty instead of erroring.
func TestRecordRunOutcome(t *testing.T) {
	t.Run("sidecar path honored", func(t *testing.T) {
		logDir := t.TempDir()
		t.Setenv("TQ_LOG_DIR", logDir)

		ctx, sink := NewSink(context.Background())

		result := ReviewResult{Summary: "clean"}
		recordRunOutcome(ctx, &result, "full output", "verify tail", task.ID("000001a0recordrunoutcome0000001"))

		if result.LogPath == "" {
			t.Fatal("LogPath empty, want the sidecar path")
		}

		if filepath.Dir(result.LogPath) != logDir {
			t.Fatalf("LogPath dir = %s, want %s", filepath.Dir(result.LogPath), logDir)
		}

		if _, err := os.Stat(result.LogPath); err != nil {
			t.Fatalf("sidecar not written: %v", err)
		}

		detail := sink.Detail()
		if len(detail) == 0 {
			t.Fatal("sink detail empty")
		}

		var decoded map[string]any
		if err := json.Unmarshal(detail, &decoded); err != nil {
			t.Fatalf("decode detail: %v", err)
		}

		if decoded["log_path"] != result.LogPath {
			t.Fatalf("detail log_path = %v, want %s", decoded["log_path"], result.LogPath)
		}

		if decoded["summary"] != "clean" {
			t.Fatalf("detail summary = %v, want clean", decoded["summary"])
		}
	})

	t.Run("no log dir leaves log_path empty", func(t *testing.T) {
		t.Setenv("TQ_LOG_DIR", "")

		ctx, sink := NewSink(context.Background())

		result := ReviewResult{Summary: "clean"}
		recordRunOutcome(ctx, &result, "out", "", task.ID("000001a0recordrunoutcome0000002"))

		if result.LogPath != "" {
			t.Fatalf("LogPath = %q, want empty without TQ_LOG_DIR", result.LogPath)
		}

		var decoded map[string]any
		if err := json.Unmarshal(sink.Detail(), &decoded); err != nil {
			t.Fatalf("decode detail: %v", err)
		}

		if _, present := decoded["log_path"]; present {
			t.Fatalf("detail must omit empty log_path: %v", decoded)
		}
	})
}
