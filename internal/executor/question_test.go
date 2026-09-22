//go:build unix

package executor

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
)

func TestQuestionPendingFromMarker(t *testing.T) {
	expiry := time.Now().Add(time.Hour)

	marker := func(ref, question string, expiresMillis int64) string {
		return fmt.Sprintf(
			`{"ref":%q,"type":"confirmation","question":%q,"expires_at":%d}`,
			ref,
			question,
			expiresMillis,
		)
	}

	for _, tc := range []struct {
		name      string
		body      string
		wantNil   bool
		wantPerm  bool
		wantWait  time.Duration
		wantSlave bool
	}{
		{name: "valid marker waits until expiry", body: marker("q-1", "Ship v3?", expiry.UnixMilli()), wantWait: time.Until(expiry)},
		{name: "expired marker clamps to the minimum wait", body: marker("q-1", "Ship v3?", time.Now().Add(-time.Hour).UnixMilli()), wantWait: minQuestionWait},
		{name: "no expiry clamps to the minimum wait", body: marker("q-1", "Ship v3?", 0), wantWait: minQuestionWait},
		{name: "absent marker is no question", body: "", wantNil: true},
		{name: "corrupt marker is permanent", body: "{not json", wantPerm: true},
		{name: "marker without ref is permanent", body: marker("", "Ship v3?", expiry.UnixMilli()), wantPerm: true},
		{name: "marker without question is permanent", body: marker("q-1", "", expiry.UnixMilli()), wantPerm: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := ""

			if tc.body != "" {
				f, err := os.CreateTemp(t.TempDir(), "marker-*.json")
				if err != nil {
					t.Fatal(err)
				}

				path = f.Name()

				if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			err := questionPendingFrom(path, time.Now())

			switch {
			case tc.wantNil:
				if err != nil {
					t.Fatalf("want no question, got %v", err)
				}
			case tc.wantPerm:
				if _, ok := errors.AsType[*PermanentError](err); err == nil || !ok {
					t.Fatalf("want permanent error, got %v", err)
				}
			default:
				qp, ok := errors.AsType[*QuestionPendingError](err)
				if !ok {
					t.Fatalf("want *QuestionPendingError, got %v", err)
				}

				if qp.RetryAfter < tc.wantWait-time.Minute || qp.RetryAfter > tc.wantWait+time.Minute {
					t.Errorf("RetryAfter = %s, want ~%s", qp.RetryAfter, tc.wantWait)
				}

				if !strings.Contains(qp.Error(), "Ship v3?") {
					t.Errorf("error text lost the question: %s", qp.Error())
				}
			}
		})
	}

	t.Run("missing file path is no question", func(t *testing.T) {
		if err := questionPendingFrom(filepath.Join(t.TempDir(), "absent.json"), time.Now()); err != nil {
			t.Fatalf("absent marker file must mean no question, got %v", err)
		}
	})

	t.Run("double wrap stays a single class", func(t *testing.T) {
		inner := QuestionPending(errors.New("asked"), time.Minute)
		if QuestionPending(inner, time.Minute) != inner {
			t.Fatal("re-wrapping a *QuestionPendingError must return it unchanged")
		}

		if QuestionPending(nil, time.Minute) != nil {
			t.Fatal("nil cause must pass through")
		}
	})
}

func TestRenderAnsweredRulings(t *testing.T) {
	if got := renderAnswered("do the thing", nil); got != "do the thing" {
		t.Fatalf("no answers must leave the prompt untouched, got %q", got)
	}

	got := renderAnswered("do the thing", []queue.QuestionAnsweredDetail{
		{Ref: "q-1", Question: "Ship v3 now?", Answer: "Stay on v2."},
	})

	for _, want := range []string{"do the thing", "Answers from the owner", "Q: Ship v3 now?", "A: Stay on v2."} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered prompt missing %q:\n%s", want, got)
		}
	}
}

// TestAgentExecutorQuestionParksTask runs the full park path: a stub agent
// writes the question marker through $TQ_QUESTION_FILE and exits cleanly;
// the executor must return *QuestionPendingError and skip verify entirely.
func TestAgentExecutorQuestionParksTask(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	setupGitRepo(t, repo)

	expiry := time.Now().Add(2 * time.Hour).UnixMilli()

	body := fmt.Sprintf(
		`printf '%%s' '{"ref":"q-9","type":"confirmation","question":"Which module owns the cursor?","expires_at":%d}' > "$TQ_QUESTION_FILE"`,
		expiry,
	)

	e := &AgentExecutor{Bin: makeStubAgent(t, body)}

	err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "do the thing", Verify: "false"}))
	if err == nil {
		t.Fatal("want question park, got nil")
	}

	qp, ok := errors.AsType[*QuestionPendingError](err)
	if !ok {
		t.Fatalf("want *QuestionPendingError, got %v", err)
	}

	if qp.ResumeCloseout {
		t.Error("work-turn question must not resume closeout")
	}

	if qp.RetryAfter <= 0 || qp.RetryAfter > 2*time.Hour {
		t.Errorf("RetryAfter = %s, want until the question expiry", qp.RetryAfter)
	}
}

