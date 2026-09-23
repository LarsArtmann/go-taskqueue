package executor

import (
	"encoding/json/v2"
	"fmt"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// decodePayload parses a task payload under the executor input convention:
// an empty payload and undecodable JSON are both permanent input-contract
// misses whose errors name the executor kind and show the wanted shape.
// Per-type business-rule validation stays with the caller.
func decodePayload[T any](t task.Task, kind, want string) (T, error) {
	var payload T

	if len(t.Payload) == 0 {
		return payload, Permanent(fmt.Errorf("%s: empty payload, want %s", kind, want))
	}

	if err := json.Unmarshal(t.Payload, &payload); err != nil {
		return payload, Permanent(fmt.Errorf("%s: decode payload: %w", kind, err))
	}

	return payload, nil
}
