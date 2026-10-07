package harvest

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// ErrRedispatchRefused is the mint-time done-check refusal (O4 ruling,
// 2026-10-05): the candidate describes work that repo-side signals prove
// already landed, so minting it buys a paid verify-only lap over done work.
// The escape hatch is --force-redispatch (tq enqueue) / Config.ForceRedispatch.
var ErrRedispatchRefused = errors.New(
	"harvest: re-dispatch refused (work already done; force the re-dispatch to verify anyway)",
)

// RedispatchRefusalError is one refusal with its named signal, so callers can
// render the reason without parsing the sentinel out of the error string.
type RedispatchRefusalError struct {
	Reason string
}

func (r *RedispatchRefusalError) Error() string { return ErrRedispatchRefused.Error() + ": " + r.Reason }

func (r *RedispatchRefusalError) Unwrap() error { return ErrRedispatchRefused }

// redispatchHexRunRe matches every maximal hex run in prompt text; runs
// of exactly the NewID length (36 hex: 16 millis + 10 seed + 10 sequence)
// are task ID citations. Length filtering after a maximal-run match keeps
// git SHAs (40) and display SHAs (8) out without RE2 lookaround.
var redispatchHexRunRe = regexp.MustCompile(`[0-9a-f]+`)

// RedispatchCheck is the mint-time done gate (row 115, O4): it judges a
// MINT CANDIDATE — before the task exists, before any spend — against the
// repo-side signals the store's dedup index cannot see. The claim-time
// DonePreflight answers "is this CLAIM a re-fire?"; this answers "should
// this MINT happen at all?". Signals, first hit wins:
//
//  1. Fix-ticket cured: a review-fix candidate whose rejected-SHA
//     disposition already reads cured (superseded, rebased away, or — the
//     four-paid-lap 2026-10-07 class — the SHA lives but the anchor text
//     is gone from HEAD). The dispatch footer-join dedup three reports
//     demanded.
//  2. Item closed: the candidate's harvest item keys resolve to rows now
//     ticked or absent (the check-off race row 164: a row can close between
//     the harvest scan and the mint — re-read the file at mint time).
//  3. Closeout indexed: the prompt cites a task ID whose closeout report
//     already exists under docs/status — a re-verification dispatch of
//     closed work (the row-110 class: five close-out doc commits for an
//     already-[x] row).
//
// Foreign payload shapes, unresolvable repos, git/file errors and timeouts
// all pass: the gate refuses only on positive proof, never on its own
// failure.
func (h *Harvester) RedispatchCheck(ctx context.Context, n task.New) error {
	ctx, cancel := context.WithTimeout(ctx, donePreflightTimeout)
	defer cancel()

	payload, ok := decodeHarvestPayload(n.Payload)
	if !ok {
		return nil
	}

	dir, ok := h.donePreflightRepoDir(payload.Repo)
	if !ok {
		return nil
	}

	if cured, reason := fixTicketCured(ctx, dir, payload); cured {
		return &RedispatchRefusalError{Reason: reason}
	}

	if closed, reason := h.mintItemClosed(dir, payload); closed {
		return &RedispatchRefusalError{Reason: reason}
	}

	if cited, reason := closeoutCited(dir, payload.Prompt); cited {
		return &RedispatchRefusalError{Reason: reason}
	}

	return nil
}

