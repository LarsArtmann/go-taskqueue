//go:build unix

package executor

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

func dlqFixTaskT(t *testing.T, p DLQFixPayload) task.Task {
	t.Helper()

	payload, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("render dlqfix payload: %v", err)
	}

	return task.Task{Type: TaskTypeDLQFix, Payload: payload}
}

func TestParseDLQFixResultTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		output     string
		wantErr    bool
		want       DLQFixVerdict
		wantSHA    string
		wantSumm   string
		errContain string
	}{
		{
			name:     "fixed with summary and commit",
			output:   "diagnosing...\nTQ_RESULT: {\"verdict\":\"fixed\",\"summary\":\"nil map write in shim\",\"commit_sha\":\"abc1234\"}\n",
			want:     VerdictFixed,
			wantSHA:  "abc1234",
			wantSumm: "nil map write in shim",
		},
		{
			name:     "wontfix with reason",
			output:   "TQ_RESULT: {\"verdict\":\"wontfix\",\"summary\":\"provider quota exhausted; retry later\"}\n",
			want:     VerdictWontFix,
			wantSumm: "provider quota exhausted; retry later",
		},
		{
			name:     "verdict case-insensitive",
			output:   "TQ_RESULT: {\"verdict\":\"FIXED\",\"summary\":\"ok\"}\n",
			want:     VerdictFixed,
			wantSumm: "ok",
		},
		{
			name:       "wontfix without summary is invalid",
			output:     "TQ_RESULT: {\"verdict\":\"wontfix\"}\n",
			wantErr:    true,
			errContain: "no summary",
		},
		{
			name:       "fixed without summary is invalid (a diagnosis is the product)",
			output:     "TQ_RESULT: {\"verdict\":\"fixed\",\"commit_sha\":\"abc\"}\n",
			wantErr:    true,
			errContain: "no summary",
		},
		{
			name:       "unknown verdict",
			output:     "TQ_RESULT: {\"verdict\":\"maybe\",\"summary\":\"x\"}\n",
			wantErr:    true,
			errContain: "unknown verdict",
		},
		{
			name:       "missing verdict field",
			output:     "TQ_RESULT: {\"summary\":\"x\"}\n",
			wantErr:    true,
			errContain: "no verdict field",
		},
		{
			name:       "no TQ_RESULT line at all",
			output:     "all work and no verdict\n",
			wantErr:    true,
			errContain: "no TQ_RESULT line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseDLQFixResult(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDLQFixResult(%q) = %+v, want error", tt.output, got)
				}

				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Fatalf("error %q missing %q", err, tt.errContain)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseDLQFixResult(%q): %v", tt.output, err)
			}

			if got.Verdict != tt.want || got.CommitSHA != tt.wantSHA || got.Summary != tt.wantSumm {
				t.Fatalf("result = %+v, want verdict %q sha %q summary %q", got, tt.want, tt.wantSHA, tt.wantSumm)
			}
		})
	}
}

// TestDLQFixExecutorVerdictContract pins that BOTH verdicts complete the
// task and the parsed result rides the sink (the disposition is the
// sweeper's job, never the exit code).
func TestDLQFixExecutorVerdictContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		agentOut string
		want     DLQFixVerdict
		wantSHA  string
		wantSumm string
	}{
		{
			name:     "fixed completes",
			agentOut: "autopsy...\nTQ_RESULT: {\"verdict\":\"fixed\",\"summary\":\"bad flag name\",\"commit_sha\":\"deadbee\"}\n",
			want:     VerdictFixed,
			wantSHA:  "deadbee",
			wantSumm: "bad flag name",
		},
		{
			name:     "wontfix also completes",
			agentOut: "TQ_RESULT: {\"verdict\":\"wontfix\",\"summary\":\"needs credentials from the operator\"}\n",
			want:     VerdictWontFix,
			wantSumm: "needs credentials from the operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bin := makeStubAgent(t, "cat <<'EOF'\n"+tt.agentOut+"EOF")
			e := &DLQFixExecutor{Agent: &AgentExecutor{Bin: bin}}

			ctx, sink := NewSink(context.Background())
			if err := e.Execute(ctx, dlqFixTaskT(t, DLQFixPayload{
				Repo:     t.TempDir(),
				DeadTask: "t-dead",
				Work:     "ship the thing",
			})); err != nil {
				t.Fatalf("Execute: %v", err)
			}

			var got DLQFixResult
			if err := json.Unmarshal(sink.Detail(), &got); err != nil {
				t.Fatalf("sink detail %s: %v", sink.Detail(), err)
			}

			if got.Verdict != tt.want || got.CommitSHA != tt.wantSHA || got.Summary != tt.wantSumm {
				t.Fatalf("result = %+v, want verdict %q sha %q summary %q", got, tt.want, tt.wantSHA, tt.wantSumm)
			}
		})
	}
}

