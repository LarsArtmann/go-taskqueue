package executor

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/larsartmann/go-retry"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TaskTypeAgent runs a headless AI coding agent (crush) in a repository.
// Register it under this type to make the queue agent-capable.
const TaskTypeAgent = "agent"

// AgentPayload is the payload contract for "agent" tasks: run a headless AI
// coding agent in a repository, then prove the result.
type AgentPayload struct {
	// Repo is the repository the agent works in: a name resolved against the
	// executor's ProjectsDir, or an absolute path. Required.
	Repo string `json:"repo"`
	// Prompt is the full instruction for the agent run. Required.
	Prompt string `json:"prompt"`
	// Model optionally overrides the crush model ("provider/model").
	Model string `json:"model,omitempty"`
	// V is the payload contract version. Zero decodes
	// as v1; a version above what this binary understands fails fast as a
	// permanent error instead of misparsing newer fields.
	V int `json:"v,omitempty"`
	// Session optionally continues a previous crush session by ID.
	Session string `json:"session,omitempty"`
	// Dedup is the harvester's item key; purely informational, used to keep
	// TODO items and tasks 1:1 across harvest runs.
	Dedup string `json:"dedup,omitempty"`
	// Item is the raw TODO_LIST work item the prompt was rendered from;
	// purely informational. Status windows pin it so report excerpts show
	// the real work instead of the prompt template's first line. Empty for
	// tasks minted outside the harvester.
	Item string `json:"item,omitempty"`
	// Verify is a shell command that must exit 0 after the agent run for the
	// task to complete — the enforced quality gate. Empty means auto-detect:
	// Go repositories (go.mod present) run "go build ./... && go test ./...
	// -count=1", everything else runs nothing.
	Verify string `json:"verify,omitempty"`
	// RequireClean refuses to start unless the repo's git tree is clean, so
	// the pool never tramples human work-in-progress. Default true. Repos
	// without a .git directory skip the check (nothing to protect).
	RequireClean *bool `json:"require_clean,omitempty"`
	// TimeoutMinutes caps the whole task (agent run + verify). Default 30.
	// The worker's task timeout still applies as a hard ceiling above this.
	TimeoutMinutes int `json:"timeout_minutes,omitempty"`
	// Yolo marks this task's agent as autonomous. crush run has NO yolo
	// flag (verified against crush v0.92: "Unknown flag: --yolo"); autonomy
	// comes from the repo's own project-local crush config granting
	// permissions (`.crushrc`: `permissions allow view ls grep edit write bash`).
	// This field makes the executor fail fast with remediation guidance when
	// autonomy is requested but the repo has no such config, instead of
	// burning agent attempts on runs that stall on permission prompts.
	Yolo bool `json:"yolo,omitempty"`
}

// DefaultAgentBinary is used when Bin, $TQ_AGENT_BIN and $TQ_CRUSH_BIN are
// all empty.
const DefaultAgentBinary = "crush"

// defaultAgentTaskTimeout bounds one agent task unless the payload overrides.
const defaultAgentTaskTimeout = 30 * time.Minute

