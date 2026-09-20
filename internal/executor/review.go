package executor

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TaskTypeReview runs a reviewer agent over a COMPLETED agent task: a
// second headless agent inspects the change (item, commit, files) and
// returns a structured verdict. Register it under this type to make a pool
// review-capable.
//
// Both verdicts complete the task — a review that requested changes did its
// job; the findings (in the completion fact detail) drive any follow-up
// work, not the exit code. Only the mechanical contract is enforced:
// output that is not a valid verdict JSON is a failed attempt, exactly like
// a failed verify.
const TaskTypeReview = "review"

// ReviewPayload is the payload contract for "review" tasks. It is
// self-contained: everything the reviewer needs to judge the change travels
// in the payload, so the executor stays a pure process runner.
type ReviewPayload struct {
	// Repo is the repository the change landed in (same resolution rules as
	// AgentPayload.Repo). Required.
	Repo string `json:"repo"`
	// ReviewedTask is the queue task ID under review — lineage for `tq show`
	// and the sweeper's loop guard. Required.
	ReviewedTask string `json:"reviewed_task"`
	// Item is the original instruction the reviewed agent was given: the bar
	// the change is judged against. Required.
	Item string `json:"item"`
	// CommitSHA is the commit the reviewed agent reported (from its
	// TQ_RESULT self-report). Empty when the agent landed no commit; the
	// reviewer then inspects the repo's latest state instead.
	CommitSHA string `json:"commit_sha,omitempty"`
	// FilesChanged lists the files the reviewed agent reported touching.
	FilesChanged []string `json:"files_changed,omitempty"`
	// Extra carries operator focus instructions appended to the prompt.
	Extra string `json:"extra,omitempty"`
	// Model optionally overrides the crush model for the reviewer.
	Model string `json:"model,omitempty"`
	// Yolo marks the reviewer as autonomous (same contract as
	// AgentPayload.Yolo: needs a repo-local crush config granting
	// permissions). Pools typically mirror the reviewed task's autonomy.
	Yolo bool `json:"yolo,omitempty"`
	// RequireClean refuses to start unless the repo tree is clean — a
	// reviewer must judge the committed state, not dirty work-in-progress.
	// Default true. Same escape hatch as AgentPayload.
	RequireClean *bool `json:"require_clean,omitempty"`
	// TimeoutMinutes caps the review run. Default 15 (reviews read; they do
	// not rebuild the world).
	TimeoutMinutes int `json:"timeout_minutes,omitempty"`
}

// ReviewVerdict is the reviewer's decision. Only these two values are
// accepted; anything else is invalid output and a failed attempt.
type ReviewVerdict string

const (
	// VerdictApprove means the change is correct and complete as it stands.
	VerdictApprove ReviewVerdict = "approve"
	// VerdictRequestChanges means the reviewer found concrete, actionable
	// problems; each finding is a candidate fix task.
	VerdictRequestChanges ReviewVerdict = "request_changes"
)

// ReviewFinding is one concrete problem the reviewer wants addressed.
type ReviewFinding struct {
	// Title is the one-line summary; it keys the dedup of auto-minted fix
	// tasks, so keep findings stable in wording across re-reviews.
	Title string `json:"title"`
	// Severity is a display hint normalized to low/medium/high at parse
	// time (unknown or empty ratings degrade to medium — severity never
	// gates anything mechanically, so leniency here is safe).
	Severity string `json:"severity,omitempty"`
	// Detail is the free-form explanation and suggested direction.
	Detail string `json:"detail,omitempty"`
	// CommitSHA is the commit the finding is anchored to (what the reviewer
	// inspected). Empty in model output is backfilled from the review
	// payload's CommitSHA at parse time — every request_changes finding
	// ends up anchored to a commit even when the model omits it.
	CommitSHA string `json:"commit_sha,omitempty"`
	// Anchor is the short VERBATIM text the finding is attached to (a line
	// or hunk of the cited file AS IT READS at the anchored commit). A bare
	// line-range citation ("lines 12-18", "x.go:3") is rejected at parse
	// time: line numbers go stale on the first rebase, and a fix agent that
	// must re-derive the location is re-doing the review (the 2026-09-10
	// 05-58 stale-anchor incident cost a rework day).
	Anchor string `json:"anchor,omitempty"`
}

