package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// fakeCrush writes a stub crush binary that logs its args to a file and
// mirrors the real CLI contract: exit 0 with the response on stdout, or exit
// non-zero when FAIL=1 is set in the repo dir.
func fakeCrush(t *testing.T) (bin string, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args.log")
	bin = filepath.Join(dir, "fake-crush")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + argsFile + "\"\n" +
		"if [ -f \"$(dirname \"$0\")/fail\" ]; then echo boom >&2; exit 4; fi\n" +
		"echo AGENT-OK\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, argsFile
}

func TestCrushExecutorHappyPath(t *testing.T) {
	bin, argsFile := fakeCrush(t)
	repo := t.TempDir()
	e := &CrushExecutor{Binary: bin, Yolo: true}

	payload, err := RenderCrushPayload(CrushPayload{
		Repo: repo, Prompt: "do the thing", Model: "prov/m1", Dedup: "k1",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = e.Execute(context.Background(), task.Task{
		ID: task.ID("x"), Type: TaskTypeCrush, Payload: payload,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	for _, line := range strings.Split(string(got), "\n") {
		if line != "" {
			args = append(args, line)
		}
	}
	want := []string{"run", "--quiet", "--cwd", repo, "--yolo", "--model", "prov/m1", "--", "do the thing"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q (full: %v)", i, args[i], want[i], args)
		}
	}
}

func TestCrushExecutorFailureCarriesOutput(t *testing.T) {
	bin, _ := fakeCrush(t)
	if err := os.WriteFile(filepath.Join(filepath.Dir(bin), "fail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e := &CrushExecutor{Binary: bin}
	payload, _ := RenderCrushPayload(CrushPayload{Repo: t.TempDir(), Prompt: "p"})
	err := e.Execute(context.Background(), task.Task{ID: "x", Type: TaskTypeCrush, Payload: payload})
	if err == nil {
		t.Fatal("expected error on non-zero exit")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error missing output tail: %v", err)
	}
}

func TestCrushExecutorPayloadValidation(t *testing.T) {
	e := &CrushExecutor{Binary: "crush-does-not-matter"}
	cases := []struct {
		name    string
		payload string
	}{
		{"empty", ""},
		{"not json", "nope"},
		{"missing prompt", `{"repo":"/tmp"}`},
		{"missing repo", `{"prompt":"hi"}`},
		{"repo not a dir", `{"repo":"/definitely/not/a/dir","prompt":"hi"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := e.Execute(context.Background(), task.Task{ID: "x", Type: TaskTypeCrush, Payload: []byte(tc.payload)})
			if err == nil {
				t.Fatalf("payload %q: expected error", tc.payload)
			}
			if !strings.Contains(err.Error(), "crush:") {
				t.Fatalf("error not crush-prefixed: %v", err)
			}
		})
	}
}

func TestCrushExecutorCancellation(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "slow-crush")
	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	e := &CrushExecutor{Binary: bin}
	payload, _ := RenderCrushPayload(CrushPayload{Repo: dir, Prompt: "p"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := e.Execute(ctx, task.Task{ID: "x", Type: TaskTypeCrush, Payload: payload})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("want cancellation error, got %v", err)
	}
}
