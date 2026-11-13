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
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/larsartmann/go-retry"
	"github.com/larsartmann/go-taskqueue/internal/queue"
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
	// Items carries the full member list when the task was minted as a
	// BATCH (harvest --batch-items > 1): one task, many work items, one
	// session. Item stays the FIRST member; purely informational.
	Items []string `json:"items,omitempty"`
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
	// Answered carries the owner's rulings for questions this task asked
	// (PapDashboard questions): the store injects them when the answer
	// unblocks the parked task, and the executor renders them into the
	// prompt so the resumed run honors the decisions instead of re-asking.
	Answered []queue.QuestionAnsweredDetail `json:"answered,omitempty"`
	// RejectedSHA is the commit a review-fix task was minted against (the
	// finding's cited commit, falling back to the reviewed run's commit).
	// Structured so the claim-time done-preflight can run the
	// rejected-SHA disposition (exists? superseded? anchor gone?) without
	// parsing prompt prose. Empty on non-fix tasks.
	RejectedSHA string `json:"rejected_sha,omitempty"`
	// Anchor is the verbatim text a review-fix finding attaches to (same
	// provenance as RejectedSHA): the disposition's no-anchor-left signal.
	// Empty when the finding carried no anchor.
	Anchor string `json:"anchor,omitempty"`
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
	// ReresolveVerify enables claim-time verify re-resolution: the payload's
	// enqueue-time verify pin is IGNORED and the gate is resolved fresh
	// (.tq-verify file, then auto-detect) — a verify-contract change in the
	// repo can no longer fire a stale pinned command at a task queued before
	// the change (09-39 f3). Default false keeps the payload contract:
	// file > pin > detect.
	ReresolveVerify bool
	// Worktrees opts the executor into worktree-per-agent delivery: every
	// agent task runs in a linked checkout on branch tq/<task-id> forked
	// from HEAD, so the human's WIP never parks the pool and N tasks of one
	// repo can coexist. The branch is the delivery until the merge
	// machinery lands (worktree-per-agent design,
	// docs/planning/archived/2026-09-12_worktree-per-agent-design.md).
	// Second-opinion clones (review/status/dlqfix/prioritize) never inherit
	// it: they read the repo at HEAD and run close-out-free.
	Worktrees bool
	// CrushServer routes every agent run through a shared `crush server`
	// (L1 client-server seam, docs/planning/2026-10-30_04-43 §L1): runs
	// add `-H <socket>` and the agent env gains CRUSH_CLIENT_SERVER=1, so
	// crush becomes a thin client whose agents execute inside the server
	// process. Empty = today's in-process path; rollback = unset the flag.
	CrushServer string

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

	// secondOpinion marks the closeout-free clones (review/status/dlqfix/
	// prioritize, all built via WithoutCloseout): they ARE the second
	// opinion, so they never receive the $TQ_QUESTION_FILE channel — a
	// reviewer or scorer must not park a task on an owner question
	// (21-04 §f42). Zero value false keeps literally-constructed work
	// executors ask-capable (the park path is pinned on a bare executor).
	secondOpinion bool
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
// atomic.Int64 must never be struct-copied (copylocks). The clone is
// marked secondOpinion: the owner-question channel stays work-turn-only
// (21-04 §f42).
func (e *AgentExecutor) WithoutCloseout() *AgentExecutor {
	return &AgentExecutor{
		Bin:             e.Bin,
		ProjectsDir:     e.ProjectsDir,
		Yolo:            e.Yolo,
		MaxConcurrent:   e.MaxConcurrent,
		ReresolveVerify: e.ReresolveVerify,
		CrushServer:     e.CrushServer,
		secondOpinion:   true,
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

// Binary resolves the agent executable exactly like the run paths do
// (Bin > $TQ_AGENT_BIN > $TQ_CRUSH_BIN > "crush"). Exported for pool-side
// actors that must spawn the SAME binary the runs use — the L1
// crush-server actor (--crush-server) is the first consumer.
func (e *AgentExecutor) Binary() string { return e.binary() }

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
	p, err := decodePayload[AgentPayload](t, "agent", "{repo, prompt}")
	if err != nil {
		return err
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

	// Owner rulings from the park: render them into the prompt so the
	// resumed run starts knowing the answers (claim-time rendering keeps
	// the payload the single source of truth).
	p.Prompt = renderAnswered(p.Prompt, p.Answered)

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

	if requireClean(p) && !e.Worktrees {
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
			if err := assertCleanTree(ctx, repoDir); err != nil {
				// Preflight, not permanent: the human will commit eventually;
				// the worker requeues without burning an attempt.
				return &PreflightError{Cause: err}
			}
		}
	}

	// Worktree-per-agent delivery: fork the task into a linked checkout on
	// branch tq/<task-id> so the human's WIP is untouchable by construction
	// (the dirty-tree preflight above is skipped — a worktree branches from
	// HEAD regardless of WIP). Non-git repos fall back to the in-place run.
	runDir := repoDir

	var wtBranch, wtBase string

	if e.Worktrees {
		// Resolve the base BEFORE the fork: the branch the repo sat on at
		// worktree-create time is the merge target once the review approves.
		wtBase = currentBranch(repoDir, gitRun)

		wtDir, branch, ok, err := createWorktree(repoDir, t.ID.String(), e.ProjectsDir, gitRun)
		if err != nil {
			return err
		}

		if ok {
			runDir, wtBranch = wtDir, branch

			defer func() {
				// A parked close-out resumes in THIS checkout (the registry
				// stores the worktree path), so the removal waits until the
				// close-out turn is no longer owed. The tq/<id> branch always
				// survives — it is the delivery until the review verdict
				// gates the merge. The worktree stays too: the merge (or,
				// past the verdict window, the reaper) owns the checkout —
				// the run itself never destroys evidence mid-review.
				if wtBranch != "" {
					return
				}

				_, _ = removeWorktree(repoDir, wtDir, gitRun)
			}()
		}
	}

	timeout := payloadTimeout(defaultAgentTaskTimeout, p.TimeoutMinutes)

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Pre-attempt baseline for verify-failure classification: when the gate
	// later fails, it is re-run in a throwaway worktree at this rev. A gate
	// that fails there too is pre-existing/environmental (VerifyGateError,
	// requeued WITHOUT burning an attempt); only an introduced failure —
	// baseline green, post-attempt red — counts against the task.
	baseRev := gitHeadRev(repoDir)

	// Session-retry ladder (paperclip's same_session → fresh_session): the
	// FIRST retry resumes the previous attempt's session so the
	// conversation carries its context — a re-ask is cheaper than a
	// re-explanation, and a retry usually follows a transient provider
	// failure. From the second retry on, back to fresh: a session that
	// failed twice is itself the prime suspect. A payload-pinned session
	// always wins.
	if p.Session == "" {
		p.Session = retrySession(t)
	}

	runStart := time.Now()

	output, verdict, err := e.runAgent(runCtx, runDir, &p, t.ID)
	if err != nil {
		// Forensics for the task.failed fact: exit code + output tail. The
		// full output survives in the sidecar only when TQ_LOG_DIR is set,
		// so the fact carries its own excerpt.
		SetFailureEvidence(ctx, "agent", err, tailBytes([]byte(output), EvidenceTailBytes))
		EnrichFailureEvidenceFiles(ctx, runDir, t.ID)

		// Routed runs severed by the shared server dying (dial failures,
		// failed respawn) classify as environmental: requeue, no burn.
		if e.CrushServer != "" && DetectCrushServerDeath(output, err.Error()) {
			return &CrushServerDeathError{Cause: err}
		}

		return err
	}

	// The rc-0 lie: a MID-RUN server death leaves the client exiting 0
	// with only a severed-events WARN in the output (observed v0.99.0) —
	// no work happened, but the verify leg below would judge the task on
	// an empty run. Classify before any verdict path.
	if e.CrushServer != "" && DetectCrushServerDeath(output, "") {
		return &CrushServerDeathError{}
	}

	verifyOut, err := runVerify(runCtx, t.ID, runDir, &p, e.ReresolveVerify, baseRev)
	if err != nil {
		SetVerifyFailureEvidence(ctx, verifyOut, err)
		EnrichFailureEvidenceFiles(ctx, runDir, t.ID)

		return err
	}
	// Success: record structured outcome detail for `tq show`. Derivation
	// is the sole source (git + crush session data — the queue's own view
	// of what the run did); the legacy TQ_RESULT stdout self-report is
	// retired. Best effort — a missing session id or empty derivation is
	// not an error.
	result := AgentResult{
		VerifyTail: verifyOut.Tail,
		Verify:     verifyOut.Receipt,
	}

	derived := result.deriveUsage(ctx, runDir, output, t.ID)
	result.Commits = derived.Commits
	result.FilesChanged = derived.Files

	// Structured closeout facts (row 1446): routed runs get the session
	// identity, usage block, and tool-call inventory from the crush
	// server's messages endpoint; in-process runs and any fetch failure
	// keep the stdout-derived values (the fallback the row mandates).
	e.enrichCloseoutFacts(ctx, runDir, &result, runStart)

	if n := len(derived.Commits); n > 0 {
		result.CommitSHA = derived.Commits[n-1].SHA
	}

	result.Attestation = verifyOut.Attest(result.CommitSHA)

	// Worktree forensics: where the run actually happened and which branch
	// carries the commits (the merge/reap machinery keys off the prefix);
	// BaseBranch names the merge target for the post-approve merge.
	result.WorktreeDir = runDir
	result.Branch = wtBranch
	result.BaseBranch = wtBase

	recordRunOutcome(ctx, &result, "agent", verdict, output, verifyOut.Tail, t.ID)

	return nil
}

// writeOutputSidecar persists the FULL agent + verify output to
// $TQ_LOG_DIR/<task-id>.log and returns the path — "" when the directory is
// unset or the write fails (logging must never fail a completed task). The
// body goes through the secrets-in-logs pass first: sidecars are the
// token-bearing surface named beside the facts (17-21 #18 / 20-58 f33).
func writeOutputSidecar(id task.ID, agentOutput, verifyOutput string) string {
	dir := os.Getenv("TQ_LOG_DIR")
	if dir == "" {
		return ""
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}

	path := filepath.Join(dir, id.String()+".log")

	body := redactOutput(agentOutput)
	if verifyOutput != "" {
		body += "\n--- verify ---\n" + redactOutput(verifyOutput)
	}

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return ""
	}

	return path
}

