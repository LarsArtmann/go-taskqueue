package executor

import (
	"context"
	"encoding/json"
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
}

// sink carries per-task result detail from an executor run back to the
// worker. Executors share one instance, so the sink travels in the task's
// context instead of on the executor.
type sinkKey struct{}

// Sink collects structured outcome detail for ONE task execution.
type Sink struct {
	mu     sync.Mutex
	detail json.RawMessage
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

var sessionRe = regexp.MustCompile(`(?im)^\s*session(?:[ _-]?id)?\s*[:=]\s*([A-Za-z0-9][A-Za-z0-9_-]*)`)

// ExtractSessionID pulls a crush session id out of agent output.
// Best-effort by design: crush's output format is not a stable contract.
func ExtractSessionID(output string) string {
	if m := sessionRe.FindStringSubmatch(output); m != nil {
		return m[1]
	}
	return ""
}
