//go:build unix

package executor

import (
	"context"
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestExecuteDerivesOutcomeFromRealRun (row 806 first slice): the
// derived-outcome contract e2e through AgentExecutor.Execute — a REAL
// stub-agent process makes a REAL footer commit in a REAL git repo, the
// verify gate runs for real, and the recorded Sink detail must carry the
// DERIVED facts (commits from the Task-Queue-ID footer via git
// diff-tree, files from the commit, verify tail) — no unit doubles on
// the derivation path.
func TestExecuteDerivesOutcomeFromRealRun(t *testing.T) {
	repo := t.TempDir()
	setupGitRepo(t, repo)

	// The stub extracts the task id from the substituted prompt (the id
	// is the 4th word of "ship the work <id>" — the verdict rail follows
	// on later lines, so word-split the prompt and take $4), commits one
	// file with the Task-Queue-ID footer as the LAST trailer — the exact
	// derived-outcome delivery contract.
	stub := makeStubAgent(t, `
tid=""
prev=""
for a in "$@"; do
	if [ "$prev" = "--" ]; then
		set -- $a
		tid="$4"
	fi
	prev="$a"
done
echo shipped > work.txt
git add -A
git commit -qm "work

Task-Queue-ID: $tid"
`)

	e := &AgentExecutor{Bin: stub}

	tk := agentTaskT(t, AgentPayload{
		Repo:   repo,
		Prompt: "ship the work {{TASK_ID}}",
		Verify: "test -f work.txt",
	})
	tk.ID = task.ID("000001a1e2e300000000000000000000000task")

	ctx, sink := NewSink(context.Background())

	if err := e.Execute(ctx, tk); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	detail := sink.Detail()
	if detail == nil {
		t.Fatal("no result detail recorded on the sink")
	}

	var result AgentResult
	if err := json.Unmarshal(detail, &result); err != nil {
		t.Fatalf("detail parse: %v", err)
	}

	if len(result.Commits) != 1 {
		t.Fatalf("derived commits = %d, want 1 (footer-based derivation): %+v", len(result.Commits), result)
	}

	if result.Commits[0].Subject == "" || !strings.Contains(result.Commits[0].Subject, "work") {
		t.Errorf("derived commit subject = %q", result.Commits[0].Subject)
	}

	if result.CommitSHA == "" {
		t.Error("CommitSHA empty — the newest derived commit should surface")
	}

	if len(result.FilesChanged) == 0 {
		t.Fatal("FilesChanged empty — git diff-tree derivation did not run")
	}

	found := false

	for _, f := range result.FilesChanged {
		if strings.Contains(f, "work.txt") {
			found = true
		}
	}

	if !found {
		t.Errorf("work.txt missing from derived files: %+v", result.FilesChanged)
	}

	if result.VerifyTail == "" {
		t.Error("VerifyTail empty — the real verify gate's output should ride the detail")
	}

	// The legacy stdout self-report is NOT the source anymore: the stub
	// prints no TQ_RESULT line, so every populated field above came from
	// derivation, not self-report.
	if result.SessionID != "" {
		t.Errorf("stub session id = %q, want empty (no session line in stub output)", result.SessionID)
	}
}