// WorktreeInUse reports whether the task still owes its close-out turn:
// the reaper must treat such worktrees as in-flight (the parked close-out
// resumes in that exact checkout).
func (e *AgentExecutor) WorktreeInUse(id string) bool {
	_, parked := e.closeoutPending.Load(id)

	return parked
}

// ParkCloseoutForTest arms a closeoutPending entry without running an
// agent turn: TEST-ONLY injection for consumers (cmd/tq's
// worktreeInFlight) that must pin how a parked executor entry interacts
// with store status — the exact cancelled+parked regression shape the
// worktree reaper had before terminal status won.
func (e *AgentExecutor) ParkCloseoutForTest(id, repoDir, session string) {
	e.armCloseoutResume(id, repoDir, session)
}

// repoDir resolves a payload repo name: absolute paths pass through,
// relative names resolve against ProjectsDir.
func (e *AgentExecutor) repoDir(repo string) (string, error) {
	return resolveRepoDir("agent", e.ProjectsDir, repo)
}

// ResolveRepoDir is the exported seam behind resolveRepoDir (the review
// sweeper resolves payload repos for the post-approve worktree merge).
func ResolveRepoDir(domain, projectsDir, repo string) (string, error) {
	return resolveRepoDir(domain, projectsDir, repo)
}

