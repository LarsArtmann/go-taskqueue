package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TaskTypeStatus runs the "done prompt": after a window of agent completions,
// a headless agent writes a full status report (docs/status/<timestamp>.md)
// and closes the loop by appending the next work items to TODO_LIST.md, where
// the next harvest tick picks them up. Register it under this type to make a
// pool status-capable; internal/status mints the tasks.
const TaskTypeStatus = "status"

// StatusCompletion is one completed agent task in the window a status report
// covers. The item is an excerpt of the agent's instruction (first line,
// bounded) — the full prompt stays in `tq show`.
type StatusCompletion struct {
	TaskID      string   `json:"task_id"`
	Item        string   `json:"item"`
	Commit      string   `json:"commit,omitempty"`
	Files       []string `json:"files,omitempty"`
	CompletedAt string   `json:"completed_at,omitempty"`
}

// StatusPayload is the payload contract for "status" tasks. Self-contained,
// like ReviewPayload: everything the reporting agent needs travels in the
// payload, so the executor stays a pure process runner.
type StatusPayload struct {
	// Repo is the repository being reported on (same resolution rules as
	// AgentPayload.Repo). Required.
	Repo string `json:"repo"`
	// Project is the queue project (repo name) the window belongs to.
	// Required; also the sweeper's loop guard identity.
	Project string `json:"project"`
	// Completed lists the agent tasks in this report's window, oldest
	// first. Bounded by the sweeper; never empty for a minted task.
	Completed []StatusCompletion `json:"completed"`
	// Model optionally overrides the crush model for the reporting agent.
	Model string `json:"model,omitempty"`
	// Yolo marks the reporting agent as autonomous (same contract as
	// AgentPayload.Yolo). Pools typically mirror the window's autonomy.
	Yolo bool `json:"yolo,omitempty"`
	// RequireClean refuses to start unless the repo tree is clean — the
	// reporter must not trample work-in-progress, and commits its own
	// report afterwards. Default true. Same escape hatch as AgentPayload.
	RequireClean *bool `json:"require_clean,omitempty"`
	// TimeoutMinutes caps the report run. Default 15.
	TimeoutMinutes int `json:"timeout_minutes,omitempty"`
	// Verify is the repo quality gate that must exit 0 after the report
	// run (the reporter commits, so a broken tree is a real possibility).
	// Empty means the usual resolution: the repo's .tq-verify file, then
	// auto-detection — identical to AgentPayload.Verify.
	Verify string `json:"verify,omitempty"`
}

// StatusResult is the structured outcome of one status run, stored in the
// completion fact detail (the sink convention) for `tq show` and the web UI.
type StatusResult struct {
	// Report is the repo-relative path of the written status report.
	Report string `json:"report"`
	// NextItems counts the next-work items the agent appended to
	// TODO_LIST.md (its own self-report, not re-verified).
	NextItems int    `json:"next_items"`
	SessionID string `json:"session_id,omitempty"`
	LogPath   string `json:"log_path,omitempty"`
}

// defaultStatusTaskTimeout bounds one status run unless the payload overrides.
const defaultStatusTaskTimeout = 15 * time.Minute

// StatusExecutor runs the done-prompt agent. It reuses the agent run
// mechanics through its Agent executor (same pattern as ReviewExecutor); the
// differences are the prompt and the output contract: the final
// TQ_RESULT line must name an existing, repo-relative report file.
type StatusExecutor struct {
	// Agent supplies the run mechanics and its configuration. nil selects a
	// default AgentExecutor.
	Agent *AgentExecutor
}

func (e *StatusExecutor) base() *AgentExecutor {
	if e.Agent == nil {
		return &AgentExecutor{}
	}

	return e.Agent
}

// Execute runs the reporting agent and enforces two gates: the output must
// end with TQ_RESULT: {"report":"docs/status/...","next_items":N} naming an
// existing, repo-relative file, and the repo verify command must exit 0 (the
// reporter commits, so it can break the tree it reports on). Malformed
// output, a missing file or a failed verify is a retryable failure; payload
// misses are permanent; dirty trees are preflight requeues.
func (e *StatusExecutor) Execute(ctx context.Context, t task.Task) error {
	var payload StatusPayload

	if len(t.Payload) == 0 {
		return Permanent(errors.New("status: empty payload, want {repo, project, completed}"))
	}

	if err := json.Unmarshal(t.Payload, &payload); err != nil {
		return Permanent(fmt.Errorf("status: decode payload: %w", err))
	}

	if payload.Repo == "" || payload.Project == "" || len(payload.Completed) == 0 {
		return Permanent(errors.New("status: payload needs non-empty repo, project and completed"))
	}

	agent := e.base()

	repoDir, err := agent.repoDir(payload.Repo)
	if err != nil {
		return Permanent(err)
	}

	if requireClean(AgentPayload{RequireClean: payload.RequireClean}) {
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
			if err := assertCleanTree(ctx, repoDir); err != nil {
				return &PreflightError{Cause: err}
			}
		}
	}

	timeout := defaultStatusTaskTimeout
	if payload.TimeoutMinutes > 0 {
		timeout = time.Duration(payload.TimeoutMinutes) * time.Minute
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := agent.runAgent(runCtx, repoDir, &AgentPayload{
		Repo:   payload.Repo,
		Prompt: statusPrompt(payload),
		Model:  payload.Model,
		Yolo:   payload.Yolo,
	})
	if err != nil {
		return err
	}

	result, err := parseStatusResult(output)
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}

	if err := requireReportFile(repoDir, result.Report); err != nil {
		return fmt.Errorf("status: %w", err)
	}

	// The reporter had write access and committed — prove the tree it left
	// behind still builds/tests before the completion counts. Same gate and
	// resolution order as the agent executor (.tq-verify file, payload,
	// auto-detect; empty resolves to nothing to run).
	if _, err := runVerify(ctx, repoDir, &AgentPayload{Verify: payload.Verify}); err != nil {
		return err
	}

	result.SessionID = ExtractSessionID(output)
	result.LogPath = writeOutputSidecar(t.ID, output, "")

	detail, _ := json.Marshal(result)
	SetResultDetail(ctx, detail)

	return nil
}

