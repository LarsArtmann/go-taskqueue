package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func TestSplitRepos(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want []string
	}{
		{"empty", "", nil},
		{"spaces only", "   ", nil},
		{"single", "a", []string{"a"}},
		{"two", "a,b", []string{"a", "b"}},
		{"spaces around entries", " a , b ", []string{"a", "b"}},
		{"trailing comma", "a,b,", []string{"a", "b"}},
		{"leading and trailing commas", ",a,,b,", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitRepos(tt.spec)
			if len(got) != len(tt.want) {
				t.Fatalf("splitRepos(%q) = %q, want %q", tt.spec, got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitRepos(%q)[%d] = %q, want %q", tt.spec, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPrintDriftReportGolden(t *testing.T) {
	item1 := harvest.Item{Repo: "/repos/alpha", RepoName: "alpha", Text: "fix the flaky worker test that races on drain", Key: "k1"}
	item2 := harvest.Item{Repo: "/repos/beta", RepoName: "beta", Text: "add a --json flag for parity with harvest", Key: "k2"}

	tests := []struct {
		name   string
		res    harvest.DriftResult
		dryRun bool
		want   string
	}{
		{
			name: "no drift",
			res:  harvest.DriftResult{Repos: 3},
			want: "(no drift)\naudit: 3 repos, 0 stale-open (0 catch-ups enqueued), 0 stale-done, 0 scan failures\n",
		},
		{
			name: "stale open enqueued",
			res: harvest.DriftResult{
				Repos:     2,
				StaleOpen: []harvest.Drift{{Kind: harvest.DriftStaleOpen, Item: item1, TaskID: "t1", TaskStatus: task.Completed}},
				Enqueued:  []harvest.Enqueued{{Item: item1, TaskID: "t9", Fresh: true}},
			},
			want: "DRIFT  alpha                    stale-open  task t1 completed, checkbox open: fix the flaky worker test that races on drain  [catch-up armed]\n" +
				"audit: 2 repos, 1 stale-open (1 catch-ups enqueued), 0 stale-done, 0 scan failures\n",
		},
		{
			name:   "stale open dry run",
			res:    harvest.DriftResult{Repos: 1, StaleOpen: []harvest.Drift{{Item: item1, TaskID: "t1", TaskStatus: task.Completed}}},
			dryRun: true,
			want: "DRIFT  alpha                    stale-open  task t1 completed, checkbox open: fix the flaky worker test that races on drain  [catch-up: dry-run, not enqueued]\n" +
				"audit: 1 repos, 1 stale-open (0 catch-ups enqueued), 0 stale-done, 0 scan failures\n",
		},
		{
			name: "stale done and scan failure",
			res: harvest.DriftResult{
				Repos:        2,
				StaleDone:    []harvest.Drift{{Item: item2, TaskID: "t2", TaskStatus: task.Pending}},
				ScanFailures: []harvest.ScanFailure{{Repo: "/repos/beta/TODO_LIST.md", Reason: "no such file"}},
			},
			want: "ERROR  TODO_LIST.md             scan failed: no such file\n" +
				"DRIFT  beta                     stale-done  task t2 is pending, checkbox ticked: add a --json flag for parity with harvest\n" +
				"audit: 2 repos, 0 stale-open (0 catch-ups enqueued), 1 stale-done, 1 scan failures\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captureStdout(t, func() { printDriftReport(tt.res, tt.dryRun) })
			if got != tt.want {
				t.Errorf("output mismatch\n--- got ---\n%s--- want ---\n%s", got, tt.want)
			}
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	os.Stdout = w

	fn()

	os.Stdout = old
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy: %v", err)
	}

	return buf.String()
}

// dispatchSubprocess re-runs the tq binary in-test for os.Exit paths.
func TestDispatchExitCodes(t *testing.T) {
	if os.Getenv("TQ_DISPATCH_SUBTEST") == "" {
		return
	}

	args := strings.Split(os.Getenv("TQ_DISPATCH_SUBTEST"), "\x1f")[1:]
	os.Args = append([]string{"tq"}, args...)
	main()
}

func runDispatch(t *testing.T, args []string) (exitCode int, stdout, stderr string) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestDispatchExitCodes")
	cmd.Env = append(os.Environ(), "TQ_DISPATCH_SUBTEST=sub\x1f"+strings.Join(args, "\x1f"))

	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err == nil {
		return 0, out.String(), errBuf.String()
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected exit error, got %v (stdout: %q)", err, out.String())
	}

	return exitErr.ExitCode(), out.String(), errBuf.String()
}

func TestDispatchUnknownCommandExits2(t *testing.T) {
	code, _, stderr := runDispatch(t, []string{"bogus-command"})
	if code != 2 {
		t.Errorf("unknown command exit code = %d, want 2", code)
	}

	if !strings.Contains(stderr, `unknown command "bogus-command"`) {
		t.Errorf("stderr missing unknown-command message: %q", stderr)
	}

	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr missing usage: %q", stderr)
	}
}

func TestDispatchHelpExits0(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		t.Run(arg, func(t *testing.T) {
			code, stdout, stderr := runDispatch(t, []string{arg})
			if code != 0 {
				t.Errorf("%s exit code = %d, want 0 (stderr: %q)", arg, code, stderr)
			}

			if !strings.Contains(stdout, "tq — projects-aware task work queue") {
				t.Errorf("%s stdout missing usage header: %q", arg, stdout)
			}
		})
	}
}

func TestDispatchNoArgsExits2(t *testing.T) {
	code, _, stderr := runDispatch(t, nil)
	if code != 2 {
		t.Errorf("no-args exit code = %d, want 2", code)
	}

	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr missing usage: %q", stderr)
	}
}