// resolveRepoDir is the shared repo resolution behind the executors that
// run inside a repo (agent, depbump): absolute paths pass through, relative
// names resolve against projectsDir. The domain string prefixes errors with
// the calling executor's name.
func resolveRepoDir(domain, projectsDir, repo string) (string, error) {
	if filepath.IsAbs(repo) {
		if info, err := os.Stat(repo); err != nil || !info.IsDir() {
			return "", fmt.Errorf("%s: repo directory does not exist: %s", domain, repo)
		}

		return repo, nil
	}

	if projectsDir == "" {
		return "", fmt.Errorf("%s: relative repo %q needs a projects dir on the executor", domain, repo)
	}

	dir := filepath.Join(projectsDir, repo)

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s: repo %q does not exist under the projects dir", domain, repo)
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
//
// art-dupl:accept exec wiring: one buffer aliased to both Stdout and Stderr; ruling: docs/status/2026-10-05_16-02_art-dupl-t3-tail-zero-dedup-sweep.md a)3
// is the os/exec idiom; a seam would outlive the three lines it saves.
func assertCleanTree(ctx context.Context, repo string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "status", "--porcelain")

	var out bytes.Buffer

	cmd.Stdout = &out

	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("agent: git status failed in %s: %w: %s", repo, err, tailBytes(out.Bytes(), 512))
	}

	if s := strings.TrimSpace(out.String()); s != "" {
		// The sentinel rides %w in BRACKETS so the rendered refusal ends
		// "… <porcelain tail> [dirty tree]" — a distinct, greppable suffix
		// instead of the bare words glued to the last porcelain line
		// (pinned in TestAgentExecutorRefusesDirtyTree so journal greps
		// stay stable).
		return fmt.Errorf(
			"agent: repo %s has uncommitted changes; refusing to run agent (commit/stash first, or set require_clean=false): %s [%w]",
			repo,
			tailBytes(out.Bytes(), 512),
			ErrDirtyTree,
		)
	}

	return nil
}

// verdictFileEnv names the per-run channel the executor hands the agent:
// `tq verdict '<json>'` (or a plain write) lands the structured result in
// this file — the ONE verdict channel. runAgent reads the file directly
// before it returns (the stdout TQ_RESULT line fallback is retired:
// every live task mints on file-channel prompts and pre-flip tasks have
// completed).
const verdictFileEnv = "TQ_RESULT_FILE"

// verdictRailTmpl is the ONE prompt rail that keeps the verdict FILE
// channel alive under `--crush-server`: the routed bash tool sees only
// the SERVER process env (M8), so $TQ_RESULT_FILE is invisible to a
// routed agent. The rail bakes the per-run path into the prompt itself
// and teaches the flag form — `tq verdict --file <path> '<json>'` — so
// local and routed runs share ONE contract. Always-on when the channel
// exists (verdictPath != "").
const verdictRailTmpl = "\n\nVerdict channel: record your final structured verdict before exiting by running: tq verdict --file %s '<one-line result JSON>' — the queue reads this file; it is the ONLY verdict channel (a stdout line is never parsed)."

// questionRailTmpl is the question channel's M8 bridge — same pattern as
// the verdict rail: the prompt names the per-run marker path so
// `tq ask --task <id> --file <path> '…'` works whether the turn runs
// in-process (env channel) or routed (env gap). Only WORK turns and
// their closeouts carry it — second-opinion clones (review/status/
// dlqfix/prioritize) never get the question channel.
const questionRailTmpl = "\n\nOwner-question channel: if you need an owner ruling before finishing, run: tq ask --task %s --file %s 'one-line question' — the task parks without attempt burn until the answer arrives."

// crushClientServerEnv turns `crush run` into a thin client whose agent
// executes inside the shared `crush server` (L1 seam; see AgentExecutor.
// CrushServer). Set ONLY alongside the matching -H <socket> run arg.
const crushClientServerEnv = "CRUSH_CLIENT_SERVER"

// crushHostFlag normalizes the seam's -H value: crush rejects a bare
// socket path with "Invalid host URL: invalid host format" — a
// scheme-less path must ride unix:// (pool smoke V2.10 leg caught the
// seam passing the flag through unnormalized; a full URL stays as-is).
func crushHostFlag(server string) string {
	if strings.Contains(server, "://") {
		return server
	}

	return "unix://" + server
}

// envDenylistForAgents is the operational environment the agent process
// must NOT inherit (secret-injection seam, M24 — first rung):
//   - TQ_DB: the queue's own journal path (in the pool process, the
//     PRODUCTION journal). An agent that shells out to `tq` in its repo
//     would otherwise read or mutate the live journal; a scratch `tq`
//     needs its own db anyway (AGENTS known-issues: the inherited-TQ_DB
//     smoke hazard). The executor passes the verdict/question channels
//     explicitly, so the agent needs nothing else from the TQ_ namespace.
//
// This is a DENYLIST — deliberately narrow. The successor design (minted
// per-run allowlist) lives in
// docs/planning/2026-10-05_secret-injection-seam-design.md; a wider list
// needs the owner ruling it names, because provider keys the agent
// legitimately consumes (crush reads them from the environment) must
// never land here by pattern-matching enthusiasm.
var envDenylistForAgents = []string{"TQ_DB"}