// ReviewResult is the structured outcome of one review run, stored in the
// completion fact detail (the sink convention) for the sweeper and `tq show`.
type ReviewResult struct {
	Verdict   ReviewVerdict   `json:"verdict"`
	Summary   string          `json:"summary,omitempty"`
	Findings  []ReviewFinding `json:"findings,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	// Session usage, derived from the local crush data (go-crush-data) when
	// the run's session id was extractable — reviews are paid turns too,
	// so their spend feeds the budget token projection. Same json keys and
	// sink convention as AgentResult and PrioritizeResult.
	SessionCostUSD          float64 `json:"session_cost_usd,omitempty"`
	SessionPromptTokens     int64   `json:"session_prompt_tokens,omitempty"`
	SessionCompletionTokens int64   `json:"session_completion_tokens,omitempty"`
	SessionMessageCount     int     `json:"session_message_count,omitempty"`
	LogPath                 string  `json:"log_path,omitempty"`
}

// defaultReviewTaskTimeout bounds one review unless the payload overrides.
const defaultReviewTaskTimeout = 15 * time.Minute

// ReviewExecutor runs reviewer agents. It reuses the agent run mechanics
// (repo resolution, autonomy preflight, clean-tree guard, process-group
// handling) through its Agent executor; the only differences are the
// prompt and the strict output contract.
type ReviewExecutor struct {
	// Agent supplies the run mechanics and its configuration (Bin,
	// ProjectsDir, Yolo). nil selects a default AgentExecutor.
	Agent *AgentExecutor
}

// base returns the underlying agent executor, lazily defaulting.
func (e *ReviewExecutor) base() *AgentExecutor {
	if e.Agent == nil {
		return &AgentExecutor{}
	}

	return e.Agent
}

// Execute runs the reviewer and enforces the verdict contract. Approve and
// request_changes both succeed; malformed output fails the attempt
// (retryable — the model may comply on a retry), input-contract misses are
// permanent, dirty trees are preflight requeues.
func (e *ReviewExecutor) Execute(ctx context.Context, t task.Task) error {
	var p ReviewPayload

	if len(t.Payload) == 0 {
		return Permanent(errors.New("review: empty payload, want {repo, reviewed_task, item}"))
	}

	if err := json.Unmarshal(t.Payload, &p); err != nil {
		return Permanent(fmt.Errorf("review: decode payload: %w", err))
	}

	if p.Repo == "" || p.ReviewedTask == "" || p.Item == "" {
		return Permanent(errors.New("review: payload needs non-empty repo, reviewed_task and item"))
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

	timeout := defaultReviewTaskTimeout
	if p.TimeoutMinutes > 0 {
		timeout = time.Duration(p.TimeoutMinutes) * time.Minute
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := agent.runAgent(runCtx, repoDir, &AgentPayload{
		Repo:   p.Repo,
		Prompt: reviewPrompt(p),
		Model:  p.Model,
		Yolo:   p.Yolo,
	}, t.ID)
	if err != nil {
		return err
	}

	result, err := ParseResult(output)
	if err != nil {
		return fmt.Errorf("review: %w", err)
	}

	// Backfill the anchor commit: a finding that omits commit_sha is
	// anchored to the change under review.
	if p.CommitSHA != "" {
		for i := range result.Findings {
			if result.Findings[i].CommitSHA == "" {
				result.Findings[i].CommitSHA = p.CommitSHA
			}
		}
	}

	result.SessionID = ExtractSessionID(output)

	// Session usage derivation (best-effort, same as agent and prioritize
	// runs): the reviewer's token/cost spend feeds the budget token
	// projection. A missing session id or unreadable crush data leaves the
	// fields zero.
	derived := deriveOutcome(ctx, repoDir, result.SessionID, t.ID)
	result.SessionCostUSD = derived.SessionCostUSD
	result.SessionPromptTokens = derived.SessionPromptTokens
	result.SessionCompletionTokens = derived.SessionCompletionTokens
	result.SessionMessageCount = derived.SessionMessageCount

	result.LogPath = writeOutputSidecar(t.ID, output, "")

	detail, _ := json.Marshal(result)
	SetResultDetail(ctx, detail)

	return nil
}

// reviewPrompt builds the reviewer instruction: context, judging criteria,
// the read-only rule, and the exact output contract.
//
// The quoted task is the REVIEWED run's contract, so its {{TASK_ID}}
// placeholder resolves to the reviewed task's id here — exactly what the
// work agent saw at run time. runAgent's substitution resolves leftovers
// against THIS task's id; leaving the placeholder in place would rewrite
// the quoted contract to demand the review's own id in commits, and a
// diligent reviewer then flags the work run's correct footer as foreign
// (the 2026-09-12 Hermes review: a sound fix got request_changes over
// exactly this).
func reviewPrompt(p ReviewPayload) string {
	var b strings.Builder

	item := strings.ReplaceAll(strings.TrimSpace(p.Item), "{{TASK_ID}}", p.ReviewedTask)

	b.WriteString(
		"You are a strict senior code reviewer. Review a change another agent made in this repository. READ-ONLY: do not create, modify, or delete any file; run only read-only commands.\n\n",
	)
	b.WriteString("## The task the agent was given\n\n" + item + "\n\n")

	if p.CommitSHA != "" {
		b.WriteString(
			"## The change\n\nThe agent reported commit " + p.CommitSHA + ". Inspect it with `git show " + p.CommitSHA + "` (plus surrounding context as needed).\n\n",
		)
	} else {
		b.WriteString(
			"## The change\n\nThe agent reported no commit; inspect the repository's current state and recent history.\n\n",
		)
	}

	if len(p.FilesChanged) > 0 {
		b.WriteString("Files the agent reported changing: " + strings.Join(p.FilesChanged, ", ") + "\n\n")
	}

	b.WriteString(`## Judge the change against

1. Correctness: does it actually fulfill the task, with real error handling and no broken edge cases?
2. Repository contracts: AGENTS.md / CLAUDE.md / docs conventions, existing patterns, and invariants.
3. Tests: behavior covered where it matters; the repo's own test suite would pass.
4. Scope: no unrelated changes, no drive-by rewrites, no dead code left behind.
5. Honesty: claims in the change match what the code does.
`)

	// Only footer-bearing contracts need the rule; for anything else it
	// would be noise (and pre-resolved or hand-written items still get it).
	if strings.Contains(item, "Task-Queue-ID") {
		b.WriteString(
			"6. Queue cross-reference: the quoted task's `Task-Queue-ID` footer contract refers to the REVIEWED task's id, " +
				p.ReviewedTask + ". A commit ending with `Task-Queue-ID: " + p.ReviewedTask +
				"` satisfies it; that is the correct state, never a finding. A different id in that footer is a finding.\n\n",
		)
	}

	if extra := strings.TrimSpace(p.Extra); extra != "" {
		b.WriteString("## Additional focus for this review\n\n" + extra + "\n\n")
	}

	b.WriteString(`## Verdict rules

- "approve": the change is correct and complete as it stands. Trivial nits do NOT block approval.
- "request_changes": you found at least one concrete, actionable problem. Every finding must be specific enough that a fix agent can act on it without re-doing the review (name the file, the behavior, and the expected direction).
- Every finding must carry "anchor": a short VERBATIM quote of the text the finding is attached to, exactly as the file reads at the commit you inspected. Never cite bare positions ("lines 12-18", "x.go:3") without the quoted text — line numbers go stale.
- Optionally carry "commit_sha": the commit the finding is anchored to (defaults to the change under review).
- Never request changes without findings; never report findings under approve.

Record your verdict by running EXACTLY ONE of:

tq verdict '{"verdict":"approve","summary":"...","findings":[]}'

tq verdict '{"verdict":"request_changes","summary":"...","findings":[{"title":"...","severity":"low|medium|high","detail":"...","anchor":"<verbatim quoted text>","commit_sha":"<sha>"}]}'

(tq validates the JSON and writes $TQ_RESULT_FILE — the queue reads that file after you exit; if tq is not on PATH, write the same one-line JSON to $TQ_RESULT_FILE yourself)

`)

	return b.String()
}