// AgentExecutor runs one headless AI coding agent per task, then enforces the
// payload's verify contract. It must be safe for concurrent use; the worker
// pool executes several tasks in parallel.
type AgentExecutor struct {
	// Bin is the agent executable. Default: $TQ_AGENT_BIN, $TQ_CRUSH_BIN or
	// "crush". Point it at a stub in tests.
	Bin string
	// ProjectsDir resolves relative Repo names in payloads ("demo" →
	// <ProjectsDir>/demo). Absolute Repo paths bypass it.
	ProjectsDir string
	// Yolo is the operator-level autonomy request (pool start). crush run
	// has no yolo flag; see AgentPayload.Yolo for how autonomy is actually
	// granted (repo-local crush config). Fail closed, never silently.
	Yolo bool
	// MaxConcurrent caps how many agent processes run at once MACHINE-WIDE
	// (flock'd slot files shared across every tq process on the host; 0 =
	// uncapped). Caps cost when several pools share a machine.
	MaxConcurrent int
	// CloseoutPrompt, when set, gives every agent task a SECOND conversation
	// turn: after the work turn ends, the same session receives this prompt
	// (the owner's brutal self-review + status report) before verify runs.
	// Empty = off (single turn, pinned argv contract unchanged).
	CloseoutPrompt string

	// rateLimitUntil is the UnixNano instant the provider is next expected
	// to accept requests (0 = clear). Set when a run's output reports
	// provider exhaustion (Z.ai 5-hour usage windows, synthetic.new quota
	// 429s); while it holds, runAgent fast-refuses without spawning the
	// agent binary. In-process by design: one pool per machine is the
	// deployment norm (tq-agent-pool), and each pool that does probe pays
	// one cheap refused run to re-learn the window. Gate logic lives in
	// ratelimit.go.
	rateLimitUntil atomic.Int64

	// rateLimitGates holds PER-REPO gates (repoDir → *atomic.Int64):
	// which provider a repo's agents use is fixed by that repo's .crushrc,
	// so the repo is the isolation key — a Z.ai 429 in repo A must not
	// park repo B's synthetic.new tasks in the same pool (13:29 report
	// f4b/g3). Repo-less evidence (no payload repo) falls back to the
	// shared gate above.
	rateLimitGates sync.Map

	// closeoutPending holds finished WORK turns whose close-out is owed
	// (rate-limited close-out → requeue without attempt burn; the
	// re-claim resumes at closeout instead of re-running the paid work
	// turn). task.ID → closeoutPending; see runAgent/runCloseoutTurn.
	closeoutPending sync.Map
}

// NewAgentExecutor builds an AgentExecutor for a projects directory.
func NewAgentExecutor(projectsDir string) *AgentExecutor {
	return &AgentExecutor{ProjectsDir: projectsDir}
}

// WithoutCloseout returns a clone with CloseoutPrompt cleared, for the
// review and status executors (they ARE the second opinion; giving them
// the work turn's self-review would double agent cost for no new signal).
// The clone carries every runtime setting but starts with a FRESH
// rate-limit gate: gates are per-executor-instance (re-arming from fresh
// provider evidence is one cheap refused run), and an armed
// atomic.Int64 must never be struct-copied (copylocks).
func (e *AgentExecutor) WithoutCloseout() *AgentExecutor {
	return &AgentExecutor{
		Bin:           e.Bin,
		ProjectsDir:   e.ProjectsDir,
		Yolo:          e.Yolo,
		MaxConcurrent: e.MaxConcurrent,
	}
}

func (e *AgentExecutor) binary() string {
	if e.Bin != "" {
		return e.Bin
	}

	if b := os.Getenv("TQ_AGENT_BIN"); b != "" {
		return b
	}

	if b := os.Getenv("TQ_CRUSH_BIN"); b != "" {
		return b
	}

	return DefaultAgentBinary
}

