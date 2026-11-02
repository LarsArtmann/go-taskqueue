package executor

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// sessionUsage is the crush-session usage block every paid agent turn's
// result type embeds (AgentResult, ReviewResult, StatusResult,
// PrioritizeResult, DLQFixResult): one field set, one json spelling, one
// derivation path.
// The budget token projection parses these keys across all result
// types (drift-pinned by the budget tests marshalling the real types).
type sessionUsage struct {
	// SessionID is the crush session id, best-effort extracted from the
	// run's output (`crush run` prints it; formats vary between versions —
	// absent when nothing matches).
	SessionID string `json:"session_id,omitempty"`
	// Session usage, derived from the local crush data (go-crush-data) when
	// the run's session id was extractable. Zero on stub or non-crush runs.
	SessionCostUSD          float64 `json:"session_cost_usd,omitempty"`
	SessionPromptTokens     int64   `json:"session_prompt_tokens,omitempty"`
	SessionCompletionTokens int64   `json:"session_completion_tokens,omitempty"`
	SessionMessageCount     int     `json:"session_message_count,omitempty"`
	// LogPath is the sidecar file holding the FULL agent + verify output,
	// written when TQ_LOG_DIR is set on the worker/pool. Absent otherwise.
	// Lives here (not per result type) because every paid turn records it
	// and the embedded flattening keeps the wire shape identical.
	LogPath string `json:"log_path,omitempty"`
}

// setLogPath lets recordRunOutcome stamp the sidecar path through the
// embedding without a second pointer parameter.
func (u *sessionUsage) setLogPath(path string) { u.LogPath = path }

// deriveUsage fills the session block from a finished run: the session id
// comes from the output, the spend from deriveOutcome. Best-effort — a
// missing session id or unreadable crush data leaves the fields zero.
// Returns the full derivation so callers that also need commits/files reuse
// the single git+crush pass instead of paying for it twice.
func (u *sessionUsage) deriveUsage(ctx context.Context, repoDir, output string, id task.ID) derivedOutcome {
	u.SessionID = ExtractSessionID(output)

	derived := deriveOutcome(ctx, repoDir, u.SessionID, id)
	u.SessionCostUSD = derived.SessionCostUSD
	u.SessionPromptTokens = derived.SessionPromptTokens
	u.SessionCompletionTokens = derived.SessionCompletionTokens
	u.SessionMessageCount = derived.SessionMessageCount

	return derived
}

// AgentResult is the structured outcome detail of one agent run, stored
// alongside the completion so `tq show` can answer "what did the agent
// actually do" without SSH-ing into logs.
type AgentResult struct {
	sessionUsage

	// VerifyTail is the last lines of the verify command's output: the
	// proof the task completed on. Bounded, and blind to the two questions a
	// closeout actually asks ("which gate ran?" and "how many checks
	// passed?") — those live in Verify.
	VerifyTail string `json:"verify_tail,omitempty"`
	// Verify is the structured receipt of the gate that judged this run:
	// the executed command, its rc, the HEAD it ran at and the ok/fail
	// tallies. Absent means the receipt predates this field or nothing ran —
	// UNVERIFIED, never invalid (see AttestVerify / AttestationNoReceipt).
	Verify *VerifyReceipt `json:"verify,omitempty"`
	// Attestation is the deterministic verdict on Verify, stamped at write
	// time by the process that held the gate output. It is a REFUSAL when the
	// executed gate was not the pinned one or the gate ran at a rev the
	// completion does not claim — the rendering surfaces show the refusal
	// instead of presenting the receipt as verified (OKF §10.5: a false
	// verdict means refuse the value, not decorate it).
	Attestation *VerifyVerdict `json:"attestation,omitempty"`
	// Commits are the commits whose `Task-Queue-ID: <id>` footer names this
	// task, DERIVED from git after the run (oldest first) — the agent never
	// reports them. FilesChanged is the union of files those commits
	// touched; CommitSHA is the newest one (the review sweeper and web UI
	// read both). Empty when derivation finds nothing — there is no agent
	// self-report: commits come only from the footer derivation.
	Commits      []Commit `json:"commits,omitempty"`
	FilesChanged []string `json:"files_changed,omitempty"`
	CommitSHA    string   `json:"commit_sha,omitempty"`
	// PreflightDone carries the done-preflight reason when the worker
	// completed the task WITHOUT an agent run because the work was provably
	// already done (footer commits exist, item ticked, fix cured…). A
	// non-empty value means no session ran and no review may be minted for
	// this completion (review.enqueueReview skips it).
	PreflightDone string `json:"preflight_done,omitempty"`
	// WorktreeDir is the linked checkout the run happened in (worktree
	// mode only; empty = in-place run). Branch is the tq/<task-id>
	// delivery branch forked from HEAD — the commits live there until the
	// merge machinery lands (worktree-per-agent design,
	// docs/planning/archived/2026-09-12_worktree-per-agent-design.md).
	WorktreeDir string `json:"worktree_dir,omitempty"`
	Branch      string `json:"branch,omitempty"`
	// BaseBranch is the branch the delivery branch was forked from (the
	// repo's HEAD branch at worktree-create time). The merge machinery
	// merges Branch into BaseBranch on review approve.
	BaseBranch string `json:"base_branch,omitempty"`
	// ToolCalls is the structured per-tool-call inventory of the session,
	// fetched from the crush server's messages endpoint when the run was
	// routed (row 1446): the closeout-report raw material without stdout
	// parsing. Empty on in-process runs — the stdout derivation stays the
	// fallback channel.
	ToolCalls []ToolCallFact `json:"tool_calls,omitempty"`
}

