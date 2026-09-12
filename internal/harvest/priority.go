package harvest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// Priority resolution for harvested items (ADR-0015 §2–§3): a trailing
// human marker beats the AI score, which beats importance + keyword bumps,
// which beats the flat default — with same-session (hot) items jumping the
// whole ladder because their /tmp evidence will not survive the session.

// DefaultImportance is the effective importance of a repo without a
// .config/metadata.yaml (project-meta file contract, ADR-0015 §7).
const DefaultImportance = 50

// MaxMarkerLevel is the highest valid marker level (P4).
const MaxMarkerLevel = 4

// The marker levels as constants (P1–P4), for switch cases and range
// checks.
const (
	markerLevel1 = 1
	markerLevel2 = 2
	markerLevel3 = 3
)

// The marker ladder (ADR-0015 §2): adjacent levels differ by 20, strictly
// more than queue.PriorityAgingMaxBonus, so aging can never reorder across
// marker levels.
const (
	markerPriorityP1 = 90
	markerPriorityP2 = 70
	markerPriorityP3 = 50
	markerPriorityP4 = 30
)

// MarkerPriority returns the backlog priority of a marker level — P1
// through P4. 0 ("no marker") maps to 0.
func MarkerPriority(level int) int {
	switch level {
	case markerLevel1:
		return markerPriorityP1
	case markerLevel2:
		return markerPriorityP2
	case markerLevel3:
		return markerPriorityP3
	case MaxMarkerLevel:
		return markerPriorityP4
	}

	return 0
}

// markerPattern matches a trailing marker segment: an em dash (U+2014,
// the established `— BLOCKED:` convention), the marker P1–P4, and an
// optional `: <free-text note>`, at end of line. Mid-text "P1" never
// matches; one marker per line.
var markerPattern = regexp.MustCompile(`\s+—\s*P([1-4])(?::.*)?$`)

// SplitMarker splits a trailing priority marker off an item line,
// returning the marker level (1–4) and the marker-free text. ok is false
// when the text carries no marker — including a marker-only line (there is
// no item left to prioritize).
func SplitMarker(text string) (int, string, bool) {
	m := markerPattern.FindStringSubmatchIndex(text)
	if m == nil {
		return 0, text, false
	}

	level, err := strconv.Atoi(text[m[2]:m[3]])
	if err != nil || level < 1 || level > MaxMarkerLevel {
		return 0, text, false
	}

	stripped := strings.TrimRight(text[:m[0]], " \t")
	if strings.TrimSpace(stripped) == "" {
		return 0, text, false
	}

	return level, stripped, true
}

// The keyword-bump ladder, strongest first (ADR-0015 §3). One bump wins —
// they do not stack. REWRITTEN FROM SPEC: the shape is inspired by
// ai-task-prioritizer's urgency keywords, the table itself is this repo's
// own; nothing is copied (proprietary license, ADR-0015 §7).
const (
	keywordBumpSecurity   = 30
	keywordBumpCritical   = 25
	keywordBumpProduction = 20
)

// KeywordBump reports the strongest keyword bump for an item text, 0 when
// none matches. Case-insensitive.
func KeywordBump(text string) int {
	lower := strings.ToLower(text)

	switch {
	case containsAny(lower, "security", "vulnerab", "cve"):
		return keywordBumpSecurity
	case containsAny(lower, "critical", "urgent", "asap"):
		return keywordBumpCritical
	case containsAny(lower, "production", "breaking", "outage"):
		return keywordBumpProduction
	}

	return 0
}

// containsAny reports whether s contains any of the substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}

	return false
}

// errImportanceMalformed is the static class behind every metadata parse
// refusal; details ride on the wrapped error.
var errImportanceMalformed = errors.New("metadata importance malformed")

// ReadImportance reads a repo's importance from its .config/metadata.yaml
// (the project-meta flat file contract). Absent file: DefaultImportance.
// Present but malformed (unparseable importance, or out of 0–100): an
// error the caller reports as a repo skip reason — an importance the owner
// DID set must never silently degrade to a default.
//
// This is a deliberate strict-subset reader: project-meta is proprietary
// (ADR-0015 §7), so tq parses the two bytes it needs instead of requiring
// the module. Only a top-level `importance:` key counts; indented keys
// belong to nested mappings and are ignored.
func ReadImportance(repo string) (int, error) {
	data, err := os.ReadFile(filepath.Join(repo, ".config", "metadata.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultImportance, nil
		}

		return 0, fmt.Errorf("read metadata: %w", err)
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		if isNestedOrIgnorable(line) {
			continue
		}

		name, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(name) != "importance" {
			continue
		}

		return parseImportanceValue(value)
	}

	// File exists but carries no top-level importance: treat like absent.
	return DefaultImportance, nil
}