// ParseResult extracts and validates the verdict JSON from reviewer output.
// Strict where it matters, lenient where nothing depends on it: the verdict
// must be exactly approve or request_changes (case-insensitive) and the
// JSON must parse; findings with empty titles are dropped and severity
// ratings are normalized to low/medium/high.
func ParseResult(output string) (ReviewResult, error) {
	raw, err := ResultLine(output)
	if err != nil {
		return ReviewResult{}, err
	}

	var parsed struct {
		Verdict  string          `json:"verdict"`
		Summary  string          `json:"summary"`
		Findings []ReviewFinding `json:"findings"`
	}

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ReviewResult{}, fmt.Errorf("verdict JSON does not parse: %w", err)
	}

	switch v := ReviewVerdict(strings.ToLower(strings.TrimSpace(parsed.Verdict))); v {
	case VerdictApprove, VerdictRequestChanges:
		result := ReviewResult{Verdict: v, Summary: parsed.Summary}

		for _, f := range parsed.Findings {
			if strings.TrimSpace(f.Title) == "" {
				continue
			}

			f.Severity = normalizeSeverity(f.Severity)
			f.Anchor = strings.TrimSpace(f.Anchor)

			if v == VerdictRequestChanges {
				if err := validateAnchor(f.Anchor); err != nil {
					return ReviewResult{}, fmt.Errorf("finding %q: %w", strings.TrimSpace(f.Title), err)
				}
			}

			result.Findings = append(result.Findings, f)
		}

		if v == VerdictRequestChanges && len(result.Findings) == 0 {
			return ReviewResult{}, errors.New(
				"verdict request_changes without findings (verdict rules require actionable findings)",
			)
		}

		return result, nil
	case "":
		return ReviewResult{}, errors.New("verdict JSON has no verdict field")
	default:
		return ReviewResult{}, fmt.Errorf("unknown verdict %q (want approve or request_changes)", parsed.Verdict)
	}
}