// FailureEvidence is the structured forensics attached to a task.failed
// fact: WHAT failed (which stage), the process exit code, and the tail of
// the output that proves it. The 21:40 window's two retry-path failures
// left empty {} detail — the why lived only in the error text.
type FailureEvidence struct {
	Stage    string `json:"stage"`               // "agent", "verify" or "command"
	ExitCode int    `json:"exit_code,omitempty"` // process exit code (0 when the error was not an exit)
	Tail     string `json:"tail,omitempty"`      // last lines of the failing output
	// VerifyStage names the failing stage OF THE VERIFY GATE on Stage
	// "verify" deaths: a VerifyStageGofmt / VerifyStageTest /
	// VerifyStageE2ETimeout / VerifyStageRun class. Stamped from the FULL
	// run output at the tail cut (verifyDeathStage), so the class stays
	// queryable after the evidence tail truncates and the sidecar logs
	// rotate. Empty for non-verify failures.
	VerifyStage string `json:"verify_stage,omitempty"`
	// Class is the retry taxonomy (FailureClass) stamped by the WORKER
	// after the run: only the worker sees the final error wrapper chain
	// (a verify gate wraps a plain failure and changes its class). Empty
	// on legacy facts; readers treat empty as "unclassified".
	Class string `json:"class,omitempty"`
	// FilesChanged is the union of files the run's footer-attributed
	// commits touched, derived AFTER a failing agent/verify run so a DLQ
	// autopsy sees what the run had already shipped when it died (work
	// left uncommitted carries no footer and stays invisible — that is
	// the derivation contract). Empty when the run died before any
	// footer commit landed.
	FilesChanged []string `json:"files_changed,omitempty"`
	// Verify is the receipt of the gate that DIED: the same four
	// observations the success path records (command, rc, HEAD, counts).
	// One shape across both paths — a failure receipt and a success receipt
	// are the same type, so no reader needs two decoders.
	Verify *VerifyReceipt `json:"verify,omitempty"`
	// Attestation is the write-time verdict on Verify, same contract as the
	// success path (a refused receipt is evidence of a broken gate contract,
	// which is exactly what a DLQ autopsy needs).
	Attestation *VerifyVerdict `json:"attestation,omitempty"`
}

// IsZero reports whether no evidence was recorded — the field-wise zero
// check FailureEvidence consumers use now that FilesChanged (a slice)
// took struct comparability away.
func (e FailureEvidence) IsZero() bool {
	return e.Stage == "" && e.VerifyStage == "" && e.Class == "" &&
		e.ExitCode == 0 && e.Tail == "" && len(e.FilesChanged) == 0
}

// EvidenceTailBytes is the ONE output-tail size every executor pins into
// FailureEvidence.Tail — agent, verify and command tails stay the same
// length so facts are comparable across executors (01:48 report f23: three
// independent 4096s, one drift waiting to happen).
const EvidenceTailBytes = 4096