// parseStatusResult enforces the mechanical contract: a TQ_RESULT JSON object
// with a non-empty report path and a next_items count.
func parseStatusResult(output string) (StatusResult, error) {
	raw, err := ResultLine(output)
	if err != nil {
		return StatusResult{}, err
	}

	var parsed struct {
		Report    string `json:"report"`
		NextItems int    `json:"next_items"`
	}

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return StatusResult{}, fmt.Errorf("report JSON does not parse: %w", err)
	}

	if strings.TrimSpace(parsed.Report) == "" {
		return StatusResult{}, errors.New(`report JSON has no report path (want {"report":"docs/status/...","next_items":N})`)
	}

	return StatusResult{Report: strings.TrimSpace(parsed.Report), NextItems: parsed.NextItems}, nil
}

// requireReportFile verifies the self-reported path is repo-relative, local
// (no absolute paths, no .. escapes), and exists as a file under repoDir.
func requireReportFile(repoDir, report string) error {
	if !filepath.IsLocal(report) {
		return fmt.Errorf("report path %q is not repo-relative (absolute paths and .. escapes are refused)", report)
	}

	info, err := os.Stat(filepath.Join(repoDir, report))
	if err != nil {
		return fmt.Errorf("reported file %s does not exist: %w", report, err)
	}

	if info.IsDir() {
		return fmt.Errorf("reported file %s is a directory", report)
	}

	return nil
}

// statusPrompt builds the done-prompt instruction: the window's completions,
// the full status report contract, the TODO_LIST.md loop-closing rules, and
// the exact output contract.
func statusPrompt(p StatusPayload) string {
	var b strings.Builder

	b.WriteString(`You are an autonomous status reporter for a shared task queue. A window of agent tasks just completed in this repository; your job is the "done prompt": reflect on what was done, write a full status report, and feed the next round of work back into the backlog.

## The completed window

`)

	for _, c := range p.Completed {
		b.WriteString("- task " + c.TaskID + ": " + firstLine(c.Item))

		if c.Commit != "" {
			b.WriteString(" (commit " + c.Commit + ")")
		}

		b.WriteString("\n")
	}

	b.WriteString(`
Inspect what these tasks actually changed (git log, git show, the repository's docs and TODO_LIST.md). Base the report on what you can verify — no invented history.

## Write a full status report

Run "date" (CLI) for the current date-time, then write a comprehensive, detailed status report to:

docs/status/<YYYY-MM-DD_HH-MM_WELL-NAMED>.md

The report MUST cover:
a) FULLY DONE — what the window's tasks completed (verified, not just claimed);
b) PARTIALLY DONE — what landed incomplete;
c) NOT STARTED — backlog items the window skipped;
d) TOTALLY FUCKED UP — regressions, broken gates, dead-lettered work, tech debt introduced;
e) WHAT WE SHOULD IMPROVE — process and code, concrete and actionable;
f) UP TO 50 NEXT THINGS — the most valuable next work items, small and concrete;
g) UP TO 3 QUESTIONS you cannot answer yourself — things only the repository owner can decide.

DO NOT RESEARCH UNRELATED STUFF. Report on this window and what you noticed in passing.

## Close the loop in TODO_LIST.md

TODO_LIST.md is machine-consumed: one checkbox item per line, "- [ ] text", never tables. Append:
- the next things from (f) as new "- [ ]" items (max ~50, each a self-contained task);
- each question from (g) as an item ending with " — BLOCKED: <the question>" (a human answers by editing the item; blocked items are never harvested until then).
Never delete or reword existing items; only append.

## Hard scope rule

Touch ONLY these files, nothing else:
- your new report under docs/status/,
- TODO_LIST.md (append-only),
- the git commit containing exactly those changes.
Do not modify code, configuration, docs, or any other tracked file. If you notice a bug or want a fix, REPORT it (as a TODO_LIST item) — do not fix it yourself.

## Finish

1. Commit your changes with a clear message (you have explicit permission to commit for this task). Never push.
2. End your final output with EXACTLY ONE line of this shape and nothing after it:

TQ_RESULT: {"report": "docs/status/<the-file-you-wrote>.md", "next_items": <number of items appended>}

The report path must be relative to the repository root; the file must exist when you finish.
`)

	return b.String()
}

// firstLine reduces a (possibly long) agent prompt to its first line, bounded
// for prompt hygiene.
func firstLine(text string) string {
	line := strings.TrimSpace(text)
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}

	const maxItemLen = 200
	if len(line) > maxItemLen {
		line = line[:maxItemLen] + "…"
	}

	return strings.TrimSpace(line)
}
