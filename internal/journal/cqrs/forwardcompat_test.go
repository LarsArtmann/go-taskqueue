package cqrs

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// This adapter is a CONSUMER of the journal contract with a deliberately
// SHALLOW decode: fact.Type crosses as a string and fact.Detail crosses as raw
// JSON, so nothing here has a key whitelist that a newer producer could fall
// foul of. That is worth pinning, because "we happen not to decode the detail"
// is invisible in review and one refactor away from becoming
// "we decode it into a struct with five known keys" — at which point every
// additive detail field in the repo starts failing a decode it never used to
// reach.
//
// Sources: OKF v0.2 SPEC §11 (consumers MUST NOT reject on unknown type values
// or unknown additional keys) and docs/research/2026-11-01_okf-lessons.md
// lessons 1/2/13.

// TestUnknownFactTypeCrossesVerbatim pins the type half: a fact family this
// binary has never heard of must cross the adapter unchanged. It is version
// skew, not corruption — the upstream engine (ADR-0019 S2) can land a family
// before tq learns it.
func TestUnknownFactTypeCrossesVerbatim(t *testing.T) {
	t.Parallel()

	fact := journal.Fact{
		Seq:    1,
		Time:   time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
		TaskID: "000001a0unknownadaptertype000000000000",
		Type:   journal.FactType("engine.future.family"),
		Detail: jsontext.Value(`{"anything":"goes"}`),
	}

	evt, err := factEvent(fact)
	if err != nil {
		t.Fatalf("factEvent(unknown type) error = %v, want nil — an unknown family is version skew", err)
	}

	if got := string(evt.Type()); got != "engine.future.family" {
		t.Fatalf("event type = %q, want the fact type verbatim", got)
	}

	var payload factPayload
	if err := json.Unmarshal(evt.Payload(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if string(payload.Detail) != `{"anything":"goes"}` {
		t.Fatalf("detail = %s, want it preserved verbatim", payload.Detail)
	}
}

// TestUnknownDetailKeysCrossVerbatim pins the key half from the other side: the
// adapter must not know which keys a detail carries. A future producer adding
// fields (or a consumer reading them) sees exactly the bytes that were written.
func TestUnknownDetailKeysCrossVerbatim(t *testing.T) {
	t.Parallel()

	const detail = `{"retry_in_ms":1000,"class":"budget","future_field":{"nested":[1,2,3]},"schema":2}`

	fact := journal.Fact{
		Seq:    1,
		Time:   time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
		TaskID: "000001a0unknowndetailkeys000000000000",
		Type:   journal.Requeued,
		Detail: jsontext.Value(detail),
	}

	evt, err := factEvent(fact)
	if err != nil {
		t.Fatalf("factEvent(unknown detail keys) error = %v, want nil", err)
	}

	if !bytes.Contains(evt.Payload(), []byte("future_field")) {
		t.Fatalf("payload dropped an unknown detail key: %s", evt.Payload())
	}

	var payload factPayload
	if err := json.Unmarshal(evt.Payload(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	// Byte-exact: the adapter neither normalises nor reshapes the detail.
	if string(payload.Detail) != detail {
		t.Fatalf("detail = %s, want %s", payload.Detail, detail)
	}
}

// TestMalformedDetailIsRefusedAtMarshal pins the ONE place this adapter judges
// a detail, and that it judges the right thing: `jsontext.Value` validates on
// marshal, so undecodable bytes are refused HERE (a corruption signal) rather
// than crossing into the event stream. That is the opposite of a key whitelist —
// it is byte validity, which no producer can break additively — and it is why
// the tolerance tests above can be absolute about unknown KEYS.
//
// The canary for a future refactor: if this error ever starts naming a missing
// or unexpected FIELD instead of a malformed one, somebody has introduced a
// detail struct here, and every additive detail field in the repo now has to
// clear a decode it never used to reach.
func TestMalformedDetailIsRefusedAtMarshal(t *testing.T) {
	t.Parallel()

	fact := journal.Fact{
		Seq:    1,
		Time:   time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
		TaskID: "000001a0rawdetailcrosses000000000000000",
		Type:   journal.Requeued,
		Detail: jsontext.Value(`{"truncated":`),
	}

	_, err := factEvent(fact)
	if err == nil {
		t.Fatal("factEvent(malformed detail) error = nil, want a refusal — undecodable bytes " +
			"must not enter the event stream")
	}

	if !strings.Contains(err.Error(), "marshal payload") {
		t.Fatalf("error = %v, want the marshal-payload corruption class", err)
	}
}

// TestSliceSourceServesUnknownFamilies guards the CLI/webui read path the same
// way: the in-memory source must hand back facts of any family, so a renderer
// can decide what to do with them.
func TestSliceSourceServesUnknownFamilies(t *testing.T) {
	t.Parallel()

	const taskID = "000001a0sliceunknown0000000000000000"

	facts := []journal.Fact{
		{Seq: 1, Time: time.Now(), TaskID: taskID, Type: journal.FactType("engine.future.family")},
		{Seq: 2, Time: time.Now(), TaskID: taskID, Type: journal.Enqueued},
	}

	got, err := NewSliceSource(facts).Facts(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("facts = %d, want 2 (the unknown family must not be filtered)", len(got))
	}

	if got[0].Type != journal.FactType("engine.future.family") {
		t.Fatalf("first fact type = %q, want the unknown family", got[0].Type)
	}
}
