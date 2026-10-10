package companion

import (
	"encoding/json/jsontext"
	"testing"

	ufacts "github.com/larsartmann/go-cqrs-lite/queue/v4/facts"
	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// TestJournalVocabularyRidesUpstreamFacts pins the ADR-0019 S2 contract:
// tq's fact vocabulary RIDES the upstream engine's facts vocabulary —
// every lifecycle constant the engine records must be the SAME string as
// tq's journal constant, so engine-written facts surface unchanged
// through the tq vocabulary (and vice versa). Since the S2 alias flip
// the vocabularies ARE one type (journal.Fact = facts.Fact), so this
// equality gate is belt-and-braces against a silent upstream constant
// rename. Heartbeat is deliberately absent: the engines record no
// heartbeat facts (S1 divergence D3) — tq-only, still written through
// the same journal.
func TestJournalVocabularyRidesUpstreamFacts(t *testing.T) {
	t.Parallel()

	pairs := []struct {
		tq     journal.FactType
		engine ufacts.FactType
	}{
		{journal.Enqueued, ufacts.Enqueued},
		{journal.Claimed, ufacts.Claimed},
		{journal.Completed, ufacts.Completed},
		{journal.Failed, ufacts.Failed},
		{journal.DeadLettered, ufacts.DeadLettered},
		{journal.Cancelled, ufacts.Cancelled},
		{journal.CancelRequested, ufacts.CancelRequested},
		{journal.Released, ufacts.Released},
		{journal.Requeued, ufacts.Requeued},
		{journal.Orphaned, ufacts.Orphaned},
		{journal.Reprioritized, ufacts.Reprioritized},
	}

	for _, pair := range pairs {
		if string(pair.tq) != string(pair.engine) {
			t.Errorf("vocabulary drift: tq %q != engine %q", pair.tq, pair.engine)
		}
	}
}

// TestFactMappingRoundTrip pins the S2 conversion seam: a tq journal fact
// mapped upstream and back is byte-identical (seq, detail, type).
func TestFactMappingRoundTrip(t *testing.T) {
	t.Parallel()

	in := journal.Fact{
		Seq:     7,
		TaskID:  "t-1",
		Type:    journal.Reprioritized,
		Owner:   "worker-1",
		Attempt: 2,
		Error:   "boom",
		Detail:  jsontext.Value(`{"to":5}`),
	}

	up := UpstreamFact(in)
	if up.Type != ufacts.Reprioritized || string(up.Detail) != `{"to":5}` {
		t.Fatalf("upstream mapping drifted: %+v", up)
	}

	up.Seq = in.Seq // the journal assigns seq; re-attach for the round trip

	out := JournalFacts([]ufacts.Fact{up})[0]
	if out.Seq != in.Seq || out.TaskID != in.TaskID || out.Type != in.Type ||
		out.Owner != in.Owner || out.Attempt != in.Attempt || out.Error != in.Error ||
		string(out.Detail) != string(in.Detail) || !out.Time.Equal(in.Time) {
		t.Fatalf("round trip drifted: in %+v out %+v", in, out)
	}
}
