package executor

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"regexp"
	"sync"
)

// AgentResult is the structured outcome detail of one agent run, stored
// alongside the completion so `tq show` can answer "what did the agent
// actually do" without SSH-ing into logs.
type AgentResult struct {
	// SessionID is the crush session id, best-effort extracted from the
	// agent's output (`crush run` prints it; formats vary between
	// versions — absent when nothing matches).
	SessionID string `json:"session_id,omitempty"`
	// VerifyTail is the last lines of the verify command's output: the
	// proof the task completed on.
	VerifyTail string `json:"verify_tail,omitempty"`
	// FilesChanged and CommitSHA are the agent's own structured report,
	// parsed from a final `TQ_RESULT: {"files_changed": [...], "commit_sha":
	// "..."}` output line (the agent prompt convention). Absent when the
	// agent did not emit one.
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
	Stage    string `json:"stage"`              // "agent", "verify" or "command"
	ExitCode int    `json:"exit_code,omitempty"` // process exit code (0 when the error was not an exit)
	Tail     string `json:"tail,omitempty"`      // last lines of the failing output
}

// sink carries per-task result detail from an executor run back to the
// worker. Executors share one instance, so the sink travels in the task's
// context instead of on the executor.
type sinkKey struct{}

// Sink collects structured outcome detail for ONE task execution.
type Sink struct {
	mu      sync.Mutex
	detail  json.RawMessage
	failure json.RawMessage
}

// NewSink returns a context carrying the sink and the sink itself.
func NewSink(ctx context.Context) (context.Context, *Sink) {
	s := &Sink{}

	return context.WithValue(ctx, sinkKey{}, s), s
}

// SetResultDetail attaches outcome detail to the current execution; a
// no-op when the context carries no sink (plain executors, tests).
func SetResultDetail(ctx context.Context, detail json.RawMessage) {
	if s, ok := ctx.Value(sinkKey{}).(*Sink); ok && len(detail) > 0 {
		s.mu.Lock()
		s.detail = detail
		s.mu.Unlock()
	}
}

// Detail returns the recorded outcome detail, or nil.
func (s *Sink) Detail() json.RawMessage {
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
func (s *Sink) Failure() json.RawMessage {
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

	return ""
}

// resultLineRe matches the agent's self-report line: a single line of JSON
// after the TQ_RESULT: marker. Everything else in the output is free-form.
var resultLineRe = regexp.MustCompile(`(?im)^\s*TQ_RESULT:\s*(\{.+\})\s*$`)

// ResultLine extracts the raw JSON of the LAST TQ_RESULT line from agent
// output. Executors with a mechanical output contract (review, status) build
// their strict parsing on top of it; ExtractResultPayload is the lenient
// consumer for plain agent runs.
func ResultLine(output string) (json.RawMessage, error) {
	m := resultLineRe.FindStringSubmatch(output)
	if m == nil {
		return nil, errors.New("output has no TQ_RESULT line")
	}

	return json.RawMessage(m[1]), nil
}

// ExtractResultPayload parses the agent's structured self-report
// ({files_changed, commit_sha}) from its output. Best-effort: no line, no
// problem — the fields simply stay empty in the result detail.
func ExtractResultPayload(output string) (files []string, sha string, ok bool) {
	m := resultLineRe.FindStringSubmatch(output)
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
