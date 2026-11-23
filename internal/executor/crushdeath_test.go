//go:build unix

package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDetectCrushServerDeath pins the two-sided death signature: the
// v0.99.0 severed-stream WARN (rc-0 lie, output-scanned), the dial-unix
// attach failure, and the v0.102.0 fail-on-drop error pair (upstream
// 2a8be165) — versus a normal routed completion that must never match.
func TestDetectCrushServerDeath(t *testing.T) {
	severed := "INFO  Running in non-interactive mode\n" +
		"INFO  Created session for non-interactive run session_id=2d5cfb51\n" +
		"warn  Events stream ended error=\"unexpected EOF\"\n"
	normal := "INFO  Running in non-interactive mode\n" +
		"INFO  Created session for non-interactive run session_id=2d5cfb51\n" +
		"result text\n"
	verboseCompletion := "INFO  Running in non-interactive mode\n" +
		"INFO  Created session for non-interactive run session_id=9f3ab1\n" +
		"INFO  Session completed usage_in=120 usage_out=340\n" +
		"result text\n"

	if !DetectCrushServerDeath(severed, "") {
		t.Fatal("severed events stream must classify as crush-server death")
	}

	if DetectCrushServerDeath(normal, "") {
		t.Fatal("a normal routed completion must NOT classify (rc-0 runs are the lie, not the rule)")
	}

	if DetectCrushServerDeath(verboseCompletion, "") {
		t.Fatal("a verbose routed completion (session + usage lines, no markers) must NOT classify")
	}

	dialErr := `Get "http://crush/v1/workspaces": dial unix /run/tq/crush.sock: connect: no such file or directory`
	if !DetectCrushServerDeath("", dialErr) {
		t.Fatal("dial-unix error must classify as crush-server death")
	}

	failOnDrop := "crush: lost connection to the crush server before the run completed"
	if !DetectCrushServerDeath("", failOnDrop) {
		t.Fatal("v0.102.0 fail-on-drop error must classify as crush-server death")
	}

	cancelled := "crush: agent run was cancelled on the server before it completed"
	if !DetectCrushServerDeath("", cancelled) {
		t.Fatal("v0.102.0 server-side-cancel error must classify as crush-server death")
	}

	if DetectCrushServerDeath("boring output", "boring error") {
		t.Fatal("no markers, no classification")
	}
}

// TestRoutedSeveredRunClassifiesAsServerDeath pins the Execute wiring: a
// routed run whose output carries the severed-stream WARN returns the
// typed environmental death BEFORE any verify/verdict path — and the
// same output under a LOCAL run (no CrushServer) stays unclassified.
func TestRoutedSeveredRunClassifiesAsServerDeath(t *testing.T) {
	stub := makeStubAgent(t, `printf 'INFO  Created session for non-interactive run session_id=s1\n'
printf 'warn  Events stream ended error="unexpected EOF"\n'`)

	for _, tc := range []struct {
		name     string
		server   string
		wantDead bool
	}{
		{name: "routed severed stream is environmental death", server: "/tmp/crush-pool.sock", wantDead: true},
		{name: "local run never matches output markers", server: "", wantDead: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			if err := os.WriteFile(
				filepath.Join(repo, ".crushrc"),
				[]byte("permissions allow view\n"),
				0o644,
			); err != nil {
				t.Fatal(err)
			}

			e := &AgentExecutor{Bin: stub, CrushServer: tc.server}
			err := e.Execute(context.Background(), agentTaskT(t, AgentPayload{Repo: repo, Prompt: "do it"}))

			death, ok := errors.AsType[*CrushServerDeathError](err)
			if ok != tc.wantDead {
				t.Fatalf("classified = %v (err %v), want %v", ok, err, tc.wantDead)
			}

			if ok && !strings.Contains(death.Error(), "["+CrushServerDeathCode+"]") {
				t.Fatalf("death error must carry the %s marker: %s", CrushServerDeathCode, death.Error())
			}
		})
	}
}

// TestIsEnvironmentalDeathCrushServer pins the dlqfix refusal: a dead
// task whose last error carries the crush-server marker is environmental
// — an autopsy can only ever rule wontfix, so none may be minted.
func TestIsEnvironmentalDeathCrushServer(t *testing.T) {
	if !IsEnvironmentalDeath((&CrushServerDeathError{}).Error()) {
		t.Fatal("CrushServerDeathError text must classify as environmental")
	}
}
