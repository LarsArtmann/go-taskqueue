package executor

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TaskTypeDLQFix runs a DLQ autopsy: a headless agent diagnoses WHY a
// dead-lettered task exhausted its retries and either fixes the root cause
// (the sweeper then rescues the original) or rules it unfixable with a
// reasoned verdict (the sweeper then dismisses the original). Register it
// under this type to make a pool autopsy-capable; internal/dlqfix mints the
// tasks and performs the dispositions.
//
// Both verdicts complete the task — a wontfix autopsy did its job; the
// disposition is mechanical, not the exit code. Only the output contract is
// enforced: no valid verdict line, or a wontfix without a reason, is a
// failed attempt.
const TaskTypeDLQFix = "dlqfix"

// DLQFixPayload is the payload contract for "dlqfix" tasks. Self-contained,
// like ReviewPayload: the evidence travels in the payload, so the executor
// stays a pure process runner.
type DLQFixPayload struct {
	// Repo is the repository the dead task ran in (same resolution rules as
	// AgentPayload.Repo). Required.
	Repo string `json:"repo"`
	// DeadTask is the dead task's queue id — lineage for `tq show` and the
	// sweeper's disposition target. Required.
	DeadTask string `json:"dead_task"`
	// DeadType is the dead task's type (the mint rule scopes autopsies to
	// agent tasks; the field rides along for the prompt and forensics).
	DeadType string `json:"dead_type,omitempty"`
	// Work is what the dead task was asked to do: the agent prompt, with
	// its {{TASK_ID}} placeholders already resolved the way the dead run
	// saw them. Required.
	Work string `json:"work"`
	// Failure is the structured forensics from the dead task's LAST
	// task.failed fact (stage, exit code, output tail). Zero when the
	// journal predates failure evidence.
	Failure FailureEvidence `json:"failure,omitempty"`
	// LastError is the dead task record's error text.
	LastError string `json:"last_error,omitempty"`
	// Attempts is how many attempts the dead task burned.
	Attempts int `json:"attempts,omitempty"`
	// Model optionally overrides the crush model for the autopsy.
	Model string `json:"model,omitempty"`
	// Yolo marks the autopsy as autonomous (same contract as
	// AgentPayload.Yolo: needs a repo-local crush config granting
	// permissions). Mints mirror the dead task's autonomy.
	Yolo bool `json:"yolo,omitempty"`
	// RequireClean defaults FALSE for autopsies, deliberately: a dead
	// agent's uncommitted partial work IS evidence, and a clean-tree
	// preflight would requeue the autopsy forever on exactly the cases the
	// feature exists for. The repo's .git absence skips the check as
	// usual. Minted payloads pin the false explicitly.
	RequireClean *bool `json:"require_clean,omitempty"`
	// TimeoutMinutes caps the autopsy run. Default 30 (a diagnosis is a
	// full work turn: read code, maybe reproduce, maybe fix).
	TimeoutMinutes int `json:"timeout_minutes,omitempty"`
}

// DLQFixVerdict is the autopsy's decision. Only these two values are
// accepted; anything else is invalid output and a failed attempt.
type DLQFixVerdict string

const (
	// VerdictFixed means the root cause lived in the repo and has been
	// fixed (and proven) by the autopsy; the original task is rescuable.
	VerdictFixed DLQFixVerdict = "fixed"
	// VerdictWontFix means the root cause is outside the repo (or the task
	// is unworkable for a stated reason); the original is dismissed. The
	// summary IS the reason — a wontfix without one is invalid output.
	VerdictWontFix DLQFixVerdict = "wontfix"
)

// DLQFixResult is the structured outcome of one autopsy run, stored in the
// completion fact detail (the sink convention) for the sweeper and `tq show`.
type DLQFixResult struct {
	Verdict DLQFixVerdict `json:"verdict"`
	// Summary is the diagnosis: for fixed, the root cause and the change;
	// for wontfix, the reason the task is unfixable from the repo.
	Summary string `json:"summary"`
	// CommitSHA is the fix commit the autopsy landed ("" for wontfix, or a
	// fixed diagnosis that needed no repo change).
	CommitSHA string `json:"commit_sha,omitempty"`
	// SessionID is the crush session id, same convention as AgentResult.
	SessionID string `json:"session_id,omitempty"`
	// LogPath is the sidecar file with the full autopsy output (written
	// when TQ_LOG_DIR is set), same convention as AgentResult.
	LogPath string `json:"log_path,omitempty"`
}

