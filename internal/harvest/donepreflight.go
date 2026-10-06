package harvest

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// donePreflightTimeout bounds one DonePreflight evaluation: a few cheap git
// calls and a todo-file parse. The gate is advisory at claim time — it must
// never stall a claim — so everything inside degrades to "not done" on
// timeout or error (fail-open to the historical behavior: run the agent).
const donePreflightTimeout = 10 * time.Second

// donePreflight reasons head the recorded strings so the journal and logs
// have one greppable voice per signal (pruneReasonHead pattern).
const (
	doneReasonFooter  = "done preflight: commit(s) already carry this task's Task-Queue-ID footer"
	doneReasonFixCure = "done preflight: fix ticket cured ("
	doneReasonTicked  = "done preflight: TODO_LIST item is now [x]: "
	doneReasonAbsent  = "done preflight: TODO_LIST item no longer present (key "
	doneReasonReport  = "done preflight: closeout report already exists (docs/status)"
)

// DonePreflight is the claim-time done gate (docs/planning/2026-10-06_18-50
// _dispatch-done-preflight-gate.md): it decides whether a claimed task's
// work is provably already done, so the worker can complete it with zero
// agent spend instead of re-running a full session on landed work — the
// re-fire class (one task: enqueued once, claimed 15×, every claim a paid
// no-op over 21 footer commits).
//
// Signals, strongest first, first hit wins (the reason names the signal):
//
//  1. Footer commits: at least one commit on any ref carries this task's
//     Task-Queue-ID footer (works for harvested AND fix tasks).
//  2. Fix-ticket disposition: payload.RejectedSHA exists AND a commit cites
//     it (superseding fix), or the SHA is gone AND the anchor text has zero
//     hits in the working tree (finding cured by the rebase).
//  3. Harvest item state (the prune-stale rules applied per claim): the
//     item's dedup key is now `[x]`, or gone from the file entirely.
//  4. Closeout report: docs/status/tasks/*_task-<id>* exists in the repo
//     (legacy docs/status/*_task-<id>* reports count too).
//
// Foreign payload shapes (no Repo/Prompt), unresolvable repos, git/file
// errors and timeouts all evaluate to NOT done: the gate never guesses and
// never blocks dispatch on its own failure.
func (h *Harvester) DonePreflight(ctx context.Context, t task.Task) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, donePreflightTimeout)
	defer cancel()

	payload, ok := donePreflightPayload(t)
	if !ok {
		return false, ""
	}

	dir, ok := h.donePreflightRepoDir(payload.Repo)
	if !ok {
		return false, ""
	}

	if done, reason := footerCommitExists(ctx, dir, t); done {
		return true, reason
	}

	if done, reason := fixTicketCured(ctx, dir, payload); done {
		return true, reason
	}

	if done, reason := h.itemStale(dir, t, payload); done {
		return true, reason
	}

	if done, reason := reportExists(dir, t); done {
		return true, reason
	}

	return false, ""
}

// donePreflightPayload decodes the agent payload a preflight can judge:
// Repo and Prompt are the minimum identity. Tasks minted outside the
// harvester/review loop (external shapes) are invisible to the gate.
func donePreflightPayload(t task.Task) (harvestPayload, bool) {
	if len(t.Payload) == 0 {
		return harvestPayload{}, false
	}

	var payload harvestPayload
	if err := json.Unmarshal(t.Payload, &payload); err != nil ||
		payload.Repo == "" || payload.Prompt == "" {
		return harvestPayload{}, false
	}

	return payload, true
}

// donePreflightRepoDir resolves the payload's Repo (absolute path or
// projects-dir-relative name) to an existing directory; false leaves the
// task alone (the executor's own resolution will surface real errors).
func (h *Harvester) donePreflightRepoDir(repo string) (string, bool) {
	dir := repo
	if !filepath.IsAbs(dir) {
		if h.cfg.ProjectsDir == "" {
			return "", false
		}

		dir = filepath.Join(h.cfg.ProjectsDir, repo)
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", false
	}

	return dir, true
}

// footerCommitExists is signal 1: the executor's own trailer scanner over
// every ref. Errors (non-git dir, old git) are not done-signals.
func footerCommitExists(ctx context.Context, dir string, t task.Task) (bool, string) {
	commits, err := (executor.GitLogScanner{}).CommitsByTrailer(
		ctx, dir, executor.TaskTrailer, t.ID.String(),
	)
	if err != nil || len(commits) == 0 {
		return false, ""
	}

	return true, doneReasonFooter
}

