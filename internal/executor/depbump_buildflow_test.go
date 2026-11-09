//go:build unix

package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSyncGomodDelegatesToBuildflow pins the delegation seam: a repo
// carrying .buildflow.yml routes the tidy/vendor parity ritual through
// `buildflow -s gomod-check --fix` (the stub proves it ran), while a repo
// without the marker keeps the hand-rolled go mod tidy pair.
func TestSyncGomodDelegatesToBuildflow(t *testing.T) {
	binDir := t.TempDir()
	callsLog := filepath.Join(t.TempDir(), "calls.log")
	stub := filepath.Join(binDir, "buildflow")

	script := "#!/bin/sh\nprintf '%s\\n' \"buildflow gomod-check --fix ran: $*\" >> " + callsLog + "\necho buildflow-delegated\n"
	if err := os.WriteFile(stub, []byte(script), 0o700); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	e := &DepBumpExecutor{ExtraEnv: []string{GoEnvExperiment}}

	covered := t.TempDir()
	if err := os.WriteFile(filepath.Join(covered, ".buildflow.yml"), []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	out, err := e.syncGomod(context.Background(), covered, time.Minute)
	if err != nil {
		t.Fatalf("covered repo sync: %v", err)
	}

	if !strings.Contains(out, "buildflow-delegated") {
		t.Fatalf("delegation expected, got out=%q", out)
	}

	logged, err := os.ReadFile(callsLog)
	if err != nil || !strings.Contains(string(logged), "gomod-check") {
		t.Fatalf("stub call not logged: %q err=%v", logged, err)
	}
}

// TestSyncGomodFallsBackWithoutBinary pins the fallback: .buildflow.yml
// present but no buildflow binary resolvable keeps the bump moving
// through the hand-rolled pair (environment class, not a contract
// violation).
func TestSyncGomodFallsBackWithoutBinary(t *testing.T) {
	e := &DepBumpExecutor{
		ExtraEnv:     []string{GoEnvExperiment},
		BuildflowBin: filepath.Join(t.TempDir(), "absent-buildflow"),
	}

	covered := t.TempDir()
	if err := os.WriteFile(filepath.Join(covered, ".buildflow.yml"), []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(covered, "go.mod"),
		[]byte("module example.com/fallback\n\ngo 1.27\n"),
		0o600,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	if _, err := e.syncGomod(context.Background(), covered, time.Minute); err != nil {
		t.Fatalf("fallback sync: %v", err)
	}
}
