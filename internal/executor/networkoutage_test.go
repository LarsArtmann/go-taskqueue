//go:build unix

package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestDetectNetworkOutage pins the connectivity-loss detector against the
// OBSERVED outage shapes (2026-11-17 facts and crush session records) and —
// equally important — the near-misses that must NOT park: provider quota
// text (rate-limit owns it), auth failures, and a LOCAL refused port (a
// dead llamacpp probe is not an internet outage; see networkOutageMarkers).
func TestDetectNetworkOutage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		out  string
		err  string
		want bool
	}{
		{
			name: "observed zai dns failure riding a bad-gateway finish",
			out: `INFO  Running in non-interactive mode
WARN  Provider request failed, retrying status_code=502 title="bad gateway" message="upstream unreachable: Post \"https://api.z.ai/api/coding/paas/v4/chat/completions\": dial tcp: lookup api.z.ai: Temporary failure in name resolution"`,
			want: true,
		},
		{
			name: "observed error-finish text on the error surface",
			err:  "retry error: bad gateway: upstream unreachable: dial tcp: lookup api.z.ai: Temporary failure in name resolution",
			want: true,
		},
		{name: "no such host", out: "Get \"https://api.example.com/v1\": dial tcp: lookup api.example.com: no such host", want: true},
		{name: "network unreachable", err: "dial tcp 1.2.3.4:443: connect: network is unreachable", want: true},
		{name: "no route to host", err: "dial tcp 1.2.3.4:443: connect: no route to host", want: true},
		{name: "marker casing varies", out: "WARN Temporary Failure In Name Resolution", want: true},
		{
			name: "429 quota is the rate-limit class, not an outage",
			out:  "ERROR rate limited: usage limit reached, renew At 2026-11-18 00:00:00",
			want: false,
		},
		{name: "auth failure", err: "401 unauthorized: invalid api key", want: false},
		{
			name: "local refused port is not an outage (llamacpp probe)",
			out:  `WARN Model discovery failed provider=llamacpp error="Get \"http://127.0.0.1:8899/v1/models\": dial tcp 127.0.0.1:8899: connect: connection refused"`,
			want: false,
		},
		{name: "plain failure", err: "exit status 1", want: false},
		{name: "empty", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := DetectNetworkOutage(tc.out, tc.err); got != tc.want {
				t.Errorf("DetectNetworkOutage(%q, %q) = %v, want %v", tc.out, tc.err, got, tc.want)
			}
		})
	}
}

// TestNetworkOutageErrorText pins the operator-facing message: the code
// token (journal/evidence greps key on it) and the park-not-burn contract.
func TestNetworkOutageErrorText(t *testing.T) {
	t.Parallel()

	cause := errors.New("dial tcp: lookup api.z.ai: Temporary failure in name resolution")
	err := &NetworkOutageError{Cause: cause}

	if !strings.Contains(err.Error(), "["+NetworkOutageCode+"]") {
		t.Errorf("error text %q lacks the [%s] code token", err.Error(), NetworkOutageCode)
	}

	if !errors.Is(err, cause) {
		t.Errorf("NetworkOutageError must unwrap to its cause")
	}
}

// TestNetworkOutageParkKeepsSession pins the outage contract END TO END on
// the agent executor (the 2026-11-17 spam shape): a run that dies on
// connectivity loss (a) classifies as *NetworkOutageError so the worker
// parks without attempt burn, and (b) leaves the failed run's session id
// in the failure sidecar trailer, so the NEXT claim re-sends the work into
// the SAME session (--session <id>) instead of minting a fresh one.
func TestNetworkOutageParkKeepsSession(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv("TQ_LOG_DIR", logDir)

	dir := t.TempDir()
	argsLog := filepath.Join(dir, "argv.log")
	bin := filepath.Join(dir, "dns-agent")

	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + argsLog + "\"\n" +
		"echo 'INFO  Created session for non-interactive run session_id=s_outage1'\n" +
		"echo 'WARN  Provider request failed: upstream unreachable: dial tcp: lookup api.z.ai: Temporary failure in name resolution'\n" +
		"exit 1\n"

	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".crushrc"), []byte("permissions allow view\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tk := agentTaskT(t, AgentPayload{Repo: repo, Prompt: "do it"})
	tk.ID = task.ID("t-outage")
	tk.Attempts = 0 // the parked shape: network failures never burn

	e := &AgentExecutor{Bin: bin, Yolo: true}

	err := e.Execute(context.Background(), tk)
	if _, ok := errors.AsType[*NetworkOutageError](err); !ok {
		t.Fatalf("Execute err = %v, want *NetworkOutageError (park, not burn)", err)
	}

	// The failure sidecar exists and carries the session trailer the
	// ladder reads on the next claim.
	body, rerr := os.ReadFile(filepath.Join(logDir, tk.ID.String()+".log"))
	if rerr != nil {
		t.Fatalf("failure sidecar not written: %v", rerr)
	}

	if got := ExtractSessionID(string(body)); got != "s_outage1" {
		t.Errorf("sidecar session = %q, want s_outage1 (trailer or Created line)", got)
	}

	if !strings.Contains(string(body), "session: s_outage1") {
		t.Errorf("sidecar lacks the explicit trailer line; body tail: %q", tailBytes(body, 200))
	}

	// The re-claim (the "retry message to the existing session"): the
	// ladder resolves the trailer and the run resumes s_outage1.
	if err := e.Execute(context.Background(), tk); err == nil {
		t.Fatal("stub still fails; second Execute must error too")
	}

	raw, rerr := os.ReadFile(argsLog)
	if rerr != nil {
		t.Fatal(rerr)
	}

	argv := strings.Fields(string(raw))
	for i, a := range argv {
		if a == "--session" && i+1 < len(argv) && argv[i+1] == "s_outage1" {
			return // pinned
		}
	}

	t.Errorf("retry argv lacks --session s_outage1: %q", tailBytes(raw, 400))
}

// TestFailureSidecarRedacts keeps the token-bearing-surface contract on the
// failure path: the output passes redaction BEFORE the session trailer is
// appended, and an unknown session degrades to no trailer (fresh retry,
// the pre-ladder behavior).
func TestFailureSidecarRedacts(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv("TQ_LOG_DIR", logDir)

	id := task.ID("t-redact")

	writeFailureSidecar(id, "output with sk-SECRETTOKEN0123456789 inside\n", "s_keep1")

	body, err := os.ReadFile(filepath.Join(logDir, id.String()+".log"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(body), "sk-SECRETTOKEN0123456789") {
		t.Errorf("failure sidecar leaked the token: %q", body)
	}

	if !strings.HasSuffix(strings.TrimRight(string(body), "\n"), "session: s_keep1") {
		t.Errorf("sidecar must end with the session trailer, got: %q", body)
	}

	// No session known: no trailer, output still persisted.
	writeFailureSidecar(task.ID("t-nosession"), "plain failure output\n", "")

	body, err = os.ReadFile(filepath.Join(logDir, "t-nosession.log"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(body), "session:") {
		t.Errorf("unknown session must not write a trailer: %q", body)
	}
}
