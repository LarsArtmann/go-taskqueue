package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStaleVerifyReasons pins the known-stale verify patterns (f46 audit
// class, 09-39 report §e3): every historical minted form flags with the
// right reasons, the current mint and non-Go gates come back clean.
func TestStaleVerifyReasons(t *testing.T) {
	t.Parallel()

	// The pre-gofmt mint (env-complete walk, no formatting stage) is not
	// broken, only less strict: clean.
	preGofmtMint := withGoEnvPrelude("go build ./... && go test ./... -count=1" +
		" && for f in $(find . -mindepth 2 -name go.mod -not -path '*/vendor/*');" +
		" do (cd \"${f%/*}\" && go build ./... && go test ./... -count=1) || exit 1; done")
	if got := StaleVerifyReasons(preGofmtMint); got != nil {
		t.Errorf("pre-gofmt mint must be clean, got %v", got)
	}

	// The live mint (now carrying the scoped gofmt stage) stays clean.
	goDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := StaleVerifyReasons(defaultVerify(goDir)); got != nil {
		t.Errorf("defaultVerify output must be clean, got %v", got)
	}

	for name, tc := range map[string]struct {
		verify string
		want   []string
	}{
		"empty": {verify: ""},
		"npm":   {verify: "npm test --silent"},
		"make":  {verify: "make test"},
		"cargo": {verify: "cargo test --quiet"},
		"nix":   {verify: "nix build && nix flake check"},
		"rootOnly": {
			// The original single-module default (pre-2026-09-09).
			verify: "go build ./... && go test ./... -count=1",
			want: []string{
				"root-module-only gate",
				GoEnvExperiment + " export",
			},
		},
		"rootOnlyWithPrelude": {
			// A hand-composed prelude does not fix the vacuous walk.
			verify: "export " + GoEnvExperiment + "; go build ./... && go test ./... -count=1",
			want:   []string{"root-module-only gate"},
		},
		"execdirWalk": {
			// The 2026-09-09 intermediate mint (swallowed exit status).
			verify: "go build ./... && go test ./... -count=1" +
				" && find . -mindepth 2 -name go.mod -not -path '*/vendor/*'" +
				" -execdir sh -c 'go build ./... && go test ./... -count=1' \\;",
			want: []string{
				"-execdir",
				GoEnvExperiment + " export",
			},
		},
		"walkWithoutPrelude": {
			// The explicit-exit walk before the env-self-contained mint.
			verify: "go build ./... && go test ./... -count=1" +
				" && for f in $(find . -mindepth 2 -name go.mod -not -path '*/vendor/*');" +
				" do (cd \"${f%/*}\" && go build ./... && go test ./... -count=1) || exit 1; done",
			want: []string{GoEnvExperiment + " export"},
		},
		"unscopedGofmt": {
			// The pre-P2-fix .tq-verify shape: env-complete, walk complete,
			// but the gofmt stage flags gitignored vendor/ trees.
			verify: "export " + GoEnvExperiment + "; go build ./... && go test ./... -count=1" +
				" && test -z \"$(gofmt -l .)\"" +
				" && for f in $(find . -mindepth 2 -name go.mod -not -path '*/vendor/*');" +
				" do (cd \"${f%/*}\" && go build ./... && go test ./... -count=1) || exit 1; done",
			want: []string{"unscoped gofmt stage"},
		},
	} {
		got := StaleVerifyReasons(tc.verify)
		if len(got) != len(tc.want) {
			t.Errorf("%s: StaleVerifyReasons(%q) = %v, want %d reason(s)", name, tc.verify, got, len(tc.want))

			continue
		}

		for i, want := range tc.want {
			if !strings.Contains(got[i], want) {
				t.Errorf("%s: reason %d = %q, want it to contain %q", name, i, got[i], want)
			}
		}
	}
}