// AgentVersion probes the agent binary's --version line (first stdout
// line, trimmed). Pools call it at startup so a missing/stale binary is a
// warning, not a surprise mid-task.
func AgentVersion(ctx context.Context, bin string) (string, error) {
	if bin == "" {
		bin = DefaultAgentBinary
	}

	if _, err := exec.LookPath(bin); err != nil {
		return "", fmt.Errorf("%q not found on PATH: %w", bin, err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out, err := execWithTransientRetry(func() ([]byte, error) {
		return exec.CommandContext(ctx, bin, "--version").Output()
	})
	if err != nil {
		return "", fmt.Errorf("%s --version: %w", bin, err)
	}

	version := strings.TrimSpace(string(out))
	if i := strings.IndexByte(version, '\n'); i >= 0 {
		version = version[:i]
	}

	return version, nil
}

// Execute guards the repo, runs the agent, then runs the verify command.
// Any miss is a failed attempt (the queue retries with backoff, then
// dead-letters). Input-contract misses (payload, repo, dirty tree, autonomy)
// are permanent: the identical retry would fail identically, and for agent
// tasks every retry is real money.
func (e *AgentExecutor) Execute(ctx context.Context, t task.Task) error {
	var p AgentPayload

	if len(t.Payload) == 0 {
		return Permanent(errors.New("agent: empty payload, want {repo, prompt}"))
	}

	if err := json.Unmarshal(t.Payload, &p); err != nil {
		return Permanent(fmt.Errorf("agent: decode payload: %w", err))
	}

	if p.V == 0 {
		p.V = 1 // payloads minted before versioning are contract v1
	}

	if p.V > 1 {
		return Permanent(fmt.Errorf("agent: payload version %d unknown (this binary understands v1)", p.V))
	}

	if p.Repo == "" || p.Prompt == "" {
		return Permanent(errors.New("agent: payload needs non-empty repo and prompt"))
	}

	repoDir, err := e.repoDir(p.Repo)
	if err != nil {
		return Permanent(err)
	}

	// Machine-wide agent cap: block until a slot frees up. Acquired before
	// the dirty-tree preflight so a waiting task does not hold a slot (and
	// a crashed process releases its flock via the kernel).
	releaseSlot, err := acquireAgentSlot(ctx, e.MaxConcurrent)
	if err != nil {
		return fmt.Errorf("agent: slot: %w", err)
	}

	defer releaseSlot()

	if requireClean(p) {
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
			if err := assertCleanTree(ctx, repoDir); err != nil {
				// Preflight, not permanent: the human will commit eventually;
				// the worker requeues without burning an attempt.
				return &PreflightError{Cause: err}
			}
		}
	}

	timeout := defaultAgentTaskTimeout
	if p.TimeoutMinutes > 0 {
		timeout = time.Duration(p.TimeoutMinutes) * time.Minute
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := e.runAgent(runCtx, repoDir, &p, t.ID)
	if err != nil {
		// Forensics for the task.failed fact: exit code + output tail. The
		// full output survives in the sidecar only when TQ_LOG_DIR is set,
		// so the fact carries its own excerpt.
		SetFailureEvidence(ctx, "agent", err, tailBytes([]byte(output), EvidenceTailBytes))

		return err
	}

	tail, err := runVerify(runCtx, repoDir, &p)
	if err != nil {
		SetFailureEvidence(ctx, "verify", err, tail)

		return err
	}
	// Success: record structured outcome detail for `tq show` (best
	// effort — a missing session id or self-report is not an error).
	result := AgentResult{
		SessionID:  ExtractSessionID(output),
		VerifyTail: tail,
	}
	if files, sha, ok := ExtractResultPayload(output); ok {
		result.FilesChanged, result.CommitSHA = files, sha
	}

	result.LogPath = writeOutputSidecar(t.ID, output, tail)
	detail, _ := json.Marshal(result)
	SetResultDetail(ctx, detail)

	return nil
}

// writeOutputSidecar persists the FULL agent + verify output to
// $TQ_LOG_DIR/<task-id>.log and returns the path — "" when the directory is
// unset or the write fails (logging must never fail a completed task).
func writeOutputSidecar(id task.ID, agentOutput, verifyOutput string) string {
	dir := os.Getenv("TQ_LOG_DIR")
	if dir == "" {
		return ""
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}

	path := filepath.Join(dir, id.String()+".log")

	body := agentOutput
	if verifyOutput != "" {
		body += "\n--- verify ---\n" + verifyOutput
	}

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return ""
	}

	return path
}

// repoDir resolves a payload repo name: absolute paths pass through,
// relative names resolve against ProjectsDir.
func (e *AgentExecutor) repoDir(repo string) (string, error) {
	if filepath.IsAbs(repo) {
		if info, err := os.Stat(repo); err != nil || !info.IsDir() {
			return "", fmt.Errorf("agent: repo directory does not exist: %s", repo)
		}

		return repo, nil
	}

	if e.ProjectsDir == "" {
		return "", fmt.Errorf("agent: relative repo %q needs a projects dir on the executor", repo)
	}

	dir := filepath.Join(e.ProjectsDir, repo)

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("agent: repo %q does not exist under the projects dir", repo)
	}

	return dir, nil
}

func requireClean(p AgentPayload) bool {
	if p.RequireClean == nil {
		return true
	}

	return *p.RequireClean
}