// mintItemClosed is the item-state signal evaluated against the CURRENT
// todo file at mint time — the anti-race half the claim-time gate cannot
// cover (a row can be ticked after the harvest scan but before the mint).
// Batch candidates use their member keys with prune's all-closed
// semantics; a partial closure leaves the mint alone.
func (h *Harvester) mintItemClosed(dir string, payload harvestPayload) (bool, string) {
	keys := candidateItemKeys(payload)
	if len(keys) == 0 {
		return false, ""
	}

	state, err := h.todoState(dir)
	if err != nil {
		return false, "" // unreadable file: not a refusal signal
	}

	allTicked, allAbsent := true, true

	for _, key := range keys {
		ticked, present := state[key]
		if !present {
			allTicked = false // an absent row is not a ticked row

			continue
		}

		allAbsent = false

		if !ticked {
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

// todoState folds the todo file under base (repo or dir) into an item
// key to ticked map: the open/closed view the redispatch gates judge
// task item keys against.
func (h *Harvester) todoState(base string) (map[string]bool, error) {
	todoFile := h.cfg.TodoFile
	if todoFile == "" {
		todoFile = DefaultTodoFile
	}

	items, err := ParseRepoAll(base, todoFile)
	if err != nil {
		return nil, err
	}

	state := make(map[string]bool, len(items)) // key -> ticked
	for _, item := range items {
		state[item.Key] = item.Done
	}

	return state, nil
}

// candidateItemKeys resolves a mint candidate's item keys: batch member
// keys when present, else the payload dedup key with the catchup: prefix
// stripped (a catchup mint against an already-closed row has nothing left
// to close). A bare batch key without members is not judgeable.
func candidateItemKeys(payload harvestPayload) []string {
	if keys := payload.ItemKeys; len(keys) > 0 {
		return keys
	}

	key := strings.TrimSpace(payload.Dedup)
	if key == "" || strings.HasPrefix(key, BatchKeyPrefix) {
		return nil
	}

	return []string{strings.TrimPrefix(key, CatchupKeyPrefix)}
}

// closeoutCited is the indexed-closeout signal: any 36-hex task ID cited
// in the prompt that already has a closeout report (docs/status/tasks/
// since O7, legacy docs/status/ root still counts) marks the candidate a
// re-verification dispatch of closed work.
func closeoutCited(dir, prompt string) (bool, string) {
	for _, run := range redispatchHexRunRe.FindAllString(prompt, -1) {
		if len(run) != 36 {
			continue
		}

		if closeoutReportExists(dir, run) {
			return true, doneReasonReport + " for cited task " + run[:8] + ")"
		}
	}

	return false, ""
}

// closeoutReportExists reports whether a closeout report for the task ID
// exists under docs/status (both placements). Shared by the mint-time
// cited-task signal and the claim-time report signal.
func closeoutReportExists(dir, id string) bool {
	for _, sub := range []string{"tasks", "."} {
		matches, err := filepath.Glob(
			filepath.Join(dir, "docs", "status", sub, "*_task-"+id+"*"),
		)
		if err == nil && len(matches) > 0 {
			return true
		}
	}

	return false
}

// refuseUnlessForced runs the mint-time check unless the escape hatch is
// armed. Shared by every harvester mint path (single, batch, catchup).
func (h *Harvester) refuseUnlessForced(ctx context.Context, n task.New) error {
	if h.cfg.ForceRedispatch {
		return nil
	}

	return h.RedispatchCheck(ctx, n)
}

// redispatchReasonFor renders a refusal error as a skip reason with the
// signal's own voice, mirroring the ErrTaskDone class split.
func redispatchReasonFor(err error) string {
	refusal, ok := errors.AsType[*RedispatchRefusalError](err)
	if !ok {
		return ""
	}

	return "redispatch refused: " + refusal.Reason + " (force-redispatch to verify anyway)"
}

// Redispatch finding classes: what the row-vs-queue join means for the
// operator reading `tq audit --redispatch`.
const (
	// RedispatchClassLive: a PENDING or RUNNING task whose row is already
	// closed — it will burn a window (or die) proving done work. The mint
	// gate refuses NEW mints of this shape; this is the residue already
	// inside the queue (tq cancel / prune-stale territory).
	RedispatchClassLive = "live-task-on-closed-row"
	// RedispatchClassChurn: a TERMINAL task on a closed row that burned
	// more than one attempt — the paid verify-churn census (the row-110
	// class: five close-out doc commits for an already-[x] row).
	RedispatchClassChurn = "spend-after-close"
)

// RedispatchFinding is one row-vs-queue disagreement the re-dispatch
// surface reports: a task whose TODO row is closed (ticked or gone) while
// the task is still live, or burned multiple attempts on closed work.
type RedispatchFinding struct {
	Repo     string
	TaskID   task.ID
	Status   task.Status
	Attempts int
	ItemKey  string
	ItemText string
	Class    string
}

// RedispatchResult summarizes one re-dispatch audit pass over all repos.
type RedispatchResult struct {
	Repos        int
	Findings     []RedispatchFinding
	ScanFailures []ScanFailure
}

// RedispatchAudit surfaces re-dispatch exposure as a queried fact (row
// 272): every task whose TODO row is already closed — live tasks that
// will spend a window proving done work, and terminal tasks that burned
// attempts on closed work. Report-only: cancelling live residue is an
// operator decision (prune-stale owns pending withdrawals at pool start).
func (h *Harvester) RedispatchAudit(ctx context.Context) (RedispatchResult, error) {
	var res RedispatchResult

	repos, err := h.resolveRepos()
	if err != nil {
		return res, err
	}

	for _, repo := range repos {
		res.Repos++

		if err := h.redispatchAuditRepo(ctx, repo, &res); err != nil {
			res.ScanFailures = append(res.ScanFailures, ScanFailure{Repo: repo, Reason: err.Error()})
		}
	}

	return res, nil
}

func (h *Harvester) redispatchAuditRepo(ctx context.Context, repo string, res *RedispatchResult) error {
	// itemKeysFor needs the payload, so fold the open/ticked state by key
	// once and judge each task's keys against it.
	state, err := h.todoState(repo)
	if err != nil {
		return err
	}

	repoName := filepath.Base(repo)

	tasks, err := h.q.List(ctx, queue.Filter{Project: &repoName, Type: &h.cfg.Type})
	if err != nil {
		return err
	}

	for _, t := range tasks {
		payload, ok := decodeHarvestPayload(t.Payload)
		if !ok {
			continue
		}

		keys := itemKeysFor(t, payload)
		if len(keys) == 0 {
			continue // foreign shapes are invisible to the join
		}

		if !rowClosed(state, keys) {
			continue
		}

		class := ""

		switch t.Status {
		case task.Pending, task.Running:
			class = RedispatchClassLive
		case task.Completed, task.Dead, task.Cancelled:
			if t.Attempts > 1 {
				class = RedispatchClassChurn
			}
		}

		if class == "" {
			continue
		}

		res.Findings = append(res.Findings, RedispatchFinding{
			Repo:     repoName,
			TaskID:   t.ID,
			Status:   t.Status,
			Attempts: t.Attempts,
			ItemKey:  keys[0],
			ItemText: truncateItem(payload.Item),
			Class:    class,
		})
	}

	return nil
}

// rowClosed applies prune's all-closed semantics to one task's item keys:
// every member ticked, or every member gone. A partial closure is open
// work (the batch still owes the open member).
func rowClosed(state map[string]bool, keys []string) bool {
	allTicked, allAbsent := true, true

	for _, key := range keys {
		ticked, present := state[key]
		if !present {
			allTicked = false

			continue
		}

		allAbsent = false

		if !ticked {
			allTicked = false
		}
	}

	return allTicked || allAbsent
}