// agentEnv strips the agent-process denylist from an inherited
// environment. Match is exact-key (KEY=VALUE split on the first '='); a
// malformed entry (no '=') passes through untouched.
func agentEnv(base []string) []string {
	out := base[:0:0]
	for _, kv := range base {
		key, _, found := strings.Cut(kv, "=")
		if !found {
			out = append(out, kv)

			continue
		}

		if !slices.Contains(envDenylistForAgents, key) {
			out = append(out, kv)
		}
	}

	return out
}

// retrySession is the retry ladder's session rung: the FIRST retry
// resumes the previous attempt's session (paperclip's same_session), later
// retries go fresh (the session itself may be what's sick). The previous
// attempt's session id comes from the run-output sidecar
// ($TQ_LOG_DIR/<task-id>.log): deterministic per task id, and at retry
// time it still holds the previous attempt's output — the current attempt
// has not overwritten it yet (writeOutputSidecar runs at outcome time).
// No sidecar, no directory, no id in the output: all degrade to fresh,
// which is today's behavior.
func retrySession(t task.Task) string {
	if t.Attempts != 1 {
		return "" // fresh: the first run, or the session had its chance
	}

	dir := os.Getenv("TQ_LOG_DIR")
	if dir == "" {
		return ""
	}

	body, err := os.ReadFile(filepath.Join(dir, t.ID.String()+".log"))
	if err != nil {
		return ""
	}

	return ExtractSessionID(string(body))
}