// assertCleanTree fails unless the repo has no uncommitted changes.
func assertCleanTree(ctx context.Context, repo string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "status", "--porcelain")

	var out bytes.Buffer

	cmd.Stdout = &out

	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("agent: git status failed in %s: %w: %s", repo, err, tailBytes(out.Bytes(), 512))
	}

	if s := strings.TrimSpace(out.String()); s != "" {
		return fmt.Errorf(
			"agent: repo %s has uncommitted changes; refusing to run agent (commit/stash first, or set require_clean=false): %s",
			repo,
			tailBytes(out.Bytes(), 512),
		)
	}

	return nil
}

// runAgent spawns the headless agent in the repo and waits for it.
func (e *AgentExecutor) runAgent(ctx context.Context, repoDir string, p *AgentPayload, id task.ID) (string, error) {
	// Closeout resume (13:29 report f15): a prior attempt finished the WORK
	// turn but was rate-limited during the close-out; re-running the work
	// turn on re-claim would double real agent cost. Resume at closeout
	// instead. In-process only (an executor restart forgets it — the
	// fallback is the old full re-run, never a lost close-out).
	if v, ok := e.closeoutPending.Load(id.String()); ok {
		pending := v.(closeoutPending)
		if pending.repoDir == repoDir {
			e.closeoutPending.Delete(id.String())

			buf := &bytes.Buffer{}
			if err := e.runCloseoutTurn(ctx, repoDir, pending.session, id, buf); err != nil {
				return buf.String(), err
			}

			return buf.String(), nil
		}

		e.closeoutPending.Delete(id.String())
	}

	// Provider gate: a sibling task just observed the provider refusing
	// (429/usage limit). Refuse BEFORE spawning crush — a probe into a
	// spent quota costs a process spawn, agent-side retry noise, and on
	// metered providers potentially billed tokens. The returned class
	// carries the wait, so the worker requeues until the reset without
	// burning an attempt.
	if wait, limited := e.rateLimitWaitRepo(repoDir); limited {
		return "", RateLimited(
			errors.New("agent: provider rate limit in effect (observed by a sibling run); deferring until reset"),
			wait,
		)
	}

	// The queue task ID is only known at execution time (the harvester
	// renders prompts before enqueue), so the {{TASK_ID}} placeholder in
	// prompt contracts resolves HERE — it lets agents put `Task-Queue-ID:
	// <id>` footers in their commits so git log ↔ tq facts cross-reference
	// (21:40 report §e3).
	p.Prompt = strings.ReplaceAll(p.Prompt, "{{TASK_ID}}", id.String())

	if e.Yolo || p.Yolo {
		if err := requireRepoAutonomy(repoDir); err != nil {
			return "", err
		}
	}

	// The close-out turn resumes the exact session by id, so the work turn
	// runs verbose when enabled: quiet mode suppresses the "Created session
	// … session_id=" line ExtractSessionID needs, and --continue would race
	// for the most recent session when another pool agent finishes at the
	// same moment.
	runVerb := "--quiet"
	if e.CloseoutPrompt != "" {
		runVerb = "--verbose"
	}

	args := []string{"run", runVerb, "--cwd", repoDir}
	if p.Model != "" {
		args = append(args, "--model", p.Model)
	}

	if p.Session != "" {
		args = append(args, "--session", p.Session)
	}

	args = append(args, "--", p.Prompt)

	runOnce := func() (*bytes.Buffer, error) {
		cmd := exec.CommandContext(ctx, e.binary(), args...)
		cmd.Dir = repoDir

		var buf bytes.Buffer

		cmd.Stdout = &buf
		cmd.Stderr = &buf
		// Kill the whole process tree on cancel (agents spawn children) and do
		// not hang the worker if grandchildren hold the pipes open.
		prepareProcessGroup(cmd)

		cmd.WaitDelay = 10 * time.Second
		err := cmd.Run()

		return &buf, err
	}

	buf, err := execWithTransientRetry(runOnce)
	if err != nil {
		// The captured output survives the error so the caller can pin the
		// failure evidence's tail excerpt. The cancelled branch must keep
		// wrapping ctx.Err(): the worker finalizes cooperative cancels by
		// matching context.Canceled.
		if ctx.Err() != nil {
			return buf.String(), fmt.Errorf("agent run cancelled (%w): %s", ctx.Err(), tailBytes(buf.Bytes(), 8192))
		}

		// Provider exhaustion (Z.ai 429 usage-limit windows, OpenAI-style
		// quota refusals from synthetic.new and friends): return the
		// requeue-able class instead of a plain failure, so the worker
		// parks the task until the provider resets WITHOUT burning an
		// attempt. Detection also arms the gate for sibling runs.
		if rl := e.rateLimitedTurn("agent run", repoDir, err, buf.String()); rl != nil {
			return buf.String(), rl
		}

		return buf.String(), fmt.Errorf("agent run failed: %w: %s", err, tailBytes(buf.Bytes(), 8192))
	}

	// Second turn, same conversation: the agent that did the work answers
	// the close-out self-review before the task completes. A failed closeout
	// fails the attempt like any other contract breach; a missing session id
	// degrades to a logged skip (the work itself already succeeded).
	if e.CloseoutPrompt != "" {
		if session := ExtractSessionID(buf.String()); session != "" {
			if err := e.runCloseoutTurn(ctx, repoDir, session, id, buf); err != nil {
				return buf.String(), err
			}
		} else {
			buf.WriteString("\n[tq] closeout skipped: no session id in agent output\n")
		}
	}

	return buf.String(), nil
}

