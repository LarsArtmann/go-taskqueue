package harvest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// defaultClaimLiveWindow mirrors tq-claim.sh's TQ_STALE_AFTER default
// (6h): a claim marker younger than the window means another dispatched
// session holds the work — re-dispatching it buys a foregone double
// session, the 2026-09-11 03:23/03:33 double-dispatch class.
const defaultClaimLiveWindow = 6 * time.Hour

// Mint-time reason heads for the repo-side coordination markers
// (.tq/verdicts/<id>, .tq/claimed/<id>) — the machine-readable surfaces
// tq-preflight's record_verdicts_marker and tq-claim.sh write for exactly
// this reader (owner rulings fam.12.2/12.3, 2026-10-17: the queue side
// was blind to both, so the df4c314 class paid eleven re-dispatches for a
// row closed at dispatch one).
const (
	doneReasonVerdict = "verdict marker reads already_done (task "
	doneReasonClaim   = "live claim marker held by another session (task "
)

// claimWindow resolves the configured claim-marker liveness window: the
// cooldown a live claim machine-blocks re-dispatch for (Config, fam.12.3).
func (h *Harvester) claimWindow() time.Duration {
	if h.cfg.RedispatchCooldown > 0 {
		return h.cfg.RedispatchCooldown
	}

	return defaultClaimLiveWindow
}

// markerBlocked is the mint-time coordination-marker signal: the candidate
// is refused when a prior dispatch of the SAME work left terminal state or
// a live claim in the repo. Two joins find the prior task's id:
//
//  1. Prompt citation: any 36-hex task id the candidate's prompt cites
//     (the re-verification dispatch shape — mirrors closeoutCited).
//  2. Item-key join through the queue: tasks already minted for the same
//     dedup keys, the re-harvest shape (the row stays open in
//     TODO_LIST.md, so every tick mints a fresh id and only the queue
//     knows the prior ones).
//
// A verdict marker blocks while PRESENT — it is terminal state and
// expires only via the marker TTL sweep (TQ_VERDICT_TTL, tq-claim.sh
// sweep). A claim marker blocks while its mtime is inside the liveness
// window; the claim script's own sweep releases stale/footer-committed
// markers, so this gate never outlives the claim it mirrors. Unreadable
// dirs, unknown ids and queue errors all pass: refusal needs positive
// proof, never its own failure.
func (h *Harvester) markerBlocked(ctx context.Context, dir string, payload harvestPayload) (bool, string) {
	for _, run := range redispatchHexRunRe.FindAllString(payload.Prompt, -1) {
		if len(run) != 36 {
			continue
		}

		if done, reason := markerRefusal(dir, run, h.claimWindow()); done {
			return true, reason
		}
	}

	keys := candidateItemKeys(payload)
	if len(keys) == 0 || h.q == nil {
		return false, ""
	}

	repoName := filepath.Base(dir)

	tasks, err := h.q.List(ctx, queue.Filter{Project: &repoName, Type: &h.cfg.Type})
	if err != nil {
		return false, "" // queue read failure: not a refusal signal
	}

	for _, t := range tasks {
		tp, ok := decodeHarvestPayload(t.Payload)
		if !ok || !keysIntersect(keys, itemKeysFor(t, tp)) {
			continue
		}

		if done, reason := markerRefusal(dir, t.ID.String(), h.claimWindow()); done {
			return true, reason
		}
	}

	return false, ""
}

// markerRefusal judges ONE prior task id against both markers; the claim
// window decides liveness of .tq/claimed/<id>.
func markerRefusal(dir, id string, window time.Duration) (bool, string) {
	if verdictAlreadyDone(dir, id) {
		return true, doneReasonVerdict + shortSHA(id) + ")"
	}

	if claimLive(dir, id, window) {
		return true, doneReasonClaim + shortSHA(id) + ")"
	}

	return false, ""
}

// verdictAlreadyDone reports whether .tq/verdicts/<id> exists and reads
// already_done — the terminal verdict tq-preflight records on every
// blocked re-emit.
func verdictAlreadyDone(dir, id string) bool {
	raw, err := os.ReadFile(filepath.Join(dir, ".tq", "verdicts", id))

	return err == nil && strings.Contains(string(raw), "verdict: already_done")
}

// claimLive reports whether .tq/claimed/<id> exists with an mtime inside
// the liveness window (a younger-than-window marker is a claim another
// session still holds).
func claimLive(dir, id string, window time.Duration) bool {
	info, err := os.Stat(filepath.Join(dir, ".tq", "claimed", id))
	if err != nil {
		return false
	}

	return time.Since(info.ModTime()) < window
}

// keysIntersect reports whether the candidate's item keys and a prior
// task's share at least one member (the same-work join).
func keysIntersect(candidate, prior []string) bool {
	set := make(map[string]struct{}, len(candidate))
	for _, k := range candidate {
		set[k] = struct{}{}
	}

	for _, k := range prior {
		if _, ok := set[k]; ok {
			return true
		}
	}

	return false
}