// fixTicketCured is signal 2: the rejected-SHA disposition for review-fix
// tasks (payload.RejectedSHA, structured by the review sweeper). The
// finding is cured when a superseding commit cites the rejected SHA, or
// when the SHA was rebased away AND its anchor text no longer exists in
// the working tree. A still-present SHA with no supersede and a live
// anchor leaves the finding open — not done.
func fixTicketCured(ctx context.Context, dir string, payload harvestPayload) (bool, string) {
	sha := strings.TrimSpace(payload.RejectedSHA)
	if sha == "" {
		return false, ""
	}

	exists := gitSucceeds(ctx, dir, "cat-file", "-e", sha)

	if exists && gitOutputs(ctx, dir, "log", "--all", "--format=%H", "--grep", sha) {
		return true, doneReasonFixCure + "superseding commit cites " + shortSHA(sha) + ")"
	}

	if !exists && strings.TrimSpace(payload.Anchor) != "" &&
		!gitOutputs(ctx, dir, "grep", "-F", "--", payload.Anchor) {
		return true, doneReasonFixCure + "rejected SHA gone and anchor text absent)"
	}

	return false, ""
}

// itemStale is signal 3: the prune-stale ticked/absent rules evaluated
// against the CURRENT todo file at claim time (prune itself only runs at
// pool start; the re-fire class happens between ticks). Batch tasks use
// their member keys with prune's all-stale semantics: every member ticked,
// or every member gone — partial staleness leaves the task.
func (h *Harvester) itemStale(dir string, t task.Task, payload harvestPayload) (bool, string) {
	keys := itemKeysFor(t, payload)
	if len(keys) == 0 {
		return false, ""
	}

	todoFile := h.cfg.TodoFile
	if todoFile == "" {
		todoFile = DefaultTodoFile
	}

	items, err := ParseRepoAll(dir, todoFile)
	if err != nil {
		return false, "" // unreadable file: not a done-signal
	}

	open := make(map[string]bool, len(items))
	for _, item := range items {
		open[item.Key] = !item.Done
	}

	allTicked, allAbsent := true, true

	for _, key := range keys {
		isOpen, present := open[key]
		if !present {
			allTicked = false // prune parity: an absent member is not ticked

			continue
		}

		allAbsent = false

		if isOpen {
			allTicked = false
		}
	}

	switch {
	case allAbsent:
		return true, doneReasonAbsent + keys[0] + ")"
	case allTicked:
		return true, doneReasonTicked + firstKeyText(keys, payload)
	default:
		return false, ""
	}
}

// itemKeysFor resolves the dedup keys a task's staleness is judged by:
// batch member keys when present, else the payload's dedup key with the
// catchup: prefix stripped (loop-closing tasks pin catchup:<key>). A bare
// batch key without members is not judgeable.
func itemKeysFor(t task.Task, payload harvestPayload) []string {
	if keys := payloadItemKeys(t); len(keys) > 0 {
		return keys
	}

	key := strings.TrimSpace(payload.Dedup)
	if key == "" || strings.HasPrefix(key, BatchKeyPrefix) {
		return nil
	}

	return []string{strings.TrimPrefix(key, "catchup:")}
}

// firstKeyText renders a short display form for the ticked reason; the
// item's own text when the payload carries it, else the key.
func firstKeyText(keys []string, payload harvestPayload) string {
	if text := strings.TrimSpace(payload.Item); text != "" {
		return truncateItem(text)
	}

	return keys[0]
}

// reportExists is signal 4: the closeout-report convention both dogfood
// repos teach their agents (docs/status/tasks/<date>_task-<id>.md since
// the O7 report-placement ruling; legacy reports at docs/status/ root
// still count — landed work must never turn invisible).
func reportExists(dir string, t task.Task) (bool, string) {
	for _, sub := range []string{"tasks", "."} {
		matches, err := filepath.Glob(
			filepath.Join(dir, "docs", "status", sub, "*_task-"+t.ID.String()+"*"),
		)
		if err == nil && len(matches) > 0 {
			return true, doneReasonReport
		}
	}

	return false, ""
}

// gitSucceeds runs one git command in dir; true iff it exited 0.
func gitSucceeds(ctx context.Context, dir string, args ...string) bool {
	return exec.CommandContext(ctx, "git", gitArgs(dir, args)...).Run() == nil
}

// gitOutputs runs one git command in dir and reports whether it printed
// anything (the --grep/-F "any hit?" question).
func gitOutputs(ctx context.Context, dir string, args ...string) bool {
	out, err := exec.CommandContext(ctx, "git", gitArgs(dir, args)...).Output()

	return err == nil && strings.TrimSpace(string(out)) != ""
}

func gitArgs(dir string, args []string) []string {
	return append([]string{"-C", dir}, args...)
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}

	return sha
}
