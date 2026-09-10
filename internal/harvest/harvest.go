// Package harvest turns repository backlogs into queue tasks.
//
// It is the ingestion half of a self-managing agent pool: every repo's
// TODO_LIST.md is the shared, human-readable work backlog; the harvester
// repeatedly scans it and enqueues one agent task per open item, paced to at
// most one in-flight task per repo. Agents (see executor.AgentExecutor) do
// the item and mark it done in the same file, which closes the loop: done
// items are never re-enqueued, edited items become new work.
package harvest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// DefaultType is the task type harvest enqueues: the headless agent executor
// (executor.AgentExecutor, registered as "agent" by the tq CLI).
const DefaultType = "agent"

// DefaultTodoFile is the backlog file scanned in each repository.
const DefaultTodoFile = "TODO_LIST.md"

// DefaultMaxPerTick bounds how many new agent tasks one harvest run may
// enqueue across all repos — the cost throttle for unattended pools.
const DefaultMaxPerTick = 10

// DefaultPromptTemplate is the agent contract handed to Crush for each item.
// Placeholders: {{REPO_ABS}}, {{REPO}}, {{HEADING}}, {{ITEM}}, and
// {{TASK_ID}} (substituted at EXECUTION time by the agent executor — the
// queue task ID does not exist when the harvester renders this template;
// it exists so commits can carry a Task-Queue-ID footer for git log ↔
// tq facts cross-reference).
const DefaultPromptTemplate = `You are an autonomous agent working from a shared task queue, unsupervised.

Repository: {{REPO_ABS}}
Work item from TODO_LIST.md, section "{{HEADING}}":
"{{ITEM}}"

Contract:
1. Read AGENTS.md (and CONTRIBUTING.md / CLAUDE.md if present) first and follow it.
2. Do exactly this work item. The smallest correct change wins: no scope creep, no drive-by refactors.
3. Verify your work: run the project's build and tests. Never leave the repo broken.
4. Never edit .crushrc, crush.json, or .tq-verify: they define your autonomy and your verify gate;
   changing them is self-dealing.
5. Close the loop in TODO_LIST.md: mark this item done ([x]) or remove it, following the file's own
   conventions. If you could NOT finish it, leave it unchecked and append " — BLOCKED: <one-line reason>".
6. Commit your changes with a clear message ending in this exact footer line (you have explicit
   permission to commit for this task):

   Task-Queue-ID: {{TASK_ID}}

   (so git log and the queue cross-reference). Never push.
7. End your final output with this exact one-line report so the queue can record what you did
   (fields optional):

TQ_RESULT: {"files_changed": ["path/of/changed/file.go"], "commit_sha": "the commit sha"}`

