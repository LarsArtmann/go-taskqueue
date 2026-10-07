package harvest

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// ErrRedispatchRefused is the mint-time done-check refusal (O4 ruling,
// 2026-10-05): the candidate describes work that repo-side signals prove
// already landed, so minting it buys a paid verify-only lap over done work.
// The escape hatch is --force-redispatch (tq enqueue) / Config.ForceRedispatch.
var ErrRedispatchRefused = errors.New(
	"harvest: re-dispatch refused (work already done; force the re-dispatch to verify anyway)",
)

// RedispatchRefusal is one refusal with its named signal, so callers can
// render the reason without parsing the sentinel out of the error string.
type RedispatchRefusal struct {
	Reason string
}

func (r *RedispatchRefusal) Error() string { return ErrRedispatchRefused.Error() + ": " + r.Reason }

func (r *RedispatchRefusal) Unwrap() error { return ErrRedispatchRefused }

// redispatchTaskIDRe matches full 36-hex task IDs cited in prompt text (the
// NewID shape: 16 hex millis + 10 hex seed + 10 hex sequence). Boundary
// guards keep a 40-hex git SHA from matching as a substring; short hex
// fragments (8-char display SHAs) are below length and never match.
var redispatchTaskIDRe = regexp.MustCompile(`(?:^|[^0-9a-f])([0-9a-f]{36})(?![0-9a-f])`)

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
		return &RedispatchRefusal{Reason: reason}
	}

	if closed, reason := h.mintItemClosed(dir, payload); closed {
		return &RedispatchRefusal{Reason: reason}
	}

	if cited, reason := closeoutCited(dir, payload.Prompt); cited {
		return &RedispatchRefusal{Reason: reason}
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

	todoFile := h.cfg.TodoFile
	if todoFile == "" {
		todoFile = DefaultTodoFile
	}

	items, err := ParseRepoAll(dir, todoFile)
	if err != nil {
		return false, "" // unreadable file: not a refusal signal
	}

	state := make(map[string]bool, len(items)) // key -> ticked
	for _, item := range items {
		state[item.Key] = item.Done
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
	for _, id := range redispatchTaskIDRe.FindAllStringSubmatch(prompt, -1) {
		if closeoutReportExists(dir, id[1]) {
			return true, doneReasonReport + " for cited task " + id[1][:8] + ")"
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
	refusal, ok := errors.AsType[*RedispatchRefusal](err)
	if !ok {
		return ""
	}

	return "redispatch refused: " + refusal.Reason + " (force-redispatch to verify anyway)"
}

// redispatchCheckSummary is the one-line mint-gate explanation the enqueue
// CLI prints with the refusal so the operator sees both the signal and the
// escape hatch without reading source.
func redispatchCheckSummary(err error) string {
	if reason := redispatchReasonFor(err); reason != "" {
		return reason
	}

	return err.Error()
}