// sink carries per-task result detail from an executor run back to the
// worker. Executors share one instance, so the sink travels in the task's
// context instead of on the executor.
type sinkKey struct{}

// Sink collects structured outcome detail for ONE task execution.
type Sink struct {
	mu      sync.Mutex
	detail  jsontext.Value
	failure jsontext.Value
}

// NewSink returns a context carrying the sink and the sink itself.
func NewSink(ctx context.Context) (context.Context, *Sink) {
	s := &Sink{}

	return context.WithValue(ctx, sinkKey{}, s), s
}

// SetResultDetail attaches outcome detail to the current execution; a
// no-op when the context carries no sink (plain executors, tests).
func SetResultDetail(ctx context.Context, detail jsontext.Value) {
	if s, ok := ctx.Value(sinkKey{}).(*Sink); ok && len(detail) > 0 {
		s.mu.Lock()
		s.detail = detail
		s.mu.Unlock()
	}
}

// recordRunOutcome finishes one paid run's bookkeeping: persist the FULL
// run output sidecar, then record the result — with its log_path — as the
// execution's outcome detail for `tq show`, and append the calibration
// row (raw verdict-channel body joined against the derived outcome) to
// the dataset. The sidecar path is stamped
// through the result's embedded sessionUsage so it lands inside the
// recorded detail; callers that derive session usage do so BEFORE calling
// (deriveUsage returns its derivation for executors that also need
// commits/files).
func recordRunOutcome[R any, T interface {
	*R
	setLogPath(path string)
}](ctx context.Context, result T, kind string, verdict jsontext.Value, output, tail string, id task.ID) {
	result.setLogPath(writeOutputSidecar(id, output, tail))

	detail, _ := json.Marshal(result)
	SetResultDetail(ctx, detail)
	recordCalibration(kind, id, verdict, detail)
}

// finishParsedRun is the shared tail of every closeout-free agent turn:
// a parse miss wraps as the executor-named error (retryable — the model
// may comply on a retry), a parsed result derives its session usage from
// the finished output and records the run outcome for `tq show` plus the
// calibration row (the raw verdict body rides in).
func finishParsedRun[R any, T interface {
	*R
	setLogPath(path string)
	deriveUsage(ctx context.Context, repoDir, output string, id task.ID) derivedOutcome
}](ctx context.Context, result T, parseErr error, kind string, verdict jsontext.Value, output, repoDir string, id task.ID) error {
	if parseErr != nil {
		return fmt.Errorf("%s: %w", kind, parseErr)
	}

	result.deriveUsage(ctx, repoDir, output, id)
	recordRunOutcome(ctx, result, kind, verdict, output, "", id)

	return nil
}

// Detail returns the recorded outcome detail, or nil.
func (s *Sink) Detail() jsontext.Value {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.detail
}

// SetFailureEvidence attaches forensics for a FAILING execution; a no-op
// when the context carries no sink. Mirrors SetResultDetail: executors call
// it on their failure paths, the worker hands it to Store.Fail so the
// task.failed fact carries the evidence.
func SetFailureEvidence(ctx context.Context, stage string, err error, tail string) {
	setFailureEvidence(ctx, FailureEvidence{Stage: stage, ExitCode: exitCode(err), Tail: tail})
}

// SetVerifyFailureEvidence attaches verify-gate forensics with the death
// class stamped: Stage "verify" plus VerifyStage (verifyDeathStage's gofmt
// / test / e2e-timeout / run), so DLQ triage can query the failing stage
// from the fact instead of re-deriving it from tails and rotated logs. The
// outcome's receipt rides along, so a FAILING gate answers the same two
// questions a passing one does (which gate, how many checks).
func SetVerifyFailureEvidence(ctx context.Context, out VerifyOutcome, err error) {
	setFailureEvidence(ctx, FailureEvidence{
		Stage:       "verify",
		VerifyStage: out.Stage,
		ExitCode:    exitCode(err),
		Tail:        out.Tail,
		Verify:      out.Receipt,
		// No claimed rev yet on this path (the failing run's own commits are
		// derived afterwards), so the HEAD comparison is skipped rather than
		// guessed — a skipped comparison is reported in the verdict, never
		// silently counted as a pass.
		Attestation: out.Attest(""),
	})
}

