// Package incident turns production error reports into journaled facts,
// folds those facts into incidents, and reacts by minting agent fix tasks
// (ADR-0021) — the command/event spine of error-driven auto-fix:
//
//   - Command: Report (from a web-client beacon or server panic/5xx
//     middleware) arrives via the API and is recorded ONCE as an
//     error.observed fact on the synthetic incident:<fingerprint> stream.
//   - Projection: the fold derives incident state (occurrences, status,
//     minted tasks, regressions) purely from journal facts — never a
//     mirrored column.
//   - Policy: the watermark-cursor sweep (the sweeper seam shared with
//     review/dlqfix/status, ADR-0009 exact-consumer semantics) reacts to
//     each new fact: first occurrence mints a hot-band agent fix task,
//     occurrences on an open incident only count, an error observed after
//     the fix task completed is a regression and mints at machine band.
//
// Storm safety: 1,000 identical reports in one day are 1,000 facts but at
// most ONE task per incident lifecycle stage; replayed pages (crash
// between consumption and checkpoint) converge on task dedup keys instead
// of minting duplicates — the same idempotency contract as every sweeper.
package incident

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// IDPrefix marks synthetic incident-stream fact TaskIDs (the session:*
// precedent: an identity no enqueue/claim machinery can ever pick up).
const IDPrefix = "incident:"

// Kind names the production surface an error was observed on.
type Kind string

const (
	// KindClient is an uncaught browser exception or rejection (beacon).
	KindClient Kind = "client"
	// KindServer is a panic or unexpected 5xx from server middleware.
	KindServer Kind = "server"
)

// Field caps enforced by Clip before anything is journaled — the journal
// is append-only and the dashboard renders payloads, so reports are
// bounded at the boundary, not trusted from the wire.
const (
	MaxMessage     = 4096
	MaxStack       = 8192
	MaxBreadcrumbs = 20
	MaxCrumb       = 256
	MaxShort       = 256
)

// Report is the ReportError command payload. Secrets are unrepresentable
// by construction: the type has no place to carry headers, cookies, or
// request bodies — only the fields a fix needs. Free-text fields are
// length-capped by Clip.
type Report struct {
	// Project is the repo (or logical owner) the error belongs to and the
	// unit the minted fix task's Project/Repo derive from. Required.
	Project string `json:"project"`
	// Kind selects client or server. Empty defaults to server.
	Kind Kind `json:"kind"`
	// Message is the error message (exception text or panic value).
	Message string `json:"message"`
	// Stack is the raw stack (minified frames are fine; the agent resolves
	// source maps from the pinned Release in-repo).
	Stack string `json:"stack,omitempty"`
	// Release is the commit SHA (or version) the error was observed on.
	Release string `json:"release,omitempty"`
	// Route is the route pattern (server) or path (client), not the raw
	// URL — query strings never enter the journal.
	Route string `json:"route,omitempty"`
	// Method is the HTTP method, server errors only.
	Method string `json:"method,omitempty"`
	// TraceURL is a deep link to the full trace in the observability stack
	// (SigNoz) — the query surface; this pipeline is the action surface.
	TraceURL string `json:"traceUrl,omitempty"`
	// SessionID correlates reports from one browser session.
	SessionID string `json:"sessionId,omitempty"`
	// Breadcrumbs are the last user actions/requests before the error.
	Breadcrumbs []string `json:"breadcrumbs,omitempty"`
	// OccurredAt is the observation time (zero = recording time).
	OccurredAt time.Time `json:"occurredAt,omitempty"`
}

// Validate checks the required fields with actionable errors.
func (r Report) Validate() error {
	if strings.TrimSpace(r.Project) == "" {
		return errors.New("project is required (the repo the fix task will target)")
	}

	if strings.TrimSpace(r.Message) == "" {
		return errors.New("message is required (the exception text or panic value)")
	}

	if r.Kind != "" && r.Kind != KindClient && r.Kind != KindServer {
		return errors.New("kind must be \"client\" or \"server\"")
	}

	return nil
}

