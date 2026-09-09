package postgres

import (
	"encoding/json"
	"strings"
	"time"
)

// The micro-utilities below mirror internal/queue/sqlite's helpers
// one-for-one. The two backends deliberately duplicate their shared
// plumbing (ADR-0007 conformance mirroring): each store stays a
// self-contained twin whose side-by-side diff IS the semantics contract.

func ms(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}

	return t.UnixMilli()
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}

	return b
}

func maybeJSON(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return nil
	}

	return r
}

// escapeLike escapes LIKE wildcards so a user query containing %, _ or \
// matches literally instead of pattern-wide.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)

	return s
}

// failureDetail picks a task.failed fact's detail: the executor's failure
// evidence when present, else the store's classification fallback.
func failureDetail(evidence json.RawMessage, class string) json.RawMessage {
	if len(evidence) > 0 {
		return evidence
	}

	return mustJSON(map[string]string{"class": class})
}

// cancelReasonDetail builds the task.cancelled detail: nil without a
// reason (no detail noise), {"reason": ...} with one.
func cancelReasonDetail(reason string) json.RawMessage {
	if reason == "" {
		return nil
	}

	return mustJSON(map[string]string{"reason": reason})
}

// cooperativeCancelDetail builds the task.cancelled detail for a
// cooperative finalize: the cooperative marker, the finalize context
// ("after" key, when set) and the operator's reason, when one was given.
func cooperativeCancelDetail(reason, after string) json.RawMessage {
	detail := map[string]string{"cooperative": "true"}
	if after != "" {
		detail["after"] = after
	}

	if reason != "" {
		detail["reason"] = reason
	}

	return mustJSON(detail)
}