func setFailureEvidence(ctx context.Context, evidence FailureEvidence) {
	if s, ok := ctx.Value(sinkKey{}).(*Sink); ok {
		if raw, merr := json.Marshal(evidence); merr == nil {
			s.mu.Lock()
			s.failure = raw
			s.mu.Unlock()
		}
	}
}

// EnrichFailureEvidenceFiles merges the files the failing run's
// footer-attributed commits touched into the sink's already-recorded
// failure evidence, so a DLQ autopsy sees what the run had changed when
// it died without re-deriving from rotated sidecar logs. Best-effort: no
// sink, no recorded evidence or an empty derivation leaves the evidence
// untouched; a recorded files list is unioned, never replaced.
func EnrichFailureEvidenceFiles(ctx context.Context, runDir string, id task.ID) {
	s, ok := ctx.Value(sinkKey{}).(*Sink)
	if !ok {
		return
	}

	s.mu.Lock()
	raw := s.failure
	s.mu.Unlock()

	if raw == nil {
		return
	}

	var evidence FailureEvidence
	if json.Unmarshal(raw, &evidence) != nil {
		return
	}

	derived := deriveOutcome(ctx, runDir, "", id)
	if len(derived.Files) == 0 {
		return
	}

	merged := slices.Clone(evidence.FilesChanged)
	for _, file := range derived.Files {
		if !slices.Contains(merged, file) {
			merged = append(merged, file)
		}
	}
	slices.Sort(merged)
	evidence.FilesChanged = merged

	setFailureEvidence(ctx, evidence)
}

// Failure returns the recorded failure evidence, or nil.
func (s *Sink) Failure() jsontext.Value {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.failure
}

// exitCode extracts a process exit code from a wrapped exec error; 0 when
// the error never was an exit (spawn failure, timeout, cancellation).
func exitCode(err error) int {
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		return ee.ExitCode()
	}

	return 0
}

var sessionRe = regexp.MustCompile(`(?im)^\s*session(?:[ _-]?id)?\s*[:=]\s*([A-Za-z0-9][A-Za-z0-9_-]*)`)

// ExtractSessionID pulls a crush session id out of agent output.
// Best-effort by design: crush's output format is not a stable contract.
func ExtractSessionID(output string) string {
	if m := sessionRe.FindStringSubmatch(output); m != nil {
		return m[1]
	}

	// crush run --verbose logs "INFO Created session for non-interactive
	// run session_id=<id>" — the line-anchored sessionRe cannot see it.
	if m := verboseSessionRe.FindStringSubmatch(output); m != nil {
		return m[1]
	}

	return ""
}

// verboseSessionRe matches the crush verbose session marker (see
// ExtractSessionID); not line-anchored because the marker rides an INFO
// log line.
var verboseSessionRe = regexp.MustCompile(
	`(?i)Created session for non-interactive run session_id=([A-Za-z0-9][A-Za-z0-9_-]+)`,
)

// ReadVerdict returns the structured verdict the agent recorded in the
// per-run file channel (TQ_RESULT_FILE): the ONE verdict channel since the
// stdout TQ_RESULT line was retired — every live task mints on
// file-channel prompts, and the file's last write wins across work +
// close-out turns. A nil value with a nil error means the channel never
// materialized (no temp file, unreadable, or empty — nothing the agent
// wrote); a non-nil error means the agent wrote something that is not
// valid JSON, which fails the attempt (retryable, same class the missing
// TQ_RESULT line used to be).
func ReadVerdict(path string) (jsontext.Value, error) {
	if path == "" {
		return nil, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}

	body := strings.TrimSpace(string(raw))
	if body == "" {
		return nil, nil
	}

	v := jsontext.Value(body)
	if !v.IsValid() {
		return nil, errors.New("verdict file does not hold valid JSON")
	}

	return v, nil
}

// RequireVerdict turns a ReadVerdict outcome into the strict-parser
// contract: executors with a mechanical output contract (review, status,
// dlqfix, prioritize) fail the attempt when the agent recorded no verdict.
func RequireVerdict(v jsontext.Value) (jsontext.Value, error) {
	if v == nil {
		return nil, errors.New("agent recorded no verdict (verdict file missing or empty)")
	}

	return v, nil
}