// runAgent spawns the headless agent in the repo and waits for it.
func (e *AgentExecutor) runAgent(
	ctx context.Context,
	repoDir string,
	p *AgentPayload,
	id task.ID,
) (string, jsontext.Value, error) {
	// The verdict channel: one temp file per run, exported to the agent
	// process. Best-effort — when the temp dir is unusable the channel is
	// absent and the run records no verdict (strict-parser executors fail
	// the attempt; plain work turns ride the footer derivation).
	verdictPath := ""

	if f, err := os.CreateTemp("", "tq-verdict-*.json"); err == nil {
		_ = f.Close()

		verdictPath = f.Name()

		defer func() { _ = os.Remove(verdictPath) }()
	}

	// The question channel: same per-run pattern. `tq ask` writes the
	// marker; a written marker parks the task after the run (question.go).
	questionPath := ""

	if f, err := os.CreateTemp("", "tq-question-*.json"); err == nil {
		_ = f.Close()

		questionPath = f.Name()

		defer func() { _ = os.Remove(questionPath) }()
	}
	// finish folds the verdict-file read into the run outcome: the file is
	// consumed DIRECTLY (never merged into the output as a synthetic line —
	// the stdout channel is retired), and an invalid verdict body fails the
	// attempt alongside any run error (errors.Join keeps Is/As classes
	// intact for the worker's cancel/rate-limit matching).
	finish := func(output string, err error) (string, jsontext.Value, error) {
		v, verr := ReadVerdict(verdictPath)

		return output, v, errors.Join(err, verr)
	}

	// Closeout resume (13:29 report f15): a prior attempt finished the WORK
	// turn but was rate-limited during the close-out; re-running the work
	// turn on re-claim would double real agent cost. Resume at closeout
	// instead. In-process only (an executor restart forgets it — the
	// fallback is the old full re-run, never a lost close-out).
	if out, handled, err := e.resumePendingCloseout(ctx, id, repoDir, verdictPath, questionPath); handled {
		return finish(out, err)
	}

	// Provider gate: a sibling task just observed the provider refusing
	// (429/usage limit). Refuse BEFORE spawning crush — a probe into a
	// spent quota costs a process spawn, agent-side retry noise, and on
	// metered providers potentially billed tokens. The returned class
	// carries the wait, so the worker requeues until the reset without
	// burning an attempt.
	if wait, limited := e.rateLimitWaitRepo(repoDir); limited {
		return finish("", &RateLimitError{
			Cause: errors.New(
				"agent: provider rate limit in effect (observed by a sibling run); deferring until reset",
			),
			RetryAfter:  wait,
			FastRefused: true,
		})
	}

	// The queue task ID is only known at execution time (the harvester
	// renders prompts before enqueue), so the {{TASK_ID}} placeholder in
	// prompt contracts resolves HERE — it lets agents put `Task-Queue-ID:
	// <id>` footers in their commits so git log ↔ tq facts cross-reference
	// (21:40 report §e3).
	p.Prompt = strings.ReplaceAll(p.Prompt, "{{TASK_ID}}", id.String())

	// The verdict rail (M8 bridge): the prompt itself names the channel,
	// so the verdict file works whether the agent turn runs in-process
	// (env channel) or routed through the pool's crush server (env gap).
	if verdictPath != "" {
		p.Prompt += fmt.Sprintf(verdictRailTmpl, verdictPath)
	}

	// The question rail rides the same bridge but ONLY for executors that
	// carry the question channel (second-opinion clones park nothing).
	if questionPath != "" && !e.secondOpinion {
		p.Prompt += fmt.Sprintf(questionRailTmpl, id.String(), questionPath)
	}

	if e.Yolo || p.Yolo {
		if err := requireRepoAutonomy(repoDir); err != nil {
			return finish("", err)
		}
	}

	// The close-out turn resumes the exact session by id, so the work turn
	// runs verbose when enabled: quiet mode suppresses the "Created session
	// … session_id=" line ExtractSessionID needs, and --continue would race
	// for the most recent session when another pool agent finishes at the
	// same moment. Routed runs are ALWAYS verbose: quiet also suppresses
	// the severed-stream WARN ("Events stream ended") — the only output
	// discriminator a mid-run server death leaves on the rc-0 lie path
	// (observed v0.99.0: a quiet severed run exits 0 with a 1-byte output;
	// pinned live by the pool smoke's V2.10 leg).
	runVerb := "--quiet"
	if e.CloseoutPrompt != "" || e.CrushServer != "" {
		runVerb = "--verbose"
	}

	args := []string{"run", runVerb, "--cwd", repoDir}
	if e.CrushServer != "" {
		// L1 seam: thin-client mode against the pool's shared crush server.
		args = append(args, "-H", crushHostFlag(e.CrushServer))
	}

	if p.Model != "" {
		args = append(args, "--model", p.Model)
	}

	if p.Session != "" {
		args = append(args, "--session", p.Session)
	}

	args = append(args, "--", p.Prompt)

	// The buffer lives OUTSIDE the attempt closure: DoWithValue returns
	// the zero value when every attempt fails, but the captured tail must
	// survive the error — failure evidence is the caller's exhibit. A
	// retried ETXTBSY attempt produced no output (execve never ran), so
	// retries cannot interleave into the buffer.
	var buf bytes.Buffer

	runOnce := func() (*bytes.Buffer, error) {
		cmd := exec.CommandContext(ctx, e.binary(), args...)
		cmd.Dir = repoDir

		// Agents get the pool environment MINUS the operational denylist
		// (agentEnv): the queue's own handles are not an agent's business.
		// The full secret-injection successor (minted per-run env) is
		// designed in docs/planning/2026-10-05_secret-injection-seam-design.md.
		env := agentEnv(os.Environ())
		if e.CrushServer != "" {
			env = append(env, crushClientServerEnv+"=1")
		}

		if verdictPath != "" {
			cmd.Env = append(env, verdictFileEnv+"="+verdictPath)
		}

		// The question channel is a WORK-turn capability (21-04 §f42):
		// second-opinion clones (review/status/dlqfix/prioritize) never
		// receive $TQ_QUESTION_FILE — a reviewer or scorer must not park
		// a task on an owner question.
		if questionPath != "" && !e.secondOpinion {
			cmd.Env = append(cmd.Env, questionFileEnv+"="+questionPath)
		}

		cmd.Stdout = &buf
		cmd.Stderr = &buf
		// Kill the whole process tree on cancel (agents spawn children) and do
		// not hang the worker if grandchildren hold the pipes open.
		prepareProcessGroup(cmd)

		cmd.WaitDelay = 10 * time.Second
		err := cmd.Run()

		return &buf, err
	}

	_, err := execWithTransientRetry(runOnce)
	if err != nil {
		// The captured output survives the error so the caller can pin the
		// failure evidence's tail excerpt. The cancelled branch must keep
		// wrapping ctx.Err(): the worker finalizes cooperative cancels by
		// matching context.Canceled.
		if ctx.Err() != nil {
			return finish(
				buf.String(),
				fmt.Errorf("agent run cancelled (%w): %s", ctx.Err(), tailBytes(buf.Bytes(), 8192)),
			)
		}

		// Provider exhaustion (Z.ai 429 usage-limit windows, OpenAI-style
		// quota refusals from synthetic.new and friends): return the
		// requeue-able class instead of a plain failure, so the worker
		// parks the task until the provider resets WITHOUT burning an
		// attempt. Detection also arms the gate for sibling runs.
		if rl := e.rateLimitedTurn("agent run", repoDir, err, buf.String()); rl != nil {
			return finish(buf.String(), rl)
		}

		return finish(buf.String(), fmt.Errorf("agent run failed: %w: %s", err, tailBytes(buf.Bytes(), 8192)))
	}

	// Owner question parked the run (tq ask wrote the marker): the turn
	// ends here — no verify (nothing to prove yet) and no close-out (the
	// task is not done). The worker requeues without burning an attempt.
	if qp := questionPendingFrom(questionPath, time.Now()); qp != nil {
		return finish(buf.String(), qp)
	}

	// Second turn, same conversation: the agent that did the work answers
	// the close-out self-review before the task completes. A failed closeout
	// fails the attempt like any other contract breach; a missing session id
	// degrades to a logged skip (the work itself already succeeded).
	out, cerr := e.runCloseoutIfNeeded(ctx, id, repoDir, &buf, verdictPath, questionPath)

	return finish(out, cerr)
}

// parkedAfterCloseout checks the question marker after a close-out turn: a
// written marker parks the task and arms the closeout resume, so the
// re-claim resumes at closeout instead of re-running the paid work turn.
// Returns nil when no question is pending.
func (e *AgentExecutor) parkedAfterCloseout(id task.ID, repoDir, session, questionPath string) error {
	qp := questionPendingFrom(questionPath, time.Now())
	if qp == nil {
		return nil
	}

	e.armCloseoutResume(id.String(), repoDir, session)

	if qpe, ok := errors.AsType[*QuestionPendingError](qp); ok {
		qpe.ResumeCloseout = true
	}

	return qp
}

