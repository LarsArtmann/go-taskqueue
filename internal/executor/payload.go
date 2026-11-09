package executor

import (
	"fmt"

	"github.com/larsartmann/go-codec"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// payloadCodec is the ONE JSON door executor payload decoding goes through:
// routing the shared seam over codec.JSONCodec keeps every payload parse on
// the same dual-build implementation the journal already stamps with, so a
// future switch to stamped/enveloped payloads (codec.DecodeEnvelopeOrLegacy)
// is a seam change, not a hunt across executors (fleet review f7).
var payloadCodec = codec.JSONCodec{}

// decodePayload parses a task payload under the executor input convention:
// an empty payload and undecodable JSON are both permanent input-contract
// misses whose errors name the executor kind and show the wanted shape.
// Per-type business-rule validation stays with the caller.
func decodePayload[T any](t task.Task, kind, want string) (T, error) {
	var payload T

	if len(t.Payload) == 0 {
		return payload, Permanent(fmt.Errorf("%s: empty payload, want %s", kind, want))
	}

	if err := payloadCodec.Decode(t.Payload, &payload); err != nil {
		return payload, Permanent(fmt.Errorf("%s: decode payload: %w", kind, err))
	}

	return payload, nil
}