// Config controls one harvester.
type Config struct {
	// ProjectsDir is scanned (depth 1) for repos containing TodoFile.
	ProjectsDir string
	// DiscoveryAddr points at a project-discovery-daemon endpoint (unix
	// socket path, unix:// prefixed, or host:port) used INSTEAD of the
	// depth-1 ProjectsDir scan to enumerate candidate repos. Additive by
	// contract: an unreachable daemon logs one warning per tick and the
	// scan takes over. Empty keeps the zero-external-services default.
	DiscoveryAddr string
	// Log receives the daemon-discovery degradation warning; nil discards.
	Log *slog.Logger
	// Repos lists explicit repo directories; when set, ProjectsDir is ignored.
	Repos []string
	// Type is the task type enqueued (must match a registered executor).
	Type string
	// TodoFile is the backlog file name inside each repo.
	TodoFile string
	// MaxPerTick caps new enqueues per run across all repos.
	MaxPerTick int
	// MaxAttempts is the attempt budget for harvested tasks (0 = default).
	MaxAttempts int
	// Priority for harvested tasks.
	Priority int
	// PromptTemplate overrides DefaultPromptTemplate.
	PromptTemplate string
	// Model overrides the crush model ("provider/model") in every harvested
	// agent payload. Empty = the agent binary's default model.
	Model string
	// RepoIntervals sets a minimum gap between new enqueues per repo (by
	// repo name): after any enqueue for a repo, further items wait until
	// the interval has passed. Empty or zero entries = only the default
	// one-per-tick pacing applies.
	RepoIntervals map[string]time.Duration
	// RepoTimeouts sets a per-repo agent-task timeout ladder (by repo
	// name), pinned into each harvested payload's TimeoutMinutes: big
	// repos get long ceilings, quick ones stay tight. Repos without an
	// entry keep the executor's 30-minute default.
	RepoTimeouts map[string]time.Duration
	// DLQBackoff pauses harvesting of a repo whose recent work all went to
	// the dead-letter queue (dead tasks present, none completed, newest
	// dead within the window): the repo is poisoned until a human fixes or
	// rescues it. Zero disables the guard. Default 0 (set it, e.g. 30m).
	DLQBackoff time.Duration
	// RequireClean passes the clean-tree policy through to agent payloads:
	// nil = executor default (require a clean git tree), false lets agents
	// run on dirty repos (scratch/fixtures only), true forces the check.
	RequireClean *bool
	// DryRun reports what a real run would enqueue, without writing to the
	// queue. Result.Enqueued entries then carry an empty TaskID.
	DryRun bool
}

func (c Config) withDefaults() Config {
	if c.Type == "" {
		c.Type = DefaultType
	}

	if c.TodoFile == "" {
		c.TodoFile = DefaultTodoFile
	}

	if c.MaxPerTick <= 0 {
		c.MaxPerTick = DefaultMaxPerTick
	}

	if c.PromptTemplate == "" {
		c.PromptTemplate = DefaultPromptTemplate
	}

	return c
}

// Item is one checkbox found in a repo's todo file.
type Item struct {
	Repo     string // absolute repo path
	RepoName string // project name (repo dir base name)
	Heading  string // nearest markdown heading above the item
	Text     string // the checkbox text, trimmed
	Key      string // stable dedup key: hash of repo name + item text
	Done     bool   // true when the checkbox is ticked ([x])
}

// Enqueued records a task created (or already present) for an item.
type Enqueued struct {
	Item   Item
	TaskID task.ID
	Fresh  bool // true when this run actually created the task
}

// Skipped records an item deliberately not enqueued, and why. Reasons are
// stable strings meant for humans and tests.
type Skipped struct {
	Item   Item
	Reason string
}

// ReasonScanFailed prefixes every repo-level scan error in Skipped.Reason
// (the detail after the colon is the underlying error).
const ReasonScanFailed = "scan failed"

// Result summarizes one harvest run.
type Result struct {
	Repos    int
	Items    int
	Enqueued []Enqueued
	Skipped  []Skipped
}

// Harvester scans repos and enqueues agent tasks for open todo items.
type Harvester struct {
	cfg Config
	q   *queue.Queue
}

// New creates a Harvester over a queue.
func New(q *queue.Queue, cfg Config) *Harvester {
	return &Harvester{cfg: cfg.withDefaults(), q: q}
}

// Run performs one scan-and-enqueue pass. It never fails on individual repos;
// repo-level errors are reported as Skipped entries with the error as reason.
func (h *Harvester) Run(ctx context.Context) (Result, error) {
	res := Result{}

	repos := h.cfg.Repos
	if len(repos) == 0 {
		if h.cfg.ProjectsDir == "" {
			return res, ErrNoRepos
		}

		var err error

		repos, err = DiscoverReposFor(ctx, h.cfg.DiscoveryAddr, h.cfg.ProjectsDir, h.cfg.TodoFile, h.cfg.Log)
		if err != nil {
			return res, fmt.Errorf("harvest: discover repos: %w", err)
		}
	}

	sort.Strings(repos)

	for _, repo := range repos {
		res.Repos++

		items, err := ParseRepo(repo, h.cfg.TodoFile)
		if err != nil {
			res.Skipped = append(res.Skipped, Skipped{Reason: ReasonScanFailed + ": " + err.Error()})

			continue
		}

		res.Items += len(items)
		h.runRepo(ctx, repo, items, &res)
	}

	return res, nil
}