// closeoutPending records a finished WORK turn whose close-out is owed: a
// rate-limited close-out requeues the task WITHOUT burning an attempt, and
// the re-claim must resume here instead of re-running the paid work turn.
type closeoutPending struct {
	repoDir string
	session string
}

// runCloseoutTurn runs the second conversation turn (the close-out
// self-review) in the session the work turn opened, appending its output to
// buf. Rate-limited close-outs register closeoutPending BEFORE returning
// the *RateLimitError so the re-claim resumes here.
func (e *AgentExecutor) runCloseoutTurn(ctx context.Context, repoDir, session string, id task.ID, buf *bytes.Buffer) error {
	closeout := strings.ReplaceAll(e.CloseoutPrompt, "{{TASK_ID}}", id.String())
	closeoutArgs := []string{"run", "--quiet", "--cwd", repoDir, "--session", session, "--", closeout}

	closeoutOnce := func() (*bytes.Buffer, error) {
		cmd := exec.CommandContext(ctx, e.binary(), closeoutArgs...)
		cmd.Dir = repoDir

		var closeoutBuf bytes.Buffer

		cmd.Stdout = &closeoutBuf
		cmd.Stderr = &closeoutBuf
		prepareProcessGroup(cmd)
		cmd.WaitDelay = 10 * time.Second
		err := cmd.Run()

		return &closeoutBuf, err
	}

	closeoutBuf, err := execWithTransientRetry(closeoutOnce)
	buf.WriteString(closeoutBuf.String())

	if err == nil {
		return nil
	}

	if ctx.Err() != nil {
		return fmt.Errorf(
			"agent closeout cancelled (%w): %s",
			ctx.Err(),
			tailBytes(buf.Bytes(), 8192),
		)
	}

	if rl := e.rateLimitedTurn("agent closeout", repoDir, err, buf.String()); rl != nil {
		// The work turn SUCCEEDED and its session is alive: on re-claim,
		// resume at closeout instead of paying for the work turn twice
		// (13:29 report f15).
		e.closeoutPending.Store(id.String(), closeoutPending{repoDir: repoDir, session: session})

		return rl
	}

	return fmt.Errorf("agent closeout failed: %w: %s", err, tailBytes(buf.Bytes(), 8192))
}