// validateAnchor is the queue-side re-anchoring pre-flight: it runs at
// verdict-parse time, so a finding that cites only a position (which cannot
// be re-anchored to content after a rebase) fails the attempt while the
// reviewer can still re-file it, instead of surfacing later as an unactionable
// fix task.
func validateAnchor(anchor string) error {
	if anchor == "" {
		return errors.New("finding has no anchor: request_changes findings must quote the verbatim text they attach to")
	}

	if anchorLooksPositional(anchor) {
		return fmt.Errorf(
			"finding anchor %q is a bare position: quote the verbatim text the finding attaches to, not line numbers",
			anchor,
		)
	}

	return nil
}

// anchorLooksPositional reports whether the anchor is only a line, line
// range, or file:line citation ("lines 12-18", "12-18", "x.go:34", "L40") —
// precisely the anchors that go stale on a rebase.
func anchorLooksPositional(a string) bool {
	trimmed := strings.TrimSpace(a)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"line ", "lines ", "line:", "lines:"} {
		if strings.HasPrefix(lower, prefix) &&
			!strings.ContainsAny(strings.TrimSpace(lower[len(prefix):]), "abcdefghijklmnopqrstuvwxyz") {
			return true
		}
	}

	if fileLinePattern.MatchString(trimmed) {
		return true
	}

	body := strings.TrimPrefix(strings.TrimPrefix(lower, "l"), ":")
	if body == "" {
		return false
	}

	for _, part := range strings.FieldsFunc(body, func(r rune) bool {
		return r == '-' || r == '–' || r == ':' || r == '.' || r == ',' || r == ' ' || r == '\t'
	}) {
		if part == "" {
			continue
		}

		for _, r := range part {
			if r < '0' || r > '9' {
				return false // a letter or symbol survives: content, not a position
			}
		}
	}

	return true
}

// fileLinePattern matches bare path:line (and path:line-range) citations.
var fileLinePattern = regexp.MustCompile(`^[^\s:]+:\d+(?:[-–]\d+)?$`)

// normalizeSeverity maps arbitrary model output onto the three display
// ratings; unknown or empty degrades to medium.
func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low", "minor":
		return "low"
	case "high", "critical", "major":
		return "high"
	default:
		return "medium"
	}
}