// runRepo applies the pacing rules for one repo:
//   - at most one non-terminal (pending/running) task per repo at any time,
//   - at most one NEW enqueue per repo per run,
//   - items already known to the queue (any status) are never re-enqueued,
//     relying on the store's DedupKey idempotency as the final guard.
func (h *Harvester) runRepo(ctx context.Context, repo string, items []Item, res *Result) {
	repoName := filepath.Base(repo)

	tasks, err := h.q.List(ctx, queue.Filter{Project: &repoName, Type: &h.cfg.Type})
	if err != nil {
		for _, it := range items {
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: "list failed: " + err.Error()})
		}

		return
	}

	busy := false
	known := make(map[string]task.Status, len(tasks))

	var (
		hasDead, hasCompleted bool
		lastDead, lastCreated time.Time
	)

	for _, t := range tasks {
		if t.Status == task.Pending || t.Status == task.Running {
			busy = true
		}

		switch t.Status {
		case task.Dead:
			hasDead = true

			if t.UpdatedAt.After(lastDead) {
				lastDead = t.UpdatedAt
			}
		case task.Completed:
			hasCompleted = true
		}

		if t.CreatedAt.After(lastCreated) {
			lastCreated = t.CreatedAt
		}

		if key := payloadDedup(t); key != "" {
			if _, dup := known[key]; !dup {
				known[key] = t.Status
			}
		}
	}
	// Poisoned repo: everything it touched recently is dead. New items
	// would die the same way — give the human the backoff window to fix
	// or rescue instead of enqueueing fresh failures every tick.
	poisoned := hasDead && !hasCompleted && h.cfg.DLQBackoff > 0 && time.Since(lastDead) < h.cfg.DLQBackoff
	repoInterval := h.cfg.RepoIntervals[repoName]

	enqueuedThisRepo := false

	for _, it := range items {
		if reason, blocked := blockedReason(it.Text); blocked {
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: "blocked: " + reason})

			continue
		}

		switch {
		case it.Key != "" && known[it.Key] != "":
			reason := "tracked: " + string(known[it.Key])
			switch task.Status(known[it.Key]) {
			case task.Dead:
				reason = "in DLQ (tq dlq --rescue to retry)"
			case task.Cancelled:
				reason = "cancelled (edit the item text to re-arm it)"
			}

			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: reason})
		case poisoned:
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: fmt.Sprintf(
				"poisoned: recent dead-letter, DLQ backoff %s (fix the repo or rescue dead tasks)", h.cfg.DLQBackoff)})
		case repoInterval > 0 && !lastCreated.IsZero() && time.Since(lastCreated) < repoInterval:
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: fmt.Sprintf(
				"paced: per-repo interval %s (last enqueue %s ago)",
				repoInterval,
				time.Since(lastCreated).Round(time.Second),
			)})
		case busy:
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: "repo busy: one agent per repo"})
		case enqueuedThisRepo:
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: "paced: one new item per repo per run"})
		case len(res.Enqueued) >= h.cfg.MaxPerTick:
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: "tick cap reached (--max-per-tick)"})
		case h.cfg.DryRun:
			res.Enqueued = append(res.Enqueued, Enqueued{Item: it, Fresh: true})
			known[it.Key] = task.Pending
			enqueuedThisRepo = true
		default:
			t, err := h.enqueue(ctx, it)
			if err != nil {
				res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: "enqueue failed: " + err.Error()})

				continue
			}

			known[it.Key] = task.Pending
			enqueuedThisRepo = true

			if t.Status == task.Pending && t.Attempts == 0 {
				res.Enqueued = append(res.Enqueued, Enqueued{Item: it, TaskID: t.ID, Fresh: true})
			} else {
				// Store dedup returned a pre-existing row (another pool won
				// the race). Count it as known, not fresh.
				res.Skipped = append(
					res.Skipped,
					Skipped{Item: it, Reason: "tracked: " + string(t.Status) + " (enqueued concurrently)"},
				)
			}
		}
	}
}

