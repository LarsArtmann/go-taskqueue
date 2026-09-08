//go:build unix

package executor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

func reviewTaskT(t *testing.T, p ReviewPayload) task.Task {
	t.Helper()

	payload, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("render review payload: %v", err)
	}

	return task.Task{Type: TaskTypeReview, Payload: payload}
}

func TestParseResultTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		output     string
		wantErr    bool
		want       ReviewVerdict
		wantSev    []string
		wantTitles []string
	}{
		{
			name:    "approve with summary",
			output:  "looks good\nTQ_RESULT: {\"verdict\":\"approve\",\"summary\":\"clean change\",\"findings\":[]}\n",
			want:    VerdictApprove,
			wantSev: []string{},
		},
		{
			name:       "request changes with findings",
			output:     "TQ_RESULT: {\"verdict\":\"request_changes\",\"summary\":\"broken edge case\",\"findings\":[{\"title\":\"nil map write\",\"severity\":\"HIGH\",\"detail\":\"see x.go:3\"}]}\n",
			want:       VerdictRequestChanges,
			wantSev:    []string{"high"},
			wantTitles: []string{"nil map write"},
		},
		{
			name:    "verdict case-insensitive",
			output:  "TQ_RESULT: {\"verdict\":\"Approve\"}\n",
			want:    VerdictApprove,
			wantSev: []string{},
		},
		{
			name:    "empty severity degrades to medium",
			output:  "TQ_RESULT: {\"verdict\":\"request_changes\",\"findings\":[{\"title\":\"t\",\"severity\":\"\"}]}\n",
			want:    VerdictRequestChanges,
			wantSev: []string{"medium"},
		},
		{
			name:    "unknown severity degrades to medium",
			output:  "TQ_RESULT: {\"verdict\":\"request_changes\",\"findings\":[{\"title\":\"t\",\"severity\":\"catastrophic\"}]}\n",
			want:    VerdictRequestChanges,
			wantSev: []string{"medium"},
		},
		{
			name:    "findings without titles are dropped",
			output:  "TQ_RESULT: {\"verdict\":\"request_changes\",\"findings\":[{\"title\":\"\",\"severity\":\"low\"},{\"title\":\"real\"}]}\n",
			want:    VerdictRequestChanges,
			wantSev: []string{"medium"},
		},
		{
			name:    "request changes without findings is invalid",
			output:  "TQ_RESULT: {\"verdict\":\"request_changes\"}\n",
			wantErr: true,
		},
		{
			name:    "no marker is invalid",
			output:  "all good, ship it",
			wantErr: true,
		},
		{
			name:    "broken json is invalid",
			output:  "TQ_RESULT: {\"verdict\": approve}",
			wantErr: true,
		},
		{
			name:    "unknown verdict is invalid",
			output:  "TQ_RESULT: {\"verdict\":\"ship_it\"}\n",
			wantErr: true,
		},
		{
			name:    "missing verdict is invalid",
			output:  "TQ_RESULT: {\"summary\":\"nothing\"}\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseResult(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseResult(%q) = %+v, want error", tt.output, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseResult(%q): %v", tt.output, err)
			}

			if got.Verdict != tt.want {
				t.Fatalf("verdict = %q, want %q", got.Verdict, tt.want)
			}

			if len(got.Findings) != len(tt.wantSev) {
				t.Fatalf("findings = %+v, want %d", got.Findings, len(tt.wantSev))
			}

			for i, f := range got.Findings {
				if f.Severity != tt.wantSev[i] {
					t.Fatalf("finding %d severity = %q, want %q", i, f.Severity, tt.wantSev[i])
				}

				if len(tt.wantTitles) > i && f.Title != tt.wantTitles[i] {
					t.Fatalf("finding %d title = %q, want %q", i, f.Title, tt.wantTitles[i])
				}
			}
		})
	}
}