// isNestedOrIgnorable reports whether a metadata line cannot carry a
// top-level key: blank, indented (nested mapping member), or a comment.
func isNestedOrIgnorable(line string) bool {
	return line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#'
}

// parseImportanceValue parses the scalar after `importance:`, tolerating
// quoted scalars, and enforces the 0–100 range.
func parseImportanceValue(value string) (int, error) {
	scalar := strings.TrimSpace(value)
	if len(scalar) >= 2 && (scalar[0] == '"' || scalar[0] == '\'') && scalar[len(scalar)-1] == scalar[0] {
		scalar = scalar[1 : len(scalar)-1]
	}

	parsed, err := strconv.Atoi(scalar)
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not an integer", errImportanceMalformed, value)
	}

	if parsed < 0 || parsed > 100 {
		return 0, fmt.Errorf("%w: %d out of range 0-100", errImportanceMalformed, parsed)
	}

	return parsed, nil
}

// PrioritySource names what produced an effective priority (ADR-0015 §3).
type PrioritySource string

const (
	// PrioritySourceHot: same-session promotion.
	PrioritySourceHot PrioritySource = "hot"
	// PrioritySourceMarker: trailing `— P[1-4]` human marker.
	PrioritySourceMarker PrioritySource = "marker"
	// PrioritySourceAI: cached priority_scores verdict.
	PrioritySourceAI PrioritySource = "ai"
	// PrioritySourceKeyword: importance + keyword bump.
	PrioritySourceKeyword PrioritySource = "keyword"
	// PrioritySourceImportance: bare repo importance.
	PrioritySourceImportance PrioritySource = "importance"
	// PrioritySourceDefault: flat config priority (legacy behavior).
	PrioritySourceDefault PrioritySource = "default"
)

// ResolveInput carries every signal the precedence resolver considers.
// Text is the MARKER-STRIPPED item text; MarkerLevel is 0 (absent) or 1–4.
type ResolveInput struct {
	Text              string
	MarkerLevel       int
	HotPriority       int  // --same-session-priority; <= 0 disables hot
	FlatPriority      int  // --priority fallback when nothing else applies
	Importance        int  // repo importance, already defaulted to 50
	ImportanceEnabled bool // importance + keyword composite on
	AIScore           *int // cached AI score; nil = no verdict yet
}

// ResolvePriority applies the precedence ladder (ADR-0015 §3) and returns
// the effective priority with the source that won. Backlog computations
// clamp to [0, queue.BacklogMax]; only HotPriority may cross upward.
func ResolvePriority(input ResolveInput) (int, PrioritySource) {
	if input.HotPriority > 0 && sameSession(input.Text) {
		return input.HotPriority, PrioritySourceHot
	}

	if input.MarkerLevel >= 1 && input.MarkerLevel <= MaxMarkerLevel {
		return MarkerPriority(input.MarkerLevel), PrioritySourceMarker
	}

	if input.AIScore != nil {
		return queue.ClampBacklog(*input.AIScore), PrioritySourceAI
	}

	if input.ImportanceEnabled {
		if bump := KeywordBump(input.Text); bump > 0 {
			return queue.ClampBacklog(input.Importance + bump), PrioritySourceKeyword
		}

		return queue.ClampBacklog(input.Importance), PrioritySourceImportance
	}

	return input.FlatPriority, PrioritySourceDefault
}

// RepriMutable reports whether a re-resolution pass (tq reprioritize, the
// prioritize sweeper) may rewrite a task currently stored at current:
// hot (100–149) and machine (150+) priorities are explicit human/flag
// acts and are never overwritten by automated re-resolution — the band
// protection half of the ADR-0015 §3 precedence.
func RepriMutable(current int) bool {
	return queue.BandOf(current) == queue.BandBacklog
}
