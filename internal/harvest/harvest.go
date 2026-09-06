// Package harvest turns repository backlogs into queue tasks.
//
// It is the ingestion half of a self-managing agent pool: every repo's
// TODO_LIST.md is the shared, human-readable work backlog; the harvester
// repeatedly scans it and enqueues one agent task per open item, paced to at
// most one in-flight task per repo. Agents (see executor.CrushExecutor) do
// the item and mark it done in the same file, which closes the loop: done
// items are never re-enqueued, edited items become new work.
package harvest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// DefaultTodoFile is the backlog file scanned in each repository.
const DefaultTodoFile = "TODO_LIST.md"

// DefaultMaxPerTick bounds how many new agent tasks one harvest run may
// enqueue across all repos — the cost throttle for unattended pools.
const DefaultMaxPerTick = 10

// DefaultPromptTemplate is the agent contract handed to Crush for each item.
// Placeholders: {{REPO_ABS}}, {{REPO}}, {{HEADING}}, {{ITEM}}.
const DefaultPromptTemplate = `You are an autonomous agent working from a shared task queue, unsupervised.

Repository: {{REPO_ABS}}
Work item from TODO_LIST.md, section "{{HEADING}}":
"{{ITEM}}"

Contract:
1. Read AGENTS.md (and CONTRIBUTING.md / CLAUDE.md if present) first and follow it.
2. Do exactly this work item. No scope creep, no drive-by refactors.
3. Verify your work: run the project's build and tests. Never leave the repo broken.
4. Close the loop in TODO_LIST.md: mark this item done ([x]) or remove it, following the file's own conventions. If you could NOT finish it, leave it unchecked and append " — BLOCKED: <one-line reason>".
5. Commit your changes with a clear message. Never push.`

// Config controls one harvester.
type Config struct {
	// ProjectsDir is scanned (depth 1) for repos containing TodoFile.
	ProjectsDir string
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
}

func (c Config) withDefaults() Config {
	if c.Type == "" {
		c.Type = executor.TaskTypeCrush
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

// Item is one open checkbox found in a repo's todo file.
type Item struct {
	Repo     string // absolute repo path
	RepoName string // project name (repo dir base name)
	Heading  string // nearest markdown heading above the item
	Text     string // the checkbox text, trimmed
	Key      string // stable dedup key: hash of repo name + item text
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
			return res, fmt.Errorf("harvest: no repos and no projects dir configured")
		}
		var err error
		repos, err = DiscoverRepos(h.cfg.ProjectsDir, h.cfg.TodoFile)
		if err != nil {
			return res, fmt.Errorf("harvest: discover repos: %w", err)
		}
	}
	sort.Strings(repos)

	for _, repo := range repos {
		res.Repos++
		items, err := ParseRepo(repo, h.cfg.TodoFile)
		if err != nil {
			res.Skipped = append(res.Skipped, Skipped{Reason: "scan failed: " + err.Error()})
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
	for _, t := range tasks {
		if t.Status == task.Pending || t.Status == task.Running {
			busy = true
		}
		if key := payloadDedup(t); key != "" {
			if _, dup := known[key]; !dup {
				known[key] = t.Status
			}
		}
	}

	enqueuedThisRepo := false
	for _, it := range items {
		switch {
		case it.Key != "" && known[it.Key] != "":
			st := known[it.Key]
			reason := "tracked: " + string(st)
			if st == task.Dead {
				reason = "in DLQ (tq dlq --rescue to retry)"
			} else if st == task.Cancelled {
				reason = "cancelled (edit the item text to re-arm it)"
			}
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: reason})
		case busy:
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: "repo busy: one agent per repo"})
		case enqueuedThisRepo:
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: "paced: one new item per repo per run"})
		case len(res.Enqueued) >= h.cfg.MaxPerTick:
			res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: "tick cap reached (--max-per-tick)"})
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
				res.Skipped = append(res.Skipped, Skipped{Item: it, Reason: "tracked: " + string(t.Status) + " (enqueued concurrently)"})
			}
		}
	}
}

func (h *Harvester) enqueue(ctx context.Context, it Item) (task.Task, error) {
	prompt := h.cfg.PromptTemplate
	prompt = strings.ReplaceAll(prompt, "{{REPO_ABS}}", it.Repo)
	prompt = strings.ReplaceAll(prompt, "{{REPO}}", it.RepoName)
	prompt = strings.ReplaceAll(prompt, "{{HEADING}}", it.Heading)
	prompt = strings.ReplaceAll(prompt, "{{ITEM}}", it.Text)

	payload, err := executor.RenderCrushPayload(executor.CrushPayload{
		Repo:   it.Repo,
		Prompt: prompt,
		Dedup:  it.Key,
	})
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

// DiscoverRepos returns the depth-1 subdirectories of dir that contain the
// todo file, sorted by name.
func DiscoverRepos(dir, todoFile string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
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
// an item; `- [x]`/`- [X]` are done and ignored. The nearest heading above
// the item (any level, text without the #s) is its Heading. Lines inside
// fenced code blocks are ignored, so TODO examples in documentation are never
// harvested. The Key changes when the item text changes, so editing an item
// re-arms it even if a previous task for the old wording exists.
func ParseRepo(repo, todoFile string) ([]Item, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(abs, todoFile))
	if err != nil {
		return nil, err
	}
	repoName := filepath.Base(abs)

	var items []Item
	heading := ""
	inFence := false
	for _, line := range strings.Split(string(data), "\n") {
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
		if text, ok := openCheckbox(trimmed); ok {
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

// openCheckbox returns the text of an open markdown checkbox line.
func openCheckbox(line string) (string, bool) {
	rest, ok := strings.CutPrefix(line, "-")
	if !ok {
		if rest, ok = strings.CutPrefix(line, "*"); !ok {
			return "", false
		}
	}
	rest = strings.TrimSpace(rest)
	rest, ok = strings.CutPrefix(rest, "[ ]")
	if !ok {
		return "", false
	}
	return rest, true
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