// resumePendingCloseout resumes a parked close-out when a prior attempt of
// this task registered one for this repoDir (rate-limited or question
// parked). Returns handled=false when nothing was registered or the
// registration went stale — the caller proceeds with the normal work turn.
func (e *AgentExecutor) resumePendingCloseout(
	ctx context.Context,
	id task.ID,
	repoDir, verdictPath, questionPath string,
) (string, bool, error) {
	v, ok := e.closeoutPending.Load(id.String())
	if !ok {
		return "", false, nil
	}

	pending := v.(closeoutPending)

	e.closeoutPending.Delete(id.String())

	if pending.repoDir != repoDir {
		return "", false, nil
	}

	buf := &bytes.Buffer{}
	if err := e.runCloseoutTurn(ctx, repoDir, pending.session, id, buf, verdictPath, questionPath); err != nil {
		return buf.String(), true, err
	}

	if qp := e.parkedAfterCloseout(id, repoDir, pending.session, questionPath); qp != nil {
		return buf.String(), true, qp
	}

	return buf.String(), true, nil
}

// runCloseoutIfNeeded performs the configured close-out turn: a missing
// session id degrades to a logged skip; a parked question after the
// close-out arms the resume and parks the task.
func (e *AgentExecutor) runCloseoutIfNeeded(
	ctx context.Context,
	id task.ID,
	repoDir string,
	buf *bytes.Buffer,
	verdictPath, questionPath string,
) (string, error) {
	if e.CloseoutPrompt == "" {
		return buf.String(), nil
	}

	session := ExtractSessionID(buf.String())
	if session == "" {
		buf.WriteString("\n[tq] closeout skipped: no session id in agent output\n")

		return buf.String(), nil
	}

	if err := e.runCloseoutTurn(ctx, repoDir, session, id, buf, verdictPath, questionPath); err != nil {
		return buf.String(), err
	}

	if qp := e.parkedAfterCloseout(id, repoDir, session, questionPath); qp != nil {
		return buf.String(), qp
	}

	return buf.String(), nil
}

// closeoutPending records a finished WORK turn whose close-out is owed: a
// rate-limited close-out requeues the task WITHOUT burning an attempt, and
// the re-claim must resume here instead of re-running the paid work turn.
//
// Lifetime audit (00-11 report e5): entries are removed on resume or
// process restart only — a task that dies CANCELLED while parked never
// re-claims, so its entry lingers (two strings, bounded, in-memory).
// Harmless here; the observable leak was the worktree reaper consulting
// WorktreeInUse without consulting the store's terminal status — fixed by
// worktreeInFlight in cmd/tq (terminal status wins).
type closeoutPending struct {
	repoDir string
	session string
}

// armCloseoutResume registers a finished WORK turn whose close-out is owed
// because the run asked a question: the re-claim resumes at closeout
// instead of re-running the paid work turn.
func (e *AgentExecutor) armCloseoutResume(id, repoDir, session string) {
	e.closeoutPending.Store(id, closeoutPending{repoDir: repoDir, session: session})
}

// runCloseoutTurn runs the second conversation turn (the close-out
// self-review) in the session the work turn opened, appending its output to
// buf. Rate-limited close-outs register closeoutPending BEFORE returning
// the *RateLimitError so the re-claim resumes here.
func (e *AgentExecutor) runCloseoutTurn(
	ctx context.Context,
	repoDir, session string,
	id task.ID,
	buf *bytes.Buffer,
	verdictPath string,
	questionPath string,
) error {
	closeout := strings.ReplaceAll(e.CloseoutPrompt, "{{TASK_ID}}", id.String())

	// The closeout turn records its own verdict (the a)-g) report turn):
	// same rail as the work turn, same always-on contract.
	if verdictPath != "" {
		closeout += fmt.Sprintf(verdictRailTmpl, verdictPath)
	}

	if questionPath != "" && !e.secondOpinion {
		closeout += fmt.Sprintf(questionRailTmpl, id.String(), questionPath)
	}

	// Routed closeouts run verbose for the same reason as routed work
	// turns: the rc-0 severed-stream lie is only visible in verbose output.
	closeoutVerb := "--quiet"
	if e.CrushServer != "" {
		closeoutVerb = "--verbose"
	}

	closeoutArgs := []string{"run", closeoutVerb, "--cwd", repoDir}
	if e.CrushServer != "" {
		closeoutArgs = append(closeoutArgs, "-H", crushHostFlag(e.CrushServer))
	}

	closeoutArgs = append(closeoutArgs, "--session", session, "--", closeout)

	// Same buffer-outside-the-attempt shape as the work turn: the closeout
	// report evidence must survive a failed DoWithValue (zero value).
	var closeoutBuf bytes.Buffer

	closeoutOnce := func() (*bytes.Buffer, error) {
		cmd := exec.CommandContext(ctx, e.binary(), closeoutArgs...)
		cmd.Dir = repoDir

		closeoutEnv := agentEnv(os.Environ())
		if e.CrushServer != "" {
			closeoutEnv = append(closeoutEnv, crushClientServerEnv+"=1")
		}

		if verdictPath != "" {
			cmd.Env = append(closeoutEnv, verdictFileEnv+"="+verdictPath)
		}

		// Second-opinion clones never get the question channel (same gate
		// as the work turn; the closeout turn only exists for work
		// executors anyway).
		if questionPath != "" && !e.secondOpinion {
			cmd.Env = append(cmd.Env, questionFileEnv+"="+questionPath)
		}

		cmd.Stdout = &closeoutBuf
		cmd.Stderr = &closeoutBuf
		prepareProcessGroup(cmd)
		cmd.WaitDelay = 10 * time.Second
		err := cmd.Run()

		return &closeoutBuf, err
	}

	_, err := execWithTransientRetry(closeoutOnce)

	buf.WriteString(closeoutBuf.String())

	if err == nil {
		// Same rc-0 lie as the work turn: a server death mid-closeout
		// exits 0 with the report unwritten — classify so the worker
		// requeues without burn instead of completing on silence.
		if e.CrushServer != "" && DetectCrushServerDeath(closeoutBuf.String(), "") {
			return &CrushServerDeathError{}
		}

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
		// (13:29 report f15). The flag rides the error so the worker pins
		// the resume contract into the requeue fact (16-00 f31).
		if rle, ok := errors.AsType[*RateLimitError](rl); ok {
			rle.ResumeCloseout = true
		}

		e.closeoutPending.Store(id.String(), closeoutPending{repoDir: repoDir, session: session})

		return rl
	}

	return fmt.Errorf("agent closeout failed: %w: %s", err, tailBytes(buf.Bytes(), 8192))
}