// defaultDLQFixTaskTimeout bounds one autopsy unless the payload overrides.
const defaultDLQFixTaskTimeout = 30 * time.Minute

// DLQFixExecutor runs autopsy agents. It reuses the agent run mechanics
// (repo resolution, autonomy preflight, process-group handling) through its
// Agent executor; the only differences are the prompt and the strict output
// contract.
type DLQFixExecutor struct {
	// Agent supplies the run mechanics and its configuration (Bin,
	// ProjectsDir, Yolo). nil selects a default AgentExecutor.
	Agent *AgentExecutor
}

func (e *DLQFixExecutor) base() *AgentExecutor {
	if e.Agent == nil {
		return &AgentExecutor{}
	}

	return e.Agent
}

// Execute runs the autopsy and enforces the verdict contract. fixed and
// wontfix both succeed; malformed output (or a wontfix without a reason)
// fails the attempt — retryable, the model may comply on a retry.
// Input-contract misses are permanent.
func (e *DLQFixExecutor) Execute(ctx context.Context, t task.Task) error {
	p, err := decodeDLQFixPayload(t)
	if err != nil {
		return err
	}

	agent := e.base()

	repoDir, err := agent.repoDir(p.Repo)
	if err != nil {
		return Permanent(err)
	}

	// Autopsies run dirty-capable by default: a dead agent's uncommitted
	// partial work IS evidence, and a clean-tree preflight would requeue
	// the autopsy forever on exactly the cases the feature exists for.
	// Only an explicit true restores the guard (repos without .git skip
	// it as usual).
	if p.RequireClean != nil && *p.RequireClean {
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
			if err := assertCleanTree(ctx, repoDir); err != nil {
				return &PreflightError{Cause: err}
			}
		}
	}

	timeout := defaultDLQFixTaskTimeout
	if p.TimeoutMinutes > 0 {
		timeout = time.Duration(p.TimeoutMinutes) * time.Minute
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := agent.runAgent(runCtx, repoDir, &AgentPayload{
		Repo:   p.Repo,
		Prompt: dlqFixPrompt(p),
		Model:  p.Model,
		Yolo:   p.Yolo,
	}, t.ID)
	if err != nil {
		return err
	}

	result, err := ParseDLQFixResult(output)
	if err != nil {
		return fmt.Errorf("dlqfix: %w", err)
	}

	result.SessionID = ExtractSessionID(output)
	result.LogPath = writeOutputSidecar(t.ID, output, "")

	detail, _ := json.Marshal(result)
	SetResultDetail(ctx, detail)

	return nil
}

// decodeDLQFixPayload enforces the input contract: present, parseable, and
// carrying the three fields without which no autopsy can start.
func decodeDLQFixPayload(t task.Task) (DLQFixPayload, error) {
	if len(t.Payload) == 0 {
		return DLQFixPayload{}, Permanent(errors.New("dlqfix: empty payload, want {repo, dead_task, work}"))
	}

	var p DLQFixPayload

	if err := json.Unmarshal(t.Payload, &p); err != nil {
		return DLQFixPayload{}, Permanent(fmt.Errorf("dlqfix: decode payload: %w", err))
	}

	if p.Repo == "" || p.DeadTask == "" || p.Work == "" {
		return DLQFixPayload{}, Permanent(errors.New("dlqfix: payload needs non-empty repo, dead_task and work"))
	}

	return p, nil
}

