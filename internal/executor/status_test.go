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

func statusTaskT(t *testing.T, p StatusPayload) task.Task {
	t.Helper()

	payload, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("render status payload: %v", err)
	}

	return task.Task{Type: TaskTypeStatus, Payload: payload}
}

func statusPayloadT() StatusPayload {
	return StatusPayload{
		Repo:    "demo",
		Project: "demo",
		Completed: []StatusCompletion{
			{TaskID: "t-1", Item: "add the frobnicator", Commit: "abc1234", Files: []string{"frob.go"}},
			{TaskID: "t-2", Item: "fix the flimflam"},
		},
	}
}

func TestParseStatusResultTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		output       string
		wantErr      bool
		wantReport   string
		wantNextItem int
	}{
		{
			name:         "report with next items",
			output:       "reporting...\nTQ_RESULT: {\"report\":\"docs/status/2026-09-08_20-40_demo.md\",\"next_items\":12}\n",
			wantReport:   "docs/status/2026-09-08_20-40_demo.md",
			wantNextItem: 12,
		},
		{
			name:       "zero next items is fine",
			output:     "TQ_RESULT: {\"report\":\"docs/status/x.md\",\"next_items\":0}",
			wantReport: "docs/status/x.md",
		},
		{
			name:    "no marker is invalid",
			output:  "I reported, trust me",
			wantErr: true,
		},
		{
			name:    "broken json is invalid",
			output:  "TQ_RESULT: {\"report\": docs/status/x.md}",
			wantErr: true,
		},
		{
			name:    "missing report path is invalid",
			output:  "TQ_RESULT: {\"next_items\":3}",
			wantErr: true,
		},
		{
			name:    "blank report path is invalid",
			output:  "TQ_RESULT: {\"report\":\"   \",\"next_items\":3}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseStatusResult(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseStatusResult(%q) = %+v, want error", tt.output, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseStatusResult(%q): %v", tt.output, err)
			}

			if got.Report != tt.wantReport {
				t.Fatalf("report = %q, want %q", got.Report, tt.wantReport)
			}

			if got.NextItems != tt.wantNextItem {
				t.Fatalf("next_items = %d, want %d", got.NextItems, tt.wantNextItem)
			}
		})
	}
}

// TestStatusExecutorHappyPath pins the done-prompt flow: the stub agent
// writes the report inside the repo (the executor runs it with cwd=repo),
// emits the contract line, and the structured detail lands on the sink.
func TestStatusExecutorHappyPath(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	body := "mkdir -p docs/status\n"
	body += "echo '# report' > docs/status/2026-09-08_20-40_demo.md\n"
	body += `printf '%s\n' 'TQ_RESULT: {"report":"docs/status/2026-09-08_20-40_demo.md","next_items":5}'`

	e := &StatusExecutor{Agent: &AgentExecutor{Bin: makeStubAgent(t, body)}}

	ctx, sink := NewSink(context.Background())
	if err := e.Execute(ctx, statusTaskT(t, StatusPayload{
		Repo:      repo,
		Project:   "demo",
		Completed: []StatusCompletion{{TaskID: "t-1", Item: "do the thing"}},
	})); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var got StatusResult
	if err := json.Unmarshal(sink.Detail(), &got); err != nil {
		t.Fatalf("sink detail %s: %v", sink.Detail(), err)
	}

	if got.Report != "docs/status/2026-09-08_20-40_demo.md" || got.NextItems != 5 {
		t.Fatalf("detail = %+v, want report path and next_items=5", got)
	}
}

func TestStatusExecutorMissingReportFileFailsRetryable(t *testing.T) {
	t.Parallel()

	bin := makeStubAgent(t, `printf '%s\n' 'TQ_RESULT: {"report":"docs/status/never-written.md","next_items":1}'`)
	e := &StatusExecutor{Agent: &AgentExecutor{Bin: bin}}

	err := e.Execute(context.Background(), statusTaskT(t, StatusPayload{
		Repo: t.TempDir(), Project: "demo",
		Completed: []StatusCompletion{{TaskID: "t-1", Item: "item"}},
	}))
	if err == nil {
		t.Fatal("Execute with missing report file must fail")
	}

	if _, ok := errors.AsType[*PermanentError](err); ok {
		t.Fatalf("missing report file must be retryable, got permanent: %v", err)
	}
}

// TestStatusExecutorPathConfinement: a model-echoed path escaping the repo is
// refused even when the file exists outside it.
func TestStatusExecutorPathConfinement(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(dir, "outside.md")
	if err := os.WriteFile(outside, []byte("stolen"), 0o644); err != nil {
		t.Fatal(err)
	}

	rel, err := filepath.Rel(repo, outside)
	if err != nil {
		t.Fatal(err)
	}

	bin := makeStubAgent(t, "printf '%s\\n' 'TQ_RESULT: {\"report\":\""+rel+"\",\"next_items\":0}'")
	e := &StatusExecutor{Agent: &AgentExecutor{Bin: bin}}

	err = e.Execute(context.Background(), statusTaskT(t, StatusPayload{
		Repo: repo, Project: "demo",
		Completed: []StatusCompletion{{TaskID: "t-1", Item: "item"}},
	}))
	if err == nil {
		t.Fatal("Execute with escaping report path must fail")
	}

	if !strings.Contains(err.Error(), "not repo-relative") {
		t.Fatalf("want confinement error, got: %v", err)
	}
}