// blockedReason reports the "BLOCKED: <reason>" suffix that the agent
// contract (DefaultPromptTemplate) and the TODO_LIST header define for items
// a human must unblock; ok is true when the item must not be harvested.
func blockedReason(text string) (reason string, ok bool) {
	_, after, ok0 := strings.Cut(text, "BLOCKED:")
	if !ok0 {
		return "", false
	}

	reason = strings.TrimSpace(after)
	if reason == "" {
		reason = "no reason given"
	}

	return reason, true
}

func (h *Harvester) enqueue(ctx context.Context, it Item) (task.Task, error) {
	payload, err := h.buildPayload(it, h.cfg.PromptTemplate, it.Key)
	if err != nil {
		return task.Task{}, err
	}

	return h.q.Enqueue(ctx, task.New{
		Project:     it.RepoName,
		Type:        h.cfg.Type,
		Payload:     payload,
		Priority:    h.cfg.Priority,
		MaxAttempts: h.cfg.MaxAttempts,
		DedupKey:    it.Key,
	})
}

// buildPayload renders prompt for it and encodes it as the task payload with
// dedupKey pinned (the item's own key for normal tasks, catchup:<key> for
// loop-closing tasks). Repos discovered under ProjectsDir are named
// relatively so payloads stay valid when the projects root moves; explicit
// repos outside it keep their absolute path.
func (h *Harvester) buildPayload(it Item, prompt, dedupKey string) ([]byte, error) {
	prompt = strings.ReplaceAll(prompt, "{{REPO_ABS}}", it.Repo)
	prompt = strings.ReplaceAll(prompt, "{{REPO}}", it.RepoName)
	prompt = strings.ReplaceAll(prompt, "{{HEADING}}", it.Heading)
	prompt = strings.ReplaceAll(prompt, "{{ITEM}}", it.Text)

	repo := it.Repo
	if h.cfg.ProjectsDir != "" {
		if abs, err := filepath.Abs(
			h.cfg.ProjectsDir,
		); err == nil &&
			strings.HasPrefix(it.Repo, abs+string(filepath.Separator)) {
			repo = it.RepoName
		}
	}

	// Pin the repo's own verify command into the payload when it declares
	// one, so the task records what it will be gated by.
	payload := harvestPayload{
		AgentPayload: executor.AgentPayload{
			Repo:         repo,
			Prompt:       prompt,
			Item:         it.Text,
			Model:        h.cfg.Model,
			Verify:       executor.ReadTQVerify(it.Repo),
			RequireClean: h.cfg.RequireClean,
		},
		Dedup: dedupKey,
	}

	// Per-repo timeout ladder: pin the ceiling into the payload so the
	// executor honors it without knowing the harvester.
	if d, ok := h.cfg.RepoTimeouts[it.RepoName]; ok && d > 0 {
		payload.TimeoutMinutes = int(d / time.Minute)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("harvest: encode payload: %w", err)
	}

	return encoded, nil
}

// harvestPayload is the agent payload plus the harvester's dedup key. The
// agent executor ignores the extra field; the harvester reads it back to
// recognize its own tasks.
type harvestPayload struct {
	executor.AgentPayload

	Dedup string `json:"dedup,omitempty"`
}

// DiscoverRepos returns the depth-1 subdirectories of dir that contain the
// todo file, sorted by name.
func DiscoverRepos(dir, todoFile string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("discover repos under %s: %w", dir, err)
	}

	var repos []string

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		repo := filepath.Join(dir, e.Name())
		if info, err := os.Stat(filepath.Join(repo, todoFile)); err == nil && !info.IsDir() {
			repos = append(repos, repo)
		}
	}

	return repos, nil
}

