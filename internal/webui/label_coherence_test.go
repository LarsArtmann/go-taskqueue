package webui

import (
	"os"
	"strings"
	"testing"
)

// stripCSSComments removes /* ... */ blocks so comment prose never counts as
// selector text.
func stripCSSComments(src string) string {
	var b strings.Builder
	for {
		start := strings.Index(src, "/*")
		if start < 0 {
			b.WriteString(src)
			return b.String()
		}
		end := strings.Index(src[start:], "*/")
		if end < 0 {
			b.WriteString(src)
			return b.String()
		}
		b.WriteString(src[:start])
		src = src[start+end+2:]
	}
}

// TestLowercaseLabelScope is the §f21/§f22 coherence audit: the below-the-hull
// sweep retired the all-caps eyebrows by retuning the legacy label classes to
// lowercase — but ONLY on the tq-* surfaces. The /health page renders the
// go-health-dashboard library (tc-* classes) and the machine API (internal/
// httpapi) speaks JSON with no label surface at all, so every
// text-transform:lowercase declaration in theme.css must sit on a rule whose
// selectors are all tq-prefixed. A bare-element or tc-* selector here would
// restyle a sister surface through shared CSS — this test pins the seam shut.
func TestLowercaseLabelScope(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("theme.css")
	if err != nil {
		t.Fatalf("read theme.css: %v", err)
	}

	for _, rule := range strings.Split(stripCSSComments(string(src)), "}") {
		if !strings.Contains(rule, "text-transform: lowercase") {
			continue
		}
		sel, _, found := strings.Cut(rule, "{")
		if !found {
			t.Fatalf("malformed rule without selector: %q", rule)
		}
		// Only the rule's own selector list matters: an enclosing @media or
		// @supports at-rule may precede the selector, so take the last segment.
		if at := strings.LastIndex(sel, "}"); at >= 0 {
			sel = sel[at+1:]
		}
		for _, name := range strings.Split(sel, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				t.Fatalf("empty selector in rule %q", rule)
			}
			if !strings.HasPrefix(name, ".tq-") && !strings.HasPrefix(name, ".dark .tq-") {
				t.Fatalf("lowercase rule leaks past the tq-* seam: selector %q (sister surfaces /health + httpapi must stay untouched)", name)
			}
		}
	}
}
