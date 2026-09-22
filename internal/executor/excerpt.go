package executor

import "strings"

// excerptMaxLen bounds one excerpt line; three surfaces (payload rendering,
// status window entries, session summaries) agreed on this pointer length.
const excerptMaxLen = 200

// Excerpt reduces an item text, agent prompt, or session summary to its
// first line, bounded to excerptMaxLen bytes — a one-line pointer, never a
// transcript (the full text stays in the payload, read via `tq show`).
// Trailing whitespace is trimmed after the cut so a mid-line truncation
// never ends in a dangling space before the ellipsis.
func Excerpt(text string) string {
	line := strings.TrimSpace(text)
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}

	if len(line) > excerptMaxLen {
		line = line[:excerptMaxLen] + "…"
	}

	return strings.TrimSpace(line)
}