// Clip bounds every free-text field and defaults Kind. Reports are capped
// before they touch the journal, so a hostile or buggy producer cannot
// bloat the append-only fact log.
func (r Report) Clip() Report {
	r.Project = clipStr(r.Project, MaxShort)
	r.Kind = Kind(clipStr(string(r.Kind), MaxShort))

	if r.Kind == "" {
		r.Kind = KindServer
	}

	r.Message = clipStr(r.Message, MaxMessage)
	r.Stack = clipStr(r.Stack, MaxStack)
	r.Release = clipStr(r.Release, MaxShort)
	r.Route = clipStr(r.Route, MaxShort)
	r.Method = clipStr(r.Method, MaxShort)
	r.TraceURL = clipStr(r.TraceURL, MaxShort)
	r.SessionID = clipStr(r.SessionID, MaxShort)

	if len(r.Breadcrumbs) > MaxBreadcrumbs {
		r.Breadcrumbs = r.Breadcrumbs[len(r.Breadcrumbs)-MaxBreadcrumbs:]
	}

	crumbs := make([]string, 0, len(r.Breadcrumbs))

	for _, c := range r.Breadcrumbs {
		crumbs = append(crumbs, clipStr(c, MaxCrumb))
	}

	r.Breadcrumbs = crumbs

	return r
}

func clipStr(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	if max > 0 && len(s) > max {
		cut := s[:max]
		if utf8.ValidString(s) {
			// A cut of a valid string can only be invalid by splitting
			// the final rune; back off to its start so no character is
			// halved into U+FFFD garbage.
			for !utf8.ValidString(cut) {
				cut = cut[:len(cut)-1]
			}
		}
		s = strings.TrimSpace(cut)
	}

	return s
}

var (
	// digitRun collapses numbers (ids, counts, ports) so "failed after 3
	// retries" and "failed after 7 retries" fingerprint alike.
	digitRun = regexp.MustCompile(`\d+`)
	// quotedRun collapses quoted strings (paths, names) the same way.
	quotedRun = regexp.MustCompile(`("[^"]*"|'[^']*')`)
	// spaceRun collapses whitespace runs to single spaces.
	spaceRun = regexp.MustCompile(`\s+`)
)

// Fingerprint groups reports that share one bug: project + kind + the
// normalized message + the top stack frame (numbers, quoted strings, and
// whitespace collapsed). The release is deliberately EXCLUDED so the same
// bug groups across releases — a recurrence after a fix is the regression
// signal, not a new release's fresh incident. Returns 16 hex chars.
func Fingerprint(r Report) string {
	top := TopFrame(r.Stack)

	h := sha256.New()
	h.Write([]byte(r.Project))
	h.Write([]byte{0})
	h.Write([]byte(r.Kind))
	h.Write([]byte{0})
	h.Write([]byte(normalize(r.Message)))
	h.Write([]byte{0})
	h.Write([]byte(normalize(top)))

	return hex.EncodeToString(h.Sum(nil))[:16]
}

// TopFrame extracts the first stack line (the crashing frame), stripped
// of common prefixes ("at ", goroutine markers). Empty stack yields "".
func TopFrame(stack string) string {
	for _, line := range strings.Split(stack, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		line = strings.TrimPrefix(line, "at ")
		if strings.HasPrefix(line, "goroutine ") || strings.HasPrefix(line, "created by ") {
			continue
		}

		return line
	}

	return ""
}

func normalize(s string) string {
	s = quotedRun.ReplaceAllString(s, `"…"`)
	s = digitRun.ReplaceAllString(s, "#")

	return spaceRun.ReplaceAllString(s, " ")
}

// IncidentID is the synthetic journal-stream identity for one
// fingerprint: the fact TaskID every error.observed and
// incident.task-minted fact of this incident carries.
func IncidentID(fingerprint string) string {
	return IDPrefix + fingerprint
}

// FingerprintOfID extracts the fingerprint from an incident stream ID
// ("" when the ID does not carry the prefix).
func FingerprintOfID(id string) string {
	return strings.TrimPrefix(id, IDPrefix)
}