// TestQuestionChannelScopePinsSecondOpinions pins the channel scope
// (21-04 §f42): literally-constructed work executors carry
// $TQ_QUESTION_FILE (the park path IS the feature), while the
// second-opinion clones built via WithoutCloseout (review / status /
// dlqfix / prioritize) never do — a reviewer or scorer must not park a
// task on an owner question.
func TestQuestionChannelScopePinsSecondOpinions(t *testing.T) {
	ctx := context.Background()
	probe := `if [ -n "$TQ_QUESTION_FILE" ]; then echo with-channel >> env-probe.txt; else echo without-channel >> env-probe.txt; fi`

	workRepo := t.TempDir()
	setupGitRepo(t, workRepo)

	work := &AgentExecutor{Bin: makeStubAgent(t, probe)}
	if err := work.Execute(ctx, agentTaskT(t, AgentPayload{Repo: workRepo, Prompt: "do the thing"})); err != nil {
		t.Fatalf("work run: %v", err)
	}

	workProbe, err := os.ReadFile(filepath.Join(workRepo, "env-probe.txt"))
	if err != nil {
		t.Fatalf("work run never wrote the probe: %v", err)
	}
	if probeOut := string(workProbe); strings.Contains(probeOut, "without-channel") || !strings.Contains(probeOut, "with-channel") {
		t.Errorf("work executor question channel state wrong: %s", probeOut)
	}

	cloneRepo := t.TempDir()
	setupGitRepo(t, cloneRepo)

	clone := work.WithoutCloseout()
	if err := clone.Execute(ctx, agentTaskT(t, AgentPayload{Repo: cloneRepo, Prompt: "review the thing"})); err != nil {
		t.Fatalf("clone run: %v", err)
	}

	cloneProbe, err := os.ReadFile(filepath.Join(cloneRepo, "env-probe.txt"))
	if err != nil {
		t.Fatalf("clone run never wrote the probe: %v", err)
	}
	if probeOut := string(cloneProbe); strings.Contains(probeOut, "with-channel") || !strings.Contains(probeOut, "without-channel") {
		t.Errorf("second-opinion clone must never receive %s: %s", questionFileEnv, probeOut)
	}
}

// TestAgentPromptRendersAnsweredRulings pins the resume contract: a payload
// carrying injected answers renders them into the prompt the agent sees.
func TestAgentPromptRendersAnsweredRulings(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	setupGitRepo(t, repo)

	argsLog := filepath.Join(t.TempDir(), "args")

	e := &AgentExecutor{Bin: makeStubAgent(t, `printf '%s\n' "$@" > `+argsLog)}

	payload := AgentPayload{
		Repo:   repo,
		Prompt: "do the thing",
		Answered: []queue.QuestionAnsweredDetail{
			{Ref: "q-1", Question: "Ship v3 now?", Answer: "Stay on v2."},
		},
	}

	if err := e.Execute(ctx, agentTaskT(t, payload)); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("stub args log: %v", err)
	}

	for _, want := range []string{"Answers from the owner", "Q: Ship v3 now?", "A: Stay on v2."} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("prompt missing %q:\n%s", want, raw)
		}
	}

	if strings.Contains(string(raw), "{{TASK_ID}}") {
		t.Error("prompt placeholder unresolved")
	}
}

// TestAgentPayloadAnsweredRoundTrip pins the wire contract of the injected
// answers: snake_case keys, decoded back into the payload by the store's
// mergeAnsweredPayload shape.
func TestAgentPayloadAnsweredRoundTrip(t *testing.T) {
	p := AgentPayload{
		Repo:   "r",
		Prompt: "p",
		Answered: []queue.QuestionAnsweredDetail{
			{Ref: "q-1", Question: "q?", Answer: "a", PapID: "pap-1", AnsweredAt: 1726582800000},
		},
	}

	rendered, err := RenderAgentPayload(p)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(rendered.String(), `"answered"`) {
		t.Fatalf("rendered payload lost the answered array: %s", rendered)
	}

	var back AgentPayload
	if err := json.Unmarshal(rendered, &back); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(back.Answered) != 1 || back.Answered[0].Ref != "q-1" || back.Answered[0].Answer != "a" {
		t.Fatalf("round trip lost answers: %+v", back.Answered)
	}
}