// TestDLQFixExecutorPromptCarriesEvidence runs a prompt-logging stub agent
// and asserts the autopsy prompt names the dead task, quotes its original
// work contract, carries the failure evidence, keeps the {{TASK_ID}}
// placeholder for runAgent's own-id substitution, and states the verdict
// contract.
func TestDLQFixExecutorPromptCarriesEvidence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	promptLog := filepath.Join(dir, "prompt.log")
	bin := filepath.Join(dir, "prompt-agent")

	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\"; done > \"" + promptLog + "\"\n"
	script += "printf '%s\\n' 'TQ_RESULT: {\"verdict\":\"fixed\",\"summary\":\"s\"}'\n"

	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	e := &DLQFixExecutor{Agent: &AgentExecutor{Bin: bin}}

	// The task carries its real queue id so the stub proves runAgent
	// substituted the AUTOPSY's own id into the footer instruction.
	payload, err := json.Marshal(DLQFixPayload{
		Repo:     dir,
		DeadTask: "000001a0deadtaskid00000000",
		DeadType: "agent",
		Work:     "fix the frobnicator\n\nTask-Queue-ID: 000001a0workrunid000000000",
		Failure:  FailureEvidence{Stage: "verify", ExitCode: 2, Tail: "FAIL: TestFrobnicate"},
		Attempts: 3,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if err := e.Execute(context.Background(), task.Task{
		ID:      "000001a0autopsyid000000000",
		Type:    TaskTypeDLQFix,
		Payload: payload,
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatal(err)
	}

	prompt := string(raw)

	for _, want := range []string{
		"000001a0deadtaskid00000000",
		"fix the frobnicator",
		"Task-Queue-ID: 000001a0workrunid000000000",
		`Stage "verify" exited with code 2`,
		"FAIL: TestFrobnicate",
		"burned 3 attempt(s)",
		// The autopsy's own id substituted into the footer instruction;
		// no placeholder may survive to the agent.
		"Task-Queue-ID: 000001a0autopsyid000000000",
		"TQ_RESULT:",
		"autopsy",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}

	if strings.Contains(prompt, "{{TASK_ID}}") {
		t.Error("prompt still carries an unresolved {{TASK_ID}} placeholder")
	}
}

// TestDLQFixExecutorRunsDirtyTree pins the evidence-over-cleanliness rule:
// an autopsy with the default (nil) RequireClean starts in a DIRTY repo —
// a dead agent's partial work is evidence, and a clean-tree preflight would
// requeue the autopsy forever on exactly the cases the feature exists for.
func TestDLQFixExecutorRunsDirtyTree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	setupGitRepo(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "partial.txt"), []byte("dead agent's unfinished work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := makeStubAgent(t, "printf '%s\\n' 'TQ_RESULT: {\"verdict\":\"wontfix\",\"summary\":\"s\"}'")
	e := &DLQFixExecutor{Agent: &AgentExecutor{Bin: bin}}

	if err := e.Execute(context.Background(), dlqFixTaskT(t, DLQFixPayload{
		Repo:     dir,
		DeadTask: "t-dead",
		Work:     "w",
	})); err != nil {
		t.Fatalf("Execute on dirty tree: %v", err)
	}
}

// TestDLQFixExecutorExplicitCleanStillPreflights pins the other direction:
// only an EXPLICIT require_clean=true restores the guard.
func TestDLQFixExecutorExplicitCleanStillPreflights(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	setupGitRepo(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "partial.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := makeStubAgent(t, "printf '%s\\n' 'TQ_RESULT: {\"verdict\":\"fixed\",\"summary\":\"s\"}'")
	e := &DLQFixExecutor{Agent: &AgentExecutor{Bin: bin}}

	yes := true
	err := e.Execute(context.Background(), dlqFixTaskT(t, DLQFixPayload{
		Repo:         dir,
		DeadTask:     "t-dead",
		Work:         "w",
		RequireClean: &yes,
	}))

	pre, ok := errors.AsType[*PreflightError](err)
	if !ok {
		t.Fatalf("err = %v, want PreflightError", err)
	}

	_ = pre
}

func TestDLQFixExecutorPayloadContractMissesArePermanent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     task.Task
		wantSub string
	}{
		{"empty payload", task.Task{Type: TaskTypeDLQFix}, "empty payload"},
		{"bad json", task.Task{Type: TaskTypeDLQFix, Payload: []byte("{oops")}, "decode payload"},
		{"missing dead_task", task.Task{Type: TaskTypeDLQFix, Payload: []byte(`{"repo":"/tmp/r","work":"w"}`)}, "needs non-empty"},
		{"missing work", task.Task{Type: TaskTypeDLQFix, Payload: []byte(`{"repo":"/tmp/r","dead_task":"t"}`)}, "needs non-empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := &DLQFixExecutor{Agent: &AgentExecutor{Bin: makeStubAgent(t, "false")}}

			err := e.Execute(context.Background(), tt.raw)
			perm, ok := errors.AsType[*PermanentError](err)
			if !ok {
				t.Fatalf("err = %v, want permanent containing %q", err, tt.wantSub)
			}

			if !strings.Contains(perm.Error(), tt.wantSub) {
				t.Fatalf("permanent err = %v, want containing %q", perm, tt.wantSub)
			}
		})
	}
}
