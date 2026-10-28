package executor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The report-body half of the secrets-in-logs pass (grid cell
// closeout × security, GAP 2026-10-23): output tails pass redactOutput
// before they reach journal facts, but a)-g) closeout reports and status
// reports are agent-authored prose persisted straight into docs/status —
// no equivalent check guarded them. The gate sweeps every report body
// through the SAME secretPatterns (via RedactSecrets) and fails the
// battery on any surviving secret-shaped string, so a token that leaks
// into a report dies at the verify gate, not in the docs tree.
//
// Sweep 2026-10-28 (one grep pass per pattern class over
// docs/status/**/*.md): zero live-shaped hits. Three benign classes
// exist and are masked before the scan — all pinned in
// reportBenignSecretShapes with their justification.

// reportBenignSecretShapes are the report-body strings that match a
// secret pattern without being secrets. The table is closed: a new entry
// needs a sweep-backed justification, and the gate battery below must
// keep proving the masks spare nothing else.
var reportBenignSecretShapes = []struct {
	name string
	re   *regexp.Regexp
}{
	// "task-000001a0…" references: the classic sk- pattern has no left
	// boundary, so every task-ID mention matches as "sk-…" — the sweep
	// found this is the only sk-class shape docs/status carries.
	{"task-id reference", regexp.MustCompile(`(?i)task-[0-9a-f]{16,}`)},
	// Self-labeled fake probes (fakeAuditProbe) quoted by the reports
	// documenting the audit machinery; no real credential contains
	// "smoketest".
	{"SMOKETEST fake probe", regexp.MustCompile(`(?i)[A-Za-z0-9_-]*smoketest[A-Za-z0-9_-]*`)},
	// All-digit sk- bodies: documentation placeholders ("sk-12345678901
	// 234567890" style test keys, quoted by the 2026-09-16 redaction
	// report — which predicted this exact gate collision). Real classic
	// sk- keys are base62-mixed; a 20+-char all-digit body is fake by
	// construction (P ≈ 4e-9 for a real key).
	{"all-digit fake sk- key", regexp.MustCompile(`sk-[0-9]{20,}`)},
}

// maskBenignReportShapes strips the benign classes so the scan sees only
// secret-shaped strings a report must never carry.
func maskBenignReportShapes(s string) string {
	for _, benign := range reportBenignSecretShapes {
		s = benign.re.ReplaceAllLiteralString(s, "")
	}

	return s
}

// reportSecretOffenses scans one report body and returns one redacted
// offense line per secret-shaped location, safe to print (every match is
// already behind RedactMarker). crossLine reports a match the per-line
// scan cannot attribute — a shape spanning a newline (e.g. a wrapped
// "Authorization: Bearer\n  <key>" log paste).
func reportSecretOffenses(content string) (offenses []string, crossLine bool) {
	masked := maskBenignReportShapes(content)

	for i, line := range strings.Split(masked, "\n") {
		if red := RedactSecrets(line); red != line {
			offenses = append(offenses, fmt.Sprintf("line %d: %s", i+1, red))
		}
	}

	if len(offenses) == 0 && RedactSecrets(masked) != masked {
		crossLine = true
	}

	return offenses, crossLine
}

// TestStatusReportBodiesCarryNoSecretShapedStrings is the report-body
// gate: every docs/status markdown report must scan clean through the
// same redaction table that guards output tails.
func TestStatusReportBodiesCarryNoSecretShapedStrings(t *testing.T) {
	t.Parallel()

	reportsRoot := filepath.Join("..", "..", "docs", "status")
	if _, err := os.Stat(reportsRoot); err != nil {
		t.Fatalf("report surface missing: %v", err)
	}

	checked := 0

	walkErr := filepath.WalkDir(reportsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		checked++

		offenses, crossLine := reportSecretOffenses(string(content))
		for _, offense := range offenses {
			t.Errorf("%s: secret-shaped string in report body: %s", path, offense)
		}

		if crossLine {
			t.Errorf("%s: secret-shaped match spans lines the line scan cannot attribute", path)
		}

		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", reportsRoot, walkErr)
	}

	if checked == 0 {
		t.Fatalf("no reports under %s — the gate is scanning nothing", reportsRoot)
	}
}

// TestReportSecretGateDetectsShapesAndSparesBenign pins the gate's
// contract on the canonical fake fixtures: every fake-shape secret is
// flagged and reported only in redacted form, while the two benign
// classes (task-ID references, SMOKETEST probes) pass through.
func TestReportSecretGateDetectsShapesAndSparesBenign(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		"ref task-000001a0123456789abcdef000000000 and probe " + fakeAuditProbe,
		"digitfake sk-12345678901234567890 documented",
		"anthropic " + fakeAnthropic,
		"openai " + fakeOpenAI,
		"github " + fakeGitHub,
		"fine-grained " + fakeGitHubPat,
		"aws " + fakeAWS,
		"google " + fakeGoogle,
		"slack " + fakeSlack,
		"jwt " + fakeJWT,
		"mixed task-000001a0123456789abcdef000000000 plus " + fakeGitHub,
		"password: hunter2hunter2",
	}, "\n")

	offenses, crossLine := reportSecretOffenses(body)
	if crossLine {
		t.Fatalf("unexpected cross-line offense")
	}

	if len(offenses) != 10 {
		t.Fatalf("offenses = %d (%v), want 10 flagged lines", len(offenses), offenses)
	}

	rawSecrets := []string{
		fakeAuditProbe, fakeAnthropic, fakeOpenAI, fakeGitHub,
		fakeGitHubPat, fakeAWS, fakeGoogle, fakeSlack, fakeJWT,
		"hunter2hunter2",
	}

	for i, offense := range offenses {
		if !strings.Contains(offense, RedactMarker) {
			t.Errorf("offense %d is not redacted: %s", i, offense)
		}

		if strings.Contains(offense, "task-000001a0") || strings.Contains(strings.ToLower(offense), "smoketest") || strings.Contains(offense, "sk-12345678901234567890") {
			t.Errorf("offense %d flags a benign class: %s", i, offense)
		}

		for _, raw := range rawSecrets {
			if strings.Contains(offense, raw) {
				t.Errorf("offense %d echoes the raw secret: %s", i, offense)
			}
		}
	}
}