// DefaultCloseoutPrompt is the second conversation turn every agent task
// runs when the pool enables --task-closeout: the same brutal self-review
// the owner uses interactively, answered by the agent that did the work,
// in the same session. {{TASK_ID}} resolves at execution time (report path
// + commit footer). The queue derives the run's commits and files from git
// trailers — the close-out no longer re-emits any result line.
const DefaultCloseoutPrompt = `What did you forget? What could you have done better? What could you still improve?

FULL COMPREHENSIVE & DETAILED STATUS UPDATE!
INCLUDE WORK: a) FULLY DONE; b) PARTIALLY DONE; c) NOT STARTED; d) TOTALLY FUCKED UP! e) WHAT WE SHOULD IMPROVE! f) Up to 50 things we should get done next! g) Ask up to 3 questions that you can NOT figure out yourself!

Write the full report at docs/status/tasks/<YYYY-MM-DD_HH-MM>_task-{{TASK_ID}}.md (timestamp from the clock, never guessed). Commit it with the same Task-Queue-ID footer as your work commit. The footer must be the FINAL line of the message: if your tooling appends an attribution block (e.g. "Generated with Crush" / "Assisted-by"), put the footer AFTER that block — a footer stranded above attribution is invisible to git trailers and the pre-commit hook rejects the commit. Never push.

TODO_LIST.md appends from THIS closeout are HARD CAP 10 new unchecked items (the status prompt's cap): pick the highest-value follow-ups from (e)/(f)/(g); the full (f) list stays in the report only. Dedup-check existing unchecked items first — a reworded duplicate mints a fresh paid dispatch.

DO NOT RESEARCH UNRELATED STUFF. Report based on THIS task's work and what you noticed in passing.`

// execWithTransientRetry retries exec attempts that failed with ETXTBSY
// ("text file busy"). Kernel 7.2 was observed returning it for freshly
// written stub executables under concurrent process churn with no writer
// holding the file — an executor-level retry (50ms, then 100ms) absorbs
// the whole failure class instead of failing a task attempt. Every other
// error passes through untouched.
func execWithTransientRetry[T any](run func() (T, error)) (T, error) {
	return retry.DoWithValue[T](context.Background(), retry.Config{
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		IsRetryable:  func(err error) bool { return errors.Is(err, syscall.ETXTBSY) },
	}, func(_ context.Context, _ int) (T, error) {
		return run()
	})
}

// VerifyOutcome is one verify-gate run's result: the bounded output tail, the
// death class (a VerifyStage* value, empty on success), the structured receipt
// describing what actually ran, and the declarations the run saw. One type for
// both paths — a failing gate and a passing gate answer the same questions.
//
// RepoVerify and PayloadVerify are the gate-time declarations (not re-read
// later: a declaration that drifts after the gate is exactly what the attester
// must not silently absorb), and Output is the captured gate output the count
// comparison re-derives from. Output is NOT stored in the journal detail — the
// receipt is the bounded summary, the sidecar is the log.
type VerifyOutcome struct {
	Tail          string
	Stage         string
	Receipt       *VerifyReceipt
	RepoVerify    string
	PayloadVerify string
	Output        []byte
}

