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

// MarkerPriorities maps marker levels P1–P4 to their priority values
// (ADR-0015 §2). Adjacent levels differ by 20 — strictly more than
// queue.PriorityAgingMaxBonus — so aging can never reorder across marker
// levels.
var MarkerPriorities = [4]int{90, 70, 50, 30}

// markerPattern matches a trailing marker segment: an em dash (U+2014,
// the established `— BLOCKED:` convention), the marker P1–P4, and an
// optional `: <free-text note>`, at end of line. Mid-text "P1" never
// matches; one marker per line.
var markerPattern = regexp.MustCompile(`\s+—\s*P([1-4])(?::.*)?$`)

// SplitMarker splits a trailing priority marker off an item line,
// returning the marker level (1–4) and the marker-free text. ok is false
// when the text carries no marker.
func SplitMarker(text string) (level int, stripped string, ok bool) {
	m := markerPattern.FindStringSubmatchIndex(text)
	if m == nil {
		return 0, text, false
	}

	level, err := strconv.Atoi(text[m[2]:m[3]])
	if err != nil || level < 1 || level > len(MarkerPriorities) {
		return 0, text, false
	}

	return level, strings.TrimRight(text[:m[0]], " \t"), true
}

// MarkerPriority returns the backlog priority of a marker level (P1=90 …
// P4=30); 0 for "no marker".
func MarkerPriority(level int) int {
	if level < 1 || level > len(MarkerPriorities) {
		return 0
	}

	return MarkerPriorities[level-1]
}

// keywordTable is the keyword-bump ladder, strongest match first (one bump
// wins, they do not stack). REWRITTEN FROM SPEC (ADR-0015 §3) — the shape
// is inspired by ai-task-prioritizer's urgency keywords, the table itself
// is this repo's own; nothing is copied (proprietary license, ADR-0015 §7).
var keywordTable = []struct {
	keywords []string
	bump     int
}{
	{[]string{"security", "vulnerab", "cve"}, 30},
	{[]string{"critical", "urgent", "asap"}, 25},
	{[]string{"production", "breaking", "outage"}, 20},
}

// KeywordBump reports the strongest keyword bump for an item text, 0 when
// none matches. Case-insensitive.
func KeywordBump(text string) int {
	lower := strings.ToLower(text)

	for _, row := range keywordTable {
		for _, kw := range row.keywords {
			if strings.Contains(lower, kw) {
				return row.bump
			}
		}
	}

	return 0
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

	if input.MarkerLevel >= 1 && input.MarkerLevel <= len(MarkerPriorities) {
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