// DefaultCloseoutPrompt is the second conversation turn every agent task
// runs when the pool enables --task-closeout: the same brutal self-review
// the owner uses interactively, answered by the agent that did the work,
// in the same session. {{TASK_ID}} resolves at execution time (report path
// + commit footer); the TQ_RESULT re-emit keeps the queue's mechanical
// gate green, since parsing reads the LAST TQ_RESULT line in the output.
const DefaultCloseoutPrompt = `What did you forget? What could you have done better? What could you still improve?

FULL COMPREHENSIVE & DETAILED STATUS UPDATE!
INCLUDE WORK: a) FULLY DONE; b) PARTIALLY DONE; c) NOT STARTED; d) TOTALLY FUCKED UP! e) WHAT WE SHOULD IMPROVE! f) Up to 50 things we should get done next! g) Ask up to 3 questions that you can NOT figure out yourself!

Run "date" (CLI) to get the current date-time, then write the full report at docs/status/<YYYY-MM-DD_HH-MM>_task-{{TASK_ID}}.md. Commit it with the same Task-Queue-ID footer as your work commit. Never push.

DO NOT RESEARCH UNRELATED STUFF. Report based on THIS task's work and what you noticed in passing.

End your final output with EXACTLY ONE line and nothing after it: the same TQ_RESULT line you reported for the work above (the queue's mechanical gate reads the last one).`

// execWithTransientRetry retries exec attempts that failed with ETXTBSY
// ("text file busy"). Kernel 7.2 was observed returning it for freshly
// written stub executables under concurrent process churn with no writer
// holding the file — an executor-level retry (50ms, then 100ms) absorbs
// the whole failure class instead of failing a task attempt. Every other
// error passes through untouched.
func execWithTransientRetry[T any](run func() (T, error)) (T, error) {
	var out T

	err := retry.Do(context.Background(), retry.Config{ //nolint:exhaustruct // optional hooks unset
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		IsRetryable:  func(err error) bool { return errors.Is(err, syscall.ETXTBSY) },
	}, func(_ context.Context, _ int) error {
		var attemptErr error

		out, attemptErr = run()

		return attemptErr
	})

	return out, err
}

// runVerify enforces the quality gate after the agent exited cleanly. The
// repo's .tq-verify file wins over everything (the repo is the source of
// truth for how it proves itself), then the payload, then auto-detect.
func runVerify(ctx context.Context, repoDir string, p *AgentPayload) (string, error) {
	verify := verifyFor(repoDir, p)
	if verify == "" {
		return "", nil // nothing to verify (unknown stack, no explicit command)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", verify)
	cmd.Dir = repoDir

	var buf bytes.Buffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf
	prepareProcessGroup(cmd)

	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Run(); err != nil {
		// The tail rides along even on error: it IS the failure evidence
		// (what the gate printed before dying).
		tail := tailBytes(buf.Bytes(), EvidenceTailBytes)
		if ctx.Err() != nil {
			return tail, fmt.Errorf("agent verify cancelled (%w): %s", ctx.Err(), tail)
		}

		return tail, fmt.Errorf("agent verify failed (%q): %w: %s", verify, err, tail)
	}

	return tailBytes(buf.Bytes(), 2048), nil
}

// userGlobalCrushConfig reports whether a user-global crush config exists:
// permissions can be granted globally, so a repo-local config is not
// strictly required. Overridable in tests.
var userGlobalCrushConfig = func() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	for _, p := range []string{
		filepath.Join(home, ".config", "crush", "crush.json"),
		filepath.Join(home, ".crush.json"),
	} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}

	return false
}

// requireRepoAutonomy refuses when an autonomous run is requested but no
// crush config could grant permissions — neither repo-local nor user-global.
// Without this check an unattended pool burns its attempt budget on runs
// that stall or die on permission prompts (crush run has no --yolo flag).
// Preflight class: the operator can add a config and the task retries
// without an attempt burn.
func requireRepoAutonomy(repoDir string) error {
	for _, name := range []string{".crushrc", "crushrc", ".crush.json", "crush.json"} {
		if _, err := os.Stat(filepath.Join(repoDir, name)); err == nil {
			return nil
		}
	}

	if userGlobalCrushConfig() {
		return nil
	}

	return &PreflightError{
		Cause: fmt.Errorf(
			"agent: autonomy requested but %s has no project-local crush config and no user-global crush config exists; add a .crushrc with 'permissions allow view ls grep edit write bash' (or unset yolo)",
			repoDir,
		),
	}
}

