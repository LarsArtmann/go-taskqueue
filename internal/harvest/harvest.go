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
	"strconv"
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
Your repo's AGENTS.md is already in your context — follow it.

Repository: {{REPO_ABS}}
Work item from TODO_LIST.md, section "{{HEADING}}":
"{{ITEM}}"

Contract:
1. Do exactly this work item. The smallest correct change wins: no scope creep, no drive-by
   refactors. Verify your work: run the project's build and tests. Never leave the repo broken.
2. Never edit .crushrc, crush.json, or .tq-verify: they define your autonomy and your verify gate;
   changing them is self-dealing.
3. Close the loop in TODO_LIST.md: mark this item done ([x]) or remove it, following the file's own
   conventions. If you could NOT finish it, leave it unchecked and append " — BLOCKED: <one-line reason>".
4. Commit your changes with a clear message ending in this exact footer line (you have explicit
   permission to commit for this task):

   Task-Queue-ID: {{TASK_ID}}

   (so git log and the queue cross-reference). Never push.
5. Follow-up work you discover belongs in the backlog, not this run: you MAY append NEW unchecked
   items to TODO_LIST.md (one per line, agent-executable, correct section) — the queue's pacing,
   budget and priority gates decide when they run. Never append an item describing THIS task's work.

The queue derives what you did — commits via the footer above, changed files via git — so do NOT
report files or commit SHAs yourself.`

// DefaultBatchPromptTemplate is the agent contract for BATCHED work items
// (harvest --batch-items > 1): one task carries a run of adjacent items from
// the same TODO_LIST.md section, worked in order by ONE session. Placeholders:
// {{REPO_ABS}}, {{REPO}}, {{HEADING}}, {{COUNT}}, {{ITEMS}} (numbered list)
// and {{TASK_ID}} (resolved at EXECUTION time, one footer for every commit
// of the batch — derived outcomes attribute them all to this task).
const DefaultBatchPromptTemplate = `You are an autonomous agent working from a shared task queue, unsupervised.
Your repo's AGENTS.md is already in your context — follow it.

Repository: {{REPO_ABS}}
Work batch from TODO_LIST.md, section "{{HEADING}}" — {{COUNT}} related items, ONE session:

{{ITEMS}}

Contract:
1. Work the items IN ORDER as one batch: you already hold the repo context from earlier items —
   use it. The smallest correct change per item wins: no scope creep, no drive-by refactors.
   An item already ticked [x] is done (an earlier attempt may have finished it) — skip it.
   Verify as you go: run the project's build and tests. Never leave the repo broken.
2. Never edit .crushrc, crush.json, or .tq-verify: they define your autonomy and your verify gate;
   changing them is self-dealing.
3. Close the loop per item in TODO_LIST.md: mark each finished item done ([x]) or remove it,
   following the file's own conventions. An item you could NOT finish stays unchecked with
   " — BLOCKED: <one-line reason>" appended; finish the remaining items anyway — partial
   completion with a green verify gate is a SUCCESS for this task.
4. Commit each item separately with a clear message ending in this exact footer line (you have
   explicit permission to commit for this task):

   Task-Queue-ID: {{TASK_ID}}

   (so git log and the queue cross-reference). Never push.
5. Follow-up work you discover belongs in the backlog, not this run: you MAY append NEW unchecked
   items to TODO_LIST.md (one per line, agent-executable, correct section) — the queue's pacing,
   budget and priority gates decide when they run. Never append an item describing THIS batch's work.

