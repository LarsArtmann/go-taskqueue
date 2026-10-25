package harvest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// writeVerdictMarker records the tq-preflight terminal-verdict marker for
// a task id (the record_verdicts_marker shape).
func writeVerdictMarker(t *testing.T, repo, id string) {
	t.Helper()

	writeVerdictMarkerState(t, repo, id, "already_done")
}

// writeVerdictMarkerState records a .tq/verdicts/<id> marker carrying an
// explicit verdict state (the record_verdicts_marker shape).
func writeVerdictMarkerState(t *testing.T, repo, id, verdict string) {
	t.Helper()

	dir := filepath.Join(repo, ".tq", "verdicts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	body := "verdict: " + verdict + "\nrecorded_at: " +
		time.Now().UTC().Format(time.RFC3339) + "\n"
	if err := os.WriteFile(filepath.Join(dir, id), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeClaimMarker records the tq-claim.sh live-claim marker and returns
// a backdater for staleness cases.
func writeClaimMarker(t *testing.T, repo, id string) {
	t.Helper()

	dir := filepath.Join(repo, ".tq", "claimed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, id), []byte(`{"note":"t"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRedispatchCheckVerdictMarkerCited is the cited-prior-dispatch
// shape: the prompt cites a task id whose .tq/verdicts marker reads
// already_done — terminal state the queue was blind to (fam.12.2, the
// df4c314 eleven-paid-re-dispatch class).
func TestRedispatchCheckVerdictMarkerCited(t *testing.T) {
	fixture := newDonePreflightFixture(t, "- [ ] open work\n")

	cited := strings.Repeat("1", 36)
	writeVerdictMarker(t, fixture.repo, cited)

	err := fixture.h.RedispatchCheck(
		context.Background(),
		candidateWith("", "", "re-verify task "+cited, "", ""),
	)

	refusal, ok := errors.AsType[*RedispatchRefusalError](err)
	if !ok {
		t.Fatalf("expected a redispatch refusal, got %v", err)
	}

	if !strings.Contains(refusal.Reason, "verdict marker reads already_done") {
		t.Fatalf("reason should name the verdict marker, got %q", refusal.Reason)
	}
}

// TestRedispatchCheckVerdictMarkerByItemKeyJoin is the re-harvest shape:
// the row is still open (so signal 2 passes) but a PRIOR task for the
// same dedup key — found through the queue — has an already_done verdict
// marker in the repo. The mint must refuse.
func TestRedispatchCheckVerdictMarkerByItemKeyJoin(t *testing.T) {
	projects := t.TempDir()
	repo := writeRepo(t, projects, "preflight", "- [ ] open work\n")

	q := openQueue(t)
	h := New(q, Config{ProjectsDir: projects})
	ctx := context.Background()

	key := ItemKey("preflight", "open work")

	prior, err := q.Enqueue(ctx, task.New{
		Project:  "preflight",
		Type:     "agent",
		Payload:  []byte(`{"repo":"preflight","prompt":"work","dedup":` + jsonString(key) + `,"item":"open work"}`),
		DedupKey: key,
	})
	if err != nil {
		t.Fatal(err)
	}

	writeVerdictMarker(t, repo, prior.ID.String())

	itemText := "open work"
	err = h.RedispatchCheck(
		context.Background(),
		candidateWith(key, itemText, "do the thing", "", ""),
	)

	refusal, ok := errors.AsType[*RedispatchRefusalError](err)
	if !ok {
		t.Fatalf("expected a redispatch refusal, got %v", err)
	}

	if !strings.Contains(refusal.Reason, prior.ID.String()[:8]) {
		t.Fatalf("reason should name the prior task, got %q", refusal.Reason)
	}
}

// TestRedispatchCheckLiveClaimMarkerBlocks: a fresh claim marker on a
// prior task for the same item machine-blocks the re-dispatch within the
// cooldown window (fam.12.3) — but an aged-out marker does not.
func TestRedispatchCheckLiveClaimMarkerBlocks(t *testing.T) {
	projects := t.TempDir()
	repo := writeRepo(t, projects, "preflight", "- [ ] open work\n")

	q := openQueue(t)
	ctx := context.Background()

	key := ItemKey("preflight", "open work")

	prior, err := q.Enqueue(ctx, task.New{
		Project:  "preflight",
		Type:     "agent",
		Payload:  []byte(`{"repo":"preflight","prompt":"work","dedup":` + jsonString(key) + `,"item":"open work"}`),
		DedupKey: key,
	})
	if err != nil {
		t.Fatal(err)
	}

	id := prior.ID.String()
	writeClaimMarker(t, repo, id)

	cooldown := time.Hour
	h := New(q, Config{ProjectsDir: projects, RedispatchCooldown: cooldown})
	itemText := "open work"

	err = h.RedispatchCheck(
		context.Background(),
		candidateWith(key, itemText, "do the thing", "", ""),
	)

	refusal, ok := errors.AsType[*RedispatchRefusalError](err)
	if !ok {
		t.Fatalf("expected a redispatch refusal for the fresh claim, got %v", err)
	}

	if !strings.Contains(refusal.Reason, "live claim marker") {
		t.Fatalf("reason should name the live claim, got %q", refusal.Reason)
	}

	// Age the marker past the cooldown: the block ends (the claim script's
	// sweep would have released it; the gate must not outlive the claim).
	marker := filepath.Join(repo, ".tq", "claimed", id)

	stale := time.Now().Add(-2 * cooldown)
	if err := os.Chtimes(marker, stale, stale); err != nil {
		t.Fatal(err)
	}

	if err := h.RedispatchCheck(
		context.Background(),
		candidateWith(key, itemText, "do the thing", "", ""),
	); err != nil {
		t.Fatalf("aged-out claim must not block, got %v", err)
	}
}

// TestRedispatchCheckMarkersOpenWorkAllowed: open row, no markers, queue
// tasks on other items — the mint proceeds (refusal needs positive proof).
func TestRedispatchCheckMarkersOpenWorkAllowed(t *testing.T) {
	projects := t.TempDir()
	writeRepo(t, projects, "preflight", "- [ ] open work\n- [ ] other work\n")

	q := openQueue(t)
	h := New(q, Config{ProjectsDir: projects})
	ctx := context.Background()

	otherKey := ItemKey("preflight", "other work")
	if _, err := q.Enqueue(ctx, task.New{
		Project: "preflight",
		Type:    "agent",
		Payload: []byte(
			`{"repo":"preflight","prompt":"work","dedup":` + jsonString(otherKey) + `,"item":"other work"}`,
		),
		DedupKey: otherKey,
	}); err != nil {
		t.Fatal(err)
	}

	key := ItemKey("preflight", "open work")
	if err := h.RedispatchCheck(
		context.Background(),
		candidateWith(key, "open work", "do the thing", "", ""),
	); err != nil {
		t.Fatalf("open work without markers must not be refused, got %v", err)
	}
}

// TestRedispatchCheckBlockedNeedsRootVerdictMarker pins the second
// terminal verdict state: a prior dispatch whose close-out proved the
// work un-doable without a root-capable shell (the 2026-10-21 PMA
// activation re-dispatch class) must machine-block re-minting, not just
// already_done.
func TestRedispatchCheckBlockedNeedsRootVerdictMarker(t *testing.T) {
	fixture := newDonePreflightFixture(t, "- [ ] open work\n")

	cited := strings.Repeat("2", 36)
	writeVerdictMarkerState(t, fixture.repo, cited, "blocked_needs_root")

	err := fixture.h.RedispatchCheck(
		context.Background(),
		candidateWith("", "", "re-verify task "+cited, "", ""),
	)

	refusal, ok := errors.AsType[*RedispatchRefusalError](err)
	if !ok {
		t.Fatalf("expected a redispatch refusal, got %v", err)
	}

	if !strings.Contains(refusal.Reason, "verdict marker reads blocked_needs_root") {
		t.Fatalf("reason should name the blocked verdict, got %q", refusal.Reason)
	}
}

// TestRedispatchCheckNonTerminalVerdictMarkerDoesNotBlock pins the
// vocabulary boundary: only the terminal states machine-block — a marker
// carrying a non-terminal state (or any unknown word) must pass the gate,
// because refusal needs positive proof of terminal state.
func TestRedispatchCheckNonTerminalVerdictMarkerDoesNotBlock(t *testing.T) {
	fixture := newDonePreflightFixture(t, "- [ ] open work\n")

	cited := strings.Repeat("3", 36)
	writeVerdictMarkerState(t, fixture.repo, cited, "requeued")

	err := fixture.h.RedispatchCheck(
		context.Background(),
		candidateWith("", "", "re-verify task "+cited, "", ""),
	)

	if err != nil {
		t.Fatalf("non-terminal verdict marker must not block, got %v", err)
	}
}
