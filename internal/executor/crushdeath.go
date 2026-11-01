package executor

import (
	"fmt"
)

// CrushServerDeathCode marks shared crush-server deaths in errors and
// journal facts: a routed run severed by the server dying mid-run (or by
// the socket being unreachable when the client's respawn also fails).
// These deaths are pool infrastructure, never the task's verdict — the
// worker requeues without burning an attempt and the supervisor's restart
// ladder owns the recovery; IsEnvironmentalDeath refuses DLQ autopsies
// for the same marker.
const CrushServerDeathCode = "crush-server"

// crushServerDeathMarkers are the observed (v0.99.0) death signatures:
//   - "Events stream ended": the severed SSE events stream — the client
//     exits 0 with only a WARN line, so the OUTPUT must be scanned, not
//     just the error (a normal routed completion emits no such line;
//     two-sided pinned by TestDetectCrushServerDeath and the
//     crush-server-route smoke's severed leg).
//   - "dial unix": the client could not reach the socket (dead at attach
//     with a failed respawn) — only matched on the error path.
var crushServerDeathMarkers = []string{
	"Events stream ended",
	"dial unix",
}

// DetectCrushServerDeath reports whether a routed run's output or error
// text carries the shared-server death signature. Output matching is only
// sound for routed runs (the caller gates on CrushServer != ""); the
// severed-stream WARN is the rc-0 lie a mid-run death produces.
func DetectCrushServerDeath(output, errText string) bool {
	for _, marker := range crushServerDeathMarkers {
		if containsFold(output, marker) || containsFold(errText, marker) {
			return true
		}
	}

	return false
}

// CrushServerDeathError is the typed outcome for a routed agent turn
// severed by the shared crush server dying: the worker requeues it
// WITHOUT burning an attempt (RequeueClassCrushServer), exempt from the
// env-streak breaker — the server supervisor restarts in seconds and a
// sustained outage must park its tasks, not spend their attempts.
type CrushServerDeathError struct {
	Cause error
}

func (e *CrushServerDeathError) Error() string {
	cause := "<run severed mid-turn>"
	if e.Cause != nil {
		cause = e.Cause.Error()
	}

	return fmt.Sprintf(
		"crush server died around the routed run [%s]: %s — the supervisor restarts it; the task requeues without attempt burn",
		CrushServerDeathCode,
		cause,
	)
}

func (e *CrushServerDeathError) Unwrap() error { return e.Cause }

// containsFold is a tiny ASCII-case-insensitive Contains (log level
// casing varies: WARN/warn).
func containsFold(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}

	if len(s) < len(substr) {
		return false
	}

	for i := 0; i+len(substr) <= len(s); i++ {
		if equalFoldASCII(s[i:i+len(substr)], substr) {
			return true
		}
	}

	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		ca, cb := a[i], b[i]

		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}

		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}

		if ca != cb {
			return false
		}
	}

	return true
}