// ParseRepo extracts the open checkbox items from a repo's todo file.
//
// Semantics: a line matching `- [ ]`/`* [ ]` (leading whitespace allowed) is
// an item; `- [x]`/`- [X]` are done and ignored (ParseRepoAll returns them
// with Done set). The nearest heading above the item (any level, text without
// the #s) is its Heading. Lines inside fenced code blocks are ignored, so
// TODO examples in documentation are never harvested. The Key changes when
// the item text changes, so editing an item re-arms it even if a previous
// task for the old wording exists.
func ParseRepo(repo, todoFile string) ([]Item, error) {
	all, err := ParseRepoAll(repo, todoFile)
	if err != nil {
		return nil, err
	}

	open := all[:0]
	for _, it := range all {
		if !it.Done {
			open = append(open, it)
		}
	}

	return open, nil
}

// ParseRepoAll returns every checkbox item (open and done) from a repo's
// todo file; Item.Done distinguishes them. It backs the drift auditor, which
// compares BOTH checkbox states against terminal task states.
func ParseRepoAll(repo, todoFile string) ([]Item, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return nil, fmt.Errorf("resolve repo path %s: %w", repo, err)
	}

	data, err := os.ReadFile(filepath.Join(abs, todoFile))
	if err != nil {
		return nil, fmt.Errorf("read todo file %s: %w", todoFile, err)
	}

	repoName := filepath.Base(abs)

	var items []Item

	heading := ""
	inFence := false

	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence

			continue
		}

		if inFence {
			continue
		}

		if h, ok := headingOf(trimmed); ok {
			heading = h

			continue
		}

		if text, done, ok := checkboxOf(trimmed); ok {
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}

			items = append(items, Item{
				Repo:     abs,
				RepoName: repoName,
				Heading:  heading,
				Text:     text,
				Key:      ItemKey(repoName, text),
				Done:     done,
			})
		}
	}

	return items, nil
}

// ItemKey derives the stable dedup key for an item: the hash of repo name and
// item text. Item text is whitespace-collapsed so reflowing a line does not
// create duplicate work.
func ItemKey(repoName, text string) string {
	collapsed := strings.Join(strings.Fields(text), " ")
	sum := sha256.Sum256([]byte(repoName + "\x00" + collapsed))

	return "todo:" + hex.EncodeToString(sum[:])[:16]
}

// headingOf returns the heading text of a markdown heading line, if it is one.
func headingOf(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}

	text := strings.TrimLeft(line, "#")

	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}

	return text, true
}

// checkboxOf returns the text of a markdown checkbox line and whether it is
// ticked. Both `- [ ] text` and `- [x] text` (any case, `*` bullets too)
// count; the boolean ok reports that the line is a checkbox at all.
func checkboxOf(line string) (text string, done bool, ok bool) {
	rest, isBullet := strings.CutPrefix(line, "-")
	if !isBullet {
		if rest, isBullet = strings.CutPrefix(line, "*"); !isBullet {
			return "", false, false
		}
	}

	rest = strings.TrimSpace(rest)
	if unticked, is := strings.CutPrefix(rest, "[ ]"); is {
		return unticked, false, true
	}

	if inner, is := strings.CutPrefix(rest, "["); is {
		if inner != "" && (inner[0] == 'x' || inner[0] == 'X') {
			if ticked, closed := strings.CutPrefix(inner[1:], "]"); closed {
				return ticked, true, true
			}
		}
	}

	return "", false, false
}

// payloadDedup extracts the "dedup" field from a task payload, if present.
func payloadDedup(t task.Task) string {
	if len(t.Payload) == 0 {
		return ""
	}

	var p struct {
		Dedup string `json:"dedup"`
	}
	if err := json.Unmarshal(t.Payload, &p); err != nil {
		return ""
	}

	return p.Dedup
}
