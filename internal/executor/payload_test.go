package executor

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// decodePayload is the single seam every typed executor payload crosses, so
// its tolerance posture is a contract with every producer that ever wrote a
// payload — including producers built against a NEWER payload struct than
// this binary knows. Two halves, and they must not be confused:
//
//   - unknown KEYS are ignored, so fields can be added additively with no
//     consumer change (pinned here);
//   - a GREATER contract VERSION is still refused, because an execution
//     payload that misparses runs the wrong action instead of merely
//     rendering blank (the `p.V > 1` guard in AgentPayload's Execute path,
//     and the sibling check in depbump).
//
// Sources: OKF v0.2 SPEC §11 (consumers "MUST NOT reject a bundle because of
// unknown additional frontmatter keys") and docs/research/
// 2026-11-01_okf-lessons.md lessons 2/15.

// TestDecodePayloadIgnoresUnknownKeys pins the additive half: a payload may
// carry keys this binary does not model.
func TestDecodePayloadIgnoresUnknownKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		want    AgentPayload
	}{
		{
			name:    "flat unknown keys",
			payload: `{"repo":"acme","prompt":"do the thing","verify":"go test ./...","future_flag":true,"schema":2}`,
			want:    AgentPayload{Repo: "acme", Prompt: "do the thing", Verify: "go test ./..."},
		},
		{
			name:    "nested unknown keys",
			payload: `{"repo":"acme","prompt":"do the thing","future":{"nested":{"list":[1,2,3]}}}`,
			want:    AgentPayload{Repo: "acme", Prompt: "do the thing"},
		},
		{
			name:    "known keys only",
			payload: `{"repo":"acme","prompt":"do the thing"}`,
			want:    AgentPayload{Repo: "acme", Prompt: "do the thing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := decodePayload[AgentPayload](
				task.Task{Type: TaskTypeAgent, Payload: jsontext.Value(tt.payload)},
				TaskTypeAgent, "object with repo+prompt",
			)
			if err != nil {
				t.Fatalf("decodePayload(%s) error = %v, want nil — unknown keys must be ignored", tt.name, err)
			}

			if got.Repo != tt.want.Repo || got.Prompt != tt.want.Prompt || got.Verify != tt.want.Verify {
				t.Errorf("decodePayload(%s) = %+v, want Repo=%q Prompt=%q Verify=%q",
					tt.name, got, tt.want.Repo, tt.want.Prompt, tt.want.Verify)
			}
		})
	}
}

// TestDecodePayloadToleranceIsNotVacuous proves the pin above bites: the
// same bytes decoded with strict-unknown-members DO fail, so
// TestDecodePayloadIgnoresUnknownKeys would go red the moment somebody adds
// jsonv2.RejectUnknownMembers to decodePayload. Without this counterfactual
// the tolerance test passes for the trivial reason that the payload happens
// to be valid.
func TestDecodePayloadToleranceIsNotVacuous(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"repo":"acme","prompt":"do the thing","future_flag":true}`)

	var lenient AgentPayload
	if err := jsonv2.Unmarshal(payload, &lenient); err != nil {
		t.Fatalf("lenient unmarshal error = %v — decodePayload's posture is unreachable if the baseline itself rejects", err)
	}

	var strict AgentPayload
	err := jsonv2.Unmarshal(payload, &strict, jsonv2.RejectUnknownMembers(true))
	if err == nil {
		t.Fatal("strict unmarshal error = nil, want a rejection — the counterfactual is not exercising anything")
	}

	if !strings.Contains(err.Error(), "future_flag") {
		t.Errorf("strict unmarshal error = %q, want it to name the unknown member", err.Error())
	}
}

// TestDecodePayloadRejectsUnusableInput is the boundary of the tolerance
// above: an empty payload and unparseable JSON stay PERMANENT input-contract
// misses, and the error names the kind plus the wanted shape so the operator
// can fix the task instead of retrying it.
func TestDecodePayloadRejectsUnusableInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		wantSub []string
	}{
		{
			name:    "empty payload",
			payload: "",
			wantSub: []string{TaskTypeAgent, "empty payload", "repo+prompt"},
		},
		{
			name:    "undecodable payload",
			payload: `{"repo":`,
			wantSub: []string{TaskTypeAgent, "decode payload"},
		},
		{
			name:    "wrong shape",
			payload: `["not","an","object"]`,
			wantSub: []string{TaskTypeAgent, "decode payload"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := decodePayload[AgentPayload](
				task.Task{Type: TaskTypeAgent, Payload: jsontext.Value(tt.payload)},
				TaskTypeAgent, "object with repo+prompt",
			)
			if err == nil {
				t.Fatalf("decodePayload(%s) error = nil, want a permanent input-contract miss", tt.name)
			}

			if _, ok := errors.AsType[*PermanentError](err); !ok {
				t.Errorf("decodePayload(%s) error = %v, want *PermanentError — retrying identical bytes can never help", tt.name, err)
			}

			for _, sub := range tt.wantSub {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("decodePayload(%s) error = %q, want it to contain %q", tt.name, err.Error(), sub)
				}
			}
		})
	}
}
