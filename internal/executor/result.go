package executor

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"os/exec"
	"regexp"
	"sync"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// sessionUsage is the crush-session usage block every paid agent turn's
// result type embeds (AgentResult, ReviewResult, StatusResult,
// PrioritizeResult): one field set, one json spelling, one derivation path.
// The budget token projection parses these keys across all four result
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
}

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
	// proof the task completed on.
	VerifyTail string `json:"verify_tail,omitempty"`
	// Commits are the commits whose `Task-Queue-ID: <id>` footer names this
	// task, DERIVED from git after the run (oldest first) — the agent never
	// reports them. FilesChanged is the union of files those commits
	// touched; CommitSHA is the newest one (the review sweeper and web UI
	// read both). A legacy `TQ_RESULT` self-report still fills them when
	// derivation finds nothing (in-flight tasks minted before derivation).
	Commits      []Commit `json:"commits,omitempty"`
	FilesChanged []string `json:"files_changed,omitempty"`
	CommitSHA    string   `json:"commit_sha,omitempty"`
	// LogPath is the sidecar file holding the FULL agent + verify output,
	// written when TQ_LOG_DIR is set on the worker/pool. Absent otherwise.
	LogPath string `json:"log_path,omitempty"`
}

// FailureEvidence is the structured forensics attached to a task.failed
// fact: WHAT failed (which stage), the process exit code, and the tail of
// the output that proves it. The 21:40 window's two retry-path failures
// left empty {} detail — the why lived only in the error text.
type FailureEvidence struct {
	Stage    string `json:"stage"`               // "agent", "verify" or "command"
	ExitCode int    `json:"exit_code,omitempty"` // process exit code (0 when the error was not an exit)
	Tail     string `json:"tail,omitempty"`      // last lines of the failing output
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
	if s, ok := ctx.Value(sinkKey{}).(*Sink); ok {
		evidence := FailureEvidence{Stage: stage, ExitCode: exitCode(err), Tail: tail}
		if raw, merr := json.Marshal(evidence); merr == nil {
			s.mu.Lock()
			s.failure = raw
			s.mu.Unlock()
		}
	}
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

// resultLineRe matches the agent's result line: a single line of JSON
// after the TQ_RESULT: marker. Everything else in the output is free-form.
var resultLineRe = regexp.MustCompile(`(?im)^\s*TQ_RESULT:\s*(\{.+\})\s*$`)

// lastResultLine scans all TQ_RESULT lines and returns the LAST match's
// JSON: last-line-wins is the documented contract (the close-out turn
// re-emits after the work turn; the verdict file is appended after both).
// FindStringSubmatch would silently read the FIRST line instead — the
// exact inversion this helper exists to prevent.
func lastResultLine(output string) []string {
	matches := resultLineRe.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}

	return matches[len(matches)-1]
}

// ResultLine extracts the raw JSON of the LAST TQ_RESULT line from agent
// output. Executors with a mechanical output contract (review, status) build
// their strict parsing on top of it; ExtractResultPayload is the lenient
// consumer for plain agent runs.
func ResultLine(output string) (jsontext.Value, error) {
	m := lastResultLine(output)
	if m == nil {
		return nil, errors.New("output has no TQ_RESULT line")
	}

	return jsontext.Value(m[1]), nil
}

// ExtractResultPayload parses the agent's structured self-report
// ({files_changed, commit_sha}) from its output — the legacy fallback for
// in-flight tasks; derivation is the primary source. Best-effort: no line,
// no problem — the fields simply stay empty in the result detail.
func ExtractResultPayload(output string) (files []string, sha string, ok bool) {
	m := lastResultLine(output)
	if m == nil {
		return nil, "", false
	}

	var rp struct {
		FilesChanged []string `json:"files_changed"`
		CommitSHA    string   `json:"commit_sha"`
	}
	if err := json.Unmarshal([]byte(m[1]), &rp); err != nil {
		return nil, "", false
	}

	return rp.FilesChanged, rp.CommitSHA, true
}
