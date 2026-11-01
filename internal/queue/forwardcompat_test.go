package queue

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"slices"
	"strings"
	"testing"
)

// The queue package is a CONSUMER of journal detail written by binaries that may
// be older or newer than this one, so its detail decoders carry a contract:
//
//   - unknown KEYS are ignored, so a producer can add a field with no consumer
//     change;
//   - malformed bytes are a SKIP (ok=false / zero value), never a failure — a
//     fact that cannot be read must not take the fold or a claim down with it;
//   - the WIRE NAMES are pinned, because they are the only contract between the
//     producer and every consumer, and a rename is silent otherwise.
//
// Sources: OKF v0.2 SPEC §11 (consumers MUST NOT reject on unknown additional
// keys) and docs/research/2026-11-01_okf-lessons.md lessons 1/2/13. The
// readmodel module pins the same class from the other side
// (internal/readmodel/detailkeys_test.go).

// TestParseReprioritizeEvidenceIgnoresUnknownKeys pins the additive half on the
// parser the claim path and the priority provenance view both ride.
func TestParseReprioritizeEvidenceIgnoresUnknownKeys(t *testing.T) {
	t.Parallel()

	detail := jsontext.Value(
		`{"old_priority":3,"new_priority":7,"source":"ai","reason":"rescored",` +
			`"evidence_ref":"abc123","confidence":0.92,"schema":2}`,
	)

	got, ok := ParseReprioritizeEvidence(detail)
	if !ok {
		t.Fatal("ParseReprioritizeEvidence(extra keys) ok = false, want true — unknown keys are additive")
	}

	if got.OldPriority != 3 || got.NewPriority != 7 || got.Source != "ai" || got.Reason != "rescored" {
		t.Fatalf("evidence = %+v, want the four known fields intact", got)
	}
}

// TestParseReprioritizeEvidenceSkipsMalformed is the other boundary: unreadable
// detail stays a skip, because the alternative is a claim path that fails on
// journal noise.
func TestParseReprioritizeEvidenceSkipsMalformed(t *testing.T) {
	t.Parallel()

	for _, detail := range []string{`{"old_priority":`, ``, `[]`, `null`} {
		if _, ok := ParseReprioritizeEvidence(jsontext.Value(detail)); ok && detail != `null` {
			t.Fatalf("ParseReprioritizeEvidence(%q) ok = true, want the skip-don't-fail policy", detail)
		}
	}
}

// TestEvidenceWireKeysPinned pins the JSON names of the detail structs this
// package decodes. A rename here rewrites the fact format, and every consumer
// outside the repo's gate set would discover it at runtime.
func TestEvidenceWireKeysPinned(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  []string
	}{
		{
			name: "ReprioritizeEvidence",
			value: ReprioritizeEvidence{
				OldPriority: 1, NewPriority: 2, Source: "human", Reason: "seeded",
			},
			want: []string{"new_priority", "old_priority", "reason", "source"},
		},
		{
			name: "RequeueEvidence",
			value: RequeueEvidence{
				Reason: "boom", RetryIn: 1000, ResumeCloseout: true, Class: "budget", Provider: "anthropic",
			},
			want: []string{"class", "provider", "reason", "resume_closeout", "retry_in_ms"},
		},
		{
			name:  "ParkedDetail",
			value: ParkedDetail{Reason: "review", UntilMS: 1000, By: "operator"},
			want:  []string{"by", "reason", "until_ms"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw, err := jsonv2.Marshal(tt.value)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var decoded map[string]any
			if err := jsonv2.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			keys := make([]string, 0, len(decoded))
			for key := range decoded {
				keys = append(keys, key)
			}

			slices.Sort(keys)
			slices.Sort(tt.want)

			if !slices.Equal(keys, tt.want) {
				t.Fatalf("wire keys = %v, want %v — a detail rename is a fact-format change", keys, tt.want)
			}
		})
	}
}

// TestRequeueEvidenceIgnoresUnknownKeys pins the tolerance for the detail the
// readmodel fold and the parked view decode: the provider field itself arrived
// additively, so the posture is already load-bearing.
func TestRequeueEvidenceIgnoresUnknownKeys(t *testing.T) {
	t.Parallel()

	detail := []byte(`{"reason":"rate limited","retry_in_ms":900000,"class":"provider",` +
		`"provider":"anthropic","requeue_seq":4,"future_note":{"nested":true}}`)

	var evidence RequeueEvidence
	if err := jsonv2.Unmarshal(detail, &evidence); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if evidence.RetryIn != 900_000 || evidence.Class != "provider" || evidence.Provider != "anthropic" {
		t.Fatalf("evidence = %+v, want the known fields intact", evidence)
	}

	if strings.Contains(evidence.Reason, "future_note") {
		t.Fatalf("unknown key leaked into a known field: %q", evidence.Reason)
	}
}