// runVerify enforces the quality gate after the agent exited cleanly. The
// repo's .tq-verify file wins over everything (the repo is the source of
// truth for how it proves itself), then the payload, then auto-detect —
// unless reresolve drops the enqueue-time pin (AgentExecutor.ReresolveVerify):
// then the file, then auto-detect, never a stale pin.
//
// Failures are classified before they count (classifyVerifyFailure): a gate
// that also fails at the pre-attempt rev (baseRev) or that was killed by the
// deadline before a verdict becomes a VerifyGateError — the worker requeues
// WITHOUT burning an attempt. Only an introduced failure (baseline green,
// post-attempt red) returns the plain verify-failed error.
func runVerify(
	ctx context.Context,
	id task.ID,
	repoDir string,
	p *AgentPayload,
	reresolve bool,
	baseRev string,
) (VerifyOutcome, error) {
	verify := verifyFor(repoDir, p, reresolve)
	if verify == "" {
		return VerifyOutcome{}, nil // nothing to verify (unknown stack, no explicit command)
	}

	// The declarations are captured at GATE time, beside the command that
	// won: the attester compares "what ran" against "what was pinned", and a
	// re-read after the fact would let the declaration drift into agreement.
	repoVerify := readTQVerify(repoDir)

	cmd := exec.CommandContext(ctx, "sh", "-c", verify)
	cmd.Dir = repoDir

	buf := bufferedProcessGroup(cmd)

	// outcome assembles the result of the run that just finished: the tail at
	// the caller's bound, the death class, the receipt, and the declarations
	// the attestation needs. Built from the buffer AFTER the run, so every
	// field is an observation.
	outcome := func(tail, stage string, rc int) VerifyOutcome {
		return VerifyOutcome{
			Tail:          tail,
			Stage:         stage,
			Receipt:       verifyReceipt(verify, repoDir, rc, buf.Bytes()),
			RepoVerify:    repoVerify,
			PayloadVerify: p.Verify,
			Output:        buf.Bytes(),
		}
	}

	if err := cmd.Run(); err != nil {
		// The tail rides along even on error: it IS the failure evidence
		// (what the gate printed before dying) — but bounded small: verify
		// gates like `nix flake check` print thousands of warning/trace
		// lines and the tail lands verbatim in the task.failed journal
		// fact. The full output goes to the sidecar evidence file; the
		// fact carries the excerpt plus the path.
		tail := tailBytes(buf.Bytes(), verifyErrorTailBytes)
		// Stage stamp from the FULL buffer, at the cut: the tail (and any
		// sidecar log, once rotated) loses the head-of-output kill markers
		// and swallows the gofmt stage's own output, so this is the only
		// point where the death class is still derivable (row 144).
		stage := verifyDeathStage(ctx, err, verify, buf.Bytes())

		if evidence := writeVerifyEvidence(id, buf.Bytes()); evidence != "" {
			tail = fmt.Sprintf("%s\n(full verify output: %s)", tail, evidence)
		}

		out := outcome(tail, stage, exitCode(err))

		if gateErr := classifyVerifyFailure(ctx, verify, repoDir, baseRev, err, tail); gateErr != nil {
			return out, gateErr
		}

		if ctx.Err() != nil {
			return out, fmt.Errorf("agent verify cancelled (%w): %s", ctx.Err(), tail)
		}

		return out, fmt.Errorf("agent verify failed (%q): %w: %s", verify, err, tail)
	}

	return outcome(tailBytes(buf.Bytes(), 2048), "", 0), nil
}

// verifyReceipt builds the gate receipt from what the run ACTUALLY did: the
// command that executed (not the one we meant to run), the exit code, the HEAD
// at gate time (captured before any post-gate derivation moves the tree), and
// the ok/fail tallies the closeout would otherwise hand-grep. Every field is an
// observation, never an intention — which is what lets the attester contradict
// it.
func verifyReceipt(command, repoDir string, rc int, output []byte) *VerifyReceipt {
	okCount, failCount := ParseVerifyCounts(string(output))

	return &VerifyReceipt{
		Command: command,
		RC:      rc,
		Head:    gitHeadRev(repoDir),
		OK:      okCount,
		Fail:    failCount,
	}
}

// verifyErrorTailBytes bounds the verify-failure excerpt inlined into the
// task.failed journal fact; the full output goes to the sidecar evidence
// file (writeVerifyEvidence) instead of the journal stream.
const verifyErrorTailBytes = 512

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
// (unless reresolve) the payload's explicit verify, then auto-detection from
// the repo layout.
func verifyFor(repoDir string, p *AgentPayload, reresolve bool) string {
	if v := readTQVerify(repoDir); v != "" {
		return v
	}

	if p.Verify != "" && !reresolve {
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

// GoEnvExperiment is the GOEXPERIMENT value minted Go verify commands export
// so the gate is identical inside and outside the flake devShell: without
// it, encoding/json/v2 imports die with "build constraints exclude all Go
// files" (the tq-agent-pool unit env carries no GOEXPERIMENT — the env lie
// that judged finished work on a broken gate).
const GoEnvExperiment = "GOEXPERIMENT=jsonv2"

// goEnvPrelude makes a minted Go verify command env-self-contained: the
// tq-agent-pool unit (and any bare shell) carries no GOEXPERIMENT, and repos
// importing encoding/json/v2 then die with "build constraints exclude all Go
// files" — an environment lie that judges finished work on a broken gate.
// Minted Go verifies carry the export so the gate is identical inside and
// outside the flake devShell. Non-Go stacks are untouched; a repo pinning an
// older toolchain that rejects the experiment pins its own .tq-verify (the
// file is the source of truth and is never auto-rewritten behind an
// existing value).
const goEnvPrelude = "export " + GoEnvExperiment + "; "

// withGoEnvPrelude prefixes a minted verify command with goEnvPrelude unless
// the command already manages GOEXPERIMENT itself (idempotent).
func withGoEnvPrelude(verify string) string {
	if strings.Contains(verify, "GOEXPERIMENT") {
		return verify
	}

	return goEnvPrelude + verify
}

// ScopedGofmtStage is the gitignore-aware gofmt stage minted Go verify
// commands carry: `gofmt -l .` walks the whole tree, gitignored vendor/
// included (generated code an agent must not reformat), and
// `git check-ignore --stdin -v --non-matching` prints `::<TAB><path>` for
// exactly the paths git does NOT ignore, so the gate fails only on
// formatting defects in files that could be committed. Untracked files
// stay checked on purpose: agents write new files before committing them,
// so a tracked-only listing would open the exact hole the gate exists to
// close. Outside a git work tree the stage degrades to a pass (agent
// verifies always run in git clones). This kills the vendor-gofmt death
// class (P2) at the mint instead of classifying it after the fact.
const ScopedGofmtStage = `test -z "$(gofmt -l . | git check-ignore --stdin -v --non-matching | grep '^::')"`

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
		return withGoEnvPrelude("go build ./... && go test ./... -count=1 && " + ScopedGofmtStage +
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