func TestStatusExecutorInvalidOutputFailsAttemptRetryable(t *testing.T) {
	t.Parallel()

	bin := makeStubAgent(t, "echo 'status: good, probably'")
	e := &StatusExecutor{Agent: &AgentExecutor{Bin: bin}}

	err := e.Execute(context.Background(), statusTaskT(t, StatusPayload{
		Repo: t.TempDir(), Project: "demo",
		Completed: []StatusCompletion{{TaskID: "t-1", Item: "item"}},
	}))
	if err == nil {
		t.Fatal("Execute with no TQ_RESULT line must fail")
	}

	if _, ok := errors.AsType[*PermanentError](err); ok {
		t.Fatalf("invalid output must be retryable, got permanent: %v", err)
	}
}

func TestStatusExecutorPayloadContractMissesArePermanent(t *testing.T) {
	t.Parallel()

	bin := makeStubAgent(t, ":")
	e := &StatusExecutor{Agent: &AgentExecutor{Bin: bin}}

	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty payload", raw: ""},
		{name: "broken json", raw: "{"},
		{name: "no repo", raw: `{"project":"demo","completed":[{"task_id":"t-1","item":"x"}]}`},
		{name: "no project", raw: `{"repo":"/tmp/demo","completed":[{"task_id":"t-1","item":"x"}]}`},
		{name: "empty window", raw: `{"repo":"/tmp/demo","project":"demo","completed":[]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := e.Execute(context.Background(), task.Task{Type: TaskTypeStatus, Payload: []byte(tt.raw)})

			_, ok := errors.AsType[*PermanentError](err)
			if err == nil || !ok {
				t.Fatalf("payload miss %q must be permanent, got: %v", tt.name, err)
			}
		})
	}
}

// TestStatusExecutorPromptCarriesWindowContext asserts the done prompt names
// the window's tasks, the TODO_LIST.md loop-closing rules, the report
// destination, and the output contract.
func TestStatusExecutorPromptCarriesWindowContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	promptLog := filepath.Join(dir, "prompt.log")
	bin := filepath.Join(dir, "prompt-agent")

	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\"; done > \"" + promptLog + "\"\n"
	script += "mkdir -p docs/status\n"
	script += "echo '# report' > docs/status/r.md\n"
	script += "printf '%s\\n' 'TQ_RESULT: {\"report\":\"docs/status/r.md\",\"next_items\":1}'\n"

	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}

	setupGitRepo(t, repo)

	e := &StatusExecutor{Agent: &AgentExecutor{Bin: bin}}

	payload := statusPayloadT()
	payload.Repo = repo

	if err := e.Execute(context.Background(), statusTaskT(t, payload)); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatal(err)
	}

	prompt := string(raw)

	for _, want := range []string{
		"t-1",
		"add the frobnicator",
		"abc1234",
		"t-2",
		"docs/status/<YYYY-MM-DD_HH-MM_WELL-NAMED>.md",
		"TODO_LIST.md",
		"— BLOCKED:",
		"Never push",
		"TQ_RESULT:",
		"status reporter",
		"Hard scope rule",
		"append-only",
		"do not fix it yourself",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

// TestStatusExecutorVerifyGateGatesCompletion pins the repo quality gate: the
// reporter commits, so it can break the tree it just reported on — a failing
// verify command must fail the attempt (retryable), a passing or absent one
// must complete.
func TestStatusExecutorVerifyGateGatesCompletion(t *testing.T) {
	t.Parallel()

	report := "mkdir -p docs/status\n"
	report += "echo '# report' > docs/status/r.md\n"
	report += `printf '%s\n' 'TQ_RESULT: {"report":"docs/status/r.md","next_items":0}'`

	tests := []struct {
		name    string
		verify  string
		wantErr bool
	}{
		{name: "passing verify completes", verify: "test -f docs/status/r.md"},
		{name: "empty verify resolves to nothing", verify: ""},
		{name: "failing verify fails the attempt", verify: "exit 7", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := &StatusExecutor{Agent: &AgentExecutor{Bin: makeStubAgent(t, report)}}

			err := e.Execute(context.Background(), statusTaskT(t, StatusPayload{
				Repo: t.TempDir(), Project: "demo",
				Completed: []StatusCompletion{{TaskID: "t-1", Item: "item"}},
				Verify:    tt.verify,
			}))
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Execute: %v", err)
				}

				return
			}

			if err == nil {
				t.Fatal("failing verify must fail the attempt")
			}

			if !strings.Contains(err.Error(), "verify failed") {
				t.Fatalf("want verify-gate error, got: %v", err)
			}

			if _, ok := errors.AsType[*PermanentError](err); ok {
				t.Fatalf("verify failure must be retryable, got permanent: %v", err)
			}
		})
	}
}
