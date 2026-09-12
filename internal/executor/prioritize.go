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

// TaskTypePrioritize runs a batch scorer agent over one repo's WORKING
// SET: a headless agent reads the open TODO_LIST items and returns a
// structured score/effort/confidence verdict per item. The sweeper
// (internal/prioritize) caches verdicts in the score cache and applies
// them as reprioritization facts. Register it under this type to make a
// pool prioritize-capable.
const TaskTypePrioritize = "prioritize"

// PrioritizeItem is one item the scorer must verdict, with the dedup key
// the verdict will be cached under (the sweeper derives it with the same
// harvest.ItemKey derivation the queue dedup uses).
type PrioritizeItem struct {
	Key     string `json:"key"`
	Text    string `json:"text"`
	Heading string `json:"heading,omitempty"`
}

// PrioritizePayload is the payload contract for "prioritize" tasks:
// self-contained like ReviewPayload — the batch, the repo for context,
// and the autonomy/timeout knobs.
type PrioritizePayload struct {
	// Repo is the repository whose TODO_LIST the items come from (same
	// resolution rules as AgentPayload.Repo). The scorer reads it for
	// context (AGENTS.md, code layout) but changes nothing. Required.
	Repo string `json:"repo"`
	// RepoName is the display name of the repo.
	RepoName string `json:"repo_name,omitempty"`
	// Items is the batch to score. Required, non-empty.
	Items []PrioritizeItem `json:"items"`
	// Model optionally overrides the crush model; empty reuses the repo's
	// .crushrc (the default and recommended setting — model+effort live
	// ONLY there, ADR/pool contract).
	Model string `json:"model,omitempty"`
	// Yolo marks the scorer as autonomous (same contract as
	// AgentPayload.Yolo).
	Yolo bool `json:"yolo,omitempty"`
	// RequireClean refuses to start on a dirty tree. Default true — the
	// scorer reads the repo for context and should see committed state.
	RequireClean *bool `json:"require_clean,omitempty"`
	// TimeoutMinutes caps the run. Default 10 (scoring reads; it does not
	// build).
	TimeoutMinutes int `json:"timeout_minutes,omitempty"`
}

// PrioritizeVerdict is one item's score.
type PrioritizeVerdict struct {
	ItemKey       string `json:"item_key"`
	Score         int    `json:"score"`
	EffortMinutes int    `json:"effort_minutes,omitempty"`
	Confidence    int    `json:"confidence,omitempty"`
	Reasoning     string `json:"reasoning,omitempty"`
}

// PrioritizeResult is the structured outcome stored on the completion
// fact detail, same sink convention as ReviewResult.
type PrioritizeResult struct {
	Verdicts  []PrioritizeVerdict `json:"verdicts"`
	SessionID string              `json:"session_id,omitempty"`
	LogPath   string              `json:"log_path,omitempty"`
}

// defaultPrioritizeTaskTimeout bounds one batch unless overridden.
const defaultPrioritizeTaskTimeout = 10 * time.Minute

// PrioritizeExecutor runs batch scorer agents. Like ReviewExecutor it
// reuses the agent run mechanics and is closeout-free (the scorer IS a
// short-lived utility turn, not paid work needing a self-review).
type PrioritizeExecutor struct {
	// Agent supplies the run mechanics and its configuration. nil selects
	// a default AgentExecutor.
	Agent *AgentExecutor
}

func (e *PrioritizeExecutor) base() *AgentExecutor {
	if e.Agent == nil {
		return &AgentExecutor{}
	}

	return e.Agent
}

