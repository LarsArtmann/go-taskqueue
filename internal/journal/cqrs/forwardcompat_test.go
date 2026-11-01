package cqrs

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
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

// TestAdapterNeverDecodesDetailIntoAStruct is the canary for the refactor this
// file exists to prevent: decoding a detail into a struct here would introduce a
// key whitelist across the whole journal read path. The assertion is behavioural
// — a detail that is NOT valid JSON still crosses untouched, because a
// struct-decoding adapter could not do that.
func TestAdapterNeverDecodesDetailIntoAStruct(t *testing.T) {
	t.Parallel()

	fact := journal.Fact{
		Seq:    1,
		Time:   time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
		TaskID: "000001a0rawdetailcrosses000000000000000",
		Type:   journal.Requeued,
		Detail: jsontext.Value(`{"truncated":`),
	}

	evt, err := factEvent(fact)
	if err != nil {
		t.Fatalf("factEvent(malformed detail) error = %v — the adapter transports detail, it does not judge it", err)
	}

	var payload factPayload
	if unmarshalErr := json.Unmarshal(evt.Payload(), &payload); unmarshalErr != nil {
		t.Fatalf("decode payload: %v", unmarshalErr)
	}

	if !bytes.Equal(payload.Detail, fact.Detail) {
		t.Fatalf("detail = %s, want the malformed bytes preserved", payload.Detail)
	}
}

// TestSliceSourceServesUnknownFamilies guards the CLI/webui read path the same
// way: the in-memory source must hand back facts of any family, so a renderer
// can decide what to do with them.
func TestSliceSourceServesUnknownFamilies(t *testing.T) {
	t.Parallel()

	facts := []journal.Fact{
		{Seq: 1, Time: time.Now(), TaskID: "000001a0sliceunknown0000000000000000", Type: journal.FactType("engine.future.family")},
		{Seq: 2, Time: time.Now(), TaskID: "000001a0sliceunknown0000000000000000", Type: journal.Enqueued},
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