// verifyFor resolves the verify command: .tq-verify file in the repo, then
// the payload's explicit verify, then auto-detection from the repo layout.
func verifyFor(repoDir string, p *AgentPayload) string {
	if v := readTQVerify(repoDir); v != "" {
		return v
	}

	if p.Verify != "" {
		return p.Verify
	}

	return autoDetectVerify(repoDir)
}

// readTQVerify returns the trimmed contents of <repoDir>/.tq-verify, or "".
func readTQVerify(repoDir string) string {
	b, err := os.ReadFile(filepath.Join(repoDir, ".tq-verify"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(b))
}

// ReadTQVerify is the exported form of readTQVerify for tools that compose
// payloads (the harvester pins the repo's verify contract into tasks).
func ReadTQVerify(repoDir string) string { return readTQVerify(repoDir) }

// goEnvPrelude makes a minted Go verify command env-self-contained: the
// tq-agent-pool unit (and any bare shell) carries no GOEXPERIMENT, and repos
// importing encoding/json/v2 then die with "build constraints exclude all Go
// files" — an environment lie that judges finished work on a broken gate.
// Minted Go verifies carry the export so the gate is identical inside and
// outside the flake devShell. Non-Go stacks are untouched; a repo pinning an
// older toolchain that rejects the experiment pins its own .tq-verify (the
// file is the source of truth and is never auto-rewritten behind an
// existing value).
const goEnvPrelude = "export GOEXPERIMENT=jsonv2; "

// withGoEnvPrelude prefixes a minted verify command with goEnvPrelude unless
// the command already manages GOEXPERIMENT itself (idempotent).
func withGoEnvPrelude(verify string) string {
	if strings.Contains(verify, "GOEXPERIMENT") {
		return verify
	}

	return goEnvPrelude + verify
}

// defaultVerify picks a sensible verification command for a repo.
func defaultVerify(repo string) string {
	if _, err := os.Stat(filepath.Join(repo, "go.mod")); err == nil {
		// Root ./... never descends into nested modules, so a multi-module
		// repo would verify vacuously; walk every go.mod below the root.
		// A plain find -execdir would swallow the inner exit status (find
		// reports only its own errors), so the loop propagates failure with
		// an explicit exit. Word-split find output: module paths containing
		// spaces are rare enough for a heuristic default; a repo can pin its
		// own .tq-verify when it needs more.
		return withGoEnvPrelude("go build ./... && go test ./... -count=1" +
			" && for f in $(find . -mindepth 2 -name go.mod -not -path '*/vendor/*');" +
			" do (cd \"${f%/*}\" && go build ./... && go test ./... -count=1) || exit 1; done")
	}

	if _, err := os.Stat(filepath.Join(repo, "package.json")); err == nil {
		return "npm test --silent"
	}

	return ""
}

// autoDetectVerify maps a repo's stack markers to its verify command. A
// failing command is a real gate failure: a repo that declares a Makefile
// without a test target SHOULD fail verification, not pass vacuously.
func autoDetectVerify(repo string) string {
	if v := defaultVerify(repo); v != "" {
		return v
	}

	if _, err := os.Stat(filepath.Join(repo, "Makefile")); err == nil {
		return "make test"
	}

	if _, err := os.Stat(filepath.Join(repo, "flake.nix")); err == nil {
		return "nix build && nix flake check"
	}

	if _, err := os.Stat(filepath.Join(repo, "Cargo.toml")); err == nil {
		return "cargo test --quiet"
	}

	return ""
}

// DetectVerify is the exported form of autoDetectVerify for tools that
// prepare repos (tq bootstrap pins the detected command into .tq-verify).
func DetectVerify(repo string) string { return autoDetectVerify(repo) }

// RenderAgentPayload marshals a payload for tasks of type agent.
func RenderAgentPayload(p AgentPayload) (jsontext.Value, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("agent: encode payload: %w", err)
	}

	return b, nil
}
