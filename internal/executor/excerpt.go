package executor

import (
	"strings"
	"unicode/utf8"
)

// excerptMaxLen bounds one excerpt line; three surfaces (payload rendering,
// status window entries, session summaries) agreed on this pointer length.
const excerptMaxLen = 200

// Excerpt reduces an item text, agent prompt, or session summary to its
// first line, bounded to excerptMaxLen bytes — a one-line pointer, never a
// transcript (the full text stays in the payload, read via `tq show`).
// Truncation is rune-safe: the byte cut backs off to a whole UTF-8 boundary
// so a multi-byte prompt never renders a broken rune. Trailing whitespace
// is trimmed before the ellipsis so a mid-line truncation never ends in a
// dangling space.
func Excerpt(text string) string {
	line := strings.TrimSpace(text)
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}

	if len(line) > excerptMaxLen {
		cut := line[:excerptMaxLen]
		for len(cut) > 0 && !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}

		line = strings.TrimSpace(cut) + "…"
	}

	return line
}