The queue derives what you did — commits via the footer above, changed files via git — so do NOT
report files or commit SHAs yourself.`

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
	// SameSessionPriority promotes items whose text references /tmp paths
	// to this priority at enqueue time ("hot": work the same session, the
	// files will not survive). Zero disables the promotion.
	SameSessionPriority int
	// UseImportance resolves harvested priorities from the repo's
	// .config/metadata.yaml importance (default 50) plus keyword bumps,
	// clamped to the backlog band (ADR-0015 §3). Markers and hot promotion
	// apply regardless of this flag; a malformed metadata file skips the
	// repo's items with a reason instead of guessing.
	UseImportance bool
	// MaxPendingPerRepo caps how many PENDING tasks one repo may hold in
	// the queue (0 = unlimited): the queue is the WORKING SET, TODO_LIST.md
	// is the warehouse — prompts stay fresh and AI cost stays O(working
	// set). Admission resumes as claims drain the slot; running tasks do
	// not count (the repo-busy rule owns those).
	MaxPendingPerRepo int
	// PromptTemplate overrides DefaultPromptTemplate.
	PromptTemplate string
	// BatchPromptTemplate overrides DefaultBatchPromptTemplate (used only
	// when BatchItems > 1).
	BatchPromptTemplate string
	// BatchItems groups up to this many ADJACENT open items from the same
	// TODO_LIST.md section into ONE agent task (one session works the run
	// in order): fewer cold sessions, more done per provider window.
	// 0/1 = off (one item per task, the fleet default). A batch is ONE task
	// against every gate (--max-per-tick, daily budget, repo pacing) — the
	// per-item cost is amortized, so raise the gates consciously.
	BatchItems int
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

	if c.BatchPromptTemplate == "" {
		c.BatchPromptTemplate = DefaultBatchPromptTemplate
	}

	if c.BatchItems < 1 {
		c.BatchItems = 1
	}

	return c
}

// Item is one checkbox found in a repo's todo file.
type Item struct {
	Repo     string // absolute repo path
	RepoName string // project name (repo dir base name)
	Heading  string // nearest markdown heading above the item
	Text     string // the checkbox text, trimmed, priority marker stripped
	Key      string // stable dedup key: hash of repo name + MARKER-STRIPPED text
	Done     bool   // true when the checkbox is ticked ([x])
	// MarkerLevel is the parsed trailing `— P[1-4]` marker (1–4); 0 = none.
	// Stripped from Text before hashing so editing a marker never forks a
	// task (ADR-0015 §2). omitempty keeps legacy JSON consumers and golden
	// fixtures byte-stable for unmarked items.
	MarkerLevel int `json:"markerLevel,omitempty"`
}

// Enqueued records a task created (or already present) for an item.
type Enqueued struct {
	Item   Item
	TaskID task.ID
	Fresh  bool // true when this run actually created the task
	Hot    bool // text referenced /tmp: enqueued at SameSessionPriority
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

// resolveRepos returns the sweep's repo list: the configured Repos, or —
// when unset — depth-1 discovery over ProjectsDir. The list comes back
// sorted. Audit and PruneStale share it; Run resolves through the
// daemon-aware DiscoverReposFor instead (offline vs live-tick divergence).
func (h *Harvester) resolveRepos() ([]string, error) {
	repos := h.cfg.Repos
	if len(repos) == 0 {
		if h.cfg.ProjectsDir == "" {
			return nil, ErrNoRepos
		}

		var err error

		repos, err = DiscoverRepos(h.cfg.ProjectsDir, h.cfg.TodoFile)
		if err != nil {
			return nil, fmt.Errorf("harvest: discover repos: %w", err)
		}
	}

	sort.Strings(repos)

	return repos, nil
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
	state, ok := h.surveyRepo(ctx, repo, items, res)
	if !ok {
		return
	}

	enqueuedThisRepo := false

	if h.cfg.BatchItems > 1 {
		h.runRepoBatched(ctx, state, items, res)

		return
	}

	for _, item := range items {
		reason := h.itemDenial(state, item, enqueuedThisRepo, len(res.Enqueued))
		if reason != "" {
			res.Skipped = append(res.Skipped, Skipped{Item: item, Reason: reason})

			continue
		}

		if h.cfg.DryRun {
			res.Enqueued = append(res.Enqueued, Enqueued{Item: item, Fresh: true, Hot: sameSession(item.Text)})
			state.known[item.Key] = task.Pending
			enqueuedThisRepo = true

			continue
		}

		if h.admitItem(ctx, item, state.importance, res) {
			state.known[item.Key] = task.Pending
			enqueuedThisRepo = true
		}
	}
}

// runRepoBatched is the batched admission path (Config.BatchItems > 1):
// runs of consecutive ADMISSIBLE items from the same TODO_LIST section
// become ONE agent task — one session works the run in order. Inadmissible
// items (blocked, known, paused, poisoned, occupied) are skipped with their
// own reasons and BREAK a run: a batch never mixes items the single-item
// path would have skipped. The first admitted run consumes the repo's
// one-new-task slot exactly like a single enqueue; later runs are paced
// out with the same reason strings the single path uses.
func (h *Harvester) runRepoBatched(ctx context.Context, state repoState, items []Item, res *Result) {
	// Phase 1: base denial per item, pacing EXCLUDED (pacing applies at run
	// granularity below). Denied items are reported and break runs.
	admissible := make([]bool, len(items))

	for i, item := range items {
		if reason := h.itemDenial(state, item, false, len(res.Enqueued)); reason != "" {
			res.Skipped = append(res.Skipped, Skipped{Item: item, Reason: reason})

			continue
		}

		admissible[i] = true
	}

	// Phase 2: walk runs of consecutive admissible same-heading items.
	enqueuedThisRepo := false

	for i := 0; i < len(items); {
		if !admissible[i] {
			i++

			continue
		}

		end := i + 1
		for end < len(items) &&
			admissible[end] &&
			items[end].Heading == items[i].Heading &&
			end-i < h.cfg.BatchItems {
			end++
		}

		run := items[i:end]
		i = end

		switch {
		case enqueuedThisRepo:
			h.skipRun(run, res, "paced: one new item per repo per run")
		case len(res.Enqueued) >= h.cfg.MaxPerTick:
			h.skipRun(run, res, "tick cap reached (--max-per-tick)")
		default:
			if h.admitRun(ctx, run, state.importance, res) {
				enqueuedThisRepo = true

				for _, item := range run {
					state.known[item.Key] = task.Pending
				}
			}
		}
	}
}

// skipRun reports every member of a paced-out run with one reason.
func (h *Harvester) skipRun(run []Item, res *Result, reason string) {
	for _, item := range run {
		res.Skipped = append(res.Skipped, Skipped{Item: item, Reason: reason})
	}
}

// admitRun enqueues one batched run and records every member under
// res.Enqueued with the SAME TaskID (one task, many items). Failure and
// store-dedup semantics mirror admitItem, reported per member.
func (h *Harvester) admitRun(ctx context.Context, run []Item, importance int, res *Result) bool {
	if h.cfg.DryRun {
		for _, item := range run {
			res.Enqueued = append(res.Enqueued, Enqueued{Item: item, Fresh: true, Hot: sameSession(item.Text)})
		}

		return true
	}

	t, err := h.enqueueBatch(ctx, run, importance)
	if err != nil {
		for _, item := range run {
			res.Skipped = append(res.Skipped, Skipped{Item: item, Reason: "enqueue failed: " + err.Error()})
		}

		return false
	}

	fresh := t.Status == task.Pending && t.Attempts == 0
	for _, item := range run {
		if fresh {
			res.Enqueued = append(
				res.Enqueued,
				Enqueued{Item: item, TaskID: t.ID, Fresh: true, Hot: sameSession(item.Text)},
			)

			continue
		}

		res.Skipped = append(
			res.Skipped,
			Skipped{Item: item, Reason: "tracked: " + string(t.Status) + " (enqueued concurrently)"},
		)
	}

	return true
}

// batchKeyOf derives the deterministic batch dedup key over a run's member
// keys: same member set → same key (an unchanged set never re-mints); any
// member edit forks the batch, mirroring single-item text-edit semantics.
// Sorted, so reordering the same items in the file does not fork the batch
// (order is cosmetic; the item set is the work).
func batchKeyOf(run []Item) string {
	keys := make([]string, len(run))
	for i, item := range run {
		keys[i] = item.Key
	}

	sort.Strings(keys)
	sum := sha256.Sum256([]byte(strings.Join(keys, "\n")))

	return "batch:" + hex.EncodeToString(sum[:])[:16]
}

// BatchKeyPrefix marks a batched task's dedup key (harvest --batch-items).
const BatchKeyPrefix = "batch:"

func (h *Harvester) enqueueBatch(ctx context.Context, run []Item, importance int) (task.Task, error) {
	payload, err := h.buildBatchPayload(ctx, run)
	if err != nil {
		return task.Task{}, err
	}

	// Batch priority: the MAX over the members' resolved priorities — a
	// batch carrying one hot or marker item must not sink below it, and a
	// batch of P4s must not ride a P1 sibling's rank.
	priority := 0

	for _, item := range run {
		var aiScore *int

		if score, ok, err := h.q.PriorityScore(ctx, item.Key); err == nil && ok {
			clamped := queue.ClampBacklog(score.Score)
			aiScore = &clamped
		}

		itemPriority, _ := ResolvePriority(ResolveInput{
			Text:              item.Text,
			MarkerLevel:       item.MarkerLevel,
			HotPriority:       h.cfg.SameSessionPriority,
			FlatPriority:      h.cfg.Priority,
			Importance:        importance,
			ImportanceEnabled: h.cfg.UseImportance,
			AIScore:           aiScore,
		})

		priority = max(priority, itemPriority)
	}

	return h.q.Enqueue(ctx, task.New{
		Project:     run[0].RepoName,
		Type:        h.cfg.Type,
		Payload:     payload,
		Priority:    priority,
		MaxAttempts: h.cfg.MaxAttempts,
		DedupKey:    batchKeyOf(run),
	})
}

// defaultBatchTimeoutMinutes is the per-item ceiling a batch scales from
// when the repo timeout ladder has no entry (executor default: 30).
const defaultBatchTimeoutMinutes = 30

// buildBatchPayload renders the batch prompt for one run and encodes the
// members' texts and keys into the payload (Item stays the FIRST member so
// review quoting, status windows and `tq show` provenance keep working).
// The payload timeout scales with the member count — one ladder/default
// ceiling per item — so a batch is not killed by a single-item ceiling;
// the pool's --task-timeout stays the hard cap above it.
func (h *Harvester) buildBatchPayload(ctx context.Context, run []Item) ([]byte, error) {
	first := run[0]

	texts := make([]string, len(run))
	keys := make([]string, len(run))
	maxMarker := 0

	for i, item := range run {
		texts[i] = item.Text
		keys[i] = item.Key
		maxMarker = max(maxMarker, item.MarkerLevel)
	}

	var list strings.Builder
	for i, text := range texts {
		fmt.Fprintf(&list, "%d. %s\n", i+1, text)
	}

	prompt := strings.ReplaceAll(h.cfg.BatchPromptTemplate, "{{REPO_ABS}}", first.Repo)
	prompt = strings.ReplaceAll(prompt, "{{REPO}}", first.RepoName)
	prompt = strings.ReplaceAll(prompt, "{{HEADING}}", first.Heading)
	prompt = strings.ReplaceAll(prompt, "{{COUNT}}", strconv.Itoa(len(run)))
	prompt = strings.ReplaceAll(prompt, "{{ITEMS}}", strings.TrimRight(list.String(), "\n"))

	if dangles := danglingSHAs(ctx, first.Repo, strings.Join(texts, "\n")); len(dangles) > 0 {
		prompt += "\n\n" + citationBlock(dangles)
	}

	repo := first.Repo
	if h.cfg.ProjectsDir != "" {
		if abs, err := filepath.Abs(
			h.cfg.ProjectsDir,
		); err == nil &&
			strings.HasPrefix(first.Repo, abs+string(filepath.Separator)) {
			repo = first.RepoName
		}
	}

	perItemMinutes := defaultBatchTimeoutMinutes
	if d, ok := h.cfg.RepoTimeouts[first.RepoName]; ok && d > 0 {
		perItemMinutes = int(d / time.Minute)
	}

	payload := harvestPayload{
		Repo:         repo,
		Prompt:       prompt,
		Item:         first.Text,
		Items:        texts,
		Model:        h.cfg.Model,
		Verify:       executor.ReadTQVerify(first.Repo),
		RequireClean: h.cfg.RequireClean,

		TimeoutMinutes: perItemMinutes * len(run),
		Dedup:          batchKeyOf(run),
		ItemKeys:       keys,
		MarkerLevel:    maxMarker,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("harvest: encode batch payload: %w", err)
	}

	return encoded, nil
}

// repoState is one repo's queue-side situation, surveyed once per run: the
// known dedup keys with their statuses, whether a non-terminal task holds
// the repo busy, the poisoned-repo verdict, the pacing timestamps, and
// (with UseImportance) the repo's metadata importance.
type repoState struct {
	repoName     string
	busy         bool
	anyRunning   bool
	pendingCount int
	known        map[string]task.Status
	poisoned     bool
	repoInterval time.Duration
	lastCreated  time.Time
	importance   int
}

// surveyRepo lists the repo's tasks and folds them into a repoState. ok is
// false when the listing failed (every item is reported skipped, mirroring
// the ReasonScanFailed pattern).
func (h *Harvester) surveyRepo(ctx context.Context, repo string, items []Item, res *Result) (repoState, bool) {
	repoName := filepath.Base(repo)
	state := repoState{
		repoName:     repoName,
		known:        make(map[string]task.Status),
		repoInterval: h.cfg.RepoIntervals[repoName],
	}

	tasks, err := h.q.List(ctx, queue.Filter{Project: &repoName, Type: &h.cfg.Type})
	if err != nil {
		for _, item := range items {
			res.Skipped = append(res.Skipped, Skipped{Item: item, Reason: "list failed: " + err.Error()})
		}

		return state, false
	}

	var (
		hasDead, hasCompleted bool
		lastDead              time.Time
	)

	for _, t := range tasks {
		state.observe(t)

		if t.Status == task.Dead {
			hasDead = true

			if t.UpdatedAt.After(lastDead) {
				lastDead = t.UpdatedAt
			}
		}

		if t.Status == task.Completed {
			hasCompleted = true
		}
	}

	// Poisoned repo: everything item touched recently is dead. New items
	// would die the same way — give the human the backoff window to fix
	// or rescue instead of enqueueing fresh failures every tick.
	poisoned := hasDead && !hasCompleted && h.cfg.DLQBackoff > 0 && time.Since(lastDead) < h.cfg.DLQBackoff
	state.poisoned = poisoned

	// Importance mode: read the repo's metadata once per run. A malformed
	// file skips the whole repo — an importance the owner DID set must
	// never silently degrade to the default (ADR-0015 §7).
	if h.cfg.UseImportance {
		importance, err := ReadImportance(repo)
		if err != nil {
			skipAll(res, items, "metadata: "+err.Error())

			return state, false
		}

		state.importance = importance
	} else {
		state.importance = DefaultImportance
	}

	return state, true
}

// skipAll reports every item of a repo as skipped with one reason.
func skipAll(res *Result, items []Item, reason string) {
	for _, item := range items {
		res.Skipped = append(res.Skipped, Skipped{Item: item, Reason: reason})
	}
}

// itemDenial reports why one item must NOT be enqueued this run, in the
// pacing precedence order; "" means the item is admissible (subject to
// DryRun, which never denies but never enqueues either).
func (h *Harvester) itemDenial(state repoState, item Item, enqueuedThisRepo bool, enqueuedThisTick int) string {
	if reason, blocked := blockedReason(item.Text); blocked {
		return "blocked: " + reason
	}

	occupancy := h.occupancyDenial(state)
	stateReason := h.stateDenial(state)

	switch {
	case item.Key != "" && state.known[item.Key] != "":
		return trackedItemDenial(state.known[item.Key])
	case stateReason != "":
		return stateReason
	case occupancy != "":
		return occupancy
	case enqueuedThisRepo:
		return "paced: one new item per repo per run"
	case enqueuedThisTick >= h.cfg.MaxPerTick:
		return "tick cap reached (--max-per-tick)"
	}

	return ""
}

// observe folds one task into the repo state: occupancy counters (busy,
// running, pending), the newest creation time, and the dedup-key index.
// Dead-letter recency and completions stay with the caller — they feed the
// poison check.
func (state *repoState) observe(t task.Task) {
	switch t.Status {
	case task.Pending:
		state.busy = true
		state.pendingCount++
	case task.Running:
		state.busy = true
		state.anyRunning = true
	case task.Completed, task.Dead, task.Cancelled:
		// No occupancy effect: lifecycle signals are derived by the caller.
	}

	if t.CreatedAt.After(state.lastCreated) {
		state.lastCreated = t.CreatedAt
	}

	if key := payloadDedup(t); key != "" {
		if _, dup := state.known[key]; !dup {
			state.known[key] = t.Status
		}

		// Batched tasks track their MEMBERS too: next tick must not re-attempt
		// items already inside a pending batch (the store dedup would catch
		// it, but the survey should not even try).
		for _, member := range payloadItemKeys(t) {
			if _, dup := state.known[member]; !dup {
				state.known[member] = t.Status
			}
		}
	}
}

// stateDenial reports the repo-lifecycle admission stops: paused
// (importance 0 in importance mode), poisoned (recent dead-letters inside
// the backoff window), or paced (per-repo enqueue interval not elapsed).
func (h *Harvester) stateDenial(state repoState) string {
	if h.cfg.UseImportance && state.importance == 0 {
		return "paused: importance 0 (repo paused from auto-admission; raise importance to resume)"
	}

	if state.poisoned {
		return fmt.Sprintf(
			"poisoned: recent dead-letter, DLQ backoff %s (fix the repo or rescue dead tasks)", h.cfg.DLQBackoff)
	}

	if state.repoInterval > 0 && !state.lastCreated.IsZero() && time.Since(state.lastCreated) < state.repoInterval {
		return fmt.Sprintf(
			"paced: per-repo interval %s (last enqueue %s ago)",
			state.repoInterval,
			time.Since(state.lastCreated).Round(time.Second),
		)
	}

	return ""
}

// trackedItemDenial explains why an item already known to the queue is
// denied admission, by its stored status.
func trackedItemDenial(status task.Status) string {
	if status == task.Dead {
		return "in DLQ (tq dlq --rescue to retry)"
	}

	if status == task.Cancelled {
		return "cancelled (edit the item text to re-arm item)"
	}

	return "tracked: " + string(status)
}

// occupancyDenial reports the repo-level occupancy rule. With
// MaxPendingPerRepo > 0 the working-set cap REPLACES the legacy
// any-pending-is-busy rule: a repo may hold up to the cap in PENDING
// (queue = working set, ADR-0015 context) while a RUNNING task still
// always denies — one agent executing per repo. Without the knob the
// legacy rule stands: any pending or running task holds the repo.
func (h *Harvester) occupancyDenial(state repoState) string {
	if h.cfg.MaxPendingPerRepo > 0 {
		if state.anyRunning {
			return "repo busy: one agent per repo"
		}

		if state.pendingCount >= h.cfg.MaxPendingPerRepo {
			return fmt.Sprintf(
				"admission: repo holds %d pending task(s), cap %d (--max-pending-per-repo; "+
					"queue = working set, TODO_LIST.md = warehouse)",
				state.pendingCount, h.cfg.MaxPendingPerRepo)
		}

		return ""
	}

	if state.busy {
		return "repo busy: one agent per repo"
	}

	return ""
}

// admitItem enqueues one item and records the outcome: a fresh task under
// res.Enqueued, a store-dedup return (another pool won the race) under
// res.Skipped. ok is false only when the enqueue FAILED — a dedup return
// still counts against pacing exactly like the original inline code.
func (h *Harvester) admitItem(ctx context.Context, item Item, importance int, res *Result) bool {
	t, err := h.enqueue(ctx, item, importance)
	if err != nil {
		res.Skipped = append(res.Skipped, Skipped{Item: item, Reason: "enqueue failed: " + err.Error()})

		return false
	}

	if t.Status == task.Pending && t.Attempts == 0 {
		res.Enqueued = append(
			res.Enqueued,
			Enqueued{Item: item, TaskID: t.ID, Fresh: true, Hot: sameSession(item.Text)},
		)

		return true
	}

	// Store dedup returned a pre-existing row (another pool won the race).
	// Count item as known, not fresh — but still consumed this run's
	// one-new-item slot, matching the pre-refactor pacing behavior.
	res.Skipped = append(
		res.Skipped,
		Skipped{Item: item, Reason: "tracked: " + string(t.Status) + " (enqueued concurrently)"},
	)

	return true
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

// sameSession reports whether the item text references a /tmp path, the
// signal for same-session priority promotion.
func sameSession(text string) bool {
	return strings.Contains(text, "/tmp")
}

func (h *Harvester) enqueue(ctx context.Context, item Item, importance int) (task.Task, error) {
	payload, err := h.buildPayload(ctx, item, h.cfg.PromptTemplate, item.Key)
	if err != nil {
		return task.Task{}, err
	}

	// Effective priority resolves the ADR-0015 §3 precedence ladder: hot >
	// marker > cached AI score > importance + keyword bumps > flat. The
	// cache lookup keys on the item's dedup key, so a verdict survives
	// until the text changes.
	var aiScore *int

	if score, ok, err := h.q.PriorityScore(ctx, item.Key); err == nil && ok {
		clamped := queue.ClampBacklog(score.Score)
		aiScore = &clamped
	}

	priority, _ := ResolvePriority(ResolveInput{
		Text:              item.Text,
		MarkerLevel:       item.MarkerLevel,
		HotPriority:       h.cfg.SameSessionPriority,
		FlatPriority:      h.cfg.Priority,
		Importance:        importance,
		ImportanceEnabled: h.cfg.UseImportance,
		AIScore:           aiScore,
	})

	return h.q.Enqueue(ctx, task.New{
		Project:     item.RepoName,
		Type:        h.cfg.Type,
		Payload:     payload,
		Priority:    priority,
		MaxAttempts: h.cfg.MaxAttempts,
		DedupKey:    item.Key,
	})
}

// buildPayload renders prompt for item and encodes item as the task payload with
// dedupKey pinned (the item's own key for normal tasks, catchup:<key> for
// loop-closing tasks). Repos discovered under ProjectsDir are named
// relatively so payloads stay valid when the projects root moves; explicit
// repos outside item keep their absolute path.
func (h *Harvester) buildPayload(ctx context.Context, item Item, prompt, dedupKey string) ([]byte, error) {
	prompt = strings.ReplaceAll(prompt, "{{REPO_ABS}}", item.Repo)
	prompt = strings.ReplaceAll(prompt, "{{REPO}}", item.RepoName)
	prompt = strings.ReplaceAll(prompt, "{{HEADING}}", item.Heading)
	prompt = strings.ReplaceAll(prompt, "{{ITEM}}", item.Text)

	if dangles := danglingSHAs(ctx, item.Repo, item.Text); len(dangles) > 0 {
		prompt += "\n\n" + citationBlock(dangles)
	}

	repo := item.Repo
	if h.cfg.ProjectsDir != "" {
		if abs, err := filepath.Abs(
			h.cfg.ProjectsDir,
		); err == nil &&
			strings.HasPrefix(item.Repo, abs+string(filepath.Separator)) {
			repo = item.RepoName
		}
	}

	// Pin the repo's own verify command into the payload when item declares
	// one, so the task records what item will be gated by.
	payload := harvestPayload{
		Repo:         repo,
		Prompt:       prompt,
		Item:         item.Text,
		Model:        h.cfg.Model,
		Verify:       executor.ReadTQVerify(item.Repo),
		RequireClean: h.cfg.RequireClean,
		Dedup:        dedupKey,
	}

	// Per-repo timeout ladder: pin the ceiling into the payload so the
	// executor honors item without knowing the harvester.
	if d, ok := h.cfg.RepoTimeouts[item.RepoName]; ok && d > 0 {
		payload.TimeoutMinutes = int(d / time.Minute)
	}

	// Pin the marker level so later automated re-resolution (the
	// prioritize sweeper's apply step) can honor marker > AI precedence
	// from the payload alone, without rescanning the file.
	payload.MarkerLevel = item.MarkerLevel

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("harvest: encode payload: %w", err)
	}

	return encoded, nil
}

// harvestPayload is the agent payload plus the harvester's dedup key. The
// agent executor ignores the extra fields; the harvester (and the
// prioritize sweeper) read them back to recognize harvest-minted tasks.
type harvestPayload struct {
	executor.AgentPayload

	Dedup       string   `json:"dedup,omitempty"`
	MarkerLevel int      `json:"markerLevel,omitempty"`
	ItemKeys    []string `json:"itemKeys,omitempty"`
}

// PayloadItem is the harvested backlog-item identity carried by a task
// payload: what was harvested, from where, under which dedup key.
type PayloadItem struct {
	// Repo is the payload's AgentPayload.Repo — the resolution-ready
	// reference the agent executor itself uses.
	Repo string
	// Text is the marker-stripped item text.
	Text string
	// Key is the item's dedup key (ItemKey derivation) — the score-cache
	// key.
	Key string
	// MarkerLevel is 0 (no marker) or 1-4 when the item carried a
	// `— P[1-4]` marker at harvest time.
	MarkerLevel int
}

// PayloadItemOf reads one task's payload back into its harvested item
// identity. ok is false for non-harvest payloads (foreign agent tasks,
// review/status/dlqfix mints): only harvest-minted tasks carry the
// "dedup" key.
func PayloadItemOf(t task.Task) (PayloadItem, bool) {
	if len(t.Payload) == 0 {
		return PayloadItem{}, false
	}

	var payload harvestPayload
	if err := json.Unmarshal(t.Payload, &payload); err != nil {
		return PayloadItem{}, false
	}

	if payload.Dedup == "" || payload.Item == "" || payload.Repo == "" {
		return PayloadItem{}, false
	}

	return PayloadItem{
		Repo:        payload.Repo,
		Text:        payload.Item,
		Key:         payload.Dedup,
		MarkerLevel: payload.MarkerLevel,
	}, true
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

			// Strip the priority marker before the item exists at all: Text,
			// Key, and the payload all carry the marker-free text, so editing
			// `— P1` to `— P2` re-derives the SAME dedup key (ADR-0015 §2).
			level := 0
			if l, stripped, ok := SplitMarker(text); ok {
				level, text = l, strings.TrimSpace(stripped)
			}

			items = append(items, Item{
				Repo:        abs,
				RepoName:    repoName,
				Heading:     heading,
				Text:        text,
				Key:         ItemKey(repoName, text),
				Done:        done,
				MarkerLevel: level,
			})
		} else if reason := damagedCheckbox(trimmed); reason != "" {
			// A damaged checkbox shape must be LOUD (04-46 §d4/§f1): the
			// 04-40 close-out shipped a backlog row as `--- [ ] …` and both
			// consumers (this parser, check-todo-list.sh) stayed silent —
			// the row was invisible to the pool until caught by eye. Reject
			// the file instead of skipping the line.
			return nil, fmt.Errorf("%s: %s; a checkbox line must start with exactly `- [ ] ` or `- [x] `", todoFile, reason)
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

// damagedCheckbox reports a line that LOOKS like an attempted checkbox but
// has a malformed bullet run — e.g. `--- [ ] text`, `* - [x] text` — where
// bullets/dashes (plus optional spaces) run into a `[ ]`/`[x]`/`[X]`
// bracket instead of the exact `- [ ] ` prefix. Only consulted when
// checkboxOf rejected the line, so well-formed shapes never reach it;
// plain prose bullets (`- see [x] below`) stop at the first non-bullet
// character and never match. The 04-40 close-out shipped a backlog row in
// the `--- [ ]` shape and it was silently invisible to every consumer
// (04-46 §d4/§f1) — this is the runtime half of making that loud.
func damagedCheckbox(line string) string {
	// A single bullet (dash or star) followed by optional spaces and the
	// bracket is a well-formed shape — checkboxOf parses it with or without
	// the space — so it is never damaged, even though a naive bullet-run
	// scan would land on the bracket.
	if len(line) > 0 && (line[0] == '-' || line[0] == '*') {
		if rest := strings.TrimLeft(line[1:], " \t"); len(rest) > 0 && rest[0] == '[' {
			return ""
		}
	}

	rest := line
	for {
		next := strings.TrimLeft(rest, "-* \t")
		if next == rest {
			break
		}

		rest = next
	}

	if len(rest) < 3 || rest[0] != '[' {
		return ""
	}

	if (rest[1] == ' ' || rest[1] == 'x' || rest[1] == 'X') && rest[2] == ']' {
		return fmt.Sprintf("checkbox line has a malformed bullet: %q", line)
	}

	return ""
}

// payloadItemKeys extracts the batch member keys ("itemKeys") from a task
// payload; empty for single-item and non-harvest payloads.
func payloadItemKeys(t task.Task) []string {
	if len(t.Payload) == 0 {
		return nil
	}

	var payload harvestPayload
	if err := json.Unmarshal(t.Payload, &payload); err != nil {
		return nil
	}

	return payload.ItemKeys
}

// payloadDedup extracts the "dedup" field from a task payload, if present.
func payloadDedup(t task.Task) string {
	if len(t.Payload) == 0 {
		return ""
	}

	var payload struct {
		Dedup string `json:"dedup"`
	}
	if err := json.Unmarshal(t.Payload, &payload); err != nil {
		return ""
	}

	return payload.Dedup
}
