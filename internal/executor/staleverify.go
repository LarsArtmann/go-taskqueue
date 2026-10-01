package executor

import "strings"

// StaleVerifyReasons reports why a verify pin matches a KNOWN-STALE minted
// verify form — the f46 audit class (09-39 report §e3): a task enqueued
// before the verify contract moved on still carries the old gate, and the
// doctor's --hygiene check must flag it as a gate, not an investigation.
// The predicates are REPO-INDEPENDENT (pure content matching), so the check
// works even where the repo directory is absent. Returns nil for a clean
// pin; each reason is one line of operator-facing text.
//
// The known-stale forms are the historical Go verify mints, in order:
//
//  1. "go build ./... && go test ./... -count=1" — root-module-only, so a
//     multi-module repo verifies vacuously (the original f46 incident).
//  2. the find -execdir walk — swallows the inner exit status (find reports
//     only its own errors), so a red nested gate passed unnoticed.
//  3. any Go gate without the GOEXPERIMENT=jsonv2 export — predates the
//     env-self-contained mint and dies outside the flake devShell on
//     repos importing encoding/json/v2 (the 2026-09-11 env-lie class).
//  4. any gofmt stage without the gitignore scope — the unscoped
//     `test -z "$(gofmt -l .)"` shape flags gitignored vendor/ trees and
//     kills the gate on files the agent cannot even commit (the
//     2026-10-01 P2 class: 167 dead letters, every dispatched task killed
//     after doing real work).
//
// The current mint (goEnvPrelude + the explicit-exit `for f in $(find …)`
// walk + the gitignore-scoped gofmt stage) matches none of the
// predicates; non-Go gates (npm/make/cargo/nix) are out of scope for the
// Go-mint evolution and always come back clean.
func StaleVerifyReasons(verify string) []string {
	if !isGoVerify(verify) {
		return nil
	}

	var reasons []string

	// The nested-module walk is the mint marker: every walked form names
	// the go.mod glob, the root-only form does not.
	if !strings.Contains(verify, "-name go.mod") {
		reasons = append(
			reasons,
			"root-module-only gate — the nested-module walk (find -name go.mod) is missing, so a multi-module repo verifies vacuously (f46 class)",
		)
	}

	if strings.Contains(verify, "-execdir") {
		reasons = append(reasons,
			"find -execdir walk swallows the inner exit status (superseded by the explicit-exit loop mint)")
	}

	if !strings.Contains(verify, GoEnvExperiment) {
		reasons = append(
			reasons,
			"missing the "+GoEnvExperiment+" export — predates the env-self-contained mint, dies outside the flake devShell on encoding/json/v2 repos",
		)
	}

	if strings.Contains(verify, "gofmt -l") && !strings.Contains(verify, "git check-ignore") {
		reasons = append(reasons,
			"unscoped gofmt stage: `gofmt -l .` flags gitignored vendor/ trees and kills the gate on files the agent cannot commit (P2 vendor-gofmt class); scope the stage with git check-ignore (executor.ScopedGofmtStage)",
		)
	}

	return reasons
}

// isGoVerify reports whether a verify command is Go-shaped (a Go build or
// test invocation), the only shape the mint evolution applies to. Token
// matching keeps near-misses like `cargo test` out.
func isGoVerify(verify string) bool {
	goTool := false

	for field := range strings.FieldsSeq(verify) {
		if field == "go" {
			goTool = true

			continue
		}

		if goTool && (field == "build" || field == "test") {
			return true
		}
	}

	return false
}