// Execute runs the scorer and enforces the verdict contract: every input
// item must come back exactly once with a 0-100 score; anything else is a
// failed attempt (retryable — the model may comply on a retry), input
// misses are permanent, dirty trees are preflight requeues.
func (e *PrioritizeExecutor) Execute(ctx context.Context, t task.Task) error {
	var p PrioritizePayload

	if len(t.Payload) == 0 {
		return Permanent(errors.New("prioritize: empty payload, want {repo, items}"))
	}

	if err := json.Unmarshal(t.Payload, &p); err != nil {
		return Permanent(fmt.Errorf("prioritize: decode payload: %w", err))
	}

	if p.Repo == "" || len(p.Items) == 0 {
		return Permanent(errors.New("prioritize: payload needs non-empty repo and items"))
	}

	agent := e.base()

	repoDir, err := agent.repoDir(p.Repo)
	if err != nil {
		return Permanent(err)
	}

	if requireClean(AgentPayload{RequireClean: p.RequireClean}) {
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
			if err := assertCleanTree(ctx, repoDir); err != nil {
				return &PreflightError{Cause: err}
			}
		}
	}

	timeout := defaultPrioritizeTaskTimeout
	if p.TimeoutMinutes > 0 {
		timeout = time.Duration(p.TimeoutMinutes) * time.Minute
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := agent.runAgent(runCtx, repoDir, &AgentPayload{
		Repo:   p.Repo,
		Prompt: prioritizePrompt(p),
		Model:  p.Model,
		Yolo:   p.Yolo,
	}, t.ID)
	if err != nil {
		return err
	}

	result, err := ParsePrioritizeResult(output, p.Items)
	if err != nil {
		return fmt.Errorf("prioritize: %w", err)
	}

	result.SessionID = ExtractSessionID(output)
	result.LogPath = writeOutputSidecar(t.ID, output, "")

	detail, _ := json.Marshal(result)
	SetResultDetail(ctx, detail)

	return nil
}

// prioritizePrompt builds the scorer instruction: the batch, the scoring
// rubric, the read-only rule, and the exact output contract.
func prioritizePrompt(p PrioritizePayload) string {
	var b strings.Builder

	b.WriteString(
		"You are a task prioritizer for an autonomous work queue. Score backlog items by how much value working them NEXT delivers. READ-ONLY: do not create, modify, or delete any file; run only read-only commands. You may read this repository (AGENTS.md, README, code) for context.\n\n",
	)

	b.WriteString("## Items to score (key | section | text)\n\n")

	for _, item := range p.Items {
		heading := item.Heading
		if heading == "" {
			heading = "-"
		}

		b.WriteString(fmt.Sprintf("- `%s` | %s | %s\n", item.Key, heading, strings.ReplaceAll(item.Text, "\n", " ")))
	}

	b.WriteString(`
## Rubric (score 0-100 per item)

- 90-100: unblocks other work, fixes an active incident or a security issue, or ships visible user value now.
- 70-89: clear value, well-understood, moderate effort.
- 40-69: useful but neither urgent nor particularly leveraged.
- 10-39: cleanup, nice-to-have, speculative.
- 0-9: redundant, obsolete, or contradicts the repo's own conventions.

Also estimate effort_minutes (focused agent minutes: 15/30/60/120 are
fine anchors) and confidence 0-100 (how sure you are of the score).

## Rules

- Score every listed item EXACTLY once; echo its key verbatim.
- Judge by THIS repository's reality (check code when unsure), not generic priors.
- Do not invent items. Do not skip items.

End your reply with EXACTLY ONE line of this shape and nothing after it:

TQ_RESULT: {"verdicts":[{"item_key":"...","score":80,"effort_minutes":30,"confidence":70,"reasoning":"one line"}]}

(one array entry per input item, in any order)
`)

	return b.String()
}

// ParsePrioritizeResult extracts and validates the verdict batch: the
// TQ_RESULT line must parse, cover every input item exactly once, and
// carry in-range scores. Lenient on optional fields.
func ParsePrioritizeResult(output string, items []PrioritizeItem) (PrioritizeResult, error) {
	raw, err := ResultLine(output)
	if err != nil {
		return PrioritizeResult{}, err
	}

	var parsed struct {
		Verdicts []PrioritizeVerdict `json:"verdicts"`
	}

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return PrioritizeResult{}, fmt.Errorf("verdict JSON: %w", err)
	}

	want := make(map[string]bool, len(items))

	for _, item := range items {
		want[item.Key] = true
	}

	got := make(map[string]bool, len(parsed.Verdicts))

	for _, verdict := range parsed.Verdicts {
		if !want[verdict.ItemKey] {
			return PrioritizeResult{}, fmt.Errorf("verdict for unknown item %q", verdict.ItemKey)
		}

		if got[verdict.ItemKey] {
			return PrioritizeResult{}, fmt.Errorf("duplicate verdict for item %q", verdict.ItemKey)
		}

		if verdict.Score < 0 || verdict.Score > 100 {
			return PrioritizeResult{}, fmt.Errorf("item %q score %d out of range 0-100", verdict.ItemKey, verdict.Score)
		}

		got[verdict.ItemKey] = true
	}

	for key := range want {
		if !got[key] {
			return PrioritizeResult{}, fmt.Errorf("no verdict for item %q", key)
		}
	}

	return PrioritizeResult{Verdicts: parsed.Verdicts}, nil
}
