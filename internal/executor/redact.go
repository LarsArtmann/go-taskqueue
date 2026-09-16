package executor

import (
	"cmp"
	"os"
	"regexp"
	"slices"
)

// RedactMarker replaces every provider-token-shaped match in tails,
// evidence facts and sidecar logs. Uniform on purpose: the tail's job is
// proving what failed, not identifying which credential leaked.
const RedactMarker = "[REDACTED]"

// secretPatterns are the provider-token shapes redacted from output tails
// before they reach journal facts, worker logs and sidecar files (the
// secrets-in-logs pass, 17-21 report #18 / 20-58 f33). Patterns are
// deliberately conservative — a false positive only mangles a debug tail,
// but a miss persists a live credential forever, so prefixes stay tight
// (sk- needs 20+ token chars so "task-skills-…" prose never matches) and
// free-form words (token, pwd) are left out on collision risk.
var secretPatterns = []*regexp.Regexp{
	// Anthropic (sk-ant-…)
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{16,}`),
	// OpenAI project / service-account keys (sk-proj-…, sk-svcacct-…)
	regexp.MustCompile(`sk-(?:proj|svcacct)-[A-Za-z0-9_-]{20,}`),
	// OpenAI classic keys: sk- + 20+ token chars (real keys carry 40+)
	regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
	// GitHub tokens (ghp_/gho_/ghu_/ghs_/ghr_ + fine-grained github_pat_)
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	// AWS access key ids
	regexp.MustCompile(`(?:AKIA|ASIA)[A-Z0-9]{16}`),
	// Google API keys (AIza + exactly 35 chars)
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),
	// Slack tokens
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
	// Bearer / Authorization headers (JWTs and raw keys)
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`),
	regexp.MustCompile(`(?i)\bauthorization["']?\s*[:=]\s*["']?(?:bearer\s+)?[A-Za-z0-9._~+/=-]{16,}`),
	// Secret-shaped key=value / key: value assignments (12+ value chars so
	// "password=true" style prose survives)
	regexp.MustCompile(`(?i)\b(?:api[_-]?key|apikey|secret|access[_-]?token|auth[_-]?token|password|passwd)\b["']?\s*[:=]\s*["']?[A-Za-z0-9+/_-]{12,}`),
}

// SecretHits counts distinct provider-token-shaped locations in s — the
// detector half of the pass: tq audit --journal scans stored facts with it
// so tokens that landed in evidence BEFORE the redaction pass are findable
// without ever printing them. Pattern shapes overlap by design (the bearer
// and authorization-header shapes both match one "Authorization: Bearer …"
// line), so overlapping match spans are merged before counting — the hit
// count estimates secret LOCATIONS, not pattern matches.
func SecretHits(s string) int {
	type span struct{ start, end int }

	var spans []span
	for _, re := range secretPatterns {
		for _, loc := range re.FindAllStringIndex(s, -1) {
			spans = append(spans, span{start: loc[0], end: loc[1]})
		}
	}

	if len(spans) == 0 {
		return 0
	}

	slices.SortFunc(spans, func(a, b span) int { return cmp.Compare(a.start, b.start) })

	hits := 1
	mergedEnd := spans[0].end

	for _, candidate := range spans[1:] {
		if candidate.start < mergedEnd {
			mergedEnd = max(mergedEnd, candidate.end)
			continue
		}
		hits++
		mergedEnd = candidate.end
	}

	return hits
}

// RedactSecrets masks every provider-token-shaped match in s. Pure: no
// env, no I/O — surfaces that must redact unconditionally (and the audit)
// call it directly.
func RedactSecrets(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllLiteralString(s, RedactMarker)
	}

	return s
}

// redactActive reports whether output redaction is on: default yes, off
// only via TQ_REDACT=false (the --redact=false escape hatch for raw-output
// debugging; the pool and worker CLIs mirror the flag into the env).
func redactActive() bool {
	return os.Getenv("TQ_REDACT") != "false"
}

// redactOutput applies the secrets-in-logs pass to process output when
// redaction is active — tails for evidence facts and error text, and the
// full sidecar bodies alike. Runs over the FULL buffer before any tail is
// cut so a token spanning the tail boundary is masked whole, not split in
// half.
func redactOutput(s string) string {
	if !redactActive() {
		return s
	}

	return RedactSecrets(s)
}

// redactBytes is redactOutput over raw process output without the
// string copy: the regex engine scans the original buffer.
func redactBytes(b []byte) []byte {
	if !redactActive() {
		return b
	}

	marker := []byte(RedactMarker)

	for _, re := range secretPatterns {
		b = re.ReplaceAllLiteral(b, marker)
	}

	return b
}