// TestReviewExecutorVerdictContract pins the reviewer flow: the review
// prompt carries the item, the commit and the read-only rule; the verdict
// JSON (not the exit code) decides success; approve and request_changes
// both complete the task with the structured detail on the sink.
func TestReviewExecutorVerdictContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		agentOut string
		want     ReviewVerdict
	}{
		{
			name:     "approve completes",
			agentOut: "reviewing...\nTQ_RESULT: {\"verdict\":\"approve\",\"summary\":\"ok\"}\n",
			want:     VerdictApprove,
		},
		{
			name:     "request_changes also completes",
			agentOut: `TQ_RESULT: {"verdict":"request_changes","findings":[{"title":"add test","severity":"medium"}]}` + "\n",
			want:     VerdictRequestChanges,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bin := makeStubAgent(t, "cat <<'EOF'\n"+tt.agentOut+"EOF")
			e := &ReviewExecutor{Agent: &AgentExecutor{Bin: bin}}

			ctx, sink := NewSink(context.Background())
			if err := e.Execute(ctx, reviewTaskT(t, ReviewPayload{
				Repo:         t.TempDir(),
				ReviewedTask: "t-1",
				Item:         "write the thing",
			})); err != nil {
				t.Fatalf("Execute: %v", err)
			}

			var got ReviewResult
			if err := json.Unmarshal(sink.Detail(), &got); err != nil {
				t.Fatalf("sink detail %s: %v", sink.Detail(), err)
			}

			if got.Verdict != tt.want {
				t.Fatalf("verdict = %q, want %q", got.Verdict, tt.want)
			}
		})
	}
}

// TestReviewExecutorPromptCarriesReviewContext runs the stub agent in a
// real git repo and asserts the prompt it received names the item, the
// commit SHA, the changed files, the read-only rule and the output
// contract.
func TestReviewExecutorPromptCarriesReviewContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	promptLog := filepath.Join(dir, "prompt.log")
	bin := filepath.Join(dir, "prompt-agent")

	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\"; done > \"" + promptLog + "\"\n"
	script += "printf '%s\\n' 'TQ_RESULT: {\"verdict\":\"approve\"}'\n"

	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}

	setupGitRepo(t, repo)

	e := &ReviewExecutor{Agent: &AgentExecutor{Bin: bin}}
	if err := e.Execute(context.Background(), reviewTaskT(t, ReviewPayload{
		Repo:         repo,
		ReviewedTask: "t-42",
		Item:         "add the frobnicator",
		CommitSHA:    "abc1234",
		FilesChanged: []string{"frob.go", "frob_test.go"},
	})); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatal(err)
	}

	prompt := string(raw)

	for _, want := range []string{
		"add the frobnicator",
		"git show abc1234",
		"frob.go, frob_test.go",
		"do not create, modify, or delete",
		"TQ_RESULT:",
		"senior code reviewer",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestReviewExecutorInvalidOutputFailsAttemptRetryable(t *testing.T) {
	t.Parallel()

	bin := makeStubAgent(t, "echo 'I reviewed it, trust me'")
	e := &ReviewExecutor{Agent: &AgentExecutor{Bin: bin}}

	err := e.Execute(context.Background(), reviewTaskT(t, ReviewPayload{
		Repo: t.TempDir(), ReviewedTask: "t-1", Item: "item",
	}))
	if err == nil {
		t.Fatal("Execute with no verdict line must fail")
	}

	var perm *PermanentError
	if errors.As(err, &perm) {
		t.Fatalf("invalid output must be retryable, got permanent: %v", err)
	}
}

func TestReviewExecutorPayloadContractMissesArePermanent(t *testing.T) {
	t.Parallel()

	bin := makeStubAgent(t, ":")
	e := &ReviewExecutor{Agent: &AgentExecutor{Bin: bin}}

	tests := []struct {
		name  string
		raw   string
		valid bool
	}{
		{name: "empty payload", raw: ""},
		{name: "not json", raw: "garbage"},
		{name: "missing item", raw: `{"repo":"/tmp/r","reviewed_task":"t-1"}`},
		{name: "missing repo", raw: `{"reviewed_task":"t-1","item":"i"}`},
		{name: "missing reviewed_task", raw: `{"repo":"/tmp/r","item":"i"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := e.Execute(context.Background(), task.Task{Type: TaskTypeReview, Payload: json.RawMessage(tt.raw)})
			if err == nil {
				t.Fatal("want error")
			}

			var perm *PermanentError
			if !errors.As(err, &perm) {
				t.Fatalf("input-contract miss must be permanent, got: %v", err)
			}
		})
	}
}

func TestReviewExecutorRefusesDirtyTreeAsPreflight(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	setupGitRepo(t, repo)

	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := makeStubAgent(t, ":")
	e := &ReviewExecutor{Agent: &AgentExecutor{Bin: bin}}

	err := e.Execute(context.Background(), reviewTaskT(t, ReviewPayload{
		Repo: repo, ReviewedTask: "t-1", Item: "item",
	}))

	var pre *PreflightError
	if !errors.As(err, &pre) {
		t.Fatalf("dirty tree must be preflight, got: %v", err)
	}
}