// dlqFixPrompt builds the autopsy instruction: the dead task's original
// contract, the failure evidence, the decide-fix-or-rule workflow, and the
// exact output contract. The {{TASK_ID}} placeholder is left in the FOOTER
// instruction on purpose — runAgent resolves it to the AUTOPSY's own id at
// run time, so fix commits cross-reference the autopsy run. The quoted work
// text is emitted verbatim (its footers stay as the dead run saw them).
func dlqFixPrompt(p DLQFixPayload) string {
	var b strings.Builder

	b.WriteString(
		"You are a senior engineer performing an autopsy. A task in this repository exhausted " +
			"its retries and was dead-lettered. Diagnose the root cause from the evidence, then EITHER " +
			"fix it OR rule the task unfixable with a precise reason. Diagnose FIRST; change nothing " +
			"before you understand the failure.\n\n",
	)

	b.WriteString("## The dead task\n\n")

	if p.DeadType != "" {
		b.WriteString("Queue task " + p.DeadTask + " (type " + p.DeadType + ") ran:\n\n")
	} else {
		b.WriteString("Queue task " + p.DeadTask + " ran:\n\n")
	}

	b.WriteString(p.Work + "\n\n")

	if p.Attempts > 0 {
		b.WriteString(fmt.Sprintf("It burned %d attempt(s) before dying", p.Attempts))
		if p.LastError != "" {
			b.WriteString("; last error: " + p.LastError)
		}

		b.WriteString("\n\n")
	} else if p.LastError != "" {
		b.WriteString("Last error: " + p.LastError + "\n\n")
	}

	if p.Failure.Stage != "" || p.Failure.Tail != "" {
		b.WriteString("## Failure evidence\n\n")

		if p.Failure.Stage != "" {
			b.WriteString(fmt.Sprintf("Stage %q exited with code %d. Last output:\n\n", p.Failure.Stage, p.Failure.ExitCode))
		} else {
			b.WriteString("Last output:\n\n")
		}

		b.WriteString("```\n" + strings.TrimSpace(p.Failure.Tail) + "\n```\n\n")
	}

	b.WriteString(`## Decide

1. The working tree may contain the dead run's uncommitted partial changes:
   inspect git status first. They are evidence — salvage what helps the
   diagnosis, revert what would corrupt a fix.
2. READ the relevant code. Reproduce the failing stage cheaply if you can.
3. If the root cause lives INSIDE this repository: apply the MINIMAL fix, then
   prove it by re-running the failed stage (or the repo's own gates). Commit
   the fix with a message ending in this exact footer line (you have explicit
   permission to commit for this task):

Task-Queue-ID: {{TASK_ID}}

4. If the root cause is OUTSIDE this repository (provider outage, operator
   error, missing external resource or credentials, a change that belongs in
   another repo): change NOTHING. Rule the task unfixable instead.

Never push.

## Verdict

End your reply with EXACTLY ONE line of this shape and nothing after it:

TQ_RESULT: {"verdict":"fixed","summary":"root cause + what you changed","commit_sha":"<sha or empty>"}

or

TQ_RESULT: {"verdict":"wontfix","summary":"why this task cannot be fixed from inside this repository"}

A wontfix without a summary is invalid output.
`)

	return b.String()
}

// ParseDLQFixResult extracts and validates the verdict JSON from autopsy
// output. Strict where it matters: the verdict must be exactly fixed or
// wontfix (case-insensitive), the JSON must parse, and a wontfix must carry
// a non-empty summary (an unexplained dismissal is the behavior being
// automated away).
func ParseDLQFixResult(output string) (DLQFixResult, error) {
	raw, err := ResultLine(output)
	if err != nil {
		return DLQFixResult{}, err
	}

	var parsed struct {
		Verdict   string `json:"verdict"`
		Summary   string `json:"summary"`
		CommitSHA string `json:"commit_sha"`
	}

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return DLQFixResult{}, fmt.Errorf("verdict JSON does not parse: %w", err)
	}

	switch v := DLQFixVerdict(strings.ToLower(strings.TrimSpace(parsed.Verdict))); v {
	case VerdictFixed, VerdictWontFix:
		summary := strings.TrimSpace(parsed.Summary)
		if summary == "" {
			return DLQFixResult{}, errors.New("verdict JSON has no summary (a diagnosis is required for either verdict)")
		}

		return DLQFixResult{Verdict: v, Summary: summary, CommitSHA: strings.TrimSpace(parsed.CommitSHA)}, nil
	case "":
		return DLQFixResult{}, errors.New("verdict JSON has no verdict field")
	default:
		return DLQFixResult{}, fmt.Errorf("unknown verdict %q (want fixed or wontfix)", parsed.Verdict)
	}
}
